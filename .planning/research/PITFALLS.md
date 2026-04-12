# Domain Pitfalls: Multi-Provider LLM Support

**Domain:** Adding multi-provider support (Ollama native + OpenAI-compatible) to existing Ollama-coupled Go application
**Researched:** 2026-04-12
**Confidence:** HIGH (verified against codebase audit, Ollama API types, OpenAI API spec, and real-world Go multi-provider projects)

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

### Pitfall 1: 228 Ollama Type References -- The Shotgun Decoupling Problem

**What goes wrong:**
The codebase has 228 references to `api.Message`, `api.Tool`, `api.ToolCall`, `api.ChatRequest`, `api.ChatResponse`, `api.Metrics`, and related Ollama types across 29 Go files. Every package -- chat, tool, session, repl, lua, config -- imports `github.com/ollama/ollama/api` directly. Developers attempt to add a provider interface by wrapping the Ollama client, but the Ollama types remain in every function signature, struct field, and test mock throughout the codebase. The "abstraction" becomes a thin veneer over Ollama -- the second provider (OpenAI-compatible) must either convert to/from Ollama types at every boundary, or you face a massive multi-file refactor that touches every package simultaneously.

**Why it happens:**
This is the classic "coupled by types, not just calls" problem. The coupling is not in the 1 place that calls the Ollama API -- it is in the 29 files that use Ollama's *data types* as their lingua franca. Specifically in this codebase:
- `chat.Conversation` stores `[]api.Message` -- every message add/read operation uses Ollama types
- `session.Session` persists `[]api.Message` to JSON -- serialized sessions are in Ollama wire format
- `tool.Tool` interface returns `api.Tool` and accepts `api.ToolCallFunctionArguments`
- `tool.Registry` returns `api.Tools` and dispatches via `api.ToolCall`
- `chat.ChatService` interface exposes `api.Tools`, `*api.Message`, `*api.Metrics` in its signatures
- `repl.REPL` directly accesses `msg.ToolCalls`, `tc.Function.Name`, `tc.Function.Arguments`, `tc.ID`

**Consequences:**
Without addressing this first, every subsequent provider-related change becomes a "change one thing, fix 29 files" exercise. Tests break across every package. The provider abstraction leaks Ollama assumptions everywhere.

**Prevention:**
1. Define your own canonical message types first, before writing any provider code: `fenec.Message`, `fenec.ToolCall`, `fenec.ToolDefinition`, `fenec.StreamMetrics`. Keep them in an internal package with zero external dependencies.
2. Make the conversion boundary explicit: Ollama provider converts `fenec.Message` to/from `api.Message` in exactly one place. OpenAI provider converts `fenec.Message` to/from OpenAI types in exactly one place.
3. Migrate the codebase to the canonical types in a dedicated phase BEFORE adding the second provider. This is the hardest part but doing it while also adding OpenAI support doubles the cognitive load and bug surface.
4. Accept that this is the single largest change in the milestone -- it touches 29 files and every test. Plan accordingly.

**Detection:**
- `grep -r "github.com/ollama/ollama/api" --include="*.go" | wc -l` shows 29+ files importing Ollama types
- Any file outside `internal/provider/ollama/` that imports `ollama/api` is a coupling point

**Phase to address:** Phase 1 (canonical types). This must be the very first phase. Everything else depends on it.

---

### Pitfall 2: Tool Call Arguments -- String vs Object Mismatch Breaks Multi-Turn

**What goes wrong:**
OpenAI returns tool call arguments as a **JSON string** (`"arguments": "{\"command\": \"ls\"}"`) while Ollama returns them as a **parsed JSON object** (`"arguments": {"command": "ls"}`). Ollama's `api.ToolCallFunctionArguments` is a custom ordered-map type with `Get(key)` accessors. OpenAI's `openai-go` returns arguments as a raw JSON string that must be unmarshaled by the caller. When you build a unified interface, the arguments type must accommodate both formats. If you get this wrong, multi-turn tool calling breaks: passing a string-encoded arguments back to Ollama causes `"json: cannot unmarshal string into Go struct field ChatRequest.messages.tool_calls.function.arguments of type api.ToolCallFunctionArguments"`. Passing an object to an OpenAI-compatible endpoint that expects a string causes the reverse failure.

