---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: GitHub Models Provider
status: planning
stopped_at: Completed 13-02-PLAN.md
last_updated: "2026-04-14T13:33:28.525Z"
last_activity: 2026-04-14 — Phase 12 executed and verified
progress:
  total_phases: 2
  completed_phases: 2
  total_plans: 4
  completed_plans: 4
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-12)

**Core value:** An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.
**Current focus:** Phase 12 — copilot provider (next)

## Current Position

Phase: 13 — Model Catalog (next)
Plan: —
Status: Phase 12 complete, Phase 13 ready to plan
Last activity: 2026-04-14 — Phase 12 executed and verified

## Performance Metrics

**Velocity (from v1.0):**

- Total plans completed: 14
- Average duration: ~4 min
- Total execution time: ~55 min

**By Phase (v1.1):**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend (v1.0):**

- Last 5 plans: 6min, 2min, 3min, 5min, 3min
- Trend: Stable

*Updated after each plan completion*
| Phase 07-canonical-types P01 | 2min | 1 tasks | 6 files |
| Phase 07-canonical-types P02 | 10min | 2 tasks | 29 files |
| Phase 08-provider-abstraction P01 | 5min | 3 tasks | 10 files |
| Phase 09 P01 | 4min | 2 tasks | 7 files |
| Phase 09-02 P02 | 2min | 2 tasks | 5 files |
| Phase 10 P01 | 4min | 2 tasks | 4 files |
| Phase 10-openai-compatible-client P02 | 5min | 2 tasks | 2 files |
| Phase 12 P01 | 1min | 2 tasks | 3 files |
| Phase 12 P02 | 2min | 3 tasks | 2 files |
| Phase 13-model-catalog P01 | 3min | 3 tasks | 3 files |
| Phase 13-model-catalog P02 | 3min | 4 tasks | 2 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- 228 Ollama type references across 29 files must be migrated to canonical types before provider abstraction
- Provider interface + Ollama adapter validates abstraction before adding OpenAI adapter
- OpenAI-compat streaming + tools is broken -- need non-streaming fallback when tools present
- Config uses BurntSushi/toml; zero-config default preserves existing behavior
- `--model provider/model` with `/` as delimiter (not `:`)
- `/model` REPL command groups models by provider
- [Phase 07-canonical-types]: Canonical model types use plain maps (map[string]ToolProperty, map[string]any) instead of Ollama ordered maps for simplicity
- [Phase 07-canonical-types]: Used mdl alias for internal/model in chat package to avoid parameter name shadowing
- [Phase 07-canonical-types]: Conversion functions placed in stream.go as the adapter boundary between canonical types and ollama/api
- [Phase 08-provider-abstraction]: Provider interface with 5 methods (Name, ListModels, Ping, StreamChat, GetContextLength) and ChatRequest type decoupled from Conversation
- [Phase 08-provider-abstraction]: Only internal/provider/ollama imports ollama/api -- all other packages use provider.Provider interface
- [Phase 09]: Used BurntSushi/toml v1.6.0 for TOML parsing per CLAUDE.md recommendation
- [Phase 09]: ProviderRegistry in internal/config/ with RWMutex; factory imports specific provider packages
- [Phase 09]: Watcher watches parent directory with 100ms debounce for editor atomic saves; watcher failure is non-fatal
- [Phase 10]: Non-streaming when tools present, streaming SSE when pure chat
- [Phase 10]: Dummy API key 'not-needed' for local providers to prevent SDK env var lookup
- [Phase 10]: GetContextLength returns 0 for OpenAI (API handles limits server-side)
- [Phase 10-openai-compatible-client]: Mock SSE decoder for ssestream.Stream testing; JSON unmarshal for SDK response construction
- [Phase 12]: Copilot provider wraps openai.Provider with delegation — no duplicated API logic
- [Phase 12]: Token resolution uses injectable functions (resolveTokenWith) for testability
- [Phase 12]: ExitError mocks use real subprocess (sh -c exit N) since exec.ExitError cannot be constructed directly
- [Phase 12]: TestNewWithoutTokenFailsWhenNoGh skips gracefully when gh CLI is installed and authenticated
- [Phase 13-model-catalog]: fetchCatalogFrom(ctx, url) pattern for testability; double-checked locking with sync.RWMutex for lazy catalog cache; GetContextLength returns 0 for unknown models
- [Phase 13-model-catalog]: Removed net/http import from copilot.go — all HTTP lives in catalog.go
- [Phase 13-model-catalog]: Ping tests use cache-seeding pattern (fetchCatalogFrom then Ping) for testability
- [Phase 13-model-catalog]: /model REPL grouping confirmed correct with copilot provider — no changes needed

### Pending Todos

None yet.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260411-tbx | Style tool call output gray and show results only in debug mode | 2026-04-11 | 21dad26 | [260411-tbx](./quick/260411-tbx-style-tool-call-output-gray-and-show-res/) |
| 260412-etg | Change pipe mode to read all stdin by default, add --line-by-line flag | 2026-04-12 | fc6fc68 | [260412-etg](./quick/260412-etg-change-pipe-mode-read-all-stdin-by-defau/) |
| 260412-f3x | Display last 3 lines of model thinking output in muted style | 2026-04-12 | 1408a78 | [260412-f3x](./quick/260412-f3x-display-last-3-lines-of-model-thinking-o/) |
| 260412-gan | Switch CLI flags to pflag with double-dash conventions, custom help, --version | 2026-04-12 | 0f40cc8 | [260412-gan](./quick/260412-gan-improve-cli-help-output-and-flag-handlin/) |
| 260412-lmh | Add --model / -m flag for selecting Ollama model | 2026-04-12 | c8c2c21 | [260412-lmh](./quick/260412-lmh-add-the-ability-to-configure-model-throu/) |

### Blockers/Concerns

- OpenAI-compatible streaming with tool calls is broken -- Phase 10 must implement non-streaming fallback
- Gemma 4 tool calling reliability has active compatibility issues with Ollama v0.20.0

## Session Continuity

Last activity: 2026-04-14
Stopped at: Completed 13-02-PLAN.md
Resume file: None
