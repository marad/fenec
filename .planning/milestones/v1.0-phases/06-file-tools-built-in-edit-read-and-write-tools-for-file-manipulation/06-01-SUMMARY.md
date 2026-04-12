---
phase: 06-file-tools
plan: 01
subsystem: tool
tags: [pathcheck, file-read, directory-listing, security, deny-list, symlink]

# Dependency graph
requires:
  - phase: 03-tool-execution
    provides: Tool interface, Registry, errorJSON helper
provides:
  - IsDeniedPath and IsOutsideCWD shared path safety functions
  - ReadFileTool with offset/limit and binary detection
  - ListDirTool with dirs-first sorted output
affects: [06-02-write-tools]

# Tech tracking
tech-stack:
  added: []
  patterns: [path-deny-list-with-safe-prefix-matching, symlink-resolution-for-path-safety, binary-file-detection, dirs-first-sorting]

key-files:
  created:
    - internal/tool/pathcheck.go
    - internal/tool/pathcheck_test.go
    - internal/tool/read.go
    - internal/tool/read_test.go
    - internal/tool/listdir.go
    - internal/tool/listdir_test.go
  modified: []

key-decisions:
  - "Safe prefix matching with separator prevents /etcetera matching /etc deny prefix"
  - "Fail-closed on path resolution errors -- deny access when symlinks or paths cannot be resolved"
  - "Truncated flag reflects whether more lines exist beyond what was returned, regardless of explicit/default limit"

patterns-established:
  - "Path safety pattern: IsDeniedPath check before any file I/O in every file tool"
  - "Read-only tool pattern: no approver needed, struct is empty (no dependencies)"
  - "TDD for tool development: RED failing test, GREEN minimal implementation, verify"

requirements-completed: [FILE-01, FILE-02, FILE-04]

# Metrics
duration: 3min
completed: 2026-04-11
---

# Phase 06 Plan 01: Path Safety and Read-Only File Tools Summary

**Path deny-list with symlink resolution, ReadFileTool with offset/limit/binary detection, ListDirTool with dirs-first sorting**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-11T21:21:55Z
- **Completed:** 2026-04-11T21:25:47Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- Path safety module with deny-list covering /etc, /usr, /bin, /sbin, /boot, ~/.ssh, ~/.gnupg
- Safe prefix matching preventing false positives (e.g. /etcetera not matching /etc)
- Symlink resolution preventing bypass via symbolic links into denied areas
- ReadFileTool with offset/limit parameters, 1000-line default truncation, and binary file detection
- ListDirTool with sorted output (directories first, files second, both alphabetical) including name/is_dir/size

## Task Commits

Each task was committed atomically:

1. **Task 1: Path safety module (pathcheck.go)** - `cd8aac8` (test) + `ae01d76` (feat)
2. **Task 2: ReadFileTool (read.go)** - `7179cb6` (test) + `c0a5d26` (feat)
3. **Task 3: ListDirTool (listdir.go)** - `cc58e20` (test) + `3fc5b70` (feat)

_Note: TDD tasks have two commits each (RED test then GREEN implementation)_

## Files Created/Modified
- `internal/tool/pathcheck.go` - IsDeniedPath and IsOutsideCWD shared safety functions with symlink resolution
- `internal/tool/pathcheck_test.go` - 16 tests covering deny list, false positives, symlinks, CWD checks
- `internal/tool/read.go` - ReadFileTool with offset/limit, binary detection, 1MB scanner buffer
- `internal/tool/read_test.go` - 9 tests covering reads, truncation, binary, denied paths, edge cases
- `internal/tool/listdir.go` - ListDirTool with dirs-first sorting, graceful entry error handling
- `internal/tool/listdir_test.go` - 7 tests covering sorting, entry fields, denied paths, empty/missing dirs

## Decisions Made
- Safe prefix matching uses `prefix + string(filepath.Separator)` to prevent /etcetera matching /etc
- Fail-closed on path resolution errors: deny access when symlinks or paths cannot be resolved
- Truncated flag reflects whether more lines exist beyond returned lines, regardless of explicit/default limit
- Binary detection checks first 512 bytes for null bytes, returns error JSON rather than binary content
- ListDirTool sets size=0 for directories, skips entries on individual stat errors

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all tools are fully implemented with real functionality.

## Next Phase Readiness
- Path safety module (IsDeniedPath, IsOutsideCWD) ready for Plan 02's write tools
- errorJSON and getOptionalInt helpers available for reuse in write_file and edit_file tools
- All 32 new tests pass alongside 51 existing tool tests (83 total, zero regressions)

## Self-Check: PASSED

All 6 created files verified on disk. All 6 commit hashes verified in git log.

---
*Phase: 06-file-tools*
*Completed: 2026-04-11*
