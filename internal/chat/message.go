package chat

import "github.com/ollama/ollama/api"

// Conversation holds the message history for a chat session.
type Conversation struct {
	Messages      []api.Message
	Model         string
	ContextLength int  // Maximum context window size in tokens (0 = not set)
	Think         bool // Enable model thinking/reasoning output
}

// NewConversation creates a conversation with a system prompt.
func NewConversation(model string, systemPrompt string) *Conversation {
	conv := &Conversation{
		Model: model,
	}
	if systemPrompt != "" {
		conv.Messages = append(conv.Messages, api.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	return conv
}

// AddUser appends a user message.
func (c *Conversation) AddUser(content string) {
	c.Messages = append(c.Messages, api.Message{
		Role:    "user",
		Content: content,
	})
}

// AddAssistant appends an assistant message.
func (c *Conversation) AddAssistant(content string) {
	c.Messages = append(c.Messages, api.Message{
		Role:    "assistant",
		Content: content,
	})
}

// AddRawMessage appends an arbitrary message to the conversation.
// Used for assistant messages containing tool calls and tool result messages.
func (c *Conversation) AddRawMessage(msg api.Message) {
	c.Messages = append(c.Messages, msg)
}

// AddToolResult appends a tool result message to the conversation.
func (c *Conversation) AddToolResult(toolCallID string, content string) {
	c.Messages = append(c.Messages, api.Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	})
}

// SetModel changes the active model. Conversation history is preserved (per D-11).
func (c *Conversation) SetModel(model string) {
	c.Model = model
}
