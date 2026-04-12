package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marad/fenec/internal/model"

	feneclua "github.com/marad/fenec/internal/lua"
)

// CreateLuaTool is a built-in tool that allows the agent to create new Lua tools.
// It validates source code (syntax + schema), writes the tool file to disk,
// and registers it in the tool registry.
type CreateLuaTool struct {
	toolsDir string
	registry *Registry
	notifier ToolEventNotifier
}

// NewCreateLuaTool creates a CreateLuaTool that persists Lua tools to toolsDir.
func NewCreateLuaTool(toolsDir string, registry *Registry, notifier ToolEventNotifier) *CreateLuaTool {
	return &CreateLuaTool{
		toolsDir: toolsDir,
		registry: registry,
		notifier: notifier,
	}
}

// Name returns the tool identifier used for dispatch.
func (c *CreateLuaTool) Name() string {
	return "create_lua_tool"
}

// Definition returns the tool definition for ChatRequest.Tools.
func (c *CreateLuaTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunction{
			Name:        "create_lua_tool",
			Description: "Create a new Lua tool by providing its source code. The tool is validated, saved to disk, and registered for immediate use.",
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

// Execute validates and persists a new Lua tool from the provided source code.
// Validation errors are returned as tool result strings (not Go errors) so the
// model can see and self-correct.
func (c *CreateLuaTool) Execute(_ context.Context, args map[string]any) (string, error) {
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

	// Syntax check via CompileFile.
	proto, err := feneclua.CompileFile(tmpPath)
	if err != nil {
		return errorJSON("syntax error: %s", err), nil
	}

	// Schema validation via NewLuaToolFromProto.
	lt, err := feneclua.NewLuaToolFromProto(proto, tmpPath)
	if err != nil {
		return errorJSON("validation error: %s", err), nil
	}

	// Check for name collision with existing tool.
	if c.registry.IsBuiltIn(lt.Name()) {
		return errorJSON("cannot create tool '%s': name conflicts with a built-in tool", lt.Name()), nil
	}
	if c.registry.Has(lt.Name()) {
		return errorJSON("tool '%s' already exists. Use update_lua_tool to replace it.", lt.Name()), nil
	}

	// Ensure tools directory exists.
	if err := os.MkdirAll(c.toolsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tools directory: %w", err)
	}

	// Write to final location.
	finalPath := filepath.Join(c.toolsDir, lt.Name()+".lua")
	if err := os.WriteFile(finalPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write tool file: %w", err)
	}

	// Re-compile from final path so scriptPath is correct on the LuaTool.
	proto, err = feneclua.CompileFile(finalPath)
	if err != nil {
		os.Remove(finalPath)
		return "", fmt.Errorf("failed to re-compile tool: %w", err)
	}
	lt, err = feneclua.NewLuaToolFromProto(proto, finalPath)
	if err != nil {
		os.Remove(finalPath)
		return "", fmt.Errorf("failed to re-validate tool: %w", err)
	}

	// Register in the registry.
	c.registry.RegisterLua(lt)

	// Notify listener.
	if c.notifier != nil {
		def := lt.Definition()
		c.notifier("created", lt.Name(), def.Function.Description)
	}

	// Return success JSON.
	return successJSON("created", lt), nil
}

// errorJSON returns a JSON error string for tool result display.
func errorJSON(format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}

// successJSON returns a JSON success response with tool metadata.
func successJSON(status string, lt Tool) string {
	def := lt.Definition()
	var paramNames []string
	for name := range def.Function.Parameters.Properties {
		paramNames = append(paramNames, name)
	}

	resp := map[string]interface{}{
		"status":      status,
		"name":        def.Function.Name,
		"description": def.Function.Description,
		"parameters":  paramNames,
	}
	b, _ := json.Marshal(resp)
	return string(b)
}
