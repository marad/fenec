# Phase 1: Foundation - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Streaming chat with a local Ollama model, formatted output, model selection. User can type a message and see the response stream token-by-token, select which Ollama model to use, and see markdown-formatted responses with syntax highlighting.

Requirements: CHAT-01, CHAT-04, CHAT-05

</domain>

<decisions>
## Implementation Decisions

### REPL interaction style
- **D-01:** Model-aware prompt showing active model name: `[gemma4]> `
- **D-02:** Single Enter sends message. Backslash at end of line continues to next line for multi-line input.
- **D-03:** Slash-prefix commands: `/quit`, `/model`, `/help`. Clear separation from messages sent to model.
- **D-04:** Non-modal key bindings — each key always does one thing. Ctrl+C cancels active generation if streaming, otherwise clears current input. Ctrl+D exits the application. No "press again to confirm" patterns. *(Amended: Escape cannot be captured during streaming since readline is not reading input. Ctrl+C handles cancellation via SIGINT instead.)*

### Streaming output presentation
- **D-05:** Animated thinking indicator (spinner/dots) shown between user pressing Enter and first token arriving. Indicator is replaced once streaming begins.
- **D-06:** Two-phase markdown rendering: tokens stream as raw text for responsiveness, then the complete response is re-rendered with glamour formatting after streaming finishes. *(Amended: No production Go library supports incremental markdown rendering during streaming — Charm's own PR #823 was closed. Two-phase approach matches Ollama's own CLI behavior.)*
- **D-07:** Blank line separator between assistant response and next prompt. No horizontal rules or timestamps.
- **D-08:** Auto-page long responses that exceed terminal height. Pause with a "more" prompt (like less/more pager). User can press Enter to continue or q to stop.

### Model selection UX
- **D-09:** Default to first available model from Ollama — no hardcoded default. Query Ollama for installed models at startup.
- **D-10:** `/model` with no args opens interactive numbered list. User picks by number. Shows which model is currently active.
- **D-11:** Conversation history preserved when switching models. New model sees all prior messages.
- **D-12:** Minimal model info — active model shown only in the `[model]>` prompt. No extra status display.

### Startup and first-run
- **D-13:** Startup banner: app name, version, help hint. Format: `fenec v0.1 — type /help for commands`
- **D-14:** If Ollama is not running or unreachable, show a clear error message with fix instructions and exit. Do not start a broken REPL.
- **D-15:** System prompt loaded from markdown file at `~/.config/fenec/system.md`. If file doesn't exist, use a sensible default. Not a config key — a standalone markdown file the user can edit directly.
- **D-16:** Connect to `localhost:11434` by default. `--host` flag to override.

### Claude's Discretion
- Thinking indicator animation style (spinner characters, frame rate)
- Exact glamour/lipgloss styling and color configuration
- Auto-pager implementation details (buffer strategy, key bindings beyond Enter/q)
- Internal message type structure
- Error message wording for non-connection errors
- readline configuration (history file location, completion settings)
- Default system prompt content when `~/.config/fenec/system.md` doesn't exist

</decisions>

<specifics>
## Specific Ideas

- Prompt should feel like a chat app prompt, not a shell — the model name in brackets gives identity without clutter
- "No modality at all — it's annoying" — each key binding has exactly one behavior regardless of state
- System prompt is a standalone markdown file (`~/.config/fenec/system.md`) so the user can edit it with any editor, not buried in a config format

</specifics>

<canonical_refs>
## Canonical References

No external specs — requirements are fully captured in decisions above and in:

- `.planning/REQUIREMENTS.md` — CHAT-01, CHAT-04, CHAT-05 requirement definitions
- `.planning/ROADMAP.md` — Phase 1 success criteria and scope
- `CLAUDE.md` — Technology stack decisions (Ollama API, readline, glamour, lipgloss)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- No existing code — greenfield project

### Established Patterns
- No patterns yet — Phase 1 establishes the foundational patterns

### Integration Points
- Ollama API at localhost:11434 — external runtime dependency
- `~/.config/fenec/` — user configuration directory (system prompt lives here)

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-04-11*
