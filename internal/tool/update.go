package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marad/fenec/internal/model"

	feneclua "github.com/marad/fenec/internal/lua"
)

// UpdateLuaTool is a built-in tool that allows the agent to replace an existing Lua tool
// with a new version. It validates the new source, then atomically replaces the tool
// on disk and in the registry.
type UpdateLuaTool struct {
	toolsDir string
	registry *Registry
	notifier ToolEventNotifier
}

// NewUpdateLuaTool creates an UpdateLuaTool that manages Lua tools in toolsDir.
func NewUpdateLuaTool(toolsDir string, registry *Registry, notifier ToolEventNotifier) *UpdateLuaTool {
	return &UpdateLuaTool{
		toolsDir: toolsDir,
		registry: registry,
		notifier: notifier,
	}
}

// Name returns the tool identifier used for dispatch.
func (u *UpdateLuaTool) Name() string {
	return "update_lua_tool"
}

// Definition returns the tool definition for ChatRequest.Tools.
func (u *UpdateLuaTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunction{
			Name:        "update_lua_tool",
			Description: "Update an existing Lua tool by providing its complete new source code. The tool is validated before replacement.",
			Parameters: model.ToolFunctionParameters{
				Type:     "object",
				Required: []string{"code"},
				Properties: map[string]model.ToolProperty{
					"code": {
						Type:        model.PropertyType{"string"},
						Description: "Complete Lua tool source code. Must return a table with name, description, and execute fields.",
					},
				},
			},
		},
	}
}

// Execute validates and replaces an existing Lua tool with new source code.
// Validation errors are returned as tool result strings so the model can self-correct.
// If validation fails, the existing tool is not modified.
func (u *UpdateLuaTool) Execute(_ context.Context, args map[string]any) (string, error) {
	codeVal, ok := args["code"]
	if !ok {
		return "", fmt.Errorf("missing required argument: code")
	}
	code, ok := codeVal.(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: code")
	}

	// Write to temp file for compilation.
	tmpFile, err := os.CreateTemp("", "fenec-tool-*.lua")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Syntax check.
	proto, err := feneclua.CompileFile(tmpPath)
	if err != nil {
		return errorJSON("syntax error: %s", err), nil
	}

	// Schema validation.
	lt, err := feneclua.NewLuaToolFromProto(proto, tmpPath)
	if err != nil {
		return errorJSON("validation error: %s", err), nil
	}

	// Check that the tool exists and is not built-in.
	if u.registry.IsBuiltIn(lt.Name()) {
		return errorJSON("cannot update tool '%s': it is a built-in tool", lt.Name()), nil
	}
	if !u.registry.Has(lt.Name()) {
		return errorJSON("tool '%s' does not exist. Use create_lua_tool to create it.", lt.Name()), nil
	}

	// Write validated code to final path (overwriting existing file).
	finalPath := filepath.Join(u.toolsDir, lt.Name()+".lua")
	if err := os.WriteFile(finalPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write tool file: %w", err)
	}

	// Re-compile from final path so scriptPath is correct.
	proto, err = feneclua.CompileFile(finalPath)
	if err != nil {
		return "", fmt.Errorf("failed to re-compile tool: %w", err)
	}
	lt, err = feneclua.NewLuaToolFromProto(proto, finalPath)
	if err != nil {
		return "", fmt.Errorf("failed to re-validate tool: %w", err)
	}

	// Unregister old, register new (ensures fresh proto).
	u.registry.Unregister(lt.Name())
	u.registry.RegisterLua(lt)

	// Notify listener.
	if u.notifier != nil {
		def := lt.Definition()
		u.notifier("updated", lt.Name(), def.Function.Description)
	}

	return successJSON("updated", lt), nil
}
