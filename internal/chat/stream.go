package chat

import (
	"context"
	"strings"
	"sync"

	mdl "github.com/marad/fenec/internal/model"
	"github.com/ollama/ollama/api"
)

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

// StreamChat sends the conversation to Ollama and streams the response.
// onToken is called for each content chunk as it arrives.
// The full assistant message and Ollama Metrics are returned after streaming completes.
// Context cancellation stops the stream (per D-04: Ctrl+C cancels active generation).
func (c *Client) StreamChat(ctx context.Context, conv *Conversation, tools []mdl.ToolDefinition, onToken func(string), onThinking func(string)) (*mdl.Message, *mdl.StreamMetrics, error) {
	var content strings.Builder
	var thinking strings.Builder
	var metrics api.Metrics
	var finalMsg api.Message

	req := &api.ChatRequest{
		Model:    conv.Model,
		Messages: toOllamaMessages(conv.Messages),
		Tools:    toOllamaTools(tools),
		Truncate: boolPtr(false),
	}

	// Explicitly set thinking — nil means "model default" which is on for
	// thinking-capable models like Gemma 4.
	req.Think = &api.ThinkValue{Value: conv.Think}

	// Set num_ctx if conversation has a known context length.
	if conv.ContextLength > 0 {
		req.Options = map[string]any{"num_ctx": conv.ContextLength}
	}

	var toolCalls []api.ToolCall
	err := c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
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
		// Accumulate tool calls from streaming chunks. Some models (e.g.,
		// Gemma 4) send tool calls in a pre-Done chunk while the Done chunk
		// itself carries zero tool calls.
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
		// Stop early if context is cancelled (per Pitfall 3 from RESEARCH.md).
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
			cancelMsg := mdl.Message{Content: content.String(), Role: "assistant"}
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

// Compile-time check: Client satisfies ChatService.
var _ ChatService = (*Client)(nil)

// --- Ollama conversion functions (adapter boundary) ---

// toOllamaMessages converts canonical messages to Ollama API messages.
func toOllamaMessages(msgs []mdl.Message) []api.Message {
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
func fromOllamaMessage(msg api.Message) mdl.Message {
	m := mdl.Message{
		Role:       msg.Role,
		Content:    msg.Content,
		Thinking:   msg.Thinking,
		ToolCallID: msg.ToolCallID,
	}
	for _, tc := range msg.ToolCalls {
		m.ToolCalls = append(m.ToolCalls, mdl.ToolCall{
			ID: tc.ID,
			Function: mdl.ToolCallFunction{
				Index:     tc.Function.Index,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments.ToMap(),
			},
		})
	}
	return m
}

// toOllamaTools converts canonical tool definitions to Ollama API tools.
func toOllamaTools(tools []mdl.ToolDefinition) api.Tools {
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
func fromOllamaMetrics(m api.Metrics) mdl.StreamMetrics {
	return mdl.StreamMetrics{
		PromptEvalCount: m.PromptEvalCount,
		EvalCount:       m.EvalCount,
	}
}

// FirstTokenNotifier calls onFirst exactly once when the first token arrives.
// Used by the REPL to stop the thinking spinner on first token.
type FirstTokenNotifier struct {
	once    sync.Once
	onFirst func()
}

// NewFirstTokenNotifier creates a notifier that calls onFirst once.
func NewFirstTokenNotifier(onFirst func()) *FirstTokenNotifier {
	return &FirstTokenNotifier{onFirst: onFirst}
}

// Notify triggers the onFirst callback. Safe to call multiple times;
// the callback executes only on the first invocation.
func (n *FirstTokenNotifier) Notify() {
	n.once.Do(func() {
		if n.onFirst != nil {
			n.onFirst()
		}
	})
}
