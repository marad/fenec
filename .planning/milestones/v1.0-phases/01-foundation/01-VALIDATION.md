---
phase: 1
slug: foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify v1.9.x |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test -v -race ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test -v -race ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | CHAT-01 | integration | `go test -v -run TestStreamingChat ./...` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | CHAT-04 | unit | `go test -v -run TestModelSelection ./...` | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 1 | CHAT-05 | unit | `go test -v -run TestMarkdownRender ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` — initialize module with testify dependency
- [ ] Test file stubs for each requirement area
- [ ] go test framework verification (runs clean)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Token-by-token streaming visible to user | CHAT-01 | Visual streaming behavior requires human observation | Run `fenec`, send a message, verify tokens appear incrementally |
| Markdown rendering with syntax highlighting | CHAT-05 | Visual formatting quality requires human judgment | Run `fenec`, ask for a code example, verify code blocks are highlighted |
| Spinner/thinking indicator appearance | CHAT-01 | Animation requires visual confirmation | Run `fenec`, send a message, verify spinner appears before first token |
| Auto-pager behavior on long output | CHAT-05 | Paging UX requires interactive testing | Run `fenec`, trigger long response, verify pager activates |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
