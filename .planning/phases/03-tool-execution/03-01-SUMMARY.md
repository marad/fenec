---
phase: 03-tool-execution
plan: 01
subsystem: tool-system
tags: [tool-registry, shell-exec, safety, ollama-api, process-management]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: Ollama API client types (api.Tool, api.ToolCall, api.ToolCallFunctionArguments)
provides:
  - Tool interface for generic tool registration and dispatch
  - Registry struct for managing tools and generating Ollama API tool definitions
  - ShellTool for executing shell commands with timeout and safety gates
  - IsDangerous function for detecting destructive commands
  - ApproverFunc callback pattern for user approval of dangerous operations
affects: [03-02-tool-execution, 04-lua-scripting]

# Tech tracking
tech-stack:
  added: [os/exec, syscall, context.WithTimeout]
  patterns: [Tool interface for extensibility, Registry dispatch pattern, ApproverFunc callback gate, process group management via Setpgid, output truncation at 4096 chars]

key-files:
  created:
    - internal/tool/registry.go
    - internal/tool/registry_test.go
    - internal/tool/shell.go
    - internal/tool/shell_test.go
    - internal/tool/safety.go
    - internal/tool/safety_test.go
  modified: []

key-decisions:
  - "Tool interface uses api.ToolCallFunctionArguments directly rather than map[string]any for type safety"
  - "Registry.Tools() returns nil for empty registry rather than empty slice"
  - "ShellTool uses process group management (Setpgid) with WaitDelay for clean timeout kills"
  - "IsDangerous uses substring matching on dangerousPatterns list rather than regex for simplicity and speed"
  - "Nil approver means deny-all for dangerous commands (secure by default)"

patterns-established:
  - "Tool interface: Name()/Definition()/Execute() contract for all tools"
  - "Registry dispatch: map-based lookup with structured error for unknown tools"
  - "Safety gate pattern: IsDangerous check then ApproverFunc callback before execution"
  - "ShellResult JSON: structured output with stdout/stderr/exit_code/timed_out"

requirements-completed: [TOOL-01, TOOL-02, TOOL-03, EXEC-01, EXEC-02, EXEC-03]

# Metrics
duration: 3min
completed: 2026-04-11
---

# Phase 3 Plan 1: Tool Registry and Shell Execution Summary

**Tool registry with generic interface for extensibility, shell_exec tool with timeout enforcement and dangerous-command safety gates using ApproverFunc callback pattern**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-11T14:00:49Z
- **Completed:** 2026-04-11T14:03:39Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Tool interface designed for Phase 4 Lua extensibility with Name/Definition/Execute contract
- Registry provides register, list, dispatch, and describe operations with structured error handling
- ShellTool captures stdout, stderr, exit code with output truncation at 4096 characters
- Timeout enforcement via context.WithTimeout with process group management (Setpgid + WaitDelay)
- Dangerous command detection covers rm, sudo, chmod, mv, kill, redirects, package managers
- ApproverFunc callback pattern for user approval of dangerous commands, nil = deny-all (secure default)

## Task Commits

Each task was committed atomically:

1. **Task 1: Tool registry with interface, registration, listing, and dispatch** - `08619c9` (feat)
2. **Task 2: Shell execution tool with timeout and safety gates** - `a177ed9` (feat)

## Files Created/Modified
- `internal/tool/registry.go` - Tool interface, Registry struct with Register/Tools/Dispatch/Describe
- `internal/tool/registry_test.go` - 6 tests for registry behavior (register, dispatch, unknown, error, describe, empty)
- `internal/tool/shell.go` - ShellTool implementing Tool interface, ShellResult, executeShell with timeout
- `internal/tool/shell_test.go` - 12 tests covering echo, stderr, exit codes, timeout, dangerous approved/denied, definition, truncation
- `internal/tool/safety.go` - IsDangerous function, ApproverFunc type, dangerousPatterns list
- `internal/tool/safety_test.go` - 10 tests for dangerous and safe command detection

## Decisions Made
- Tool interface uses `api.ToolCallFunctionArguments` directly (not `map[string]any`) for type safety with the Ollama API
- Registry returns nil slice from `Tools()` on empty registry rather than empty slice (idiomatic Go, no allocation)
- ShellTool uses process group management (`Setpgid: true`) with `WaitDelay = 5s` for clean timeout kills of child process trees
- IsDangerous uses substring matching rather than regex for simplicity and speed -- sufficient for the pattern set
- Nil approver means deny-all for dangerous commands (secure by default -- explicit opt-in required)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- Tool interface ready for Phase 4 Lua tool integration (no shell-specific assumptions)
- Registry ready for 03-02 plan to wire into the agentic chat loop
- ShellTool ready to be registered and dispatched by the model via tool calling

## Self-Check: PASSED

All 7 files verified present. Both task commits (08619c9, a177ed9) verified in git log.

---
*Phase: 03-tool-execution*
*Completed: 2026-04-11*
