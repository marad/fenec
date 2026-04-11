package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ollama/ollama/api"
)

// DeleteLuaTool is a built-in tool that allows the agent to remove Lua tools
// from disk and the registry.
type DeleteLuaTool struct {
	toolsDir string
	registry *Registry
	notifier ToolEventNotifier
}

// NewDeleteLuaTool creates a DeleteLuaTool that manages Lua tools in toolsDir.
func NewDeleteLuaTool(toolsDir string, registry *Registry, notifier ToolEventNotifier) *DeleteLuaTool {
	return &DeleteLuaTool{
		toolsDir: toolsDir,
		registry: registry,
		notifier: notifier,
	}
}

// Name returns the tool identifier used for dispatch.
func (d *DeleteLuaTool) Name() string {
	return "delete_lua_tool"
}

// Definition returns the Ollama API tool definition for ChatRequest.Tools.
func (d *DeleteLuaTool) Definition() api.Tool {
	props := api.NewToolPropertiesMap()
	props.Set("name", api.ToolProperty{
		Type:        api.PropertyType{"string"},
		Description: "Name of the Lua tool to delete",
	})

	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "delete_lua_tool",
			Description: "Delete a Lua tool by name. Removes the tool file from disk and unregisters it.",
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Required:   []string{"name"},
				Properties: props,
			},
		},
	}
}

// Execute removes a Lua tool from disk and the registry.
// Returns error strings (not Go errors) for user-facing issues like
// not-found or built-in rejection.
func (d *DeleteLuaTool) Execute(_ context.Context, args api.ToolCallFunctionArguments) (string, error) {
	nameVal, ok := args.Get("name")
	if !ok {
		return "", fmt.Errorf("missing required argument: name")
	}
	name, ok := nameVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: name")
	}

	// Reject deletion of built-in tools.
	if d.registry.IsBuiltIn(name) {
		return errorJSON("cannot delete built-in tool '%s'", name), nil
	}

	// Check that the tool exists.
	if !d.registry.Has(name) {
		return errorJSON("tool '%s' not found", name), nil
	}

	// Remove file from disk. Log but don't fail if file already gone.
	toolPath := filepath.Join(d.toolsDir, name+".lua")
	if err := os.Remove(toolPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove tool file: %w", err)
	}

	// Unregister from registry.
	d.registry.Unregister(name)

	// Notify listener.
	if d.notifier != nil {
		d.notifier("deleted", name, "")
	}

	resp := map[string]string{
		"status": "deleted",
		"name":   name,
	}
	b, _ := json.Marshal(resp)
	return string(b), nil
}
