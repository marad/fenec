---
phase: 13-model-catalog
plan: 01
subsystem: provider
tags: [copilot, github-models, catalog, http-client, caching]

requires:
  - phase: 12-copilot-provider
    provides: "copilot Provider struct wrapping openai.Provider with token resolution"
provides:
  - "catalog.go: HTTP client for GitHub Models catalog with lazy double-checked locking cache"
  - "ListModels() returns publisher-prefixed model IDs from live catalog"
  - "GetContextLength() returns real max_input_tokens from catalog"
  - "fetchCatalogFrom() test helper for mock HTTP testing"
affects: [13-02-ping-model-grouping, copilot-provider]

tech-stack:
  added: []
  patterns: [double-checked-locking-cache, fetchCatalogFrom-test-injection]

key-files:
  created:
    - internal/provider/copilot/catalog.go
    - internal/provider/copilot/catalog_test.go
  modified:
    - internal/provider/copilot/copilot.go

key-decisions:
  - "Used fetchCatalogFrom(ctx, url) pattern for testability instead of modifying Provider struct with catalogEndpoint field"
  - "Double-checked locking with sync.RWMutex for thread-safe lazy catalog caching"
  - "GetContextLength returns 0, nil for unknown models (consistent with openai provider)"

patterns-established:
  - "fetchCatalogFrom pattern: separate URL-parameterized method for mock HTTP testing"
  - "httptest.NewServer-based catalog tests seeding cache before calling public methods"

metrics:
  duration: "3min"
  completed: "2026-04-14"
  tasks_completed: 3
  tasks_total: 3
---

# Phase 13 Plan 01: Catalog HTTP Client + ListModels + GetContextLength Summary

**One-liner:** Direct net/http catalog client with double-checked locking cache; ListModels returns publisher-prefixed IDs, GetContextLength returns real max_input_tokens from GitHub Models API.

## What Was Built

Replaced Phase 12's stub `ListModels()` and `GetContextLength()` with catalog-backed implementations. The GitHub Models catalog at `https://models.github.ai/v1/models` uses a non-standard `{"data": [...]}` response schema incompatible with the openai-go SDK, so a direct `net/http` client was implemented with session-lifetime lazy caching.

### Files Created

- **`internal/provider/copilot/catalog.go`** — HTTP client for the GitHub Models catalog
  - `ghModel` struct matching catalog JSON: id, name, capabilities, limits (max_input_tokens, max_output_tokens), rate_limit_tier
  - `modelsResponse` wrapper for `{"data": [...]}`
  - `fetchCatalog(ctx)` — public API using default catalogURL
  - `fetchCatalogFrom(ctx, url)` — URL-parameterized for testing
  - Double-checked locking with `sync.RWMutex` (read lock fast path, write lock slow path)
  - Bearer token auth, 401 detection with auth-specific error message

- **`internal/provider/copilot/catalog_test.go`** — 7 test functions using `httptest.NewServer`
  - `TestFetchCatalogReturnsModels`: 2-model JSON response parsing
  - `TestListModelsReturnsIDs`: publisher-prefixed ID extraction
  - `TestGetContextLengthKnownModel`: max_input_tokens = 131072
  - `TestGetContextLengthUnknownModel`: returns 0, nil
  - `TestCatalogIsCached`: atomic counter verifies single HTTP call
  - `TestFetchCatalog401ReturnsAuthError`: auth error message
  - `TestFetchCatalogNetworkError`: closed server error propagation

### Files Modified

- **`internal/provider/copilot/copilot.go`**
  - Added `sync` import
  - Added `mu sync.RWMutex` and `catalog []ghModel` fields to Provider struct
  - Replaced stub `ListModels` → calls `fetchCatalog`, returns `[]string` of model IDs
  - Replaced stub `GetContextLength` → calls `fetchCatalog`, looks up by model ID, returns `limits.max_input_tokens`

## Decisions Made

- **fetchCatalogFrom pattern over struct field:** Rather than adding a `catalogEndpoint` field to Provider, split `fetchCatalog` into `fetchCatalog` (uses const) and `fetchCatalogFrom` (accepts URL). Tests seed the cache via `fetchCatalogFrom(ctx, srv.URL)`, then public methods like `ListModels` use the cached result. Clean separation without test-only struct fields.
- **Double-checked locking:** RWMutex with read-lock fast path (cached), upgrade to write-lock only on first fetch. Prevents concurrent duplicate HTTP calls.
- **Zero for unknown models:** `GetContextLength` returns `(0, nil)` for unknown model IDs, consistent with the openai provider's behavior.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Pre-existing `TestLoadSystemPromptFromFile` failure in `internal/config` (unrelated to this plan's changes — system prompt loading behavior changed without updating the test expectation). Logged to deferred items.

## Verification Results

- `go build ./...` ✅
- `go test ./internal/provider/copilot/...` ✅ (20 tests pass: 7 catalog + 13 existing)
- `go vet ./...` ✅

## Next Phase Readiness

- Catalog infrastructure ready for Plan 13-02 (Ping via catalog + `/model` REPL grouping)
- `fetchCatalog` is reusable by `Ping()` — call it and check for ≥1 model

---
*Phase: 13-model-catalog*
*Completed: 2026-04-14*
