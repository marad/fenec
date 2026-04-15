# Phase 18: Profile Flag - Pattern Map

**Mapped:** 2025-07-24
**Files analyzed:** 1 modified file
**Analogs found:** 1 / 1

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `main.go` | controller (CLI entry point) | request-response | `main.go` (self — `--system` flag at lines 33, 176-194; `--model` flag at lines 27, 132-174) | exact |

**Note:** This phase modifies a single file (`main.go`). The analog is the file itself — the existing `--system` and `--model` flag patterns are the exact templates for the new `--profile` flag. No new files are created.

## Pattern Assignments

### `main.go` (controller, request-response) — MODIFY

**Analog:** `main.go` (self — existing flag patterns)

---

#### Pattern 1: Flag Registration (lines 27-33)

**Source:** `main.go` lines 27-33

All flags use `pflag.StringP` / `pflag.BoolP` with consistent pattern: `varName := pflag.TypeP("long", "short", default, "description")`.

```go
modelName := pflag.StringP("model", "m", "", "Model to use (provider/model or just model name)")
pipeMode := pflag.BoolP("pipe", "p", false, "Read all stdin as a single message and send to model")
debugMode := pflag.BoolP("debug", "d", false, "Show tool call results and other debug output")
yoloMode := pflag.BoolP("yolo", "y", false, "Auto-approve all dangerous commands (use with caution)")
lineByLine := pflag.Bool("line-by-line", false, "In pipe mode, send each stdin line separately (default: batch)")
showVersion := pflag.BoolP("version", "v", false, "Print version and exit")
systemFile := pflag.StringP("system", "s", "", "File to use as system prompt for this session")
```

**New flag follows this exact pattern:**
```go
profileName := pflag.StringP("profile", "P", "", "Activate a named profile (loads model + prompt)")
```

---

#### Pattern 2: Usage Text (lines 35-48)

**Source:** `main.go` lines 35-48

Custom usage function with example lines and `pflag.PrintDefaults()`:

```go
pflag.Usage = func() {
    fmt.Fprintf(os.Stderr, `fenec - AI assistant powered by local Ollama models

Usage:
  fenec                    Start interactive chat
  fenec --model gemma4     Use a specific model
  echo "prompt" | fenec    Send piped input to model
  fenec --yolo             Auto-approve all tool commands
  fenec --system prompt.md  Use a custom system prompt

Flags:
`)
    pflag.PrintDefaults()
}
```

**Add profile example line to the usage block** (e.g., `fenec --profile coder    Activate a named profile`).

---

#### Pattern 3: Error Handling — Config/Directory Resolution (lines 70-82)

**Source:** `main.go` lines 70-82

Pattern for resolving config directories and loading config with hard-fail:

```go
configDir, err := config.ConfigDir()
if err != nil {
    fmt.Fprintln(os.Stderr, render.FormatError(
        fmt.Sprintf("Failed to resolve config directory: %v", err)))
    os.Exit(1)
}
configPath := filepath.Join(configDir, "config.toml")
cfg, err := config.LoadOrCreateConfig(configPath)
if err != nil {
    fmt.Fprintln(os.Stderr, render.FormatError(
        fmt.Sprintf("Failed to load config: %v", err)))
    os.Exit(1)
}
```

**Profile loading follows this same `render.FormatError` + `os.Exit(1)` pattern** for both `config.ProfilesDir()` failure and `profile.Load()` failure.

---

#### Pattern 4: Error Handling — System Prompt File Read (lines 178-184)

**Source:** `main.go` lines 178-184

This is the closest error pattern for `--profile` since `--system` also hard-fails when the user explicitly requested a specific file:

```go
if *systemFile != "" {
    data, err := os.ReadFile(*systemFile)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to read system prompt file: %v", err)))
        os.Exit(1)
    }
    systemPrompt = string(data)
}
```

**Profile loading replicates this: user explicitly asked for `--profile X`, hard-fail if not found.**

---

#### Pattern 5: Provider Registry Lookup — `--model` flag (lines 132-148)

**Source:** `main.go` lines 132-148

Pattern for resolving a provider from `providerRegistry.Get()` with error listing available providers:

```go
if *modelName != "" {
    if idx := strings.Index(*modelName, "/"); idx != -1 {
        parts := strings.SplitN(*modelName, "/", 2)
        providerName, modelPart := parts[0], parts[1]
        namedProvider, ok := providerRegistry.Get(providerName)
        if !ok {
            fmt.Fprintf(os.Stderr, "Provider %q not found. Available providers:\n", providerName)
            for _, n := range providerRegistry.Names() {
                fmt.Fprintf(os.Stderr, "  - %s\n", n)
            }
            os.Exit(1)
        }
        p = namedProvider
        activeProviderName = providerName
        *modelName = modelPart
    }
} else if cfg.DefaultModel != "" {
    *modelName = cfg.DefaultModel
}
```

**Profile's provider resolution uses the same `providerRegistry.Get()` + error listing pattern.**

---

#### Pattern 6: System Prompt Resolution — Three-Layer Precedence (lines 176-194)

**Source:** `main.go` lines 176-194

