package lua

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: Compile-time interface check (var _ tool.Tool = (*LuaTool)(nil)) was
// removed to break an import cycle: internal/tool now imports internal/lua for
// the self-extension tools (create/update/delete). The interface compliance is
// verified by the tool package tests and by RegisterLua accepting LuaTool.

func loadTestTool(t *testing.T, path string) *LuaTool {
	t.Helper()
	proto, err := CompileFile(path)
	require.NoError(t, err, "CompileFile(%s) should succeed", path)
	lt, err := NewLuaToolFromProto(proto, path)
	require.NoError(t, err, "NewLuaToolFromProto(%s) should succeed", path)
	return lt
}

func TestLuaToolName(t *testing.T) {
	lt := loadTestTool(t, "testdata/word_count.lua")
	assert.Equal(t, "word_count", lt.Name())
}

func TestLuaToolDefinition(t *testing.T) {
	lt := loadTestTool(t, "testdata/word_count.lua")
	def := lt.Definition()

	assert.Equal(t, "function", def.Type)
	assert.Equal(t, "word_count", def.Function.Name)
	assert.Contains(t, def.Function.Description, "Count words")
	assert.Equal(t, "object", def.Function.Parameters.Type)
	assert.Contains(t, def.Function.Parameters.Required, "text")
}

func TestLuaToolExecute(t *testing.T) {
	lt := loadTestTool(t, "testdata/word_count.lua")

	args := map[string]any{"text": "hello world foo"}

	result, err := lt.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Equal(t, "3", result)
}

func TestLuaToolExecuteEmptyArgs(t *testing.T) {
	lt := loadTestTool(t, "testdata/word_count.lua")

	args := map[string]any{}

	result, err := lt.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Equal(t, "0", result)
}

func TestLuaToolExecuteTimeout(t *testing.T) {
	// Create a Lua script that loops forever in execute
	infiniteScript := `return {
		name = "infinite",
		description = "loops forever",
		parameters = {},
		execute = function(args) while true do end return "done" end
	}`

	// Write it to a temp file
	tmpPath := t.TempDir() + "/infinite.lua"
	require.NoError(t, writeFile(tmpPath, infiniteScript))

	proto, err := CompileFile(tmpPath)
	require.NoError(t, err)
	lt, err := NewLuaToolFromProto(proto, tmpPath)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = lt.Execute(ctx, map[string]any{})
	assert.Error(t, err, "should error on context timeout")
}

func TestLuaToolMissingExecute(t *testing.T) {
	proto, err := CompileFile("testdata/no_execute.lua")
	require.NoError(t, err, "CompileFile should succeed for syntactically valid Lua")

	_, err = NewLuaToolFromProto(proto, "testdata/no_execute.lua")
	assert.Error(t, err, "should error on missing execute")
	assert.Contains(t, err.Error(), "execute")
}

func TestLuaToolMissingName(t *testing.T) {
	proto, err := CompileFile("testdata/no_name.lua")
	require.NoError(t, err, "CompileFile should succeed for syntactically valid Lua")

	_, err = NewLuaToolFromProto(proto, "testdata/no_name.lua")
	assert.Error(t, err, "should error on missing name")
	assert.Contains(t, err.Error(), "name")
}

func TestArgsToLuaTable(t *testing.T) {
	L := NewSandboxedState(context.Background())
	defer L.Close()

	args := map[string]any{
		"str_val":  "hello",
		"num_val":  float64(42),
		"bool_val": true,
	}

	tbl := ArgsToLuaTable(L, args)

	// Check string conversion
	strVal := L.GetField(tbl, "str_val")
	assert.Equal(t, "hello", strVal.String())

	// Check number conversion
	numVal := L.GetField(tbl, "num_val")
	assert.Equal(t, "42", numVal.String())

	// Check bool conversion
	boolVal := L.GetField(tbl, "bool_val")
	assert.Equal(t, "true", boolVal.String())
}

func TestLuaToolImplementsInterface(t *testing.T) {
	// Verify LuaTool has the methods expected by tool.Tool without importing
	// the tool package (which would create a cycle since tool imports lua).
	lt := loadTestTool(t, "testdata/word_count.lua")
	assert.NotEmpty(t, lt.Name())
	def := lt.Definition()
	assert.Equal(t, "function", def.Type)
	_, err := lt.Execute(context.Background(), map[string]any{})
	assert.NoError(t, err)
}

// writeFile is a test helper to write content to a file.
func writeFile(path, content string) error {
	return writeFileHelper(path, content)
}
