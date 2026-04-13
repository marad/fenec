---
phase: 10
slug: openai-compatible-client
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-13
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify v1.9.x |
| **Config file** | none — existing test infrastructure |
| **Quick run command** | `go test ./internal/provider/openai/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/provider/openai/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | OAIC-01, OAIC-02, OAIC-03, OAIC-04 | unit | `go test ./internal/provider/openai/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/provider/openai/openai_test.go` — tests for OpenAI adapter with mock HTTP server
- [ ] `internal/provider/openai/think_test.go` — tests for <think> tag extraction and reasoning_content parsing

*Existing test infrastructure covers framework and fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Chat with LM Studio model | OAIC-01 | Requires live LM Studio instance | Start LM Studio, load model, configure fenec to point at it, chat |
| Chat with OpenAI cloud model | OAIC-02 | Requires live API key | Configure OPENAI_API_KEY, chat with gpt-4o |
| Tool calling with non-streaming fallback | OAIC-03 | Requires live model | Run tool-calling conversation, verify tool executes |
| Mid-session provider switch preserves history | OAIC-04 | End-to-end behavior | Start with ollama, switch to openai provider, continue conversation |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
