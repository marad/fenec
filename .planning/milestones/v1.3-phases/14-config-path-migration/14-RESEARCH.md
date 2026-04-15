# Phase 14: Config Path Migration - Research

**Researched:** 2025-07-18
**Domain:** Go filesystem operations, config directory conventions, data migration
**Confidence:** HIGH

## Summary

Phase 14 changes fenec's config directory from the macOS-native `~/Library/Application Support/fenec` to `~/.config/fenec` on all platforms, with automatic migration for existing macOS users. The scope is narrow and well-contained: one function (`ConfigDir()`) in `internal/config/config.go` is the single source of truth for the config path, and all other code derives paths through it. No external libraries are needed — Go's `os` standard library provides everything required.

The key insight is that Go's `os.UserConfigDir()` returns `~/Library/Application Support` on macOS by design (per Apple conventions), so we must stop using it and instead hardcode `~/.config` relative to `os.UserHomeDir()`. The migration is a one-time `os.Rename()` of the entire directory, which is atomic on macOS since both paths reside on the same APFS volume. The legacy path detection is macOS-only (gated on `runtime.GOOS == "darwin"`).

**Primary recommendation:** Replace `os.UserConfigDir()` with `os.UserHomeDir()` + `/.config/fenec`, add a `MigrateIfNeeded()` function that atomically renames the legacy directory, and call it in `main.go` before any config loading.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CFG-01 | Config directory is `~/.config/fenec` on all platforms | Change `ConfigDir()` to use `os.UserHomeDir() + "/.config/fenec"` instead of `os.UserConfigDir() + "/fenec"` |
| CFG-02 | Existing data auto-migrates from `~/Library/Application Support/fenec` to `~/.config/fenec` on macOS first run | Add `MigrateIfNeeded()` that detects legacy path on darwin and does `os.Rename()` |
| CFG-03 | User sees migration feedback message on stderr after successful migration | `fmt.Fprintln(os.Stderr, ...)` with clear message in `MigrateIfNeeded()` |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os` (stdlib) | Go 1.25 | `UserHomeDir()`, `Rename()`, `Stat()`, `MkdirAll()` | All operations needed are in stdlib. No external dependencies. |
| `runtime` (stdlib) | Go 1.25 | `runtime.GOOS` for platform detection | Standard way to gate macOS-specific migration logic. |
| `path/filepath` (stdlib) | Go 1.25 | Path joining | Already used throughout codebase. |
| `fmt` (stdlib) | Go 1.25 | Stderr migration message | Already used in `main.go` for all user-facing output. |

[VERIFIED: Go stdlib docs — these are all standard library packages]

### Supporting
No additional libraries needed. This phase is purely stdlib.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.UserHomeDir()` + hardcoded `.config` | Continue using `os.UserConfigDir()` | Would keep macOS at `~/Library/Application Support` — contradicts CFG-01 |
| `os.Rename()` for migration | Copy + delete | Rename is atomic and instant; copy risks partial state on crash. Both paths are same APFS volume so rename works. |
| Hardcoded `~/.config/fenec` | Respect `XDG_CONFIG_HOME` | XDG support is explicitly in Future Requirements (CFGX-01/CFGX-02), not v1.3 scope. Hardcoding now is correct. |

## Architecture Patterns

### Current Architecture (before changes)

```
internal/config/config.go
├── ConfigDir()           ← SINGLE function that determines base path (uses os.UserConfigDir())
├── LoadSystemPrompt()    ← calls ConfigDir()
├── SessionDir()          ← calls ConfigDir()
├── ToolsDir()            ← calls ConfigDir()
└── HistoryFile()         ← calls ConfigDir()

main.go
├── config.ConfigDir()       ← line 64, gets configDir for config.toml path
├── config.LoadOrCreateConfig() ← line 71, loads/creates config.toml
├── config.LoadSystemPrompt()   ← line 171
├── config.SessionDir()          ← line 193
└── config.ToolsDir()            ← line 240
```

[VERIFIED: grep of codebase — see research above]

### Recommended Change Architecture

