---
phase: 3
slug: tool-execution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None needed -- Go convention |
| **Quick run command** | `go test ./internal/tool/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/tool/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | TOOL-01 | unit | `go test ./internal/tool/ -run TestRegistryDispatch -v` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | TOOL-02 | unit | `go test ./internal/tool/ -run TestRegistryTools -v` | ❌ W0 | ⬜ pending |
| 03-01-03 | 01 | 1 | TOOL-03 | unit | `go test ./internal/tool/ -run TestDispatchError -v` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | EXEC-01 | unit | `go test ./internal/tool/ -run TestShellExec -v` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | EXEC-02 | unit | `go test ./internal/tool/ -run TestDangerous -v` | ❌ W0 | ⬜ pending |
| 03-02-03 | 02 | 1 | EXEC-03 | unit | `go test ./internal/tool/ -run TestShellTimeout -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/tool/registry_test.go` — stubs for TOOL-01, TOOL-02, TOOL-03
- [ ] `internal/tool/shell_test.go` — stubs for EXEC-01, EXEC-03
- [ ] `internal/tool/safety_test.go` — stubs for EXEC-02

*Existing infrastructure covers test framework -- only test file stubs needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dangerous command approval prompt | EXEC-02 | Requires interactive readline Y/n input | Run `fenec`, ask agent to delete a file, verify Y/n prompt appears before execution |
| Streaming tool call display | TOOL-01 | Visual output verification | Run `fenec`, trigger a tool call, verify tool result is displayed and fed back |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
