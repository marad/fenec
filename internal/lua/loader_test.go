package lua

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// copyFixture copies a testdata file into the target directory.
func copyFixture(t *testing.T, srcName, dstDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", srcName))
	require.NoError(t, err, "reading fixture %s", srcName)
	err = os.WriteFile(filepath.Join(dstDir, srcName), data, 0644)
	require.NoError(t, err, "writing fixture %s to %s", srcName, dstDir)
}

func TestLoadToolsValidDir(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "word_count.lua", dir)

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Len(t, result.Errors, 0)
	assert.Equal(t, "word_count", result.Tools[0].Name())
}

func TestLoadToolsEmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 0)
	assert.Len(t, result.Errors, 0)
}

func TestLoadToolsMissingDir(t *testing.T) {
	result, err := LoadTools("/tmp/nonexistent-dir-12345")
	require.NoError(t, err, "missing directory should not be an error")
	assert.Len(t, result.Tools, 0)
	assert.Len(t, result.Errors, 0)
}

func TestLoadToolsSyntaxError(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "syntax_error.lua", dir)

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 0)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Path, "syntax_error.lua")
	assert.NotEmpty(t, result.Errors[0].Reason)
}

func TestLoadToolsMissingFields(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "no_execute.lua", dir)

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 0)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Path, "no_execute.lua")
	assert.Contains(t, result.Errors[0].Reason, "execute")
}

func TestLoadToolsMixedDir(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "word_count.lua", dir)
	copyFixture(t, "syntax_error.lua", dir)
	copyFixture(t, "no_name.lua", dir)

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1, "only word_count should load")
	assert.Equal(t, "word_count", result.Tools[0].Name())
	assert.Len(t, result.Errors, 2, "syntax_error and no_name should both fail")
}

func TestLoadToolsIgnoresNonLua(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "word_count.lua", dir)
	// Create a non-Lua file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.txt"), []byte("hello"), 0644))

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Len(t, result.Errors, 0)
}

func TestLoadToolsIgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "word_count.lua", dir)
	// Create a subdirectory.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0755))

	result, err := LoadTools(dir)
	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Len(t, result.Errors, 0)
}
