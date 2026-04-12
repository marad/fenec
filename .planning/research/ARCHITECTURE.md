# Architecture Patterns: Multi-Provider LLM Support

**Domain:** Multi-provider abstraction for existing AI agent platform
**Researched:** 2026-04-12
**Overall confidence:** HIGH

## Current Architecture: Where Ollama Types Leak

The existing codebase has **six Ollama type contamination points** that must be addressed:

| Location | Ollama Type Used | Impact |
|----------|-----------------|--------|
| `chat.ChatService` interface | `api.Tools`, `api.Message`, `api.Metrics` in `StreamChat` signature | **Critical** -- this is the main abstraction boundary |
| `chat.Conversation` | `[]api.Message` as the message store | **Critical** -- every component touches this |
| `tool.Tool` interface | `api.Tool` in `Definition()`, `api.ToolCallFunctionArguments` in `Execute()` | **Critical** -- every tool implements this |
| `tool.Registry` | `api.Tools` in `Tools()`, `api.ToolCall` in `Dispatch()` | **Critical** -- bridges tools to chat |
| `session.Session` | `[]api.Message` in `Messages` field (JSON serialized) | **High** -- persisted to disk, migration needed |
| `repl.REPL.sendMessage` | Direct access to `msg.ToolCalls`, `tc.Function.Name`, `tc.ID` | **Moderate** -- consumer code, follows the interfaces |

## Recommended Architecture

### Design Principle: Own Your Types

Introduce Fenec-native message and tool types that sit between the REPL/tool layer and the provider implementations. Each provider adapter translates to/from these types. This is the standard adapter pattern, not a generic LLM framework.

**Why not just use OpenAI types as the universal format?** Because:
1. Ollama's native API has features OpenAI lacks (thinking/reasoning output, model management, `num_ctx` control)
2. Tying to OpenAI types creates the same vendor lock-in we're trying to escape
3. Fenec-native types can evolve independently of any provider's API changes

**Why not use a library like `any-llm-go` or `langchaingo`?** Because:
1. Fenec has a custom tool system with Lua extensibility -- generic libraries don't model this
2. The provider count is small (2 protocols: Ollama native, OpenAI-compatible) -- the abstraction cost of a generic library exceeds the integration cost of 2 adapters
3. Dependency weight: `any-llm-go` pulls 8+ provider SDKs; we need exactly 2

### Component Boundaries

```
                    main.go (wiring)
                         |
                    internal/repl
                    (uses fenec types)
                         |
              +---------+----------+
              |                    |
    internal/provider         internal/tool
    (Provider interface)      (uses fenec types)
         |         |               |
   +-----+----+   +-------+   internal/lua
   |          |            |   (uses fenec types)
 ollama/    openai/     internal/model
 adapter    adapter     (fenec-native types)
   |          |
 Ollama    openai-go/v3
 api pkg   (pointed at any
            compatible endpoint)
```

### New Package: `internal/model` -- Fenec-Native Types

This is the keystone package. Everything else depends on it. It depends on nothing.

```go
package model

// Message is Fenec's provider-agnostic message type.
type Message struct {
    Role       Role       `json:"role"`
    Content    string     `json:"content"`
    Thinking   string     `json:"thinking,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

