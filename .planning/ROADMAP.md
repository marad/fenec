# Roadmap: Fenec

## Milestones

- ✅ **v1.0 Fenec Platform Foundation** -- Phases 1-6 (shipped 2026-04-12)
- **v1.1 Multi-Provider Support** -- Phases 7-11 (in progress)

## Phases

<details>
<summary>v1.0 Fenec Platform Foundation (Phases 1-6) -- SHIPPED 2026-04-12</summary>

- [x] Phase 1: Foundation (3/3 plans) -- completed 2026-04-11
- [x] Phase 2: Conversation (3/3 plans) -- completed 2026-04-11
- [x] Phase 3: Tool Execution (2/2 plans) -- completed 2026-04-11
- [x] Phase 4: Lua Runtime (2/2 plans) -- completed 2026-04-11
- [x] Phase 5: Self-Extension (2/2 plans) -- completed 2026-04-11
- [x] Phase 6: File Tools (2/2 plans) -- completed 2026-04-12

See `.planning/milestones/v1.0-ROADMAP.md` for full phase details.

</details>

### v1.1 Multi-Provider Support (In Progress)

**Milestone Goal:** Enable Fenec to connect to any LLM provider (Ollama, LM Studio, OpenAI) through a config-driven provider abstraction with unified tool calling.

- [ ] **Phase 7: Canonical Types** - Replace Ollama-specific types with project-owned message/tool types across the codebase
- [ ] **Phase 8: Provider Abstraction** - Define Provider interface and validate it with Ollama adapter
- [ ] **Phase 9: Configuration** - Config-driven provider definitions with TOML file and zero-config default
- [ ] **Phase 10: OpenAI-Compatible Client** - OpenAI-protocol adapter for LM Studio, OpenAI, and compatible backends
- [ ] **Phase 11: Model Routing** - Unified model selection with `--model provider/model` syntax and model discovery

## Phase Details

### Phase 7: Canonical Types
**Goal**: Fenec owns its own message and tool types, decoupled from any single provider's API types
**Depends on**: Phase 6 (v1.0 complete)
**Requirements**: PROV-03
**Success Criteria** (what must be TRUE):
  1. User can start Fenec and chat with Ollama exactly as before -- no behavioral difference from v1.0
  2. No import of `github.com/ollama/ollama/api` types exists outside the Ollama adapter package
  3. All existing tests pass with the new canonical types
**Plans:** 2 plans

Plans:
- [x] 07-01-PLAN.md -- Create internal/model package with canonical types and JSON round-trip tests
- [x] 07-02-PLAN.md -- Migrate all packages to canonical types with Ollama conversion layer

### Phase 8: Provider Abstraction
**Goal**: A Provider interface exists and the Ollama backend works through it, proving the abstraction supports streaming chat and tool calling
**Depends on**: Phase 7
**Requirements**: PROV-01, PROV-02
**Success Criteria** (what must be TRUE):
  1. User can chat with Fenec through the provider abstraction with identical behavior to v1.0 (streaming, multi-turn, tools)
  2. User can use all existing tools (shell, file, Lua) and see correct tool call/result round-trips through the abstraction
  3. A second provider implementation can be added by implementing the Provider interface without modifying existing code
**Plans:** 1 plan

Plans:
- [x] 08-01-PLAN.md -- Create Provider interface, Ollama adapter, and wire REPL + main.go

### Phase 9: Configuration
**Goal**: Users can define and manage providers through a TOML config file, with sensible defaults that preserve the zero-config Ollama experience
**Depends on**: Phase 8
**Requirements**: CONF-01, CONF-02, CONF-03, CONF-04
**Success Criteria** (what must be TRUE):
  1. User can define providers in `~/.config/fenec/config.toml` with name, type, URL, and optional API key
  2. User can use `$ENV_VAR` syntax for API keys in config and have them resolved at load time
  3. User can run Fenec with no config file and get the default Ollama provider at localhost:11434 automatically
  4. User can edit config.toml while Fenec is running and have changes take effect without restarting
**Plans:** 2 plans

Plans:
- [x] 09-01-PLAN.md -- TOML config loading, env var resolution, provider registry, and config-driven main.go
- [x] 09-02-PLAN.md -- Config file watcher with fsnotify for hot-reload without restart

### Phase 10: OpenAI-Compatible Client
**Goal**: Users can chat and use tools with any OpenAI-compatible provider (LM Studio, OpenAI cloud, etc.) alongside the existing Ollama provider
**Depends on**: Phase 9
**Requirements**: OAIC-01, OAIC-02, OAIC-03, OAIC-04
**Success Criteria** (what must be TRUE):
  1. User can chat with a model served by LM Studio using the OpenAI-compatible protocol
  2. User can chat with OpenAI cloud models (GPT-4o etc.) by configuring an API key
  3. User can use tool calling with OpenAI-compatible providers, with automatic non-streaming fallback when tools are present
  4. User can switch providers mid-session (e.g., `/provider lmstudio`) and continue the same conversation
**Plans:** 2 plans

Plans:
- [ ] 10-01-PLAN.md -- Add openai-go SDK, create OpenAI adapter with streaming/non-streaming dispatch, wire factory
- [ ] 10-02-PLAN.md -- Comprehensive adapter test suite and factory test extension

### Phase 11: Model Routing
**Goal**: Users have a unified model selection experience across all providers, with discovery and CLI ergonomics
**Depends on**: Phase 10
**Requirements**: ROUT-01, ROUT-02, ROUT-03, ROUT-04
**Success Criteria** (what must be TRUE):
  1. User can run `fenec --model ollama/gemma4` to target a specific provider using `/` as delimiter
  2. User can run `fenec --model gemma4` (no prefix) and have it routed to the default provider
  3. User can type `/model` in the REPL and see available models grouped by provider
  4. Models are discovered automatically from each provider's API (Ollama list, OpenAI models endpoint)
**Plans**: TBD

Plans:
- [ ] 11-01: TBD
- [ ] 11-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 7 -> 8 -> 9 -> 10 -> 11

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-11 |
| 2. Conversation | v1.0 | 3/3 | Complete | 2026-04-11 |
| 3. Tool Execution | v1.0 | 2/2 | Complete | 2026-04-11 |
| 4. Lua Runtime | v1.0 | 2/2 | Complete | 2026-04-11 |
| 5. Self-Extension | v1.0 | 2/2 | Complete | 2026-04-11 |
| 6. File Tools | v1.0 | 2/2 | Complete | 2026-04-12 |
| 7. Canonical Types | v1.1 | 0/2 | Planning complete | - |
| 8. Provider Abstraction | v1.1 | 0/1 | Planning complete | - |
| 9. Configuration | v1.1 | 0/2 | Planning complete | - |
| 10. OpenAI-Compatible Client | v1.1 | 0/2 | Planning complete | - |
| 11. Model Routing | v1.1 | 0/0 | Not started | - |
