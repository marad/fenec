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

func TestDeleteLuaToolName(t *testing.T) {
	d := NewDeleteLuaTool(t.TempDir(), NewRegistry(), nil)
	assert.Equal(t, "delete_lua_tool", d.Name())
}

func TestDeleteLuaToolDefinition(t *testing.T) {
	d := NewDeleteLuaTool(t.TempDir(), NewRegistry(), nil)
	def := d.Definition()
	assert.Equal(t, "delete_lua_tool", def.Function.Name)
	assert.Contains(t, def.Function.Parameters.Required, "name")
}

func TestDeleteLuaToolSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()

	// Create tool first
	c := NewCreateLuaTool(tmpDir, reg, nil)
	createArgs := makeToolArgs(map[string]interface{}{"code": validToolSource})
	_, err := c.Execute(context.Background(), createArgs)
	require.NoError(t, err)
	require.True(t, reg.Has("test_tool"))

	// Verify file exists
	toolPath := filepath.Join(tmpDir, "test_tool.lua")
	_, err = os.Stat(toolPath)
	require.NoError(t, err)

	// Delete it
	d := NewDeleteLuaTool(tmpDir, reg, nil)
	args := makeToolArgs(map[string]interface{}{"name": "test_tool"})
	result, err := d.Execute(context.Background(), args)
	require.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Equal(t, "deleted", resp["status"])
	assert.Equal(t, "test_tool", resp["name"])

	// Tool should be unregistered
	assert.False(t, reg.Has("test_tool"))

	// File should be gone
	_, err = os.Stat(toolPath)
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteLuaToolNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	d := NewDeleteLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{"name": "nonexistent"})
	result, err := d.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "not found")
}

func TestDeleteLuaToolBuiltInRejection(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "shell_exec", desc: "Built-in shell"})

	d := NewDeleteLuaTool(tmpDir, reg, nil)
	args := makeToolArgs(map[string]interface{}{"name": "shell_exec"})
	result, err := d.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
	assert.Contains(t, result, "built-in")
}

func TestDeleteLuaToolMissingName(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()
	d := NewDeleteLuaTool(tmpDir, reg, nil)

	args := makeToolArgs(map[string]interface{}{})
	_, err := d.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestDeleteLuaToolNotifier(t *testing.T) {
	tmpDir := t.TempDir()
	reg := NewRegistry()

	// Create tool first
	c := NewCreateLuaTool(tmpDir, reg, nil)
	createArgs := makeToolArgs(map[string]interface{}{"code": validToolSource})
	_, err := c.Execute(context.Background(), createArgs)
	require.NoError(t, err)

	var gotEvent, gotName string
	notifier := func(event, name, desc string) {
		gotEvent = event
		gotName = name
	}

	d := NewDeleteLuaTool(tmpDir, reg, notifier)
	args := makeToolArgs(map[string]interface{}{"name": "test_tool"})
	_, err = d.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Equal(t, "deleted", gotEvent)
	assert.Equal(t, "test_tool", gotName)
}
