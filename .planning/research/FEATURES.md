# Feature Landscape: Multi-Provider LLM Support

**Domain:** Multi-provider LLM integration for existing CLI AI agent platform
**Researched:** 2026-04-12
**Scope:** v1.1 milestone -- adding provider abstraction and OpenAI-compatible API support to Fenec

## Context: What Already Exists

Fenec v1.0 has a working agentic loop tightly coupled to the Ollama native API (`github.com/ollama/ollama/api`). The following are already implemented and working:

- `ChatService` interface with `StreamChat`, `ListModels`, `Ping`, `GetContextLength`
- `Conversation` struct using `[]api.Message` (Ollama types directly)
- Tool registry with `Tool` interface returning `api.Tool` definitions
- Agentic loop in REPL dispatching `api.ToolCall` objects
- Streaming with thinking support, context tracking, session persistence
- 8 built-in tools + Lua tool loading

**Key coupling points to Ollama types:**
- `api.Message` used in Conversation, Session, REPL, and all tool code
- `api.Tool` / `api.ToolFunction` / `api.ToolFunctionParameters` in every tool's `Definition()` method
- `api.ToolCall` / `api.ToolCallFunctionArguments` in Registry.Dispatch and Tool.Execute
- `api.ChatRequest` / `api.ChatResponse` / `api.Metrics` in Client/StreamChat
- `api.Tools` (slice type) passed through REPL to StreamChat

## Table Stakes

Features users expect when a CLI agent supports multiple providers. Missing = the multi-provider claim feels fake.

| Feature | Why Expected | Complexity | Dep on Existing Code |
|---------|--------------|------------|---------------------|
| Provider abstraction interface | Users expect `--model provider/model` to "just work" without knowing protocol details. Every multi-provider tool (LiteLLM, Continue, OpenCode, Hermes) has a provider interface. | High | Must replace or wrap every use of `api.Message`, `api.Tool`, `api.ToolCall` with provider-neutral types. The `ChatService` interface is a good starting point but its methods use Ollama types. |
| OpenAI-compatible API client | The OpenAI `/v1/chat/completions` format is the lingua franca. LM Studio, Ollama's compat endpoint, OpenRouter, vLLM, and dozens of others speak it. Supporting it covers 90% of use cases. | High | Need a new client implementing the provider interface using `github.com/openai/openai-go/v3` or raw HTTP. Must handle streaming via SSE with delta chunks (different from Ollama's streaming format). |
| Config-driven provider definitions | Users expect to add providers via config file, not code changes. Standard format: name, type, URL, API key, optional model overrides. Every multi-provider tool does this. | Medium | Currently no config file for providers. Need TOML/YAML with provider sections. Must handle API key storage (env var references, not plaintext). |
| Unified model selection (`provider/model`) | Users expect to specify both provider and model in one string. The `provider/model` or `provider:model` syntax is standard across Hermes, OpenCode, Continue, and others. Fenec already has `--model` flag. | Medium | Existing `--model` flag passes model name directly to Ollama. Need to parse `provider/model`, route to correct provider, validate model exists on that provider. Must handle ambiguity (e.g., `ollama/gemma4:latest` where `:` is an Ollama tag, not a provider separator). |
| Tool calling across providers | The whole point of Fenec is tool use. If switching providers breaks tool calling, the multi-provider support is useless. Users expect identical tool behavior regardless of provider. | High | Tools currently return `api.Tool` (Ollama type). Need provider-neutral tool definitions that translate to both Ollama native format and OpenAI format. The formats are similar but differ in specifics (Ollama uses `api.NewToolPropertiesMap()` with ordered maps; OpenAI uses `map[string]any` JSON schema). |
| Streaming responses from all providers | Fenec streams by default. If a new provider doesn't stream, it feels broken. Users expect the same streaming experience regardless of backend. | High | Ollama native streaming uses a callback function (`api.ChatResponseFunc`). OpenAI streaming uses SSE with `data:` prefixed JSON chunks and a `ChatCompletionAccumulator`. Need a unified streaming interface that normalizes both into the same token-callback pattern. |
| Provider health checks | When a provider is unreachable, show a clear error with the provider name and URL, not a generic connection failure. Users with multiple providers need to know which one failed. | Low | Existing `Ping()` method on `ChatService` is provider-agnostic already. Each provider implements its own health check. |
| Graceful fallback when provider lacks features | Not all providers support thinking/reasoning, context length queries, or model listing. Users expect the agent to work with reduced features rather than crash. | Medium | `GetContextLength` and `ListModels` may not be available on all OpenAI-compatible endpoints. Need sensible defaults and feature detection. |