**Why it happens:**
This is a known, documented incompatibility. Ollama's native API was designed for structured tool calling where arguments are already parsed. OpenAI's API treats arguments as opaque JSON strings because their streaming format sends partial argument JSON across chunks. The two approaches are fundamentally different at the wire level, and the conversion is not symmetric -- you lose ordering information when converting Ollama's ordered map to a plain `map[string]any`, and you gain parsing overhead when converting OpenAI's string to a structured type.

**Consequences:**
- Multi-turn conversations with tool calls fail silently or with cryptic JSON errors
- Tool results cannot be fed back correctly if the message history contains the wrong arguments format
- Session persistence (which currently serializes `api.Message` including `ToolCalls`) becomes provider-dependent

**Prevention:**
1. Canonical `ToolCall` type should store arguments as `map[string]any` (parsed, unordered). This is the lowest common denominator both formats can convert to/from.
2. Each provider's conversion layer must handle: Ollama ordered-map to `map[string]any` (use `ToMap()`), and OpenAI JSON string to `map[string]any` (use `json.Unmarshal`).
3. When converting back for API calls: Ollama provider must reconstruct `ToolCallFunctionArguments` using `Set()` from the map. OpenAI provider must `json.Marshal` the map back to a string.
4. Test the full round-trip: model returns tool call -> dispatch tool -> append result to history -> send history back -> model sees correct history. Test this for BOTH providers.

**Detection:**
- Tool calling works for the first turn but fails on subsequent turns
- JSON unmarshal errors mentioning `ToolCallFunctionArguments`
- Provider works in isolation but breaks when switching providers mid-session

**Phase to address:** Phase 1 (canonical types) for the type definition, Phase 2 (provider implementation) for the conversion round-trip testing.

---

### Pitfall 3: Session Serialization Backward Compatibility Breaks

**What goes wrong:**
Existing saved sessions (in `~/.config/fenec/sessions/`) contain `[]api.Message` serialized as JSON with Ollama's wire format. This includes fields like `thinking` (Ollama-specific), `tool_calls` with Ollama's nested structure, `tool_call_id`, and `tool_name`. When you switch to canonical `fenec.Message` types, existing session files cannot be deserialized into the new types without a migration layer. Users lose their saved sessions, or worse, the application panics on startup when auto-save loads an incompatible session.

**Why it happens:**
The `session.Session` struct directly embeds `[]api.Message` and uses `encoding/json` for persistence. The JSON field names and structure are Ollama's wire format. Changing to canonical types changes the JSON schema. There is no version field in the session format to enable migration detection.

**Consequences:**
- Auto-save file from previous version crashes new version on startup
- Named saved sessions become unloadable
- Users lose conversation history (especially painful for the "personal assistant" use case where history has accumulated value)

**Prevention:**
1. Add a `"version": 1` field to the session JSON format NOW, before changing anything else. This costs almost nothing and enables future migrations.
2. Implement a migration function: `migrateV1ToV2(oldJSON) -> newJSON` that converts Ollama-format messages to canonical format. Run this transparently during `Store.Load()`.
3. Design canonical message types with JSON tags that match common conventions (not Ollama-specific, not OpenAI-specific). Use `role`, `content`, `tool_calls`, `tool_call_id` -- these happen to be shared between both APIs.
4. Keep a `Provider` field in the session metadata so loaded sessions can be associated with the correct provider for any provider-specific reconstruction needed.

**Detection:**
- App crashes on `/load` or auto-save restore after upgrade
- `json.Unmarshal` errors in session loading
- Silent data loss where tool call history in loaded sessions is empty

