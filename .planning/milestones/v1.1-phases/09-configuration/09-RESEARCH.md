# Phase 9: Configuration - Research

**Researched:** 2026-04-12
**Domain:** TOML config parsing, file watching, provider registry, env var resolution
**Confidence:** HIGH

## Summary

Phase 9 introduces a TOML configuration file (`~/.config/fenec/config.toml`) that defines providers with name, type, URL, and optional API key. The system must preserve the zero-config Ollama experience when no config file exists, support `$ENV_VAR` syntax for API keys resolved at load time, and hot-reload config changes via filesystem watching without disrupting the active session.

The technical surface is well-understood: `BurntSushi/toml` for TOML parsing (the standard Go TOML library, explicitly recommended in CLAUDE.md over viper), `fsnotify` for filesystem notifications, and a ProviderRegistry type that maps provider names to `provider.Provider` instances. The main complexity lies in the hot-reload path -- specifically watching a single file correctly (must watch parent directory, not the file itself due to editor atomic saves) and debouncing rapid filesystem events.

The existing codebase already has the right abstractions: `internal/config` for config directory resolution, `provider.Provider` interface for multi-provider support, and `ollama.New(host)` for creating Ollama providers. The main.go currently hardcodes Ollama provider creation and needs refactoring to: load config, build provider registry, pick default provider, pass to REPL.

**Primary recommendation:** Implement a `Config` struct decoded from TOML, a `ProviderRegistry` type (map of name to Provider), a `ProviderFactory` function that creates Provider instances from config entries, and a `ConfigWatcher` that watches the config directory and reloads on changes with debouncing.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Config file location: `~/.config/fenec/config.toml` (consistent with existing `config.ConfigDir()`)
- Provider structure: `[providers.<name>]` TOML sections with fields `type`, `url`, `api_key` (optional), `default_model` (optional)
- Default provider specified via top-level `default_provider = "ollama"` key
- Minimal top-level keys: `default_provider`, `default_model` -- no other settings yet
- Env var syntax: `$VAR_NAME` prefix-only (simple, no braces)
- Resolution timing: at config load time (fail fast if missing)
- Missing env var handling: log warning, leave value empty -- provider fails on first use with clear error
- Literal API keys allowed but warn user ("API keys in plaintext config are not recommended")
- Hot-reload trigger: file watcher via `fsnotify` library
- Active session: preserved intact -- only provider registry updates
- Invalid config on reload: keep old config, log error, show brief message in REPL
- Changes that take effect: new providers added, URLs/API keys updated on existing providers
- Default config created on first run if missing

### Claude's Discretion
- Exact error message wording
- fsnotify debouncing strategy (reasonable default)
- Internal config struct shape
- ProviderFactory design pattern for creating Provider instances from config
- How main.go wires the config loader with provider factory
- Whether to use `BurntSushi/toml` (recommended in research) or alternative

### Deferred Ideas (OUT OF SCOPE)
- Per-provider model metadata (context lengths, capabilities) -- defer to Phase 11 or later
- Provider credential rotation, OS keychain integration -- explicitly out of scope per REQUIREMENTS.md
- Config schema versioning -- add when first breaking change needed
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONF-01 | User can define providers in a TOML config file with name, type, URL, and API key | BurntSushi/toml DecodeFile into Config struct with `[providers.<name>]` sections |
| CONF-02 | User can reference environment variables for API keys in config (e.g., `$OPENAI_API_KEY`) | String prefix detection (`$`) + `os.Getenv()` resolution at load time |
| CONF-03 | User can run Fenec with no config file and get the default Ollama provider automatically | Fallback logic: if config file missing, create default Config with ollama at localhost:11434 |
| CONF-04 | User can modify provider config and have changes take effect without restarting Fenec | fsnotify watching parent directory + debounced reload + registry swap |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/BurntSushi/toml | v1.6.0 | TOML parsing | The dominant Go TOML library. Reflection-based like encoding/json. Supports TOML v1.1.0. CLAUDE.md explicitly recommends it over viper. Zero transitive dependencies beyond stdlib. |
| github.com/fsnotify/fsnotify | v1.9.0 | Filesystem notifications | The standard Go file watcher (used by viper, Hugo, air, etc). Cross-platform (Linux inotify, macOS kqueue, Windows ReadDirectoryChanges). |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| log/slog | stdlib | Structured logging for config events | Log config load, reload, env var warnings, validation errors |
| os | stdlib | Env var resolution | `os.Getenv()` for `$VAR_NAME` resolution at config load time |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| BurntSushi/toml | pelletier/go-toml/v2 | Slightly faster, but BurntSushi is the ecosystem standard and explicitly recommended in CLAUDE.md. No compelling reason to deviate. |
| BurntSushi/toml | knadh/koanf | Full config management framework with hot-reload built in. But it is a much heavier dependency and the project explicitly avoids viper-style config frameworks. |
| fsnotify | polling (time.Ticker + os.Stat) | Simpler, no dependency. But higher latency (must poll every N seconds), misses rapid changes, wastes CPU. fsnotify is event-driven and standard. |

