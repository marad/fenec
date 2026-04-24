package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marad/fenec/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderImplementsInterface(t *testing.T) {
	var _ provider.Provider = (*Provider)(nil)
}

func TestNewWithGHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "gho_test_token_for_new")
	t.Setenv("GITHUB_TOKEN", "")

	p, err := New()
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestProviderName(t *testing.T) {
	t.Setenv("GH_TOKEN", "gho_test_token_for_name")
	t.Setenv("GITHUB_TOKEN", "")

	p, err := New()
	require.NoError(t, err)
	assert.Equal(t, "copilot", p.Name())
}

func TestProviderDefaultModel(t *testing.T) {
	t.Setenv("GH_TOKEN", "gho_test_token_for_model")
	t.Setenv("GITHUB_TOKEN", "")

	p, err := New()
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", p.DefaultModel())
}

func TestNewWithoutTokenFailsWhenNoGh(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	_, err := New()
	if err == nil {
		t.Skip("gh CLI is installed and authenticated — cannot test no-token failure path")
	}
	assert.Error(t, err)
}

// newSessionServer creates a test HTTP server that serves Copilot session tokens.
func newSessionServer(t *testing.T, expiresAt int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(copilotSession{
			Token:     "copilot-session-token-123",
			ExpiresAt: expiresAt,
		})
	}))
}

func TestEnsureSessionFetchesToken(t *testing.T) {
	// Session token expires in 1 hour.
	srv := newSessionServer(t, time.Now().Unix()+3600)
	defer srv.Close()

	p := &Provider{
		githubToken: "gh-token",
		sessionURL:  srv.URL,
	}

	inner, err := p.ensureSession(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, inner)
	assert.Equal(t, "copilot-session-token-123", p.session.Token)
}

func TestEnsureSessionCachesToken(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(copilotSession{
			Token:     "copilot-token",
			ExpiresAt: time.Now().Unix() + 3600,
		})
	}))
	defer srv.Close()

	p := &Provider{
		githubToken: "gh-token",
		sessionURL:  srv.URL,
	}

	_, err := p.ensureSession(context.Background())
	require.NoError(t, err)
	_, err = p.ensureSession(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, callCount, "session token should be fetched only once while valid")
}

func TestEnsureSessionRefreshesExpiredToken(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(copilotSession{
			Token:     fmt.Sprintf("token-%d", callCount),
			ExpiresAt: time.Now().Unix() + 3600,
		})
	}))
	defer srv.Close()

	p := &Provider{
		githubToken: "gh-token",
		sessionURL:  srv.URL,
	}

	// First call: fetches token.
	_, err := p.ensureSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token-1", p.session.Token)

	// Simulate expired token (set expiry to past).
	p.session.ExpiresAt = time.Now().Unix() - 100

	// Second call: should refresh.
	_, err = p.ensureSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token-2", p.session.Token)
	assert.Equal(t, 2, callCount)
}

func TestEnsureSessionAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := &Provider{
		githubToken: "bad-token",
		sessionURL:  srv.URL,
	}

	_, err := p.ensureSession(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestEnsureSessionForbiddenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := &Provider{
		githubToken: "no-copilot-token",
		sessionURL:  srv.URL,
	}

	_, err := p.ensureSession(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Copilot access denied")
}

func TestEnsureSessionDeviceFlowRecovery(t *testing.T) {
	// Use temp HOME so storeCopilotToken doesn't write to real home.
	t.Setenv("HOME", t.TempDir())

	// Session token endpoint returns 404 first (no copilot scope),
	// then succeeds after device flow provides a new token.
	callCount := 0
	sessionSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		auth := r.Header.Get("Authorization")
		if auth == "token old-gh-token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// New token from device flow works.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(copilotSession{
			Token:     "copilot-session-from-device-flow",
			ExpiresAt: time.Now().Unix() + 3600,
		})
	}))
	defer sessionSrv.Close()

	// Mock device flow — immediately returns a token.
	origDeviceFlowAuth := DeviceFlowAuth
	DeviceFlowAuth = func(ctx context.Context, notify func(string, string)) (string, error) {
		return "new-copilot-token", nil
	}
	defer func() { DeviceFlowAuth = origDeviceFlowAuth }()

	p := &Provider{
		githubToken: "old-gh-token",
		sessionURL:  sessionSrv.URL,
	}

	inner, err := p.ensureSession(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, inner)
	assert.Equal(t, "new-copilot-token", p.githubToken)
	assert.Equal(t, 2, callCount, "should have called session endpoint twice (404 + success)")
}

func TestGetContextLengthReturnsZero(t *testing.T) {
	p := &Provider{}
	length, err := p.GetContextLength(context.Background(), "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, 0, length)
}
