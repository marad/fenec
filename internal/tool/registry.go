package tool

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ollama/ollama/api"
)

// ToolEventNotifier is called when a tool is created, updated, or deleted.
// event is one of "created", "updated", "deleted".
type ToolEventNotifier func(event string, toolName string, description string)

// ToolInfoEntry describes a registered tool for listing/inspection.
type ToolInfoEntry struct {
	Name        string
	Description string
	BuiltIn     bool
}

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
// It tracks provenance (built-in vs Lua) for each tool.
type Registry struct {
	tools   map[string]Tool
	builtIn map[string]bool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:   make(map[string]Tool),
		builtIn: make(map[string]bool),
	}
}

// Register adds a built-in tool to the registry, keyed by its Name().
// Built-in tools are protected from deletion and overwrite by Lua tools.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
	r.builtIn[t.Name()] = true
}

// RegisterLua adds a Lua tool to the registry without marking it as built-in.
func (r *Registry) RegisterLua(t Tool) {
	r.tools[t.Name()] = t
}

// Unregister removes a tool from the registry by name.
// Returns true if the tool was found and removed, false if it did not exist.
func (r *Registry) Unregister(name string) bool {
	_, ok := r.tools[name]
	if ok {
		delete(r.tools, name)
		delete(r.builtIn, name)
	}
	return ok
}

// Has reports whether a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// IsBuiltIn reports whether the named tool was registered as a built-in (via Register).
func (r *Registry) IsBuiltIn(name string) bool {
	return r.builtIn[name]
}

// ToolInfo returns a sorted slice describing all registered tools.
func (r *Registry) ToolInfo() []ToolInfoEntry {
	entries := make([]ToolInfoEntry, 0, len(r.tools))
	for _, t := range r.tools {
		def := t.Definition()
		entries = append(entries, ToolInfoEntry{
			Name:        def.Function.Name,
			Description: def.Function.Description,
			BuiltIn:     r.builtIn[def.Function.Name],
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
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
