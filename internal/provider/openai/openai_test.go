package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	sdkoai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/pagination"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock types ---

// mockCompletions implements completionsAPI for testing.
type mockCompletions struct {
	newFunc       func(ctx context.Context, body sdkoai.ChatCompletionNewParams, opts ...option.RequestOption) (*sdkoai.ChatCompletion, error)
	newStreamFunc func(ctx context.Context, body sdkoai.ChatCompletionNewParams, opts ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk]
	// captured stores the last params for assertions.
	captured *sdkoai.ChatCompletionNewParams
}

func (m *mockCompletions) New(ctx context.Context, body sdkoai.ChatCompletionNewParams, opts ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
	m.captured = &body
	if m.newFunc != nil {
		return m.newFunc(ctx, body, opts...)
	}
	return nil, fmt.Errorf("mockCompletions.New not configured")
}

func (m *mockCompletions) NewStreaming(ctx context.Context, body sdkoai.ChatCompletionNewParams, opts ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
	m.captured = &body
	if m.newStreamFunc != nil {
		return m.newStreamFunc(ctx, body, opts...)
	}
	// Return a stream that immediately ends with no data.
	return ssestream.NewStream[sdkoai.ChatCompletionChunk](newMockDecoder(nil, nil), nil)
}

// mockModels implements modelsAPI for testing.
type mockModels struct {
	listFunc func(ctx context.Context, opts ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model]
}

func (m *mockModels) ListAutoPaging(ctx context.Context, opts ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model] {
	if m.listFunc != nil {
		return m.listFunc(ctx, opts...)
	}
	// Return empty pager.
	return pagination.NewPageAutoPager[sdkoai.Model](nil, nil)
}

// --- Mock SSE decoder for streaming tests ---

// mockDecoder implements ssestream.Decoder to feed JSON events to the stream.
// Mirrors the real eventStreamDecoder contract: Next() advances and sets
// the current event, Event() returns the current event without advancing.
type mockDecoder struct {
	events []ssestream.Event
	cur    ssestream.Event
	idx    int
	err    error
}

func newMockDecoder(jsonItems []string, err error) *mockDecoder {
	events := make([]ssestream.Event, len(jsonItems))
	for i, j := range jsonItems {
		events[i] = ssestream.Event{Data: []byte(j + "\n")}
	}
	return &mockDecoder{events: events, err: err}
}

func (d *mockDecoder) Next() bool {
	if d.idx < len(d.events) {
		d.cur = d.events[d.idx]
		d.idx++
		return true
	}
	return false
}

func (d *mockDecoder) Event() ssestream.Event {
	return d.cur
}

func (d *mockDecoder) Close() error { return nil }

func (d *mockDecoder) Err() error { return d.err }

// --- Helper to unmarshal SDK types from JSON ---

func unmarshalCompletion(t *testing.T, jsonStr string) *sdkoai.ChatCompletion {
	t.Helper()
	var c sdkoai.ChatCompletion
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &c))
	return &c
}

func modelsPage(models ...sdkoai.Model) *pagination.PageAutoPager[sdkoai.Model] {
	page := &pagination.Page[sdkoai.Model]{}
	page.Data = models
	return pagination.NewPageAutoPager(page, nil)
}

func modelsPageError(err error) *pagination.PageAutoPager[sdkoai.Model] {
	return pagination.NewPageAutoPager[sdkoai.Model](nil, err)
}

func modelWith(id string) sdkoai.Model {
	var m sdkoai.Model
	data := fmt.Sprintf(`{"id":%q,"object":"model","created":1000,"owned_by":"test"}`, id)
	json.Unmarshal([]byte(data), &m)
	return m
}

// streamWith creates a stream that returns the given JSON chunk strings.
func streamWith(jsonChunks ...string) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
	return ssestream.NewStream[sdkoai.ChatCompletionChunk](newMockDecoder(jsonChunks, nil), nil)
}

// streamError creates a stream whose decoder reports an error after events are exhausted.
func streamError(err error) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
	return ssestream.NewStream[sdkoai.ChatCompletionChunk](newMockDecoder(nil, err), nil)
}

// chunk builds a JSON string for a ChatCompletionChunk with the given content delta.
func chunk(content string) string {
	return fmt.Sprintf(`{"id":"c1","object":"chat.completion.chunk","created":1000,"model":"test","choices":[{"index":0,"delta":{"content":%q},"finish_reason":null}]}`, content)
}

// emptyChunk builds a chunk with empty content.
func emptyChunk() string {
	return `{"id":"c1","object":"chat.completion.chunk","created":1000,"model":"test","choices":[{"index":0,"delta":{"content":""},"finish_reason":null}]}`
}

