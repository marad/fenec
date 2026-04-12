package ollama

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPI implements chatAPI for testing.
type mockAPI struct {
	listFunc func(ctx context.Context) (*api.ListResponse, error)
	chatFunc func(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error
	showFunc func(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error)
}

func (m *mockAPI) List(ctx context.Context) (*api.ListResponse, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	return &api.ListResponse{}, nil
}

func (m *mockAPI) Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req, fn)
	}
	return nil
}

func (m *mockAPI) Show(ctx context.Context, req *api.ShowRequest) (*api.ShowResponse, error) {
	if m.showFunc != nil {
		return m.showFunc(ctx, req)
	}
	return &api.ShowResponse{}, nil
}

func TestNewDefaultHost(t *testing.T) {
	p, err := New("")
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.NotNil(t, p.api)
}

func TestNewCustomHost(t *testing.T) {
	p, err := New("http://myhost:11434")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewInvalidHost(t *testing.T) {
	_, err := New("://bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid host URL")
}

func TestListModelsFormatsNames(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return &api.ListResponse{
				Models: []api.ListModelResponse{
					{Name: "gemma4:latest"},
					{Name: "llama3:8b"},
					{Name: "codellama:7b"},
				},
			}, nil
		},
	}

	p := newWithAPI(mock)
	names, err := p.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"gemma4:latest", "llama3:8b", "codellama:7b"}, names)
}

func TestListModelsError(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	p := newWithAPI(mock)
	_, err := p.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list models")
}

func TestPingSuccess(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return &api.ListResponse{
				Models: []api.ListModelResponse{
					{Name: "gemma4:latest"},
				},
			}, nil
		},
	}

	p := newWithAPI(mock)
	err := p.Ping(context.Background())
	assert.NoError(t, err)
}

func TestPingNoModelsError(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return &api.ListResponse{
				Models: []api.ListModelResponse{},
			}, nil
		},
	}

	p := newWithAPI(mock)
	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no models installed")
	assert.Contains(t, err.Error(), "ollama pull gemma4")
}

func TestPingConnectionError(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	p := newWithAPI(mock)
	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect to Ollama")
}

func TestGetContextLengthFromModelInfo(t *testing.T) {
	mock := &mockAPI{
		showFunc: func(_ context.Context, req *api.ShowRequest) (*api.ShowResponse, error) {
			assert.Equal(t, "gemma4", req.Model)
			return &api.ShowResponse{
				ModelInfo: map[string]any{
					"general.architecture": "gemma3",
					"gemma3.context_length": float64(8192),
				},
			}, nil
		},
	}

	p := newWithAPI(mock)
	length, err := p.GetContextLength(context.Background(), "gemma4")
	require.NoError(t, err)
	assert.Equal(t, 8192, length)
}

func TestGetContextLengthFallbackNoKey(t *testing.T) {
	mock := &mockAPI{
		showFunc: func(_ context.Context, _ *api.ShowRequest) (*api.ShowResponse, error) {
			return &api.ShowResponse{
				ModelInfo: map[string]any{
					"general.architecture": "gemma3",
				},
			}, nil
		},
	}

	p := newWithAPI(mock)
	length, err := p.GetContextLength(context.Background(), "gemma4")
	require.NoError(t, err)
	assert.Equal(t, 4096, length)
}

func TestGetContextLengthFallbackOnError(t *testing.T) {
	mock := &mockAPI{
		showFunc: func(_ context.Context, _ *api.ShowRequest) (*api.ShowResponse, error) {
			return nil, fmt.Errorf("model not found")
		},
	}

	p := newWithAPI(mock)
	length, err := p.GetContextLength(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, 4096, length)
}

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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	var received []string
	msg, _, err := p.StreamChat(context.Background(), req, func(token string) {
		received = append(received, token)
	}, nil)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	var received []string
	msg, _, err := p.StreamChat(context.Background(), req, func(token string) {
		received = append(received, token)
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	assert.Equal(t, []string{"Hello", " world"}, received)
}

func TestStreamChatSendsMessages(t *testing.T) {
	var capturedReq *api.ChatRequest

	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, _ api.ChatResponseFunc) error {
			capturedReq = req
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "gemma4",
		Messages: []model.Message{
			{Role: "system", Content: "Be helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	msg, metrics, err := p.StreamChat(ctx, req, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	// The partial content should still be available
	assert.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Hello")
	// Metrics should be non-nil even on cancellation
	assert.NotNil(t, metrics)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	var firstTokenCalled atomic.Int32
	var once bool
	_, _, err := p.StreamChat(context.Background(), req, func(_ string) {
		if !once {
			once = true
			firstTokenCalled.Add(1)
		}
	}, nil)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	msg, metrics, err := p.StreamChat(context.Background(), req, nil, nil)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model:         "test-model",
		ContextLength: 8192,
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
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

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)

	// Truncate should still be false
	require.NotNil(t, capturedReq.Truncate)
	assert.False(t, *capturedReq.Truncate)

	// Options should be nil when ContextLength is 0
	assert.Nil(t, capturedReq.Options)
}

func TestStreamChatToolCalls(t *testing.T) {
	// Mock returns a response with ToolCalls on the Done chunk
	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			fn(api.ChatResponse{
				Done: true,
				Message: api.Message{
					Role: "assistant",
					ToolCalls: []api.ToolCall{
						{
							ID: "call_1",
							Function: api.ToolCallFunction{
								Name: "shell_exec",
							},
						},
					},
				},
				Metrics: api.Metrics{PromptEvalCount: 10, EvalCount: 5},
			})
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test",
		Messages: []model.Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "run ls"},
		},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, "call_1", msg.ToolCalls[0].ID)
	assert.Equal(t, "shell_exec", msg.ToolCalls[0].Function.Name)
}

