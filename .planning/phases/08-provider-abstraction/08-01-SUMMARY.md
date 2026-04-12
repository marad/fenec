---
phase: 08-provider-abstraction
plan: 01
subsystem: api
tags: [provider, ollama, interface, abstraction, refactoring]

# Dependency graph
requires:
  - phase: 07-canonical-types
    provides: Canonical model types (Message, ToolDefinition, StreamMetrics) decoupled from Ollama API
provides:
  - Provider interface with Name, ListModels, Ping, StreamChat, GetContextLength
  - ChatRequest type decoupled from Conversation
  - Ollama adapter implementing Provider in internal/provider/ollama
  - Clean chat package with only Conversation, ContextTracker, FirstTokenNotifier
affects: [09-openai-compat-client, 10-config-system, 11-model-routing]

# Tech tracking
tech-stack:
  added: []
  patterns: [provider-interface-abstraction, chatrequest-decoupled-from-conversation, adapter-pattern-for-llm-backends]

key-files:
  created:
    - internal/provider/provider.go
    - internal/provider/ollama/ollama.go
    - internal/provider/ollama/ollama_test.go
  modified:
    - internal/chat/client.go
    - internal/chat/stream.go
    - internal/chat/client_test.go
    - internal/chat/stream_test.go
    - internal/repl/repl.go
    - main.go
    - internal/tool/registry.go

key-decisions:
  - "ChatRequest struct carries model, messages, tools, think, context_length -- decoupled from Conversation"
  - "Provider interface has 5 methods: Name, ListModels, Ping, StreamChat, GetContextLength"
  - "Only internal/provider/ollama imports ollama/api -- all other packages use canonical types"

patterns-established:
  - "Provider adapter pattern: implement provider.Provider interface for each LLM backend"
  - "ChatRequest construction: build from Conversation state at call site (REPL)"

requirements-completed: [PROV-01, PROV-02]

# Metrics
duration: 5min
completed: 2026-04-12
---

# Phase 8 Plan 1: Provider Abstraction Summary

**Provider interface with 5 methods and Ollama adapter, moving all Ollama-specific code behind internal/provider/ollama while REPL and main.go consume only the abstract Provider**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-12T20:27:12Z
- **Completed:** 2026-04-12T20:32:54Z
- **Tasks:** 3
- **Files modified:** 10

## Accomplishments
- Created Provider interface and ChatRequest type in internal/provider/provider.go
- Moved all Ollama client code (Client, StreamChat, conversion functions) to internal/provider/ollama
- Migrated 25+ tests from chat package to provider/ollama with ChatRequest-based API
- Cleaned chat package to retain only Conversation, ContextTracker, FirstTokenNotifier
- Wired REPL and main.go to depend on Provider interface instead of ChatService

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Provider interface and ChatRequest type** - `673a96a` (feat)
2. **Task 2: Create Ollama adapter and migrate tests** - `0f30830` (feat)
3. **Task 3: Wire REPL and main.go to use Provider interface** - `734aca0` (feat)

## Files Created/Modified
- `internal/provider/provider.go` - Provider interface and ChatRequest type (no ollama/api dependency)
- `internal/provider/ollama/ollama.go` - Ollama adapter implementing Provider with all conversion functions
- `internal/provider/ollama/ollama_test.go` - 26 migrated tests covering StreamChat, ListModels, Ping, GetContextLength
- `internal/chat/client.go` - Gutted to empty package (ChatService, Client, chatAPI removed)
- `internal/chat/stream.go` - Stripped to only FirstTokenNotifier (StreamChat, conversions, boolPtr removed)
- `internal/chat/client_test.go` - Reduced to 2 Conversation tests
- `internal/chat/stream_test.go` - Reduced to 2 FirstTokenNotifier tests
- `internal/repl/repl.go` - Changed to use provider.Provider field, builds ChatRequest from Conversation
- `main.go` - Creates ollama.New() instead of chat.NewClient()
- `internal/tool/registry.go` - Updated stale ChatService comment

## Decisions Made
- ChatRequest carries all fields needed by providers (Model, Messages, Tools, Think, ContextLength) decoupled from Conversation
- Provider.Name() returns string identifier ("ollama") for future provider routing
- REPL builds ChatRequest from Conversation state at each StreamChat call site
- TestStreamChatFirstTokenNotifier was adapted to use manual once-flag instead of chat.FirstTokenNotifier to keep the test self-contained in the ollama package

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed stale ChatService comment in tool/registry.go**
- **Found during:** Task 3 (final verification)
- **Issue:** Comment referenced ChatService.StreamChat which no longer exists
- **Fix:** Updated comment to reference provider.StreamChat via ChatRequest
- **Files modified:** internal/tool/registry.go
- **Verification:** grep confirmed no ChatService references remain
- **Committed in:** 734aca0 (part of Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Trivial comment fix. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- Provider interface is ready for a second implementation (OpenAI-compatible client)
- Any new provider just implements 5 methods from provider.Provider
- REPL already decoupled from Ollama specifics
- Only internal/provider/ollama/ imports ollama/api

---
*Phase: 08-provider-abstraction*
*Completed: 2026-04-12*
