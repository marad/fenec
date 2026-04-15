---
phase: 18-profile-flag
plan: 01
subsystem: cli
tags: [profile, flag, precedence, cli]
dependency_graph:
  requires: [16-profile-package, 17-system-flag]
  provides: [profile-flag-integration]
  affects: [main.go]
tech_stack:
  added: []
  patterns: [three-layer-precedence, pflag-Changed-guard]
key_files:
  created: []
  modified: [main.go]
decisions:
  - "Used pflag.CommandLine.Changed(\"model\") to distinguish explicit --model from profile-set model — prevents Pitfall 1 (profile modelName triggering --model branch) and Pitfall 2 (provider leak from profile to --model override)"
  - "Three-layer prompt precedence (--system > profile > config default) with empty-body fallthrough for model-only profiles (D-05)"
  - "Uppercase -P shorthand for --profile since -p is taken by --pipe (D-07)"
metrics:
  duration: "3min"
  completed: "2026-04-15T11:50:04Z"
  tasks: 2
  files: 1
requirements: [FLAG-02, FLAG-03, FLAG-04]
---

# Phase 18 Plan 01: Profile Flag Integration Summary

**One-liner:** `--profile/-P` flag wired into CLI with three-layer model/prompt precedence using `pflag.Changed("model")` guard to prevent override leaks.

## What Was Done

### Task 1: Register --profile flag, add import, usage example, and profile loading block
- **Commit:** `2a630b1`
- Added `internal/profile` import to main.go
- Registered `--profile` / `-P` flag via `pflag.StringP("profile", "P", ...)`
- Added `fenec --profile coder    Activate a named profile` usage example to help text
- Added profile loading block after provider registry setup with hard-fail error handling via `render.FormatError()` + `os.Exit(1)`

### Task 2: Implement model and prompt precedence chains with profile layer
- **Commit:** `3db6c58`
- **Model precedence:** `--model` > profile > config default
  - `modelExplicit := pflag.CommandLine.Changed("model")` guard prevents profile model from triggering the `--model` code path
  - Profile's provider resolved via `providerRegistry.Get(prof.Provider)` — same pattern as `--model`
  - Config default only applies when neither `--model` nor profile set a model
- **Prompt precedence:** `--system` > profile > config default
  - Three-layer `if/else if/else` chain
  - Empty profile body (`prof.SystemPrompt == ""`) falls through to config default per D-05
  - `--system` and `--profile` compose: `--system` overrides prompt while profile's model still applies per D-03

## Decisions Made

1. **`Changed("model")` guard pattern:** Using `pflag.CommandLine.Changed("model")` instead of `*modelName != ""` to detect explicit `--model` usage. This prevents two pitfalls: (a) profile-set model name triggering the `--model` provider/slash parsing branch, (b) profile's provider leaking when `--model` is explicitly passed.

2. **Temporary `_ = prof` for Task 1 compilation:** Added `_ = prof` placeholder in Task 1 since Go rejects unused variables, removed in Task 2 when `prof` was used in precedence chains.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `_ = prof` placeholder for Task 1 compilation**
- **Found during:** Task 1
- **Issue:** Go compiler rejects unused variables; `prof` was declared in Task 1 but not used until Task 2
- **Fix:** Added `_ = prof // Used in model/prompt precedence chains below.` — removed in Task 2
- **Files modified:** main.go
- **Commit:** 2a630b1

## Out-of-Scope Observations

Pre-existing test failures in `internal/tool` package (7 tests) related to macOS-specific `/etc` path checks and symlinks. Not caused by this plan's changes — verified by running tests on the commit prior to changes.

## Verification Results

| Check | Result |
|-------|--------|
| `go build .` | ✅ Pass |
| `go vet ./...` | ✅ Pass |
| `go test ./... -count=1` (excluding pre-existing tool failures) | ✅ Pass |
| `fenec --help` shows `-P, --profile` | ✅ Pass |
| `profile.Load(profileDir` in main.go | ✅ Present |
| `Changed("model")` guard | ✅ Present |
| Three-layer prompt precedence | ✅ Present |
| Profile import | ✅ Present |
| Old `if *modelName != ""` pattern removed | ✅ Confirmed |

## Self-Check: PASSED