## Differentiators

Features that would make Fenec's multi-provider support notably better than alternatives. Not expected, but valuable.

| Feature | Value Proposition | Complexity | Dep on Existing Code |
|---------|-------------------|------------|---------------------|
| Model discovery from providers | Auto-list available models from each configured provider. Unlike tools that require manual model lists, Fenec queries providers directly. Ollama has `/api/tags`, OpenAI-compat has `/v1/models`. | Low | Existing `ListModels` in `ChatService` does this for Ollama. OpenAI-compat endpoint supports `/v1/models` too. LM Studio and Ollama both serve this. |
| Provider-specific feature negotiation | Detect what each provider supports (thinking, tool calling, streaming+tools combo) and adapt behavior. E.g., disable thinking for providers that don't support it, fall back to non-streaming for OpenAI-compat endpoints where streaming+tools is broken. | Medium | The Ollama native API supports streaming+tool calling simultaneously. The Ollama OpenAI-compatible endpoint has documented issues with streaming+tools (tool calls silently dropped). Need per-provider feature flags. |
| Default provider with seamless upgrade | Keep Ollama native as the default zero-config experience. Users only touch provider config when they want LM Studio/OpenAI/other. Existing `fenec` command works exactly as before. | Low | Current code already defaults to Ollama at localhost:11434. Multi-provider is additive, not replacement. The default provider should be implicit Ollama native. |
| Session portability across providers | Conversation history works across provider switches. Start with Ollama, switch to LM Studio mid-session, continue the conversation. | Medium | Currently `Conversation` uses `[]api.Message` (Ollama types). If provider-neutral types are introduced, sessions saved with Ollama types need migration or the neutral format must be the persistence format. |
| Config hot-reload | Change provider config without restarting Fenec. Add a new provider, modify a URL, update an API key. | Low | No config reload currently. A `/providers` REPL command to list/check providers and a file watcher or explicit reload command. |
| `/provider` REPL command | Interactive provider switching within a session, similar to existing `/model` command. | Low | Follows the same pattern as `handleModelCommand()` in REPL. List providers, show current, allow selection. |

## Anti-Features

Features to explicitly NOT build for the multi-provider milestone.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Universal provider abstraction (Anthropic, Google, etc.) | Each provider has a different API shape (Anthropic uses XML-ish tool results, Google Gemini has a completely different format). Supporting all of them requires per-provider adapters with significant testing burden. The OpenAI-compatible format already covers Ollama, LM Studio, OpenRouter, vLLM, text-generation-inference, and most local inference servers. | Support exactly two protocols: Ollama native and OpenAI-compatible. This covers all the providers in PROJECT.md scope. Add specific provider protocols only if demand justifies the maintenance cost. |
| LangChain-style provider chain / fallback routing | Automatic failover between providers, load balancing, retry with different provider. This is infrastructure for production services, not a personal CLI agent. Adds enormous complexity for a single-user tool. | Manual provider selection via `--model provider/model`. If one provider is down, the user picks another. Simple and predictable. |
| API key management UI / keychain integration | Secure credential storage with OS keychain, encrypted config, key rotation. Over-engineered for a personal tool. | API keys in config file referencing environment variables (e.g., `api_key = "$OPENAI_API_KEY"`). Env vars are the standard for CLI tools. Warn if plaintext keys appear in config. |
| Provider-specific model parameter tuning | Per-provider temperature, top_p, frequency_penalty mappings. Different providers have different parameter ranges and defaults. Normalizing them is a rabbit hole. | Pass parameters through to the provider as-is. If a parameter isn't supported, the provider ignores it or errors. Document which parameters each provider type supports. |
| OpenAI Responses API support | OpenAI is pushing the Responses API as the successor to Chat Completions, but it's OpenAI-specific, not an interop standard. No other provider implements it. | Stick with Chat Completions API (`/v1/chat/completions`) which is the universal compatibility target. |
| Streaming tool calls via OpenAI-compatible endpoint | Ollama's OpenAI-compatible endpoint silently drops tool calls when streaming is enabled (documented issue as of 2026). Trying to work around this creates fragile provider-specific code. | For OpenAI-compatible providers, disable streaming when tools are present in the request. Use streaming only for pure chat (no tools). For Ollama native, streaming+tools works fine and should remain the default. |

