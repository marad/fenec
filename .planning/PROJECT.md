# Fenec

## What This Is

A personal AI assistant platform built in Go with Lua extensibility. Provides a CLI chat interface that connects to local Ollama models (like Gemma 4). The agent can use tools via structured function calling and extend itself by writing Lua scripts that persist as new tools — becoming more capable over time.

## Core Value

An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## Current Milestone: v1.1 Multi-Provider Support

**Goal:** Enable Fenec to connect to any LLM provider (Ollama, LM Studio, OpenAI) through a config-driven provider abstraction with unified tool calling.

**Target features:**
- Provider abstraction layer with named providers and type-based protocol selection
- OpenAI-compatible API client for LM Studio, OpenAI, and other compatible backends
- Config-driven provider definitions (type, URL, API key, model overrides)
- `--model provider/model` syntax for unified model selection with provider routing
- Model discovery from providers with optional config overrides
- Tool calling support across all provider types

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
- ✓ Existing Ollama workflow works with zero config changes after type decoupling — v1.1 Phase 7
- ✓ Provider abstraction with Ollama adapter — v1.1 Phase 8
- ✓ Config-driven provider definitions with TOML + hot-reload — v1.1 Phase 9
- ✓ OpenAI-compatible API client with tool calling (LM Studio, OpenAI cloud) — v1.1 Phase 10

### Active
- [ ] Unified `--model provider/model` selection with provider routing
- [ ] Model discovery from providers

### Out of Scope

- Email reading/tagging — future milestone
- Note search/organization — future milestone
- TUI with panels/history/status — start with basic REPL, enhance later
- Cloud/remote model providers — ~~focus on local Ollama~~ now supported via OpenAI-compatible provider type
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
*Last updated: 2026-04-13 after Phase 10 completion*
