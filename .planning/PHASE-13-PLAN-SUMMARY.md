# Phase 13: Model Catalog — Plan Summary

**Goal**: Model listing, context length, and Ping use the GitHub Models catalog instead of the incompatible SDK endpoint
**Requirements**: COPILOT-06, COPILOT-07, COPILOT-08, COPILOT-10
**Depends on**: Phase 12 (copilot provider skeleton exists with stub methods)

## What This Phase Builds

Replaces Phase 12's stub implementations of `ListModels()`, `GetContextLength()`, and `Ping()` with catalog-backed versions. The GitHub Models catalog at `https://models.github.ai/v1/models` uses a non-standard schema incompatible with the openai-go SDK's `ListAutoPaging`, so this phase implements a direct `net/http` client with lazy caching.

## Plans

### Plan 13-01: Catalog HTTP Client + ListModels + GetContextLength

**Creates:**
- `internal/provider/copilot/catalog.go`:
  - `ghModel` struct matching catalog JSON: `id`, `name`, `capabilities` ([]string), `limits.max_input_tokens`, `limits.max_output_tokens`, `rate_limit_tier`
  - `modelsResponse` struct: `{"data": [...]}` wrapper
  - `fetchCatalog(ctx)` method on Provider:
    - Lazy-loaded with double-checked locking (sync.RWMutex)
    - `GET https://models.github.ai/v1/models` with `Authorization: Bearer <token>`
    - Cached in `p.catalog` for session lifetime (model list doesn't change during a session)
    - Returns `[]ghModel`

**Modifies:**
- `internal/provider/copilot/copilot.go`:
  - `ListModels()` → calls fetchCatalog, returns `[]string` of model IDs (e.g., `openai/gpt-4o-mini`)
  - `GetContextLength(model)` → calls fetchCatalog, looks up model by ID, returns `limits.max_input_tokens`; returns `0, nil` for unknown models (consistent with existing openai provider behavior)

**Creates tests:**
- `internal/provider/copilot/catalog_test.go`:
  - Mock HTTP server returning catalog JSON
  - ListModels returns all model IDs from catalog
  - GetContextLength returns correct max_input_tokens for known model
  - GetContextLength returns 0 for unknown model
  - Catalog is fetched once and cached (verify single HTTP call on repeated ListModels)
  - HTTP error returns meaningful error message
  - 401 response produces auth-specific error

**Key technical decisions:**
- Response format is `{"data": [...]}` (NOT a bare JSON array — verified live)
- Model IDs use publisher-prefixed format: `openai/gpt-4o-mini`, `meta/llama-3.3-70b-instruct`
- Include Bearer token in catalog request (endpoint works without auth but including it ensures consistent behavior)
- Use `http.NewRequestWithContext` for cancellation support

### Plan 13-02: Ping via Catalog + `/model` REPL Grouping

**Modifies:**
- `internal/provider/copilot/copilot.go`:
  - `Ping()` → calls fetchCatalog; if it succeeds and returns ≥1 model, provider is healthy
  - On 401: return auth-specific error ("GitHub token is invalid or expired")
  - On network/5xx error: return connectivity error

**Verifies (may need no code changes):**
- `/model` REPL command already groups models by provider name via `Provider.Name()` + `ListModels()`
- Since `Name()` returns `"copilot"` and `ListModels()` returns `["openai/gpt-4o-mini", ...]`, the REPL should display models as `copilot/openai/gpt-4o-mini` etc.
- If the existing `/model` grouping code needs adjustment, make it here

**Creates tests:**
- Ping tests:
  - Successful catalog fetch → Ping returns nil
  - 401 from catalog → Ping returns auth error
  - Network error → Ping returns connectivity error
  - Empty catalog (0 models) → Ping returns error ("no models available")
- Integration-level:
  - Provider init → Ping → ListModels → GetContextLength chain works end-to-end with mock HTTP

## Success Criteria

1. `/model` command lists all GitHub Models catalog entries grouped under `copilot/*`
2. `GetContextLength()` returns real `max_input_tokens` values from the catalog (e.g., 131072 for gpt-4o-mini)
3. `Ping()` validates connectivity and auth via a catalog fetch — no chat request needed

## Technical Context from Research

- Catalog URL: `GET https://models.github.ai/v1/models`
- Response: `{"data": [{id, name, capabilities, limits, rate_limit_tier}, ...]}`
- 43 models currently listed (mix of OpenAI, Meta, DeepSeek, Mistral, etc.)
- `limits.max_input_tokens` field gives real context lengths (e.g., gpt-4o-mini=131072, gpt-4.1=1048576)
- The openai-go SDK's `ListAutoPaging` fails on this endpoint (missing `created`, `object`, `owned_by` fields)
- Not all models support tool calling — catalog `capabilities` array indicates support
- `rate_limit_tier` (low/high/custom) available for future display in `/model` output
