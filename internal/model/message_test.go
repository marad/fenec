package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageJSONRoundTrip(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "Hello, world!",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var got Message
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, msg, got)
}

func TestMessageJSONOmitsEmptyOptionalFields(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hi",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Should NOT contain thinking, tool_calls, or tool_call_id
	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "thinking")
	assert.NotContains(t, jsonStr, "tool_calls")
	assert.NotContains(t, jsonStr, "tool_call_id")
}

func TestMessageJSONWithToolCalls(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{
				Function: ToolCallFunction{
					Index:     0,
					Name:      "read_file",
					Arguments: map[string]any{"path": "/tmp/test.txt"},
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var got Message
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, "assistant", got.Role)
	require.Len(t, got.ToolCalls, 1)
	assert.Equal(t, "read_file", got.ToolCalls[0].Function.Name)
	assert.Equal(t, 0, got.ToolCalls[0].Function.Index)
	assert.Equal(t, "/tmp/test.txt", got.ToolCalls[0].Function.Arguments["path"])
}

func TestMessageJSONWithThinking(t *testing.T) {
	msg := Message{
		Role:     "assistant",
		Content:  "The answer is 42.",
		Thinking: "Let me think about this...",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"thinking"`)

	var got Message
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, "Let me think about this...", got.Thinking)
}

func TestMessageJSONWithToolCallID(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    `{"result": "ok"}`,
		ToolCallID: "call_123",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"tool_call_id"`)

	var got Message
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, "tool", got.Role)
	assert.Equal(t, "call_123", got.ToolCallID)
}
