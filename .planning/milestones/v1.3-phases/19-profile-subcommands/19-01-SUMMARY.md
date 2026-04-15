---
phase: 19-profile-subcommands
plan: "01"
subsystem: cli/profile-management
tags: [cli, profiles, subcommands, editor-integration]
dependency_graph:
  requires: [16-01]
  provides: [profilecmd-package, profile-cli-surface]
  affects: [main.go]
tech_stack:
  added: []
  patterns: [pre-pflag-dispatch, dir-injection-testability, tabwriter-columns]
key_files:
  created:
    - internal/profilecmd/profilecmd.go
    - internal/profilecmd/profilecmd_test.go
  modified:
    - main.go
decisions:
  - Pre-pflag os.Args dispatch keeps profile subcommands from conflicting with pflag flag parsing
  - Dir-parameter injection pattern (runList/doCreate/doEdit accept dir) enables testing without config.ProfilesDir() dependency
  - EDITOR=true in tests for no-op editor execution on Unix
metrics:
  duration: 2min
  completed: "2026-04-15T12:22:23Z"
  tasks: 3
  files: 3
  test_count: 15
  loc_added: 335
---

# Phase 19 Plan 01: Profile Subcommands Summary

`fenec profile list|create|edit` CLI subcommands with pre-pflag dispatch, tabwriter-aligned output, path traversal protection, and $EDITOR integration

## What Was Done

### Task 1: Create profilecmd package (3a30182)
Created `internal/profilecmd/profilecmd.go` with:
- `Run()` dispatcher that handles `list`, `create`, `edit` subcommands and prints usage for unknown commands
- `runList()` with tabwriter-aligned NAME/MODEL columns and "(default)" fallback for empty models
- `doCreate()` with path traversal protection (`ContainsAny` for `/`, `\`, `.`), MkdirAll for profiles dir, existence check, template writing, and editor launch
- `doEdit()` with path traversal protection and existence check before editor launch
- `getEditor()` with `$EDITOR` env fallback to `vi`
- `openEditor()` handling multi-word editors via `strings.Fields` splitting
- Profile template constant with TOML frontmatter scaffold

### Task 2: Comprehensive tests (93c2738)
Created `internal/profilecmd/profilecmd_test.go` with 15 test cases:
- **List**: profiles with models, empty directory, empty model shows "(default)", non-existent directory
- **Create**: new profile file creation, already-exists error, invalid names (/, ., \\), directory auto-creation
- **Edit**: non-existent profile error, invalid name error, successful edit of existing profile
- **Editor**: `$EDITOR` env resolution, fallback to `vi`

### Task 3: Main.go dispatch wiring (66946cd)
- Added pre-pflag `os.Args[1] == "profile"` check before `pflag.Parse()` — routes to `profilecmd.Run(os.Args[2:])`
- Normal fenec invocation (REPL) unaffected — dispatch only triggers on `fenec profile ...`
- Help text updated with `fenec profile list` example

## Deviations from Plan

None — plan executed exactly as written.

## Verification

- `go build .` succeeds
- `go vet ./...` succeeds
- `go test ./internal/profilecmd/ -count=1` — 15/15 tests pass
- `go test ./internal/profile/ -count=1` — existing tests still pass

## Commits

| Task | Hash    | Message                                                        |
|------|---------|----------------------------------------------------------------|
| 1    | 3a30182 | feat(19-01): add profilecmd package with dispatch, list, create, edit handlers |
| 2    | 93c2738 | test(19-01): add comprehensive profilecmd tests                |
| 3    | 66946cd | feat(19-01): wire pre-pflag profile subcommand dispatch in main.go |

## Self-Check: PASSED

All 3 files exist, all 3 commits verified, no unexpected deletions.
