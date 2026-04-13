---
phase: 09-configuration
plan: 02
subsystem: config
tags: [fsnotify, file-watcher, hot-reload, debounce]

# Dependency graph
requires:
  - phase: 09-configuration
    provides: Config struct, LoadConfig, ProviderRegistry with Update method
provides:
  - ConfigWatcher with fsnotify directory watch, debounce, and reload callback
  - Hot-reload wiring in main.go that rebuilds providers on config change
affects: [10-openai-provider, 11-model-routing, configuration]

# Tech tracking
tech-stack:
  added: [github.com/fsnotify/fsnotify v1.9.0]
  patterns: [directory watch with file filtering, debounced reload callback, non-fatal watcher startup]

key-files:
  created:
    - internal/config/watcher.go
    - internal/config/watcher_test.go
  modified:
    - main.go
    - go.mod
    - go.sum

key-decisions:
  - "Watcher watches parent directory, not the file itself, to handle editor atomic saves"
  - "100ms debounce via time.AfterFunc to collapse rapid filesystem events"
  - "Config watcher failure is non-fatal: hot-reload disabled but app continues"

patterns-established:
  - "Directory watch + filename filter pattern for config file monitoring"
  - "Debounce-and-rebuild: onChange callback re-parses config, rebuilds all providers, atomically swaps registry"

requirements-completed: [CONF-04]

# Metrics
duration: 2min
completed: 2026-04-13
---

# Phase 9 Plan 2: Config Hot-Reload Summary

**fsnotify config file watcher with 100ms debounce, directory-level watch for editor atomic saves, and main.go reload callback that rebuilds providers on config change**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-13T04:59:25Z
- **Completed:** 2026-04-13T05:01:44Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- ConfigWatcher watches config directory via fsnotify, filters events by config filename, reacts to Write and Create events
- Debounce collapses rapid filesystem events (editor saves generate 3-5 events) into a single onChange callback
- main.go reload callback re-parses TOML, rebuilds all providers from new config, atomically updates registry
- Invalid config on reload keeps old config active and logs error (graceful degradation)
- 4 new watcher tests covering onChange, debounce, file filtering, and clean stop

## Task Commits

Each task was committed atomically:

1. **Task 1: Config file watcher with debounce (TDD RED)** - `c1dfab7` (test)
2. **Task 1: Config file watcher with debounce (TDD GREEN)** - `9629912` (feat)
3. **Task 2: Wire config watcher into main.go for hot-reload** - `b13900e` (feat)

_Task 1 used TDD: tests written first (RED), then implementation (GREEN)._

## Files Created/Modified
- `internal/config/watcher.go` - ConfigWatcher struct with fsnotify, directory watch, debounce via time.AfterFunc, Stop cleanup
- `internal/config/watcher_test.go` - 4 tests: onChange fires, debounce collapses, other files ignored, clean stop
- `main.go` - Config watcher started after provider registry build, reload callback rebuilds providers and updates registry
- `go.mod` - Added github.com/fsnotify/fsnotify v1.9.0
- `go.sum` - Updated checksums

## Decisions Made
- Watched parent directory instead of config file directly to handle editor atomic saves (vim, VSCode write temp + rename)
- Used 100ms debounce window -- fast enough to feel responsive, long enough to collapse burst events
- Config watcher failure is non-fatal: logs warning and continues without hot-reload capability
- Provider import added to main.go for typed map in reload callback

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 9 (Configuration) is now complete: TOML config + provider registry + hot-reload
- Ready for Phase 10 (OpenAI-compatible provider) -- CreateProvider switch statement ready for "openai" case
- ProviderRegistry.Update() tested and wired for live provider swapping
- All existing tests pass with zero regressions (31 config tests, full suite green)

---
*Phase: 09-configuration*
*Completed: 2026-04-13*
