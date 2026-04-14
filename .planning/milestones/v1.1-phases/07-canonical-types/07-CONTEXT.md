# Phase 7: Canonical Types - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace all usage of `github.com/ollama/ollama/api` types (Message, Tool, ToolCall, ToolCallFunctionArguments, etc.) with Fenec-owned canonical types in a new `internal/model` package. After this phase, only the Ollama adapter (to be created in Phase 8) imports `ollama/api` types. All other packages use Fenec-native types.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

All implementation choices are at Claude's discretion — pure infrastructure phase.

Key constraints from research:
- 30 Go files across 6 packages (chat, tool, session, repl, lua, config) import `ollama/api`
- New `internal/model` package must have zero external dependencies
- Canonical types: Message, ToolDefinition, ToolCall, StreamMetrics (at minimum)
- Tool.Execute() signature changes from `api.ToolCallFunctionArguments` to `map[string]any`
- Tool.Definition() return type changes from `api.Tool` to canonical ToolDefinition
- Session persistence JSON must remain backward-compatible or include version migration
- All existing tests must pass after migration

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/chat/message.go` — Conversation type using `[]api.Message`
- `internal/chat/client.go` — ChatService interface with StreamChat, ListModels, Ping, GetContextLength
- `internal/tool/registry.go` — Tool interface, Registry with Dispatch
- `internal/session/session.go` — Session persistence using api.Message

### Established Patterns
- Interface-based design (ChatService, Tool)
- ApproverFunc callback pattern for dangerous operations
- Registry with provenance tracking (built-in vs Lua)
- JSON serialization for session persistence

### Integration Points
- `main.go` wires all components together
- REPL imports chat, tool, session packages
- Tool interface used by all 8 built-in tools + LuaTool
- Conversation used by REPL for multi-turn chat
- StreamChat takes []api.Message and []api.Tool

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None — infrastructure phase stays within scope.

</deferred>
