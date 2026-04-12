package chat

import (
	"context"
	"strings"
	"sync"

	"github.com/ollama/ollama/api"
)

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

// StreamChat sends the conversation to Ollama and streams the response.
// onToken is called for each content chunk as it arrives.
// The full assistant message and Ollama Metrics are returned after streaming completes.
// Context cancellation stops the stream (per D-04: Ctrl+C cancels active generation).
func (c *Client) StreamChat(ctx context.Context, conv *Conversation, tools api.Tools, onToken func(string), onThinking func(string)) (*api.Message, *api.Metrics, error) {
	var content strings.Builder
	var thinking strings.Builder
	var metrics api.Metrics
	var finalMsg api.Message

	req := &api.ChatRequest{
		Model:    conv.Model,
		Messages: conv.Messages,
		Tools:    tools,
		Truncate: boolPtr(false),
	}

	// Enable thinking/reasoning output when requested.
	if conv.Think {
		req.Think = &api.ThinkValue{Value: true}
	}

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
			finalMsg.Content = content.String()
			finalMsg.Role = "assistant"
			return &finalMsg, &metrics, ctx.Err()
		}
		return nil, &metrics, err
	}

	// If no Done chunk was received (e.g. stream ended without it),
	// ensure content and role are still set from the accumulated builder.
	if finalMsg.Role == "" {
		finalMsg.Role = "assistant"
		finalMsg.Content = content.String()
	}

	return &finalMsg, &metrics, nil
}

// Compile-time check: Client satisfies ChatService.
var _ ChatService = (*Client)(nil)

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
