# Phase 15: Clear Command - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 15-clear-command
**Areas discussed:** Pre-clear save behavior, Token tracker reset, User feedback

---

## Pre-clear Save Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Named timestamped file via Store.Save() | Each /clear creates a permanent session file — guarantees no data loss, matches CONV-02 | ✓ |
| AutoSave only | Reuse existing _autosave.json mechanism — simpler, but next autosave overwrites it | |

**User's choice:** Named timestamped file via Store.Save()
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Skip save if no user content | Only system prompt exists, nothing to preserve — Session.HasContent() already does this check | ✓ |
| Always save regardless | Even empty conversations get a file | |

**User's choice:** Skip save if no user content
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| New session ID | /clear creates a fresh Session with new timestamp ID, so subsequent /save lands in a new file — clean separation | ✓ |
| Keep same session ID | Continue appending to same session file after clear | |

**User's choice:** New session ID
**Notes:** None

---

## Token Tracker Reset

| Option | Description | Selected |
|--------|-------------|----------|
| Zero both counters on clear | ContextTracker gets a Reset() method that zeroes lastPromptEval and lastEval — directly prevents phantom truncation | ✓ |
| Replace tracker entirely | Create a fresh ContextTracker instance | |

**User's choice:** Zero both counters via new Reset() method
**Notes:** None

---

## User Feedback

| Option | Description | Selected |
|--------|-------------|----------|
| Confirmation with save path | e.g. "Conversation saved: 2026-04-15T07-15-09 (12 messages). Session cleared." — user knows where data went | ✓ |
| Minimal confirmation | Just "Session cleared." | |
| Silent | No output, just a fresh prompt | |

**User's choice:** Confirmation with save path
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Skip save message when empty | Just print "Session cleared." if there was nothing to save — no confusing "saved 0 messages" | ✓ |
| Always show both lines regardless | Show save + clear messages even when empty | |

**User's choice:** Skip save message when empty
**Notes:** None

---

## Agent's Discretion

- sync.Once reset mechanism for autoSave
- /clear support in pipe mode
- Internal ordering of save → reset → new session

## Deferred Ideas

None — discussion stayed within phase scope
