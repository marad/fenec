# Roadmap: Fenec

## Milestones

- ✅ **v1.0 Fenec Platform Foundation** — Phases 1-6 (shipped 2026-04-12)
- ✅ **v1.1 Multi-Provider Support** — Phases 7-11 (shipped 2026-04-14)
- [ ] **v1.2 GitHub Models Provider** — Phases 12-13

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

<details>
<summary>✅ v1.1 Multi-Provider Support (Phases 7-11) — SHIPPED 2026-04-14</summary>

- [x] Phase 7: Canonical Types (2/2 plans) — completed 2026-04-12
- [x] Phase 8: Provider Abstraction (1/1 plan) — completed 2026-04-12
- [x] Phase 9: Configuration (2/2 plans) — completed 2026-04-13
- [x] Phase 10: OpenAI-Compatible Client (2/2 plans) — completed 2026-04-13
- [x] Phase 11: Model Routing (2/2 plans) — completed 2026-04-14

See `.planning/milestones/v1.1-ROADMAP.md` for full phase details.

</details>

## v1.2 GitHub Models Provider (Phases 12-13)

- [ ] **Phase 12: Copilot Provider** — Token resolution, openai-go wrapper, config integration
- [ ] **Phase 13: Model Catalog** — Custom HTTP listing, context length from catalog, Ping, `/model` grouping

### Phase 12: Copilot Provider
**Goal**: Users can chat with GitHub Models using `type = "copilot"` in config, with automatic auth from `gh` CLI
**Depends on**: Phase 11 (model routing — provides provider/model syntax and /model REPL)
**Requirements**: COPILOT-01, COPILOT-02, COPILOT-03, COPILOT-04, COPILOT-05, COPILOT-09
**Success Criteria** (what must be TRUE):
  1. User adds `[providers.copilot] type = "copilot"` to config (no url or api_key) and the provider initializes
  2. Auth token resolves automatically via GH_TOKEN → GITHUB_TOKEN → `gh auth token` priority chain
  3. Missing or unauthenticated `gh` CLI produces an actionable error message with specific remediation steps
  4. Streaming chat and tool calling work through the copilot provider identically to the openai provider
**Plans:** 2 plans
Plans:
  - [ ] 12-01-PLAN.md — Provider skeleton + token resolution (token.go, copilot.go, config integration)
  - [ ] 12-02-PLAN.md — Tests + error handling (token_test.go, copilot_test.go, full verification)

### Phase 13: Model Catalog
**Goal**: Model listing, context length, and Ping use the GitHub Models catalog instead of the incompatible SDK endpoint
**Depends on**: Phase 12 (copilot provider skeleton with stub ListModels/GetContextLength/Ping)
**Requirements**: COPILOT-06, COPILOT-07, COPILOT-08, COPILOT-10
**Success Criteria** (what must be TRUE):
  1. `/model` command lists all GitHub Models catalog entries grouped under `copilot/*`
  2. `GetContextLength()` returns real `max_input_tokens` values from the catalog for any listed model
  3. `Ping()` validates connectivity and auth via a catalog fetch — no chat request needed
**Plans**:
  - 13-01: Catalog HTTP client + ListModels + GetContextLength — ghModel struct, fetchCatalog with lazy caching (double-checked locking), direct HTTP GET to `https://models.github.ai/v1/models`, replace ListModels/GetContextLength stubs with catalog-backed implementations, mock HTTP server tests
  - 13-02: Ping via catalog + `/model` grouping — Ping() delegates to fetchCatalog (validates auth + connectivity in one call), verify `/model` REPL lists copilot models under `copilot/*` namespace, tests for Ping auth-failure/network-error/success paths

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-11 |
| 2. Conversation | v1.0 | 3/3 | Complete | 2026-04-11 |
| 3. Tool Execution | v1.0 | 2/2 | Complete | 2026-04-11 |
| 4. Lua Runtime | v1.0 | 2/2 | Complete | 2026-04-11 |
| 5. Self-Extension | v1.0 | 2/2 | Complete | 2026-04-11 |
| 6. File Tools | v1.0 | 2/2 | Complete | 2026-04-12 |
| 7. Canonical Types | v1.1 | 2/2 | Complete | 2026-04-12 |
| 8. Provider Abstraction | v1.1 | 1/1 | Complete | 2026-04-12 |
| 9. Configuration | v1.1 | 2/2 | Complete | 2026-04-13 |
| 10. OpenAI-Compatible Client | v1.1 | 2/2 | Complete | 2026-04-13 |
| 11. Model Routing | v1.1 | 2/2 | Complete | 2026-04-14 |
| 12. Copilot Provider | v1.2 | 0/2 | Planned | - |
| 13. Model Catalog | v1.2 | 0/2 | Planned | - |
