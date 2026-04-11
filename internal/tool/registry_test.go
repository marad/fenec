package tool

import (
	"context"
	"fmt"
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dummyTool implements Tool for testing.
type dummyTool struct {
	name   string
	desc   string
	result string
	err    error
}

func (d *dummyTool) Name() string { return d.name }

func (d *dummyTool) Definition() api.Tool {
	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        d.name,
			Description: d.desc,
			Parameters:  api.ToolFunctionParameters{Type: "object"},
		},
	}
}

func (d *dummyTool) Execute(_ context.Context, _ api.ToolCallFunctionArguments) (string, error) {
	return d.result, d.err
}

func TestRegistryRegisterAndTools(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "tool_a", desc: "Tool A"})
	reg.Register(&dummyTool{name: "tool_b", desc: "Tool B"})

	tools := reg.Tools()
	assert.Len(t, tools, 2)

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}
	assert.True(t, names["tool_a"])
	assert.True(t, names["tool_b"])
}

func TestRegistryDispatchSuccess(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "echo", desc: "Echo tool", result: "ok"})

	args := api.NewToolCallFunctionArguments()
	call := api.ToolCall{
		Function: api.ToolCallFunction{
			Name:      "echo",
			Arguments: args,
		},
	}

	result, err := reg.Dispatch(context.Background(), call)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestRegistryDispatchUnknownTool(t *testing.T) {
	reg := NewRegistry()

	args := api.NewToolCallFunctionArguments()
	call := api.ToolCall{
		Function: api.ToolCallFunction{
			Name:      "nonexistent",
			Arguments: args,
		},
	}

	_, err := reg.Dispatch(context.Background(), call)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool: nonexistent")
}

func TestRegistryDispatchError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "fail", desc: "Failing tool", err: fmt.Errorf("tool error")})

	args := api.NewToolCallFunctionArguments()
	call := api.ToolCall{
		Function: api.ToolCallFunction{
			Name:      "fail",
			Arguments: args,
		},
	}

	_, err := reg.Dispatch(context.Background(), call)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool error")
}

func TestRegistryDescribe(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "tool_a", desc: "Does A things"})
	reg.Register(&dummyTool{name: "tool_b", desc: "Does B things"})

	desc := reg.Describe()
	assert.Contains(t, desc, "tool_a")
	assert.Contains(t, desc, "Does A things")
	assert.Contains(t, desc, "tool_b")
	assert.Contains(t, desc, "Does B things")
}

func TestRegistryToolsEmpty(t *testing.T) {
	reg := NewRegistry()
	tools := reg.Tools()
	assert.Empty(t, tools)
}

func TestRegistryUnregister(t *testing.T) {
	t.Run("existing tool returns true and removes it", func(t *testing.T) {
		reg := NewRegistry()
		reg.Register(&dummyTool{name: "tool_a", desc: "Tool A"})

		ok := reg.Unregister("tool_a")
		assert.True(t, ok)

		// Should no longer be in Tools()
		tools := reg.Tools()
		assert.Empty(t, tools)

		// Should no longer dispatch
		args := api.NewToolCallFunctionArguments()
		call := api.ToolCall{Function: api.ToolCallFunction{Name: "tool_a", Arguments: args}}
		_, err := reg.Dispatch(context.Background(), call)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("nonexistent tool returns false without panic", func(t *testing.T) {
		reg := NewRegistry()
		ok := reg.Unregister("nonexistent")
		assert.False(t, ok)
	})
}

func TestRegistryHas(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "tool_a", desc: "Tool A"})

	assert.True(t, reg.Has("tool_a"))
	assert.False(t, reg.Has("missing"))
}

func TestRegistryIsBuiltIn(t *testing.T) {
	reg := NewRegistry()

	// Register marks as built-in
	reg.Register(&dummyTool{name: "shell_exec", desc: "Shell"})
	assert.True(t, reg.IsBuiltIn("shell_exec"))

	// RegisterLua does not mark as built-in
	reg.RegisterLua(&dummyTool{name: "word_count", desc: "Word count"})
	assert.False(t, reg.IsBuiltIn("word_count"))

	// Unknown tool is not built-in
	assert.False(t, reg.IsBuiltIn("nonexistent"))
}

func TestRegistryToolInfo(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "shell_exec", desc: "Execute shell commands"})
	reg.RegisterLua(&dummyTool{name: "word_count", desc: "Count words"})

	info := reg.ToolInfo()
	require.Len(t, info, 2)

	// Should be sorted by name
	assert.Equal(t, "shell_exec", info[0].Name)
	assert.Equal(t, "Execute shell commands", info[0].Description)
	assert.True(t, info[0].BuiltIn)

	assert.Equal(t, "word_count", info[1].Name)
	assert.Equal(t, "Count words", info[1].Description)
	assert.False(t, info[1].BuiltIn)
}

func TestRegistryUnregisterClearsBuiltIn(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyTool{name: "tool_a", desc: "Tool A"})
	assert.True(t, reg.IsBuiltIn("tool_a"))

	reg.Unregister("tool_a")
	assert.False(t, reg.IsBuiltIn("tool_a"))
	assert.False(t, reg.Has("tool_a"))
}
