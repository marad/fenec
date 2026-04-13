package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	sdkoai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/pagination"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

// Compile-time check: Provider satisfies provider.Provider.
var _ provider.Provider = (*Provider)(nil)

// thinkRegex matches <think>...</think> blocks in model output.
var thinkRegex = regexp.MustCompile(`(?s)<think>(.*?)</think>`)

// completionsAPI is the narrow interface for chat completions.
// In production this is satisfied by *sdkoai.ChatCompletionService; in tests by a mock.
type completionsAPI interface {
	New(ctx context.Context, body sdkoai.ChatCompletionNewParams, opts ...option.RequestOption) (*sdkoai.ChatCompletion, error)
	NewStreaming(ctx context.Context, body sdkoai.ChatCompletionNewParams, opts ...option.RequestOption) *ssestream.Stream[sdkoai.ChatCompletionChunk]
}

// modelsAPI is the narrow interface for model listing.
// In production this is satisfied by *sdkoai.ModelService; in tests by a mock.
type modelsAPI interface {
	ListAutoPaging(ctx context.Context, opts ...option.RequestOption) *pagination.PageAutoPager[sdkoai.Model]
}

// Provider wraps an OpenAI-compatible API client, implementing provider.Provider.
type Provider struct {
	completions completionsAPI
	models      modelsAPI
}

// New creates a Provider connecting to the given OpenAI-compatible endpoint.
// If baseURL is empty, the SDK default (https://api.openai.com/v1) is used.
// If apiKey is empty, a dummy key is set to prevent the SDK from reading
// the OPENAI_API_KEY environment variable (local providers like LM Studio
// do not need a real key).
func New(baseURL, apiKey string) (*Provider, error) {
	opts := []option.RequestOption{
		option.WithMaxRetries(2),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	} else {
		opts = append(opts, option.WithAPIKey("not-needed"))
	}
	client := sdkoai.NewClient(opts...)
	return &Provider{
		completions: &client.Chat.Completions,
		models:      &client.Models,
	}, nil
}

// newWithAPI creates a Provider with custom API implementations (for testing).
func newWithAPI(completions completionsAPI, models modelsAPI) *Provider {
	return &Provider{completions: completions, models: models}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "openai"
}

// ListModels returns the names of all available models.
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	pager := p.models.ListAutoPaging(ctx)
	var names []string
	for pager.Next() {
		m := pager.Current()
		names = append(names, m.ID)
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("listing models: %w", err)
	}
	return names, nil
}

// Ping verifies the OpenAI-compatible endpoint is reachable and has models.
func (p *Provider) Ping(ctx context.Context) error {
	models, err := p.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("cannot connect to OpenAI-compatible endpoint: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("no models available from OpenAI-compatible endpoint")
	}
	return nil
}

// GetContextLength returns the context window size for a model.
// The OpenAI /v1/models endpoint does not expose context length, so we
// return 0 to signal "unknown / use model default". The OpenAI API handles
// context limits server-side.
func (p *Provider) GetContextLength(ctx context.Context, modelName string) (int, error) {
	return 0, nil
}

// StreamChat sends a chat request and streams the response.
// When tools are present, falls back to non-streaming (tool call arguments
// arrive as a complete JSON string in non-streaming mode, avoiding complex
// chunk assembly). When no tools are present, uses SSE streaming.
func (p *Provider) StreamChat(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
	if len(req.Tools) > 0 {
		return p.chatNonStreaming(ctx, req, onToken, onThinking)
	}
	return p.chatStreaming(ctx, req, onToken, onThinking)
}

// thinkParser is a state machine that incrementally separates <think>...</think>
// blocks from content as streaming chunks arrive. Thinking content is routed to
// onThinking, everything else to onToken. Both callbacks are optional (nil-safe).
type thinkParser struct {
	inThink  bool            // currently inside <think> block
	buf      strings.Builder // carries partial tag text across chunk boundaries
	thinking strings.Builder // accumulated thinking content
	content  strings.Builder // accumulated non-thinking content

	onToken    func(string)
	onThinking func(string)
}

// openTag and closeTag are the delimiters for thinking blocks.
const openTag = "<think>"
const closeTag = "</think>"

// process feeds a chunk of streaming content through the parser.
// It appends the chunk to an internal buffer, then drains as much as possible,
// leaving only a potential partial tag match at the end of the buffer.
func (tp *thinkParser) process(s string) {
	tp.buf.WriteString(s)
	tp.drain()
}

