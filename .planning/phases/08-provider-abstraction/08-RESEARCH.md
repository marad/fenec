# Phase 8: Provider Abstraction - Research

**Researched:** 2026-04-12
**Domain:** Go interface design, provider pattern, LLM client abstraction
**Confidence:** HIGH

## Summary

Phase 8 extracts a Provider interface from the existing `chat.ChatService` and wraps the Ollama-specific implementation as the first adapter. The codebase is already well-prepared: Phase 7 created canonical types in `internal/model`, and only two files (`internal/chat/client.go`, `internal/chat/stream.go`) import `ollama/api`. The existing `ChatService` interface in `internal/chat/client.go` already has the right shape (ListModels, Ping, StreamChat, GetContextLength).

The main architectural work is: (1) create a new `internal/provider` package with a `Provider` interface, (2) move/refactor the Ollama client code into `internal/provider/ollama` as the first adapter implementation, (3) update `main.go` and `internal/repl` to depend on the Provider interface instead of `chat.ChatService`, and (4) relocate `Conversation` and `ContextTracker` out of `internal/chat` so they are provider-agnostic.

**Primary recommendation:** Define the Provider interface to match the existing ChatService method signatures (using canonical model types), then move the Ollama implementation into `internal/provider/ollama`. The REPL should depend only on the Provider interface. Conversation and ContextTracker should move to `internal/chat` (they already are provider-agnostic) or stay put -- the key is the REPL depends on the Provider interface, not on `chat.Client`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
No locked decisions -- all implementation choices are at Claude's discretion.

### Claude's Discretion
All implementation choices are at Claude's discretion -- pure infrastructure phase.

Key constraints from research and Phase 7 outcome:
- Phase 7 created `internal/model` with canonical types (Message, ToolDefinition, ToolCall, StreamMetrics)
- `ChatService` interface in `internal/chat/client.go` already has the right shape: ListModels, Ping, StreamChat, GetContextLength
- Only `internal/chat/client.go` and `internal/chat/stream.go` import `ollama/api` -- this IS the adapter boundary
- The Provider interface should live in a new `internal/provider` package
- Ollama adapter wraps the existing `chat.Client` (or refactors it into `internal/provider/ollama`)
- REPL currently depends on `chat.ChatService` -- needs to switch to Provider interface
- Conversation type lives in `internal/chat/message.go` -- used by REPL and stream
- Session persistence uses `model.Message` -- no provider coupling
- All existing behavior must be preserved exactly (PROV-01, PROV-02)

### Deferred Ideas (OUT OF SCOPE)
None -- infrastructure phase stays within scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PROV-01 | User can chat with Fenec using any configured provider without knowing the underlying protocol | Provider interface hides protocol details; REPL depends on interface only; all streaming/tool calling behavior preserved through abstraction |
| PROV-02 | User experiences identical tool calling behavior regardless of which provider is active | Tool definitions use canonical `model.ToolDefinition`; dispatch uses canonical `model.ToolCall`; provider converts to/from wire format internally |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib interfaces | Go 1.25+ | Provider interface definition | Standard Go pattern: small interfaces, implicit satisfaction |
| github.com/marad/fenec/internal/model | local | Canonical types (Message, ToolDefinition, etc.) | Already created in Phase 7; provider-agnostic by design |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/stretchr/testify | v1.11.1 | Test assertions | Already in use; continue for provider adapter tests |

No new external dependencies needed. This phase is a pure structural refactoring using existing Go patterns.

## Architecture Patterns

### Recommended Project Structure

```
internal/
  provider/
    provider.go          # Provider interface + ChatRequest/ChatOptions types
    ollama/
      ollama.go          # Ollama adapter implementing Provider
      ollama_test.go     # Tests for Ollama adapter (moved from chat/)
  chat/
    conversation.go      # Conversation type (renamed from message.go)
    context.go           # ContextTracker (unchanged)
    context_test.go      # (unchanged)
    stream.go            # DELETE or gut -- streaming logic moves into ollama adapter
    client.go            # DELETE or gut -- ChatService replaced by Provider
  model/                 # (unchanged from Phase 7)
  repl/
    repl.go              # Updated: depends on provider.Provider, not chat.ChatService
```