**Installation:**
```bash
go get github.com/BurntSushi/toml@v1.6.0
go get github.com/fsnotify/fsnotify@v1.9.0
```

**Version verification:** BurntSushi/toml v1.6.0 published 2025-12-18. fsnotify v1.9.0 published 2025-04-04. Both verified via Go module proxy.

## Architecture Patterns

### Recommended Project Structure
```
internal/config/
  config.go         # Existing: ConfigDir(), LoadSystemPrompt(), etc.
  toml.go           # NEW: Config struct, LoadConfig(), resolveEnvVars()
  toml_test.go      # NEW: Tests for TOML loading + env var resolution
  watcher.go        # NEW: ConfigWatcher with fsnotify + debounce
  watcher_test.go   # NEW: Tests for watcher (can test debounce logic)
  registry.go       # NEW: ProviderRegistry type
  registry_test.go  # NEW: Tests for registry
```

### Pattern 1: Config Struct with TOML Tags
**What:** Typed Go struct that maps directly to the TOML file structure using struct tags.
**When to use:** Always -- this is how BurntSushi/toml decodes files.
**Example:**
```go
// Source: BurntSushi/toml documentation
type Config struct {
    DefaultProvider string                      `toml:"default_provider"`
    DefaultModel    string                      `toml:"default_model"`
    Providers       map[string]ProviderConfig   `toml:"providers"`
}

type ProviderConfig struct {
    Type         string `toml:"type"`          // "ollama" or "openai"
    URL          string `toml:"url"`
    APIKey       string `toml:"api_key"`       // Raw or $ENV_VAR
    DefaultModel string `toml:"default_model"` // Optional per-provider default
}

func LoadConfig(path string) (*Config, error) {
    var cfg Config
    _, err := toml.DecodeFile(path, &cfg)
    if err != nil {
        return nil, fmt.Errorf("parsing config %s: %w", path, err)
    }
    resolveEnvVars(&cfg)
    return &cfg, nil
}
```

### Pattern 2: Env Var Resolution at Load Time
**What:** Scan all string fields for `$` prefix and resolve via `os.Getenv()`.
**When to use:** During config loading, before creating providers.
**Example:**
```go
func resolveEnvVars(cfg *Config) {
    for name, pc := range cfg.Providers {
        if strings.HasPrefix(pc.APIKey, "$") {
            envName := pc.APIKey[1:] // Strip the $ prefix
            val := os.Getenv(envName)
            if val == "" {
                slog.Warn("env var not set for provider API key",
                    "provider", name, "var", envName)
            } else {
                // Warn if it looks like a literal key was intended
                pc.APIKey = val
            }
            cfg.Providers[name] = pc
        } else if pc.APIKey != "" {
            slog.Warn("API key in plaintext config is not recommended",
                "provider", name)
        }
    }
}
```

### Pattern 3: ProviderRegistry
**What:** A thread-safe map of provider name to `provider.Provider` instance, with a method to get the default.
**When to use:** Created at startup from config, updated on hot-reload.
**Example:**
```go
type ProviderRegistry struct {
    mu              sync.RWMutex
    providers       map[string]provider.Provider
    defaultProvider string
}

func (r *ProviderRegistry) Get(name string) (provider.Provider, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    p, ok := r.providers[name]
    return p, ok
}

func (r *ProviderRegistry) Default() (provider.Provider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    p, ok := r.providers[r.defaultProvider]
    if !ok {
        return nil, fmt.Errorf("default provider %q not found", r.defaultProvider)
    }
    return p, nil
}

func (r *ProviderRegistry) Update(providers map[string]provider.Provider, defaultName string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.providers = providers
    r.defaultProvider = defaultName
}

func (r *ProviderRegistry) Names() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]string, 0, len(r.providers))
    for name := range r.providers {
        names = append(names, name)
    }
    return names
}
```

