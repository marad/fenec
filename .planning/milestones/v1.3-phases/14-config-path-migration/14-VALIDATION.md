---
phase: 14
slug: config-path-migration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-15
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None needed (go test built-in) |
| **Quick run command** | `go test ./internal/config/ -count=1 -run TestMigrat` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/ -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 0 | CFG-01 | — | N/A | unit | `go test ./internal/config/ -run TestConfigDir -count=1` | ✅ (needs update) | ⬜ pending |
| 14-01-02 | 01 | 0 | CFG-02 | T-14-01 | Rename is atomic, no partial state | unit | `go test ./internal/config/ -run TestMigrate -count=1` | ❌ W0 | ⬜ pending |
| 14-01-03 | 01 | 0 | CFG-02 | — | N/A | unit | `go test ./internal/config/ -run TestMigrateNoLegacy -count=1` | ❌ W0 | ⬜ pending |
| 14-01-04 | 01 | 0 | CFG-02 | — | N/A | unit | `go test ./internal/config/ -run TestMigrateNewPathExists -count=1` | ❌ W0 | ⬜ pending |
| 14-01-05 | 01 | 0 | CFG-03 | — | N/A | unit | `go test ./internal/config/ -run TestMigrateFeedback -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/config_test.go` — update `TestConfigDir` to verify `~/.config/fenec` pattern
- [ ] `internal/config/config_test.go` — add `TestMigrateIfNeeded*` tests (5 scenarios)
- [ ] `internal/config/config_test.go` — fix existing `TestLoadSystemPromptFromFile` which fails on macOS

*Existing infrastructure covers framework requirements. Only test files need creation/update.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Migration feedback visible on stderr | CFG-03 | Automated test captures stderr, but visual check confirms readability | Run `fenec` on macOS with legacy path present, verify message appears on stderr |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
