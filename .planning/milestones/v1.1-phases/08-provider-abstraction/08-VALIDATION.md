---
phase: 8
slug: provider-abstraction
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-12
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify v1.9.x |
| **Config file** | none — existing test infrastructure |
| **Quick run command** | `go test ./internal/provider/... ./internal/chat/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/provider/... ./internal/chat/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | PROV-01 | unit | `go test ./internal/provider/...` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 2 | PROV-01, PROV-02 | integration | `go test ./...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/provider/ollama/ollama_test.go` — tests migrated from chat package for Ollama adapter

*Existing test infrastructure covers framework and fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Chat with Ollama works identically to v1.0 | PROV-01 | End-to-end streaming experience | Start fenec, chat, use tools, verify behavior |
| Tool call round-trips work correctly | PROV-02 | Requires live Ollama model | Run tool-calling conversation, verify results |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