Current two-layer prompt precedence (`--system` flag > config default):

```go
var systemPrompt string
if *systemFile != "" {
    data, err := os.ReadFile(*systemFile)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to read system prompt file: %v", err)))
        os.Exit(1)
    }
    systemPrompt = string(data)
} else {
    var err error
    systemPrompt, err = config.LoadSystemPrompt()
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to load system prompt: %v", err)))
        os.Exit(1)
    }
}
```

**Profile adds a middle layer: `--system` > `prof.SystemPrompt` > `config.LoadSystemPrompt()`. The `else` branch becomes `else if prof != nil && prof.SystemPrompt != ""` with a final `else` for the config default.**

---

#### Pattern 7: Import Block (lines 1-23)

**Source:** `main.go` lines 1-23

Current imports — profile package needs to be added:

```go
import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "time"

    pflag "github.com/spf13/pflag"
    "golang.org/x/term"

    "github.com/marad/fenec/internal/chat"
    "github.com/marad/fenec/internal/config"
    feneclua "github.com/marad/fenec/internal/lua"
    "github.com/marad/fenec/internal/provider"
    "github.com/marad/fenec/internal/render"
    "github.com/marad/fenec/internal/repl"
    "github.com/marad/fenec/internal/session"
    "github.com/marad/fenec/internal/tool"
)
```

**Add `"github.com/marad/fenec/internal/profile"` to the internal imports block.**

---

#### Pattern 8: pflag.Changed() for Detecting Explicit Flag Usage

**Source:** `github.com/spf13/pflag` API (already in go.mod v1.0.10)

```go
// Detect if --model was explicitly passed (vs. set by profile)
modelExplicit := pflag.CommandLine.Changed("model")
```

**Critical for D-01:** Prevents confusion when profile sets `*modelName` and the existing `--model` block sees it as non-empty. Use `Changed("model")` to guard the `--model` override block so it only fires when the user explicitly passed `--model`.

---

## Shared Patterns

### Error Handling — Hard Fail with FormatError
**Source:** `internal/render/style.go` lines 57-59, `main.go` throughout
**Apply to:** All new error paths in profile loading

```go
// render.FormatError pattern — used consistently in main.go
fmt.Fprintln(os.Stderr, render.FormatError(
    fmt.Sprintf("Human-readable message: %v", err)))
os.Exit(1)
```

Every user-facing error in `main.go` follows this exact pattern: `render.FormatError()` wrapping a `fmt.Sprintf`, printed to `os.Stderr`, followed by `os.Exit(1)`.

### Provider Registry Lookup
**Source:** `internal/config/registry.go` lines 41-46
**Apply to:** Profile provider resolution

```go
// Thread-safe lookup — returns (provider, bool)
func (r *ProviderRegistry) Get(name string) (provider.Provider, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    p, ok := r.providers[name]
    return p, ok
}
```

### Profile Package API
**Source:** `internal/profile/profile.go` lines 96-114
**Apply to:** Profile loading in main.go

```go
// Load reads a profile by name from the given directory.
// Returns error for invalid names (path traversal) or missing/unparseable files.
func Load(dir, name string) (*Profile, error)

// Profile struct fields used in main.go:
type Profile struct {
    Name         string      // "coder"
    Provider     string      // "ollama" or "" (empty if bare model)
    ModelName    string      // "gemma4"
    SystemPrompt string      // markdown body or "" (empty for model-only profiles)
}
```

### Config ProfilesDir
**Source:** `internal/config/config.go` lines 167-173
**Apply to:** Resolving the profiles directory before calling `profile.Load()`

```go
// ProfilesDir returns the path to the profiles directory.
// Located at {ConfigDir}/profiles/.
// Does NOT create the directory.
func ProfilesDir() (string, error) {
    dir, err := ConfigDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, "profiles"), nil
}
```

## Insertion Points in main.go

The following describes WHERE new code goes relative to existing code, which is critical because this phase modifies a single large function:

| Code Block | Insert After | Insert Before | Lines |
|------------|-------------|---------------|-------|
| Flag registration (`profileName := pflag.StringP(...)`) | `systemFile` flag (line 33) | `pflag.Usage` (line 35) | ~34 |
| Usage text update (add profile example) | Inside `pflag.Usage` func | Between existing examples | ~44 |
| Profile loading (`profile.Load()`) | Config loading + provider registry setup (line 94) | Provider health check (line 124) | ~95 |
| Profile model/provider application | Profile loading block | Existing `--model` block (line 132) | after profile load |
| Model resolution guard (`Changed("model")`) | Replaces current `if *modelName != ""` (line 133) | Same block | line 133 |
| Prompt resolution (3-layer) | Replaces current prompt block (lines 176-194) | Context window query (line 196) | lines 176-194 |

## No Analog Found

No files without analogs — all patterns for this phase exist in the codebase.

## Metadata

**Analog search scope:** `main.go`, `internal/profile/`, `internal/config/`, `internal/render/`
**Files scanned:** 7 (main.go, profile.go, profile_test.go, config.go, registry.go, style.go, 18-RESEARCH.md)
**Pattern extraction date:** 2025-07-24