// drain consumes the internal buffer, flushing content/thinking as tags are
// found. After drain, buf contains only a possible partial tag suffix.
func (tp *thinkParser) drain() {
	for {
		text := tp.buf.String()
		if len(text) == 0 {
			return
		}

		if tp.inThink {
			// Look for closing </think> tag.
			if idx := strings.Index(text, closeTag); idx >= 0 {
				// Everything before the tag is thinking content.
				before := text[:idx]
				if before != "" {
					tp.thinking.WriteString(before)
					if tp.onThinking != nil {
						tp.onThinking(before)
					}
				}
				// Advance past the close tag and continue draining.
				tp.buf.Reset()
				tp.buf.WriteString(text[idx+len(closeTag):])
				tp.inThink = false
				continue
			}
			// No complete close tag found. Flush everything except a possible
			// partial match of </think> at the tail.
			safe := tp.safePrefixLen(text, closeTag)
			if safe > 0 {
				chunk := text[:safe]
				tp.thinking.WriteString(chunk)
				if tp.onThinking != nil {
					tp.onThinking(chunk)
				}
				tp.buf.Reset()
				tp.buf.WriteString(text[safe:])
			}
			return
		}

		// Not in think mode: look for opening <think> tag.
		if idx := strings.Index(text, openTag); idx >= 0 {
			before := text[:idx]
			if before != "" {
				tp.content.WriteString(before)
				if tp.onToken != nil {
					tp.onToken(before)
				}
			}
			tp.buf.Reset()
			tp.buf.WriteString(text[idx+len(openTag):])
			tp.inThink = true
			continue
		}
		// No complete open tag found. Flush safe prefix.
		safe := tp.safePrefixLen(text, openTag)
		if safe > 0 {
			chunk := text[:safe]
			tp.content.WriteString(chunk)
			if tp.onToken != nil {
				tp.onToken(chunk)
			}
			tp.buf.Reset()
			tp.buf.WriteString(text[safe:])
		}
		return
	}
}

// safePrefixLen returns the number of bytes at the start of text that can be
// safely flushed, leaving behind any suffix that could be the beginning of tag.
// For example, if text is "abc<thi" and tag is "<think>", returns 3 (flush "abc",
// keep "<thi" in buffer).
func (tp *thinkParser) safePrefixLen(text, tag string) int {
	// Find the longest suffix of text that is a proper prefix of tag.
	maxK := len(tag) - 1
	if maxK > len(text) {
		maxK = len(text)
	}
	for k := maxK; k >= 1; k-- {
		if strings.HasPrefix(tag, text[len(text)-k:]) {
			return len(text) - k
		}
	}
	return len(text)
}

// flush drains any remaining buffered content at end of stream.
func (tp *thinkParser) flush() {
	if tp.buf.Len() == 0 {
		return
	}
	remaining := tp.buf.String()
	tp.buf.Reset()
	if tp.inThink {
		tp.thinking.WriteString(remaining)
		if tp.onThinking != nil {
			tp.onThinking(remaining)
		}
	} else {
		tp.content.WriteString(remaining)
		if tp.onToken != nil {
			tp.onToken(remaining)
		}
	}
}

// chatStreaming handles the streaming path for pure chat (no tools).
func (p *Provider) chatStreaming(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
	params := buildParams(req)
	stream := p.completions.NewStreaming(ctx, params)
	defer stream.Close()

	tp := &thinkParser{onToken: onToken, onThinking: onThinking}
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				tp.process(delta.Content)
			}
		}
	}
	tp.flush()

	if err := stream.Err(); err != nil {
		return nil, nil, fmt.Errorf("openai streaming chat: %w", err)
	}

	msg := model.Message{
		Role:     "assistant",
		Content:  tp.content.String(),
		Thinking: strings.TrimSpace(tp.thinking.String()),
	}

	return &msg, &model.StreamMetrics{}, nil
}

