# Architecture: Profiles, Subcommands & Config Migration

**Domain:** Integrating profiles, CLI subcommands, and config path migration into existing Fenec CLI
**Researched:** 2025-07-14
**Overall confidence:** HIGH

## Current Architecture Summary

The existing codebase follows a clean layered structure:

```
main.go (340 lines)
  ├── pflag parsing (global FlagSet, no subcommands)
  ├── config.ConfigDir() → config.LoadOrCreateConfig()
  ├── ProviderRegistry build from config.Providers
  ├── --model flag → provider/model resolution
  ├── config.LoadSystemPrompt() → system.md or default
  ├── Tool registry setup
  └── repl.NewREPL(provider, model, systemPrompt, ...) → r.Run()

internal/config/
  ├── config.go    — ConfigDir(), LoadSystemPrompt(), SessionDir(), ToolsDir(), HistoryFile()
  ├── toml.go      — Config struct, LoadConfig(), CreateProvider()
  ├── registry.go  — ProviderRegistry (thread-safe provider map)
  └── watcher.go   — Hot-reload via fsnotify

internal/repl/
  ├── repl.go      — REPL struct, Run(), sendMessage(), slash command dispatch
  └── commands.go  — ParseCommand(), helpText

internal/chat/     — Conversation, ContextTracker
internal/provider/ — Provider interface + ollama/openai/copilot adapters
internal/session/  — Session persistence (JSON files)
internal/tool/     — Tool registry, built-in tools, Lua tools
```

### Key Data Flow: System Prompt

```
config.LoadSystemPrompt()  →  main.go (systemPrompt string)
                                  ↓
                           repl.NewREPL(systemPrompt)
                                  ↓
                           baseSystemPrompt = systemPrompt
                           systemPrompt += tool descriptions
                                  ↓
                           chat.NewConversation(model, systemPrompt)
                                  ↓
                           conv.Messages[0] = {Role: "system", Content: systemPrompt}
```

### Key Data Flow: Provider/Model Resolution

```
cfg.DefaultProvider + cfg.DefaultModel  →  providerRegistry.Default()
         ↓                                          ↓
--model flag override  →  provider/model split  →  p, modelName
         ↓
p.Ping() → p.ListModels() → defaultModel
         ↓
repl.NewREPL(p, defaultModel, activeProviderName, ...)
```

### Key Data Flow: Config Paths

All paths funnel through `config.ConfigDir()`:
```
ConfigDir() = os.UserConfigDir() + "/fenec"
  ├── config.toml       (LoadOrCreateConfig)
  ├── system.md         (LoadSystemPrompt)
  ├── sessions/         (SessionDir)
  ├── tools/            (ToolsDir)
  └── history           (HistoryFile)
```

On macOS: `os.UserConfigDir()` → `~/Library/Application Support`
On Linux: `os.UserConfigDir()` → `$XDG_CONFIG_HOME` or `~/.config`

## Recommended Architecture

### Component Boundaries

| Component | Responsibility | New/Modified | Communicates With |
|-----------|---------------|--------------|-------------------|
| `internal/profile/` | Profile struct, load, parse, list, create, validate | **NEW** | config (paths) |
| `internal/config/config.go` | ConfigDir() now returns `~/.config/fenec`; ProfilesDir(); MigrateIfNeeded() | **MODIFIED** | main.go, profile |
| `main.go` | Subcommand routing, `--profile`/`--system` flags, migration call | **MODIFIED** | config, profile, repl |
| `internal/repl/repl.go` | `/clear` command handler | **MODIFIED** | chat.Conversation |
| `internal/repl/commands.go` | Updated helpText with `/clear` | **MODIFIED** | — |

### 1. Profile System (`internal/profile/`)

**New package.** Profiles are markdown files with TOML frontmatter stored in `~/.config/fenec/profiles/`.

#### Profile File Format

```markdown
+++
model = "copilot/gpt-4o"
+++

You are a senior Go developer. Always use table-driven tests...
```

