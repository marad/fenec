# Phase 10: OpenAI-Compatible Client - Research

**Researched:** 2026-04-12
**Domain:** OpenAI-compatible chat completions protocol, openai-go SDK v3
**Confidence:** HIGH

## Summary

Phase 10 implements a new `provider.Provider` adapter that speaks the OpenAI `/v1/chat/completions` protocol, enabling Fenec to work with LM Studio, OpenAI cloud, and any compatible endpoint. The adapter lives in `internal/provider/openai/` and mirrors the existing Ollama adapter's structure: same interface, same canonical types, format translation at the boundary.

The official `github.com/openai/openai-go/v3` SDK (v3.31.0, released 2026-04-08) provides the client. It supports streaming via SSE, non-streaming completions, and tool calling. The SDK requires Go 1.22+ (our project uses Go 1.25.8, no conflict). Key design points: streaming is used for pure chat, non-streaming when tools are present (workaround for chunked tool call assembly complexity), tool call arguments arrive as JSON strings (parsed to `map[string]any` at boundary), and thinking/reasoning is parsed opportunistically from `reasoning_content` extra fields or `<think>` tags in content.

**Primary recommendation:** Pattern the OpenAI adapter closely after `internal/provider/ollama/ollama.go`. Use the same internal test interface pattern (define a narrow `chatAPI` interface wrapping the SDK client), same compile-time interface check, and same conversion function organization.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- When tools are present in the request: fall back to non-streaming request, then call `onToken` once with full content after completion
- When no tools: stream normally via SSE with `onToken` per delta chunk
- Chunked tool call arguments are assembled via openai-go SDK's `ChatCompletionAccumulator` helper
- Non-streaming tool call flow: full response -> parse tool_calls array -> return as `model.Message` with ToolCalls populated
- OpenAI returns tool arguments as a JSON string in `function.arguments` -> parse to `map[string]any` at adapter boundary
- Tool results sent back: `role: "tool"`, `tool_call_id: <id>`, `content: <result>` (format already matches canonical)
- Tool call ID: use OpenAI's `call_xyz` ID verbatim on canonical `model.ToolCall.ID`
- Canonical `model.ToolDefinition` -> OpenAI `ChatCompletionToolUnionParam` conversion happens in adapter only
- Phase 10 focuses only on making OpenAI adapter a functional Provider -- actual switching UX is Phase 11 scope
- Conversation history preserved as-is across provider switches (canonical types work universally)
- Opportunistic thinking: parse `reasoning_content` field from response if present, parse `<think>...</think>` tags embedded in content if present, otherwise Thinking stays empty
- No `think` flag sent in request (not in standard OpenAI API)
- New package: `internal/provider/openai/openai.go`
- Extend `internal/config/toml.go` `CreateProvider` switch with `case "openai":`
- Dependency: `github.com/openai/openai-go/v3` at v3.31.0

### Claude's Discretion
- Exact SDK initialization pattern (openai-go client construction)
- How the adapter is registered in the `CreateProvider` factory (likely `case "openai":`)
- Error message wording for missing API key, failed requests, etc.
- Internal struct shape of the OpenAI adapter
- Timeout/retry/backoff defaults for HTTP requests
- Whether `<think>` tag parsing uses a streaming parser or post-hoc regex

