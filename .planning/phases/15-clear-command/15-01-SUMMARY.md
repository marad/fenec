---
phase: 15-clear-command
plan: 01
subsystem: repl
tags: [clear-command, session-management, context-tracker, tdd]
dependency_graph:
  requires: []
  provides: ["/clear REPL command", "ContextTracker.Reset() method"]
  affects: [internal/repl/repl.go, internal/repl/commands.go, internal/chat/context.go]
tech_stack:
  added: []
  patterns: [handleClearCommand handler, ContextTracker.Reset, sync.Once value replacement]
key_files:
  created: []
  modified:
    - internal/chat/context.go
    - internal/chat/context_test.go
    - internal/repl/repl.go
    - internal/repl/commands.go
    - internal/repl/repl_test.go
decisions:
  - "Session ID collision in tests resolved by setting explicit 'old-session' ID (NewSession uses second-precision timestamps)"
metrics:
  duration: "3m 38s"
  completed: "2026-04-15T08:51:46Z"
  tasks: 2
  files: 5
---

# Phase 15 Plan 01: Clear Command Summary

`/clear` REPL command with pre-clear session save via Store.Save(), full state reset (conversation, session, tracker, autoSave guard), system prompt rebuild with tool descriptions, and Think/ContextLength preservation

## Tasks Completed

| # | Task | Commit | Key Changes |
|---|------|--------|-------------|
| 1 | Add ContextTracker.Reset() method with test | `1dbb9ba` | Added `Reset()` to `ContextTracker` zeroing `lastPromptEval` and `lastEval`; added `TestContextTrackerResetZeroesCounters` |
| 2 | Implement /clear command handler, routing, help text, and REPL tests | `2b89801` | Added `handleClearCommand()`, `/clear` routing in Run() switch, helpText entry, 4 new tests |

## Implementation Details

### ContextTracker.Reset() (Task 1)
- New `Reset()` method on `*ContextTracker` zeroes both `lastPromptEval` and `lastEval` counters
- Placed immediately after `Update()` method following existing method ordering
- Test verifies `TokenUsage() == 0` and `ShouldTruncate() == false` after Reset

### handleClearCommand() (Task 2)
Five-step handler implementing all CONTEXT.md decisions:

1. **Save (D-01, D-02):** Syncs conv→session, calls `Store.Save()` if `HasContent()` is true. Continues on save failure (don't trap user).
2. **Capture flags:** Preserves `Think` flag (Pitfall 1) and `ContextLength` (Pitfall 3) before reset.
3. **Rebuild system prompt (CONV-03):** Reconstructs from `baseSystemPrompt` + `registry.Describe()` tool descriptions.
4. **Fresh state (D-03, D-04):** Creates new `Conversation` and `Session`, resets `ContextTracker`, re-arms `autoSaved` via `sync.Once{}` value replacement.
5. **Feedback (D-05, D-06):** "Conversation saved: {id} ({N} messages). Session cleared." when content existed; "Session cleared." when empty.

### Test Coverage
- `TestHelpTextContainsClear` — helpText includes `/clear` and description
- `TestHandleClearCommandSavesAndResets` — full flow: save, reset, new session ID, tracker zeroed, Think preserved, ContextLength preserved
- `TestHandleClearCommandSkipsSaveWhenEmpty` — empty conv produces only "Session cleared."
- `TestHandleClearCommandPreservesToolDescriptions` — system prompt rebuilt from baseSystemPrompt (without registry, no stale tool desc)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Session ID collision in test due to second-precision timestamps**
- **Found during:** Task 2 RED→GREEN transition
- **Issue:** `TestHandleClearCommandSavesAndResets` failed because `NewSession()` generates IDs from `time.Now().Format("2006-01-02T15-04-05")` — both the test setup and `handleClearCommand()` ran within the same second, producing identical IDs
- **Fix:** Set `r.session.ID = "old-session"` before calling `handleClearCommand()` so the new session always has a distinguishable ID
- **Files modified:** `internal/repl/repl_test.go`
- **Commit:** `2b89801`

## Verification Results

- `go test ./internal/chat/ -count=1` — 19/19 PASS
- `go test ./internal/repl/ -count=1` — 30/30 PASS
- `go test ./internal/session/ -count=1` — PASS
- Pre-existing failures in `internal/tool` (pathcheck macOS tests) — unrelated to this plan

## Self-Check: PASSED
