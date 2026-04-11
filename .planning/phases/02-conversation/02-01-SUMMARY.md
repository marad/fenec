---
phase: 02-conversation
plan: 01
subsystem: chat
tags: [ollama, context-window, metrics, streaming, truncation]

requires:
  - phase: 01-foundation
    provides: "chat package with StreamChat, Client, Conversation, chatAPI interface"
provides:
  - "StreamChat returning *api.Metrics with PromptEvalCount and EvalCount"
  - "GetContextLength querying model context_length via Show API"
  - "ContextTracker with threshold-based truncation of oldest messages"
  - "Conversation.ContextLength field for downstream context management"
  - "ChatRequest Truncate=false and num_ctx enforcement"
affects: [02-conversation, repl-integration]

tech-stack:
  added: []
  patterns:
    - "Client-side context management: Truncate=false + num_ctx on every ChatRequest"
    - "Proportional token estimation for iterative truncation without re-querying"
    - "Family-prefix key lookup for model_info context_length (e.g., gemma3.context_length)"

key-files:
  created:
    - internal/chat/context.go
    - internal/chat/context_test.go
  modified:
    - internal/chat/client.go
    - internal/chat/client_test.go
    - internal/chat/stream.go
    - internal/chat/stream_test.go
    - internal/chat/message.go

key-decisions:
  - "Conservative 4096 fallback when Show API fails or context_length key missing"
  - "Proportional token estimation in TruncateOldest rather than per-message token counting"
  - "Pair-based removal (user+assistant) to maintain conversation coherence during truncation"

patterns-established:
  - "Show API integration pattern: family-prefix suffix matching for model_info keys"
  - "Metrics capture pattern: capture resp.Metrics when resp.Done in streaming callback"

requirements-completed: [CHAT-02, CHAT-03]

duration: 4min
completed: 2026-04-11
---

# Phase 02 Plan 01: Context Window Management Summary

**StreamChat metrics capture, Show API context length discovery, and ContextTracker with threshold-based truncation**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-11T11:47:19Z
- **Completed:** 2026-04-11T11:50:59Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- StreamChat now returns actual Ollama Metrics (PromptEvalCount, EvalCount) from the final streaming chunk
- GetContextLength queries the model's context_length via Show API with family-prefix key lookup and 4096 fallback
- ContextTracker monitors token usage against a configurable threshold and truncates oldest non-system messages
- ChatRequest enforces client-side context management with Truncate=false and num_ctx from Conversation.ContextLength
- 36 total tests in the chat package, all passing

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend chat client with Show API and modify StreamChat to return Metrics** - `cdb057e` (feat)
2. **Task 2: Implement ContextTracker with threshold-based truncation** - `132653d` (feat)

_Note: TDD tasks with test and implementation in single commits (tests written first, implementation passes them)_

## Files Created/Modified
- `internal/chat/context.go` - ContextTracker with ShouldTruncate, Update, TruncateOldest, Available, Threshold
- `internal/chat/context_test.go` - 14 tests for context tracking and truncation edge cases
- `internal/chat/client.go` - GetContextLength method, Show in chatAPI interface, updated ChatService interface
- `internal/chat/client_test.go` - mockAPI Show method, GetContextLength tests, ContextLength tests
- `internal/chat/stream.go` - StreamChat returns 3 values, metrics capture, Truncate=false, num_ctx
- `internal/chat/stream_test.go` - Updated all existing tests for 3-return signature, added metrics and options tests
- `internal/chat/message.go` - Added ContextLength field to Conversation struct

## Decisions Made
- Conservative 4096 fallback when Show API fails or context_length key not found -- avoids crashing on unknown models
- Proportional token estimation in TruncateOldest (ratio-based reduction) rather than per-message counting -- simpler, and Ollama corrects on next response
- Pair-based removal (user+assistant together) maintains conversation coherence during truncation
- boolPtr helper for Truncate field since api.ChatRequest uses *bool

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Chat package ready for REPL integration in Plan 03 (StreamChat 3-return signature, ContextTracker, GetContextLength)
- `go build ./...` will fail until Plan 03 updates the StreamChat call site in repl.go (expected, documented in plan)
- Session persistence (Plan 02) can proceed independently

## Self-Check: PASSED

All 7 files verified present. Both commit hashes (cdb057e, 132653d) found in git log. SUMMARY.md created.

---
*Phase: 02-conversation*
*Completed: 2026-04-11*