### Deferred Ideas (OUT OF SCOPE)
- `/provider` REPL command -- explicitly rejected, `/model` handles everything in Phase 11
- Provider health dashboard -- deferred to future milestone
- Multimodal support (images in messages) -- not in v1.1 scope
- Provider-specific parameter tuning (temperature, top_p) -- out of scope per REQUIREMENTS.md
- Automatic failover / retry with different provider -- out of scope per REQUIREMENTS.md
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OAIC-01 | User can chat with models served by LM Studio via the OpenAI-compatible protocol | SDK `option.WithBaseURL` points client at any endpoint; LM Studio serves `/v1/chat/completions` and `/v1/models` |
| OAIC-02 | User can chat with OpenAI cloud models (GPT-4o, etc.) via the OpenAI API | Same SDK, default base URL or `https://api.openai.com/v1`, `option.WithAPIKey` for auth |
| OAIC-03 | User can use tool calling with OpenAI-compatible providers (non-streaming when tools present) | Non-streaming `client.Chat.Completions.New()` returns full `ToolCalls` array; arguments as JSON string parsed to `map[string]any` |
| OAIC-04 | User can switch providers mid-session and continue the conversation | Canonical `model.Message` types are provider-agnostic; ProviderRegistry already supports `Get(name)` for switching |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/openai/openai-go/v3 | v3.31.0 | OpenAI-compatible API client | Official SDK from OpenAI. Typed request/response structs, built-in SSE streaming, ChatCompletionAccumulator for tool calls, ExtraFields for reasoning_content. Requires Go 1.22+. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/openai/openai-go/v3/option | (part of SDK) | Client configuration | `WithBaseURL`, `WithAPIKey`, `WithMaxRetries`, `WithRequestTimeout` for configuring per-provider clients |
| encoding/json (stdlib) | - | Parse tool call arguments | `json.Unmarshal` to convert JSON string arguments to `map[string]any` |
| regexp (stdlib) | - | `<think>` tag extraction | Post-hoc regex to strip thinking tags from content when present |

### Alternatives Considered
| Recommended | Alternative | Tradeoff |
|-------------|-----------|----------|
| openai-go/v3 (official) | sashabaranov/go-openai | Community lib, larger install base but not official. Official SDK has better type safety, ExtraFields for non-standard fields, and guaranteed compatibility with OpenAI API changes. |
| Non-streaming for tool calls | Streaming + ChatCompletionAccumulator for all | Streaming tool calls add significant complexity (chunked argument assembly, partial state). Non-streaming is simpler and reliable for the tool path. |
| Post-hoc regex for `<think>` | Streaming parser for think tags | Regex on final content is simpler, sufficient given Fenec uses non-streaming for tool calls (where thinking is most useful). For streaming pure chat, think tags are rare enough that post-hoc strip on final message works. |

**Installation:**
```bash
go get github.com/openai/openai-go/v3@v3.31.0
```

**Version verification:** v3.31.0 confirmed on Go proxy (published 2026-04-08). Requires Go 1.22+. Project uses Go 1.25.8 -- no conflict. Direct dependencies: tidwall/gjson, tidwall/sjson, Azure SDK (for Azure auth, not used by Fenec).

## Architecture Patterns

### Recommended Project Structure
```
internal/provider/openai/
    openai.go       # Provider struct, New(), all Provider interface methods
    openai_test.go  # Unit tests with mock chatAPI interface
```

### Pattern 1: Narrow Test Interface (from Ollama adapter)
**What:** Define a minimal interface wrapping only the SDK methods used, inject it into the Provider struct. Production uses real SDK client; tests use mock.
**When to use:** Always -- this is the established pattern in the codebase.
**Example:**
```go
// Source: Pattern from internal/provider/ollama/ollama.go
type chatAPI interface {
    CreateChatCompletion(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) (*openai.ChatCompletion, error)
    CreateChatCompletionStream(ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption) *ssestream.Stream[openai.ChatCompletionChunk]
    ListModels(ctx context.Context, opts ...option.RequestOption) *pagination.Page[openai.Model]
}
```

### Pattern 2: Client Construction with option.WithBaseURL
**What:** Create SDK client pointing at any OpenAI-compatible endpoint.
**When to use:** All provider instantiation.
**Example:**
```go
// Source: https://github.com/openai/openai-go README + option package docs
import (
    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)

func New(baseURL, apiKey string) (*Provider, error) {
    opts := []option.RequestOption{
        option.WithBaseURL(baseURL),
        option.WithMaxRetries(2),
    }
    if apiKey != "" {
        opts = append(opts, option.WithAPIKey(apiKey))
    } else {
        // For local providers (LM Studio), API key may be empty.
        // SDK defaults to OPENAI_API_KEY env var; set dummy to suppress.
        opts = append(opts, option.WithAPIKey("not-needed"))
    }
    client := openai.NewClient(opts...)
    return &Provider{api: client.Chat.Completions, models: client.Models}, nil
}
```

### Pattern 3: Non-Streaming Tool Call Flow
**What:** Use `client.Chat.Completions.New()` (non-streaming) when tools are present. Parse tool calls from response.
**When to use:** Any ChatRequest that has `len(req.Tools) > 0`.
**Example:**
```go
// Source: https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go
completion, err := p.api.CreateChatCompletion(ctx, params)
if err != nil {
    return nil, nil, err
}
choice := completion.Choices[0]

// Parse tool calls -- arguments are JSON strings
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
```

