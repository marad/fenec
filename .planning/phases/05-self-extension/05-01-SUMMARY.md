---
phase: 05-self-extension
plan: 01
subsystem: tool
tags: [lua, self-extension, tool-registry, validation, gopher-lua]

# Dependency graph
requires:
  - phase: 04-lua-runtime
    provides: LuaTool, CompileFile, NewLuaToolFromProto, sandboxed execution
provides:
  - Registry provenance tracking (built-in vs Lua tools)
  - Registry Unregister, Has, RegisterLua, IsBuiltIn, ToolInfo methods
  - create_lua_tool built-in tool with full validation pipeline
  - update_lua_tool built-in tool with atomic replace
  - delete_lua_tool built-in tool with built-in protection
  - ToolEventNotifier callback type for tool lifecycle events
affects: [05-self-extension]

# Tech tracking
tech-stack:
  added: []
  patterns: [tool-result-errors, temp-file-compile-validate, atomic-unregister-register]

key-files:
  created:
    - internal/tool/create.go
    - internal/tool/update.go
    - internal/tool/delete.go
    - internal/tool/create_test.go
    - internal/tool/update_test.go
    - internal/tool/delete_test.go
  modified:
    - internal/tool/registry.go
    - internal/tool/registry_test.go
    - internal/lua/luatool_test.go

key-decisions:
  - "Validation errors returned as JSON tool result strings, not Go errors, so model can self-correct"
  - "Temp-file compilation before disk write prevents partial/corrupt tool files"
  - "Re-compile from final path after write so LuaTool scriptPath is correct for execution"
  - "Removed compile-time tool.Tool interface check from lua tests to break import cycle"

patterns-established:
  - "Tool-result errors: user-facing errors from self-extension tools are JSON strings, not Go errors"
  - "Temp-file-validate-write: write to temp, compile+validate, then copy to final location"
  - "Atomic replace: Unregister old then RegisterLua new for tool updates"

requirements-completed: [LUA-01, LUA-05]

# Metrics
duration: 6min
completed: 2026-04-11
---

# Phase 05 Plan 01: Self-Extension Tools Summary

**Three self-extension built-in tools (create/update/delete_lua_tool) with full Lua validation pipeline and Registry provenance tracking**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-11T16:13:00Z
- **Completed:** 2026-04-11T16:19:34Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Extended Registry with provenance tracking (built-in vs Lua), unregistration, has-check, and tool info queries
- Implemented create_lua_tool that validates Lua source (syntax + schema), writes to disk, and registers for immediate use
- Implemented update_lua_tool that atomically replaces existing Lua tools with full revalidation
- Implemented delete_lua_tool that removes Lua tools from disk and registry with built-in protection
- 30 new tests covering success paths, error paths, edge cases, and notifier callbacks

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend Registry** - `b0a494b` (test: failing tests), `388264b` (feat: implementation)
2. **Task 2: Implement create/update/delete tools** - `86b1c19` (test: failing tests), `6809d4c` (feat: implementation)

_Note: TDD tasks have RED (test) and GREEN (feat) commits._

## Files Created/Modified
- `internal/tool/registry.go` - Extended with builtIn map, Unregister, Has, RegisterLua, IsBuiltIn, ToolInfo, ToolEventNotifier, ToolInfoEntry
- `internal/tool/registry_test.go` - 6 new test functions for registry extensions
- `internal/tool/create.go` - CreateLuaTool: validates Lua source, persists to disk, registers
- `internal/tool/create_test.go` - 10 tests: success, duplicate, syntax error, schema error, built-in collision, missing code, dir creation, notifier
- `internal/tool/update.go` - UpdateLuaTool: validates and atomically replaces existing Lua tools
- `internal/tool/update_test.go` - 7 tests: success, not-found, validation failure preserves original, built-in rejection, notifier
- `internal/tool/delete.go` - DeleteLuaTool: removes from disk and registry with built-in protection
- `internal/tool/delete_test.go` - 6 tests: success, not-found, built-in rejection, missing name, notifier
- `internal/lua/luatool_test.go` - Removed tool.Tool import to break import cycle

## Decisions Made
- Validation errors returned as JSON tool result strings (not Go errors) so the model can see syntax/schema errors and self-correct
- Temp-file compilation before disk write prevents partial or corrupt tool files from being persisted
- Re-compile from final path after writing so LuaTool.scriptPath is correct for later execution
- Removed compile-time `var _ tool.Tool = (*LuaTool)(nil)` check from lua test to break import cycle (tool now imports lua)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed import cycle between internal/tool and internal/lua**
- **Found during:** Task 2 (self-extension tool implementation)
- **Issue:** internal/lua/luatool_test.go imported internal/tool for compile-time interface check. With tool now importing lua, this created a cycle.
- **Fix:** Removed tool import from luatool_test.go, replaced compile-time interface check with method-based assertions
- **Files modified:** internal/lua/luatool_test.go
- **Verification:** `go test ./internal/lua/... -count=1` passes (27 tests)
- **Committed in:** 6809d4c (part of Task 2 commit)

**2. [Rule 3 - Blocking] Fixed makeArgs helper name collision**
- **Found during:** Task 2 (test file creation)
- **Issue:** shell_test.go already had makeArgs(string) in the same package; new tests needed makeArgs(map[string]interface{})
- **Fix:** Renamed new helper to makeToolArgs to avoid redeclaration
- **Files modified:** internal/tool/create_test.go, internal/tool/update_test.go, internal/tool/delete_test.go
- **Verification:** All tests compile and pass
- **Committed in:** 6809d4c (part of Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary for compilation. No scope creep.

## Issues Encountered
None beyond the deviations documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Self-extension tools ready for wiring into main.go (Plan 02)
- Registry supports all operations needed for tool lifecycle management
- ToolEventNotifier ready for UI feedback integration

## Self-Check: PASSED

All 9 created/modified files verified on disk. All 4 task commits verified in git log.

---
*Phase: 05-self-extension*
*Completed: 2026-04-11*