## Feature Dependencies

```
Provider-Neutral Types (Message, Tool, ToolCall)
  -> Provider Interface (abstracts ChatService per-provider)
     -> Ollama Native Provider (wraps existing Client)
     -> OpenAI-Compatible Provider (new, uses openai-go or raw HTTP)
        -> Streaming via SSE
        -> Tool call translation (provider-neutral <-> OpenAI format)
        -> Non-streaming fallback for tools (OpenAI-compat streaming+tools bug)
  -> Config System (provider definitions in TOML)
     -> API key from env vars
     -> Provider URL + type + model overrides
  -> Unified Model Selection (--model provider/model parsing)
     -> Provider routing (parse provider prefix, dispatch to correct provider)
     -> Model discovery from providers (/v1/models, /api/tags)
  -> Session Portability (neutral message types in persistence)

Tool Definition Translation
  -> Provider-neutral tool definitions (not api.Tool)
  -> Translation to Ollama api.Tool format
  -> Translation to OpenAI ChatCompletionToolUnionParam format
  -> Tool.Execute stays the same (args are already just key-value maps)

Existing Tool System (UNCHANGED)
  -> Registry, Dispatch, built-in tools, Lua tools all work as-is
  -> Only the Definition() return type changes
  -> Execute() signature may change from api.ToolCallFunctionArguments to map[string]any
```

## Critical Type Mapping: Ollama Native vs OpenAI-Compatible

This is the core technical challenge. Both formats express the same concepts but with different type shapes.

### Tool Definitions (Request Side)

| Concept | Ollama Native (`api.Tool`) | OpenAI Chat Completions |
|---------|---------------------------|------------------------|
| Wrapper | `api.Tool{Type: "function", Function: api.ToolFunction{...}}` | `{"type": "function", "function": {...}}` |
| Name | `Function.Name` | `function.name` |
| Description | `Function.Description` | `function.description` |
| Parameters | `api.ToolFunctionParameters{Type: "object", Properties: OrderedMap, Required: []string}` | `{"type": "object", "properties": map[string]any, "required": [...]}` |
| Properties | `api.NewToolPropertiesMap()` (ordered map with `.Set()`) | Standard JSON Schema `map[string]any` |

### Messages

| Concept | Ollama Native (`api.Message`) | OpenAI Chat Completions |
|---------|------------------------------|------------------------|
| Role | `Role string` ("system", "user", "assistant", "tool") | Same roles |
| Content | `Content string` | `content string` (or array for multimodal) |
| Tool calls | `ToolCalls []api.ToolCall` on assistant message | `tool_calls []` on assistant message |
| Tool result | `Role: "tool"`, `ToolCallID string`, `Content string` | `role: "tool"`, `tool_call_id string`, `content string` |
| Thinking | `Thinking string` field | Not standardized (provider-specific) |

### Tool Calls (Response Side)

| Concept | Ollama Native (`api.ToolCall`) | OpenAI Chat Completions |
|---------|-------------------------------|------------------------|
| ID | `ID string` | `id string` (e.g., "call_xyz123") |
| Function name | `Function.Name` | `function.name` |
| Arguments | `Function.Arguments` (ordered map, `.Get(key)` method) | `function.arguments` (JSON string, must `json.Unmarshal`) |
| Type | (implicit) | `type: "function"` |

