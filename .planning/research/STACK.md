# Technology Stack: Profiles & Config Migration

**Project:** Fenec v1.3 — Profiles, Subcommands, Config Migration
**Researched:** 2025-07-18
**Confidence:** HIGH

## Scope

This document covers ONLY the stack decisions needed for v1.3 features: profile system, CLI subcommands, config path migration, --system/--profile flags, and /clear. The existing v1.0–v1.2 stack (Go 1.25, gopher-lua, Ollama, openai-go v3, BurntSushi/toml, pflag, readline, fsnotify, glamour/lipgloss) is validated and unchanged.

## Executive Decision

**Zero new dependencies needed.** All v1.3 features are implementable with the existing dependency set plus Go stdlib. This is a direct consequence of good prior stack choices (BurntSushi/toml already has string-based decoding, pflag already has FlagSet scoping, Go stdlib provides os/exec and path utilities).

## Recommended Stack (Existing Deps Only)

### Feature-to-Library Mapping

| Feature | Implementation | Library | Status |
|---------|---------------|---------|--------|
| TOML frontmatter parsing | `toml.Decode(string, &struct)` | BurntSushi/toml v1.6.0 | Already in go.mod |
| CLI subcommand routing | `os.Args` dispatch + `pflag.NewFlagSet()` | spf13/pflag v1.0.10 | Already in go.mod |
| $EDITOR integration | `os/exec.Command()` with stdio wiring | Go stdlib `os/exec` | Always available |
| XDG config paths | `os.UserHomeDir()` + `$XDG_CONFIG_HOME` | Go stdlib `os` | Always available |
| Config path migration | `os.Rename()` + `os.MkdirAll()` | Go stdlib `os` | Always available |
| /clear REPL command | `chat.NewConversation()` reset | Internal `chat` package | Already exists |
| Profile file I/O | `os.ReadFile()` / `os.WriteFile()` | Go stdlib `os` | Always available |
| macOS platform detection | `runtime.GOOS == "darwin"` | Go stdlib `runtime` | Always available |

## Detailed Analysis

### 1. TOML Frontmatter Parsing

**Approach:** Hand-roll `+++` delimiter splitting, decode with existing BurntSushi/toml.

**Why:** BurntSushi/toml v1.6.0 already provides `toml.Decode(data string, v any) (MetaData, error)` — confirmed via `go doc`. The project currently only uses `toml.DecodeFile()` (in `internal/config/toml.go:38`), but `Decode()` from a string is a first-class API in the same package.

**Profile file format:**
```markdown
+++
model = "ollama/gemma4"
provider = "ollama"
temperature = 0.7
+++

You are a coding assistant specialized in Go...
```

**Why `+++` not `---`:** `---` is the YAML frontmatter convention (Hugo, Jekyll). `+++` is the TOML frontmatter convention (Hugo, Zola). Since the project is TOML-standardized, use `+++` for consistency with the broader ecosystem.

**Implementation pattern (~25 lines):**
```go
func ParseProfile(data []byte) (*ProfileFrontmatter, string, error) {
    content := string(data)
    if !strings.HasPrefix(content, "+++\n") {
        // No frontmatter — entire file is markdown body
        return &ProfileFrontmatter{}, strings.TrimSpace(content), nil
    }
    endIdx := strings.Index(content[4:], "\n+++")
    if endIdx == -1 {
        return nil, "", fmt.Errorf("unclosed TOML frontmatter (missing closing +++)")
    }
    tomlStr := content[4 : 4+endIdx]
    body := content[4+endIdx+4:] // skip past "\n+++"

    var fm ProfileFrontmatter
    if _, err := toml.Decode(tomlStr, &fm); err != nil {
        return nil, "", fmt.Errorf("parsing frontmatter: %w", err)
    }
    return &fm, strings.TrimSpace(body), nil
}
```

