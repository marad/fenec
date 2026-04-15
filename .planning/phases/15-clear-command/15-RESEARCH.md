# Phase 15: Clear Command - Research

**Researched:** 2026-04-15
**Domain:** Go REPL state management — conversation reset with persistence
**Confidence:** HIGH

## Summary

Phase 15 adds a `/clear` command to the Fenec REPL that resets the conversation mid-session without losing previous context or breaking REPL state. The implementation touches four REPL struct fields (`conv`, `session`, `tracker`, `autoSaved`) and requires a new `Reset()` method on `ContextTracker`.

The codebase is well-structured for this change. The existing `handleSaveCommand()` provides a reusable pattern for syncing conversation→session and calling `Store.Save()`. The `NewConversation()` and `NewSession()` constructors create clean initial state. The `refreshSystemPrompt()` method (used for tool hot-reload) already rebuilds the system prompt with tool descriptions — it can be reused after clear to ensure tools remain functional. The `baseSystemPrompt` field stores the pre-tool-description prompt, which is exactly what's needed to reconstruct the system prompt.

**Primary recommendation:** Implement `/clear` as a self-contained handler method `handleClearCommand()` in `repl.go` that: (1) syncs and saves via `Store.Save()`, (2) creates fresh `Session` and `Conversation`, (3) resets the `ContextTracker` and `sync.Once`, (4) rebuilds the system prompt with tool descriptions via `refreshSystemPrompt()`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** `/clear` saves the current conversation to a named timestamped file via `Store.Save()` before resetting — each clear creates a permanent, recoverable session file
- **D-02:** Skip save if conversation has no user content (system prompt only) — reuse `Session.HasContent()` guard
- **D-03:** After clear, create a fresh `Session` with a new timestamp-based ID — clean separation between pre- and post-clear sessions
- **D-04:** Zero both `lastPromptEval` and `lastEval` counters on clear — add a `Reset()` method to `ContextTracker` to prevent phantom truncation on fresh conversation
- **D-05:** When conversation had content: print "Conversation saved: {session-id} ({N} messages). Session cleared."
- **D-06:** When conversation was empty (no user messages): print only "Session cleared." — no confusing save message

### Agent's Discretion
- How to reset `sync.Once` for autoSave (likely replace with a new `sync.Once` instance or use a different mechanism)
- Whether `/clear` in pipe mode is supported or ignored (pipe mode currently only supports `/tools`, `/help`, `/history`)
- Internal ordering of save → reset → new session creation

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONV-01 | User can type `/clear` in REPL to reset conversation to initial state | Command routing via `ParseCommand()` → switch in `Run()` loop; fresh `Conversation` via `NewConversation()` with rebuilt system prompt |
| CONV-02 | Previous session auto-saves to named file before clear (no data loss) | `Store.Save(session)` with `HasContent()` guard; existing pattern from `handleSaveCommand()` |
| CONV-03 | System prompt and tool descriptions preserved after clear (tools remain functional) | `refreshSystemPrompt()` rebuilds system prompt from `baseSystemPrompt` + `registry.Describe()`; already proven in tool hot-reload |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `/clear` command parsing | REPL (CLI) | — | Slash commands are parsed and dispatched entirely within the REPL loop |
| Pre-clear session save | REPL → Session Store | Filesystem | REPL syncs conv→session, Store writes JSON atomically to disk |
| Conversation reset | REPL (in-memory) | — | Creates new `Conversation` struct, replaces `r.conv` pointer |
| Token tracker reset | REPL → ContextTracker | — | New `Reset()` method zeroes counters in the tracker |
| System prompt reconstruction | REPL | Tool Registry | `refreshSystemPrompt()` reads `baseSystemPrompt` + `registry.Describe()` |
| autoSave `sync.Once` reset | REPL (in-memory) | — | Replace `r.autoSaved` with a fresh `sync.Once{}` value |

## Standard Stack

No new dependencies needed. Phase uses existing codebase components:

### Core (existing)
| Component | Location | Purpose |
|-----------|----------|---------|
| `chat.NewConversation()` | `internal/chat/message.go` | Creates fresh conversation with system prompt |
| `session.NewSession()` | `internal/session/session.go` | Creates session with timestamp-based ID |
| `session.Store.Save()` | `internal/session/store.go` | Atomic JSON write to `{id}.json` |
| `session.Session.HasContent()` | `internal/session/session.go` | Guards against saving empty sessions |
| `chat.ContextTracker` | `internal/chat/context.go` | Token tracking — needs new `Reset()` method |
| `REPL.refreshSystemPrompt()` | `internal/repl/repl.go:779` | Rebuilds system prompt with tool descriptions |
| `REPL.baseSystemPrompt` | `internal/repl/repl.go:42` | Pre-tool-description system prompt |

