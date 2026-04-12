package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func editArgs(path, oldText, newText string) map[string]any {
	return map[string]any{"path": path, "old_text": oldText, "new_text": newText}
}

func TestEditFileReplace(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world\nfoo bar\nbaz qux\n"), 0644))

	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("test.txt", "foo bar", "replaced line"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "replaced line")
	assert.NotContains(t, string(content), "foo bar")
}

func TestEditFileFirstOccurrenceOnly(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("AAA\nBBB\nAAA\n"), 0644))

	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("test.txt", "AAA", "CCC"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	// First AAA replaced, second AAA still present.
	assert.Equal(t, "CCC\nBBB\nAAA\n", string(content))
}

func TestEditFileTextNotFound(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world\n"), 0644))

	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("test.txt", "not found text", "replacement"))
	require.NoError(t, err)
	assert.Contains(t, result, `"text not found in file"`)
}

func TestEditFileNonExistent(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("nonexistent.txt", "old", "new"))
	require.NoError(t, err)
	assert.Contains(t, result, `"file does not exist`)
}

func TestEditFilePreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "crlf.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\r\nline2\r\nline3\r\n"), 0644))

	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("crlf.txt", "line2", "replaced"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "line1\r\nreplaced\r\nline3\r\n", string(content))
}

func TestEditFileDeniedPath(t *testing.T) {
	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("/etc/passwd", "old", "new"))
	require.NoError(t, err)
	assert.Contains(t, result, `"access denied: path is in restricted area"`)
}

func TestEditFileOutsideCWDNilApprover(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsidePath, []byte("content"), 0644))

	et := NewEditFileTool(nil)
	_, err := et.Execute(context.Background(), editArgs(outsidePath, "content", "new"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no approver configured")
}

func TestEditFileOutsideCWDApproved(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsidePath, []byte("old content"), 0644))

	approver := func(_ string) bool { return true }
	et := NewEditFileTool(approver)
	result, err := et.Execute(context.Background(), editArgs(outsidePath, "old content", "new content"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(outsidePath)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestEditFileMissingArgs(t *testing.T) {
	et := NewEditFileTool(nil)

	// Missing path
	args := map[string]any{"old_text": "old", "new_text": "new"}
	_, err := et.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: path")

	// Missing old_text
	args2 := map[string]any{"path": "test.txt", "new_text": "new"}
	_, err = et.Execute(context.Background(), args2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: old_text")

	// Missing new_text
	args3 := map[string]any{"path": "test.txt", "old_text": "old"}
	_, err = et.Execute(context.Background(), args3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: new_text")
}

func TestEditFileDefinition(t *testing.T) {
	et := NewEditFileTool(nil)
	def := et.Definition()
	assert.Equal(t, "function", def.Type)
	assert.Equal(t, "edit_file", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "path")
	assert.Contains(t, def.Function.Parameters.Required, "old_text")
	assert.Contains(t, def.Function.Parameters.Required, "new_text")
}

func TestEditFileName(t *testing.T) {
	et := NewEditFileTool(nil)
	assert.Equal(t, "edit_file", et.Name())
}

func TestEditFileLinesChanged(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0644))

	et := NewEditFileTool(nil)
	result, err := et.Execute(context.Background(), editArgs("test.txt", "line2", "replaced2\nextra"))
	require.NoError(t, err)

	var editResult EditResult
	require.NoError(t, json.Unmarshal([]byte(result), &editResult))
	assert.Equal(t, "ok", editResult.Status)
	assert.Greater(t, editResult.LinesChanged, 0)
}

func TestEditFilePreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "perms.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content here"), 0755))

	et := NewEditFileTool(nil)
	_, err := et.Execute(context.Background(), editArgs("perms.txt", "content here", "new content"))
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}
