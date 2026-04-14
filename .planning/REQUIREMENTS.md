# Requirements: Fenec v1.2 — GitHub Models Provider

**Defined:** 2026-04-14
**Core Value:** An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## v1.2 Requirements

### Copilot Provider

- [x] **COPILOT-01**: User can add a `copilot` provider with only `type = "copilot"` in TOML — no url or api_key fields required
- [x] **COPILOT-02**: Provider resolves auth token automatically: `GH_TOKEN` env var → `GITHUB_TOKEN` env var → `gh auth token` subprocess
- [x] **COPILOT-03**: Provider returns a clear, actionable error if `gh` CLI is not installed or the user is not authenticated
- [x] **COPILOT-04**: Chat completions and streaming work via `https://models.github.ai/inference` using the existing openai-go/v3 SDK
- [x] **COPILOT-05**: Tool calling works identically to the existing `openai` provider (same format, no changes to tool system)
- [ ] **COPILOT-06**: `ListModels()` returns the full GitHub Models catalog via a direct HTTP call to `https://models.github.ai/v1/models`
- [ ] **COPILOT-07**: `GetContextLength()` returns real values sourced from the catalog `limits.max_input_tokens` field
- [ ] **COPILOT-08**: `Ping()` validates connectivity and auth via a catalog fetch (no chat request needed)
- [x] **COPILOT-09**: Default model for the `copilot` provider is `gpt-4o-mini` (Copilot Free compatible, supports tool calling)
- [ ] **COPILOT-10**: `/model` REPL command lists models grouped under `copilot/*` alongside other providers

## Out of Scope

| Feature | Reason |
|---------|--------|
| GitHub Enterprise (GHES) hostname support | Adds complexity; personal tool targets github.com only |
| Token refresh/rotation during session | GitHub OAuth tokens are long-lived; not needed for CLI sessions |
| Reasoning model `max_completion_tokens` special-casing | Edge case; o-series and gpt-5 models can be added in follow-on if needed |
| Hardcoded model list fallback | Catalog endpoint is stable; dynamic listing preferred |
| Rate limit retry with backoff | 50 RPD free tier — surfacing the error is more useful than silently retrying |
| Azure Content Safety filter bypass | Not possible; document behavior in error messages instead |
| GitHub Copilot Chat API (`api.githubcopilot.com`) | Different product, complex token exchange; out of scope |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| COPILOT-01 | Phase 12 | Complete |
| COPILOT-02 | Phase 12 | Complete |
| COPILOT-03 | Phase 12 | Complete |
| COPILOT-04 | Phase 12 | Complete |
| COPILOT-05 | Phase 12 | Complete |
| COPILOT-06 | Phase 13 | Pending |
| COPILOT-07 | Phase 13 | Pending |
| COPILOT-08 | Phase 13 | Pending |
| COPILOT-09 | Phase 12 | Complete |
| COPILOT-10 | Phase 13 | Pending |

**Coverage:**
- v1.2 requirements: 10 total
- Mapped to phases: 10
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-14 after initial definition*