```
internal/config/config.go (modified)
├── ConfigDir()           ← Change to: os.UserHomeDir() + "/.config/fenec"
├── legacyConfigDir()     ← NEW: returns ~/Library/Application Support/fenec (darwin only)
├── MigrateIfNeeded()     ← NEW: detects legacy path, renames, prints stderr message
├── LoadSystemPrompt()    ← unchanged (derives from ConfigDir)
├── SessionDir()          ← unchanged (derives from ConfigDir)
├── ToolsDir()            ← unchanged (derives from ConfigDir)
└── HistoryFile()         ← unchanged (derives from ConfigDir)

main.go (modified)
├── config.MigrateIfNeeded()  ← NEW: called FIRST, before ConfigDir() 
├── config.ConfigDir()        ← existing call, now returns new path
└── ... (rest unchanged)
```

### Pattern: Single Point of Change
The entire codebase resolves config paths through `ConfigDir()`. Changing this ONE function changes the path everywhere. No shotgun surgery needed. [VERIFIED: codebase grep confirmed all path derivation flows through ConfigDir()]

### Pattern: Migration Before Load
`MigrateIfNeeded()` must be called BEFORE any `ConfigDir()` usage to ensure the directory exists at the new path before code tries to read from it.

```go
// In main.go, before any config loading:
config.MigrateIfNeeded() // Move legacy data if present (macOS only)

configDir, err := config.ConfigDir() // Now returns ~/.config/fenec
```

### Pattern: Platform-Gated Migration
The legacy path only exists on macOS. Use build tags or runtime check:

```go
// Recommended: runtime check (simpler, no build tags needed)
func MigrateIfNeeded() {
    if runtime.GOOS != "darwin" {
        return // Only macOS had the legacy path
    }
    // ... migration logic
}
```

Build tags (`//go:build darwin`) are an alternative but add file complexity for a single function. Runtime check is simpler and more debuggable. [ASSUMED — either approach works, runtime check recommended for simplicity]

### Anti-Patterns to Avoid
- **Don't use `os.UserConfigDir()` anywhere:** It returns different paths per platform. The whole point is to standardize on `~/.config/fenec`. [VERIFIED: Go docs confirm Darwin returns `~/Library/Application Support`]
- **Don't copy files instead of renaming:** `os.Rename()` is atomic on same volume. Copying creates a window where data exists in both places, risks partial copy on crash. [VERIFIED: both paths are on same APFS volume `/dev/disk3s5`]
- **Don't leave the legacy directory behind:** After a successful `os.Rename()`, the source directory is gone (that's how rename works). No cleanup needed.
- **Don't migrate if new path already exists:** If `~/.config/fenec` already has content, the user may have manually set things up. Don't overwrite.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic directory move | Recursive copy + delete | `os.Rename()` | Same filesystem = instant atomic rename. Copy risks corruption. |
| Platform detection | `#ifdef`-style preprocessor | `runtime.GOOS == "darwin"` | Go standard idiom. |
| Home directory | Parsing `$HOME` env var | `os.UserHomeDir()` | Handles edge cases (Windows, Plan 9, missing env). |

## Common Pitfalls

### Pitfall 1: Migration Race with ConfigDir
**What goes wrong:** If `ConfigDir()` is called before `MigrateIfNeeded()`, it returns `~/.config/fenec` which doesn't exist yet. `LoadOrCreateConfig` then creates a fresh default config at the new path, and migration is skipped because the new path already exists.
**Why it happens:** Startup order matters.
**How to avoid:** Call `MigrateIfNeeded()` as the VERY FIRST thing in `main()`, before any `config.ConfigDir()` or `config.LoadOrCreateConfig()` calls.
**Warning signs:** Fresh default config after migration instead of user's customized config.

### Pitfall 2: New Path Already Exists
**What goes wrong:** `os.Rename()` fails if the destination already exists as a non-empty directory.
**Why it happens:** User manually created `~/.config/fenec` or ran a fresh install before first migration.
**How to avoid:** Check if new path exists BEFORE attempting rename. If it exists, skip migration silently (or with a debug log).
**Warning signs:** Error messages about "directory not empty" on macOS.