### Pattern 4: ProviderFactory
**What:** Function that creates a `provider.Provider` from a `ProviderConfig`.
**When to use:** During config loading to populate the registry.
**Example:**
```go
func CreateProvider(name string, cfg ProviderConfig) (provider.Provider, error) {
    switch cfg.Type {
    case "ollama":
        return ollama.New(cfg.URL)
    // Phase 10 will add:
    // case "openai":
    //     return openai.New(cfg.URL, cfg.APIKey)
    default:
        return nil, fmt.Errorf("unknown provider type %q for %q", cfg.Type, name)
    }
}
```

### Pattern 5: Config File Watcher with Debounce
**What:** Watch the config directory (not the file) for changes, debounce events, reload config on write.
**When to use:** Started in main.go after initial config load.
**Example:**
```go
// Source: fsnotify docs + community patterns
type ConfigWatcher struct {
    watcher    *fsnotify.Watcher
    configPath string
    onChange   func() // Called after debounced reload
    done       chan struct{}
}

func (cw *ConfigWatcher) Start() error {
    // Watch the DIRECTORY, not the file (editors use atomic saves)
    dir := filepath.Dir(cw.configPath)
    if err := cw.watcher.Add(dir); err != nil {
        return fmt.Errorf("watching %s: %w", dir, err)
    }

    go func() {
        var debounceTimer *time.Timer
        for {
            select {
            case event, ok := <-cw.watcher.Events:
                if !ok {
                    return
                }
                // Filter: only care about our config file
                if filepath.Clean(event.Name) != filepath.Clean(cw.configPath) {
                    continue
                }
                // Only react to Write and Create (Create handles atomic saves)
                if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
                    continue
                }
                // Debounce: reset timer on each event, fire after 100ms of quiet
                if debounceTimer != nil {
                    debounceTimer.Stop()
                }
                debounceTimer = time.AfterFunc(100*time.Millisecond, cw.onChange)

            case err, ok := <-cw.watcher.Errors:
                if !ok {
                    return
                }
                slog.Error("config watcher error", "error", err)

            case <-cw.done:
                return
            }
        }
    }()
    return nil
}
```

### Pattern 6: Default Config Generation
**What:** When no config file exists, generate a sensible default and optionally write it.
**When to use:** First run or when config file is deleted.
**Example:**
```go
func DefaultConfig() *Config {
    return &Config{
        DefaultProvider: "ollama",
        Providers: map[string]ProviderConfig{
            "ollama": {
                Type: "ollama",
                URL:  "http://localhost:11434",
            },
        },
    }
}

// WriteDefaultConfig writes the default config to disk if it doesn't exist.
func WriteDefaultConfig(path string) error {
    if _, err := os.Stat(path); err == nil {
        return nil // Already exists
    }
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }
    defaultTOML := `default_provider = "ollama"

[providers.ollama]
type = "ollama"
url = "http://localhost:11434"
`
    return os.WriteFile(path, []byte(defaultTOML), 0644)
}
```

### Anti-Patterns to Avoid
- **Watching the file directly instead of the directory:** Text editors (vim, VSCode, nano) use atomic saves (write temp file, rename over original). This invalidates the inode watch and you stop receiving events. Always watch the parent directory and filter by filename.
- **No debouncing on file events:** A single save can generate 2-5 filesystem events (chmod, write, create, rename). Without debouncing, you reload config multiple times for one save. Use a 100ms debounce timer.
- **Blocking the REPL goroutine on config reload:** The watcher runs in its own goroutine. The reload callback must be non-blocking and use the registry's mutex to swap providers atomically.
- **Validating config after creating providers:** Validate the TOML structure and env vars first, then create providers. If provider creation fails for one entry, log the error but keep other valid providers.
- **Using sync.Mutex where sync.RWMutex suffices:** The registry is read-heavy (every chat message reads the provider). Use RWMutex so reads don't block each other.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TOML parsing | Custom key=value parser | BurntSushi/toml | TOML spec is complex (nested tables, arrays of tables, datetime types). Edge cases are numerous. |
| Filesystem watching | Polling loop with os.Stat | fsnotify | inotify/kqueue is OS-specific. fsnotify abstracts cross-platform differences. Polling misses rapid changes and wastes CPU. |
| Config framework | Custom config loader with validation, defaults, env override | Keep it simple: TOML + manual env resolution | The feature set is small (one file, env vars, defaults). A framework adds complexity without value for 3 top-level keys. |

