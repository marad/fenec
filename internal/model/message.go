package model

// Message is a single message in a chat sequence.
// Fields mirror the subset of ollama/api.Message that Fenec uses,
// with identical JSON tags for wire compatibility.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Thinking   string     `json:"thinking,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
