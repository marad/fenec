# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Fenec Platform Foundation

**Shipped:** 2026-04-12
**Phases:** 6 | **Plans:** 14 | **Commits:** 102

### What Was Built
- Streaming CLI chat with local Ollama models (Gemma 4 support)
- Multi-turn conversation management with context tracking and auto-truncation
- Session persistence with atomic writes and auto-save
- Tool system: registry, agentic loop, shell_exec with dangerous-command approval
- Sandboxed Lua runtime with startup loading and error reporting
- Self-extension: agent writes, validates, and hot-reloads its own Lua tools
- File tools (read/write/edit/list) with path deny-list and CWD approval gating

### What Worked
- TDD approach in plans kept implementation quality high — tests caught real issues early
- Wave-based parallel execution reduced wall-clock time for multi-plan phases
- ApproverFunc callback pattern scaled well across tool types (shell, write, edit)
- Interface-driven design (tool.Tool, ChatService) made adding new tools trivial

### What Was Inefficient
- Phase 01 glamour rendering was built then never wired — wasted effort on a feature the user didn't want
- PageOutput auto-pager was implemented and tested but never connected to the REPL
- Both were cleaned up as tech debt, but the effort to build them was unnecessary

### Patterns Established
- All tools implement `tool.Tool` interface (Name, Definition, Execute)
- ApproverFunc callback for permission gating (deferred via closure in main.go)
- Tool results are JSON strings returned to the model
- Path safety: IsDeniedPath before IsOutsideCWD (deny-before-approve ordering)
- Registry provenance tracking (built-in vs Lua) for safe self-extension

### Key Lessons
1. Ask about user preferences for UI/rendering BEFORE building — the glamour deviation could have been caught in discuss phase
2. Pure Go dependencies (gopher-lua instead of cgo LuaJIT) pay off for build simplicity
3. Closure-based wiring in main.go (approver, notifier, replRef) is clean but fragile if initialization order changes

### Cost Observations
- Model mix: ~80% opus (planning + execution), ~20% sonnet (verification + integration checks)
- Sessions: 1 (full autonomous run)
- Notable: Autonomous mode completed entire milestone in a single session

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 102 | 6 | First milestone — established GSD workflow patterns |

### Cumulative Quality

| Milestone | Go LOC | Test Files | Packages |
|-----------|--------|------------|----------|
| v1.0 | 6,970 | 15+ | 7 |

### Top Lessons (Verified Across Milestones)

1. Validate user preferences for visible features before implementation
2. Interface-driven tool design enables painless extensibility
