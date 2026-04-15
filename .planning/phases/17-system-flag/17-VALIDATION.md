---
phase: 17
slug: system-flag
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-15
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None (Go conventions) |
| **Quick run command** | `go test ./internal/config/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | FLAG-01 | — | N/A | unit | `go test ./internal/config/... -run TestLoadSystemPromptFromFile -x` | ❌ W0 | ⬜ pending |
| 17-01-02 | 01 | 1 | FLAG-01 | — | N/A | unit | `go test ./internal/config/... -run TestLoadSystemPromptFromFile_NotExist -x` | ❌ W0 | ⬜ pending |
| 17-01-03 | 01 | 1 | FLAG-01 | — | N/A | integration | `go test ./... -run TestExisting -x` | ✅ | ⬜ pending |
| 17-01-04 | 01 | 1 | FLAG-01 | — | N/A | integration | Manual — verify REPL tool descriptions after --system override | manual-only | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Test for file reading success case (if config helper approach)
- [ ] Test for file reading failure case (nonexistent file)

*Existing test infrastructure covers default system prompt loading.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Tool descriptions appended after --system override | FLAG-01 | REPL integration requires running binary | Run `fenec --system <file>`, verify tools still callable |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
