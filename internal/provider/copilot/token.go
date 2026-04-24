package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// sessionTokenURL is the GitHub API endpoint that exchanges a GitHub token
// for a short-lived Copilot session token.
const sessionTokenURL = "https://api.github.com/copilot_internal/v2/token"

// copilotSession holds a short-lived Copilot API session token and its expiry.
type copilotSession struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// fetchSessionToken exchanges a GitHub token for a short-lived Copilot session token
// using the default endpoint.
func fetchSessionToken(ctx context.Context, githubToken string) (*copilotSession, error) {
	return fetchSessionTokenFrom(ctx, sessionTokenURL, githubToken)
}

// fetchSessionTokenFrom exchanges a GitHub token for a Copilot session token
// from the given URL. Separated for testability.
func fetchSessionTokenFrom(ctx context.Context, url, githubToken string) (*copilotSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("copilot: building session token request: %w", err)
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.35.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot: fetching session token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("copilot: GitHub token is invalid or expired. Run: gh auth login")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("copilot: GitHub Copilot access denied. Ensure you have an active Copilot subscription")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, &errSessionNotFound{}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot: session token endpoint returned %s", resp.Status)
	}

	var session copilotSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("copilot: decoding session token: %w", err)
	}

	if session.Token == "" {
		return nil, fmt.Errorf("copilot: session token is empty")
	}

	return &session, nil
}

// errSessionNotFound is a sentinel error for 404 responses from the session token endpoint.
// This means the token lacks the copilot scope and device flow auth is needed.
type errSessionNotFound struct{}

func (e *errSessionNotFound) Error() string {
	return "copilot: session token endpoint returned 404 — token likely lacks copilot scope"
}

// resolveTokenWith resolves the GitHub auth token using injectable functions for testability.
// Priority: GH_TOKEN env var > GITHUB_TOKEN env var > Copilot config files > gh auth token CLI.
// Does NOT trigger device flow — that is handled by the Provider's ensureSession recovery.
func resolveTokenWith(lookPath func(string) (string, error), command func(string, ...string) ([]byte, error)) (string, error) {
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// Try Copilot editor plugin config files — these tokens always have the copilot scope.
	if token, err := readCopilotConfigToken(); err == nil && token != "" {
		return token, nil
	}

	if _, err := lookPath("gh"); err != nil {
		return "", fmt.Errorf("copilot provider requires GitHub authentication.\n" +
			"  Install GitHub CLI: https://cli.github.com\n" +
			"  Or set GH_TOKEN / GITHUB_TOKEN env var")
	}
	out, err := command("gh", "auth", "token", "--hostname", "github.com")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 4 {
				return "", fmt.Errorf("GitHub CLI is not authenticated. Run: gh auth login")
			}
			return "", fmt.Errorf("gh auth token failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("gh auth token failed: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("gh auth token returned empty token")
	}
	return token, nil
}

// resolveToken resolves the GitHub auth token using the real environment and gh CLI.
func resolveToken() (string, error) {
	return resolveTokenWith(
		exec.LookPath,
		func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		},
	)
}

// copilotHostEntry represents a single host entry in a Copilot config file.
type copilotHostEntry struct {
	OAuthToken string `json:"oauth_token"`
}

// copilotConfigDir returns the path to the Copilot config directory.
func copilotConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "github-copilot"), nil
}

// readCopilotConfigToken reads the GitHub OAuth token from Copilot editor plugin
// config files. These are created by the official Copilot plugins for VS Code,
// Neovim, Vim, etc. and always have the copilot scope.
//
// Checked paths (in order):
//   - ~/.config/github-copilot/hosts.json
//   - ~/.config/github-copilot/apps.json
func readCopilotConfigToken() (string, error) {
	dir, err := copilotConfigDir()
	if err != nil {
		return "", err
	}

	for _, filename := range []string{"hosts.json", "apps.json"} {
		path := filepath.Join(dir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var hosts map[string]copilotHostEntry
		if err := json.Unmarshal(data, &hosts); err != nil {
			continue
		}

		if entry, ok := hosts["github.com"]; ok && entry.OAuthToken != "" {
			return entry.OAuthToken, nil
		}
	}

	return "", fmt.Errorf("no copilot config token found")
}

// storeCopilotToken saves a Copilot OAuth token to ~/.config/github-copilot/hosts.json
// so it can be reused in subsequent sessions.
func storeCopilotToken(token string) error {
	dir, err := copilotConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("copilot: creating config dir: %w", err)
	}

	hosts := map[string]copilotHostEntry{
		"github.com": {OAuthToken: token},
	}
	data, err := json.MarshalIndent(hosts, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "hosts.json")
	return os.WriteFile(path, data, 0600)
}
