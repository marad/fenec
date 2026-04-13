---
phase: 12-multi-provider-integration-polish
plan: 02
subsystem: config
tags: [provider-registry, default-model, hot-reload, repl]

# Dependency graph
requires:
  - phase: 09-config-driven-providers
    provides: "ProviderRegistry, TOML config, hot-reload watcher"
  - phase: 11-model-routing
    provides: "--model provider/model syntax, /model REPL command"
provides:
  - "Per-provider default_model storage and retrieval in ProviderRegistry"
  - "Registry-based provider resolution in REPL (hot-reload takes effect immediately)"
  - "DefaultModelFor fallback in --model and /model resolution"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Registry-based provider resolution via currentProvider() instead of cached field"
    - "RegisterWithDefault for combined provider + default model registration"

key-files:
  created: []
  modified:
    - internal/config/registry.go
    - internal/config/registry_test.go
    - main.go
    - internal/repl/repl.go

key-decisions:
  - "currentProvider() falls back to cached r.provider if registry lookup fails"
  - "Empty default model strings are not stored in the registry map"
  - "Update() with nil defaultModels resets to empty map rather than keeping old values"

patterns-established:
  - "Registry resolution pattern: REPL resolves provider from registry on each message via currentProvider()"

requirements-completed: [CONF-01, CONF-04, ROUT-01]

# Metrics
duration: 3min
completed: 2026-04-13
---

# Phase 12 Plan 02: Per-Provider Default Model and Registry-Based REPL Resolution Summary

**Per-provider default_model wired from config through registry to --model and /model resolution; REPL resolves provider from registry on each message so hot-reload changes take effect immediately**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-13T06:33:41Z
- **Completed:** 2026-04-13T06:36:58Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- ProviderRegistry now stores and retrieves per-provider default models via RegisterWithDefault/DefaultModelFor
- --model provider/ (no model part) and no --model flag both consult per-provider default model before top-level default
- REPL resolves provider from registry on every StreamChat/ListModels/GetContextLength call, so hot-reload URL/API-key changes take effect without restart or /model
- /model provider/ in REPL uses per-provider default model with clear error when none configured

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend ProviderRegistry with default model storage** - `f0bc220` (feat - TDD)
2. **Task 2: Wire default_model into main.go and convert REPL to registry-based provider resolution** - `c7ab54b` (feat)

## Files Created/Modified
- `internal/config/registry.go` - Added defaultModels map, RegisterWithDefault, DefaultModelFor, updated Update signature
- `internal/config/registry_test.go` - Added 3 new tests, updated existing Update and concurrent tests for new signatures
- `main.go` - RegisterWithDefault in registration, newDefaultModels in hot-reload, DefaultModelFor in --model resolution
- `internal/repl/repl.go` - currentProvider() method, replaced all r.provider calls, /model provider/ default model support

## Decisions Made
- currentProvider() falls back to the cached r.provider field if registry lookup fails, ensuring REPL continues to function even if a provider is removed during hot-reload
- Empty default model strings are not stored in the defaultModels map to keep lookups clean
- Update() with nil defaultModels resets to an empty map rather than preserving old values, matching the atomic-replace semantics

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All v1.1 milestone audit gaps (CONF-01, CONF-04, ROUT-01) are now closed
- Per-provider default_model is fully functional end-to-end
- Hot-reload changes take effect immediately without REPL restart

## Self-Check: PASSED

- All 4 modified files exist
- Both task commits (f0bc220, c7ab54b) found in git log
- No stubs or placeholders detected

---
*Phase: 12-multi-provider-integration-polish*
*Completed: 2026-04-13*