### Pattern 4: Streaming Pure Chat
**What:** Use `client.Chat.Completions.NewStreaming()` when no tools. Iterate chunks, call `onToken` per delta.
**When to use:** Any ChatRequest with `len(req.Tools) == 0`.
**Example:**
```go
// Source: https://github.com/openai/openai-go/blob/main/examples/chat-completion-streaming/main.go
stream := p.api.CreateChatCompletionStream(ctx, params)
defer stream.Close()

for stream.Next() {
    chunk := stream.Current()
    if len(chunk.Choices) > 0 {
        delta := chunk.Choices[0].Delta
        if delta.Content != "" {
            content.WriteString(delta.Content)
            if onToken != nil {
                onToken(delta.Content)
            }
        }
    }
}
if err := stream.Err(); err != nil {
    return nil, nil, err
}
```

### Pattern 5: Opportunistic Thinking Extraction
**What:** Check `reasoning_content` in ExtraFields, then `<think>` tags in content.
**When to use:** On every response (both streaming and non-streaming).
**Example:**
```go
// Source: openai-go ExtraFields documentation
// Non-streaming: check response message ExtraFields
if field, ok := choice.Message.JSON.ExtraFields["reasoning_content"]; ok {
    raw := field.Raw()
    // raw is a JSON-encoded string, strip quotes
    var reasoning string
    json.Unmarshal([]byte(raw), &reasoning)
    if reasoning != "" {
        msg.Thinking = reasoning
    }
}

// Fallback: extract <think> tags from content
var thinkRegex = regexp.MustCompile(`(?s)<think>(.*?)</think>`)
if msg.Thinking == "" {
    if matches := thinkRegex.FindStringSubmatch(msg.Content); len(matches) > 1 {
        msg.Thinking = strings.TrimSpace(matches[1])
        msg.Content = strings.TrimSpace(thinkRegex.ReplaceAllString(msg.Content, ""))
    }
}
```

### Pattern 6: Message Conversion (Canonical -> OpenAI SDK)
**What:** Convert `model.Message` to `openai.ChatCompletionMessageParamUnion` using SDK helper functions.
**When to use:** Building the params for every API call.
**Example:**
```go
// Source: openai-go SDK message helpers
func toOpenAIMessages(msgs []model.Message) []openai.ChatCompletionMessageParamUnion {
    out := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs))
    for _, m := range msgs {
        switch m.Role {
        case "system":
            out = append(out, openai.SystemMessage(m.Content))
        case "user":
            out = append(out, openai.UserMessage(m.Content))
        case "assistant":
            if len(m.ToolCalls) > 0 {
                // Assistant message with tool calls needs special handling
                param := openai.ChatCompletionAssistantMessageParam{
                    Content: openai.ChatCompletionAssistantMessageParamContentUnion{
                        OfString: openai.String(m.Content),
                    },
                }
                for _, tc := range m.ToolCalls {
                    argsJSON, _ := json.Marshal(tc.Function.Arguments)
                    param.ToolCalls = append(param.ToolCalls, openai.ChatCompletionMessageToolCallParam{
                        ID:   tc.ID,
                        Type: "function",
                        Function: openai.ChatCompletionMessageToolCallFunctionParam{
                            Name:      tc.Function.Name,
                            Arguments: string(argsJSON),
                        },
                    })
                }
                out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &param})
            } else {
                out = append(out, openai.AssistantMessage(m.Content))
            }
        case "tool":
            out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
        }
    }
    return out
}
```

### Pattern 7: Tool Definition Conversion
**What:** Convert `model.ToolDefinition` to `openai.ChatCompletionToolUnionParam`.
**When to use:** Building params when tools are present.
**Example:**
```go
// Source: https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go
func toOpenAITools(tools []model.ToolDefinition) []openai.ChatCompletionToolUnionParam {
    out := make([]openai.ChatCompletionToolUnionParam, len(tools))
    for i, td := range tools {
        props := make(map[string]any)
        for name, prop := range td.Function.Parameters.Properties {
            p := map[string]any{"type": prop.Type[0]} // PropertyType is []string
            if prop.Description != "" {
                p["description"] = prop.Description
            }
            if len(prop.Enum) > 0 {
                p["enum"] = prop.Enum
            }
            props[name] = p
        }
        out[i] = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
            Name:        td.Function.Name,
            Description: openai.String(td.Function.Description),
            Parameters: openai.FunctionParameters{
                "type":       td.Function.Parameters.Type,
                "properties": props,
                "required":   td.Function.Parameters.Required,
            },
        })
    }
    return out
}
```