**Key insight:** The config surface is small enough that BurntSushi/toml + manual env resolution is simpler and more maintainable than pulling in a config framework. The complexity is in the hot-reload lifecycle, not the parsing.

## Common Pitfalls

### Pitfall 1: Editor Atomic Saves Break File Watches
**What goes wrong:** You watch the config file directly. User saves in vim/VSCode. The editor writes a new file and renames it over the old one. Your watch is now on a deleted inode. No more events.
**Why it happens:** Editors use atomic saves to prevent data corruption on crash. The original file is replaced, not modified in place.
**How to avoid:** Watch the parent directory (`filepath.Dir(configPath)`). Filter events by `filepath.Clean(event.Name) == filepath.Clean(configPath)`. React to both `Write` and `Create` events (Create handles the atomic rename case).
**Warning signs:** Hot-reload works once but stops working after the first edit. Works with `echo >> file` but not with your editor.

### Pitfall 2: Rapid Event Flooding on Save
**What goes wrong:** A single file save triggers 3-5 events (chmod, truncate, write, etc.). Config reloads multiple times, creating and destroying providers rapidly.
**Why it happens:** OS-level filesystem events are granular. What looks like one "save" operation is multiple kernel events.
**How to avoid:** Debounce with `time.AfterFunc(100*time.Millisecond, reload)`. Reset the timer on each new event. Only the final timer fires.
**Warning signs:** Log shows "config reloaded" 3-5 times per save. Provider connections flap.

### Pitfall 3: Race Between Config Reload and Active Chat
**What goes wrong:** Config reloads mid-stream. The provider pointer changes while a StreamChat call is in progress. Panic or garbled response.
**Why it happens:** The registry update and the streaming goroutine both access the provider.
**How to avoid:** The registry uses RWMutex. The streaming goroutine gets a provider reference at the start of the request (under read lock). The reload swaps the registry map (under write lock). In-flight requests finish with the old provider; new requests get the new one.
**Warning signs:** Panics during streaming when config is being saved.

### Pitfall 4: Missing Config Directory on First Run
**What goes wrong:** `DecodeFile` fails because `~/.config/fenec/` doesn't exist yet. Error handling creates the file but not the directory first.
**Why it happens:** Fresh install has no config directory.
**How to avoid:** When config file is missing: (1) create directory with `os.MkdirAll`, (2) write default config, (3) return default Config in memory. The existing `ConfigDir()` function resolves the path but does not create it.
**Warning signs:** "no such file or directory" error on first run.

### Pitfall 5: Env Var Changes Not Reflected Until Restart
**What goes wrong:** User changes `$OPENAI_API_KEY` in their shell. Config hot-reloads. But `os.Getenv()` returns the value from process start, not the current shell session.
**Why it happens:** Environment variables are inherited at process creation. Changing them in a different shell session doesn't affect the running process.
**How to avoid:** This is expected behavior and not a bug. Document it: "Environment variable changes require restarting Fenec." The `$VAR` syntax is for keeping secrets out of the config file, not for dynamic secret rotation.
**Warning signs:** User reports "I changed the API key but it still doesn't work."

### Pitfall 6: TOML Map Iteration Order
**What goes wrong:** Provider names from `map[string]ProviderConfig` iterate in random order. UI listing or default selection becomes unpredictable.
**Why it happens:** Go maps have random iteration order by design.
**How to avoid:** When listing providers, sort the keys. The `default_provider` key explicitly names the default -- never rely on map iteration order for default selection.
**Warning signs:** `/model` command shows providers in different order each time.

## Code Examples

Verified patterns from official sources:

