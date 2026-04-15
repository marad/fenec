---
phase: 14-config-path-migration
reviewed: 2026-04-15T08:59:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/config/config.go
  - internal/config/config_test.go
  - main.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-04-15T08:59:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Phase 14 migrates the fenec config directory from the platform-specific `os.UserConfigDir()` (which returns `~/Library/Application Support` on macOS) to a consistent `~/.config/fenec` path on all platforms, with automatic one-time migration from the legacy macOS path via `os.Rename`.

The implementation is clean, well-tested (14 config tests, 5 new migration tests), and correctly ordered in `main.go` (migration before config access). The `doMigrate` function is properly dependency-injected with `io.Writer` for testable stderr output. Test isolation is solid using `t.TempDir()` + `t.Setenv("HOME", ...)`.

One warning-level issue found in error handling, and one informational observation. No critical issues.

## Warnings

### WR-01: Incomplete `os.Stat` error handling on legacy path in `doMigrate`

**File:** `internal/config/config.go:91`
**Issue:** The legacy-path existence check uses `os.IsNotExist(err)` to decide whether to skip migration. If `os.Stat(legacy)` returns a non-nil error that is NOT `IsNotExist` (e.g., `ELOOP` from a symlink loop, or an I/O error on a flaky filesystem), the function falls through and attempts `os.Rename`. The rename will also fail, but the error message ("failed to migrate config") obscures the root cause (the Stat error). The original Stat error is silently discarded.

While the current behavior is defensible (aggressive migration — only skip when legacy is provably absent), it violates the Go idiom of checking `err != nil` first and creates a debugging pitfall: a user with a broken symlink at the legacy path would see a confusing rename error instead of a clear indication that the legacy path couldn't be inspected.

**Fix:**
```go
// Check if legacy directory exists.
if _, err := os.Stat(legacy); err != nil {
    return // No legacy data or path not accessible, nothing to migrate
}
```

If the intent is to distinguish "doesn't exist" from "exists but broken," handle both cases explicitly:

```go
if _, err := os.Stat(legacy); err != nil {
    if !os.IsNotExist(err) {
        fmt.Fprintf(w, "fenec: cannot inspect legacy config path: %v\n", err)
    }
    return
}
```

## Info

### IN-01: TOCTOU gap between stat checks and rename in `doMigrate`

**File:** `internal/config/config.go:91-107`
**Issue:** There is a time-of-check-to-time-of-use gap between the `os.Stat` checks (lines 91–98) and the `os.Rename` call (line 107). If two fenec processes start simultaneously, both could pass the stat checks and race on the rename. In practice this is benign: `os.Rename` is atomic on POSIX, so one process succeeds and the other gets an error that is caught and reported. No data loss or corruption is possible. Noted for completeness only — no fix needed given this is a one-shot startup migration.

---

_Reviewed: 2026-04-15T08:59:00Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
