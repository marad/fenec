---
phase: 11-model-routing
plan: 01
subsystem: api
tags: [provider-routing, cli, repl, model-selection]

# Dependency graph
requires:
  - phase: 09-config-hot-reload
    provides: ProviderRegistry with Register/Get/SetDefault/Update/Names
  - phase: 10-openai-compatible-client
    provides: OpenAI-compatible provider implementing Provider interface
provides:
  - DefaultName() method on ProviderRegistry
  - --model provider/model CLI flag parsing with / delimiter
  - /model provider/model REPL command for cross-provider switching
  - /model modelname REPL command for same-provider switching
  - ContextTracker.Reset for provider switch context length updates
affects: [11-02-model-discovery]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "provider/model slash-delimited routing syntax"
    - "REPL holds providerRegistry + activeProvider for runtime switching"

key-files:
  created: []
  modified:
    - internal/config/registry.go
    - internal/config/registry_test.go
    - internal/repl/repl.go
    - internal/repl/commands.go
    - internal/repl/repl_test.go
    - internal/chat/context.go
    - main.go

key-decisions:
  - "Skip model validation at startup -- trust user input and let provider error at runtime"
  - "ContextTracker.Reset added for provider switches that change context window size"
  - "handleModelCommand refactored: args-based dispatch with interactive list as fallback"

patterns-established:
  - "Provider routing: strings.SplitN on / delimiter, first part = provider, second = model"
  - "REPL parameter naming: toolRegistry vs providerRegistry for disambiguation"

requirements-completed: [ROUT-01, ROUT-02]

# Metrics
duration: 3min
completed: 2026-04-13
---

# Phase 11 Plan 01: Model Routing Summary

**Provider/model routing via / delimiter in --model CLI flag and /model REPL command with cross-provider switching**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-13T06:06:05Z
- **Completed:** 2026-04-13T06:09:51Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- DefaultName() method on ProviderRegistry with full test coverage
- --model provider/model CLI flag parses provider and model, resolves via registry
- /model provider/model REPL command switches provider and model at runtime
- /model modelname switches model within current provider
- Conversation history preserved across all provider and model switches

## Task Commits

Each task was committed atomically:

1. **Task 1: Add DefaultName to registry and model routing logic to main.go** - `531fefb` (feat)
2. **Task 2: Wire registry into REPL and implement /model switching with args** - `67fc245` (feat)

## Files Created/Modified
- `internal/config/registry.go` - Added DefaultName() method returning default provider name
- `internal/config/registry_test.go` - Added 3 tests for DefaultName (set, after update, empty)
- `internal/repl/repl.go` - Added providerRegistry/activeProvider fields, rewrote handleModelCommand with args
- `internal/repl/commands.go` - Updated helpText with provider/model syntax
- `internal/repl/repl_test.go` - Added parse command tests for provider/model, bare model, no args
- `internal/chat/context.go` - Added Reset() method for context length updates on provider switch
- `main.go` - Rewrote --model handling with / delimiter parsing, updated NewREPL call, updated flag help

## Decisions Made
- Skip model validation at startup: trust user input and let provider return error at runtime instead of blocking with ListModels call
- Added ContextTracker.Reset() to support context length changes when switching providers (Rule 3 - blocking)
- Refactored handleModelCommand to accept args parameter, with interactive list as the no-args fallback

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added ContextTracker.Reset method**
- **Found during:** Task 2 (handleModelCommand implementation)
- **Issue:** handleModelCommand needs to update context window size when switching providers, but ContextTracker had no method to update maxTokens
- **Fix:** Added Reset(maxTokens int) method to ContextTracker
- **Files modified:** internal/chat/context.go
- **Verification:** go build ./... and go test ./... both pass
- **Committed in:** 67fc245 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Essential for correctness when switching between providers with different context windows. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Provider routing wired end-to-end, ready for Plan 02 (model discovery with provider grouping)
- /model no-args path currently shows interactive list from current provider; Plan 02 can enhance with multi-provider model listing

## Self-Check: PASSED

- All 7 modified files exist on disk
- Both task commits (531fefb, 67fc245) found in git log
- All acceptance criteria verified (DefaultName method, tests, SplitN routing, flag help, registry fields, helpText)

---
*Phase: 11-model-routing*
*Completed: 2026-04-13*
