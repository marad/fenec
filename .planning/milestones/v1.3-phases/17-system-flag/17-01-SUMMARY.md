---
phase: 17-system-flag
plan: 01
subsystem: cli
tags: [flag, system-prompt, cli-override]
dependency_graph:
  requires: []
  provides: [--system-flag, ad-hoc-system-prompt-override]
  affects: [main.go]
tech_stack:
  added: []
  patterns: [conditional-flag-override, pflag-StringP]
key_files:
  created: []
  modified:
    - main.go
decisions:
  - "Inline file reading in main.go rather than config package helper — 5 lines of logic, no abstraction needed"
  - "Hard fail on missing/unreadable --system file per D-01 — no silent fallback"
  - "Complete prompt replacement per D-03 — config.LoadSystemPrompt() bypassed entirely when --system set"
metrics:
  duration: 1min
  completed: "2026-04-15T10:20:47Z"
  tasks: 1
  files: 1
requirements_completed: [FLAG-01]
---

# Phase 17 Plan 01: System Flag Summary

`--system/-s <file>` flag added to fenec CLI — reads file content as system prompt override, completely replacing config-based prompt for one invocation.

## What Was Done

### Task 1: Implement --system / -s flag with conditional system prompt loading
**Commit:** `e78f1db`

Three surgical changes to `main.go`:
1. **Flag definition** — `systemFile := pflag.StringP("system", "s", "", "File to use as system prompt for this session")` added after `showVersion` flag
2. **Help text** — Added `fenec --system prompt.md  Use a custom system prompt` example to usage block
3. **Conditional loading** — Replaced single `config.LoadSystemPrompt()` call with `if *systemFile != ""` branch: reads file via `os.ReadFile` on `--system`, falls through to `config.LoadSystemPrompt()` otherwise

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | ✅ PASS |
| `--system` in `fenec --help` | ✅ PASS |
| Usage example in help | ✅ PASS |
| `go test ./internal/config/...` | ✅ PASS |
| Flag definition present | ✅ PASS |
| Conditional loading present | ✅ PASS |
| Default fallback preserved | ✅ PASS |
| Hard fail error message | ✅ PASS |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — all functionality is fully wired.

## Decisions Made

1. **Inline in main.go** — File reading logic is 5 lines; creating a config package helper would be over-abstraction for a simple flag-to-variable flow.
2. **Hard fail on error** — Per D-01, `os.ReadFile` errors exit with non-zero code and clear message. No silent fallback to default prompt.
3. **Complete replacement** — Per D-03, when `--system` is set, `config.LoadSystemPrompt()` is never called. The override is total.

## Self-Check: PASSED

- [x] `main.go` modified with all three changes
- [x] Commit `e78f1db` exists in git log
- [x] Build succeeds, tests pass
- [x] No files created or deleted unexpectedly