### Loading TOML Config File
```go
// Source: BurntSushi/toml pkg.go.dev documentation
import "github.com/BurntSushi/toml"

type Config struct {
    DefaultProvider string                    `toml:"default_provider"`
    DefaultModel    string                    `toml:"default_model"`
    Providers       map[string]ProviderConfig `toml:"providers"`
}

type ProviderConfig struct {
    Type         string `toml:"type"`
    URL          string `toml:"url"`
    APIKey       string `toml:"api_key"`
    DefaultModel string `toml:"default_model"`
}

func LoadConfig(path string) (*Config, error) {
    var cfg Config
    md, err := toml.DecodeFile(path, &cfg)
    if err != nil {
        return nil, fmt.Errorf("parsing config %s: %w", path, err)
    }
    // Warn about unknown keys (typos in config)
    if undecoded := md.Undecoded(); len(undecoded) > 0 {
        for _, key := range undecoded {
            slog.Warn("unknown config key", "key", key)
        }
    }
    return &cfg, nil
}
```

### Fsnotify Directory Watch with File Filtering
```go
// Source: fsnotify README + cmd/fsnotify/file.go pattern
import "github.com/fsnotify/fsnotify"

func watchConfigFile(configPath string, onReload func()) (*fsnotify.Watcher, error) {
    w, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    // Watch the DIRECTORY, not the file (handles editor atomic saves)
    dir := filepath.Dir(configPath)
    if err := w.Add(dir); err != nil {
        w.Close()
        return nil, err
    }

    go func() {
        var debounce *time.Timer
        for {
            select {
            case event, ok := <-w.Events:
                if !ok {
                    return
                }
                if filepath.Clean(event.Name) != filepath.Clean(configPath) {
                    continue
                }
                if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
                    continue
                }
                if debounce != nil {
                    debounce.Stop()
                }
                debounce = time.AfterFunc(100*time.Millisecond, onReload)
            case err, ok := <-w.Errors:
                if !ok {
                    return
                }
                slog.Error("config watcher error", "error", err)
            }
        }
    }()

    return w, nil
}
```

### Main.go Integration Pattern
```go
// Sketch of how main.go wiring changes:

// 1. Load or create config
configPath := filepath.Join(configDir, "config.toml")
cfg, err := config.LoadOrCreateConfig(configPath)

// 2. Build provider registry from config
registry := config.NewProviderRegistry()
for name, pc := range cfg.Providers {
    p, err := config.CreateProvider(name, pc)
    if err != nil {
        slog.Error("failed to create provider", "name", name, "error", err)
        continue
    }
    registry.Register(name, p)
}
registry.SetDefault(cfg.DefaultProvider)

// 3. Get default provider for REPL
defaultProvider, err := registry.Default()

// 4. Start config watcher
watcher, err := config.WatchConfig(configPath, func() {
    newCfg, err := config.LoadConfig(configPath)
    if err != nil {
        slog.Error("config reload failed, keeping old config", "error", err)
        return
    }
    // Rebuild provider map
    newProviders := make(map[string]provider.Provider)
    for name, pc := range newCfg.Providers {
        p, err := config.CreateProvider(name, pc)
        if err != nil {
            slog.Error("failed to create provider on reload", "name", name, "error", err)
            continue
        }
        newProviders[name] = p
    }
    registry.Update(newProviders, newCfg.DefaultProvider)
    slog.Info("config reloaded", "providers", len(newProviders))
})
defer watcher.Close()
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| viper for config management | BurntSushi/toml + manual env | Always (for small Go CLIs) | Avoids heavy dependency graph. CLAUDE.md explicitly forbids viper. |
| fsnotify v1.4.x | fsnotify v1.9.0 | v1.5+ (2022) | Added `Event.Has()` method, buffered watcher option. v1.9.0 is latest. |
| BurntSushi/toml v1.3.x | BurntSushi/toml v1.6.0 | Dec 2025 | Latest stable. TOML v1.1.0 support. No breaking API changes from v1.x series. |

**Deprecated/outdated:**
- `fsnotify.Op == fsnotify.Write`: Use `Event.Has(fsnotify.Write)` instead (bitmask, not equality).
- Watching files directly: Always watch the parent directory per fsnotify v1.5+ guidance.

## Open Questions

1. **Where should ProviderRegistry live?**
   - What we know: It needs to be accessible from main.go and potentially REPL. It is closely tied to config but used by provider consumers.
   - What's unclear: Should it be in `internal/config/` (near config loading) or `internal/provider/` (near Provider interface)?
   - Recommendation: Put it in `internal/config/` since it is created from config and the provider package should stay interface-only. The factory function that creates providers from config also belongs in `internal/config/` since it imports specific provider packages (ollama, future openai).

2. **Should the REPL hold a registry reference or a single Provider?**
   - What we know: Currently REPL takes a single `provider.Provider`. Hot-reload needs to swap providers.
   - What's unclear: Whether to pass the registry to REPL or keep passing a single Provider and have the reload callback update it.
   - Recommendation: For Phase 9, keep REPL taking a single Provider. The registry manages which provider is "active" and the reload callback can update a pointer that main.go passes. Phase 10/11 (provider switching) will introduce registry access to the REPL. This minimizes Phase 9 changes to the REPL.

3. **Config file creation on first run: write to disk or in-memory only?**
   - What we know: CONTEXT.md says "default config created on first run if missing" and shows the TOML content.
   - What's unclear: Whether to actually write the file or just use defaults in memory.
   - Recommendation: Write the default config file to disk. This gives users a starting point to edit and makes the watcher work immediately (has a file to watch). The Specifics section in CONTEXT.md provides the exact TOML to write.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed (go test discovers *_test.go) |
| Quick run command | `go test ./internal/config/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONF-01 | Parse TOML config with providers | unit | `go test ./internal/config/... -run TestLoadConfig -x` | No -- Wave 0 |
| CONF-01 | Validate provider config fields | unit | `go test ./internal/config/... -run TestValidateConfig -x` | No -- Wave 0 |
| CONF-02 | Resolve $ENV_VAR in api_key | unit | `go test ./internal/config/... -run TestResolveEnvVars -x` | No -- Wave 0 |
| CONF-02 | Warn on missing env var | unit | `go test ./internal/config/... -run TestMissingEnvVar -x` | No -- Wave 0 |
| CONF-02 | Warn on plaintext API key | unit | `go test ./internal/config/... -run TestPlaintextKeyWarning -x` | No -- Wave 0 |
| CONF-03 | Return default config when file missing | unit | `go test ./internal/config/... -run TestDefaultConfig -x` | No -- Wave 0 |
| CONF-03 | Write default config file on first run | unit | `go test ./internal/config/... -run TestWriteDefaultConfig -x` | No -- Wave 0 |
| CONF-04 | Debounced reload on file change | unit | `go test ./internal/config/... -run TestWatcherDebounce -x` | No -- Wave 0 |
| CONF-04 | Keep old config on invalid reload | unit | `go test ./internal/config/... -run TestInvalidReload -x` | No -- Wave 0 |
| CONF-04 | Registry thread-safe read/update | unit | `go test ./internal/config/... -run TestRegistry -x -race` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/...`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/config/toml_test.go` -- covers CONF-01, CONF-02, CONF-03
- [ ] `internal/config/watcher_test.go` -- covers CONF-04 debounce + invalid reload
- [ ] `internal/config/registry_test.go` -- covers CONF-04 thread safety