func TestStreamChatCapturesThinking(t *testing.T) {
	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			// Thinking chunks arrive first
			_ = fn(api.ChatResponse{Message: api.Message{Thinking: "Let me think"}})
			_ = fn(api.ChatResponse{Message: api.Message{Thinking: " about this"}})
			// Then content
			_ = fn(api.ChatResponse{Message: api.Message{Content: "Here is my answer."}})
			// Done
			_ = fn(api.ChatResponse{
				Done:    true,
				Message: api.Message{Role: "assistant"},
			})
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Think: true,
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Let me think about this", msg.Thinking)
	assert.Equal(t, "Here is my answer.", msg.Content)
}

func TestStreamChatCallsOnThinking(t *testing.T) {
	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			_ = fn(api.ChatResponse{Message: api.Message{Thinking: "Step 1"}})
			_ = fn(api.ChatResponse{Message: api.Message{Thinking: "Step 2"}})
			_ = fn(api.ChatResponse{Message: api.Message{Content: "Done."}})
			_ = fn(api.ChatResponse{Done: true, Message: api.Message{Role: "assistant"}})
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Think: true,
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	var received []string
	_, _, err := p.StreamChat(context.Background(), req, nil, func(chunk string) {
		received = append(received, chunk)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Step 1", "Step 2"}, received)
}

func TestStreamChatSkipsEmptyThinking(t *testing.T) {
	mock := &mockAPI{
		chatFunc: func(_ context.Context, _ *api.ChatRequest, fn api.ChatResponseFunc) error {
			_ = fn(api.ChatResponse{Message: api.Message{Thinking: "Real thought"}})
			_ = fn(api.ChatResponse{Message: api.Message{Thinking: ""}})
			_ = fn(api.ChatResponse{Message: api.Message{Content: "Response."}})
			_ = fn(api.ChatResponse{Done: true, Message: api.Message{Role: "assistant"}})
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Think: true,
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	var received []string
	_, _, err := p.StreamChat(context.Background(), req, nil, func(chunk string) {
		received = append(received, chunk)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Real thought"}, received)
}

func TestStreamChatThinkEnabled(t *testing.T) {
	var capturedReq *api.ChatRequest
	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, _ api.ChatResponseFunc) error {
			capturedReq = req
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Think: true,
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)
	require.NotNil(t, capturedReq.Think)
	assert.Equal(t, true, capturedReq.Think.Value)
}

func TestStreamChatThinkDisabledByDefault(t *testing.T) {
	var capturedReq *api.ChatRequest
	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, _ api.ChatResponseFunc) error {
			capturedReq = req
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)
	require.NotNil(t, capturedReq.Think)
	assert.Equal(t, false, capturedReq.Think.Value)
}

func TestStreamChatPassesTools(t *testing.T) {
	var capturedReq *api.ChatRequest
	mock := &mockAPI{
		chatFunc: func(_ context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
			capturedReq = req
			fn(api.ChatResponse{Done: true, Message: api.Message{Role: "assistant", Content: "ok"}})
			return nil
		},
	}

	p := newWithAPI(mock)
	req := &provider.ChatRequest{
		Model: "test",
		Messages: []model.Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "hello"},
		},
		Tools: []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "test_tool"}}},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)
	require.Len(t, capturedReq.Tools, 1)
	assert.Equal(t, "test_tool", capturedReq.Tools[0].Function.Name)
}

func TestProviderName(t *testing.T) {
	p := newWithAPI(&mockAPI{})
	assert.Equal(t, "ollama", p.Name())
}