// chatNonStreaming handles the non-streaming path (used when tools are present).
func (p *Provider) chatNonStreaming(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
	params := buildParams(req)
	completion, err := p.completions.New(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, nil, fmt.Errorf("openai: no choices in response")
	}

	choice := completion.Choices[0]
	msg := model.Message{
		Role:    "assistant",
		Content: choice.Message.Content,
	}

	// Parse tool calls: arguments arrive as JSON strings, parse to map[string]any.
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{"_raw": tc.Function.Arguments}
		}
		msg.ToolCalls = append(msg.ToolCalls, model.ToolCall{
			ID: tc.ID,
			Function: model.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: args,
			},
		})
	}

	// Extract thinking from reasoning_content extra field (DeepSeek, etc.).
	extractReasoningContent(&msg, choice)

	// Fallback: extract <think> tags from content.
	extractThinkingFromContent(&msg)

	// Deliver content to callback (non-streaming delivers full content at once).
	if msg.Content != "" && onToken != nil {
		onToken(msg.Content)
	}

	// Deliver thinking to callback.
	if msg.Thinking != "" && onThinking != nil {
		onThinking(msg.Thinking)
	}

	metrics := model.StreamMetrics{
		PromptEvalCount: int(completion.Usage.PromptTokens),
		EvalCount:       int(completion.Usage.CompletionTokens),
	}

	return &msg, &metrics, nil
}

// buildParams constructs the SDK request params from a ChatRequest.
func buildParams(req *provider.ChatRequest) sdkoai.ChatCompletionNewParams {
	params := sdkoai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: toOpenAIMessages(req.Messages),
	}
	if len(req.Tools) > 0 {
		params.Tools = toOpenAITools(req.Tools)
	}
	return params
}

// toOpenAIMessages converts canonical messages to OpenAI SDK message params.
// Thinking content is NOT included in outgoing messages (some providers like
// DeepSeek return 400 if reasoning_content appears in input).
func toOpenAIMessages(msgs []model.Message) []sdkoai.ChatCompletionMessageParamUnion {
	out := make([]sdkoai.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "system":
			out = append(out, sdkoai.SystemMessage(m.Content))
		case "user":
			out = append(out, sdkoai.UserMessage(m.Content))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				param := sdkoai.ChatCompletionAssistantMessageParam{
					Content: sdkoai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: sdkoai.String(m.Content),
					},
				}
				for _, tc := range m.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Function.Arguments)
					param.ToolCalls = append(param.ToolCalls, sdkoai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &sdkoai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: sdkoai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Function.Name,
								Arguments: string(argsJSON),
							},
						},
					})
				}
				out = append(out, sdkoai.ChatCompletionMessageParamUnion{OfAssistant: &param})
			} else {
				out = append(out, sdkoai.AssistantMessage(m.Content))
			}
		case "tool":
			out = append(out, sdkoai.ToolMessage(m.Content, m.ToolCallID))
		}
	}
	return out
}

// toOpenAITools converts canonical tool definitions to OpenAI SDK tool params.
func toOpenAITools(tools []model.ToolDefinition) []sdkoai.ChatCompletionToolUnionParam {
	out := make([]sdkoai.ChatCompletionToolUnionParam, len(tools))
	for i, td := range tools {
		props := make(map[string]any)
		for name, prop := range td.Function.Parameters.Properties {
			p := map[string]any{}
			if len(prop.Type) > 0 {
				p["type"] = prop.Type[0]
			}
			if prop.Description != "" {
				p["description"] = prop.Description
			}
			if len(prop.Enum) > 0 {
				p["enum"] = prop.Enum
			}
			props[name] = p
		}
		out[i] = sdkoai.ChatCompletionFunctionTool(sdkoai.FunctionDefinitionParam{
			Name:        td.Function.Name,
			Description: sdkoai.String(td.Function.Description),
			Parameters: sdkoai.FunctionParameters{
				"type":       td.Function.Parameters.Type,
				"properties": props,
				"required":   td.Function.Parameters.Required,
			},
		})
	}
	return out
}

// extractReasoningContent checks the non-streaming response for a
// reasoning_content extra field (used by DeepSeek and similar providers).
func extractReasoningContent(msg *model.Message, choice sdkoai.ChatCompletionChoice) {
	if field, ok := choice.Message.JSON.ExtraFields["reasoning_content"]; ok {
		raw := field.Raw()
		var reasoning string
		if err := json.Unmarshal([]byte(raw), &reasoning); err == nil && reasoning != "" {
			msg.Thinking = reasoning
		}
	}
}

// extractThinkingFromContent checks the message content for <think>...</think>
// tags and moves the thinking content to the Thinking field. Only acts if
// Thinking is still empty (reasoning_content takes priority).
func extractThinkingFromContent(msg *model.Message) {
	if msg.Thinking != "" {
		return
	}
	if matches := thinkRegex.FindStringSubmatch(msg.Content); len(matches) > 1 {
		msg.Thinking = strings.TrimSpace(matches[1])
		msg.Content = strings.TrimSpace(thinkRegex.ReplaceAllString(msg.Content, ""))
	}
}
