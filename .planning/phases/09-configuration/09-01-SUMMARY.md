---
phase: 09-configuration
plan: 01
subsystem: config
tags: [toml, config, provider-registry, env-vars, BurntSushi-toml]

# Dependency graph
requires:
  - phase: 08-provider-abstraction
    provides: provider.Provider interface and ollama.New factory
provides:
  - Config struct with TOML loading and env var resolution
  - ProviderRegistry with thread-safe Get/Default/Update/Names
  - CreateProvider factory for config-driven provider creation
  - LoadOrCreateConfig with default Ollama fallback
  - Config-driven main.go startup replacing hardcoded Ollama
affects: [10-openai-provider, 11-model-routing, configuration]

# Tech tracking
tech-stack:
  added: [github.com/BurntSushi/toml v1.6.0]
  patterns: [TOML config with env var resolution, provider registry pattern, config-driven startup]

key-files:
  created:
    - internal/config/toml.go
    - internal/config/toml_test.go
    - internal/config/registry.go
    - internal/config/registry_test.go
  modified:
    - main.go
    - go.mod
    - go.sum

key-decisions:
  - "Used BurntSushi/toml v1.6.0 for TOML parsing per CLAUDE.md recommendation"
  - "ProviderRegistry lives in internal/config since it is created from config and factory imports provider packages"
  - "Renamed tool registry variable to toolRegistry in main.go to avoid collision with providerRegistry"

patterns-established:
  - "Config-driven provider initialization: LoadOrCreateConfig -> CreateProvider -> ProviderRegistry"
  - "Env var resolution at load time with $ prefix syntax and slog warnings"
  - "WriteDefaultConfig no-overwrite pattern for first-run experience"

requirements-completed: [CONF-01, CONF-02, CONF-03]

# Metrics
duration: 4min
completed: 2026-04-13
---

# Phase 9 Plan 1: Configuration Summary

**TOML config loading with $ENV_VAR API key resolution, provider registry, and config-driven main.go startup replacing hardcoded Ollama**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-13T04:52:55Z
- **Completed:** 2026-04-13T04:57:22Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Config struct with TOML tags parses `[providers.<name>]` sections from `~/.config/fenec/config.toml`
- `$ENV_VAR` syntax resolved at load time via os.Getenv with slog warnings for missing/plaintext keys
- Zero-config first run creates default Ollama config file and returns sensible defaults
- ProviderRegistry with RWMutex for thread-safe concurrent access
- main.go is fully config-driven: no hardcoded ollama.New, uses registry.Default()
- 18 new tests covering all config functions, registry operations, and concurrent access

## Task Commits

Each task was committed atomically:

1. **Task 1: Config loading, env var resolution, and provider factory** - `6676efc` (feat)
2. **Task 2: Provider registry and main.go config-driven startup** - `b2eb98d` (feat)

_Both tasks used TDD: tests written first (RED), then implementation (GREEN)._

## Files Created/Modified
- `internal/config/toml.go` - Config/ProviderConfig structs, LoadConfig, resolveEnvVars, DefaultConfig, WriteDefaultConfig, LoadOrCreateConfig, CreateProvider
- `internal/config/toml_test.go` - 12 tests for TOML loading, env var resolution, default config, provider factory
- `internal/config/registry.go` - ProviderRegistry with sync.RWMutex, Get/Default/Update/Names/Register/SetDefault
- `internal/config/registry_test.go` - 6 tests including concurrent access with race detector
- `main.go` - Config-driven startup, removed --host flag and hardcoded ollama import, renamed tool registry
- `go.mod` - Added github.com/BurntSushi/toml v1.6.0
- `go.sum` - Updated checksums

## Decisions Made
- Used BurntSushi/toml v1.6.0 as recommended in CLAUDE.md (over viper or pelletier/go-toml)
- Placed ProviderRegistry in internal/config/ since it is created from config and the factory function imports specific provider packages
- Renamed tool registry variable from `registry` to `toolRegistry` to avoid name collision with the provider registry
- Health check error message now shows config-driven provider name and URL instead of hardcoded ollamaHost

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. The default config is auto-created on first run.

## Next Phase Readiness
- Config loading infrastructure ready for Phase 9 Plan 2 (config file watcher / hot-reload)
- ProviderRegistry.Update() method ready for hot-reload use case
- CreateProvider factory ready to add OpenAI-compatible case in Phase 10
- All existing tests pass with zero regressions

---
*Phase: 09-configuration*
*Completed: 2026-04-13*
