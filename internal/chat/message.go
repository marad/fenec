package chat

import "github.com/ollama/ollama/api"

// Conversation holds the message history for a chat session.
type Conversation struct {
	Messages []api.Message
	Model    string
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

// SetModel changes the active model. Conversation history is preserved (per D-11).
func (c *Conversation) SetModel(model string) {
	c.Model = model
}
