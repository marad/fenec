---
phase: 2
slug: conversation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None — Go convention (go test ./...) |
| **Quick run command** | `go test ./internal/chat/ ./internal/session/ -v -count=1` |
| **Full suite command** | `go test -race -cover ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/chat/ ./internal/session/ -v -count=1`
- **After every plan wave:** Run `go test -race -cover ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | CHAT-02 | unit | `go test ./internal/chat/ -run TestConversation -v -count=1` | Partial | ⬜ pending |
| 02-01-02 | 01 | 1 | CHAT-03 | unit | `go test ./internal/chat/ -run TestContext -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 1 | SESS-01 | unit | `go test ./internal/session/ -run TestSession -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 1 | SESS-02 | unit + integration | `go test ./internal/session/ -run TestAutoSave -v -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/chat/context_test.go` — stubs for CHAT-03 (token tracking, truncation logic, threshold behavior)
- [ ] `internal/session/session_test.go` — stubs for SESS-01 (serialization, save/load, atomic writes)
- [ ] `internal/session/store_test.go` — stubs for SESS-01, SESS-02 (file persistence, listing, auto-save)
- [ ] `internal/chat/message_test.go` — stubs for CHAT-02 (multi-turn message accumulation, verify full history sent)
- [ ] Framework install: None — testify already in go.mod

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Graceful auto-save on SIGINT/SIGTERM | SESS-02 | Requires real signal delivery to process | 1. Start fenec chat session 2. Send a few messages 3. Press Ctrl+C 4. Verify session file exists in ~/.fenec/sessions/ |
| Conversation survives application restart | SESS-01 | End-to-end requires process restart | 1. Start fenec, send messages 2. Save session (`/save`) 3. Exit and restart 4. Load session (`/load`) 5. Verify previous messages influence response |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