### Anti-Patterns to Avoid
- **Streaming when tools are present:** The standard OpenAI API streams tool call arguments as incremental chunks across multiple SSE events. Assembling these correctly requires ChatCompletionAccumulator and adds error-prone state management. Use non-streaming for tool calls.
- **Sending `reasoning_content` back in messages:** DeepSeek API returns 400 if `reasoning_content` is included in input messages. Strip it from outgoing messages -- only populate the `Thinking` field on canonical `model.Message` for display.
- **Hardcoding model names:** Different OpenAI-compatible providers serve different models. Never assume specific model names exist.
- **Requiring API key for local providers:** LM Studio and similar local providers do not need API keys. The constructor must handle empty API key gracefully.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SSE streaming | Custom HTTP + event parser | `client.Chat.Completions.NewStreaming()` | SDK handles reconnection, buffering, chunked transfer encoding |
| Tool call chunk assembly | Manual string concatenation of argument fragments | `ChatCompletionAccumulator` (for future use if streaming+tools needed) | SDK tracks indices, handles out-of-order chunks |
| JSON extra field access | Raw HTTP response parsing | `choice.Message.JSON.ExtraFields["reasoning_content"]` | SDK's `respjson.Field` type handles nulls, missing fields safely |
| OpenAI request construction | Manual JSON building | SDK typed params (`ChatCompletionNewParams`, `FunctionDefinitionParam`) | Type safety, correct serialization of optionals |
| Message role helpers | Manual struct construction | `openai.UserMessage()`, `openai.SystemMessage()`, `openai.ToolMessage()`, `openai.AssistantMessage()` | Correct union type construction |

**Key insight:** The openai-go SDK handles all the HTTP, SSE, and serialization complexity. The adapter's job is purely format translation between canonical types and SDK types.

## Common Pitfalls

### Pitfall 1: Tool Call Arguments Are Strings, Not Objects
**What goes wrong:** OpenAI returns `function.arguments` as a JSON string (e.g., `"{\"location\":\"NYC\"}"`) while Fenec canonical types use `map[string]any`. Forgetting to unmarshal produces a type error or empty args.
**Why it happens:** Ollama returns structured maps; OpenAI returns serialized JSON strings. Different wire formats.
**How to avoid:** Always `json.Unmarshal([]byte(tc.Function.Arguments), &args)` at the adapter boundary. Handle parse errors gracefully (log warning, pass raw string as `_raw` key).
**Warning signs:** Tool calls execute but arguments are always empty or nil.

### Pitfall 2: Empty API Key for Local Providers
**What goes wrong:** The SDK defaults to reading `OPENAI_API_KEY` env var. If it's not set and no key is provided, the SDK may error on client creation or add an empty `Authorization` header.
**Why it happens:** The SDK was designed for OpenAI cloud first; local providers like LM Studio accept any or no API key.
**How to avoid:** When `apiKey` is empty, set a dummy value like `"not-needed"` via `option.WithAPIKey("not-needed")`. LM Studio and Ollama's OpenAI-compat endpoint ignore the Authorization header.
**Warning signs:** "missing API key" error when connecting to local provider.

### Pitfall 3: Context Length Not Available via API
**What goes wrong:** The OpenAI `/v1/models` endpoint does NOT include context window size. `GetContextLength()` cannot query it like Ollama's Show API.
**Why it happens:** OpenAI's model listing returns only `id`, `object`, `created`, `owned_by` -- no metadata about context limits.
**How to avoid:** Return a reasonable default (e.g., 128000 for cloud OpenAI models, or 0 to signal "unknown/use model default"). The `ChatRequest.ContextLength` field is only used by Ollama's `num_ctx` option; OpenAI API handles context limits server-side.
**Warning signs:** N/A -- this is a design decision, not a runtime failure.