Use `+++` delimiters for TOML frontmatter (Hugo convention — unambiguous vs `---` which is YAML). The project already depends on `github.com/BurntSushi/toml`, so parsing is free.

#### Profile Struct

```go
package profile

type Profile struct {
    Name   string // derived from filename (without .md extension)
    Model  string // "provider/model" or bare "model" — uses same syntax as --model flag
    Prompt string // markdown body = system prompt
}
```

**Design decisions:**
- **No separate `provider` field.** The `--model` flag already supports `provider/model` syntax. Profiles reuse the same parsing. One field, one syntax, one code path.
- **Name from filename**, not a field in frontmatter. `profiles/coder.md` → name is `coder`. No sync issues between filename and content.
- **Prompt is the entire markdown body** after the frontmatter. No special parsing needed.
- **Model is optional.** If omitted, uses config default. This lets profiles be prompt-only.

#### Parsing Logic

```go
func Load(profilesDir, name string) (*Profile, error) {
    data, err := os.ReadFile(filepath.Join(profilesDir, name+".md"))
    if err != nil { return nil, err }

    // Split on +++ delimiters
    parts := splitFrontmatter(string(data))
    
    var fm struct {
        Model string `toml:"model"`
    }
    if parts.frontmatter != "" {
        toml.Decode(parts.frontmatter, &fm)
    }
    
    return &Profile{
        Name:   name,
        Model:  fm.Model,
        Prompt: strings.TrimSpace(parts.body),
    }, nil
}
```

The `splitFrontmatter()` helper handles `+++` delimiter splitting. No external dependency needed — it's ~15 lines of string splitting.

#### Profile Integration Point in main.go

Profiles slot into the startup flow **between config loading and provider resolution**:

```
BEFORE (current):
  1. config.ConfigDir()
  2. config.LoadOrCreateConfig()
  3. Build ProviderRegistry
  4. --model flag → provider/model resolution
  5. config.LoadSystemPrompt()
  6. Create REPL

AFTER (with profiles):
  1. config.ConfigDir()
  1a. config.MigrateIfNeeded()          ← NEW: config migration
  2. config.LoadOrCreateConfig()
  3. Build ProviderRegistry
  4. --profile flag → profile.Load()     ← NEW: may override model + prompt
  5. --system flag → read file            ← NEW: may override prompt
  6. --model flag → provider/model        (profile may have set this already)
  7. systemPrompt resolution (profile prompt > --system file > system.md > default)
  8. Create REPL
```

