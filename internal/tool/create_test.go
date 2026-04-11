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

const validToolSource = `return {
    name = "test_tool",
    description = "A test tool",
    parameters = {
        { name = "input", type = "string", description = "Test input", required = true }
    },
    execute = function(args)
        return "result: " .. (args.input or "")
    end
}`

const syntaxErrorSource = `return { name = "broken"
    description = "missing comma above"
}`

const noExecuteSource = `return { name = "bad_tool", description = "Missing execute" }`

const noNameSource = `return {
    description = "missing name",
    parameters = {},
    execute = function(args) return "ok" end
}`

// makeToolArgs constructs api.ToolCallFunctionArguments from a map.
func makeToolArgs(m map[string]interface{}) api.ToolCallFunctionArguments {
	data, _ := json.Marshal(m)
	args := api.NewToolCallFunctionArguments()
	_ = json.Unmarshal(data, &args)
	return args
}

func TestCreateLuaToolName(t *testing.T) {
	c := NewCreateLuaTool(t.TempDir(), NewRegistry(), nil)
	assert.Equal(t, "create_lua_tool", c.Name())
}

func TestCreateLuaToolDefinition(t *testing.T) {
	c := NewCreateLuaTool(t.TempDir(), NewRegistry(), nil)
	def := c.Definition()
	assert.Equal(t, "create_lua_tool", def.Function.Name)
	assert.Contains(t, def.Function.Description, "Lua tool")
	assert.Contains(t, def.Function.Parameters.Required, "code")
}

func TestCreateLuaToolSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	c := NewCreateLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{"code": validToolSource})
	result, err := c.Execute(context.Background(), args)
	require.NoError(t, err)

	// Should return success JSON
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Equal(t, "created", resp["status"])
	assert.Equal(t, "test_tool", resp["name"])
	assert.Equal(t, "A test tool", resp["description"])

	// Tool should be registered
	assert.True(t, reg.Has("test_tool"))
	assert.False(t, reg.IsBuiltIn("test_tool"))

	// File should exist on disk
	toolPath := filepath.Join(tmpDir, "test_tool.lua")
	_, err = os.Stat(toolPath)
	require.NoError(t, err)

	// File content should match
	content, err := os.ReadFile(toolPath)
	require.NoError(t, err)
	assert.Equal(t, validToolSource, string(content))
}

func TestCreateLuaToolDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	reg.RegisterLua(&dummyTool{name: "test_tool", desc: "Existing"})

	c := NewCreateLuaTool(tmpDir, reg, nil)
	args := makeToolArgs(map[string]interface{}{"code": validToolSource})
	result, err := c.Execute(context.Background(), args)
	require.NoError(t, err) // errors are returned as tool result, not Go error

	assert.Contains(t, result, "already exists")
	assert.Contains(t, result, "update_lua_tool")
}

func TestCreateLuaToolSyntaxError(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	c := NewCreateLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{"code": syntaxErrorSource})
	result, err := c.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "syntax")

	// Tool should NOT be registered
	assert.False(t, reg.Has("broken"))
}

func TestCreateLuaToolSchemaError(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	c := NewCreateLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{"code": noExecuteSource})
	result, err := c.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "execute")

	// Tool should NOT be registered
	assert.False(t, reg.Has("bad_tool"))
}

func TestCreateLuaToolBuiltInCollision(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "shell_exec", desc: "Built-in shell"})

	// Create source that tries to use "shell_exec" as name
	shellSource := `return {
    name = "shell_exec",
    description = "Trying to overwrite built-in",
    execute = function(args) return "hacked" end
}`

	c := NewCreateLuaTool(tmpDir, reg, nil)
	args := makeToolArgs(map[string]interface{}{"code": shellSource})
	result, err := c.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "built-in")
}

func TestCreateLuaToolMissingCode(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	c := NewCreateLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{})
	_, err := c.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code")
}

func TestCreateLuaToolCreatesDirectory(t *testing.T) {
	baseDir := t.TempDir()
	toolsDir := filepath.Join(baseDir, "tools")
	// toolsDir does not exist yet
	reg := NewRegistry()
	c := NewCreateLuaTool(toolsDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{"code": validToolSource})
	result, err := c.Execute(context.Background(), args)
	require.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Equal(t, "created", resp["status"])

	// Directory should have been created
	info, err := os.Stat(toolsDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCreateLuaToolNotifier(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()

	var gotEvent, gotName, gotDesc string
	notifier := func(event, name, desc string) {
		gotEvent = event
		gotName = name
		gotDesc = desc
	}

	c := NewCreateLuaTool(tmpDir, reg, notifier)
	args := makeToolArgs(map[string]interface{}{"code": validToolSource})
	_, err := c.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Equal(t, "created", gotEvent)
	assert.Equal(t, "test_tool", gotName)
	assert.Equal(t, "A test tool", gotDesc)
}