### Pattern 1: Go Interface-Based Provider Abstraction

**What:** Define a small Provider interface in `internal/provider/provider.go` that every backend must implement. Consumers (REPL, main.go) depend only on this interface.

**When to use:** Whenever adding a new backend (Ollama, OpenAI-compatible, etc.).

**Interface design:**

```go
package provider

import (
    "context"
    "github.com/marad/fenec/internal/model"
)

// Provider abstracts an LLM backend. Every provider implementation
// (Ollama, OpenAI-compatible, etc.) satisfies this interface.
type Provider interface {
    // Name returns the provider identifier (e.g., "ollama", "openai").
    Name() string

    // ListModels returns available model names.
    ListModels(ctx context.Context) ([]string, error)

    // Ping verifies the backend is reachable and operational.
    Ping(ctx context.Context) error

    // StreamChat sends messages to the model and streams the response.
    // onToken is called for each content chunk; onThinking for reasoning chunks.
    // Returns the full assistant message and performance metrics.
    StreamChat(ctx context.Context, req *ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error)

    // GetContextLength returns the model's context window size in tokens.
    GetContextLength(ctx context.Context, modelName string) (int, error)
}
```

**Key design decision -- ChatRequest instead of Conversation:**

The Provider interface should NOT take `*chat.Conversation` directly. Instead, define a `ChatRequest` struct in the provider package that holds only what the provider needs:

```go
// ChatRequest holds the data needed for a single chat completion.
type ChatRequest struct {
    Model         string
    Messages      []model.Message
    Tools         []model.ToolDefinition
    Think         bool
    ContextLength int  // 0 = not set
}
```

**Why:** This decouples the provider from the Conversation type (which is a session-management concern, not a protocol concern). The REPL builds a `ChatRequest` from its `Conversation` before calling `StreamChat`. This makes the provider interface usable from any consumer, not just the REPL.

### Pattern 2: Ollama Adapter

**What:** Move the existing `chat.Client` and its conversion functions into `internal/provider/ollama/ollama.go`.

**When to use:** This is the first and only adapter in Phase 8.

```go
package ollama

import (
    "context"
    "github.com/marad/fenec/internal/model"
    "github.com/marad/fenec/internal/provider"
    "github.com/ollama/ollama/api"
)

// Provider implements provider.Provider using the Ollama API.
type Provider struct {
    api chatAPI
}

// Compile-time check.
var _ provider.Provider = (*Provider)(nil)

func New(host string) (*Provider, error) {
    // Same logic as current chat.NewClient
}

func (p *Provider) Name() string { return "ollama" }

func (p *Provider) StreamChat(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
    // Existing StreamChat logic from chat/stream.go
    // Uses toOllamaMessages, toOllamaTools, fromOllamaMessage, fromOllamaMetrics
    // All conversion functions stay here (they are Ollama-specific)
}
```

### Pattern 3: Compile-Time Interface Checks

**What:** Every adapter includes `var _ provider.Provider = (*OllamaProvider)(nil)` to catch interface drift at compile time.

**Already used:** The codebase already does this: `var _ ChatService = (*Client)(nil)` in `stream.go`.

### Pattern 4: Constructor with Functional Options (Future-Ready)

**What:** While the Ollama adapter currently just needs a host string, structure the constructor to be extensible.

```go
func New(host string, opts ...Option) (*Provider, error) { ... }
```

**Skip for Phase 8:** A simple `New(host string)` is sufficient for now. Functional options can be added in Phase 9/10 when OpenAI adapter needs API keys, custom HTTP clients, etc. Do not over-engineer.

