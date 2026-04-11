package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeArgs(path, content string) api.ToolCallFunctionArguments {
	args := api.NewToolCallFunctionArguments()
	args.Set("path", path)
	args.Set("content", content)
	return args
}

func TestWriteFileNewFile(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	wt := NewWriteFileTool(nil)
	result, err := wt.Execute(context.Background(), writeArgs("test.txt", "hello world"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)
	assert.Contains(t, result, `"bytes_written":11`)

	content, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestWriteFileMkdirP(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	wt := NewWriteFileTool(nil)
	result, err := wt.Execute(context.Background(), writeArgs("a/b/c/deep.txt", "nested content"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(filepath.Join(dir, "a", "b", "c", "deep.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(content))
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	filePath := filepath.Join(dir, "existing.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("old content"), 0644))

	wt := NewWriteFileTool(nil)
	result, err := wt.Execute(context.Background(), writeArgs("existing.txt", "new content"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestWriteFileDeniedPath(t *testing.T) {
	wt := NewWriteFileTool(nil)
	result, err := wt.Execute(context.Background(), writeArgs("/etc/test", "bad"))
	require.NoError(t, err)
	assert.Contains(t, result, `"access denied: path is in restricted area"`)
}

func TestWriteFileOutsideCWDNilApprover(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	wt := NewWriteFileTool(nil)
	_, err := wt.Execute(context.Background(), writeArgs(outsidePath, "data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no approver configured")
}

func TestWriteFileOutsideCWDDenied(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	approver := func(_ string) bool { return false }
	wt := NewWriteFileTool(approver)
	_, err := wt.Execute(context.Background(), writeArgs(outsidePath, "data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied by user")
}

func TestWriteFileOutsideCWDApproved(t *testing.T) {
	dir := t.TempDir()
	oldCwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(oldCwd)

	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "approved.txt")
	approver := func(_ string) bool { return true }
	wt := NewWriteFileTool(approver)
	result, err := wt.Execute(context.Background(), writeArgs(outsidePath, "approved data"))
	require.NoError(t, err)
	assert.Contains(t, result, `"status":"ok"`)

	content, err := os.ReadFile(outsidePath)
	require.NoError(t, err)
	assert.Equal(t, "approved data", string(content))
}

func TestWriteFileMissingArgs(t *testing.T) {
	wt := NewWriteFileTool(nil)

	// Missing path
	args := api.NewToolCallFunctionArguments()
	args.Set("content", "data")
	_, err := wt.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: path")

	// Missing content
	args2 := api.NewToolCallFunctionArguments()
	args2.Set("path", "test.txt")
	_, err = wt.Execute(context.Background(), args2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: content")
}

func TestWriteFileDefinition(t *testing.T) {
	wt := NewWriteFileTool(nil)
	def := wt.Definition()
	assert.Equal(t, "function", def.Type)
	assert.Equal(t, "write_file", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "path")
	assert.Contains(t, def.Function.Parameters.Required, "content")
}

func TestWriteFileName(t *testing.T) {
	wt := NewWriteFileTool(nil)
	assert.Equal(t, "write_file", wt.Name())
}
