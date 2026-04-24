package copilot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceFlowAuthSuccess(t *testing.T) {
	var pollCount atomic.Int32

	// Mock device code endpoint.
	codeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deviceCodeResponse{
			DeviceCode:      "test-device-code",
			UserCode:        "TEST-1234",
			VerificationURI: "https://github.com/login/device",
			ExpiresIn:       60,
			Interval:        1, // Short interval for test speed.
		})
	}))
	defer codeSrv.Close()

	// Mock access token endpoint — return pending once, then success.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		count := pollCount.Add(1)
		if count < 2 {
			json.NewEncoder(w).Encode(accessTokenResponse{
				Error:     "authorization_pending",
				ErrorDesc: "waiting for user",
			})
		} else {
			json.NewEncoder(w).Encode(accessTokenResponse{
				AccessToken: "gho_copilot_device_token",
				TokenType:   "bearer",
				Scope:       "copilot",
			})
		}
	}))
	defer tokenSrv.Close()

	var notified bool
	token, err := deviceFlowAuthWith(context.Background(), codeSrv.URL, tokenSrv.URL, func(userCode, uri string) {
		notified = true
		assert.Equal(t, "TEST-1234", userCode)
		assert.Equal(t, "https://github.com/login/device", uri)
	})

	require.NoError(t, err)
	assert.Equal(t, "gho_copilot_device_token", token)
	assert.True(t, notified, "notify callback should have been called")
	assert.GreaterOrEqual(t, pollCount.Load(), int32(2))
}

func TestDeviceFlowAuthAccessDenied(t *testing.T) {
	codeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deviceCodeResponse{
			DeviceCode:      "test-code",
			UserCode:        "DENY-0000",
			VerificationURI: "https://github.com/login/device",
			ExpiresIn:       60,
			Interval:        1,
		})
	}))
	defer codeSrv.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessTokenResponse{
			Error:     "access_denied",
			ErrorDesc: "user denied",
		})
	}))
	defer tokenSrv.Close()

	_, err := deviceFlowAuthWith(context.Background(), codeSrv.URL, tokenSrv.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestDeviceFlowAuthCancelled(t *testing.T) {
	codeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deviceCodeResponse{
			DeviceCode:      "test-code",
			UserCode:        "CANCEL-00",
			VerificationURI: "https://github.com/login/device",
			ExpiresIn:       60,
			Interval:        1,
		})
	}))
	defer codeSrv.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessTokenResponse{
			Error:     "authorization_pending",
			ErrorDesc: "waiting",
		})
	}))
	defer tokenSrv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := deviceFlowAuthWith(ctx, codeSrv.URL, tokenSrv.URL, nil)
	require.Error(t, err)
}

func TestDeviceFlowAuthCodeEndpointError(t *testing.T) {
	codeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer codeSrv.Close()

	_, err := deviceFlowAuthWith(context.Background(), codeSrv.URL, "http://unused", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPollAccessTokenPending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessTokenResponse{
			Error:     "authorization_pending",
			ErrorDesc: "still waiting",
		})
	}))
	defer srv.Close()

	token, done, err := pollAccessToken(context.Background(), srv.URL, "test-code")
	require.NoError(t, err)
	assert.False(t, done)
	assert.Empty(t, token)
}

func TestPollAccessTokenSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accessTokenResponse{
			AccessToken: "gho_test_token",
			TokenType:   "bearer",
		})
	}))
	defer srv.Close()

	token, done, err := pollAccessToken(context.Background(), srv.URL, "test-code")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, "gho_test_token", token)
}
