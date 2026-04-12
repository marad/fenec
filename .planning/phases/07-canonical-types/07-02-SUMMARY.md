---
phase: 07-canonical-types
plan: 02
subsystem: api
tags: [type-migration, adapter-pattern, ollama, canonical-types]

# Dependency graph
requires:
  - phase: 07-canonical-types plan 01
    provides: canonical model types (Message, ToolDefinition, ToolCall, StreamMetrics) in internal/model
provides:
  - All packages migrated from ollama/api types to internal/model canonical types
  - Ollama adapter boundary in internal/chat/stream.go with conversion functions
  - Clean separation -- only internal/chat imports ollama/api
affects: [08-provider-interface, 09-openai-compat, 10-config]

# Tech tracking
tech-stack:
  added: []
  patterns: [adapter-boundary-pattern, import-alias-for-shadow-avoidance]

key-files:
  created: []
  modified:
    - internal/tool/registry.go
    - internal/tool/shell.go
    - internal/tool/read.go
    - internal/tool/write.go
    - internal/tool/edit.go
    - internal/tool/listdir.go
    - internal/tool/create.go
    - internal/tool/delete.go
    - internal/tool/update.go
    - internal/lua/luatool.go
    - internal/lua/convert.go
    - internal/chat/client.go
    - internal/chat/message.go
    - internal/chat/stream.go
    - internal/session/session.go
    - internal/repl/repl.go

key-decisions:
  - "Used mdl alias for internal/model import in chat package to avoid shadowing model parameter names"
  - "Conversion functions (toOllamaMessages, fromOllamaMessage, toOllamaTools, fromOllamaMetrics) placed in stream.go alongside StreamChat"

patterns-established:
  - "Adapter boundary: only internal/chat imports ollama/api; all other packages use internal/model types"
  - "Tool arguments use map[string]any instead of Ollama ordered maps"

requirements-completed: [PROV-03]

# Metrics
duration: 10min
completed: 2026-04-12
---

# Phase 07 Plan 02: Migrate All Packages to Canonical Types Summary

**Full type decoupling from ollama/api -- only internal/chat retains the import as adapter boundary with 4 conversion functions**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-12T18:58:21Z
- **Completed:** 2026-04-12T19:08:41Z
- **Tasks:** 2
- **Files modified:** 29

## Accomplishments
- Migrated 19 files in internal/tool/ and internal/lua/ from ollama/api to internal/model types
- Migrated 10 files in internal/chat/, internal/session/, and internal/repl/ to canonical types
- Added Ollama conversion layer (toOllamaMessages, fromOllamaMessage, toOllamaTools, fromOllamaMetrics) to stream.go
- Tool interface now uses model.ToolDefinition and map[string]any -- replaced all ordered map patterns
- All 30+ test cases pass including race detector

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate tool package and lua package to canonical types** - `a1e4f53` (refactor)
2. **Task 2: Migrate chat, session, and repl packages with Ollama conversion layer** - `3b23104` (feat)

## Files Created/Modified
- `internal/tool/registry.go` - Tool interface uses model.ToolDefinition and map[string]any, Dispatch takes model.ToolCall
- `internal/tool/shell.go` - Shell tool uses canonical types, map access instead of .Get()
- `internal/tool/read.go` - Read tool uses canonical types, getOptionalInt takes map[string]any
- `internal/tool/write.go` - Write tool uses canonical types
- `internal/tool/edit.go` - Edit tool uses canonical types
- `internal/tool/listdir.go` - ListDir tool uses canonical types
- `internal/tool/create.go` - CreateLuaTool uses canonical types, successJSON uses plain map range
- `internal/tool/delete.go` - DeleteLuaTool uses canonical types
- `internal/tool/update.go` - UpdateLuaTool uses canonical types
- `internal/lua/luatool.go` - LuaTool.Definition() returns model.ToolDefinition, Execute takes map[string]any
- `internal/lua/convert.go` - ArgsToLuaTable takes map[string]any, no ollama import
- `internal/chat/client.go` - ChatService interface uses canonical types for StreamChat
- `internal/chat/message.go` - Conversation.Messages is []mdl.Message
- `internal/chat/stream.go` - Ollama adapter boundary with 4 conversion functions
- `internal/session/session.go` - Session.Messages is []model.Message
- `internal/repl/repl.go` - Uses model.ToolDefinition, map access for arguments

## Decisions Made
- Used `mdl` import alias for `internal/model` in the chat package to avoid shadowing the `model` parameter name in `NewConversation(model string, ...)`
- Placed all 4 conversion functions in stream.go alongside StreamChat rather than a separate file, since they are tightly coupled to the streaming adapter logic

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ollama type references now confined to internal/chat/client.go and internal/chat/stream.go
- Phase 8 (Provider Interface) can introduce the Provider abstraction without touching packages outside internal/chat
- ChatService interface is ready to be generalized into a provider-agnostic contract

## Self-Check: PASSED

All key files verified present. Both task commits (a1e4f53, 3b23104) verified in git history.

---
*Phase: 07-canonical-types*
*Completed: 2026-04-12*
