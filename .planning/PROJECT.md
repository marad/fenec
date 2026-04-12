# Fenec

## What This Is

A personal AI assistant platform built in Go with Lua extensibility. Provides a CLI chat interface that connects to local Ollama models (like Gemma 4). The agent can use tools via structured function calling and extend itself by writing Lua scripts that persist as new tools — becoming more capable over time.

## Core Value

An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## Current State

Shipped **v1.0** on 2026-04-12. The platform foundation is complete:
- 6,970 lines of Go across 52 source files
- 8 built-in tools: shell_exec, read_file, write_file, edit_file, list_directory, create_lua_tool, update_lua_tool, delete_lua_tool
- Sandboxed Lua runtime with startup loading and hot-reload
- Session persistence with auto-save
- Context tracking with automatic truncation
- Path safety (deny list + CWD approval gating)

## Requirements

### Validated

- ✓ CLI chat interface — v1.0
- ✓ Ollama streaming integration — v1.0
- ✓ Multi-turn conversation with context tracking — v1.0
- ✓ Session persistence with auto-save — v1.0
- ✓ Tool system with structured function calling — v1.0
- ✓ Shell execution with approval for dangerous commands — v1.0
- ✓ Sandboxed Lua runtime with startup loading — v1.0
- ✓ Self-extension (agent writes its own tools) — v1.0
- ✓ Built-in file tools with path safety — v1.0

### Active

(No active requirements — next milestone not yet planned)

### Out of Scope

- Email reading/tagging — future milestone
- Note search/organization — future milestone
- TUI with panels/history/status — start with basic REPL, enhance later
- Cloud/remote model providers — focus on local Ollama
- Multi-user support — personal assistant, single user
- Web or GUI interface — CLI only for now

## Context

- Ollama provides an OpenAI-compatible API for local model inference
- Gemma 4 supports tool/function calling via structured output
- gopher-lua (pure Go Lua 5.1 VM) embeds well and provides fast script execution
- The self-extension pattern means the agent's tool library grows with use
- Architecture is designed for easy addition of new built-in tools and integrations

## Constraints

- **Language**: Go — performance, single binary deployment, strong concurrency
- **Scripting**: Lua 5.1 (gopher-lua) — embedded scripting for tool extensibility
- **Models**: Ollama local models — no cloud dependencies
- **Interface**: CLI — simple REPL to start

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go + gopher-lua | Go for core performance, pure-Go Lua VM for portable embedding | ✓ Good — single binary, no cgo |
| Ollama native API client | Local-first, no cloud costs, dogfooded by Ollama CLI itself | ✓ Good — full tool calling support |
| Tool calling via system prompt | Model sees tool list and outputs structured calls | ✓ Good — works with Gemma 4 |
| ApproverFunc callback pattern | Deferred approval logic allows reuse across tool types | ✓ Good — used by shell, write, edit |
| Registry with provenance | Track built-in vs Lua tools for safe self-extension | ✓ Good — prevents overwriting built-ins |
| Path deny list + CWD boundary | Layered safety for file operations | ✓ Good — deny-before-approve ordering |

## Evolution

This document evolves at phase transitions and milestone boundaries.

---
*Last updated: 2026-04-12 after v1.0 milestone completion*
