package lua

import (
	"context"
	"testing"
	"time"

	"github.com/marad/fenec/internal/tool"
	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface compliance check.
var _ tool.Tool = (*LuaTool)(nil)

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

	args := api.NewToolCallFunctionArguments()
	args.Set("text", "hello world foo")

	result, err := lt.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Equal(t, "3", result)
}

func TestLuaToolExecuteEmptyArgs(t *testing.T) {
	lt := loadTestTool(t, "testdata/word_count.lua")

	args := api.NewToolCallFunctionArguments()

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

	_, err = lt.Execute(ctx, api.NewToolCallFunctionArguments())
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

	args := api.NewToolCallFunctionArguments()
	args.Set("str_val", "hello")
	args.Set("num_val", float64(42))
	args.Set("bool_val", true)

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
	// This is a compile-time check via the var _ declaration above.
	// If LuaTool does not implement tool.Tool, the file won't compile.
	t.Log("LuaTool implements tool.Tool interface (compile-time check)")
}

// writeFile is a test helper to write content to a file.
func writeFile(path, content string) error {
	return writeFileHelper(path, content)
}