// --- Constructor tests ---

func TestNew(t *testing.T) {
	p, err := New("http://localhost:1234", "")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewWithAPIKey(t *testing.T) {
	p, err := New("https://api.openai.com/v1", "sk-test")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewEmptyURL(t *testing.T) {
	p, err := New("", "")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

// --- Name test ---

func TestProviderName(t *testing.T) {
	p := newWithAPI(&mockCompletions{}, &mockModels{})
	assert.Equal(t, "openai", p.Name())
}

// --- ListModels tests ---

func TestListModelsReturnsIDs(t *testing.T) {
	mc := &mockModels{
		listFunc: func(_ context.Context, _ ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model] {
			return modelsPage(modelWith("gpt-4o"), modelWith("gpt-3.5-turbo"), modelWith("llama3"))
		},
	}
	p := newWithAPI(&mockCompletions{}, mc)

	names, err := p.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-4o", "gpt-3.5-turbo", "llama3"}, names)
}

func TestListModelsError(t *testing.T) {
	mc := &mockModels{
		listFunc: func(_ context.Context, _ ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model] {
			return modelsPageError(fmt.Errorf("connection refused"))
		},
	}
	p := newWithAPI(&mockCompletions{}, mc)

	_, err := p.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing models")
	assert.Contains(t, err.Error(), "connection refused")
}

// --- Ping tests ---

func TestPingSuccess(t *testing.T) {
	mc := &mockModels{
		listFunc: func(_ context.Context, _ ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model] {
			return modelsPage(modelWith("gpt-4o"))
		},
	}
	p := newWithAPI(&mockCompletions{}, mc)

	err := p.Ping(context.Background())
	assert.NoError(t, err)
}

func TestPingNoModels(t *testing.T) {
	mc := &mockModels{
		listFunc: func(_ context.Context, _ ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model] {
			return modelsPage() // empty
		},
	}
	p := newWithAPI(&mockCompletions{}, mc)

	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no models available")
}

func TestPingConnectionError(t *testing.T) {
	mc := &mockModels{
		listFunc: func(_ context.Context, _ ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model] {
			return modelsPageError(fmt.Errorf("dial tcp: connection refused"))
		},
	}
	p := newWithAPI(&mockCompletions{}, mc)

	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect to OpenAI-compatible endpoint")
}

// --- GetContextLength test ---

func TestGetContextLengthReturnsZero(t *testing.T) {
	p := newWithAPI(&mockCompletions{}, &mockModels{})

	length, err := p.GetContextLength(context.Background(), "gpt-4o")
	require.NoError(t, err)
	assert.Equal(t, 0, length)
}

// --- StreamChat streaming tests (no tools) ---

func TestStreamChatStreaming(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("Hello"), chunk(" "), chunk("world"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var tokens []string
	msg, metrics, err := p.StreamChat(context.Background(), req, func(tok string) {
		tokens = append(tokens, tok)
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	assert.Equal(t, "assistant", msg.Role)
	assert.Equal(t, []string{"Hello", " ", "world"}, tokens)
	assert.NotNil(t, metrics)
}

func TestStreamChatStreamingSkipsEmptyContent(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("Hello"), emptyChunk(), chunk(" world"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var tokens []string
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		tokens = append(tokens, tok)
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	assert.Equal(t, []string{"Hello", " world"}, tokens)
}

func TestStreamChatStreamingError(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamError(fmt.Errorf("network timeout"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai streaming chat")
}

// --- StreamChat non-streaming tests (with tools) ---

func TestStreamChatNonStreamingToolCalls(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{
						"id": "call_abc123",
						"type": "function",
						"function": {"name": "shell_exec", "arguments": "{\"command\":\"ls\"}"}
					},
					{
						"id": "call_def456",
						"type": "function",
						"function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/test.txt\"}"}
					}
				]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "run ls"}},
		Tools: []model.ToolDefinition{{
			Type:     "function",
			Function: model.ToolFunction{Name: "shell_exec"},
		}},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.Len(t, msg.ToolCalls, 2)

	// First tool call
	assert.Equal(t, "call_abc123", msg.ToolCalls[0].ID)
	assert.Equal(t, "shell_exec", msg.ToolCalls[0].Function.Name)
	assert.Equal(t, map[string]any{"command": "ls"}, msg.ToolCalls[0].Function.Arguments)

	// Second tool call
	assert.Equal(t, "call_def456", msg.ToolCalls[1].ID)
	assert.Equal(t, "read_file", msg.ToolCalls[1].Function.Name)
	assert.Equal(t, map[string]any{"path": "/tmp/test.txt"}, msg.ToolCalls[1].Function.Arguments)
}

func TestStreamChatNonStreamingToolCallBadJSON(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [{
					"id": "call_bad",
					"type": "function",
					"function": {"name": "test_tool", "arguments": "not valid json {{{"}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "test"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "test_tool"}}},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, "call_bad", msg.ToolCalls[0].ID)
	// Bad JSON falls back to _raw
	assert.Equal(t, map[string]any{"_raw": "not valid json {{{"}, msg.ToolCalls[0].Function.Arguments)
}

func TestStreamChatNonStreamingContent(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Hello world"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	var tokenCalled bool
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		tokenCalled = true
		assert.Equal(t, "Hello world", tok)
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello world", msg.Content)
	assert.True(t, tokenCalled)
}

func TestStreamChatNonStreamingNoChoices(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [],
		"usage": {"prompt_tokens": 10, "completion_tokens": 0, "total_tokens": 10}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no choices")
}

// --- StreamChat metrics tests ---

func TestStreamChatNonStreamingMetrics(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "ok"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 42, "completion_tokens": 10, "total_tokens": 52}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	_, metrics, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Equal(t, 42, metrics.PromptEvalCount)
	assert.Equal(t, 10, metrics.EvalCount)
}

// --- Thinking extraction tests ---

func TestStreamChatThinkTags(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "<think>reasoning here</think>actual content"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "think"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "reasoning here", msg.Thinking)
	assert.Equal(t, "actual content", msg.Content)
}

func TestStreamChatNoThinkTags(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "just regular content"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "", msg.Thinking)
	assert.Equal(t, "just regular content", msg.Content)
}

