package copilot

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock helpers ---

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
		// Run a real subprocess to produce a genuine exec.ExitError
		cmd := exec.Command("sh", "-c", fmt.Sprintf("echo -n '%s' >&2; exit %d", stderr, code))
		out, err := cmd.Output()
		return out, err
	}
}

// --- Tests ---

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

	_, err := resolveTokenWith(
		mockLookPathOK("/usr/bin/gh"),
		mockCommandOK(""),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
}