**`toml.MetaData` for validation:** `toml.Decode()` returns `MetaData` which has `.Undecoded()` — the project already uses this pattern in `config/toml.go:43` to warn about unknown config keys. Reuse the same pattern for profile frontmatter to catch typos.

**Confidence:** HIGH — `toml.Decode()` signature verified via `go doc github.com/BurntSushi/toml Decode`.

### 2. CLI Subcommand Routing

**Approach:** Pre-parse `os.Args[1]` dispatch before `pflag.Parse()`, with `pflag.NewFlagSet()` per subcommand.

**Why:** pflag supports `FlagSet` — independent, scoped flag sets (confirmed via `go doc`). The project has exactly 1 subcommand group (`profile`) with 3 actions (`create`, `list`, `edit`). Cobra is overkill and would require restructuring the entire `main.go` into a `cmd/` pattern.

**Integration with existing main.go:**
```go
func main() {
    // Subcommand dispatch BEFORE global pflag.Parse()
    if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
        switch os.Args[1] {
        case "profile":
            runProfileCommand(os.Args[2:])
            return
        }
    }

    // Normal flag parsing for chat mode (existing code, unchanged)
    modelName := pflag.StringP("model", "m", "", "...")
    // ...
}
```

**Per-subcommand FlagSets:**
```go
func runProfileCreate(args []string) {
    fs := pflag.NewFlagSet("profile-create", pflag.ExitOnError)
    model := fs.StringP("model", "m", "", "Model for this profile (provider/model)")
    provider := fs.StringP("provider", "p", "", "Provider override")
    fs.Parse(args)
    name := fs.Arg(0) // positional: profile name
    // ...
}
```

**Why not Cobra:** Cobra would add ~5K LOC of dependency, require restructuring into `cmd/root.go`, `cmd/profile.go`, etc., and change the project's CLI initialization pattern. For 3 subcommands, the hand-rolled approach is 40 lines total and maps 1:1 to the existing codebase.

**Migration path:** If fenec later adds many subcommand groups (`fenec session list`, `fenec tool create`, `fenec provider add`), migrating to Cobra is straightforward since Cobra uses pflag underneath. The hand-rolled dispatch maps directly to Cobra commands.

**Confidence:** HIGH — `pflag.NewFlagSet()` confirmed via `go doc github.com/spf13/pflag FlagSet`.

### 3. XDG Config Path Standardization

**Approach:** Replace `os.UserConfigDir()` with a custom function using `os.UserHomeDir()`.

**Why:** `os.UserConfigDir()` on macOS returns `~/Library/Application Support` (confirmed via `go doc os UserConfigDir`). The project wants `~/.config/fenec` on macOS to match XDG conventions. A 10-line custom function handles this.

**Current code** (`internal/config/config.go:46-52`):
```go
func ConfigDir() (string, error) {
    base, err := os.UserConfigDir()  // macOS: ~/Library/Application Support
    // ...
}
```

**New code:**
```go
func ConfigDir() (string, error) {
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        return filepath.Join(xdg, AppName), nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".config", AppName), nil
}
```

**Test compatibility:** Existing tests in `config_test.go` already use `t.Setenv("XDG_CONFIG_HOME", tmpDir)` — the new implementation respects the same env var, so tests remain green without changes.

**Confidence:** HIGH — `os.UserHomeDir()` confirmed via `go doc`, test patterns verified in source.

### 4. Config Path Migration

**Approach:** One-time atomic directory move on startup, macOS only.

**Why:** Existing macOS users have config/sessions/tools at `~/Library/Application Support/fenec`. After the `ConfigDir()` change, those files would be orphaned.

**Key insight:** `os.Rename()` works atomically when source and destination are on the same filesystem. Both `~/Library/Application Support/` and `~/.config/` are under `$HOME` on macOS, so they're on the same APFS volume. No cross-device fallback needed.