func TestStreamChatThinkTagsMultiline(t *testing.T) {
	completionJSON := `{
		"id": "comp_1",
		"object": "chat.completion",
		"created": 1000,
		"model": "test",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "<think>line1\nline2\nline3</think>the answer"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, completionJSON), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "think"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\nline3", msg.Thinking)
	assert.Equal(t, "the answer", msg.Content)
}

// --- Thinking extraction in streaming path ---

func TestStreamChatStreamingThinkTags(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("<think>reasoning</think>response text"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	msg, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "reasoning", msg.Thinking)
	assert.Equal(t, "response text", msg.Content)
}

// --- Message conversion tests ---

func TestStreamChatSendsMessages(t *testing.T) {
	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, `{
				"id": "comp_1",
				"object": "chat.completion",
				"created": 1000,
				"model": "test",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 1, "total_tokens": 11}
			}`), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
			{Role: "tool", Content: `{"result":"ok"}`, ToolCallID: "call_1"},
		},
		Tools: []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mc.captured)
	assert.Equal(t, "test-model", mc.captured.Model)
	assert.Len(t, mc.captured.Messages, 5) // system + user + assistant + user + tool
}

func TestStreamChatPassesTools(t *testing.T) {
	mc := &mockCompletions{
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			return unmarshalCompletion(t, `{
				"id": "comp_1",
				"object": "chat.completion",
				"created": 1000,
				"model": "test",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 1, "total_tokens": 11}
			}`), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		Tools: []model.ToolDefinition{
			{
				Type: "function",
				Function: model.ToolFunction{
					Name:        "shell_exec",
					Description: "Execute a shell command",
					Parameters: model.ToolFunctionParameters{
						Type:     "object",
						Required: []string{"command"},
						Properties: map[string]model.ToolProperty{
							"command": {Type: model.PropertyType{"string"}, Description: "The command to run"},
						},
					},
				},
			},
		},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mc.captured)
	require.Len(t, mc.captured.Tools, 1)
}

// --- Streaming vs non-streaming dispatch tests ---

func TestStreamChatNoToolsUsesStreaming(t *testing.T) {
	streamingCalled := false
	nonStreamingCalled := false

	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			streamingCalled = true
			return streamWith(chunk("hello"))
		},
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			nonStreamingCalled = true
			return nil, fmt.Errorf("should not be called")
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		// No tools
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.True(t, streamingCalled, "streaming path should be used when no tools")
	assert.False(t, nonStreamingCalled, "non-streaming path should not be used when no tools")
}

func TestStreamChatWithToolsUsesNonStreaming(t *testing.T) {
	streamingCalled := false
	nonStreamingCalled := false

	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			streamingCalled = true
			return nil
		},
		newFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) (*sdkoai.ChatCompletion, error) {
			nonStreamingCalled = true
			return unmarshalCompletion(t, `{
				"id": "comp_1",
				"object": "chat.completion",
				"created": 1000,
				"model": "test",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 1, "total_tokens": 11}
			}`), nil
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.ToolFunction{Name: "dummy"}}},
	}

	_, _, err := p.StreamChat(context.Background(), req, nil, nil)
	require.NoError(t, err)
	assert.False(t, streamingCalled, "streaming path should not be used when tools present")
	assert.True(t, nonStreamingCalled, "non-streaming path should be used when tools present")
}

// --- Streaming thinking delivery tests ---

func TestStreamChatStreamingThinkingDelivery(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("<think>I'm reasoning</think>Here is the answer"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var thinkingTokens []string
	var contentTokens []string
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		contentTokens = append(contentTokens, tok)
	}, func(tok string) {
		thinkingTokens = append(thinkingTokens, tok)
	})
	require.NoError(t, err)

	// onThinking must have been called with reasoning content
	assert.NotEmpty(t, thinkingTokens, "onThinking should have been called")
	combinedThinking := ""
	for _, t := range thinkingTokens {
		combinedThinking += t
	}
	assert.Equal(t, "I'm reasoning", combinedThinking)

	// onToken must have been called with content (not thinking)
	assert.NotEmpty(t, contentTokens, "onToken should have been called")
	combinedContent := ""
	for _, t := range contentTokens {
		combinedContent += t
	}
	assert.Equal(t, "Here is the answer", combinedContent)

	// Final message fields
	assert.Equal(t, "I'm reasoning", msg.Thinking)
	assert.Equal(t, "Here is the answer", msg.Content)
}

func TestStreamChatStreamingThinkingSplitAcrossChunks(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("<think>reason"), chunk("ing</think>response"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var thinkingTokens []string
	var contentTokens []string
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		contentTokens = append(contentTokens, tok)
	}, func(tok string) {
		thinkingTokens = append(thinkingTokens, tok)
	})
	require.NoError(t, err)

	// onThinking should receive concatenated thinking content
	combinedThinking := ""
	for _, t := range thinkingTokens {
		combinedThinking += t
	}
	assert.Equal(t, "reasoning", combinedThinking)

	// onToken should receive content after </think>
	combinedContent := ""
	for _, t := range contentTokens {
		combinedContent += t
	}
	assert.Equal(t, "response", combinedContent)

	// Final message fields
	assert.Equal(t, "reasoning", msg.Thinking)
	assert.Equal(t, "response", msg.Content)
}

func TestStreamChatStreamingThinkingOnlyNoContent(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("<think>just thinking</think>"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var thinkingTokens []string
	var contentTokens []string
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		contentTokens = append(contentTokens, tok)
	}, func(tok string) {
		thinkingTokens = append(thinkingTokens, tok)
	})
	require.NoError(t, err)

	// onThinking called
	combinedThinking := ""
	for _, t := range thinkingTokens {
		combinedThinking += t
	}
	assert.Equal(t, "just thinking", combinedThinking)

	// onToken never called (no content after think tags)
	assert.Empty(t, contentTokens, "onToken should not have been called")

	// Final message fields
	assert.Equal(t, "just thinking", msg.Thinking)
	assert.Equal(t, "", msg.Content)
}

func TestStreamChatStreamingThinkingNilCallback(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("<think>reasoning</think>content here"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var contentTokens []string
	// Pass nil for onThinking -- must not panic
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		contentTokens = append(contentTokens, tok)
	}, nil)
	require.NoError(t, err)

	// msg.Thinking should still be populated even with nil callback
	assert.Equal(t, "reasoning", msg.Thinking)

	// onToken still receives content
	combinedContent := ""
	for _, t := range contentTokens {
		combinedContent += t
	}
	assert.Equal(t, "content here", combinedContent)
	assert.Equal(t, "content here", msg.Content)
}

func TestStreamChatStreamingNoThinkTags(t *testing.T) {
	mc := &mockCompletions{
		newStreamFunc: func(_ context.Context, _ sdkoai.ChatCompletionNewParams, _ ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk] {
			return streamWith(chunk("plain content"))
		},
	}
	p := newWithAPI(mc, &mockModels{})

	req := &provider.ChatRequest{
		Model:    "test",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	var thinkingTokens []string
	var contentTokens []string
	msg, _, err := p.StreamChat(context.Background(), req, func(tok string) {
		contentTokens = append(contentTokens, tok)
	}, func(tok string) {
		thinkingTokens = append(thinkingTokens, tok)
	})
	require.NoError(t, err)

	// onThinking never called
	assert.Empty(t, thinkingTokens, "onThinking should not have been called")

	// onToken called with plain content
	combinedContent := ""
	for _, t := range contentTokens {
		combinedContent += t
	}
	assert.Equal(t, "plain content", combinedContent)

	// Final message fields
	assert.Equal(t, "", msg.Thinking)
	assert.Equal(t, "plain content", msg.Content)
}

