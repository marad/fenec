---
phase: 15
slug: clear-command
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-15
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.9.x |
| **Config file** | None needed — Go convention |
| **Quick run command** | `go test ./internal/chat/ ./internal/repl/ -count=1 -run TestClear -v` |
| **Full suite command** | `go test ./internal/... -count=1 -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/chat/ ./internal/repl/ -count=1 -v`
- **After every plan wave:** Run `go test ./internal/... -count=1 -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | CONV-01 | — | N/A | unit | `go test ./internal/repl/ -run TestClearResetsConversation -v` | ❌ W0 | ⬜ pending |
| 15-01-02 | 01 | 1 | CONV-01 | — | N/A | unit | `go test ./internal/repl/ -run TestHelpTextContainsClear -v` | ❌ W0 | ⬜ pending |
| 15-01-03 | 01 | 1 | CONV-02 | — | N/A | unit | `go test ./internal/repl/ -run TestClearSavesBeforeReset -v` | ❌ W0 | ⬜ pending |
| 15-01-04 | 01 | 1 | CONV-02 | — | N/A | unit | `go test ./internal/repl/ -run TestClearSkipsSaveEmpty -v` | ❌ W0 | ⬜ pending |
| 15-01-05 | 01 | 1 | CONV-03 | — | N/A | unit | `go test ./internal/repl/ -run TestClearPreservesToolDescriptions -v` | ❌ W0 | ⬜ pending |
| 15-01-06 | 01 | 1 | D-04 | — | N/A | unit | `go test ./internal/chat/ -run TestContextTrackerReset -v` | ❌ W0 | ⬜ pending |
| 15-01-07 | 01 | 1 | D-05/D-06 | — | N/A | unit | `go test ./internal/repl/ -run TestClearFeedback -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/chat/context_test.go` — add `TestContextTrackerReset` / `TestContextTrackerResetZeroesCounters`
- [ ] `internal/repl/repl_test.go` — add clear command tests (may require `newTestREPL` helper extension with session store)
- No framework install needed — Go testing is built in, testify already in go.mod

*Existing infrastructure covers test framework needs.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
