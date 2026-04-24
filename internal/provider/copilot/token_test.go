package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock helpers for resolveTokenWith ---

func mockLookPathOK(path string) func(string) (string, error) {
	return func(file string) (string, error) { return path, nil }
}

func mockLookPathErr() func(string) (string, error) {
	return func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
}

func mockCommandOK(output string) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		return []byte(output), nil
	}
}

func mockCommandExitError(code int, stderr string) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("echo -n '%s' >&2; exit %d", stderr, code))
		out, err := cmd.Output()
		return out, err
	}
}

// --- resolveTokenWith tests ---

func TestResolveTokenWithGHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "gh-token-value")
	t.Setenv("GITHUB_TOKEN", "")

	neverCalled := func(name string, args ...string) ([]byte, error) {
		t.Fatal("command should not be called when GH_TOKEN is set")
		return nil, nil
	}

	token, err := resolveTokenWith(mockLookPathOK("/usr/bin/gh"), neverCalled)
	require.NoError(t, err)
	assert.Equal(t, "gh-token-value", token)
}

func TestResolveTokenWithGitHubToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-token-value")

	neverCalled := func(name string, args ...string) ([]byte, error) {
		t.Fatal("command should not be called when GITHUB_TOKEN is set")
		return nil, nil
	}

	token, err := resolveTokenWith(mockLookPathOK("/usr/bin/gh"), neverCalled)
	require.NoError(t, err)
	assert.Equal(t, "github-token-value", token)
}

func TestResolveTokenWithGHTokenPriority(t *testing.T) {
	t.Setenv("GH_TOKEN", "gh-wins")
	t.Setenv("GITHUB_TOKEN", "github-loses")

	neverCalled := func(name string, args ...string) ([]byte, error) {
		t.Fatal("command should not be called when env vars are set")
		return nil, nil
	}

	token, err := resolveTokenWith(mockLookPathOK("/usr/bin/gh"), neverCalled)
	require.NoError(t, err)
	assert.Equal(t, "gh-wins", token)
}

func TestResolveTokenWithGhCLI(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("HOME", t.TempDir()) // No copilot config files.

	token, err := resolveTokenWith(
		mockLookPathOK("/usr/bin/gh"),
		mockCommandOK("gho_test_token_123\n"),
	)
	require.NoError(t, err)
	assert.Equal(t, "gho_test_token_123", token)
}

func TestResolveTokenWithGhNotInstalled(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("HOME", t.TempDir())

	_, err := resolveTokenWith(
		mockLookPathErr(),
		mockCommandOK("should-not-be-called"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cli.github.com")
}

func TestResolveTokenWithGhNotAuthenticated(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("HOME", t.TempDir())

	_, err := resolveTokenWith(
		mockLookPathOK("/usr/bin/gh"),
		mockCommandExitError(4, "To get started with GitHub CLI, please run: gh auth login"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh auth login")
}

func TestResolveTokenWithGhOtherError(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("HOME", t.TempDir())

	_, err := resolveTokenWith(
		mockLookPathOK("/usr/bin/gh"),
		mockCommandExitError(1, "some other error"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh auth token failed")
}

func TestResolveTokenWithEmptyOutput(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("HOME", t.TempDir())

	_, err := resolveTokenWith(
		mockLookPathOK("/usr/bin/gh"),
		mockCommandOK(""),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
}

// --- readCopilotConfigToken tests ---

func TestReadCopilotConfigTokenFromHostsJSON(t *testing.T) {
	// Override HOME to use a temp directory.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config", "github-copilot")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	hostsData := map[string]copilotHostEntry{
		"github.com": {OAuthToken: "gho_copilot_hosts_token"},
	}
	data, _ := json.Marshal(hostsData)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "hosts.json"), data, 0644))

	token, err := readCopilotConfigToken()
	require.NoError(t, err)
	assert.Equal(t, "gho_copilot_hosts_token", token)
}

func TestReadCopilotConfigTokenFromAppsJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config", "github-copilot")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	appsData := map[string]copilotHostEntry{
		"github.com": {OAuthToken: "gho_copilot_apps_token"},
	}
	data, _ := json.Marshal(appsData)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "apps.json"), data, 0644))

	token, err := readCopilotConfigToken()
	require.NoError(t, err)
	assert.Equal(t, "gho_copilot_apps_token", token)
}

func TestReadCopilotConfigTokenHostsPriority(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config", "github-copilot")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// Both files exist — hosts.json should win.
	hostsData := map[string]copilotHostEntry{
		"github.com": {OAuthToken: "gho_hosts_wins"},
	}
	hd, _ := json.Marshal(hostsData)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "hosts.json"), hd, 0644))

	appsData := map[string]copilotHostEntry{
		"github.com": {OAuthToken: "gho_apps_loses"},
	}
	ad, _ := json.Marshal(appsData)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "apps.json"), ad, 0644))

	token, err := readCopilotConfigToken()
	require.NoError(t, err)
	assert.Equal(t, "gho_hosts_wins", token)
}

func TestReadCopilotConfigTokenNoFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	_, err := readCopilotConfigToken()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no copilot config token found")
}

func TestReadCopilotConfigTokenEmptyToken(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config", "github-copilot")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	hostsData := map[string]copilotHostEntry{
		"github.com": {OAuthToken: ""},
	}
	data, _ := json.Marshal(hostsData)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "hosts.json"), data, 0644))

	_, err := readCopilotConfigToken()
	require.Error(t, err)
}

// --- fetchSessionTokenFrom tests ---

func TestFetchSessionTokenSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token gh-test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(copilotSession{
			Token:     "copilot-session-abc",
			ExpiresAt: 1700000000,
		})
	}))
	defer srv.Close()

	session, err := fetchSessionTokenFrom(context.Background(), srv.URL, "gh-test-token")
	require.NoError(t, err)
	assert.Equal(t, "copilot-session-abc", session.Token)
	assert.Equal(t, int64(1700000000), session.ExpiresAt)
}

func TestFetchSessionToken401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := fetchSessionTokenFrom(context.Background(), srv.URL, "bad-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestFetchSessionToken403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := fetchSessionTokenFrom(context.Background(), srv.URL, "no-copilot")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Copilot access denied")
}

func TestFetchSessionToken404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchSessionTokenFrom(context.Background(), srv.URL, "no-scope-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "copilot")
	assert.Contains(t, err.Error(), "404")
}

func TestFetchSessionTokenEmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(copilotSession{
			Token:     "",
			ExpiresAt: 1700000000,
		})
	}))
	defer srv.Close()

	_, err := fetchSessionTokenFrom(context.Background(), srv.URL, "gh-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session token is empty")
}

func TestFetchSessionTokenNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := fetchSessionTokenFrom(context.Background(), url, "gh-token")
	require.Error(t, err)
}
