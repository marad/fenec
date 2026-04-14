---
phase: 11
slug: model-routing
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-14
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for Phase 11: model-routing

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test ./internal/repl/ ./internal/config/ ./internal/render/ -count=1` |
| **Full suite command** | `go build ./... && go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/repl/ ./internal/config/ ./internal/render/ -count=1`
- **After every plan wave:** Run `go build ./... && go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | ROUT-01 | unit | `go test ./internal/config/ -run TestRegistryDefaultName -v` | ✅ | ✅ green |
| 11-01-01 | 01 | 1 | ROUT-01 | unit | `go test ./internal/config/ -run TestRegistryDefaultNameAfterUpdate -v` | ✅ | ✅ green |
| 11-01-01 | 01 | 1 | ROUT-01 | unit | `go test ./internal/config/ -run TestRegistryDefaultNameEmpty -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestParseCommandModelWithProvider -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestParseCommandModelBare -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestParseCommandModelNoArgs -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestHelpTextContainsProviderSyntax -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestHandleModelCommandProviderModel -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestHandleModelCommandUnknownProvider -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | ROUT-02 | unit | `go test ./internal/repl/ -run TestHandleModelCommandBareModel -v` | ✅ | ✅ green |
| 11-02-01 | 02 | 2 | ROUT-04 | unit | `go test ./internal/render/ -run TestFormatProviderHeader -v` | ✅ | ✅ green |
| 11-02-01 | 02 | 2 | ROUT-04 | unit | `go test ./internal/render/ -run TestFormatModelEntryActive -v` | ✅ | ✅ green |
| 11-02-01 | 02 | 2 | ROUT-04 | unit | `go test ./internal/render/ -run TestFormatModelEntryInactive -v` | ✅ | ✅ green |
| 11-02-01 | 02 | 2 | ROUT-04 | unit | `go test ./internal/render/ -run TestFormatProviderError -v` | ✅ | ✅ green |
| 11-02-02 | 02 | 2 | ROUT-03 | unit | `go test ./internal/repl/ -run TestListModels -v` | ✅ | ✅ green |
| 11-02-02 | 02 | 2 | ROUT-03 | unit | `go test ./internal/repl/ -run TestListModelsUnreachableProvider -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `--model provider/model` CLI flag routes to correct provider | ROUT-02 | main.go parsing not unit-testable without binary invocation | Run `fenec --model ollama/gemma4`; verify prompt shows `[gemma4]` and uses ollama provider |
| `--model badprovider/model` exits with error listing providers | ROUT-02 | main.go error path | Run `fenec --model badprovider/gemma4`; verify error lists available providers and exits |
| Conversation history preserved across provider switch | ROUT-02 | Requires live REPL session | Start REPL, send a message, switch provider, run `/history`; verify message count unchanged |

---

## Validation Audit 2026-04-14

| Metric | Count |
|--------|-------|
| Gaps found | 5 |
| Resolved (automated) | 5 |
| Escalated to manual-only | 0 |
| Pre-existing manual-only | 3 (CLI flag paths) |

---

## Validation Sign-Off

- [x] All tasks have `automated` verify or manual-only justification
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 N/A — existing infrastructure covers all requirements
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-14
