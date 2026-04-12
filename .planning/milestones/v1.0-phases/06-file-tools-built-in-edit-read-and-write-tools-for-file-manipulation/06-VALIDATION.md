---
phase: 06
slug: file-tools-built-in-edit-read-and-write-tools-for-file-manipulation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — existing Go test infrastructure |
| **Quick run command** | `go test ./internal/tool/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/tool/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | D-06,D-07 | unit | `go test ./internal/tool/ -run TestPathCheck` | ❌ W0 | ⬜ pending |
| 06-01-02 | 01 | 1 | D-09-D-13 | unit | `go test ./internal/tool/ -run TestReadFile` | ❌ W0 | ⬜ pending |
| 06-01-03 | 01 | 1 | D-14-D-16 | unit | `go test ./internal/tool/ -run TestWriteFile` | ❌ W0 | ⬜ pending |
| 06-01-04 | 01 | 1 | D-17-D-21 | unit | `go test ./internal/tool/ -run TestEditFile` | ❌ W0 | ⬜ pending |
| 06-01-05 | 01 | 1 | D-22-D-24 | unit | `go test ./internal/tool/ -run TestListDir` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/tool/pathcheck_test.go` — deny list and CWD boundary tests
- [ ] `internal/tool/readfile_test.go` — read_file tool tests
- [ ] `internal/tool/writefile_test.go` — write_file tool tests
- [ ] `internal/tool/editfile_test.go` — edit_file tool tests
- [ ] `internal/tool/listdir_test.go` — list_directory tool tests

*Existing Go test infrastructure covers framework needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Approval prompt for out-of-CWD writes | D-05 | Requires interactive terminal input | Run fenec, ask model to write outside CWD, verify prompt appears |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
