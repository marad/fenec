---
phase: 04-lua-runtime
plan: 02
subsystem: lua
tags: [gopher-lua, tool-loader, startup-wiring, config]

# Dependency graph
requires:
  - phase: 04-lua-runtime-01
    provides: CompileFile, NewLuaToolFromProto, LuaTool, NewSandboxedState
  - phase: 03-tool-execution
    provides: tool.Tool interface, Registry.Register
provides:
  - LoadTools function for scanning directory and returning tools + errors
  - LoadError and LoadResult types for structured error reporting
  - ToolsDir config helper for tools directory path resolution
  - main.go startup wiring that loads Lua tools into the registry
affects: [05-self-extension]

# Tech tracking
tech-stack:
  added: []
  patterns: [directory-scan-with-partial-success, non-fatal-startup-loading]

key-files:
  created:
    - internal/lua/loader.go
    - internal/lua/loader_test.go
    - internal/lua/testdata/syntax_error.lua
    - internal/lua/testdata/returns_string.lua
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - main.go

key-decisions:
  - "ToolsDir does NOT create directory -- deferred to Phase 5 when agent writes first tool"
  - "LoadTools returns partial success: valid tools load even with broken scripts present"
  - "Lua loading is non-fatal in main.go: missing dir, scan errors, and load errors all allow app to start"

patterns-established:
  - "Directory scanner pattern: os.ReadDir + filter by extension + compile-and-validate loop"
  - "Partial success pattern: collect valid results and errors separately, return both"
  - "Non-fatal startup loading: slog.Warn/render.FormatError for errors, continue execution"

requirements-completed: [LUA-02, LUA-06]

# Metrics
duration: 3min
completed: 2026-04-11
---

# Phase 4 Plan 2: Lua Tool Loader Summary

**Directory scanner that loads .lua tools at startup with partial-success semantics and descriptive error reporting for broken scripts**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-11T15:22:34Z
- **Completed:** 2026-04-11T15:25:51Z
- **Tasks:** 2 (Task 1: TDD RED + GREEN, Task 2: wiring)
- **Files modified:** 7

## Accomplishments
- LoadTools scans a directory for .lua files, compiles and validates each via CompileFile + NewLuaToolFromProto, returning valid tools and descriptive LoadErrors
- Missing tools directory handled gracefully as zero tools (not an error)
- main.go loads Lua tools at startup, registers them alongside shell_exec, and prints load errors to stderr
- Comprehensive test coverage: 8 loader tests + 2 ToolsDir tests, all passing alongside existing 27 tests

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests for loader and ToolsDir** - `0b463e6` (test)
2. **Task 1 (GREEN): Implement loader and ToolsDir** - `2535f5c` (feat)
3. **Task 2: Wire Lua tool loading into main.go** - `22eaf5d` (feat)

_TDD task: RED committed failing tests, GREEN committed passing implementation. No REFACTOR needed._

## Files Created/Modified
- `internal/lua/loader.go` - LoadTools, LoadError, LoadResult types
- `internal/lua/loader_test.go` - 8 tests covering valid/empty/missing dirs, syntax errors, missing fields, mixed, non-lua, subdirs
- `internal/lua/testdata/syntax_error.lua` - Fixture with missing comma (Lua parse error)
- `internal/lua/testdata/returns_string.lua` - Fixture returning non-table value
- `internal/config/config.go` - Added ToolsDir helper (no MkdirAll)
- `internal/config/config_test.go` - Added TestToolsDir and TestToolsDirDoesNotCreate
- `main.go` - Lua tool loading between registry creation and REPL creation

## Decisions Made
- ToolsDir does NOT create the directory -- unlike SessionDir which calls MkdirAll, the tools directory is deferred to Phase 5 when the agent writes its first tool
- LoadTools returns partial success: valid tools load even when broken scripts exist in the same directory
- Lua loading is entirely non-fatal in main.go: ToolsDir failure logs warning, LoadTools failure prints error, individual load errors print per-file errors -- app always starts

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Lua tool loading pipeline complete end-to-end: script -> compile -> validate -> register -> available to model
- Phase 4 fully complete: sandbox (Plan 01) + loader (Plan 02) provide the runtime foundation
- Ready for Phase 5 (self-extension): agent can write .lua files to ToolsDir, and they will be loaded on next startup

## Self-Check: PASSED

- All 7 files verified present on disk
- Commit 0b463e6 (RED) verified in git log
- Commit 2535f5c (GREEN) verified in git log
- Commit 22eaf5d (Task 2) verified in git log
- 29/29 tests passing (8 loader + 2 ToolsDir + 19 existing lua/config)
- go vet clean
- Full test suite green (no regressions)

---
*Phase: 04-lua-runtime*
*Completed: 2026-04-11*
