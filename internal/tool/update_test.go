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

const updatedToolSource = `return {
    name = "test_tool",
    description = "An updated test tool",
    parameters = {
        { name = "input", type = "string", description = "Updated input", required = true }
    },
    execute = function(args)
        return "updated: " .. (args.input or "")
    end
}`

func TestUpdateLuaToolName(t *testing.T) {
	u := NewUpdateLuaTool(t.TempDir(), NewRegistry(), nil)
	assert.Equal(t, "update_lua_tool", u.Name())
}

func TestUpdateLuaToolDefinition(t *testing.T) {
	u := NewUpdateLuaTool(t.TempDir(), NewRegistry(), nil)
	def := u.Definition()
	assert.Equal(t, "update_lua_tool", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "code")
}

func TestUpdateLuaToolSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()

	// First create the tool
	c := NewCreateLuaTool(tmpDir, reg, nil)
	createArgs := makeToolArgs(map[string]interface{}{"code": validToolSource})
	_, err := c.Execute(context.Background(), createArgs)
	require.NoError(t, err)
	require.True(t, reg.Has("test_tool"))

	// Now update it
	u := NewUpdateLuaTool(tmpDir, reg, nil)
	updateArgs := makeToolArgs(map[string]interface{}{"code": updatedToolSource})
	result, err := u.Execute(context.Background(), updateArgs)
	require.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Equal(t, "updated", resp["status"])
	assert.Equal(t, "test_tool", resp["name"])
	assert.Equal(t, "An updated test tool", resp["description"])

	// Tool should still be registered (not built-in)
	assert.True(t, reg.Has("test_tool"))
	assert.False(t, reg.IsBuiltIn("test_tool"))

	// File should contain updated content
	content, err := os.ReadFile(filepath.Join(tmpDir, "test_tool.lua"))
	require.NoError(t, err)
	assert.Equal(t, updatedToolSource, string(content))
}

func TestUpdateLuaToolNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	u := NewUpdateLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{"code": validToolSource})
	result, err := u.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "does not exist")
	assert.Contains(t, result, "create_lua_tool")
}

func TestUpdateLuaToolValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()

	// First create a valid tool
	c := NewCreateLuaTool(tmpDir, reg, nil)
	createArgs := makeToolArgs(map[string]interface{}{"code": validToolSource})
	_, err := c.Execute(context.Background(), createArgs)
	require.NoError(t, err)

	// Try to update with invalid source -- register with name "test_tool" so we
	// need a source that parses but has the same name... Actually, the syntax error
	// won't produce a name. Let's try a source that has correct name but missing execute.
	invalidUpdateSource := `return { name = "test_tool", description = "Missing execute" }`

	u := NewUpdateLuaTool(tmpDir, reg, nil)
	updateArgs := makeToolArgs(map[string]interface{}{"code": invalidUpdateSource})
	result, err := u.Execute(context.Background(), updateArgs)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "execute")

	// Original tool should still be intact
	assert.True(t, reg.Has("test_tool"))

	// Original file should be unchanged
	content, err := os.ReadFile(filepath.Join(tmpDir, "test_tool.lua"))
	require.NoError(t, err)
	assert.Equal(t, validToolSource, string(content))
}

func TestUpdateLuaToolBuiltInRejection(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "shell_exec", desc: "Built-in"})

	shellSource := `return {
    name = "shell_exec",
    description = "Trying to update built-in",
    execute = function(args) return "hacked" end
}`

	u := NewUpdateLuaTool(tmpDir, reg, nil)
	args := makeToolArgs(map[string]interface{}{"code": shellSource})
	result, err := u.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "built-in")
}

func TestUpdateLuaToolNotifier(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()

	// Create tool first
	c := NewCreateLuaTool(tmpDir, reg, nil)
	createArgs := makeToolArgs(map[string]interface{}{"code": validToolSource})
	_, err := c.Execute(context.Background(), createArgs)
	require.NoError(t, err)

	var gotEvent, gotName, gotDesc string
	notifier := func(event, name, desc string) {
		gotEvent = event
		gotName = name
		gotDesc = desc
	}

	u := NewUpdateLuaTool(tmpDir, reg, notifier)
	updateArgs := makeToolArgs(map[string]interface{}{"code": updatedToolSource})
	_, err = u.Execute(context.Background(), updateArgs)
	require.NoError(t, err)

	assert.Equal(t, "updated", gotEvent)
	assert.Equal(t, "test_tool", gotName)
	assert.Equal(t, "An updated test tool", gotDesc)
}
