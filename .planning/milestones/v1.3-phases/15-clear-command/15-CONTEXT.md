# Phase 15: Clear Command - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

User can reset conversation mid-session via `/clear` REPL command without losing previous context or breaking REPL state. System prompt, tool descriptions, and token tracking all reset cleanly.

</domain>

<decisions>
## Implementation Decisions

### Pre-clear Save Behavior
- **D-01:** `/clear` saves the current conversation to a named timestamped file via `Store.Save()` before resetting — each clear creates a permanent, recoverable session file
- **D-02:** Skip save if conversation has no user content (system prompt only) — reuse `Session.HasContent()` guard
- **D-03:** After clear, create a fresh `Session` with a new timestamp-based ID — clean separation between pre- and post-clear sessions

### Token Tracker Reset
- **D-04:** Zero both `lastPromptEval` and `lastEval` counters on clear — add a `Reset()` method to `ContextTracker` to prevent phantom truncation on fresh conversation

### User Feedback
- **D-05:** When conversation had content: print "Conversation saved: {session-id} ({N} messages). Session cleared."
- **D-06:** When conversation was empty (no user messages): print only "Session cleared." — no confusing save message

### Agent's Discretion
- How to reset `sync.Once` for autoSave (likely replace with a new `sync.Once` instance or use a different mechanism)
- Whether `/clear` in pipe mode is supported or ignored (pipe mode currently only supports `/tools`, `/help`, `/history`)
- Internal ordering of save → reset → new session creation

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — CONV-01, CONV-02, CONV-03 define acceptance criteria for this phase

### Roadmap
- `.planning/ROADMAP.md` §Phase 15 — Success criteria including token tracking reset requirement

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.Store.Save(session)` — Named session persistence with atomic JSON writes, already handles `{id}.json` file creation
- `session.Session.HasContent()` — Returns true if session has user-generated content (more than system message)
- `session.NewSession(model)` — Creates session with timestamp-based ID
- `chat.NewConversation(model, systemPrompt)` — Creates conversation with system prompt message
- `repl.REPL.baseSystemPrompt` — Stores pre-tool-description system prompt for refresh scenarios

### Established Patterns
- Slash commands routed via `ParseCommand()` → `switch cmd.Name` in `Run()` loop — `/clear` follows same pattern
- `handleSaveCommand()` syncs conversation → session, then calls `Store.Save()` — reusable save pattern
- Tool descriptions appended to system prompt in `NewREPL` (lines 64-73) — must be replicated on clear
- `autoSaved` uses `sync.Once` — needs reset mechanism after clear to allow future auto-saves

### Integration Points
- `REPL.Run()` switch statement (line 153) — add `/clear` case
- `helpText` constant in `commands.go` — add `/clear` entry
- `ContextTracker` — needs new `Reset()` method
- `REPL` struct fields: `conv`, `session`, `tracker`, `autoSaved` — all touched by clear

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 15-clear-command*
*Context gathered: 2026-04-15*
