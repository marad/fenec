package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSystemPromptDefault(t *testing.T) {
	// Set up a temp dir with no system.md file.
	tmpDir := t.TempDir()
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
	} else {
		// On macOS, UserConfigDir uses ~/Library/Application Support
		// which we can't easily override, so we skip the env override.
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
	}

	prompt, err := LoadSystemPrompt()
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "Fenec")
}

func TestLoadSystemPromptFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create the fenec config directory and system.md file.
	fenecDir := filepath.Join(tmpDir, "fenec")
	require.NoError(t, os.MkdirAll(fenecDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(fenecDir, "system.md"), []byte("Custom prompt"), 0644))

	prompt, err := LoadSystemPrompt()
	assert.NoError(t, err)
	assert.Equal(t, "Custom prompt", prompt)
}

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	assert.NoError(t, err)
	assert.True(t, filepath.Base(dir) == "fenec", "ConfigDir should end with 'fenec', got: %s", dir)
}

func TestHistoryFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path, err := HistoryFile()
	require.NoError(t, err)
	assert.True(t, filepath.Base(path) == "history", "HistoryFile should end with 'history', got: %s", path)

	// Verify the parent directory was created.
	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "parent directory should exist")
}

func TestSessionDirCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	dir, err := SessionDir()
	require.NoError(t, err)
	assert.Contains(t, dir, "sessions")

	// Verify directory exists.
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDefaultHostValue(t *testing.T) {
	assert.Equal(t, "http://localhost:11434", DefaultHost)
}

func TestVersionValue(t *testing.T) {
	assert.Equal(t, "v0.1", Version)
}
