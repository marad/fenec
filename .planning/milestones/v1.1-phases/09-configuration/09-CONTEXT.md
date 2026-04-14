# Phase 9: Configuration - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Introduce a TOML config file at `~/.config/fenec/config.toml` that defines providers (name, type, URL, API key) and defaults. When no config file exists, Fenec auto-creates a default Ollama provider at localhost:11434. Config changes are detected via file watcher and applied to the provider registry without restarting. API keys reference environment variables with `$VAR` syntax.

</domain>

<decisions>
## Implementation Decisions

### Config File Format

- Config file location: `~/.config/fenec/config.toml` (consistent with existing `config.ConfigDir()`)
- Provider structure: `[providers.<name>]` TOML sections with fields `type`, `url`, `api_key` (optional), `default_model` (optional)
- Default provider specified via top-level `default_provider = "ollama"` key
- Minimal top-level keys: `default_provider`, `default_model` — no other settings yet
- Example structure:
  ```toml
  default_provider = "ollama"
  default_model = "gemma4"

  [providers.ollama]
  type = "ollama"
  url = "http://localhost:11434"

  [providers.lmstudio]
  type = "openai"
  url = "http://localhost:1234"

  [providers.openai]
  type = "openai"
  url = "https://api.openai.com"
  api_key = "$OPENAI_API_KEY"
  ```

### Env Var Resolution & API Keys

- Env var syntax: `$VAR_NAME` prefix-only (simple, no braces)
- Resolution timing: at config load time (fail fast if missing)
- Missing env var handling: log warning, leave value empty — provider fails on first use with clear error
- Literal API keys allowed but warn user ("API keys in plaintext config are not recommended")

### Hot-Reload Behavior

- Trigger mechanism: file watcher via `fsnotify` library
- Active session: preserved intact — only provider registry updates
- Invalid config on reload: keep old config, log error, show brief message in REPL
- Changes that take effect: new providers added, URLs/API keys updated on existing providers

### Claude's Discretion

- Exact error message wording
- fsnotify debouncing strategy (reasonable default)
- Internal config struct shape
- ProviderFactory design pattern for creating Provider instances from config
- How main.go wires the config loader with provider factory
- Whether to use `BurntSushi/toml` (recommended in research) or alternative

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` — existing config package with ConfigDir(), SessionDir(), ToolsDir(), LoadSystemPrompt()
- `internal/provider/provider.go` — Provider interface ready for multi-provider registry
- `internal/provider/ollama/ollama.go` — Ollama adapter (constructor: `ollama.NewProvider(host string)`)
- `main.go` — currently creates Ollama provider directly via `ollama.NewProvider()`

### Established Patterns
- `os.UserConfigDir()` for cross-platform config directory resolution
- Graceful fallback for missing files (system.md returns default)
- Constructor functions with defaults when values absent
- Package-level constants for defaults (DefaultHost, AppName, Version)

### Integration Points
- `main.go` needs refactoring: load config → build provider registry → pick default provider → pass to REPL
- Multiple providers need a registry type (map by name → Provider instance)
- REPL still uses a single Provider (selection is Phase 10/11 concern)
- New dependency: `github.com/BurntSushi/toml` for TOML parsing
- New dependency: `github.com/fsnotify/fsnotify` for config file watching

</code_context>

<specifics>
## Specific Ideas

Default config created on first run if missing:
```toml
default_provider = "ollama"

[providers.ollama]
type = "ollama"
url = "http://localhost:11434"
```

</specifics>

<deferred>
## Deferred Ideas

- Per-provider model metadata (context lengths, capabilities) — defer to Phase 11 or later
- Provider credential rotation, OS keychain integration — explicitly out of scope per REQUIREMENTS.md
- Config schema versioning — add when first breaking change needed

</deferred>