## Architecture Patterns

### System Architecture Diagram

```
User types "/clear"
       │
       ▼
ParseCommand() → Command{Name: "/clear"}
       │
       ▼
Run() switch → handleClearCommand()
       │
       ├─── [HasContent?] ──YES──► Sync conv→session → Store.Save()
       │         │                      │
       │         NO                     ▼
       │         │              Print "Conversation saved: {id} ({N} messages). Session cleared."
       │         │
       │         └──────────────► Print "Session cleared."
       │
       ▼
Create fresh Session (NewSession)
Create fresh Conversation (NewConversation)
       │
       ▼
refreshSystemPrompt() → appends tool descriptions
       │
       ▼
Reset ContextTracker (zero counters)
Reset autoSaved (new sync.Once{})
       │
       ▼
Continue REPL loop with clean state
```

### Pattern 1: handleClearCommand Implementation

**What:** A method on `REPL` that orchestrates save → reset → reconstruct
**When to use:** When `/clear` is parsed in the Run() loop

```go
// handleClearCommand saves the current conversation (if non-empty) and resets to a fresh state.
func (r *REPL) handleClearCommand() {
    saved := false
    var savedID string
    var msgCount int

    // Step 1: Save if conversation has user content (D-01, D-02).
    if r.store != nil && r.session != nil {
        r.session.Messages = r.conv.Messages
        r.session.UpdatedAt = time.Now()
        if r.tracker != nil {
            r.session.TokenCount = r.tracker.TokenUsage()
        }
        if r.session.HasContent() {
            if err := r.store.Save(r.session); err != nil {
                fmt.Fprintln(r.rl.Stdout(), render.FormatError(
                    fmt.Sprintf("Save failed: %v", err)))
                // Continue with clear despite save failure — don't trap user
            } else {
                saved = true
                savedID = r.session.ID
                msgCount = len(r.session.Messages)
            }
        }
    }

    // Step 2: Build full system prompt with tool descriptions.
    systemPrompt := r.baseSystemPrompt
    if r.registry != nil {
        toolDesc := r.registry.Describe()
        if toolDesc != "" {
            systemPrompt = systemPrompt + "\n\n## Available Tools\n\n" + toolDesc
        }
    }

    // Step 3: Create fresh conversation and session (D-03).
    r.conv = chat.NewConversation(r.conv.Model, systemPrompt)
    if r.tracker != nil {
        r.conv.ContextLength = r.tracker.Available()
    }
    r.session = session.NewSession(r.conv.Model)

    // Step 4: Reset tracker and auto-save guard (D-04).
    if r.tracker != nil {
        r.tracker.Reset()
    }
    r.autoSaved = sync.Once{}

    // Step 5: User feedback (D-05, D-06).
    if saved {
        fmt.Fprintf(r.rl.Stdout(), "Conversation saved: %s (%d messages). Session cleared.\n",
            savedID, msgCount)
    } else {
        fmt.Fprintln(r.rl.Stdout(), "Session cleared.")
    }
}
```

[VERIFIED: codebase inspection of repl.go, session.go, context.go, message.go]

### Pattern 2: ContextTracker.Reset()

**What:** New method on `ContextTracker` that zeroes token counters
**When to use:** After `/clear` to prevent phantom truncation

```go
// Reset zeroes the token counters so a fresh conversation starts clean.
func (ct *ContextTracker) Reset() {
    ct.lastPromptEval = 0
    ct.lastEval = 0
}
```

[VERIFIED: fields from internal/chat/context.go lines 7-8]

### Pattern 3: sync.Once Reset via Value Replacement

**What:** Replace `r.autoSaved` with a fresh zero-value `sync.Once{}` to re-arm auto-save
**Why:** Go's `sync.Once` has no `Reset()` method. Once `Do()` has fired, the only way to re-arm is to replace the entire value. Since `autoSaved` is a value field (not a pointer), simple assignment works:

```go
r.autoSaved = sync.Once{}
```

This is safe because:
1. `handleClearCommand` runs on the REPL goroutine (the `Run()` loop) — same goroutine that calls `autoSave()` on defer
2. No concurrent access during clear — streaming is not active when processing commands
3. The old `sync.Once` becomes garbage when replaced

[VERIFIED: sync.Once is a struct value on REPL (repl.go:40), not a pointer. Assignment creates a fresh zero-value.]