### Pitfall 4: Streaming Delta Content Type
**What goes wrong:** In streaming chunks, `delta.Content` is a string (not a pointer). Check for empty string, not nil.
**Why it happens:** openai-go v3 uses plain string fields on `ChatCompletionChunkChoiceDelta`, not `*string` pointers.
**How to avoid:** Check `delta.Content != ""` before calling `onToken`.
**Warning signs:** Empty strings passed to onToken, causing blank lines in output.

### Pitfall 5: Sending Thinking Content Back to DeepSeek
**What goes wrong:** If `reasoning_content` from a previous response is included in subsequent input messages, DeepSeek returns HTTP 400.
**Why it happens:** DeepSeek's API explicitly rejects `reasoning_content` in input.
**How to avoid:** The canonical `model.Message.Thinking` field is display-only. The `toOpenAIMessages` conversion function must NOT include thinking content in outgoing messages. This is already correct by design since `Thinking` has no mapping in the OpenAI message param types.
**Warning signs:** HTTP 400 errors on multi-turn conversations with DeepSeek R1.

### Pitfall 6: Usage/Metrics Differences
**What goes wrong:** OpenAI returns `CompletionUsage` with `PromptTokens`, `CompletionTokens`, `TotalTokens`. Ollama returns `Metrics` with `PromptEvalCount`, `EvalCount`. Mapping is not 1:1.
**Why it happens:** Different API designs.
**How to avoid:** Map `usage.PromptTokens` -> `StreamMetrics.PromptEvalCount` and `usage.CompletionTokens` -> `StreamMetrics.EvalCount`. For streaming, usage is in the final chunk (if `stream_options: {"include_usage": true}` is set) or unavailable.
**Warning signs:** Metrics always showing zeros.

## Code Examples

### Complete Provider Struct Skeleton
```go
// Source: Pattern from internal/provider/ollama/ollama.go adapted for openai-go
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
)

var _ provider.Provider = (*Provider)(nil)

type Provider struct {
    completions completionsAPI
    models      modelsAPI
}

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
        completions: client.Chat.Completions,
        models:      client.Models,
    }, nil
}

func (p *Provider) Name() string { return "openai" }
```

### CreateProvider Factory Extension
```go
// Source: internal/config/toml.go -- add to existing switch
case "openai":
    return openaiProvider.New(cfg.URL, cfg.APIKey)
```