### Anti-Patterns to Avoid

- **Leaking provider types through the interface:** The Provider interface must ONLY use types from `internal/model` and `internal/provider`. Never expose `api.Message`, `api.ToolCall`, etc. in the interface.
- **God interface:** Do not add methods the REPL doesn't need yet. Keep the interface minimal (5 methods: Name, ListModels, Ping, StreamChat, GetContextLength). More can be added when needed.
- **Wrapping instead of moving:** Do NOT create a thin wrapper in `internal/provider/ollama` that delegates to `chat.Client`. Instead, MOVE the implementation. Having two layers of indirection is wasteful and confusing. The `chat` package should slim down to just Conversation and ContextTracker.
- **Premature channel-based streaming:** The community trend (any-llm-go, etc.) favors channel-based streaming. However, Fenec already uses callback-based streaming everywhere (REPL, tests). Switching to channels is a separate refactor that would touch many files. Keep callbacks for now.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Provider type registry | Custom reflection-based registry | Simple switch/map in factory function | Only 1-2 providers; a registry is over-engineered |
| Interface compatibility checks | Runtime type assertions | Compile-time `var _ = (*)` checks | Go idiom; catches errors at build time |
| Streaming abstraction | Custom channel/iterator framework | Keep existing callback pattern | Already works, tested, understood |
| Error normalization | Custom error type hierarchy | Standard `fmt.Errorf` with wrapping | Only one provider now; normalize when adding second |

**Key insight:** This phase adds structure, not new functionality. Every line of new code should be moving/reorganizing existing code, not writing novel logic.

## Common Pitfalls

### Pitfall 1: Circular Import Between provider and chat
**What goes wrong:** If `provider.go` imports types from `internal/chat` and `internal/chat` also imports from `internal/provider`, Go will refuse to compile.
**Why it happens:** Conversation lives in `internal/chat`, and you might be tempted to reference it from the Provider interface.
**How to avoid:** The Provider interface uses `ChatRequest` (defined in `internal/provider`) which contains `[]model.Message` (from `internal/model`). Conversation stays in `internal/chat`. The REPL (in `internal/repl`) imports both `provider` and `chat` and bridges them. No circular dependency.
**Warning signs:** Compile error "import cycle not allowed."

### Pitfall 2: Breaking the Agentic Loop
**What goes wrong:** The agentic loop in REPL's `sendMessage()` calls `StreamChat` repeatedly, passing tool results back. If the Provider interface changes the return type or message handling, the loop breaks silently (e.g., tool calls stop being detected).
**Why it happens:** Subtle type mismatches between old `api.ToolCall` fields and new `model.ToolCall` fields during migration.
**How to avoid:** The existing tests in `stream_test.go` cover tool call detection, streaming, cancellation, and metrics. After migration, ALL existing tests must pass unchanged. Add a specific integration-style test that simulates a full tool call round-trip through the Provider interface.
**Warning signs:** Tool calls silently not dispatched; model enters infinite loop requesting same tool.

### Pitfall 3: Losing Ollama-Specific Features in Abstraction
**What goes wrong:** Provider interface omits Ollama-specific features like `Think` (extended thinking), `Truncate: false`, or `num_ctx` options. These silently revert to defaults.
**Why it happens:** Trying to make the interface "generic" by removing provider-specific knobs.
**How to avoid:** The `ChatRequest` struct includes `Think bool` and `ContextLength int` -- these are not Ollama-specific, they are common LLM features. OpenAI has `reasoning_effort`, most providers accept context length. Keep them in the shared request type.
**Warning signs:** Model suddenly truncates context or stops producing thinking output.

