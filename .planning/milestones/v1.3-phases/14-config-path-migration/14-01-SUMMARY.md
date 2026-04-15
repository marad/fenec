---
phase: 14-config-path-migration
plan: 01
subsystem: config
tags: [config, migration, xdg, macos, tdd]
dependency_graph:
  requires: []
  provides: [ConfigDir-new-path, MigrateIfNeeded, doMigrate, legacyConfigDir]
  affects: [main.go-startup, all-config-consumers]
tech_stack:
  added: []
  patterns: [HOME-based-config-path, atomic-rename-migration, io.Writer-for-testable-stderr]
key_files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - main.go
decisions:
  - "os.UserHomeDir + hardcoded .config path over os.UserConfigDir — consistent cross-platform behavior"
  - "doMigrate accepts io.Writer for testable stderr output — no global state in tests"
  - "Migration is silent no-op when legacy path absent — zero friction for non-macOS and new installs"
metrics:
  duration: 5min
  completed: 2026-04-15T06:54:01Z
  tasks: 3
  files: 3
---

# Phase 14 Plan 01: Config Path Migration Summary

Standardized fenec config directory to `~/.config/fenec` on all platforms using `os.UserHomeDir()` with automatic one-time migration from legacy macOS path via atomic `os.Rename`.

## What Was Done

### Task 1: Failing tests for config path change and migration (RED)
- Updated `TestConfigDir` to expect `~/.config/fenec` path with HOME env override
- Added 5 new `TestDoMigrate_*` test functions covering: move, no-op, skip-existing, stderr feedback, parent dir creation
- All tests fail because `doMigrate` is undefined — RED phase confirmed
- **Commit:** `f47fc62`

### Task 2: Implement ConfigDir change and migration logic (GREEN)
- Replaced `os.UserConfigDir()` with `os.UserHomeDir()` + `filepath.Join(home, ".config", "fenec")`
- Added `legacyConfigDir()` returning macOS legacy path (empty string on non-darwin)
- Added `MigrateIfNeeded()` as the public entry point for startup
- Added `doMigrate(legacy, newDir string, w io.Writer)` with atomic rename, parent mkdir, stderr feedback
- Fixed all 6 existing tests: `XDG_CONFIG_HOME` → `HOME` env var, updated path assertions
- All 14 config tests pass — GREEN phase confirmed
- **Commit:** `0896d19`

### Task 3: Wire MigrateIfNeeded at startup
- Added `config.MigrateIfNeeded()` call in `main.go` between version check and stdin detection
- MigrateIfNeeded (line 57) runs before ConfigDir (line 68) — correct ordering
- Full config test suite passes, binary compiles
- **Commit:** `2519636`

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

| Check | Result |
|-------|--------|
| `go test ./internal/config/ -count=1` | ✅ PASS (14 tests) |
| `grep -c "XDG_CONFIG_HOME" config_test.go` | ✅ 0 references |
| `grep "os.UserConfigDir" config.go` | ✅ None found |
| `grep "os.UserHomeDir" config.go` | ✅ 2 matches |
| MigrateIfNeeded before ConfigDir in main.go | ✅ Line 57 < Line 68 |
| `go build .` | ✅ Compiles |

**Pre-existing failures:** `TestReadFileDeniedPath` and `TestWriteFileDeniedPath` in `internal/tool/` fail on macOS (test expects `/etc/shadow` which doesn't exist on darwin). Not caused by this plan — verified by running tests against pre-change codebase.

## Key Artifacts

| File | Functions | Purpose |
|------|-----------|---------|
| `internal/config/config.go` | `ConfigDir`, `legacyConfigDir`, `MigrateIfNeeded`, `doMigrate` | Config path standardization + migration |
| `internal/config/config_test.go` | 6 updated + 5 new test functions | Full coverage of migration scenarios |
| `main.go` | Startup sequence | Migration wired before config access |

## Self-Check: PASSED
