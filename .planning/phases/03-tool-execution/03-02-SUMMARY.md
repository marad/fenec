---
phase: 03-tool-execution
plan: 02
subsystem: tool-system
tags: [agentic-loop, tool-calling, ollama-api, repl, shell-exec, streaming]

# Dependency graph
requires:
  - phase: 03-tool-execution
    plan: 01
    provides: Tool interface, Registry, ShellTool, ApproverFunc, IsDangerous
  - phase: 02-conversation
    provides: ChatService interface, Conversation type, ContextTracker, REPL with streaming
provides:
  - StreamChat with tools parameter passing tool definitions to Ollama API
  - StreamChat tool call accumulation from streaming response Done chunk
  - Conversation.AddRawMessage for preserving assistant messages with ToolCalls
  - Conversation.AddToolResult for appending tool result messages
  - Agentic loop in REPL sendMessage with dispatch/feed/re-send cycle
  - Dangerous command approval prompt via readline
  - Tool registry wiring in main.go with shell_exec registered
  - System prompt injection of tool descriptions
affects: [04-lua-scripting]

# Tech tracking
tech-stack:
  added: []
  patterns: [agentic tool-call loop with max rounds, closure-based approver wiring, finalMsg pattern for preserving ToolCalls from streaming, tool description injection into system prompt]

key-files:
  created: []
  modified:
    - internal/chat/client.go
    - internal/chat/stream.go
    - internal/chat/stream_test.go
    - internal/chat/message.go
    - internal/repl/repl.go
    - internal/repl/commands.go
    - main.go
    - internal/config/config.go

key-decisions:
  - "StreamChat captures full api.Message on Done chunk rather than constructing new Message, preserving ToolCalls"
  - "Fallback to accumulated content when no Done chunk received (backward compatible with mocks)"
  - "ApproveCommand is exported on REPL, wired to ShellTool via closure in main.go to break initialization cycle"
  - "Max 10 tool rounds with forced summary request when limit reached"
  - "Tool descriptions appended to system prompt with ## Available Tools header"

patterns-established:
  - "Agentic loop: for round < maxToolRounds { stream -> check tool calls -> dispatch -> feed results -> re-send }"
  - "Tool call indicators: [tool: name] command, [result: N bytes] for user visibility"
  - "Closure-based approver wiring: main.go creates closure, sets after REPL creation"
  - "finalMsg pattern: capture resp.Message on Done, overlay accumulated content"

requirements-completed: [TOOL-01, TOOL-02, TOOL-03, EXEC-01, EXEC-02]

# Metrics
duration: 5min
completed: 2026-04-11
---

# Phase 3 Plan 2: Agentic Tool-Call Loop Summary

**Agentic loop wired in REPL with StreamChat tool passing, tool call dispatch/result feeding, dangerous command approval via readline, and shell_exec registered in main.go**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-11T14:05:42Z
- **Completed:** 2026-04-11T14:10:59Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- StreamChat now accepts and passes tool definitions to Ollama API, and returns ToolCalls from model responses
- REPL sendMessage implements full agentic loop: detect tool calls, dispatch via registry, feed results back, re-send
- Dangerous shell commands prompt user for Y/n approval via readline before executing
- Tool registry created in main.go with shell_exec tool registered and approval closure wired
- System prompt includes tool descriptions so model knows what tools are available
- Max 10 tool rounds prevents infinite loops, with forced summary on limit

## Task Commits

Each task was committed atomically:

1. **Task 1: Update chat layer for tool call support** - `15fcdbc` (feat)
2. **Task 2: Wire agentic loop in REPL and main.go** - `91ac67f` (feat)

## Files Created/Modified
- `internal/chat/client.go` - ChatService interface updated with tools parameter on StreamChat
- `internal/chat/stream.go` - StreamChat implementation passes tools to ChatRequest, captures finalMsg with ToolCalls
- `internal/chat/stream_test.go` - Updated all existing tests for new signature, added TestStreamChatToolCalls and TestStreamChatPassesTools
- `internal/chat/message.go` - Added AddRawMessage and AddToolResult methods on Conversation
- `internal/repl/repl.go` - Agentic loop in sendMessage, registry field, ApproveCommand method, tool description injection
- `internal/repl/commands.go` - Help text updated with Tools section
- `main.go` - Registry creation, ShellTool registration, approval closure wiring
- `internal/config/config.go` - Default system prompt updated to mention tool/shell capabilities

## Decisions Made
- StreamChat captures full `api.Message` on Done chunk (preserving ToolCalls) with fallback for no-Done-chunk mocks
- ApproveCommand exported on REPL, wired to ShellTool via closure in main.go to break the initialization dependency cycle
- Max 10 tool rounds to prevent infinite loops; forced summary request when limit hit
- Tool descriptions appended to system prompt under `## Available Tools` header

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed missing content on no-Done-chunk responses**
- **Found during:** Task 1 (chat layer updates)
- **Issue:** Existing mock tests don't always send Done=true; new finalMsg pattern returned empty content for those tests
- **Fix:** Added fallback after Chat call: if finalMsg.Role is empty, set Role and Content from accumulated builder
- **Files modified:** internal/chat/stream.go
- **Verification:** All 40 chat tests pass including existing ones that don't send Done chunks
- **Committed in:** 15fcdbc (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Essential backward-compatibility fix. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- Agentic loop complete, model can now call shell_exec and receive results
- Tool interface and registry ready for Phase 4 Lua tool integration
- ApproverFunc pattern established for any future tools needing user approval

## Self-Check: PASSED

All 8 modified files verified present. Both task commits (15fcdbc, 91ac67f) verified in git log.

---
*Phase: 03-tool-execution*
*Completed: 2026-04-11*