**Priority chain for system prompt:**
1. `--system <file>` (highest — explicit per-invocation override)
2. `--profile <name>` prompt (profile's markdown body)
3. `config.LoadSystemPrompt()` (system.md or hardcoded default)

**Priority chain for model:**
1. `--model <provider/model>` (highest — explicit per-invocation override)
2. `--profile <name>` model field
3. `cfg.DefaultModel` / first available model (existing behavior)

This means `--profile coder --model ollama/gemma4` uses the coder profile's system prompt but overrides its model. `--profile coder --system custom.md` uses coder's model but overrides the prompt. Both overrides are useful and natural.

### 2. CLI Subcommands (Manual Routing, No Cobra)

**Do NOT add cobra.** The project needs exactly one subcommand group (`profile`) with 3 sub-subcommands. Cobra would:
- Require rewriting the entire CLI layer
- Add a heavy transitive dependency
- Be massive overkill for 3 commands

**Instead:** Use `pflag.Args()` for manual subcommand dispatch. pflag already separates flags from positional arguments.

#### Routing Pattern

```go
func main() {
    // Parse global flags (existing)
    modelName := pflag.StringP("model", "m", "", "...")
    profileName := pflag.StringP("profile", "P", "", "Named profile to activate")
    systemFile := pflag.String("system", "", "System prompt file override")
    // ... existing flags ...
    pflag.Parse()

    // Check for subcommands FIRST
    args := pflag.Args()
    if len(args) > 0 {
        switch args[0] {
        case "profile":
            handleProfileSubcommand(args[1:])
            return
        default:
            fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
            os.Exit(1)
        }
    }

    // ... existing chat startup flow ...
}

func handleProfileSubcommand(args []string) {
    if len(args) == 0 {
        // same as "list"
        profileList()
        return
    }
    switch args[0] {
    case "list":
        profileList()
    case "create":
        if len(args) < 2 { usage(); return }
        profileCreate(args[1])
    case "edit":
        if len(args) < 2 { usage(); return }
        profileEdit(args[1])
    default:
        fmt.Fprintf(os.Stderr, "Unknown profile command: %s\n", args[0])
        os.Exit(1)
    }
}
```

**Why this works:**
- `fenec` → no positional args → enters chat mode (existing behavior)
- `fenec --profile coder` → flag consumed by pflag, no positional args → chat with profile
- `fenec profile list` → `pflag.Args()` = `["profile", "list"]` → subcommand dispatch
- `fenec profile create coder` → `pflag.Args()` = `["profile", "create", "coder"]`
- `fenec --model ollama/gemma4 profile list` → global flags parsed, positional args dispatched

**Subcommand implementations:**
- `profile list` — reads profiles dir, prints name + model for each
- `profile create <name>` — creates template `.md` file, opens `$EDITOR`
- `profile edit <name>` — opens existing profile in `$EDITOR`

These are simple enough to live in a `cmd_profile.go` file in `main` package or as functions in `internal/profile/`.

### 3. Config Path Migration

**Problem:** On macOS, `os.UserConfigDir()` returns `~/Library/Application Support`. This is correct per Apple HIG but terrible for CLI tools:
- Hidden in Finder (Library is hidden by default)
- Long path with spaces (annoying for shell)
- Inconsistent with every other CLI tool (`brew`, `gh`, `git` all use `~/.config` or dotfiles)

**Solution:** Change `ConfigDir()` to always return `~/.config/fenec` on macOS. On Linux, `os.UserConfigDir()` already returns `~/.config` when `$XDG_CONFIG_HOME` is unset, so no change needed.

#### Modified ConfigDir()

```go
func ConfigDir() (string, error) {
    if runtime.GOOS == "darwin" {
        home, err := os.UserHomeDir()
        if err != nil {
            return "", err
        }
        return filepath.Join(home, ".config", AppName), nil
    }
    // Linux/Windows: use standard os.UserConfigDir()
    base, err := os.UserConfigDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(base, AppName), nil
}
```

#### Migration Logic

```go
func MigrateIfNeeded() error {
    if runtime.GOOS != "darwin" {
        return nil // Only macOS needs migration
    }
    
    oldDir := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", AppName)
    newDir, err := ConfigDir() // now returns ~/.config/fenec
    if err != nil {
        return err
    }
    
    oldExists := dirExists(oldDir)
    newExists := dirExists(newDir)
    
    switch {
    case !oldExists:
        return nil // Nothing to migrate
    case oldExists && newExists:
        slog.Warn("both old and new config dirs exist",
            "old", oldDir, "new", newDir)
        return nil // User already migrated or has both; don't touch
    case oldExists && !newExists:
        // Migrate: move old → new
        if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
            return err
        }
        if err := os.Rename(oldDir, newDir); err != nil {
            // Cross-device? Fall back to copy+remove
            return copyDir(oldDir, newDir)
        }
        slog.Info("migrated config directory", "from", oldDir, "to", newDir)
        return nil
    }
    return nil
}
```

**Why `os.Rename()` first:**
- Same filesystem → atomic, instant
- Cross-device (unlikely for home dir) → fall back to recursive copy
- No partial state possible with rename

**Why not symlink:** Creates confusion, `ls -la` shows symlink, tools that resolve symlinks get different paths. Clean move is simpler.

**Call site in main.go:**
```go
// Very first thing after parsing flags, before ConfigDir() is used
if err := config.MigrateIfNeeded(); err != nil {
    slog.Warn("config migration failed", "error", err)
    // Non-fatal: continue with whatever path works
}
```

#### New Helper: ProfilesDir()

```go
func ProfilesDir() (string, error) {
    dir, err := ConfigDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, "profiles"), nil
}
```

### 4. /clear Command

Minimal change to REPL. Add to slash command dispatch in `Run()`:

```go
case "/clear":
    r.handleClearCommand()
```

#### Implementation

```go
func (r *REPL) handleClearCommand() {
    // Preserve system prompt (always Messages[0] if present)
    var systemMsg []model.Message
    if len(r.conv.Messages) > 0 && r.conv.Messages[0].Role == "system" {
        systemMsg = r.conv.Messages[:1]
    }
    
    // Reset conversation
    r.conv.Messages = systemMsg
    
    // Reset context tracker
    if r.tracker != nil {
        r.tracker.Update(0, 0)
    }
    
    // Start fresh session (don't corrupt existing auto-save)
    r.session = session.NewSession(r.conv.Model)
    r.autoSaved = sync.Once{} // Reset so new session can auto-save
    
    fmt.Fprintln(r.rl.Stdout(), "Conversation cleared.")
}
```

**Key detail:** `r.autoSaved = sync.Once{}` resets the sync.Once so the new session can be auto-saved on exit. Without this, the old session's auto-save flag would prevent saving the new conversation.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Cobra for 3 Commands
**What:** Adding `github.com/spf13/cobra` as a dependency for profile subcommands
**Why bad:** Cobra requires restructuring the entire CLI into a command tree. The current pflag-based main.go is 340 lines and straightforward. Cobra would triple the boilerplate for 3 simple commands.
**Instead:** Manual routing with `pflag.Args()`. If a future milestone adds 5+ subcommand groups, migrate to cobra then.

### Anti-Pattern 2: Profile as TOML-only Config
**What:** Storing profiles as `.toml` files with a `prompt = "..."` field
**Why bad:** System prompts are multi-paragraph markdown. Escaping multi-line strings in TOML is awkward (`"""..."""`), no syntax highlighting for the prompt content, terrible editing experience.
**Instead:** Markdown files with TOML frontmatter. The body IS the prompt. Edit with any markdown editor. Syntax highlighting works naturally.

### Anti-Pattern 3: Separate Provider Field in Profiles
**What:** `model = "gpt-4o"` + `provider = "copilot"` as separate frontmatter fields
**Why bad:** Duplicates the `provider/model` parsing that `--model` already handles. Two fields to keep in sync. Inconsistent with CLI usage.
**Instead:** Single `model = "copilot/gpt-4o"` field using the same `provider/model` syntax as `--model` flag. One code path for resolution.

### Anti-Pattern 4: Migrating Config by Copying File-by-File
**What:** `cp config.toml ...; cp system.md ...; cp -r sessions/ ...`
**Why bad:** Partial failure leaves split state. Misses files added in future versions. More code, more bugs.
**Instead:** `os.Rename()` on the entire directory. Atomic on same filesystem. Single operation, zero partial states.

### Anti-Pattern 5: Making --system and --profile Mutually Exclusive
**What:** Error if both `--system` and `--profile` are specified
**Why bad:** Unnecessary restriction. A user may want a profile's model settings with a different system prompt.
**Instead:** Clear priority chain: `--system` overrides profile prompt, `--model` overrides profile model. Composable, predictable.

## Patterns to Follow

### Pattern 1: Flag Layering with Clear Priority
**What:** Each configuration source has a defined priority, later sources override earlier ones.
**When:** Resolving model and system prompt from multiple sources.
```
config.toml defaults < profile settings < CLI flags
```
This matches standard Unix convention (config file < environment < CLI flag).

### Pattern 2: Filename-as-Identity
**What:** Profile name is derived from filename, not stored in file content.
**When:** Any named resource stored as a file.
```
profiles/coder.md    → name = "coder"
profiles/writer.md   → name = "writer"
```
Prevents name/filename desync. `ls profiles/` is the source of truth.

### Pattern 3: Package Separation for New Domain Concepts
**What:** Profile is a new domain concept → new `internal/profile/` package.
**When:** Adding a concept that has its own struct, parsing, validation, and file I/O.
**Why not config package:** Config package handles TOML config + path resolution. Profile has different file format (frontmatter+markdown), different storage location, different lifecycle. Mixing them would bloat config into a god package.

## Build Order (Dependency-Driven)

The six features have these dependencies:

```
Config migration ──→ (all features depend on correct config path)
       │
       ├──→ Profile package (needs ProfilesDir())
       │         │
       │         ├──→ --profile flag (needs profile.Load())
       │         │
       │         └──→ fenec profile subcommands (needs profile.*)
       │
       ├──→ --system flag (needs ConfigDir() to resolve relative paths)
       │
       └──→ /clear command (independent, only touches REPL)
```

### Recommended Build Order

| Phase | Feature | Why This Order |
|-------|---------|---------------|
| 1 | Config path migration | Foundation — all other features use config paths. Must be first so profiles dir, system prompt, etc. use the new `~/.config/fenec` path. |
| 2 | `/clear` command | Independent, zero dependencies on other features. Quick win, builds confidence. |
| 3 | Profile package (`internal/profile/`) | Core data model needed by both `--profile` flag and `fenec profile` subcommands. No main.go changes yet. |
| 4 | `--system` flag | Simple flag addition to main.go. Modifies startup flow minimally. Good warmup for the profile flag integration. |
| 5 | `--profile` flag | Uses profile package from phase 3. Modifies the startup flow in main.go (model/provider/prompt resolution). More complex integration. |
| 6 | `fenec profile` subcommands | Requires profile package + adds subcommand routing to main.go. Most invasive to main.go structure. Benefits from having the profile loading path already tested via `--profile`. |

**Why not build subcommands before --profile flag?** The `--profile` flag exercises `profile.Load()` in the real startup flow. If the profile format or loading has issues, you'll find them before building management commands on top.

## Files Changed Per Feature

### Config Migration
- **Modified:** `internal/config/config.go` — new `ConfigDir()` logic, `MigrateIfNeeded()`, `ProfilesDir()`
- **Modified:** `main.go` — call `MigrateIfNeeded()` early
- **New:** `internal/config/migrate.go` (optional: could put migration logic in separate file for clarity)

### /clear Command
- **Modified:** `internal/repl/repl.go` — add `/clear` case + `handleClearCommand()`
- **Modified:** `internal/repl/commands.go` — update `helpText`

### Profile Package
- **New:** `internal/profile/profile.go` — Profile struct, Load(), List(), Create()
- **New:** `internal/profile/profile_test.go`

### --system Flag
- **Modified:** `main.go` — new pflag, load file, pass to system prompt chain

### --profile Flag
- **Modified:** `main.go` — new pflag, load profile, override model/provider/prompt in startup flow

### fenec profile Subcommands
- **Modified:** `main.go` — subcommand routing via pflag.Args()
- **New:** `cmd_profile.go` (or function block in main.go — depends on complexity)

## Scalability Considerations

| Concern | Now (v1.3) | Future |
|---------|-----------|--------|
| Number of profiles | 1-10 files, `readdir` is instant | If 100+, add caching. Unlikely for personal tool. |
| Profile loading | Read file + parse frontmatter per invocation | Already fast (<1ms). No optimization needed. |
| Config migration | One-time `os.Rename()` | Runs once, then never again. Self-cleaning. |
| Subcommand growth | 1 group (`profile`), 3 commands | If 3+ groups emerge, evaluate cobra migration. |

## Sources

- **Go `os.UserConfigDir()` behavior:** Official Go docs (verified via `go doc os UserConfigDir`) — HIGH confidence
- **Go `os.UserHomeDir()`:** Official Go docs — HIGH confidence
- **pflag `Args()` for subcommand routing:** pflag source code, standard Go CLI pattern — HIGH confidence
- **TOML frontmatter `+++` convention:** Hugo documentation convention — HIGH confidence
- **BurntSushi/toml already in go.mod:** Verified in `go.mod` — HIGH confidence
- **Existing codebase analysis:** Direct source code reading — HIGH confidence
