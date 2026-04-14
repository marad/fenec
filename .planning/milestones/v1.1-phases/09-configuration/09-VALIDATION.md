---
phase: 9
slug: configuration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-13
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify v1.9.x |
| **Config file** | none — existing test infrastructure |
| **Quick run command** | `go test ./internal/config/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | CONF-01, CONF-02 | unit | `go test ./internal/config/...` | ❌ W0 | ⬜ pending |
| 09-02-01 | 02 | 2 | CONF-01, CONF-03 | integration | `go test ./...` | ✅ | ⬜ pending |
| 09-03-01 | 03 | 3 | CONF-04 | integration | `go test ./...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/providers_test.go` — tests for TOML parsing + env var resolution
- [ ] `internal/config/registry_test.go` — tests for ProviderRegistry thread-safety
- [ ] `internal/config/watcher_test.go` — tests for file watcher + debounce

*Existing test infrastructure covers framework and fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Edit config.toml while Fenec running, changes take effect | CONF-04 | Requires live process + file edit | Start fenec, edit config, observe reload |
| Zero-config startup creates default Ollama provider | CONF-03 | Requires clean config directory | Remove config.toml, start fenec, verify connects to localhost:11434 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