### Pitfall 4: Test Migration Breakage
**What goes wrong:** Existing tests in `internal/chat/stream_test.go` and `internal/chat/client_test.go` use the `mockAPI` struct that implements `chatAPI` (the internal Ollama interface). Moving code to `internal/provider/ollama` means these tests must move too.
**Why it happens:** Tests are coupled to the package they test; moving the implementation means moving the tests.
**How to avoid:** Move tests alongside the implementation. The `mockAPI` struct and all test functions that test Ollama-specific behavior go to `internal/provider/ollama/ollama_test.go`. Tests for Conversation and ContextTracker stay in `internal/chat/`.
**Warning signs:** Tests left behind in `internal/chat/` that no longer compile because the types they test have moved.

### Pitfall 5: Forgetting to Update main.go
**What goes wrong:** main.go still creates `chat.NewClient()` and passes it as `ChatService` after the interface has been renamed/moved.
**Why it happens:** main.go is the integration point and easy to miss when refactoring internal packages.
**How to avoid:** main.go must create an `ollama.New(host)` and pass it as `provider.Provider` to the REPL. This is a small but critical change. Verify by running the binary end-to-end.
**Warning signs:** Compilation failure in main.go.

## Code Examples

### Provider Interface Definition

```go
// internal/provider/provider.go
package provider

import (
    "context"
    "github.com/marad/fenec/internal/model"
)

// ChatRequest holds parameters for a chat completion request.
type ChatRequest struct {
    Model         string
    Messages      []model.Message
    Tools         []model.ToolDefinition
    Think         bool
    ContextLength int // 0 means "not set, use model default"
}

// Provider abstracts an LLM backend.
type Provider interface {
    Name() string
    ListModels(ctx context.Context) ([]string, error)
    Ping(ctx context.Context) error
    StreamChat(ctx context.Context, req *ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error)
    GetContextLength(ctx context.Context, modelName string) (int, error)
}
```

### REPL Integration Change

```go
// internal/repl/repl.go -- the key change
type REPL struct {
    provider  provider.Provider  // was: client chat.ChatService
    conv      *chat.Conversation
    // ... rest unchanged
}

// In sendMessage, build ChatRequest from Conversation:
func (r *REPL) sendMessage(input string) {
    // ...
    req := &provider.ChatRequest{
        Model:         r.conv.Model,
        Messages:      r.conv.Messages,
        Tools:         tools,
        Think:         r.conv.Think,
        ContextLength: r.conv.ContextLength,
    }
    msg, metrics, err := r.provider.StreamChat(ctx, req, onToken, onThinking)
    // ... rest unchanged
}
```

### main.go Integration Change

```go
// main.go -- create provider instead of chat client
import "github.com/marad/fenec/internal/provider/ollama"

// Replace:
//   client, err := chat.NewClient(ollamaHost)
// With:
p, err := ollama.New(ollamaHost)

// Pass to REPL:
r, err := repl.NewREPL(p, defaultModel, systemPrompt, tracker, store, registry)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct Ollama client in REPL | Provider interface abstraction | This phase | Enables multi-provider support in Phase 9-11 |
| `ChatService` in `internal/chat` | `Provider` in `internal/provider` | This phase | Cleaner separation of concerns |
| `chat.Client` creates Ollama API client | `ollama.Provider` creates Ollama API client | This phase | Same functionality, better organized |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go convention: `go test ./...`) |
| Quick run command | `go test ./internal/provider/... ./internal/chat/... ./internal/repl/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROV-01 | Provider interface satisfied by Ollama adapter | unit | `go test ./internal/provider/ollama/ -run TestCompileCheck -x` | Wave 0 |
| PROV-01 | StreamChat returns correct message through provider | unit | `go test ./internal/provider/ollama/ -run TestStreamChat -x` | Wave 0 (migrated from chat/) |
| PROV-01 | ListModels works through provider | unit | `go test ./internal/provider/ollama/ -run TestListModels -x` | Wave 0 (migrated from chat/) |
| PROV-01 | Ping works through provider | unit | `go test ./internal/provider/ollama/ -run TestPing -x` | Wave 0 (migrated from chat/) |
| PROV-02 | Tool calls detected and returned through provider | unit | `go test ./internal/provider/ollama/ -run TestStreamChatToolCalls -x` | Wave 0 (migrated from chat/) |
| PROV-02 | Tool definitions passed to provider correctly | unit | `go test ./internal/provider/ollama/ -run TestStreamChatPassesTools -x` | Wave 0 (migrated from chat/) |
| PROV-01 | REPL works with Provider interface (not ChatService) | build | `go build ./...` | Compile-time check |
| PROV-01 | Full test suite passes after refactor | integration | `go test ./...` | Existing |