*(No framework install needed -- Go testing + testify already in go.mod)*

## Sources

### Primary (HIGH confidence)
- [BurntSushi/toml pkg.go.dev](https://pkg.go.dev/github.com/BurntSushi/toml) - DecodeFile, MetaData.Undecoded(), struct tag API verified
- [fsnotify pkg.go.dev](https://pkg.go.dev/github.com/fsnotify/fsnotify) - Watcher API, Event.Has(), Op constants verified
- [fsnotify GitHub README](https://github.com/fsnotify/fsnotify) - Directory vs file watching guidance, atomic save warning
- [fsnotify file.go example](https://github.com/fsnotify/fsnotify/blob/main/cmd/fsnotify/file.go) - Parent directory watch pattern
- Go module proxy: BurntSushi/toml v1.6.0 (2025-12-18), fsnotify v1.9.0 (2025-04-04)

### Secondary (MEDIUM confidence)
- [fsnotify debouncing patterns](https://www.golinuxcloud.com/golang-watcher-fsnotify/) - Community debounce patterns cross-verified with official docs
- [Hot-reload Go applications](https://itnext.io/clean-and-simple-hot-reloading-on-uninterrupted-go-applications-5974230ab4c5) - General hot-reload lifecycle patterns

### Tertiary (LOW confidence)
- None -- all findings verified with primary sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - BurntSushi/toml and fsnotify are well-established, versions verified via Go module proxy
- Architecture: HIGH - Patterns derived from official documentation and existing codebase analysis
- Pitfalls: HIGH - Atomic save issue is documented by fsnotify itself; race conditions are standard Go concurrency concerns

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable libraries, low churn)