### Non-Streaming with Tool Result Parsing
```go
// Full non-streaming flow for tool calls
func (p *Provider) chatWithTools(ctx context.Context, params sdkoai.ChatCompletionNewParams) (*model.Message, *model.StreamMetrics, error) {
    completion, err := p.completions.CreateChatCompletion(ctx, params)
    if err != nil {
        return nil, nil, fmt.Errorf("openai chat completion: %w", err)
    }
    if len(completion.Choices) == 0 {
        return nil, nil, fmt.Errorf("openai: no choices in response")
    }

    choice := completion.Choices[0]
    msg := &model.Message{
        Role:    "assistant",
        Content: choice.Message.Content,
    }

    // Parse tool calls
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

    // Extract thinking opportunistically
    extractThinking(msg, choice)

    // Map usage metrics
    metrics := &model.StreamMetrics{}
    if completion.Usage.PromptTokens > 0 {
        metrics.PromptEvalCount = int(completion.Usage.PromptTokens)
        metrics.EvalCount = int(completion.Usage.CompletionTokens)
    }

    return msg, metrics, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| sashabaranov/go-openai (community) | openai/openai-go/v3 (official) | Official SDK launched Feb 2025 | Type-safe, maintained by OpenAI, ExtraFields for non-standard providers |
| Manual SSE parsing | SDK `NewStreaming()` + iterator | v3.0.0 | No hand-rolled SSE parser needed |
| Separate streaming + accumulator for tools | Non-streaming fallback when tools present | Best practice for reliability | Simpler code, avoids chunked argument assembly bugs |
| openai.F() field wrapper | Direct field assignment (v3.x) | v3.0.0 | SDK moved to omitzero semantics; fields set directly, not wrapped |

**Deprecated/outdated:**
- `openai.F()` field wrapper: Was required in v2.x but v3.x uses Go 1.24+ `omitzero` JSON tag semantics. Fields are set directly on param structs.
- `openai.ChatModel*` string constants: Use plain string model names with OpenAI-compatible providers since model names vary by endpoint.

## Open Questions

1. **ListModels pagination for LM Studio**
   - What we know: OpenAI API uses `pagination.Page[Model]` return type. LM Studio supports `/v1/models` and returns all models.
   - What's unclear: Whether LM Studio returns paginated results or a single page. Most likely a single page for local providers.
   - Recommendation: Use auto-paging iterator (`ListAutoPaging`) which works for both single-page and multi-page responses.

2. **Streaming usage metrics**
   - What we know: Non-streaming responses include usage in `completion.Usage`. Streaming requires `stream_options: {"include_usage": true}` to get usage in the final chunk.
   - What's unclear: Whether LM Studio supports `stream_options`.
   - Recommendation: For streaming, return zero metrics initially. This matches Ollama's behavior when metrics aren't available. If needed, add `stream_options` support later.

3. **ExtraFields API stability**
   - What we know: `JSON.ExtraFields["reasoning_content"]` pattern works in v3.31.0.
   - What's unclear: Whether the `respjson.Field.Raw()` return type is stable across minor versions.
   - Recommendation: Use it for now; it's part of the official SDK's documented approach for non-standard fields.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed (stdlib testing) |
| Quick run command | `go test ./internal/provider/openai/ -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OAIC-01 | Chat with LM Studio via OpenAI protocol | unit | `go test ./internal/provider/openai/ -run TestStreamChat -v` | Wave 0 |
| OAIC-02 | Chat with OpenAI cloud models | unit | `go test ./internal/provider/openai/ -run TestNew -v` | Wave 0 |
| OAIC-03 | Tool calling with non-streaming fallback | unit | `go test ./internal/provider/openai/ -run TestToolCall -v` | Wave 0 |
| OAIC-04 | Provider switching mid-session | unit | `go test ./internal/config/ -run TestCreateProviderOpenAI -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/provider/openai/ -v -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/provider/openai/openai_test.go` -- covers OAIC-01 through OAIC-03
- [ ] `internal/config/toml_test.go` -- add TestCreateProviderOpenAI case (extends existing file)

## Sources

### Primary (HIGH confidence)
- [openai-go v3.31.0 on Go proxy](https://proxy.golang.org/github.com/openai/openai-go/v3/@v/v3.31.0.info) -- version and release date verified (2026-04-08)
- [openai-go GitHub repository](https://github.com/openai/openai-go) -- README, examples, type definitions
- [openai-go/v3 option package](https://pkg.go.dev/github.com/openai/openai-go/v3/option) -- WithBaseURL, WithAPIKey, WithMaxRetries documented
- [Chat completion tool calling example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go) -- Non-streaming tool flow verified
- [Chat completion accumulating example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-accumulating/main.go) -- ChatCompletionAccumulator pattern verified
- [Chat completion streaming example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-streaming/main.go) -- SSE streaming pattern verified
- [LM Studio OpenAI-compat docs](https://lmstudio.ai/docs/developer/openai-compat) -- endpoint compatibility confirmed

### Secondary (MEDIUM confidence)
- [openai-go ExtraFields pattern](https://github.com/openai/openai-go) -- JSON.ExtraFields for reasoning_content, verified in SDK docs
- [DeepSeek reasoning_content API docs](https://api-docs.deepseek.com/guides/reasoning_model) -- reasoning_content field format and restrictions
- [LM Studio tool use docs](https://lmstudio.ai/docs/developer/openai-compat/tools) -- streaming tool call support confirmed

### Tertiary (LOW confidence)
- OpenAI /v1/models does not expose context_length -- based on community reports and forum threads, not official OpenAI documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - openai-go v3.31.0 verified on Go proxy, examples verified in GitHub repo
- Architecture: HIGH - Adapter pattern mirrors existing Ollama adapter; SDK examples provide exact code patterns
- Pitfalls: HIGH - Tool argument JSON string format verified in SDK examples; reasoning_content behavior verified in DeepSeek docs
- Message conversion: MEDIUM - Helper functions (UserMessage, ToolMessage, etc.) confirmed in multiple sources but exact assistant-with-tool-calls param construction needs verification at implementation time

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (SDK is stable, 30-day window reasonable)
