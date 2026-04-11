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