### Pitfall 3: Test Isolation on macOS
**What goes wrong:** Tests that use `os.UserConfigDir()` or `XDG_CONFIG_HOME` don't work properly on macOS because Go ignores `XDG_CONFIG_HOME` on darwin.
**Why it happens:** Go's `os.UserConfigDir()` on darwin always returns `~/Library/Application Support`, regardless of env vars. The existing `TestLoadSystemPromptFromFile` is already failing for this exact reason.
**How to avoid:** After this phase, `ConfigDir()` uses `os.UserHomeDir()` + `.config/fenec`. Tests can now use `t.Setenv("HOME", tmpDir)` on macOS to redirect the path. Alternatively, make `ConfigDir()` accept an override (but that changes the API).
**Warning signs:** Tests passing on Linux CI but failing on macOS dev machines.

### Pitfall 4: Parent Directory Creation
**What goes wrong:** `os.Rename()` succeeds but `~/.config/` directory doesn't exist on a fresh macOS system.
**Why it happens:** `~/.config/` is not a standard macOS directory; it may not exist.
**How to avoid:** Create `~/.config/` with `os.MkdirAll()` before attempting the rename.
**Warning signs:** "no such file or directory" error on rename.

### Pitfall 5: Permissions on ~/.config
**What goes wrong:** `~/.config` created with wrong permissions.
**Why it happens:** `os.MkdirAll` uses the provided mode, but umask applies.
**How to avoid:** Use `0755` mode (standard for config dirs). This is what every other tool does.
**Warning signs:** Other applications complaining about `~/.config` permissions.

## Code Examples

### ConfigDir — New Implementation

```go
// Source: Go stdlib os.UserHomeDir() docs
// ConfigDir returns the fenec configuration directory path.
// Always returns ~/.config/fenec on all platforms.
func ConfigDir() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".config", "fenec"), nil
}
```

### Legacy Path Detection (macOS only)

```go
// legacyConfigDir returns the old macOS config path, or empty string on non-darwin.
func legacyConfigDir() string {
    if runtime.GOOS != "darwin" {
        return ""
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, "Library", "Application Support", "fenec")
}
```

### MigrateIfNeeded — Full Implementation Pattern

```go
// MigrateIfNeeded migrates config data from the legacy macOS path
// (~/Library/Application Support/fenec) to the new path (~/.config/fenec).
// It is a no-op on non-macOS platforms or if no legacy data exists.
// Prints a confirmation message to stderr on successful migration.
func MigrateIfNeeded() {
    legacy := legacyConfigDir()
    if legacy == "" {
        return // Not macOS, nothing to migrate
    }

    // Check if legacy directory exists
    if _, err := os.Stat(legacy); os.IsNotExist(err) {
        return // No legacy data, nothing to migrate
    }

    // Determine new path
    newDir, err := ConfigDir()
    if err != nil {
        return // Can't determine new path, skip silently
    }

    // Don't overwrite if new path already exists
    if _, err := os.Stat(newDir); err == nil {
        return // New path exists, don't clobber
    }

    // Ensure parent (~/.config/) exists
    if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
        fmt.Fprintf(os.Stderr, "fenec: failed to create config directory: %v\n", err)
        return
    }

    // Atomic rename
    if err := os.Rename(legacy, newDir); err != nil {
        fmt.Fprintf(os.Stderr, "fenec: failed to migrate config: %v\n", err)
        return
    }

    // CFG-03: User feedback on stderr
    fmt.Fprintf(os.Stderr, "fenec: migrated config from %s to %s\n", legacy, newDir)
}
```

### main.go Integration

