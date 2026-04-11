package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/ollama/ollama/api"
)

// Tool defines the interface that all tools (built-in and Lua) must implement.
// This is the extension point for Phase 4 Lua tools.
type Tool interface {
	// Name returns the unique identifier used for dispatch.
	Name() string

	// Definition returns the Ollama API tool definition for ChatRequest.Tools.
	Definition() api.Tool

	// Execute runs the tool with the given arguments and returns the result string.
	Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error)
}

// Registry holds registered tools and dispatches tool calls by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry, keyed by its Name().
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Tools returns the Ollama API tool definitions for all registered tools.
// The returned slice is suitable for passing to ChatRequest.Tools.
func (r *Registry) Tools() api.Tools {
	if len(r.tools) == 0 {
		return nil
	}
	defs := make(api.Tools, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Dispatch looks up a tool by the call's function name and executes it.
// Returns an error if the tool is not found.
func (r *Registry) Dispatch(ctx context.Context, call api.ToolCall) (string, error) {
	t, ok := r.tools[call.Function.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Function.Name)
	}
	return t.Execute(ctx, call.Function.Arguments)
}

// Describe returns a multi-line string listing all tools with name and description.
// Format: "- tool_name: description". Used for system prompt injection.
func (r *Registry) Describe() string {
	if len(r.tools) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range r.tools {
		def := t.Definition()
		fmt.Fprintf(&b, "- %s: %s\n", def.Function.Name, def.Function.Description)
	}
	return b.String()
}