// ToolCall represents a model's request to invoke a tool.
type ToolCall struct {
    ID       string           `json:"id"`
    Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

// ToolDef is a provider-agnostic tool definition.
// The JSON shape matches both OpenAI and Ollama tool schemas.
type ToolDef struct {
    Type     string      `json:"type"` // always "function"
    Function FunctionDef `json:"function"`
}

type FunctionDef struct {
    Name        string        `json:"name"`
    Description string        `json:"description"`
    Parameters  ParametersDef `json:"parameters"`
}

type ParametersDef struct {
    Type       string                 `json:"type"` // "object"
    Properties map[string]PropertyDef `json:"properties"`
    Required   []string               `json:"required,omitempty"`
}

type PropertyDef struct {
    Type        string `json:"type"`
    Description string `json:"description"`
}

// StreamMetrics captures token usage from any provider.
type StreamMetrics struct {
    PromptTokens     int
    CompletionTokens int
}
```

**Key design choice: `Arguments` is `map[string]any`, not a custom ordered-map type.** The current Ollama `ToolCallFunctionArguments` is a `wk8/go-ordered-map` which is fine for Ollama but the OpenAI client returns a JSON string. `map[string]any` is the simplest common denominator that both can produce. Tool implementations already call `args.Get("key")` which maps trivially to `args["key"]`.

### New Package: `internal/provider` -- Provider Interface

```go
package provider

import (
    "context"
    "github.com/marad/fenec/internal/model"
)

// ChatOptions holds per-request settings that vary by provider.
type ChatOptions struct {
    Model         string
    ContextLength int   // Ollama: sets num_ctx. OpenAI-compat: ignored.
    Think         bool  // Ollama: sets Think field. OpenAI-compat: ignored.
    Tools         []model.ToolDef
}

// Provider is the core abstraction for multi-provider support.
type Provider interface {
    // Name returns the provider's configured name (e.g., "ollama", "lmstudio").
    Name() string

    // StreamChat sends messages and streams the response.
    // onToken is called for each content chunk.
    // onThinking is called for thinking/reasoning chunks (nil-safe).
    // Returns the complete assistant message and metrics.
    StreamChat(ctx context.Context, messages []model.Message, opts ChatOptions,
        onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error)

    // ListModels returns available model names from this provider.
    ListModels(ctx context.Context) ([]string, error)

    // Ping verifies the provider is reachable.
    Ping(ctx context.Context) error

    // SupportsThinking reports whether this provider supports thinking output.
    SupportsThinking() bool
}
```

**Why a single `StreamChat` method instead of separate `Chat` and `StreamChat`?** Because Fenec always streams. The current `ChatService.StreamChat` is the only chat method used. Non-streaming would be dead code.

**Why not `GetContextLength` on the Provider interface?** Because context length discovery is provider-specific. Ollama has `Show()` API for it; OpenAI-compatible APIs don't expose it. Context length comes from config (with optional Ollama auto-detection accessed via the concrete type).

### Model Resolution

```go
// ModelResolver maps a "provider/model" string to the right Provider + model name.
type ModelResolver struct {
    providers       map[string]Provider
    defaultProvider string
}

// Resolve parses "provider/model" or plain "model" (uses default provider).
func (r *ModelResolver) Resolve(spec string) (Provider, string, error) {
    parts := strings.SplitN(spec, "/", 2)
    if len(parts) == 2 {
        p, ok := r.providers[parts[0]]
        if !ok {
            return nil, "", fmt.Errorf("unknown provider: %s", parts[0])
        }
        return p, parts[1], nil
    }
    p, ok := r.providers[r.defaultProvider]
    if !ok {
        return nil, "", fmt.Errorf("no default provider configured")
    }
    return p, spec, nil
}
```

### Provider Implementations

#### `internal/provider/ollama` -- Ollama Native Adapter

Wraps the existing `api.Client`. This is mostly a refactor of the current `chat.Client` with type conversion added.

```go
package ollama

type OllamaProvider struct {
    client chatAPI  // same internal interface as current chat.Client
    name   string
}
```

Conversion between types is straightforward because Ollama's types map 1:1 to fenec types:
- `api.Message.Role` (string) <-> `model.Role` (string typedef) -- same values
- `api.Message.Content` <-> `model.Message.Content`
- `api.Message.Thinking` <-> `model.Message.Thinking`
- `api.ToolCall` <-> `model.ToolCall` -- same field structure
- `api.Tool` <-> `model.ToolDef` -- same JSON shape

The only nontrivial conversion: `api.ToolCallFunctionArguments` (ordered map) -> `map[string]any`. Simple iteration over the ordered map's entries.

Private conversion functions in the adapter package:
- `fenecToOllamaMessages([]model.Message) []api.Message`
- `ollamaToFenecMessage(api.Message) model.Message`
- `fenecToOllamaTools([]model.ToolDef) api.Tools`
- `ollamaToFenecMetrics(api.Metrics) model.StreamMetrics`

Ollama-specific capabilities (context length detection via `Show()`, model pull) live on the concrete `OllamaProvider` type, not on the `Provider` interface.

#### `internal/provider/openaicompat` -- OpenAI-Compatible Adapter

Uses `github.com/openai/openai-go/v3` with `option.WithBaseURL()` to point at any compatible endpoint.

```go
package openaicompat

import (
    oai "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)

type OpenAIProvider struct {
    client *oai.Client
    name   string
}

func New(name, baseURL, apiKey string) *OpenAIProvider {
    opts := []option.RequestOption{
        option.WithBaseURL(baseURL),
    }
    if apiKey != "" {
        opts = append(opts, option.WithAPIKey(apiKey))
    }
    client := oai.NewClient(opts...)
    return &OpenAIProvider{client: client, name: name}
}
```

Key differences from Ollama adapter:

| Aspect | Ollama Adapter | OpenAI-Compatible Adapter |
|--------|---------------|--------------------------|
| Streaming | Callback-based (`ChatResponseFunc`) | Iterator-based (`stream.Next()`) with `ChatCompletionAccumulator` |
| Tool call args | Already parsed (ordered map) | JSON string, needs `json.Unmarshal` -> `map[string]any` |
| Thinking | Supported via `Message.Thinking` | Not supported, `SupportsThinking()` returns false |
| Context length | Controllable via `num_ctx` | Server-managed, not controllable |
| Metrics | `api.Metrics.PromptEvalCount`, `.EvalCount` | `usage.prompt_tokens`, `usage.completion_tokens` via `stream_options` |
| Tool call IDs | Model-dependent (may be empty) | Always server-generated (`call_xxx` format) |
| Model listing | `/api/tags` endpoint | `/v1/models` endpoint |
| Health check | `List()` succeeds | `/v1/models` succeeds |

### Modified Package: `internal/tool` -- Decouple from Ollama

The `Tool` interface and `Registry` switch to fenec-native types:

```go
// BEFORE (current):
type Tool interface {
    Name() string
    Definition() api.Tool
    Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error)
}

// AFTER:
type Tool interface {
    Name() string
    Definition() model.ToolDef
    Execute(ctx context.Context, args map[string]any) (string, error)
}
```

**Impact on existing tools (8 files):** Every built-in tool's `Definition()` and `Execute()` needs updating. The changes are mechanical:
- `Definition()`: Replace `api.Tool{...}` with `model.ToolDef{...}` (same field names)
- `Execute()`: Replace `args.Get("key")` with `args["key"]` (map lookup instead of ordered-map method)

**Impact on `Registry`:**
```go
// BEFORE:
func (r *Registry) Tools() api.Tools
func (r *Registry) Dispatch(ctx context.Context, call api.ToolCall) (string, error)

// AFTER:
func (r *Registry) Tools() []model.ToolDef
func (r *Registry) Dispatch(ctx context.Context, call model.ToolCall) (string, error)
```

**Impact on `LuaTool`:** Same pattern -- `Definition()` builds `model.ToolDef`, `Execute()` receives `map[string]any`. The `ArgsToLuaTable` helper needs minor adjustment (accepts `map[string]any` instead of ordered map).

### Modified Package: `internal/chat` -- Conversation Keeps Fenec Types

The `chat` package splits:
1. **Chat client logic** (talking to Ollama) -> moves to `internal/provider/ollama`
2. **Conversation management** (message list, model tracking) -> stays, uses fenec types
3. **ContextTracker** -> stays unchanged (only uses int counts)

```go
// Conversation switches message type:
type Conversation struct {
    Messages      []model.Message
    Model         string
    ContextLength int
    Think         bool
}
```

The `ChatService` interface is **replaced** by `provider.Provider`. The `chat.Client` type is removed (absorbed into `provider/ollama`).

### Modified Package: `internal/session` -- Migration Required

`Session.Messages` changes from `[]api.Message` to `[]model.Message`.

**Migration risk assessment:** The JSON field names are identical between `api.Message` and `model.Message` (`role`, `content`, `tool_calls`, `tool_call_id`). Existing session files will deserialize correctly **except** for `ToolCallFunctionArguments` which uses a custom ordered-map JSON serialization in the Ollama package. Since `map[string]any` deserializes standard JSON objects, and the ordered-map serializes to standard JSON, this should round-trip. Must verify with test.

### Modified: `internal/repl` -- Minimal Changes

The REPL switches from `chat.ChatService` to `provider.Provider`. The agentic loop field access is unchanged because fenec types mirror the Ollama field names:

```go
msg.ToolCalls           // same field name, different type
tc.Function.Name        // same
tc.Function.Arguments   // same (but map[string]any instead of ordered map)
tc.ID                   // same
```

The REPL also needs to pass `ChatOptions` instead of directly building `api.ChatRequest`, but this is a straightforward refactor.

### New: Config-Driven Provider Definitions

Extend `internal/config` with TOML-based provider configuration:

```toml
# ~/.config/fenec/config.toml

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
api_key_env = "OPENAI_API_KEY"
```

**Config types:**

```go
type Config struct {
    DefaultProvider string                    `toml:"default_provider"`
    Providers       map[string]ProviderConfig `toml:"providers"`
}

type ProviderConfig struct {
    Type      string `toml:"type"`        // "ollama" or "openai"
    URL       string `toml:"url"`
    APIKeyEnv string `toml:"api_key_env"` // env var name for API key
    APIKey    string `toml:"api_key"`     // inline (not recommended)
}
```

**Why `api_key_env` instead of just `api_key`?** Config files get committed to dotfile repos. `api_key_env = "OPENAI_API_KEY"` reads from the environment at runtime.

**Backward compatibility:** When no config file exists, default to a single "ollama" provider at `http://localhost:11434`. The app works identically to v1.0 with no config file present.

### `--model provider/model` Syntax

```
fenec --model gemma4              # Uses default provider
fenec --model ollama/gemma4       # Explicit provider
fenec --model lmstudio/qwen3:32b  # LM Studio
fenec --model openai/gpt-4o      # OpenAI
```

## Data Flow

### Current Flow (Ollama-only)

```
User input -> REPL -> chat.Client.StreamChat(conv, tools) -> Ollama API
                          |
                    api.Message types throughout
```

### New Flow (Multi-provider)

```
User input -> REPL -> provider.StreamChat(messages, opts) -> Adapter -> Backend
                |                                               |
          model.Message                                   api.Message (Ollama)
          model.ToolDef                                   OR
          model.ToolCall                                  openai types
```

The REPL and tool system only see `model.*` types. Provider adapters handle all type conversion internally.

## Patterns to Follow

### Pattern 1: Adapter with Internal Conversion Functions

**What:** Each provider adapter has private `toFenec*` and `fromFenec*` functions for type conversion.
**When:** Every provider implementation.
**Why:** Keeps conversion logic co-located with the provider that needs it. No conversion logic in shared packages.

```go
// internal/provider/ollama/convert.go
func fenecToOllamaMessages(msgs []model.Message) []api.Message { ... }
func ollamaToFenecMessage(m api.Message) model.Message { ... }
func fenecToOllamaTools(defs []model.ToolDef) api.Tools { ... }
```

### Pattern 2: Provider Factory from Config

**What:** Factory function creates the right Provider based on config type.
**When:** Application startup in `main.go`.

```go
func NewProvider(name string, cfg config.ProviderConfig) (provider.Provider, error) {
    switch cfg.Type {
    case "ollama":
        return ollama.New(name, cfg.URL)
    case "openai":
        apiKey := resolveAPIKey(cfg)
        return openaicompat.New(name, cfg.URL, apiKey)
    default:
        return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
    }
}
```

### Pattern 3: Graceful Degradation for Provider-Specific Features

**What:** Features like thinking output and context length auto-detection work when available, silently skip otherwise.
**When:** Any provider-specific capability.

```go
// In REPL:
if currentProvider.SupportsThinking() {
    opts.Think = true
}
// onThinking callback always passed -- providers that don't support it never call it.
```

### Pattern 4: Concrete Type Access for Provider-Specific Operations

**What:** Provider-specific operations (Ollama's `GetContextLength`, `Show`) are accessed by type-asserting the concrete provider, not by bloating the interface.
**When:** `main.go` startup for context length detection.

```go
// In main.go after creating provider:
if op, ok := p.(*ollama.OllamaProvider); ok {
    ctxLen, err := op.GetContextLength(ctx, modelName)
    // use ctxLen
}
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Using OpenAI Types as the Universal Message Format

**What:** Making everything speak `openai.ChatCompletionMessageParamUnion` internally.
**Why bad:** Creates dependency on `openai-go` throughout the codebase. Ollama has features (thinking, `num_ctx`) that OpenAI types can't represent. You'd need out-of-band fields.
**Instead:** Own your types in `internal/model`. Each adapter converts.

### Anti-Pattern 2: Interface Bloat with Provider-Specific Methods

**What:** Adding `GetContextLength()`, `ShowModel()`, `PullModel()` to the Provider interface.
**Why bad:** Not all providers support these. Methods returning `ErrNotSupported` everywhere.
**Instead:** Keep Provider small (5 methods). Provider-specific features on concrete type.

### Anti-Pattern 3: Abstracting Streaming Differences

**What:** Creating a generic `StreamReader` or channel-based streaming abstraction.
**Why bad:** Ollama uses callback streaming. OpenAI-go uses iterator streaming. An abstraction adds latency and complexity for zero benefit -- the Provider interface already hides the mechanism.
**Instead:** Each adapter uses its native streaming approach, translates to `onToken`/`onThinking` callbacks.

### Anti-Pattern 4: Big-Bang Migration

**What:** Rewriting all packages simultaneously to use fenec types.
**Why bad:** Massive diff, hard to review, hard to bisect regressions.
**Instead:** Incremental phase-by-phase approach (see Build Order).

## Suggested Build Order

Dependency direction: `model` (no deps) -> `tool` (deps model) -> `chat/session` (deps model) -> `provider` (deps model) -> REPL (deps both) -> config.

### Phase 1: Foundation Types (`internal/model`)

**Create** `internal/model` with all fenec-native types. No other packages change.

- `Message`, `Role`, `ToolCall`, `ToolCallFunction`
- `ToolDef`, `FunctionDef`, `ParametersDef`, `PropertyDef`
- `StreamMetrics`
- Tests for JSON serialization (validates session persistence compatibility)

**Risk:** Low. Pure addition.

### Phase 2: Decouple Tool System (`internal/tool`, `internal/lua`)

**Modify** `Tool` interface, all built-in tools, `LuaTool`, and `Registry` to use fenec types.

8 tool files + registry + luatool change. Each change is mechanical type substitution.

**Risk:** Medium (many files, small changes each). Fully testable.

### Phase 3: Decouple Conversation and Session

**Modify** `Conversation` to use `[]model.Message`.
**Modify** `Session` to use `[]model.Message`.
**Test** existing session file deserialization.

**Risk:** Medium. Session persistence compatibility must be verified.

### Phase 4: Provider Interface and Ollama Adapter

**Create** `internal/provider` with `Provider` interface and `ModelResolver`.
**Create** `internal/provider/ollama` wrapping existing Ollama client logic.
**Modify** REPL to use `Provider` instead of `ChatService`.

At this point: app works exactly as before, through the new abstraction.

**Risk:** Medium. Ollama adapter is refactored tested code.

### Phase 5: OpenAI-Compatible Adapter

**Create** `internal/provider/openaicompat` using `openai-go/v3`.
**Add** dependency: `go get github.com/openai/openai-go/v3`.
**Test** against Ollama's `/v1/` endpoint and optionally LM Studio.

**Risk:** Medium. New dependency, new streaming model, but isolated.

### Phase 6: Config and CLI Integration

**Add** TOML config file with provider definitions.
**Add** `--model provider/model` parsing.
**Add** provider factory in `main.go`.
**Add** model discovery across providers.

**Risk:** Low-Medium. Outermost layer, depends on everything else.

### Phase ordering rationale

- Phase 1 before 2: Tools need types to exist before referencing them.
- Phase 2 before 3: Tool system is the most complex consumer; validates type design.
- Phase 3 before 4: Conversation must use fenec types before provider can return them.
- Phase 4 before 5: Ollama adapter validates Provider interface with known-working code.
- Phase 5 before 6: OpenAI adapter must work before config-driven selection is useful.
- Phase 6 last: Config/CLI are outermost; depend on everything else.

## Component Change Summary

| Component | Status | Nature of Change |
|-----------|--------|-----------------|
| `internal/model` | **NEW** | Fenec-native types package |
| `internal/provider` | **NEW** | Provider interface + ModelResolver |
| `internal/provider/ollama` | **NEW** | Ollama adapter (refactored from chat.Client) |
| `internal/provider/openaicompat` | **NEW** | OpenAI-compatible adapter (openai-go/v3) |
| `internal/tool` (all 8 tool files) | **MODIFIED** | `api.Tool` -> `model.ToolDef`, `api.ToolCallFunctionArguments` -> `map[string]any` |
| `internal/tool/registry.go` | **MODIFIED** | `api.Tools` -> `[]model.ToolDef`, `api.ToolCall` -> `model.ToolCall` |
| `internal/lua/luatool.go` | **MODIFIED** | Same type switch as tool package |
| `internal/lua/convert.go` | **MODIFIED** | `ArgsToLuaTable` takes `map[string]any` |
| `internal/chat/message.go` | **MODIFIED** | `Conversation.Messages` uses `[]model.Message` |
| `internal/chat/client.go` | **REMOVED** | Logic moves to `internal/provider/ollama` |
| `internal/chat/stream.go` | **REMOVED** | Logic moves to `internal/provider/ollama` |
| `internal/chat/context.go` | **UNCHANGED** | Only uses ints, no provider types |
| `internal/session/session.go` | **MODIFIED** | `Messages` field type change |
| `internal/repl/repl.go` | **MODIFIED** | Uses `provider.Provider` instead of `chat.ChatService` |
| `internal/config/config.go` | **MODIFIED** | Provider config types, TOML parsing |
| `main.go` | **MODIFIED** | Provider factory, config loading, model resolver wiring |

## Scalability Considerations

| Concern | 2 providers (now) | 5+ providers (future) |
|---------|-------------------|----------------------|
| Provider interface | Simple, works fine | Still fine -- the OpenAI-compat adapter covers most backends |
| Type conversion | Manual per adapter | 2 adapters handle unlimited backends (Ollama native + OpenAI-compat) |
| Config | TOML with manual sections | TOML still works |
| Testing | Mock provider per test | Shared contract test suite per provider |

The "2 protocol adapters" architecture is key: the OpenAI-compatible adapter covers LM Studio, OpenAI, Anthropic (via proxy), Ollama's `/v1/`, and any other compatible endpoint. Adding a "new provider" is just adding a TOML section, not writing code.

## Sources

- [openai-go GitHub repository](https://github.com/openai/openai-go) -- v3.31.0, import path `github.com/openai/openai-go/v3` (HIGH confidence)
- [openai-go option package](https://pkg.go.dev/github.com/openai/openai-go/option) -- `WithBaseURL`, `WithAPIKey` (HIGH confidence)
- [openai-go tool calling example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go) -- message types and flow (HIGH confidence)
- [openai-go streaming accumulator](https://github.com/openai/openai-go/blob/main/examples/chat-completion-accumulating/main.go) -- `ChatCompletionAccumulator` pattern (HIGH confidence)
- [Ollama OpenAI compatibility docs](https://docs.ollama.com/api/openai-compatibility) -- supported endpoints and limitations (HIGH confidence)
- [LM Studio tool calling docs](https://lmstudio.ai/docs/developer/openai-compat/tools) -- OpenAI-compatible tool format (HIGH confidence)
- [any-llm-go by Mozilla](https://blog.mozilla.ai/run-openai-claude-mistral-llamafile-and-more-from-one-interface-now-in-go/) -- multi-provider abstraction patterns, evaluated and rejected (MEDIUM confidence)
- [Provider Strategy pattern](https://dev.to/daniloab/how-to-integrate-multiple-llm-providers-without-turning-your-codebase-into-a-mess-provider-36g9) -- design patterns reference (MEDIUM confidence)
- Existing codebase: `internal/chat/`, `internal/tool/`, `internal/repl/`, `internal/session/` -- direct code review (HIGH confidence)
