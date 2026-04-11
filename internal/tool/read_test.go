package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFileArgs(path string, extras ...interface{}) api.ToolCallFunctionArguments {
	args := api.NewToolCallFunctionArguments()
	args.Set("path", path)
	for i := 0; i+1 < len(extras); i += 2 {
		key := extras[i].(string)
		args.Set(key, extras[i+1])
	}
	return args
}

func TestReadFileSimple(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	content := "line one\nline two\nline three\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	rt := NewReadFileTool()
	result, err := rt.Execute(context.Background(), makeFileArgs(path))
	require.NoError(t, err)

	var rr ReadResult
	require.NoError(t, json.Unmarshal([]byte(result), &rr))
	assert.Equal(t, 3, rr.TotalLines)
	assert.Equal(t, 3, rr.LinesShown)
	assert.False(t, rr.Truncated)
	assert.Contains(t, rr.Content, "line one")
	assert.Contains(t, rr.Content, "line three")
}

func TestReadFileOffsetLimit(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "numbered.txt")
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644))

	rt := NewReadFileTool()
	// offset=2, limit=3 should return lines 3, 4, 5 (0-based offset)
	result, err := rt.Execute(context.Background(), makeFileArgs(path, "offset", float64(2), "limit", float64(3)))
	require.NoError(t, err)

	var rr ReadResult
	require.NoError(t, json.Unmarshal([]byte(result), &rr))
	assert.Equal(t, 10, rr.TotalLines)
	assert.Equal(t, 3, rr.LinesShown)
	assert.False(t, rr.Truncated)
	assert.Contains(t, rr.Content, "line 3")
	assert.Contains(t, rr.Content, "line 5")
	assert.NotContains(t, rr.Content, "line 2")
	assert.NotContains(t, rr.Content, "line 6")
}

func TestReadFileDefaultTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "big.txt")
	var lines []string
	for i := 1; i <= 1500; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644))

	rt := NewReadFileTool()
	result, err := rt.Execute(context.Background(), makeFileArgs(path))
	require.NoError(t, err)

	var rr ReadResult
	require.NoError(t, json.Unmarshal([]byte(result), &rr))
	assert.Equal(t, 1500, rr.TotalLines)
	assert.Equal(t, 1000, rr.LinesShown)
	assert.True(t, rr.Truncated)
}

func TestReadFileBinary(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "binary.dat")
	data := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x00, 0x57, 0x6f, 0x72, 0x6c, 0x64}
	require.NoError(t, os.WriteFile(path, data, 0644))

	rt := NewReadFileTool()
	result, err := rt.Execute(context.Background(), makeFileArgs(path))
	require.NoError(t, err)

	var resp map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Contains(t, resp["error"], "binary file detected")
}

func TestReadFileDeniedPath(t *testing.T) {
	rt := NewReadFileTool()
	result, err := rt.Execute(context.Background(), makeFileArgs("/etc/shadow"))
	require.NoError(t, err)

	var resp map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Contains(t, resp["error"], "access denied")
}

func TestReadFileNonExistent(t *testing.T) {
	rt := NewReadFileTool()
	_, err := rt.Execute(context.Background(), makeFileArgs("/tmp/nonexistent_fenec_test_abc123.txt"))
	assert.Error(t, err)
}

func TestReadFileMissingPath(t *testing.T) {
	rt := NewReadFileTool()
	args := api.NewToolCallFunctionArguments()
	_, err := rt.Execute(context.Background(), args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument")
}

func TestReadFileToolName(t *testing.T) {
	rt := NewReadFileTool()
	assert.Equal(t, "read_file", rt.Name())
}

func TestReadFileToolDefinition(t *testing.T) {
	rt := NewReadFileTool()
	def := rt.Definition()
	assert.Equal(t, "function", def.Type)
	assert.Equal(t, "read_file", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "path")
}
