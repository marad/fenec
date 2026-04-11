package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ollama/ollama/api"
)

// ChatService is the interface that REPL and other consumers use.
// Decouples downstream code from the concrete Client implementation.
type ChatService interface {
	ListModels(ctx context.Context) ([]string, error)
	Ping(ctx context.Context) error
	StreamChat(ctx context.Context, conv *Conversation, onToken func(string)) (*api.Message, error)
}

// chatAPI is an internal interface for testing the chat logic.
// In production this is satisfied by *api.Client; in tests by a mock.
type chatAPI interface {
	Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error
	List(ctx context.Context) (*api.ListResponse, error)
}

// Client wraps the Ollama API client.
type Client struct {
	api chatAPI
}

// NewClient creates a client connecting to the given host.
// If host is empty, falls back to OLLAMA_HOST env var, then localhost:11434.
func NewClient(host string) (*Client, error) {
	var ollamaClient *api.Client
	var err error

	if host != "" {
		u, parseErr := url.Parse(host)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid host URL %q: %w", host, parseErr)
		}
		ollamaClient = api.NewClient(u, http.DefaultClient)
	} else {
		ollamaClient, err = api.ClientFromEnvironment()
		if err != nil {
			return nil, fmt.Errorf("creating Ollama client from environment: %w", err)
		}
	}

	return &Client{api: ollamaClient}, nil
}

// newClientWithAPI creates a Client with a custom chatAPI (for testing).
func newClientWithAPI(api chatAPI) *Client {
	return &Client{api: api}
}

// ListModels returns the names of all available models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	resp, err := c.api.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	names := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// Ping verifies the Ollama server is reachable and has models installed.
// Returns a descriptive error if the server is unreachable or has no models.
func (c *Client) Ping(ctx context.Context) error {
	models, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("cannot connect to Ollama: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("no models installed — pull one with: ollama pull gemma4")
	}
	return nil
}
