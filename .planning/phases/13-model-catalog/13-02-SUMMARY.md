---
phase: 13-model-catalog
plan: 02
subsystem: provider
tags: [copilot, github-models, ping, catalog, repl]

requires:
  - phase: 13-model-catalog
    plan: 01
    provides: "catalog.go with fetchCatalog/fetchCatalogFrom, double-checked locking cache"
provides:
  - "Ping() backed by catalog fetch — validates connectivity and authentication"
  - "4 Ping test functions covering success, auth error, network error, empty catalog"
  - "Verified /model REPL grouping works correctly with copilot provider"
affects: [copilot-provider, repl]

tech-stack:
  added: []
  patterns: [catalog-backed-ping]

key-files:
  created: []
  modified:
    - internal/provider/copilot/copilot.go
    - internal/provider/copilot/catalog_test.go

key-decisions:
  - "Removed net/http import from copilot.go — all HTTP lives in catalog.go"
  - "Ping tests use cache-seeding pattern from 13-01 (fetchCatalogFrom then Ping)"
  - "/model REPL grouping confirmed correct — no code changes needed"

metrics:
  duration: "3min"
  completed: "2026-04-14"
  tasks_completed: 4
  tasks_total: 4
---

# Phase 13 Plan 02: Catalog-Backed Ping + /model REPL Verification Summary

**One-liner:** Replaced direct HTTP Ping() with catalog-backed delegation; added 4 Ping tests; verified /model REPL groups copilot models correctly under provider heading.

## What Was Built

### Task 1: Catalog-Backed Ping

Replaced the stub `Ping()` in `copilot.go` which made a direct `net/http` GET to catalogURL with a clean 3-line implementation delegating to `fetchCatalog(ctx)`:

- On fetch error → wraps with `"cannot connect to GitHub Models"` context
- On empty catalog (len == 0) → returns `"GitHub Models catalog returned no models"`
- On success → returns nil

Removed the `net/http` import from `copilot.go` entirely — all HTTP now lives in `catalog.go`. The `catalogURL` constant remains in `copilot.go` (same package, used by `catalog.go`).

### Task 2: Ping Tests

Added 4 test functions to `catalog_test.go` following the existing `httptest.NewServer` + cache-seeding pattern:

| Test | Scenario | Assertion |
|------|----------|-----------|
| `TestPingSuccess` | Mock returns valid 2-model catalog, cache seeded | `Ping()` returns nil |
| `TestPingAuthError` | Mock returns 401 | Error contains "invalid or expired" |
| `TestPingNetworkError` | Server closed before call | Error is non-nil |
| `TestPingEmptyCatalog` | Mock returns `{"data":[]}`, cache seeded | Error contains "no models" |

Total test suite: 24 tests (23 pass, 1 skip for `TestNewWithoutTokenFailsWhenNoGh` when gh CLI is authenticated).

### Task 3: /model REPL Grouping Verification

Reviewed `listModels()` in `repl.go`. The implementation:

1. Calls `p.ListModels(ctx)` for each registered provider
2. Groups output under `render.FormatProviderHeader(res.name)` → `## copilot`
3. Lists each model via `render.FormatModelEntry(m, active)` → `     openai/gpt-4o-mini`

**No double-slash issue.** Provider name appears only in the header. Model IDs from `ListModels()` (e.g., `openai/gpt-4o-mini`) are displayed as-is. The `/model copilot/openai/gpt-4o-mini` switch syntax works correctly — `SplitN(target, "/", 2)` splits into provider=`copilot` and model=`openai/gpt-4o-mini`.

### Task 4: Full Verification

- `go build ./...` ✅
- `go test ./internal/provider/copilot/...` ✅ (24 tests)
- `go vet ./...` ✅

## Decisions Made

- **net/http removed from copilot.go:** After Ping delegation, copilot.go has zero direct HTTP usage. Clean separation: copilot.go is the provider facade, catalog.go handles all HTTP.
- **Cache-seeding test pattern:** Ping tests use `fetchCatalogFrom(srv.URL)` to seed the cache, then call `Ping()` which reads cached data. Auth/network error tests use `fetchCatalogFrom` directly since errors prevent caching.
- **/model REPL: no changes needed:** Grouping already works correctly with the copilot provider's `Name()="copilot"` and `ListModels()` returning publisher-prefixed IDs.

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

- `go build ./...` ✅
- `go test ./internal/provider/copilot/... -v` ✅ (11 catalog tests + 13 existing = 24 total)
- `go vet ./...` ✅
- `grep 'fetchCatalog' copilot.go` ✅ (3 occurrences: Ping, ListModels, GetContextLength)

## Self-Check: PASSED
