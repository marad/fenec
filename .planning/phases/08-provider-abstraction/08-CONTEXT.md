# Phase 8: Provider Abstraction - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Define a Provider interface and wrap the existing Ollama client as the first adapter. After this phase, the REPL and all consumers use the Provider interface — not the Ollama-specific ChatService. A second provider (OpenAI-compatible) can be added by implementing the Provider interface without modifying existing code.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

All implementation choices are at Claude's discretion — pure infrastructure phase.

Key constraints from research and Phase 7 outcome:
- Phase 7 created `internal/model` with canonical types (Message, ToolDefinition, ToolCall, StreamMetrics)
- `ChatService` interface in `internal/chat/client.go` already has the right shape: ListModels, Ping, StreamChat, GetContextLength
- Only `internal/chat/client.go` and `internal/chat/stream.go` import `ollama/api` — this IS the adapter boundary
- The Provider interface should live in a new `internal/provider` package
- Ollama adapter wraps the existing `chat.Client` (or refactors it into `internal/provider/ollama`)
- REPL currently depends on `chat.ChatService` — needs to switch to Provider interface
- Conversation type lives in `internal/chat/message.go` — used by REPL and stream
- Session persistence uses `model.Message` — no provider coupling
- All existing behavior must be preserved exactly (PROV-01, PROV-02)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/chat/client.go` — ChatService interface + Ollama Client implementation
- `internal/chat/stream.go` — StreamChat with 4 conversion functions (toOllamaMessages, fromOllamaMessage, toOllamaTools, fromOllamaMetrics)
- `internal/chat/message.go` — Conversation type with model.Message
- `internal/model/` — canonical types (Message, ToolDefinition, ToolCall, StreamMetrics)

### Established Patterns
- Interface-based design (ChatService, Tool)
- Compile-time interface checks (`var _ ChatService = (*Client)(nil)`)
- Constructor functions (NewClient with host parameter)
- Context-based cancellation

### Integration Points
- `main.go` creates `chat.Client` and passes to REPL as `ChatService`
- REPL's `Run()` method takes `ChatService` for streaming
- REPL agentic loop calls `StreamChat` with tools
- ContextTracker in REPL uses `GetContextLength`

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None — infrastructure phase stays within scope.

</deferred>