### Anti-Patterns to Avoid

- **Reusing refreshSystemPrompt() for the full reset:** `refreshSystemPrompt()` only updates `Messages[0].Content` — it doesn't create a new conversation or session. Don't try to make it do more than its job.
- **Calling autoSave() before clear:** Don't trigger auto-save (which writes to `_autosave.json`) — use `Store.Save()` which writes to a named `{id}.json` file per D-01.
- **Forgetting to preserve Think and ContextLength:** The fresh conversation needs `r.conv.Think` preserved (if thinking was enabled) and `r.conv.ContextLength` set from the tracker.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| System prompt with tools | Manual string concatenation in clear handler | `refreshSystemPrompt()` or replicate the exact pattern from `NewREPL` (lines 68-73) | Tool description format must be consistent across init and clear |
| Session persistence | Custom file writing | `Store.Save(session)` with atomic JSON writes | Already handles temp file + rename atomically |
| Empty session detection | Custom message counting | `Session.HasContent()` | Already returns `len(s.Messages) > 1` |

## Common Pitfalls

### Pitfall 1: Forgetting to preserve `conv.Think`
**What goes wrong:** After `/clear`, the `Think` flag is `false` (default) even though the user started in interactive mode with thinking enabled.
**Why it happens:** `NewConversation()` creates a conversation with `Think: false`. The `EnableThink()` call happens only once at startup in `main.go`.
**How to avoid:** Capture `r.conv.Think` before creating the new conversation, then set it on the new one. Or use a dedicated field on REPL (but Think is currently only on Conversation).
**Warning signs:** After `/clear`, model responses no longer show thinking output in interactive mode.

### Pitfall 2: sync.Once not re-arming for auto-save
**What goes wrong:** After `/clear`, the exit-time auto-save (`defer r.autoSave()` in `Run()`) doesn't fire because `sync.Once` already ran (or was already consumed).
**Why it happens:** If auto-save fired for the pre-clear session, the `sync.Once` won't fire again for the post-clear session.
**How to avoid:** Replace `r.autoSaved` with `sync.Once{}` during clear. This re-arms the auto-save for the new session.
**Warning signs:** After clear, quitting the REPL doesn't auto-save the new conversation.

### Pitfall 3: Stale ContextLength after clear
**What goes wrong:** The new conversation has `ContextLength = 0` causing the model to use its default (possibly different) context window.
**Why it happens:** `NewConversation()` doesn't set `ContextLength`. It was set from the tracker in `NewREPL`.
**How to avoid:** After creating the new conversation, set `r.conv.ContextLength = r.tracker.Available()` (same as NewREPL line 80-82).
**Warning signs:** Context truncation behavior changes unexpectedly after clear.

### Pitfall 4: Race condition during clear
**What goes wrong:** Theoretically, if clear runs while streaming, state could be inconsistent.
**Why it happens:** Commands are processed in the same `for` loop as `sendMessage` — they're mutually exclusive.
**How to avoid:** This is already safe by design — the REPL loop processes one input at a time. Commands only execute when the model is not streaming. No additional locking needed.
**Warning signs:** N/A — this is a non-issue by architecture.

### Pitfall 5: Pipe mode `/clear` behavior
**What goes wrong:** `/clear` in pipe mode (via `runPipeLineByLine`) falls into the `default` case printing "Unsupported in pipe mode: /clear".
**Why it happens:** Pipe mode has its own command switch (repl.go:241-249) that only handles `/tools`, `/help`, `/history`.
**How to avoid:** Decision: either add `/clear` to pipe mode's switch or leave it unsupported (agent's discretion per CONTEXT.md). Recommendation: leave unsupported — pipe mode is non-interactive, and "clear" is an interactive concept.
**Warning signs:** Users piping input that includes `/clear` get an error message.

## Code Examples

### Adding /clear to the command switch

```go
// In Run() method, add to the switch statement (after /tools case):
case "/clear":
    r.handleClearCommand()
```

[VERIFIED: switch statement at repl.go:153-171]

### Adding /clear to help text

```go
// In helpText constant, add after /history line:
const helpText = `Available commands:
  /help    - Show this help message
  /model              - List models or switch: /model [provider/]name
  /save    - Save current conversation to disk
  /load    - List and load a saved conversation
  /history - Show conversation stats (messages, tokens)
  /clear   - Save and reset conversation (start fresh)
  /tools   - List all loaded tools with provenance
  /quit    - Exit fenec
...`
```

[VERIFIED: helpText at commands.go:36-52]

### Preserving Think flag across clear

