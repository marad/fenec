---
phase: 18
slug: profile-flag
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2025-07-24
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 1 | FLAG-02 | — | N/A | integration | `go build . && ./fenec --profile test-profile --help` | ❌ W0 | ⬜ pending |
| 18-01-02 | 01 | 1 | FLAG-03 | — | N/A | integration | `go build .` | ❌ W0 | ⬜ pending |
| 18-01-03 | 01 | 1 | FLAG-04 | — | N/A | integration | `go build .` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements — Phase 18 is a pure main.go integration, validated by `go build` and manual invocation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Profile loads system prompt + model | FLAG-02 | Requires Ollama running + profile file | Create profile, run `fenec --profile <name>`, check model/prompt |
| --model overrides profile model | FLAG-03 | Requires Ollama running | Run `fenec --profile <name> --model <other>`, verify model used |
| --system + --profile compose | FLAG-04 | Requires Ollama running + both files | Run `fenec --profile <name> --system <file>`, verify prompt override |
| Invalid profile name errors | FLAG-02 | Requires no matching profile | Run `fenec --profile nonexistent`, verify error message |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
