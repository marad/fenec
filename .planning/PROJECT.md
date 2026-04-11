# Fenec

## What This Is

A personal AI assistant platform built in Go with LuaJIT extensibility. Provides a CLI chat interface that connects to local Ollama models (like Gemma 4). The agent can use tools via structured function calling and extend itself by writing Lua scripts that persist as new tools — becoming more capable over time.

## Core Value

An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## Requirements

### Validated

- [x] CLI chat interface — basic REPL for conversing with the agent (Validated in Phase 1: Foundation)
- [x] Ollama integration — connect to local models, send messages, stream responses (Validated in Phase 1: Foundation)
- [x] Multi-turn conversation — context maintained across turns, token tracking with auto-truncation (Validated in Phase 2: Conversation)
- [x] Session persistence — save/load conversations to disk, auto-save on exit (Validated in Phase 2: Conversation)
- [x] Tool system — model receives available tools in prompt, outputs structured tool calls (Validated in Phase 3: Tool Execution)
- [x] Tool execution engine — parse tool calls from model output, dispatch to handlers, return results (Validated in Phase 3: Tool Execution)
- [x] Built-in bash tool — execute shell commands and return output (Validated in Phase 3: Tool Execution)
- [x] LuaJIT integration — sandboxed Lua 5.1 VM, LuaTool adapter, startup loader (Validated in Phase 4: Lua Runtime)

### Active
- [ ] Self-extension — agent can write new Lua tools that persist to disk and become available in future sessions
- [ ] Tool discovery — agent sees all available tools (built-in + Lua) in its system prompt

### Out of Scope

- Email reading/tagging — future milestone, not part of the platform foundation
- Note search/organization — future milestone
- TUI with panels/history/status — start with basic REPL, enhance later
- Cloud/remote model providers — focus on local Ollama
- Multi-user support — personal assistant, single user
- Web or GUI interface — CLI only for now

## Context

- Ollama provides an OpenAI-compatible API for local model inference
- Gemma 4 supports tool/function calling via structured output
- LuaJIT (via gopher-lua or similar) embeds well in Go and provides fast script execution
- The self-extension pattern means the agent's tool library grows with use — Lua scripts written by the agent are saved to a tools directory and loaded on startup
- This is a foundation project — the architecture should make it straightforward to add new built-in tools and integrations in future milestones

## Constraints

- **Language**: Go — performance, single binary deployment, strong concurrency
- **Scripting**: LuaJIT — embedded scripting for tool extensibility
- **Models**: Ollama local models — no cloud dependencies
- **Interface**: CLI — simple REPL to start

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go + LuaJIT | Go for core performance and deployment simplicity, LuaJIT for lightweight embedded scripting that the agent itself can author | — Pending |
| Ollama as model backend | Local-first, no cloud costs, compatible with Gemma 4 and other open models | — Pending |
| Tool calling via prompt injection | Model sees tool list in system prompt and outputs structured calls — works with models that support function calling format | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-11 after Phase 4 completion*