**Phase to address:** Phase 1, specifically before the type migration. Add version field first, then migrate.

---

### Pitfall 4: Streaming Format Impedance Mismatch

**What goes wrong:**
Ollama streams NDJSON (one JSON object per line) with a callback pattern (`func(api.ChatResponse) error`). OpenAI-compatible endpoints stream SSE (`data: {json}\n\n`) with `data: [DONE]` termination. The current `StreamChat` implementation is deeply coupled to Ollama's callback pattern and `api.ChatResponse` fields (`resp.Message.Content`, `resp.Message.Thinking`, `resp.Message.ToolCalls`, `resp.Done`, `resp.Metrics`). An OpenAI-compatible client cannot use this same streaming pathway because:
- OpenAI streams use `choices[0].delta.content` (not `message.content`)
- OpenAI thinking uses `choices[0].delta.reasoning_content` (not `message.thinking`)
- OpenAI tool calls stream as partial JSON across chunks with index-based assembly
- OpenAI completion signals via `finish_reason: "stop"` (not a boolean `done` flag)
- OpenAI token usage is in `usage.prompt_tokens` / `usage.completion_tokens` (not Ollama `Metrics`)

**Why it happens:**
Streaming is inherently provider-specific at the wire level. The stream parsing logic (SSE vs NDJSON), chunk structure, and signaling conventions are different. The temptation is to try to normalize at the stream-reading level, but the real complexity is in the chunk assembly -- especially for tool calls, which arrive as partial JSON across multiple SSE events in OpenAI format but as complete objects in Ollama format.

**Consequences:**
- Duplicated streaming logic across providers
- Subtle bugs where tool calls are dropped during streaming because the assembly logic differs
- Thinking/reasoning content handled incorrectly for one provider
- Metrics/token counting broken for non-Ollama providers

