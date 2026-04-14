# Fenec

## What This Is

A personal AI assistant platform built in Go with Lua extensibility. Provides a CLI chat interface connecting to multiple LLM providers (Ollama, LM Studio, OpenAI-compatible, GitHub Models) through a config-driven provider abstraction with unified tool calling and model routing.

## Core Value

An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## Current Milestone: _Planning next milestone_

**Status:** v1.2 shipped — GitHub Models Provider fully implemented. Run `/gsd-new-milestone` to define v1.3.

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
- ✓ Existing Ollama workflow works with zero config changes after type decoupling — v1.1
- ✓ Provider abstraction with Ollama adapter — v1.1
- ✓ Config-driven provider definitions with TOML + hot-reload — v1.1
- ✓ OpenAI-compatible API client with tool calling (LM Studio, OpenAI cloud) — v1.1
- ✓ `--model provider/model` unified model selection with provider routing — v1.1
- ✓ `/model` REPL command with provider-grouped model listing — v1.1
- ✓ `type = "copilot"` provider with zero-config auth via `gh auth token` — v1.2
- ✓ GitHub Models catalog listing (40+ models) with real context lengths — v1.2
- ✓ Ping validates auth/connectivity via catalog fetch, no chat round-trip — v1.2

### Active

_(Define with `/gsd-new-milestone`)_

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
- As of v1.2: ~11,300 LOC Go; 3 provider adapters (ollama, openai-compat, copilot); 5-method Provider interface
- GitHub Models copilot provider connects to `https://models.github.ai/inference` via the openai-go v3 SDK, authenticated through the `gh` CLI session

## Constraints

- **Language**: Go — performance, single binary deployment, strong concurrency
- **Scripting**: Lua 5.1 (gopher-lua) — embedded scripting for tool extensibility
- **Models**: Ollama local models + any OpenAI-compatible API — flexible local or cloud
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
| `internal/model` canonical types | Project-owned types decouple all packages from provider SDKs | ✓ Good — single import boundary at adapter |
| `provider.Provider` interface | 5-method interface enables adding providers without changing core | ✓ Good — OpenAI adapter proved the pattern |
| openai-go v3 SDK | Official OpenAI Go client for OpenAI-compatible adapters | ✓ Good — full streaming + tool call support |
| `provider/model` syntax via `/` delimiter | Natural, URL-inspired routing syntax for CLI and REPL | ✓ Good — `/model` listing + `--model` flag consistent |
| Hot-reload with fsnotify + debounce | Live config changes without restart | ✓ Good — 100ms debounce handles editor atomic saves |

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

| Copilot provider wraps openai.Provider via delegation | No duplicated streaming/tool-calling logic; delegation pattern proven by Phase 12 | ✓ Good — clean adapter boundary |
| Token resolution uses injectable functions (resolveTokenWith) | Testability for os/exec subprocess calls without mocking entire exec package | ✓ Good — 8 unit test paths covered |
| ExitError mocks via real subprocess (sh -c exit N) | exec.ExitError cannot be constructed directly in Go | ✓ Good — pragmatic test approach |
| fetchCatalogFrom(ctx, url) for testability | Separate URL-parameterized method avoids test-only struct fields | ✓ Good — clean test isolation |
| Double-checked locking for catalog cache | Thread-safe lazy loading — fast read path, single HTTP call per session | ✓ Good — prevents duplicate catalog fetches |
| net/http removed from copilot.go after Phase 13 | All HTTP lives in catalog.go — copilot.go is pure provider facade | ✓ Good — clean separation of concerns |

---
*Last updated: 2026-04-14 after v1.2 milestone*
