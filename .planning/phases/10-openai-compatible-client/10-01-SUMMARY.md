---
phase: 10-openai-compatible-client
plan: 01
subsystem: provider
tags: [openai, openai-go, streaming, sse, tool-calling, lm-studio]

# Dependency graph
requires:
  - phase: 08-provider-abstraction
    provides: Provider interface with 5 methods, ChatRequest type
  - phase: 09-config-driven-providers
    provides: CreateProvider factory, ProviderConfig with URL/APIKey fields
provides:
  - OpenAI-compatible Provider adapter in internal/provider/openai/
  - Factory registration for type=openai in CreateProvider
affects: [10-02, 11-model-selection]

# Tech tracking
tech-stack:
  added: [github.com/openai/openai-go/v3@v3.31.0]
  patterns: [non-streaming fallback for tool calls, narrow test interface for SDK services, opportunistic thinking extraction]

key-files:
  created: [internal/provider/openai/openai.go]
  modified: [go.mod, go.sum, internal/config/toml.go]

key-decisions:
  - "Non-streaming when tools present, streaming SSE when pure chat"
  - "Dummy API key 'not-needed' for local providers to prevent SDK env var lookup"
  - "GetContextLength returns 0 (unknown) since OpenAI API does not expose context window size"
  - "Thinking extraction: reasoning_content ExtraFields first, <think> tag regex fallback"

patterns-established:
  - "OpenAI adapter narrow interfaces: completionsAPI and modelsAPI wrap only used SDK methods"
  - "Tool call arguments parsed from JSON string to map[string]any at adapter boundary"
  - "Thinking content excluded from outgoing messages (DeepSeek 400 prevention)"

requirements-completed: [OAIC-01, OAIC-02, OAIC-03, OAIC-04]

# Metrics
duration: 4min
completed: 2026-04-13
---

# Phase 10 Plan 01: OpenAI-Compatible Provider Adapter Summary

**OpenAI-compatible Provider adapter with streaming/non-streaming dispatch, tool call argument parsing, and factory wiring via openai-go v3 SDK**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-13T05:37:54Z
- **Completed:** 2026-04-13T05:42:13Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Full Provider interface implementation for OpenAI-compatible endpoints (LM Studio, OpenAI cloud, etc.)
- Streaming SSE for pure chat with onToken callbacks; non-streaming fallback when tools present
- Tool call arguments parsed from JSON strings to map[string]any at adapter boundary
- Opportunistic thinking extraction from reasoning_content extra fields and <think> tags
- Factory wired: config.toml type="openai" creates the new provider

## Task Commits

Each task was committed atomically:

1. **Task 1: Add openai-go SDK dependency and create OpenAI adapter** - `4b4df6b` (feat)
2. **Task 2: Wire OpenAI provider into config factory** - `7e098a8` (feat)

## Files Created/Modified
- `internal/provider/openai/openai.go` - OpenAI-compatible Provider adapter with all 5 interface methods
- `internal/config/toml.go` - Added case "openai" to CreateProvider factory
- `go.mod` - Added openai-go/v3 v3.31.0 dependency
- `go.sum` - Updated checksums

## Decisions Made
- Non-streaming when tools present, streaming SSE when pure chat -- avoids complex chunked tool call argument assembly
- Dummy API key "not-needed" set for empty apiKey to prevent SDK from reading OPENAI_API_KEY env var
- GetContextLength returns 0 (unknown) since OpenAI /v1/models does not expose context window size; OpenAI API handles limits server-side
- Thinking extraction is two-stage: reasoning_content ExtraFields (DeepSeek) first, then <think> tag regex fallback

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pointer receivers on SDK service types**
- **Found during:** Task 1 (adapter compilation)
- **Issue:** SDK service methods (New, NewStreaming, ListAutoPaging) have pointer receivers, but NewClient returns struct values
- **Fix:** Used `&client.Chat.Completions` and `&client.Models` to pass pointers to the Provider struct
- **Files modified:** internal/provider/openai/openai.go
- **Verification:** `go build ./internal/provider/openai/` succeeds
- **Committed in:** 4b4df6b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor fix for SDK interface satisfaction. No scope creep.

## Issues Encountered
None beyond the pointer receiver deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- OpenAI adapter is ready for unit testing (Plan 02 scope)
- Factory wiring complete -- users can define openai providers in config.toml
- Narrow test interfaces (completionsAPI, modelsAPI) ready for mock injection in tests

## Self-Check: PASSED

- internal/provider/openai/openai.go: FOUND
- 10-01-SUMMARY.md: FOUND
- Commit 4b4df6b: FOUND
- Commit 7e098a8: FOUND

---
*Phase: 10-openai-compatible-client*
*Completed: 2026-04-13*
