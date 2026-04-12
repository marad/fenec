# Technology Stack: Multi-Provider Support

**Project:** Fenec v1.1 -- Multi-Provider LLM Support
**Researched:** 2026-04-12
**Confidence:** HIGH

## Scope

This document covers ONLY the new dependencies and patterns needed for multi-provider support. The existing v1.0 stack (Ollama native client, gopher-lua, glamour/lipgloss, readline, etc.) is validated and unchanged. See the v1.0 STACK.md in CLAUDE.md for the full baseline.

## New Dependencies

### OpenAI-Compatible Client

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| github.com/openai/openai-go/v3 | v3.31.0 | OpenAI-compatible API client for LM Studio, OpenAI, and other /v1/chat/completions providers | The official OpenAI Go SDK. Supports `option.WithBaseURL()` to point at any OpenAI-compatible endpoint (LM Studio, Ollama /v1, vLLM, etc.). Provides typed chat completion requests/responses with streaming (`NewStreaming` + `ChatCompletionAccumulator`), tool calling (`ChatCompletionToolUnionParam`), and model listing (`client.Models.List`). 343 importers, actively maintained (v3.31.0 released Apr 8, 2026). The only serious alternative is `github.com/sashabaranov/go-openai` but that is unofficial and lags behind on API features. |

**Import paths:**
```go
import (
    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)
```

**Key types for the provider adapter:**
- `openai.ChatCompletionNewParams` -- request parameters (messages, tools, model, stream)
- `openai.ChatCompletionMessageParamUnion` -- message variant type (user, assistant, tool, system)
- `openai.ChatCompletionToolUnionParam` -- tool definition variant
- `openai.FunctionDefinitionParam` -- function name + description + parameter schema
- `openai.FunctionParameters` -- `map[string]any` for JSON Schema parameter definitions
- `openai.ChatCompletionAccumulator` -- streaming chunk accumulator with `JustFinishedToolCall()`
- `openai.ChatCompletionChunk` -- individual streaming chunk type

**Client creation pattern for different providers:**
```go
// LM Studio
client := openai.NewClient(
    option.WithBaseURL("http://localhost:1234/v1"),
    option.WithAPIKey("lm-studio"),  // LM Studio ignores this but field is required
)

// OpenAI
client := openai.NewClient(
    option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    // BaseURL defaults to https://api.openai.com/v1
)

// Ollama via OpenAI-compatible endpoint
client := openai.NewClient(
    option.WithBaseURL("http://localhost:11434/v1"),
    option.WithAPIKey("ollama"),  // Ollama ignores this
)
```

**Go version requirement:** Go 1.22+. Fenec already requires Go 1.24+ (Ollama module floor), so no conflict.