**Prevention:**
1. Define a canonical streaming callback: `func(chunk StreamChunk) error` where `StreamChunk` has `Content string`, `Thinking string`, `ToolCalls []ToolCall` (fully assembled), `Done bool`, `Metrics StreamMetrics`.
2. Each provider is responsible for parsing its wire format and emitting canonical `StreamChunk` values. The chunk assembly (especially for OpenAI's partial tool call JSON) happens inside the provider, not in shared code.
3. The REPL/consumer code ONLY sees `StreamChunk` -- it never touches wire-level types.
4. For OpenAI tool call streaming: accumulate partial argument strings by index, only emit a `ToolCall` in the chunk when the arguments are complete. This is the trickiest part and must be provider-internal.

**Detection:**
- Tool calls work in non-streaming mode but break in streaming mode for one provider
- Thinking/reasoning output appears for Ollama but not OpenAI (or vice versa)
- Token counts are zero or wrong for one provider

**Phase to address:** Phase 2 (provider implementations). The canonical `StreamChunk` type should be defined in Phase 1 alongside other canonical types.

---

### Pitfall 5: Context Length Discovery Has No Universal API

**What goes wrong:**
Fenec currently uses Ollama's `Show` API to query `model_info.*.context_length`, which returns the model's maximum context window. This is essential for the `ContextTracker` that manages truncation. OpenAI's API has no equivalent endpoint -- there is no way to query a model's context length via the API. LM Studio's OpenAI-compatible endpoint also does not expose this. Without context length, the tracker either uses a hardcoded fallback (4096 -- dangerously low for modern models) or has to be configured manually per model. If the fallback is too low, aggressive truncation kicks in and drops messages too early. If too high, the provider returns context-exceeded errors.

**Why it happens:**
This is a genuine API gap, not an implementation oversight. OpenAI intentionally does not expose context limits via API (they've had open feature requests for years with no action). Each provider has different capabilities for model introspection. Ollama is unusually generous here; most OpenAI-compatible providers give you nothing.

**Consequences:**
- Context tracker breaks for non-Ollama providers (uses 4096 fallback)
- Users experience either premature truncation (fallback too low) or context exceeded errors (fallback too high)
- Each model from each provider potentially has different context limits, creating a matrix of hardcoded values to maintain

**Prevention:**
1. Make context length a configurable property per provider+model in the config file. Example: `providers.lmstudio.models.gemma4.context_length = 32768`.
2. Implement a capability-based discovery: provider interface has `GetContextLength(model) (int, bool)` where the bool indicates "is this a known value or a guess?" Ollama provider queries Show API. OpenAI provider checks config overrides. If neither knows, return a sensible default (8192) with a warning.
3. Ship a built-in lookup table for well-known models (GPT-4o = 128K, Claude = 200K, Gemma 4 = 128K) as a fallback.
4. Log a warning when using fallback values so users know to configure context length for optimal behavior.

**Detection:**
- Context tracking works perfectly with Ollama but produces wrong truncation behavior with other providers
- Messages being truncated much earlier than expected
- "Context length exceeded" errors from the provider

**Phase to address:** Phase 2 (provider implementations) for the discovery interface, Phase 3 (config system) for user-configurable overrides.

---

## Moderate Pitfalls

### Pitfall 6: The Leaky "Thinking" Abstraction

**What goes wrong:**
Ollama exposes thinking/reasoning via `Message.Thinking` field and `ChatRequest.Think` control. OpenAI uses `delta.reasoning_content` in streaming and `reasoning_effort` parameter for control. LM Studio may or may not support reasoning depending on the model. The current code has `conv.Think` as a boolean and reads `resp.Message.Thinking` during streaming. A naive abstraction maps these 1:1, but the semantics differ: Ollama's Think is a binary on/off toggle, OpenAI's `reasoning_effort` is a graduated control (`low`/`medium`/`high`), and some providers have no thinking support at all.

**Prevention:**
1. Canonical thinking support as `ThinkingMode` enum: `Off`, `On`, `Effort(level)`.
2. Provider converts to its native format. Ollama: `On` -> `Think: true`, `Effort(any)` -> `Think: true`. OpenAI: `On` -> `reasoning_effort: medium`, `Effort(level)` -> corresponding level.
3. Make thinking a provider capability that can be queried: `provider.SupportsThinking() bool`.

**Phase to address:** Phase 2 (provider implementations).

### Pitfall 7: Tool Definition Schema Differences Between Providers

**What goes wrong:**
Ollama's `api.Tool` uses `api.ToolPropertiesMap` (an ordered map with custom JSON) and `api.PropertyType` (a custom type, not a plain string). OpenAI expects standard JSON Schema for tool parameter definitions with `"type": "string"` as a plain string. The current `tool.Tool` interface returns `api.Tool` which means every built-in tool and every Lua tool is coded to Ollama's specific schema types. Converting between these requires handling the ordered-map wrapper and custom property types.

**Prevention:**
1. Canonical tool definition should use standard JSON Schema structures -- plain `map[string]any` or a simple struct with `Type string`. Both Ollama and OpenAI can consume standard JSON Schema.
2. Let each provider convert from canonical to its native format. Ollama provider wraps into `ToolPropertiesMap`. OpenAI provider passes through as-is.
3. The `tool.Tool` interface should return `fenec.ToolDefinition`, not `api.Tool`.

**Phase to address:** Phase 1 (canonical types), since the `tool.Tool` interface is a foundational type.

### Pitfall 8: Model Listing and Discovery Diverges Per Provider

**What goes wrong:**
Ollama lists models via `api.Client.List()` returning `ListResponse` with model names, sizes, and digests. OpenAI-compatible endpoints use `GET /v1/models` returning a different schema with `id`, `object`, `owned_by`. LM Studio uses the OpenAI format but with local model names. The current `ChatService.ListModels()` returns `[]string` (just names), which looks provider-agnostic but the model naming conventions differ: Ollama uses `gemma4:latest`, OpenAI uses `gpt-4o-2024-08-06`, LM Studio uses local filenames. The `--model provider/model` syntax requires routing logic that understands which names belong to which provider.

**Prevention:**
1. Model listing returns `[]ModelInfo{Name, Provider, DisplayName}` not just `[]string`.
2. Provider prefix is handled at the routing layer, not in the model name itself. Model names stay provider-native internally.
3. When user specifies `--model ollama/gemma4`, the router knows to use the Ollama provider with model "gemma4". When user specifies `--model lmstudio/deepseek-r2`, the router uses LM Studio provider with model "deepseek-r2".

**Phase to address:** Phase 3 (unified model selection and provider routing).

### Pitfall 9: Error Handling Semantics Differ Per Provider

**What goes wrong:**
Ollama returns errors as Go errors from the client library with provider-specific messages ("model not found", "context length exceeded"). OpenAI-compatible APIs return HTTP status codes with JSON error bodies (`{"error": {"message": "...", "type": "...", "code": "..."}}`). Rate limiting, authentication failures, model unavailability, and context overflow each have different error formats. Without normalization, error handling in the REPL becomes a mess of provider-specific `if` branches.

**Prevention:**
1. Define canonical error types: `ErrModelNotFound`, `ErrContextExceeded`, `ErrRateLimit`, `ErrAuth`, `ErrProviderUnavailable`.
2. Each provider maps its native errors to canonical errors in its conversion layer.
3. REPL handles only canonical errors with appropriate user-facing messages.

**Phase to address:** Phase 2 (provider implementations).

### Pitfall 10: The "OpenAI-Compatible" Assumption Trap

**What goes wrong:**
"OpenAI-compatible" does not mean "identical to OpenAI." LM Studio, Ollama's `/v1` endpoint, vLLM, and LocalAI each have their own deviations: LM Studio may fail to parse tool calls from smaller models (returns them in `content` instead of `tool_calls`), Ollama's `/v1` endpoint does not support `tool_choice`, some providers do not support `stream_options.include_usage` for token counting. Building one "OpenAI-compatible" client and assuming it works everywhere leads to subtle failures that only appear with specific providers.

**Prevention:**
1. Test with every provider you claim to support. "OpenAI-compatible" means "needs testing against this specific provider."
2. Build provider capability detection: can this provider do tool calling? Streaming? Thinking? Token usage reporting? Make these queryable booleans, not assumptions.
3. Degrade gracefully: if a provider does not report usage, skip context tracking for that provider. If tool calls come back in `content` instead of `tool_calls`, attempt to parse them from content as a fallback.

**Phase to address:** Phase 2 (provider implementations) for initial support, ongoing through Phase 3.

---

## Minor Pitfalls

### Pitfall 11: Ollama-Specific Features Lost in Abstraction

**What goes wrong:**
Ollama has features no other provider offers: `keep_alive` to prevent model unloading, `num_ctx` per-request, `Truncate` control, model pulling/management, `/api/ps` for running model inspection. A too-aggressive abstraction strips these out in the name of portability. Users who were happy with Ollama-specific behavior find the abstracted version worse.

**Prevention:**
Implement provider-specific options as an escape hatch. The canonical `ChatOptions` has common fields, plus a `ProviderOptions map[string]any` for pass-through. Ollama provider checks for `keep_alive`, `num_ctx` in this map. Other providers ignore them.

### Pitfall 12: The "Big Bang" Migration Temptation

**What goes wrong:**
Developers try to do the type migration (228 references), provider interface, OpenAI client, config system, and `--model` routing all in one massive PR. The PR becomes unreviewable, debugging is impossible because everything changed at once, and you end up in a state where neither the old Ollama path nor the new abstracted path works correctly.

**Prevention:**
1. Phase 1: Introduce canonical types and migrate internal code. At the end of Phase 1, only the Ollama provider exists but it goes through the abstraction.
2. Phase 2: Add OpenAI-compatible provider. At the end of Phase 2, both providers work but config is hardcoded/flag-driven.
3. Phase 3: Add config file, `--model` syntax, model discovery. 
Each phase is independently shippable and testable. 

### Pitfall 13: Test Mock Fragility

**What goes wrong:**
The current test suite mocks `chatAPI` (an interface matching `api.Client` methods) with Ollama-specific types in the mock responses. When canonical types replace Ollama types in interfaces, every mock in every test file needs updating. If you do this as part of the provider addition rather than as a separate step, you are debugging type conversion logic and test mock updates simultaneously.

**Prevention:**
The type migration (Phase 1) should include updating all test mocks to use canonical types. This is a mechanical change that should be committed and verified (all tests pass) before any provider logic is added.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Canonical types (Phase 1) | Trying to make canonical types match Ollama or OpenAI exactly | Design canonical types for YOUR domain, not either API. Both APIs convert to/from your types. |
| Canonical types (Phase 1) | Missing a field that one provider needs | Audit both Ollama Message and OpenAI ChatCompletionMessage fields completely before designing canonical type. Include `Images`, `Thinking`, `ToolCalls`, `ToolCallID`, `ToolName`. |
| Session migration (Phase 1) | Breaking existing sessions without migration path | Add version field to session format before changing message types. Write V1->V2 migrator. |
| Ollama provider (Phase 1) | Regression in existing Ollama behavior | Run full existing test suite after migration. Nothing should change in behavior, only in internal types. |
| OpenAI provider (Phase 2) | Tool call argument round-trip failure | Write integration tests that do: send tools -> model calls tool -> dispatch -> append result -> send back -> model responds. |
| OpenAI provider (Phase 2) | Streaming tool call assembly bugs | OpenAI streams partial tool call JSON across chunks by index. Build accumulator that waits for complete arguments. |
| Config system (Phase 3) | Config schema that does not accommodate future providers | Design config as `[providers.NAME]` sections with `type`, `url`, `api_key`, `models` fields. Provider type determines which client to instantiate. |
| Model routing (Phase 3) | Ambiguous model names across providers | Require `provider/model` syntax for disambiguation. Default provider configurable. Bare model names resolve to default provider only. |

## What Specifically Breaks in This Codebase

A concrete audit of which files need changes and what breaks:

| File | Current Ollama Coupling | What Must Change |
|------|------------------------|------------------|
| `chat/message.go` | `Conversation.Messages` is `[]api.Message`, all add methods create `api.Message` | Swap to `[]fenec.Message`, update all 6 methods |
| `chat/client.go` | `ChatService` interface exposes `api.Tools`, `*api.Message`, `*api.Metrics` | Replace with canonical types in interface signature |
| `chat/stream.go` | `StreamChat` constructs `api.ChatRequest`, reads `api.ChatResponse` fields, returns `*api.Message` | Move request construction into Ollama provider, stream through canonical `StreamChunk` |
| `chat/context.go` | `TruncateOldest` accesses `conv.Messages[i].Role` (works with any type that has Role) | Should need minimal change if canonical Message also has Role field |
| `tool/registry.go` | `Tool` interface returns `api.Tool`, accepts `api.ToolCallFunctionArguments`, `api.ToolCall` | Core interface change -- cascades to all 8 built-in tools and LuaTool |
| `tool/shell.go` | `Definition()` returns `api.Tool`, `Execute()` accepts `api.ToolCallFunctionArguments` | Update to canonical types (repeated for read.go, write.go, edit.go, listdir.go, create.go, update.go, delete.go) |
| `lua/luatool.go` | `Definition()` returns `api.Tool`, `Execute()` accepts `api.ToolCallFunctionArguments`, `ArgsToLuaTable` converts from Ollama args | Update to canonical types, update Lua bridge conversion |
| `session/session.go` | `Session.Messages` is `[]api.Message` | Swap to canonical type, add version field, write migration |
| `session/store.go` | Serializes `[]api.Message` via `json.Encoder` | No code change needed if canonical Message has compatible JSON tags; add migration on Load |
| `repl/repl.go` | Directly accesses `msg.ToolCalls`, `tc.Function.Name`, `tc.Function.Arguments`, `tc.ID`, `api.Tools` | Update field access to canonical type fields |
| `main.go` | `chat.NewClient(host)` directly creates Ollama client | Replace with provider factory: `provider.New(config)` |

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Attempted big-bang migration, code in broken state | HIGH | Git reset to last working commit. Restart with Phase 1 (types only). |
| Session files incompatible with new types | LOW | Write migration script, or add fallback JSON unmarshaling that tries both old and new format. |
| Tool call arguments round-trip broken for one provider | MEDIUM | Add provider-specific integration tests. Debug by printing raw JSON at conversion boundary. |
| Streaming broken for OpenAI provider | MEDIUM | Implement non-streaming fallback. Debug SSE parsing separately from chunk assembly. |
| Context tracking wrong for non-Ollama provider | LOW | Use conservative fallback (8192), add config override, log warning. |
| Provider "abstraction" that is just Ollama types with an interface wrapper | HIGH | This is the biggest risk. If discovered late, requires restarting the type migration. Catch early by ensuring the interface has NO Ollama imports. |

## Sources

- [Ollama API types (pkg.go.dev)](https://pkg.go.dev/github.com/ollama/ollama/api) -- Message, ToolCall, ToolCallFunctionArguments struct definitions (HIGH confidence)
- [Ollama OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility) -- supported/unsupported features matrix (HIGH confidence)
- [OpenAI function calling guide](https://developers.openai.com/api/docs/guides/function-calling) -- tool_calls format specification (HIGH confidence)
- [ToolCallFunctionArguments string vs object bug](https://github.com/aliasrobotics/cai/issues/76) -- JSON unmarshal mismatch between OpenAI string and Ollama object format (HIGH confidence)
- [Ollama tool_calls arguments format issue](https://github.com/openclaw/openclaw/issues/46679) -- arguments as string breaks multi-turn (HIGH confidence)
- [Mozilla any-llm-go](https://blog.mozilla.ai/run-openai-claude-mistral-llamafile-and-more-from-one-interface-now-in-go/) -- Go multi-provider abstraction patterns, OpenAI-compatible base provider (MEDIUM confidence)
- [Multi-provider LLM orchestration guide](https://dev.to/ash_dubai/multi-provider-llm-orchestration-in-production-a-2026-guide-1g10) -- common mistakes in multi-provider setups (MEDIUM confidence)
- [Same Beat, Different Synths (Mule AI)](https://muleai.io/blog/any-llm-go-mozilla-provider-abstraction/) -- provider abstraction design principles (MEDIUM confidence)
- [OpenAI model context length limitation](https://community.openai.com/t/request-query-for-a-models-max-tokens/161891) -- no API endpoint for context length (HIGH confidence)
- [LM Studio tool calling docs](https://lmstudio.ai/docs/developer/openai-compat/tools) -- tool calling support and limitations (HIGH confidence)
- [OpenAI streaming API](https://developers.openai.com/api/docs/guides/streaming-responses) -- SSE format, delta vs message, finish_reason (HIGH confidence)
- [Ollama streaming tool calling](https://ollama.com/blog/tool-support) -- NDJSON format, tool calls in pre-Done chunk (HIGH confidence)
- [Ollama OpenAI compatibility layer internals](https://deepwiki.com/ollama/ollama/3.4-openai-compatibility-layer) -- transformation logic between formats (MEDIUM confidence)
- [openai-go tool calling example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go) -- official Go SDK tool calling pattern (HIGH confidence)
- [OpenAI reasoning models](https://developers.openai.com/api/docs/guides/reasoning) -- reasoning_content and reasoning_effort (HIGH confidence)
- [Ollama context length docs](https://docs.ollama.com/context-length) -- Show API for context length discovery (HIGH confidence)

---
*Pitfalls research for: Fenec v1.1 -- Multi-provider LLM support*
*Researched: 2026-04-12*
