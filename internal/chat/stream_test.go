package chat

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamChatAccumulatesTokens(t *testing.T) {
	tokens := []string{"Hello", " ", "world"}

	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			for _, tok := range tokens {
				if err := fn(api.ChatResponse{
					Message: api.Message{Content: tok},
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "You are helpful.")
	conv.AddUser("Hi")

	msg, _, err := client.StreamChat(context.Background(), conv, nil)
	require.NoError(t, err)
	assert.Equal(t, "assistant", msg.Role)
	assert.Equal(t, "Hello world", msg.Content)
}

func TestStreamChatCallsOnToken(t *testing.T) {
	tokens := []string{"Hello", " ", "world"}

	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			for _, tok := range tokens {
				if err := fn(api.ChatResponse{
					Message: api.Message{Content: tok},
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.AddUser("Hi")

	var received []string
	msg, _, err := client.StreamChat(context.Background(), conv, func(token string) {
		received = append(received, token)
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	assert.Equal(t, tokens, received)
}

func TestStreamChatSkipsEmptyTokens(t *testing.T) {
	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			_ = fn(api.ChatResponse{Message: api.Message{Content: "Hello"}})
			_ = fn(api.ChatResponse{Message: api.Message{Content: ""}})
			_ = fn(api.ChatResponse{Message: api.Message{Content: " world"}})
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.AddUser("Hi")

	var received []string
	msg, _, err := client.StreamChat(context.Background(), conv, func(token string) {
		received = append(received, token)
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	assert.Equal(t, []string{"Hello", " world"}, received)
}

func TestStreamChatSendsConversationMessages(t *testing.T) {
	var capturedReq *api.ChatRequest

	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, _ api.ChatResponseFunc) error {
			capturedReq = req
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("gemma4", "Be helpful.")
	conv.AddUser("Hello")
	conv.AddAssistant("Hi there!")
	conv.AddUser("How are you?")

	_, _, err := client.StreamChat(context.Background(), conv, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)
	assert.Equal(t, "gemma4", capturedReq.Model)
	assert.Len(t, capturedReq.Messages, 4) // system + user + assistant + user
	assert.Equal(t, "system", capturedReq.Messages[0].Role)
	assert.Equal(t, "user", capturedReq.Messages[3].Role)
}

func TestStreamChatCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mock := &mockAPI{
		chatFunc: func(ctx context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			// Simulate: first token arrives, then context is cancelled
			if err := fn(api.ChatResponse{
				Message: api.Message{Content: "Hello"},
			}); err != nil {
				return err
			}

			// Cancel the context
			cancel()

			// The callback should return ctx.Err() which is non-nil now
			err := fn(api.ChatResponse{
				Message: api.Message{Content: " world"},
			})
			if err != nil {
				return err
			}
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.AddUser("Hi")

	msg, metrics, err := client.StreamChat(ctx, conv, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	// The partial content should still be available
	assert.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Hello")
	// Metrics should be non-nil even on cancellation
	assert.NotNil(t, metrics)
}

func TestFirstTokenNotifierCallsOnce(t *testing.T) {
	var callCount atomic.Int32

	notifier := NewFirstTokenNotifier(func() {
		callCount.Add(1)
	})

	// Call Notify multiple times
	notifier.Notify()
	notifier.Notify()
	notifier.Notify()

	assert.Equal(t, int32(1), callCount.Load())
}

func TestFirstTokenNotifierNilCallback(t *testing.T) {
	notifier := NewFirstTokenNotifier(nil)
	// Should not panic
	assert.NotPanics(t, func() {
		notifier.Notify()
	})
}

func TestStreamChatFirstTokenNotifier(t *testing.T) {
	tokens := []string{"Hello", " ", "world"}

	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			for _, tok := range tokens {
				if err := fn(api.ChatResponse{
					Message: api.Message{Content: tok},
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.AddUser("Hi")

	var firstTokenCalled atomic.Int32
	notifier := NewFirstTokenNotifier(func() {
		firstTokenCalled.Add(1)
	})

	// Wire the notifier into the onToken callback
	_, _, err := client.StreamChat(context.Background(), conv, func(_ string) {
		notifier.Notify()
	})
	require.NoError(t, err)

	// Should have been called exactly once despite 3 tokens
	assert.Equal(t, int32(1), firstTokenCalled.Load())
}

func TestStreamChatReturnsMetrics(t *testing.T) {
	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			// Simulate streaming tokens
			_ = fn(api.ChatResponse{
				Message: api.Message{Content: "Hello"},
			})
			// Final chunk with Done=true and metrics
			_ = fn(api.ChatResponse{
				Message: api.Message{Content: " world"},
				Done:    true,
				Metrics: api.Metrics{
					PromptEvalCount: 42,
					EvalCount:       10,
				},
			})
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.AddUser("Hi")

	msg, metrics, err := client.StreamChat(context.Background(), conv, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	require.NotNil(t, metrics)
	assert.Equal(t, 42, metrics.PromptEvalCount)
	assert.Equal(t, 10, metrics.EvalCount)
}

func TestStreamChatSetsTruncateFalseAndNumCtx(t *testing.T) {
	var capturedReq *api.ChatRequest

	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, _ api.ChatResponseFunc) error {
			capturedReq = req
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.ContextLength = 8192
	conv.AddUser("Hi")

	_, _, err := client.StreamChat(context.Background(), conv, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	// Truncate should be explicitly false
	require.NotNil(t, capturedReq.Truncate)
	assert.False(t, *capturedReq.Truncate)

	// num_ctx should be set from ContextLength
	require.NotNil(t, capturedReq.Options)
	assert.Equal(t, 8192, capturedReq.Options["num_ctx"])
}

func TestStreamChatOmitsNumCtxWhenContextLengthZero(t *testing.T) {
	var capturedReq *api.ChatRequest

	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, _ api.ChatResponseFunc) error {
			capturedReq = req
			return nil
		},
	}

	client := newClientWithAPI(mock)
	conv := NewConversation("test-model", "")
	conv.AddUser("Hi")

	_, _, err := client.StreamChat(context.Background(), conv, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	// Truncate should still be false
	require.NotNil(t, capturedReq.Truncate)
	assert.False(t, *capturedReq.Truncate)

	// Options should be nil when ContextLength is 0
	assert.Nil(t, capturedReq.Options)
}
