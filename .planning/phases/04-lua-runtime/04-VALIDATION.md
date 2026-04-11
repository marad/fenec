---
phase: 04
slug: lua-runtime
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify v1.9.x |
| **Config file** | none — go test discovers *_test.go automatically |
| **Quick run command** | `go test ./internal/lua/... -v` |
| **Full suite command** | `go test ./... -v` |
| **Estimated runtime** | ~6 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/lua/... -v`
- **After every plan wave:** Run `go test ./... -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | LUA-02 | unit | `go test ./internal/lua/ -run TestSandbox -v` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 1 | LUA-04 | unit | `go test ./internal/lua/ -run TestLuaTool -v` | ❌ W0 | ⬜ pending |
| 04-02-01 | 02 | 2 | LUA-06 | unit | `go test ./internal/lua/ -run TestLoader -v` | ❌ W0 | ⬜ pending |
| 04-02-02 | 02 | 2 | LUA-06 | integration | `go test ./... -v` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/lua/sandbox_test.go` — stubs for sandbox whitelist verification (LUA-04)
- [ ] `internal/lua/luatool_test.go` — stubs for LuaTool interface compliance (LUA-02)
- [ ] `internal/lua/loader_test.go` — stubs for tool loading and error handling (LUA-06)

*Existing go test infrastructure covers all framework requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Tool appears in system prompt | LUA-02 | Requires running Ollama + inspecting prompt | Start fenec, check system prompt includes Lua tool descriptions |
| Broken Lua file reported to user | LUA-06 | Requires visual inspection of error output | Place malformed .lua in tools dir, start fenec, verify error message |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
