# Phase 10: OpenAI-Compatible Client - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement a new Provider adapter for the OpenAI-compatible protocol (`/v1/chat/completions`). Supports LM Studio, OpenAI cloud, and any other compatible endpoint. Handles streaming for pure chat, non-streaming for tool calls (workaround for the known streaming+tools limitation). Tool call format translation happens at the adapter boundary. Thinking/reasoning is parsed opportunistically from response fields/tags when present. After this phase, users can chat and use tools with any OpenAI-compatible provider alongside the existing Ollama provider.

</domain>

<decisions>
## Implementation Decisions

### Streaming & Tool Calls

- When tools are present in the request: fall back to non-streaming request, then call `onToken` once with full content after completion
- When no tools: stream normally via SSE with `onToken` per delta chunk
- Chunked tool call arguments are assembled via openai-go SDK's `ChatCompletionAccumulator` helper
- Non-streaming tool call flow: full response → parse tool_calls array → return as `model.Message` with ToolCalls populated

### Tool Call Format Translation

- OpenAI returns tool arguments as a JSON string in `function.arguments` → parse to `map[string]any` at adapter boundary
- Tool results sent back: `role: "tool"`, `tool_call_id: <id>`, `content: <result>` (format already matches canonical)
- Tool call ID: use OpenAI's `call_xyz` ID verbatim on canonical `model.ToolCall.ID`
- Canonical `model.ToolDefinition` → OpenAI `ChatCompletionToolUnionParam` conversion happens in adapter only
- `model.Message.ToolCallID` already maps cleanly to OpenAI's `tool_call_id` field

### Provider Switching & Session Continuity

- Phase 10 focuses only on making OpenAI adapter a functional Provider — actual switching UX is Phase 11 scope
- Conversation history preserved as-is across provider switches (canonical types work universally)
- Tools remain registered even if the active model doesn't support tool calling; user warned at model selection time (Phase 11)
- No `/provider` REPL command — Phase 11 uses `/model provider/model` and `--model provider/model`

### Thinking/Reasoning for OpenAI-Compat

- Strategy: opportunistic parsing — no `think` flag sent in request (not in standard OpenAI API)
- Parse `reasoning_content` field from response if present (DeepSeek R1 via LM Studio, some other models)
- Parse `<think>...</think>` tags embedded in content if present, extract to `Thinking` field
- When neither is present, Thinking stays empty — normal chat response

### Claude's Discretion

- Exact SDK initialization pattern (openai-go client construction)
- How the adapter is registered in the `CreateProvider` factory (likely `case "openai":`)
- Error message wording for missing API key, failed requests, etc.
- Internal struct shape of the OpenAI adapter
- Timeout/retry/backoff defaults for HTTP requests
- Whether `<think>` tag parsing uses a streaming parser or post-hoc regex

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/provider/provider.go` — Provider interface and ChatRequest struct (ready for new adapter)
- `internal/provider/ollama/ollama.go` — Reference implementation to pattern the new adapter after
- `internal/model/` — Canonical types (Message, ToolDefinition, ToolCall, ToolCallFunction, StreamMetrics)
- `internal/config/toml.go` — `CreateProvider` factory has a switch statement that needs a new `case "openai":`
- `internal/config/toml_test.go` — tests for factory already cover pattern

### Established Patterns
- Provider constructor: `New(cfg ProviderConfig) (*Provider, error)` — signature may need to expand for API key
- Compile-time interface check: `var _ provider.Provider = (*Provider)(nil)`
- All Provider methods take context.Context first
- Fresh-style streaming with `onToken` / `onThinking` callbacks
- Test pattern: mock HTTP server + testify

### Integration Points
- `internal/provider/openai/openai.go` (new package) for the adapter
- `internal/config/toml.go` `CreateProvider` switch — add `"openai"` case
- `ProviderConfig` struct already has `Type`, `URL`, `APIKey`, `DefaultModel` fields — sufficient
- `main.go` doesn't change — config-driven factory handles provider creation

### New Dependency
- `github.com/openai/openai-go/v3` at v3.31.0 (already recommended in research)
- Run `go get github.com/openai/openai-go/v3@v3.31.0`

</code_context>

<specifics>
## Specific Ideas

Example config usage after this phase:
```toml
default_provider = "ollama"

[providers.ollama]
type = "ollama"
url = "http://localhost:11434"

[providers.lmstudio]
type = "openai"
url = "http://localhost:1234/v1"

[providers.openai]
type = "openai"
url = "https://api.openai.com/v1"
api_key = "$OPENAI_API_KEY"
```

Reasoning content extraction priority order:
1. Response's `reasoning_content` field (if SDK exposes it via extra fields)
2. `<think>...</think>` regex match in content → extract to Thinking, strip from Content
3. Empty Thinking field

</specifics>

<deferred>
## Deferred Ideas

- `/provider` REPL command — explicitly rejected, `/model` handles everything in Phase 11
- Provider health dashboard — deferred to future milestone
- Multimodal support (images in messages) — not in v1.1 scope
- Provider-specific parameter tuning (temperature, top_p) — out of scope per REQUIREMENTS.md
- Automatic failover / retry with different provider — out of scope per REQUIREMENTS.md

</deferred>