### Sampling Rate
- **Per task commit:** `go test ./internal/provider/... ./internal/chat/... ./internal/repl/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/provider/provider.go` -- Provider interface + ChatRequest type
- [ ] `internal/provider/ollama/ollama.go` -- Ollama adapter (moved from chat/)
- [ ] `internal/provider/ollama/ollama_test.go` -- Tests (moved from chat/stream_test.go, chat/client_test.go)

Existing test infrastructure (`go test ./...`, testify) covers all phase requirements. No new test framework needed.

## Open Questions

1. **Should Conversation move out of `internal/chat`?**
   - What we know: Conversation is used by REPL and ContextTracker. It contains only `model.Message` slice, Model string, and config flags. It has no Ollama-specific code.
   - What's unclear: Whether `internal/chat` is the right home for it long-term. It could move to `internal/model` or `internal/provider`.
   - Recommendation: Leave Conversation in `internal/chat` for now. It works, the REPL already imports `chat`, and moving it would create unnecessary churn. The important thing is that the Provider interface does NOT reference Conversation -- it uses ChatRequest. This is sufficient for decoupling.

2. **Should `chat.FirstTokenNotifier` move?**
   - What we know: It is used only by the REPL for spinner management. It has no provider dependency.
   - What's unclear: Whether it should stay in `internal/chat` which is becoming a "conversation management" package.
   - Recommendation: Leave it. It is a tiny utility; moving it adds churn for no benefit. It can always move later.

3. **What happens to the `internal/chat` package after extraction?**
   - What we know: After moving Client and StreamChat to provider/ollama, `internal/chat` retains: Conversation, ContextTracker, FirstTokenNotifier, and commands.go.
   - Recommendation: `internal/chat` becomes a "conversation management" package. This is a fine landing state. Do not rename or reorganize further in this phase.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/chat/client.go`, `internal/chat/stream.go` -- existing interface and implementation
- Codebase analysis: `internal/model/` -- canonical types from Phase 7
- Codebase analysis: `internal/repl/repl.go` -- consumer of ChatService
- Codebase analysis: `main.go` -- integration point
- Go specification: Interface satisfaction, implicit implementation, compile-time checks

### Secondary (MEDIUM confidence)
- [any-llm-go (Mozilla)](https://blog.mozilla.ai/run-openai-claude-mistral-llamafile-and-more-from-one-interface-now-in-go/) -- Provider abstraction patterns in Go LLM libraries
- [Go official blog: LLM-powered applications](https://go.dev/blog/llmpowered) -- Interface patterns for LLM provider abstraction
- [GoAI multi-provider client](https://dev.to/dariubs/goai-a-clean-multi-provider-llm-client-for-go-27o5) -- Lightweight provider interface patterns
- [Building production-ready LLM integration in Go](https://www.ksred.com/building-a-production-ready-go-package-for-llm-integration/) -- Streaming callback vs channel patterns

### Tertiary (LOW confidence)
- None. All findings verified against codebase and official Go patterns.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries needed, pure structural refactoring
- Architecture: HIGH -- interface shape derived directly from existing working ChatService + codebase analysis
- Pitfalls: HIGH -- identified from direct codebase inspection of import graph and type dependencies

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable; internal refactoring, no external dependency changes)
