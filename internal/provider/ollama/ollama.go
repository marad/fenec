package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	"github.com/ollama/ollama/api"
)

// Compile-time check: Provider satisfies provider.Provider.
var _ provider.Provider = (*Provider)(nil)

// chatAPI is an internal interface for testing the chat logic.
// In production this is satisfied by *api.Client; in tests by a mock.
type chatAPI interface {
	Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error
	List(ctx context.Context) (*api.ListResponse, error)
	Show(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error)
}

// Provider wraps the Ollama API client, implementing the provider.Provider interface.
type Provider struct {
	api chatAPI
}

// New creates a Provider connecting to the given host.
// If host is empty, falls back to OLLAMA_HOST env var, then localhost:11434.
func New(host string) (*Provider, error) {
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

	return &Provider{api: ollamaClient}, nil
}

// newWithAPI creates a Provider with a custom chatAPI (for testing).
func newWithAPI(api chatAPI) *Provider {
	return &Provider{api: api}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "ollama"
}

// ListModels returns the names of all available models.
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	resp, err := p.api.List(ctx)
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
func (p *Provider) Ping(ctx context.Context) error {
	models, err := p.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("cannot connect to Ollama: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("no models installed — pull one with: ollama pull gemma4")
	}
	return nil
}

// GetContextLength queries the model's context window size via the Show API.
// Returns a conservative fallback of 4096 if the Show API fails or the
// context_length key is not found in model_info.
func (p *Provider) GetContextLength(ctx context.Context, modelName string) (int, error) {
	resp, err := p.api.Show(ctx, &api.ShowRequest{Model: modelName})
	if err != nil {
		return 4096, nil // Conservative fallback
	}
	for key, val := range resp.ModelInfo {
		if strings.HasSuffix(key, ".context_length") {
			switch v := val.(type) {
			case float64:
				return int(v), nil
			case int:
				return v, nil
			}
		}
	}
	return 4096, nil // Fallback if key not found
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

// StreamChat sends the chat request to Ollama and streams the response.
// onToken is called for each content chunk as it arrives.
// The full assistant message and stream metrics are returned after streaming completes.
// Context cancellation stops the stream.
func (p *Provider) StreamChat(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
	var content strings.Builder
	var thinking strings.Builder
	var metrics api.Metrics
	var finalMsg api.Message

	ollamaReq := &api.ChatRequest{
		Model:    req.Model,
		Messages: toOllamaMessages(req.Messages),
		Tools:    toOllamaTools(req.Tools),
		Truncate: boolPtr(false),
	}

	// Explicitly set thinking — nil means "model default" which is on for
	// thinking-capable models like Gemma 4.
	ollamaReq.Think = &api.ThinkValue{Value: req.Think}

	// Set num_ctx if request has a known context length.
	if req.ContextLength > 0 {
		ollamaReq.Options = map[string]any{"num_ctx": req.ContextLength}
	}

	var toolCalls []api.ToolCall
	err := p.api.Chat(ctx, ollamaReq, func(resp api.ChatResponse) error {
		if resp.Message.Content != "" {
			content.WriteString(resp.Message.Content)
			if onToken != nil {
				onToken(resp.Message.Content)
			}
		}
		// Accumulate thinking content from streaming chunks.
		if resp.Message.Thinking != "" {
			thinking.WriteString(resp.Message.Thinking)
			if onThinking != nil {
				onThinking(resp.Message.Thinking)
			}
		}
		// Accumulate tool calls from streaming chunks.
		if len(resp.Message.ToolCalls) > 0 {
			toolCalls = append(toolCalls, resp.Message.ToolCalls...)
		}
		// Capture metrics from the final chunk.
		if resp.Done {
			metrics = resp.Metrics
			finalMsg = resp.Message
			finalMsg.Content = content.String()
			finalMsg.Thinking = thinking.String()
			finalMsg.Role = "assistant"
		}
		// Stop early if context is cancelled.
		return ctx.Err()
	})
	// Ensure accumulated tool calls are on the final message regardless of
	// which chunk originally carried them.
	if len(toolCalls) > 0 {
		finalMsg.ToolCalls = toolCalls
	}
	if err != nil {
		// If the error is due to context cancellation, return the partial content
		// along with the error so the caller can distinguish cancellation from failure.
		if ctx.Err() != nil {
			cancelMsg := model.Message{Content: content.String(), Role: "assistant"}
			cancelMetrics := fromOllamaMetrics(metrics)
			return &cancelMsg, &cancelMetrics, ctx.Err()
		}
		m := fromOllamaMetrics(metrics)
		return nil, &m, err
	}

	// If no Done chunk was received (e.g. stream ended without it),
	// ensure content and role are still set from the accumulated builder.
	if finalMsg.Role == "" {
		finalMsg.Role = "assistant"
		finalMsg.Content = content.String()
	}

	canonicalMsg := fromOllamaMessage(finalMsg)
	canonicalMetrics := fromOllamaMetrics(metrics)
	return &canonicalMsg, &canonicalMetrics, nil
}

// --- Ollama conversion functions (adapter boundary) ---

// toOllamaMessages converts canonical messages to Ollama API messages.
func toOllamaMessages(msgs []model.Message) []api.Message {
	out := make([]api.Message, len(msgs))
	for i, m := range msgs {
		out[i] = api.Message{
			Role:       m.Role,
			Content:    m.Content,
			Thinking:   m.Thinking,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			args := api.NewToolCallFunctionArguments()
			for k, v := range tc.Function.Arguments {
				args.Set(k, v)
			}
			out[i].ToolCalls = append(out[i].ToolCalls, api.ToolCall{
				ID: tc.ID,
				Function: api.ToolCallFunction{
					Index:     tc.Function.Index,
					Name:      tc.Function.Name,
					Arguments: args,
				},
			})
		}
	}
	return out
}

// fromOllamaMessage converts an Ollama API message to a canonical message.
func fromOllamaMessage(msg api.Message) model.Message {
	m := model.Message{
		Role:       msg.Role,
		Content:    msg.Content,
		Thinking:   msg.Thinking,
		ToolCallID: msg.ToolCallID,
	}
	for _, tc := range msg.ToolCalls {
		m.ToolCalls = append(m.ToolCalls, model.ToolCall{
			ID: tc.ID,
			Function: model.ToolCallFunction{
				Index:     tc.Function.Index,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments.ToMap(),
			},
		})
	}
	return m
}

// toOllamaTools converts canonical tool definitions to Ollama API tools.
func toOllamaTools(tools []model.ToolDefinition) api.Tools {
	if tools == nil {
		return nil
	}
	out := make(api.Tools, len(tools))
	for i, td := range tools {
		tool := api.Tool{
			Type: td.Type,
			Function: api.ToolFunction{
				Name:        td.Function.Name,
				Description: td.Function.Description,
				Parameters: api.ToolFunctionParameters{
					Type:     td.Function.Parameters.Type,
					Required: td.Function.Parameters.Required,
				},
			},
		}
		if td.Function.Parameters.Properties != nil {
			props := api.NewToolPropertiesMap()
			for name, prop := range td.Function.Parameters.Properties {
				props.Set(name, api.ToolProperty{
					Type:        api.PropertyType(prop.Type),
					Description: prop.Description,
					Enum:        prop.Enum,
				})
			}
			tool.Function.Parameters.Properties = props
		}
		out[i] = tool
	}
	return out
}

// fromOllamaMetrics converts Ollama API metrics to canonical stream metrics.
func fromOllamaMetrics(m api.Metrics) model.StreamMetrics {
	return model.StreamMetrics{
		PromptEvalCount: m.PromptEvalCount,
		EvalCount:       m.EvalCount,
	}
}