```go
// Before creating new conversation:
thinkEnabled := r.conv.Think

// After creating new conversation:
r.conv.Think = thinkEnabled
```

[VERIFIED: Think field on Conversation at message.go:10, EnableThink at repl.go:800-802]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No mid-session reset | `/clear` command with auto-save | Phase 15 | Users can reset without losing context |

**Note:** This is a new feature, not a migration.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `sync.Once{}` value assignment is safe without locks because REPL loop is single-goroutine for command dispatch | Pattern 3 | Race condition if commands can run concurrently — but they can't by design |
| A2 | `/clear` should be unsupported in pipe mode (recommendation) | Pitfall 5 | If users need it in pipe mode, they'd need a code path added there too |

## Open Questions

1. **Think flag preservation**
   - What we know: `Think` is a field on `Conversation`, set via `EnableThink()` in `main.go` only at startup
   - What's unclear: Should there be a REPL-level field for this? Currently it's the simplest approach to capture-and-restore
   - Recommendation: Capture `r.conv.Think` before creating new conversation, restore after. No struct change needed.

2. **Pipe mode /clear**
   - What we know: Pipe mode switch at repl.go:241 doesn't include `/clear`. Agent discretion per CONTEXT.md.
   - Recommendation: Leave unsupported. Print "Unsupported in pipe mode: /clear" (existing default behavior). Clear is an interactive concept.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.9.x |
| Config file | None needed — Go convention |
| Quick run command | `go test ./internal/chat/ ./internal/repl/ -count=1 -run TestClear -v` |
| Full suite command | `go test ./internal/... -count=1 -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONV-01 | `/clear` resets conversation to initial state (only system prompt remains) | unit | `go test ./internal/repl/ -run TestClearResetsConversation -v` | ❌ Wave 0 |
| CONV-01 | Help text includes `/clear` | unit | `go test ./internal/repl/ -run TestHelpTextContainsClear -v` | ❌ Wave 0 |
| CONV-02 | Pre-clear auto-save creates named session file | unit | `go test ./internal/repl/ -run TestClearSavesBeforeReset -v` | ❌ Wave 0 |
| CONV-02 | No save when conversation empty | unit | `go test ./internal/repl/ -run TestClearSkipsSaveEmpty -v` | ❌ Wave 0 |
| CONV-03 | Tools remain callable after clear (system prompt has tool descriptions) | unit | `go test ./internal/repl/ -run TestClearPreservesToolDescriptions -v` | ❌ Wave 0 |
| D-04 | Token tracker resets to zero | unit | `go test ./internal/chat/ -run TestContextTrackerReset -v` | ❌ Wave 0 |
| D-05/D-06 | Correct user feedback messages | unit | `go test ./internal/repl/ -run TestClearFeedback -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/chat/ ./internal/repl/ -count=1 -v`
- **Per wave merge:** `go test ./internal/... -count=1 -v`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/chat/context_test.go` — add `TestContextTrackerReset` / `TestContextTrackerResetZeroesCounters`
- [ ] `internal/repl/repl_test.go` — add clear command tests (may require `newTestREPL` helper extension with session store)
- No framework install needed — Go testing is built in, testify already in go.mod

## Security Domain

Not applicable for this phase. `/clear` is a local state management command with no authentication, network, cryptographic, or input validation concerns beyond what already exists in the REPL command handling.

## Sources

### Primary (HIGH confidence)
- **Codebase inspection** — all code patterns verified by reading source files directly:
  - `internal/repl/repl.go` — REPL struct, Run() loop, handleSaveCommand(), autoSave(), refreshSystemPrompt()
  - `internal/repl/commands.go` — ParseCommand(), helpText
  - `internal/chat/message.go` — Conversation struct, NewConversation()
  - `internal/chat/context.go` — ContextTracker struct, field names
  - `internal/session/session.go` — Session struct, NewSession(), HasContent()
  - `internal/session/store.go` — Store.Save(), atomic writes
  - `internal/repl/repl_test.go` — newTestREPL helper, existing test patterns

### Secondary (MEDIUM confidence)
- Go `sync.Once` semantics: zero-value is ready to use, no Reset() method exists — replace with value assignment [ASSUMED: standard Go knowledge, well-documented behavior]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all components exist in codebase, no new dependencies
- Architecture: HIGH — all integration points inspected, patterns proven (handleSaveCommand, refreshSystemPrompt)
- Pitfalls: HIGH — identified from actual code paths (Think flag, sync.Once, ContextLength)

**Research date:** 2026-04-15
**Valid until:** 2026-05-15 (stable — pure Go codebase, no external dependency changes)