**Implementation (~25 lines):**
```go
func MigrateConfigDir() (bool, error) {
    if runtime.GOOS != "darwin" {
        return false, nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return false, err
    }
    oldPath := filepath.Join(home, "Library", "Application Support", AppName)
    newPath, err := ConfigDir()
    if err != nil {
        return false, err
    }

    if _, err := os.Stat(oldPath); os.IsNotExist(err) {
        return false, nil // Nothing to migrate
    }
    if _, err := os.Stat(newPath); err == nil {
        return false, nil // New path exists, don't overwrite
    }

    if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
        return false, err
    }
    if err := os.Rename(oldPath, newPath); err != nil {
        return false, fmt.Errorf("migrating config from %s to %s: %w", oldPath, newPath, err)
    }
    return true, nil
}
```

**Startup integration in main.go:**
```go
// Before config load
if migrated, err := config.MigrateConfigDir(); err != nil {
    slog.Warn("config migration failed", "error", err)
} else if migrated {
    slog.Info("migrated config to XDG path")
}
```

**Confidence:** HIGH — `os.Rename()` behavior on same filesystem is well-documented stdlib behavior.

### 5. $EDITOR Integration

**Approach:** Go stdlib `os/exec` with editor resolution chain.

**Implementation (~15 lines):**
```go
func OpenInEditor(path string) error {
    editor := os.Getenv("VISUAL")
    if editor == "" {
        editor = os.Getenv("EDITOR")
    }
    if editor == "" {
        editor = "vi"
    }
    cmd := exec.Command(editor, path)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

**Resolution order:** `$VISUAL` → `$EDITOR` → `vi`. This matches `git commit` behavior. `$VISUAL` is for full-screen editors (vim, nano, code --wait); `$EDITOR` is traditionally line-mode. Modern practice: check both, prefer `$VISUAL`.

**Note:** The `fenec profile edit` subcommand exits main.go before starting the REPL, so there's no conflict with readline's terminal mode.

**Confidence:** HIGH — standard Go/Unix pattern, no edge cases for a blocking editor invocation.

### 6. /clear REPL Command

**Approach:** Create fresh `Conversation` and `Session`, reuse existing `baseSystemPrompt`.

**Integration points** (all verified in REPL source):
- `r.conv` — replace with `chat.NewConversation(r.conv.Model, fullPrompt)`
- `r.tracker` — call `Update(0, 0)` to reset token counts
- `r.session` — replace with `session.NewSession(r.conv.Model)`
- `r.baseSystemPrompt` — already stored on REPL struct (line 42)
- `r.refreshSystemPrompt()` — reappends tool descriptions (line 779)

**Implementation (~15 lines in repl.go):**
```go
case "/clear":
    r.handleClearCommand()

