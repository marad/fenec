package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDirArgs(path string) api.ToolCallFunctionArguments {
	args := api.NewToolCallFunctionArguments()
	args.Set("path", path)
	return args
}

func TestListDirSorted(t *testing.T) {
	tmpDir := t.TempDir()
	// Create files and subdirectories
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "banana.txt"), []byte("b"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "apple.txt"), []byte("a"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "zebra"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "alpha"), 0755))

	lt := NewListDirTool()
	result, err := lt.Execute(context.Background(), makeDirArgs(tmpDir))
	require.NoError(t, err)

	var entries []DirEntry
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	require.Len(t, entries, 4)

	// Dirs first, alphabetically
	assert.Equal(t, "alpha", entries[0].Name)
	assert.True(t, entries[0].IsDir)
	assert.Equal(t, int64(0), entries[0].Size)

	assert.Equal(t, "zebra", entries[1].Name)
	assert.True(t, entries[1].IsDir)

	// Files next, alphabetically
	assert.Equal(t, "apple.txt", entries[2].Name)
	assert.False(t, entries[2].IsDir)
	assert.Equal(t, int64(1), entries[2].Size) // single byte "a"

	assert.Equal(t, "banana.txt", entries[3].Name)
	assert.False(t, entries[3].IsDir)
	assert.Equal(t, int64(1), entries[3].Size) // single byte "b"
}

func TestListDirEntryFields(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(content), 0644))

	lt := NewListDirTool()
	result, err := lt.Execute(context.Background(), makeDirArgs(tmpDir))
	require.NoError(t, err)

	var entries []DirEntry
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "test.txt", entries[0].Name)
	assert.False(t, entries[0].IsDir)
	assert.Equal(t, int64(len(content)), entries[0].Size)
}

func TestListDirDeniedPath(t *testing.T) {
	lt := NewListDirTool()
	result, err := lt.Execute(context.Background(), makeDirArgs("/etc"))
	require.NoError(t, err)

	var resp map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Contains(t, resp["error"], "access denied")
}

func TestListDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	lt := NewListDirTool()
	result, err := lt.Execute(context.Background(), makeDirArgs(tmpDir))
	require.NoError(t, err)

	var entries []DirEntry
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	assert.Empty(t, entries)
}

func TestListDirNonExistent(t *testing.T) {
	lt := NewListDirTool()
	_, err := lt.Execute(context.Background(), makeDirArgs("/tmp/nonexistent_fenec_dir_abc123"))
	assert.Error(t, err)
}

func TestListDirToolName(t *testing.T) {
	lt := NewListDirTool()
	assert.Equal(t, "list_directory", lt.Name())
}

func TestListDirToolDefinition(t *testing.T) {
	lt := NewListDirTool()
	def := lt.Definition()
	assert.Equal(t, "function", def.Type)
	assert.Equal(t, "list_directory", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "path")
}
