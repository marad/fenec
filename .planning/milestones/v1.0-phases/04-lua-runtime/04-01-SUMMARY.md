---
phase: 04-lua-runtime
plan: 01
subsystem: lua
tags: [gopher-lua, sandbox, luajit, tool-interface, bytecode]

# Dependency graph
requires:
  - phase: 03-tool-execution
    provides: tool.Tool interface, Registry, api.ToolCallFunctionArguments
provides:
  - NewSandboxedState factory for safe Lua VM creation
  - ArgsToLuaTable for Go-to-Lua argument conversion
  - LuaTool implementing tool.Tool interface
  - CompileFile for Lua source to bytecode compilation
affects: [04-lua-runtime-02, 05-self-extension]

# Tech tracking
tech-stack:
  added: [github.com/yuin/gopher-lua v1.1.2, github.com/vadv/gopher-lua-libs/json]
  patterns: [SkipOpenLibs sandboxing, fresh LState per execution, pre-compiled FunctionProto reuse]

key-files:
  created:
    - internal/lua/sandbox.go
    - internal/lua/convert.go
    - internal/lua/luatool.go
    - internal/lua/sandbox_test.go
    - internal/lua/luatool_test.go
    - internal/lua/helpers_test.go
    - internal/lua/testdata/word_count.lua
    - internal/lua/testdata/sandbox_escape.lua
    - internal/lua/testdata/no_execute.lua
    - internal/lua/testdata/no_name.lua
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Package named 'lua' with import alias 'glua' for gopher-lua -- reads naturally from consumer side"
  - "Fresh sandboxed LState per execution instead of shared state -- prevents cross-tool pollution"
  - "Pre-compiled FunctionProto stored on LuaTool -- avoids re-parsing on every execution"

patterns-established:
  - "SkipOpenLibs + selective open for Lua sandboxing (whitelist, not blacklist)"
  - "Nil-out dofile/loadfile after OpenBase to remove unsafe base functions"
  - "Lua tool metadata extracted by executing script in temp LState, validated before registration"
  - "glua import alias convention throughout internal/lua package"

requirements-completed: [LUA-02, LUA-04]

# Metrics
duration: 9min
completed: 2026-04-11
---

# Phase 4 Plan 1: Lua Sandbox and LuaTool Summary

**Sandboxed gopher-lua VM with whitelist-only library access and LuaTool type implementing tool.Tool for executing Lua scripts as agent tools**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-11T15:11:14Z
- **Completed:** 2026-04-11T15:20:08Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 12

## Accomplishments
- Sandboxed LState factory blocks os/io/debug and dofile/loadfile while exposing base/table/string/math/package
- LuaTool implements tool.Tool interface with Name/Definition/Execute, creating fresh sandbox per invocation
- Go-to-Lua argument conversion handles string, float64, bool, nil, and complex types via JSON fallback
- CompileFile pre-compiles Lua source to bytecode for reuse across invocations
- Context-based timeout enforcement cancels long-running Lua scripts
- Malformed scripts (missing name, missing execute) caught at construction with descriptive errors
- 19 tests covering sandbox security, value conversion, tool execution, error cases, and interface compliance

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests for sandbox, conversion, LuaTool** - `5e45d17` (test)
2. **Task 1 (GREEN): Implement sandbox, conversion, LuaTool** - `b665351` (feat)

_TDD task: RED committed failing tests, GREEN committed passing implementation. No REFACTOR needed._

## Files Created/Modified
- `internal/lua/sandbox.go` - NewSandboxedState factory with SkipOpenLibs + selective opens
- `internal/lua/convert.go` - ArgsToLuaTable for Go-to-Lua value conversion
- `internal/lua/luatool.go` - LuaTool struct, CompileFile, NewLuaToolFromProto, Name/Definition/Execute
- `internal/lua/sandbox_test.go` - 10 tests: safe libs, blocked modules, nil'd functions, timeout
- `internal/lua/luatool_test.go` - 9 tests: name, definition, execute, empty args, timeout, missing fields, conversion, interface
- `internal/lua/helpers_test.go` - writeFileHelper test utility
- `internal/lua/testdata/word_count.lua` - Valid tool fixture with text parameter
- `internal/lua/testdata/sandbox_escape.lua` - Escape attempt fixture using pcall(require, "os")
- `internal/lua/testdata/no_execute.lua` - Missing execute function fixture
- `internal/lua/testdata/no_name.lua` - Missing name field fixture
- `go.mod` / `go.sum` - Added gopher-lua v1.1.2 and gopher-lua-libs dependencies

## Decisions Made
- Package named `lua` with `glua` import alias for gopher-lua -- reads naturally as `lua.NewSandboxedState` from consumers
- Fresh sandboxed LState per execution rather than shared state -- prevents cross-tool state pollution per research recommendation
- Pre-compiled FunctionProto stored on LuaTool -- avoids re-parsing source on every execution while maintaining sandbox isolation

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- LuaTool and sandbox ready for Plan 02 (loader) to scan tools directory and register Lua tools
- CompileFile and NewLuaToolFromProto provide the building blocks the loader needs
- tool.Tool interface compliance verified -- LuaTools can be registered in existing Registry

## Self-Check: PASSED

- All 10 created files verified present on disk
- Commit 5e45d17 (RED) verified in git log
- Commit b665351 (GREEN) verified in git log
- 19/19 tests passing
- go vet clean
- Full test suite green (no regressions)

---
*Phase: 04-lua-runtime*
*Completed: 2026-04-11*