func (r *REPL) handleClearCommand() {
    prompt := r.baseSystemPrompt
    if r.registry != nil {
        if desc := r.registry.Describe(); desc != "" {
            prompt += "\n\n## Available Tools\n\n" + desc
        }
    }
    thinkEnabled := r.conv.Think
    r.conv = chat.NewConversation(r.conv.Model, prompt)
    r.conv.Think = thinkEnabled
    if r.tracker != nil {
        r.conv.ContextLength = r.tracker.Available()
        r.tracker.Update(0, 0)
    }
    r.session = session.NewSession(r.conv.Model)
    fmt.Fprintln(r.rl.Stdout(), "Conversation cleared.")
}
```

**Confidence:** HIGH — all APIs confirmed in source code review. Pattern matches existing `refreshSystemPrompt()`.

## What Changes in Existing Code

| File | Change | Risk |
|------|--------|------|
| `internal/config/config.go` | Replace `ConfigDir()` to use XDG paths | LOW — tests use `XDG_CONFIG_HOME`, compatible |
| `internal/config/config.go` | Add `MigrateConfigDir()`, `ProfilesDir()` | LOW — new functions, no existing changes |
| `main.go` | Add subcommand dispatch before `pflag.Parse()` | LOW — additive guard clause, existing flow untouched |
| `main.go` | Add `--system` and `--profile` flags via pflag | LOW — additive flags |
| `main.go` | Call `MigrateConfigDir()` early in startup | LOW — before config load |
| `internal/repl/repl.go` | Add `/clear` case to command switch (line 153) | LOW — single case addition |
| `internal/repl/commands.go` | Add `/clear` to `helpText` constant | LOW — string edit |

## New Internal Package

| Package | Purpose | Size Estimate |
|---------|---------|--------------|
| `internal/profile` | Profile struct, frontmatter parsing, CRUD (create/list/edit), file I/O, `OpenInEditor()` | ~200 LOC |

**Do NOT create** separate packages for migration or editor — migration belongs in `internal/config` (same package as `ConfigDir`), editor invocation belongs in `internal/profile` (only consumer).

## What NOT to Add

| Library | Why Skip |
|---------|----------|
| spf13/cobra | Adds ~5K LOC, requires `cmd/` restructuring, overkill for 3 subcommands. pflag.FlagSet handles scoped parsing. |
| adrg/xdg | 10 lines of stdlib code replaces this. Its macOS behavior may diverge from desired XDG-everywhere semantics. |
| gohugoio/hugo/parser/metadecoders | Massive dependency tree for frontmatter parsing. `toml.Decode()` + string splitting is ~25 lines. |
| gopkg.in/yaml.v3 (for frontmatter) | Project is TOML-standardized. Don't introduce a second config format. |
| mitchellh/go-homedir | Deprecated. `os.UserHomeDir()` in stdlib since Go 1.12. |
| kirsle/configdir | Unmaintained. Custom 10-line function is cleaner. |
| any "frontmatter parser" library | No Go library handles TOML frontmatter well. Hugo's parser is internal. The task is trivially 25 lines. |

## Installation

```bash
# No new dependencies needed for v1.3.
# Existing go.mod is sufficient.
go build ./...
```

## Version Compatibility

| Dependency | Version | Used For (v1.3) | Status |
|------------|---------|-----------------|--------|
| Go | 1.25.8 | os/exec, os.UserHomeDir, runtime.GOOS | ✓ Already installed |
| BurntSushi/toml | v1.6.0 | `toml.Decode()` for frontmatter | ✓ Already in go.mod |
| spf13/pflag | v1.0.10 | `pflag.NewFlagSet()` for subcommands | ✓ Already in go.mod |
| Go stdlib os/exec | — | $EDITOR launch | ✓ Always available |
| Go stdlib runtime | — | `GOOS` check for migration | ✓ Always available |

## Sources

- `go doc github.com/BurntSushi/toml Decode` — confirmed `func Decode(data string, v any) (MetaData, error)` (HIGH confidence)
- `go doc github.com/spf13/pflag FlagSet` — confirmed `NewFlagSet()` with scoped parsing and `Args()` for positional args (HIGH confidence)
- `go doc os UserConfigDir` — confirmed macOS returns `$HOME/Library/Application Support` (HIGH confidence)
- `go doc os UserHomeDir` — confirmed returns `$HOME` on Unix/macOS (HIGH confidence)
- Source review: `internal/config/config.go` lines 46-52 (current ConfigDir), `internal/config/toml.go` line 38 (current toml.DecodeFile usage) (HIGH confidence)
- Source review: `internal/repl/repl.go` lines 42, 153-172, 779-789 (REPL struct fields, command switch, refreshSystemPrompt) (HIGH confidence)
- Source review: `internal/chat/message.go` lines 14-25 (NewConversation API) (HIGH confidence)
- Source review: `internal/config/config_test.go` (XDG_CONFIG_HOME test pattern) (HIGH confidence)
- Source review: `go.mod` (current dependency versions) (HIGH confidence)

---
*Stack research for: Fenec v1.3 Profiles & Config Migration*
*Researched: 2025-07-18*
