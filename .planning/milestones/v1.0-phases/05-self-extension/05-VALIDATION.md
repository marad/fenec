---
phase: 05
slug: self-extension
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./internal/lua/... ./internal/tool/... ./internal/repl/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/lua/... ./internal/tool/... ./internal/repl/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | LUA-01 | unit | `go test ./internal/tool/... -run CreateLuaTool` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | LUA-05 | unit | `go test ./internal/lua/... -run Validate` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | LUA-01 | unit | `go test ./internal/tool/... -run UpdateLuaTool` | ❌ W0 | ⬜ pending |
| 05-01-04 | 01 | 1 | LUA-01 | unit | `go test ./internal/tool/... -run DeleteLuaTool` | ❌ W0 | ⬜ pending |
| 05-02-01 | 02 | 2 | LUA-03 | unit | `go test ./internal/tool/... -run Registry` | ❌ W0 | ⬜ pending |
| 05-02-02 | 02 | 2 | LUA-03 | integration | `go test ./internal/repl/... -run HotReload` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/tool/create_tool_test.go` — stubs for create/update/delete tool tests
- [ ] `internal/lua/validate_test.go` — stubs for source validation tests
- [ ] `internal/repl/tools_command_test.go` — stubs for /tools command tests

*Existing test infrastructure (go test) covers framework needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Tool banner display | LUA-03 | Visual output formatting | Create a tool via chat, verify `✦ New tool registered:` banner appears |
| End-to-end agent authoring | LUA-01 | Requires live Ollama model | Ask agent to create a tool, verify it persists and works in next turn |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
