package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// copilotClientID is the OAuth client ID used by the Copilot Vim/Neovim plugin.
	// It is commonly reused by CLI tools for device flow authentication.
	copilotClientID = "Iv1.b507a08c87ecfe98"

	deviceCodeURL   = "https://github.com/login/device/code"
	accessTokenURL  = "https://github.com/login/oauth/access_token"
)

// deviceCodeResponse is the response from the device code request.
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// accessTokenResponse is the response from the access token polling request.
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// DeviceFlowAuth performs the GitHub OAuth device flow to obtain a Copilot-scoped token.
// It prints the user code and verification URL to stderr, then polls until the user
// authorizes the device or the code expires.
//
// The notify callback is called with the user code and verification URL so the caller
// can display them to the user. If notify is nil, a default fmt.Fprintf(os.Stderr, ...) is used.
//
// This is a package-level variable so it can be replaced in tests.
var DeviceFlowAuth = func(ctx context.Context, notify func(userCode, verificationURI string)) (string, error) {
	return deviceFlowAuthWith(ctx, deviceCodeURL, accessTokenURL, notify)
}

// deviceFlowAuthWith performs the device flow using injectable URLs for testability.
func deviceFlowAuthWith(ctx context.Context, codeURL, tokenURL string, notify func(userCode, verificationURI string)) (string, error) {
	// Step 1: Request device code.
	codeReq := map[string]string{
		"client_id": copilotClientID,
		"scope":     "copilot",
	}
	body, _ := json.Marshal(codeReq)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codeURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("copilot auth: building device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("copilot auth: requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("copilot auth: device code endpoint returned %s", resp.Status)
	}

	var codeResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&codeResp); err != nil {
		return "", fmt.Errorf("copilot auth: decoding device code response: %w", err)
	}

	// Step 2: Notify user.
	if notify != nil {
		notify(codeResp.UserCode, codeResp.VerificationURI)
	}

	// Step 3: Poll for access token.
	interval := time.Duration(codeResp.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(codeResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		token, done, err := pollAccessToken(ctx, tokenURL, codeResp.DeviceCode)
		if err != nil {
			return "", err
		}
		if done {
			return token, nil
		}
	}

	return "", fmt.Errorf("copilot auth: device code expired. Please try again")
}

// pollAccessToken makes a single poll request for the access token.
// Returns (token, true, nil) on success, ("", false, nil) if still pending,
// or ("", false, err) on terminal errors.
func pollAccessToken(ctx context.Context, tokenURL, deviceCode string) (string, bool, error) {
	return pollAccessTokenWith(ctx, tokenURL, deviceCode, copilotClientID)
}

func pollAccessTokenWith(ctx context.Context, tokenURL, deviceCode, clientID string) (string, bool, error) {
	tokenReq := map[string]string{
		"client_id":   clientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}
	body, _ := json.Marshal(tokenReq)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("copilot auth: building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("copilot auth: polling for token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", false, fmt.Errorf("copilot auth: decoding token response: %w", err)
	}

	switch tokenResp.Error {
	case "":
		// Success.
		if tokenResp.AccessToken == "" {
			return "", false, fmt.Errorf("copilot auth: received empty access token")
		}
		return tokenResp.AccessToken, true, nil
	case "authorization_pending":
		// User hasn't authorized yet, keep polling.
		return "", false, nil
	case "slow_down":
		// We're polling too fast — next iteration will wait the normal interval.
		return "", false, nil
	case "expired_token":
		return "", false, fmt.Errorf("copilot auth: device code expired. Please try again")
	case "access_denied":
		return "", false, fmt.Errorf("copilot auth: user denied the authorization request")
	default:
		return "", false, fmt.Errorf("copilot auth: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
}
