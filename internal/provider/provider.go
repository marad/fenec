package provider

import (
	"context"

	"github.com/marad/fenec/internal/model"
)

// ChatRequest contains the fields a provider needs to execute a chat request.
// Decoupled from Conversation — callers build this from whatever state they have.
type ChatRequest struct {
	Model         string
	Messages      []model.Message
	Tools         []model.ToolDefinition
	Think         bool
	ContextLength int // 0 means "not set, use model default"
}

// Provider is the abstraction layer for LLM backends. Consumers depend on this
// interface rather than any specific provider implementation.
type Provider interface {
	// Name returns the provider identifier (e.g., "ollama", "openai").
	Name() string

	// ListModels returns the names of all available models.
	ListModels(ctx context.Context) ([]string, error)

	// Ping verifies the provider is reachable and has models installed.
	Ping(ctx context.Context) error

	// StreamChat sends a chat request and streams the response.
	// onToken is called for each content chunk; onThinking for each reasoning chunk.
	StreamChat(ctx context.Context, req *ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error)

	// GetContextLength queries the model's context window size.
	GetContextLength(ctx context.Context, modelName string) (int, error)
}
