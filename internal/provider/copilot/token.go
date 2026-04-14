package copilot

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// resolveTokenWith resolves the GitHub auth token using injectable functions for testability.
// Priority: GH_TOKEN env var > GITHUB_TOKEN env var > gh auth token CLI.
func resolveTokenWith(lookPath func(string) (string, error), command func(string, ...string) ([]byte, error)) (string, error) {
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	if _, err := lookPath("gh"); err != nil {
		return "", fmt.Errorf("copilot provider requires the GitHub CLI (gh). Install from: https://cli.github.com")
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
