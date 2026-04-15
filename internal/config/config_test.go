package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSystemPromptDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	prompt, err := LoadSystemPrompt()
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "Fenec")
}

func TestLoadSystemPromptFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the fenec config directory and system.md file at the NEW path.
	fenecDir := filepath.Join(tmpDir, ".config", "fenec")
	require.NoError(t, os.MkdirAll(fenecDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(fenecDir, "system.md"), []byte("Custom prompt"), 0644))

	prompt, err := LoadSystemPrompt()
	assert.NoError(t, err)
	assert.Equal(t, "Custom prompt", prompt)
}

func TestConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := ConfigDir()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".config", "fenec"), dir)
}

func TestDoMigrate_MovesLegacyDir(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "legacy-config")
	newDir := filepath.Join(tmpDir, "new-parent", "fenec")

	// Create legacy dir with a config file.
	require.NoError(t, os.MkdirAll(legacy, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("key = \"value\""), 0644))

	var buf bytes.Buffer
	doMigrate(legacy, newDir, &buf)

	// Legacy dir should be gone.
	_, err := os.Stat(legacy)
	assert.True(t, os.IsNotExist(err), "legacy dir should no longer exist")

	// New dir should contain the config file.
	data, err := os.ReadFile(filepath.Join(newDir, "config.toml"))
	require.NoError(t, err)
	assert.Equal(t, "key = \"value\"", string(data))
}

func TestDoMigrate_NoLegacyDir(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "nonexistent-legacy")
	newDir := filepath.Join(tmpDir, "new-config", "fenec")

	var buf bytes.Buffer
	doMigrate(legacy, newDir, &buf)

	// New dir should NOT have been created.
	_, err := os.Stat(newDir)
	assert.True(t, os.IsNotExist(err), "new dir should not be created when no legacy exists")

	// No output on stderr.
	assert.Empty(t, buf.String())
}

func TestDoMigrate_NewPathAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "legacy-config")
	newDir := filepath.Join(tmpDir, "new-config", "fenec")

	// Create both dirs with different content.
	require.NoError(t, os.MkdirAll(legacy, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "old.toml"), []byte("old"), 0644))
	require.NoError(t, os.MkdirAll(newDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(newDir, "new.toml"), []byte("new"), 0644))

	var buf bytes.Buffer
	doMigrate(legacy, newDir, &buf)

	// Legacy dir should still exist (not moved).
	_, err := os.Stat(filepath.Join(legacy, "old.toml"))
	assert.NoError(t, err, "legacy dir should be untouched")

	// New dir should still have its original content.
	data, err := os.ReadFile(filepath.Join(newDir, "new.toml"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))

	// No migration output.
	assert.Empty(t, buf.String())
}

func TestDoMigrate_StderrFeedback(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "legacy-config")
	newDir := filepath.Join(tmpDir, "new-parent", "fenec")

	require.NoError(t, os.MkdirAll(legacy, 0755))

	var buf bytes.Buffer
	doMigrate(legacy, newDir, &buf)

	assert.Contains(t, buf.String(), "fenec: migrated config from")
	assert.Contains(t, buf.String(), legacy)
	assert.Contains(t, buf.String(), newDir)
}

func TestDoMigrate_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "legacy-config")
	// Parent "deep/nested" does NOT exist yet.
	newDir := filepath.Join(tmpDir, "deep", "nested", "fenec")

	require.NoError(t, os.MkdirAll(legacy, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("data"), 0644))

	var buf bytes.Buffer
	doMigrate(legacy, newDir, &buf)

	// Parent directories should have been created.
	data, err := os.ReadFile(filepath.Join(newDir, "config.toml"))
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

func TestHistoryFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path, err := HistoryFile()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".config", "fenec", "history"), path)

	// Verify the parent directory was created.
	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "parent directory should exist")
}

func TestSessionDirCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := SessionDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".config", "fenec", "sessions"), dir)

	// Verify directory exists.
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestToolsDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := ToolsDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".config", "fenec", "tools"), dir)
}

func TestToolsDirDoesNotCreate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := ToolsDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".config", "fenec", "tools"), dir)
	// The directory should NOT have been created by ToolsDir.
	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "ToolsDir should not create the directory")
}

func TestDefaultHostValue(t *testing.T) {
	assert.Equal(t, "http://localhost:11434", DefaultHost)
}

func TestVersionValue(t *testing.T) {
	assert.Equal(t, "v0.1", Version)
}