### Config File Parsing

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| github.com/BurntSushi/toml | v1.6.0 | Parse `~/.config/fenec/config.toml` for provider definitions | The de facto TOML parser for Go. Reflection-based API mirrors `encoding/json`. TOML 1.1 enabled by default in v1.6.0. Zero transitive dependencies. TOML is the right format here because: (1) provider configs have nested tables (TOML's strength), (2) TOML supports inline comments (users will want to annotate API keys and URLs), (3) TOML is human-editable without footgun indentation (unlike YAML). v1.6.0 released Dec 18, 2025. |

**Why TOML over YAML:** The project already has `gopkg.in/yaml.v3` as a transitive dependency (from gopher-lua-libs), but YAML's indentation sensitivity is error-prone for config files that users hand-edit. TOML's explicit `[section]` syntax is clearer for provider definitions. TOML also does not require quoting strings that look like booleans or numbers (YAML's "Norway problem").

**Why TOML over JSON:** JSON has no comments. Provider configs need comments for API keys, URL formats, and model aliases.

**Config file location:** `~/.config/fenec/config.toml` (follows existing `config.ConfigDir()` pattern).

**Example config structure:**
```toml
# Default provider used when no --model prefix is given
default_provider = "ollama"

[providers.ollama]
type = "ollama"
url = "http://localhost:11434"

[providers.lmstudio]
type = "openai"
url = "http://localhost:1234/v1"
api_key = ""  # LM Studio does not require one

[providers.openai]
type = "openai"
api_key_env = "OPENAI_API_KEY"  # Read from environment variable
# url defaults to https://api.openai.com/v1

[providers.openai.models]
# Optional model aliases / overrides
default = "gpt-4o"
```

## No New Dependencies Needed For

| Concern | Why No New Dep |
|---------|---------------|
| Provider abstraction interface | Pure Go interface design -- no library needed. Define a `Provider` interface in `internal/provider/` that both Ollama and OpenAI adapters implement. |
| Message type conversion | Manual mapping functions between `api.Message` (Ollama) and `openai.ChatCompletionMessageParamUnion` (OpenAI). Straightforward struct-to-struct conversion. |
| Tool definition conversion | Manual mapping from `api.Tool` (Ollama) to `openai.ChatCompletionToolUnionParam` (OpenAI). Both use JSON Schema for parameter definitions. |
| Model routing (`provider/model` syntax) | Simple string parsing with `strings.SplitN(model, "/", 2)`. No library needed. |

## Integration Architecture

### Provider Interface Design

The central abstraction is a `Provider` interface that both the existing Ollama client and the new OpenAI-compatible client implement. This interface must match what the REPL and agentic loop currently consume (the existing `chat.ChatService` interface).

**Current `ChatService` interface (to be evolved into Provider):**
```go
type ChatService interface {
    ListModels(ctx context.Context) ([]string, error)
    Ping(ctx context.Context) error
    StreamChat(ctx context.Context, conv *Conversation, tools api.Tools,
        onToken func(string), onThinking func(string)) (*api.Message, *api.Metrics, error)
    GetContextLength(ctx context.Context, model string) (int, error)
}
```

**Key design decision: Keep `api.Message` as the internal message type or define a Fenec-native type?**

Use a Fenec-native message type. The current codebase is deeply coupled to `api.Message` (Ollama types) throughout `Conversation`, `Session`, and the tool `Registry`. Converting to a Fenec-native message type at the boundary is the clean approach because:

1. `api.Message` has Ollama-specific fields (`Thinking`, `Images`, `ThinkValue`) that do not apply to OpenAI providers
2. `openai.ChatCompletionMessageParamUnion` is a union/variant type that maps poorly onto a concrete struct
3. A Fenec-native type lets you control JSON serialization for session persistence independently of either provider's type
4. The conversion cost is negligible -- it is a one-time struct copy at the provider boundary

**Fenec-native message type (proposed):**
```go
type Message struct {
    Role       string            `json:"role"`       // system, user, assistant, tool
    Content    string            `json:"content"`
    ToolCalls  []ToolCall        `json:"tool_calls,omitempty"`
    ToolCallID string            `json:"tool_call_id,omitempty"`
    Thinking   string            `json:"thinking,omitempty"`
}

type ToolCall struct {
    ID       string         `json:"id"`
    Function ToolCallFunc   `json:"function"`
}

type ToolCallFunc struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}
```

Similarly, define a Fenec-native `ToolDef` type for the tool registry instead of `api.Tool`.

### Adapter Pattern

Each provider type gets an adapter that converts between Fenec-native types and the provider's SDK types:

```
REPL / Agentic Loop
        |
        v
  Provider Interface (Fenec-native types)
        |
   +---------+----------+
   |                     |
OllamaAdapter     OpenAIAdapter
   |                     |
api.Client        openai.Client
```

- **OllamaAdapter**: wraps `github.com/ollama/ollama/api`. Converts `api.Message` to/from Fenec `Message`. Handles Ollama-specific features (Think, num_ctx, Truncate).
- **OpenAIAdapter**: wraps `github.com/openai/openai-go/v3`. Converts `openai.ChatCompletionMessageParamUnion` to/from Fenec `Message`. Maps tool definitions from Fenec `ToolDef` to `openai.ChatCompletionToolUnionParam`.

### Provider Registry

A `ProviderRegistry` created from config at startup:

```go
type ProviderRegistry struct {
    providers map[string]Provider   // name -> provider instance
    default_  string                // default provider name
}
```

The `--model provider/model` flag does `strings.SplitN(flag, "/", 2)` to get (provider, model). If no `/` prefix, use the default provider.

## OpenAI-Compatible Endpoint Limitations

When using the OpenAI-compatible adapter (for LM Studio, Ollama /v1, etc.), these features are NOT available:

| Feature | Ollama Native | OpenAI-Compatible | Impact |
|---------|:---:|:---:|--------|
| Think/reasoning control | Yes | No | Cannot enable/disable thinking mode per-request. Models that support it may still think, but you cannot control it. |
| `tool_choice` parameter | N/A | Not on Ollama /v1 | Cannot force a specific tool to be called. Works on real OpenAI API. |
| `num_ctx` (context length) | Yes | No | Cannot set context window size. Ollama /v1 uses the model default. |
| `Truncate` control | Yes | No | Cannot disable Ollama's automatic truncation. |
| Model Show (context length query) | Yes | No | `GetContextLength` unavailable via /v1. Use config-defined defaults. |
| Token metrics (`Metrics`) | Ollama-specific | `usage` field in response | Different format. Need to normalize. OpenAI returns `prompt_tokens` + `completion_tokens`. |
| Image attachments | base64 only | base64 only (via /v1) | Parity on base64. OpenAI /v1 also supports URLs; Ollama /v1 does not. |

**Implication:** The Ollama native adapter should remain the primary path for Ollama. The OpenAI-compatible adapter exists for LM Studio, OpenAI itself, and other providers -- not as a replacement for the native Ollama client.

## Installation

```bash
# New dependencies for v1.1
go get github.com/openai/openai-go/v3@v3.31.0
go get github.com/BurntSushi/toml@v1.6.0
```

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| OpenAI client | github.com/openai/openai-go/v3 | github.com/sashabaranov/go-openai | Unofficial. Historically popular but now the official SDK is mature (v3.31.0). The official SDK gets new model constants and API features on release day. sashabaranov/go-openai lags. |
| OpenAI client | github.com/openai/openai-go/v3 | Raw net/http + encoding/json | Too much boilerplate for streaming SSE parsing, tool call accumulation, and error handling. The SDK handles all of this. Worth the dependency. |
| Config format | TOML (BurntSushi/toml) | YAML (gopkg.in/yaml.v3) | Already a transitive dep, but YAML is indentation-sensitive and has the "Norway problem" (bare `no` parses as `false`). TOML is safer for hand-edited config. |
| Config format | TOML (BurntSushi/toml) | JSON | No comments. Users need to annotate provider configs with notes about API keys, URLs, model names. |
| Config format | TOML (BurntSushi/toml) | github.com/spf13/viper | Massively over-scoped. Viper pulls in Consul, etcd, and remote config support. We need to parse one file. |
| Config format | TOML (BurntSushi/toml) | github.com/pelletier/go-toml/v2 | Viable alternative. Faster than BurntSushi in benchmarks. But BurntSushi is the ecosystem standard (used by Rust's Cargo, Hugo, etc.) and has a simpler API. Performance irrelevant for a single config file read at startup. |
| Message abstraction | Fenec-native types | Keep api.Message everywhere | Would require the OpenAI adapter to fake Ollama types, creating a confusing dependency direction. The REPL would import Ollama types even when talking to OpenAI. Clean break is worth the refactor. |
| Message abstraction | Fenec-native types | Use openai types everywhere | Same problem in reverse. Also, openai-go uses union/variant types that are awkward as a canonical internal representation. |

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| github.com/tmc/langchaingo | Multi-provider abstraction is exactly what LangChain does, but it brings 10+ provider SDKs and forces their chain/agent abstractions. Fenec has its own agentic loop. | Two thin adapters (Ollama, OpenAI) behind a simple Go interface. |
| github.com/anthropic-go or similar per-provider SDKs | Anthropic, Cohere, etc. all offer OpenAI-compatible endpoints now. Do not add N provider SDKs when one OpenAI SDK + WithBaseURL covers them all. | openai-go with WithBaseURL pointed at each provider. |
| Generic "LLM router" libraries | Adds a layer of indirection that obscures what is happening. Fenec's provider routing is simple enough to be a switch statement. | Provider registry with named providers from config. |
| github.com/spf13/viper for config | Massively overscoped. Remote config, watch, multi-format -- none needed. | BurntSushi/toml for a single config file. |

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| github.com/openai/openai-go/v3 v3.31.0 | Go 1.22+ | Well within Fenec's Go 1.24+ floor. |
| github.com/BurntSushi/toml v1.6.0 | Go 1.18+ | No compatibility concerns. |
| github.com/ollama/ollama/api v0.20.x | Go 1.24+ | Unchanged from v1.0. |
| Ollama /v1/chat/completions | Tool calling | Supported. No tool_choice. No streaming tool responses (accumulated in final chunk). |
| LM Studio /v1/chat/completions | Tool calling | Supported for tool-trained models. Same OpenAI format. |
| OpenAI /v1/chat/completions | Tool calling | Full support including tool_choice, parallel tool calls. |

## Key Technical Notes

### openai-go Dependency Weight
The `github.com/openai/openai-go` module is relatively lightweight compared to the Ollama module. It has minimal transitive dependencies (primarily `github.com/tidwall/gjson` for JSON parsing and standard library). Adding it will not significantly increase binary size or build time.

### Streaming Pattern Difference
Ollama uses a callback function (`ChatResponseFunc`) for streaming. openai-go uses an iterator pattern (`stream.Next()` / `stream.Current()`). The provider adapter must normalize these into a single callback-based interface to match what the REPL expects. The OpenAI adapter will run `stream.Next()` in a loop and call the same `onToken(string)` callback.

### Tool Call ID Generation
Ollama generates tool call IDs server-side. OpenAI also generates them server-side. Both return IDs in the response. The Fenec-native ToolCall type should carry whatever ID the provider returned. No client-side ID generation needed.

### Thinking/Reasoning Output
Only the Ollama native adapter supports `Think` mode control. The OpenAI adapter should silently ignore think enablement. Some OpenAI-compatible providers (e.g., DeepSeek via OpenAI compat) may return reasoning tokens, but there is no standard way to request or control this via the OpenAI API. Mark this as a provider-specific feature.

### API Key Security
The config file may contain API keys. The config loader should:
1. Support `api_key_env = "ENV_VAR_NAME"` for environment variable references (preferred)
2. Support inline `api_key = "sk-..."` for convenience
3. Warn if the config file has world-readable permissions (mode > 0600)
4. Never log or display API key values

## Sources

- [openai-go v3 package docs](https://pkg.go.dev/github.com/openai/openai-go/v3) -- v3.31.0, types verified (HIGH confidence)
- [openai-go option package](https://pkg.go.dev/github.com/openai/openai-go/v3/option) -- WithBaseURL, WithAPIKey confirmed (HIGH confidence)
- [openai-go GitHub repository](https://github.com/openai/openai-go) -- examples verified (HIGH confidence)
- [openai-go tool calling example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go) -- type names verified (HIGH confidence)
- [openai-go streaming accumulator example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-accumulating/main.go) -- ChatCompletionAccumulator pattern verified (HIGH confidence)
- [BurntSushi/toml v1.6.0](https://github.com/BurntSushi/toml/releases) -- version and TOML 1.1 default confirmed (HIGH confidence)
- [BurntSushi/toml package docs](https://pkg.go.dev/github.com/BurntSushi/toml) -- API verified (HIGH confidence)
- [Ollama OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility) -- supported endpoints, tool calling, limitations (HIGH confidence)
- [LM Studio OpenAI-compatible tools](https://lmstudio.ai/docs/developer/openai-compat/tools) -- tool calling support confirmed (HIGH confidence)
- [OpenAI list models API](https://developers.openai.com/api/reference/go/resources/models/methods/list) -- client.Models.List pattern verified (HIGH confidence)

---
*Stack research for: Fenec v1.1 Multi-Provider Support*
*Researched: 2026-04-12*
