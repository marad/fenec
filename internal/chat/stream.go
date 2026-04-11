package chat

import (
	"context"
	"strings"
	"sync"

	"github.com/ollama/ollama/api"
)

// StreamChat sends the conversation to Ollama and streams the response.
// onToken is called for each content chunk as it arrives.
// The full assistant message is returned after streaming completes.
// Context cancellation stops the stream (per D-04: Ctrl+C cancels active generation).
func (c *Client) StreamChat(ctx context.Context, conv *Conversation, onToken func(string)) (*api.Message, error) {
	var content strings.Builder

	req := &api.ChatRequest{
		Model:    conv.Model,
		Messages: conv.Messages,
	}

	err := c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
		if resp.Message.Content != "" {
			content.WriteString(resp.Message.Content)
			if onToken != nil {
				onToken(resp.Message.Content)
			}
		}
		// Stop early if context is cancelled (per Pitfall 3 from RESEARCH.md).
		return ctx.Err()
	})
	if err != nil {
		// If the error is due to context cancellation, return the partial content
		// along with the error so the caller can distinguish cancellation from failure.
		if ctx.Err() != nil {
			return &api.Message{
				Role:    "assistant",
				Content: content.String(),
			}, ctx.Err()
		}
		return nil, err
	}

	return &api.Message{
		Role:    "assistant",
		Content: content.String(),
	}, nil
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