### Streaming

| Concept | Ollama Native | OpenAI Chat Completions |
|---------|--------------|------------------------|
| Format | Callback function `func(api.ChatResponse) error` | SSE stream, `data:` prefixed JSON chunks |
| Content | `resp.Message.Content` | `chunk.Choices[0].Delta.Content` |
| Tool calls | `resp.Message.ToolCalls` in pre-Done chunk | Delta tool_calls accumulated across chunks |
| Done signal | `resp.Done == true` with Metrics | `finish_reason: "stop"` or `"tool_calls"` |
| Metrics | `resp.Metrics` (PromptEvalCount, EvalCount, etc.) | `usage` object (prompt_tokens, completion_tokens) |
| Accumulation | Manual (current code uses `strings.Builder`) | `ChatCompletionAccumulator` helper in openai-go |

## MVP Recommendation for v1.1

Prioritize in this order:

1. **Provider-neutral types** -- Define `Message`, `ToolDefinition`, `ToolCall`, `StreamChunk` types that don't import `github.com/ollama/ollama/api`. This is the foundation everything else depends on.

2. **Provider interface** -- Define the `Provider` interface (analogous to current `ChatService` but returning neutral types). Include `StreamChat`, `ListModels`, `Ping`, and feature flags.

3. **Ollama native provider** -- Wrap existing `Client` code as a Provider. Translate between neutral types and `api.*` types. All existing functionality preserved. This is the "keep what works" step.

4. **OpenAI-compatible provider** -- New provider using `openai-go/v3` with `option.WithBaseURL`. Non-streaming for tool calls (streaming+tools is broken on many compat endpoints). Streaming for pure chat.

5. **Config system** -- TOML file with provider definitions. Default Ollama provider implicit. API key via env var references.

6. **Unified `--model provider/model` routing** -- Parse the prefix, resolve to provider, pass model name. Fall back to default provider if no prefix.

Defer:
- **Session migration**: Existing sessions can be invalidated or auto-migrated. Not blocking.
- **Provider REPL command**: Nice but `/model` already works within a provider. Add after core works.
- **Config hot-reload**: Restart is fine for config changes initially.
- **Feature negotiation**: Start with manual provider type flags. Auto-detection later.

## Sources

- [Ollama OpenAI compatibility docs](https://docs.ollama.com/api/openai-compatibility) -- supported/unsupported features (HIGH confidence)
- [LM Studio tool calling docs](https://lmstudio.ai/docs/developer/openai-compat/tools) -- format and limitations (HIGH confidence)
- [openai-go examples](https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go) -- tool calling types (HIGH confidence)
- [openai-go streaming accumulator](https://github.com/openai/openai-go/blob/main/examples/chat-completion-accumulating/main.go) -- streaming pattern (HIGH confidence)
- [Ollama streaming+tools issue](https://github.com/ollama/ollama/issues/12557) -- OpenAI-compat endpoint drops tool calls when streaming (MEDIUM confidence)
- [OpenAI Chat Completions API reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create) -- tool_calls format (HIGH confidence)
- [Mozilla any-llm-go](https://blog.mozilla.ai/run-openai-claude-mistral-llamafile-and-more-from-one-interface-now-in-go/) -- provider abstraction patterns in Go (MEDIUM confidence)
- [Hermes agent model selection](https://deepwiki.com/NousResearch/hermes-agent/2.3-model-and-provider-selection) -- provider:model syntax patterns (MEDIUM confidence)
- [OpenCode provider configuration](https://opencode.ai/docs/providers/) -- config patterns (MEDIUM confidence)
- [Ollama native vs OpenAI-compat analysis](https://openclaw-ai.com/en/docs/providers/ollama/) -- capability comparison (MEDIUM confidence)
- [openai-go option.WithBaseURL](https://pkg.go.dev/github.com/openai/openai-go/option) -- custom endpoint configuration (HIGH confidence)
- [LiteLLM config format](https://docs.litellm.ai/docs/proxy/configs) -- multi-provider config patterns (MEDIUM confidence)