```go
func main() {
    // Migrate legacy config path before anything else (CFG-02)
    config.MigrateIfNeeded()

    // ... existing flag parsing ...

    // Load or create config file (existing code, now uses new path)
    configDir, err := config.ConfigDir()
    // ... rest unchanged ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `os.UserConfigDir()` (platform-native) | `os.UserHomeDir()` + `/.config/` (XDG-style everywhere) | Common in CLI tools since ~2020 | Many Go CLI tools (gh, lazygit, etc.) use `~/.config` on macOS despite Apple conventions |

**Context:** Apple's convention (`~/Library/Application Support/`) is intended for GUI apps with Info.plist. For CLI tools, `~/.config/` is the de facto standard because:
- It's visible in terminals (no hidden `Library` folder navigation needed)
- It's consistent with Linux
- Many developer tools already use it (gh, lazygit, starship, alacritty, etc.)
[ASSUMED — based on common CLI tool conventions]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Runtime check (`runtime.GOOS`) is preferred over build tags for this case | Architecture Patterns | LOW — build tags work too, just different file organization |
| A2 | Many CLI tools use `~/.config` on macOS | State of the Art | LOW — factual but not verified with specific source this session |

## Open Questions

1. **Should migration errors be fatal or silent?**
   - What we know: The code examples above use non-fatal (log + continue). This means a fresh config is created if migration fails.
   - What's unclear: Whether the user would prefer fenec to refuse to start if migration fails (preserving their data awareness).
   - Recommendation: Non-fatal is safer. User still has data at legacy path. A hard failure would block usage entirely.

2. **Should we leave a symlink at the legacy path?**
   - What we know: After `os.Rename()`, the legacy path is gone.
   - What's unclear: Whether any external tools/scripts reference the legacy path.
   - Recommendation: No symlink. This is a personal tool, not a system service. Keep it simple. If someone needs the old path, they can symlink manually.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed (go test built-in) |
| Quick run command | `go test ./internal/config/ -count=1 -run TestMigrat` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CFG-01 | ConfigDir returns `~/.config/fenec` on all platforms | unit | `go test ./internal/config/ -run TestConfigDir -count=1` | ✅ (needs update) |
| CFG-02 | MigrateIfNeeded moves legacy dir to new path | unit | `go test ./internal/config/ -run TestMigrate -count=1` | ❌ Wave 0 |
| CFG-02 | MigrateIfNeeded is no-op when no legacy data | unit | `go test ./internal/config/ -run TestMigrateNoLegacy -count=1` | ❌ Wave 0 |
| CFG-02 | MigrateIfNeeded skips when new path exists | unit | `go test ./internal/config/ -run TestMigrateNewPathExists -count=1` | ❌ Wave 0 |
| CFG-03 | Migration prints stderr feedback message | unit | `go test ./internal/config/ -run TestMigrateFeedback -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/ -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/config/config_test.go` — update `TestConfigDir` to verify `~/.config/fenec` pattern
- [ ] `internal/config/config_test.go` — add `TestMigrateIfNeeded*` tests (5 scenarios)
- [ ] `internal/config/config_test.go` — fix existing `TestLoadSystemPromptFromFile` which already fails on macOS

**Note:** The existing `TestLoadSystemPromptFromFile` is failing because it sets `XDG_CONFIG_HOME` which Go ignores on macOS. After this phase, tests will work by overriding `HOME` env var instead, since `ConfigDir()` will use `os.UserHomeDir()`.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | No user input in this phase — paths are derived from OS |
| V6 Cryptography | no | — |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Symlink attack on migration | Tampering | `os.Rename()` operates on the directory itself, not contents. Low risk for single-user tool in home directory. |
| Permission escalation via config dir | Elevation | Use `0755` for directories, `0644` for files (already established pattern in codebase) |

**Note:** This is a personal CLI tool running in userspace. The threat surface is minimal — all paths are under `$HOME` and operated by the same user.

## Sources

### Primary (HIGH confidence)
- Go stdlib `os.UserConfigDir()` docs — confirmed returns `~/Library/Application Support` on Darwin, ignores XDG_CONFIG_HOME [VERIFIED: `go doc os.UserConfigDir` and runtime test]
- Go stdlib `os.UserHomeDir()` docs — returns `$HOME` on Unix/Darwin [VERIFIED: Go docs]
- Go stdlib `os.Rename()` docs — atomic rename on same filesystem [VERIFIED: Go docs]
- Codebase grep — `ConfigDir()` at line 46 of `internal/config/config.go` is the sole source of config path, all other functions derive from it [VERIFIED: grep]
- Filesystem check — legacy path `~/Library/Application Support/fenec` and new path `~/.config/fenec` are on same APFS volume (`/dev/disk3s5`) [VERIFIED: `df` command]
- Existing config structure — 3 files: `config.toml`, `history`, `sessions/_autosave.json` [VERIFIED: `find` command]

### Secondary (MEDIUM confidence)
- None needed — this phase is entirely stdlib operations

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, no library decisions
- Architecture: HIGH — codebase fully inspected, single point of change confirmed
- Pitfalls: HIGH — tested actual behavior on the development machine, verified failing test
- Migration: HIGH — same-volume rename verified, legacy path contents inspected

**Research date:** 2025-07-18
**Valid until:** indefinite (stdlib behavior is stable)
