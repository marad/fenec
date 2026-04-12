---
phase: 05-self-extension
plan: 02
subsystem: tool
tags: [lua, self-extension, hot-reload, repl, tool-lifecycle]

# Dependency graph
requires:
  - phase: 05-self-extension
    provides: CreateLuaTool, UpdateLuaTool, DeleteLuaTool, ToolEventNotifier, Registry provenance tracking
provides:
  - FormatToolEvent banner notification for tool lifecycle events
  - /tools slash command with provenance tags
  - System prompt hot-reload after tool create/update/delete
  - Full self-extension tool wiring in main.go startup
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [closure-deferred-wiring, hot-reload-system-prompt]

key-files:
  created: []
  modified:
    - internal/render/style.go
    - internal/repl/repl.go
    - internal/repl/commands.go
    - main.go
    - go.mod

key-decisions:
  - "Self-extension tools registered as built-in (Register not RegisterLua) so they appear as [built-in] and cannot be deleted"
  - "Disk-loaded Lua tools changed from Register to RegisterLua for correct provenance tagging"
  - "Notifier uses closure-deferred replRef wiring (same pattern as approver) to break init cycle"
  - "baseSystemPrompt stored before tool description append to support refreshSystemPrompt"

patterns-established:
  - "Closure-deferred wiring: declare var pointer before REPL creation, assign after, use in closure"
  - "Hot-reload system prompt: rebuild system prompt message[0] content from baseSystemPrompt + registry.Describe()"

requirements-completed: [LUA-01, LUA-03]

# Metrics
duration: 2min
completed: 2026-04-11
---

# Phase 05 Plan 02: REPL Integration and Hot-Reload Summary

**Self-extension tools wired into main.go with banner notifications, /tools command, and system prompt hot-reload for end-to-end tool lifecycle**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-11T16:22:59Z
- **Completed:** 2026-04-11T16:25:10Z
- **Tasks:** 1
- **Files modified:** 5

## Accomplishments
- Wired create_lua_tool, update_lua_tool, and delete_lua_tool into main.go startup as built-in tools
- Added FormatToolEvent with muted #7AA2F7 color for tool lifecycle banner notifications
- Implemented /tools slash command showing all tools sorted by name with [built-in]/[lua] provenance tags
- Added RefreshSystemPrompt method that rebuilds conversation system message after tool changes
- Fixed disk-loaded Lua tools to use RegisterLua instead of Register for correct provenance

## Task Commits

Each task was committed atomically:

1. **Task 1: Add FormatToolEvent, /tools command, system prompt refresh, main.go wiring** - `6331b66` (feat)

## Files Created/Modified
- `internal/render/style.go` - Added toolEventStyle and FormatToolEvent function for created/updated/deleted banners
- `internal/repl/repl.go` - Added baseSystemPrompt field, RefreshSystemPrompt method, handleToolsCommand, /tools dispatch
- `internal/repl/commands.go` - Added /tools to helpText
- `main.go` - Wired 3 self-extension tools with notifier callback and closure-deferred replRef
- `go.mod` - Promoted gopher-lua and gopher-lua-libs from indirect to direct requires

## Decisions Made
- Self-extension tools registered as built-in (via Register, not RegisterLua) so they show as [built-in] and are protected from deletion
- Disk-loaded Lua tools changed from Register to RegisterLua for correct provenance tagging in /tools output
- Notifier uses closure-deferred replRef (same pattern as the existing approver closure) to break the init cycle between notifier and REPL creation
- baseSystemPrompt stored before tool descriptions are appended, enabling refreshSystemPrompt to rebuild cleanly

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 05 complete: full self-extension capability is live
- Agent can create, update, and delete Lua tools at runtime
- Tools are validated, persisted to disk, and hot-reloaded into model context
- User sees banner notifications and can inspect tools via /tools command

## Self-Check: PASSED

All 5 modified files verified on disk. Task commit 6331b66 verified in git log.

---
*Phase: 05-self-extension*
*Completed: 2026-04-11*
