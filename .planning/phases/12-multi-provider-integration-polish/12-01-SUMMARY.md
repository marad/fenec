---
phase: 12-multi-provider-integration-polish
plan: 01
subsystem: api
tags: [openai, streaming, think-tags, state-machine]

requires:
  - phase: 10-openai-compatible-client
    provides: OpenAI adapter with chatStreaming and chatNonStreaming paths
provides:
  - Incremental think-tag parsing in OpenAI streaming path
  - Real-time onThinking callback delivery during SSE streaming
  - thinkParser state machine handling cross-chunk tag boundaries
affects: [repl, chat]

tech-stack:
  added: []
  patterns: [incremental tag parsing state machine for streaming content]

key-files:
  created: []
  modified:
    - internal/provider/openai/openai.go
    - internal/provider/openai/openai_test.go

key-decisions:
  - "Buffer-and-drain parser instead of byte-by-byte: accumulate chunk into buffer, scan for complete tags, flush safe prefix keeping only potential partial tag suffix"
  - "Preserve extractThinkingFromContent for non-streaming path; streaming path uses thinkParser exclusively"

patterns-established:
  - "thinkParser pattern: accumulate in buffer, drain complete tags, keep partial suffix for next chunk"

requirements-completed: [OAIC-01, OAIC-02]

duration: 3min
completed: 2026-04-13
---

# Phase 12 Plan 01: Streaming Thinking Delivery Summary

**Incremental think-tag parser in OpenAI chatStreaming delivering thinking content via onThinking callback in real-time**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-13T06:33:29Z
- **Completed:** 2026-04-13T06:36:31Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added thinkParser state machine that incrementally parses `<think>...</think>` tags as streaming chunks arrive
- onThinking callback now invoked in real-time during SSE streaming (previously never called)
- Handles edge cases: tags split across chunk boundaries, thinking-only content, nil callbacks
- 5 new tests covering single-chunk, split-chunk, thinking-only, nil-callback, and no-think-tag scenarios

## Task Commits

Each task was committed atomically:

1. **Task 1: Add streaming thinking delivery tests** - `56f0ec2` (test) - TDD RED phase, 5 failing tests
2. **Task 2: Implement incremental think-tag parsing** - `fa56197` (feat) - TDD GREEN phase, parser implementation

## Files Created/Modified
- `internal/provider/openai/openai.go` - Added thinkParser struct with process/drain/flush methods, rewired chatStreaming to use it
- `internal/provider/openai/openai_test.go` - Added 5 new streaming thinking delivery tests

## Decisions Made
- Used buffer-and-drain approach instead of byte-by-byte parsing: each chunk is appended to a buffer, then drained by scanning for complete tags with strings.Index. Only a potential partial tag suffix is retained across chunks. This preserves natural token batching (onToken called with full chunks, not individual characters).
- Kept extractThinkingFromContent for the non-streaming path (chatNonStreaming) since it still needs post-hoc tag extraction. Only the streaming path uses the new incremental parser.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- OpenAI streaming thinking delivery now works correctly for all thinking-capable models
- Ready for plan 12-02

---
*Phase: 12-multi-provider-integration-polish*
*Completed: 2026-04-13*
