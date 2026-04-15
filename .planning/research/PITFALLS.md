# Domain Pitfalls: Profiles, Config Migration & CLI Subcommands

**Domain:** Adding profiles, config path migration, CLI subcommands, and /clear to an existing Go CLI assistant
**Project:** Fenec v1.3
**Researched:** 2025-07-14
**Confidence:** HIGH (verified against direct codebase audit of all affected files, Go stdlib documentation)

## Critical Pitfalls

Mistakes that cause data loss, state corruption, or require rewrites.

### Pitfall 1: `/clear` Breaks `sync.Once` AutoSave — New Conversations Never Saved

**What goes wrong:** The REPL's `autoSaved sync.Once` field ensures `autoSave()` runs exactly once (called from both `defer` in `Run()` and `Close()`). After `/clear` resets the conversation, `sync.Once` cannot be reset — it has already fired or will fire once. The post-clear conversation will NEVER be auto-saved.

**Why it happens:** `sync.Once` is designed to be single-use. There is no `Reset()` method. The current design assumes one session per REPL lifetime, but `/clear` introduces a second logical session within the same REPL instance.

**Consequences:** User has a productive conversation after `/clear`, exits Fenec, and their work is silently lost. No auto-save file is written for the post-clear conversation.

**Prevention:** Replace `sync.Once` with a simple `bool` + `sync.Mutex` guard that `/clear` can reset:

```go
// In REPL struct, replace:
//   autoSaved sync.Once
// With:
autoSaveDone bool
autoSaveMu   sync.Mutex

func (r *REPL) autoSave() {
    r.autoSaveMu.Lock()
    defer r.autoSaveMu.Unlock()
    if r.autoSaveDone {
        return
    }
    r.autoSaveDone = true
    // ... existing save logic
}

func (r *REPL) resetAutoSave() {
    r.autoSaveMu.Lock()
    r.autoSaveDone = false
    r.autoSaveMu.Unlock()
}
```

**Detection:** Test: send messages, `/clear`, send more messages, exit — verify auto-save file contains post-clear content.

---

### Pitfall 2: `/clear` Auto-Saves Empty Session Over Valuable Previous Session

**What goes wrong:** If `/clear` triggers auto-save before resetting, or if the user exits immediately after `/clear`, the auto-save file (`_autosave.json`) is overwritten with an empty conversation. The previous session's auto-save is lost permanently. There's no versioning or backup.

**Why it happens:** `AutoSave()` always writes to `_autosave.json` unconditionally. The `HasContent()` check prevents saving system-prompt-only sessions, but the ordering of save-then-clear vs clear-then-save determines whether data is lost.

**Consequences:** Valuable conversation data from before `/clear` is permanently destroyed.

**Prevention:** `/clear` must follow this exact sequence:
1. Persist the current session to a named file FIRST (if it has content) via `store.Save(r.session)`
2. Create a new session with `session.NewSession(model)` — fresh ID, fresh timestamps
3. Reset the conversation to system prompt only
4. Reset the auto-save guard

```go
func (r *REPL) handleClearCommand() {
    // Step 1: Persist current session if it has content
    if len(r.conv.Messages) > 1 {
        r.session.Messages = r.conv.Messages
        r.session.UpdatedAt = time.Now()
        _ = r.store.Save(r.session) // Best-effort save before clear
    }
    
    // Step 2: Create fresh session
    r.session = session.NewSession(r.conv.Model)
    
    // Step 3: Reset conversation — preserve system prompt
    if len(r.conv.Messages) > 0 && r.conv.Messages[0].Role == "system" {
        r.conv.Messages = r.conv.Messages[:1]
    } else {
        r.conv.Messages = nil
    }
    
    // Step 4: Reset tracker and auto-save guard
    r.resetAutoSave()
}
```

**Detection:** Test: send several messages, `/clear`, exit immediately — verify the pre-clear conversation exists as a named session file in sessions/.

---

### Pitfall 3: Profile System Prompt Clobbers Tool Descriptions — Model Loses Tool Access

**What goes wrong:** When activating a profile, the `baseSystemPrompt` is replaced with the profile's markdown body. But tool descriptions are only present in the conversation because `refreshSystemPrompt()` appends them by combining `r.baseSystemPrompt` + `r.registry.Describe()`. If profile activation updates `baseSystemPrompt` but forgets to call `refreshSystemPrompt()`, or if it directly sets `conv.Messages[0].Content` to just the profile body, tool descriptions vanish. The model no longer knows tools exist.

**Why it happens:** The codebase has a two-layer system: `baseSystemPrompt` (human-authored text) + tool descriptions (auto-appended by `refreshSystemPrompt()`). This layering is implicit — nothing in the type system enforces it. Profile activation is a new code path that must respect both layers but has no compile-time reminder to do so.

**Consequences:** The model stops using tools entirely — shell_exec, file tools, Lua tools all become invisible. The agent degrades to a plain chatbot. This is silent — no error is shown.

**Prevention:** Profile activation MUST update `baseSystemPrompt` then call `refreshSystemPrompt()`. Never set `conv.Messages[0].Content` directly:

```go
func (r *REPL) activateProfile(profile Profile) {
    r.baseSystemPrompt = profile.SystemPrompt  // Update base
    r.refreshSystemPrompt()                     // Re-appends tool descriptions
}
```

Consider adding a comment guard to `conv.Messages[0]` access: any direct write to it outside `refreshSystemPrompt()` is a bug.

**Detection:** Test: activate a profile, inspect `conv.Messages[0].Content` — verify it contains `## Available Tools` section with tool listings.

---

### Pitfall 4: Config Path Migration Race — fsnotify Watches the Wrong Directory

**What goes wrong:** Migration moves files from `~/Library/Application Support/fenec/` to `~/.config/fenec/` on macOS. But `ConfigWatcher` was started with the OLD `configPath`. After migration, config changes in the new directory are invisible to the watcher. Hot-reload silently stops working.

**Why it happens:** In `main.go`, the startup sequence is: `ConfigDir()` → `configPath` → `LoadOrCreateConfig(configPath)` → `NewConfigWatcher(configPath, ...)`. If migration runs AFTER `ConfigDir()` returns but BEFORE the watcher starts, or if `ConfigDir()` returns the old path because migration hasn't happened yet, the watcher watches the wrong directory.

**Consequences:** Hot-reload silently stops working after migration. User edits config in `~/.config/fenec/config.toml`, nothing happens. No error is visible.

**Prevention:** Migration must happen INSIDE `ConfigDir()` before it returns — so the returned path is always the correct, post-migration path:

```go
func ConfigDir() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    
    newDir := filepath.Join(home, ".config", AppName)
    if dirExists(newDir) {
        return newDir, nil
    }
    
    // macOS migration: check old location
    if runtime.GOOS == "darwin" {
        oldDir := filepath.Join(home, "Library", "Application Support", AppName)
        if dirExists(oldDir) {
            if err := os.MkdirAll(filepath.Dir(newDir), 0755); err == nil {
                if err := os.Rename(oldDir, newDir); err == nil {
                    slog.Info("migrated config", "from", oldDir, "to", newDir)
                    return newDir, nil
                }
            }
            // Migration failed — fall back to old dir
            return oldDir, nil
        }
    }
    
    return newDir, nil
}
```

**Detection:** Test on macOS: start with old path populated, verify `ConfigDir()` returns new path AND watcher fires on new-path config edits.

---

### Pitfall 5: pflag.Parse() Consumes Subcommand Arguments — `fenec profile create` Fails

**What goes wrong:** The current `main.go` calls `pflag.Parse()` which parses ALL arguments. When the user runs `fenec profile create --name coder`, pflag sees `profile` and `create` as unknown positional args and `--name` as an unknown flag, causing either an error exit or silent swallowing of the subcommand.

**Why it happens:** pflag is a flag parser, not a command router. It was the right choice when fenec had only flags (`--model`, `--pipe`, `--debug`). Adding subcommands requires pre-pflag routing.

**Consequences:** `fenec profile create` exits with "unknown flag" error or silently ignores the subcommand and starts the REPL.

**Prevention:** Inspect `os.Args` for subcommands BEFORE `pflag.Parse()`:

```go
func main() {
    // Route subcommands before flag parsing
    if len(os.Args) > 1 && os.Args[1] == "profile" {
        handleProfileSubcommand(os.Args[2:])
        return
    }
    
    // Existing pflag parsing for the REPL flow
    modelName := pflag.StringP("model", "m", "", "...")
    // ... rest of existing code
    pflag.Parse()
}
```

Do NOT introduce cobra for this. Fenec has 6 flags and one subcommand group (`profile`). Cobra adds ~5000 LOC of dependency. A 20-line manual router is the right tool.

**Detection:** Test: `fenec profile list` returns profile list without REPL startup or flag errors.

---

### Pitfall 6: Config Migration Moves config.toml but Forgets Sessions, Tools, History

**What goes wrong:** Migration copies `config.toml` from old to new path but leaves `sessions/`, `tools/`, `system.md`, and `history` file behind. After migration, fenec finds no sessions, no Lua tools, no command history.

**Why it happens:** Developer thinks "config migration" means moving the config file. But `ConfigDir()` is the root for ALL app data — sessions, tools, history, system prompt are all under it. Everything must move.

**Consequences:** User loses all saved sessions, custom Lua tools, and readline history. Data still exists at old path but is invisible to fenec.

**Prevention:** Move the entire directory tree, not individual files:

```go
// os.Rename moves the entire directory atomically on same filesystem
// On macOS, ~/Library/Application Support and ~/.config are on the same FS
if err := os.Rename(oldDir, newDir); err != nil {
    // Cross-device link: fall back to recursive copy
    return copyDirRecursive(oldDir, newDir)
}
```

After successful migration, optionally leave a symlink at the old location for external tools:
```go
os.Symlink(newDir, oldDir) // Best-effort, ignore error
```

**Detection:** Test: populate old dir with sessions/, tools/, system.md, history. Run migration. Verify ALL files and directories present in new location.

---

## Moderate Pitfalls

### Pitfall 7: Profile Model/Provider Override Doesn't Update Context Length or Tracker

**What goes wrong:** A profile specifies `provider = "copilot"` and `model = "gpt-4o"`. Profile activation switches the provider and model, but doesn't: ping the new provider, query `GetContextLength`, update the `ContextTracker`, or update the REPL prompt string. The tracker still has the old model's token limit (e.g., 8192 for an Ollama model). With GPT-4o's 128K context, truncation thresholds are wildly wrong.

**Why it happens:** The existing `/model` command in `handleModelCommand()` already handles context length updates (lines 602-619). But profile activation is a new code path that might not follow the same pattern. Code duplication risk.

**Prevention:** Extract the model-switching logic from `handleModelCommand()` into a shared method, call from both:

```go
func (r *REPL) switchModel(providerName, modelName string) error {
    p, ok := r.providerRegistry.Get(providerName)
    if !ok {
        return fmt.Errorf("provider %q not found", providerName)
    }
    r.provider = p
    r.activeProvider = providerName
    r.conv.SetModel(modelName)
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if ctxLen, err := p.GetContextLength(ctx, modelName); err == nil && ctxLen > 0 {
        r.conv.ContextLength = ctxLen
        if r.tracker != nil {
            r.tracker = chat.NewContextTracker(ctxLen, r.tracker.Threshold())
        }
    }
    
    r.rl.SetPrompt(render.FormatPrompt(modelName))
    return nil
}
```

**Detection:** Test: activate profile with a different model, verify `tracker.Available()` returns the new model's context length.

---

### Pitfall 8: TOML Frontmatter Parsing — No Standard Go Library, Custom Parser Bugs

**What goes wrong:** Go has YAML frontmatter libraries but no widely-adopted TOML frontmatter parser. A custom parser fails on: files starting with UTF-8 BOM, `+++` delimiters with trailing whitespace, Windows `\r\n` line endings, empty frontmatter (`+++\n+++\n`), frontmatter-only files (no body), or `+++` appearing in the markdown body after the closing delimiter.

**Why it happens:** TOML frontmatter uses `+++` delimiters (Hugo convention), but there's no standalone Go package for just "parse TOML between `+++` and return the rest as body." You must write ~40 lines of careful string parsing.

**Prevention:** Write a focused parser with explicit edge case handling and table-driven tests:

```go
func ParseProfile(data []byte) (tomlData string, body string, err error) {
    content := string(data)
    content = strings.TrimPrefix(content, "\xef\xbb\xbf") // Strip BOM
    content = strings.ReplaceAll(content, "\r\n", "\n")     // Normalize CRLF
    
    if !strings.HasPrefix(strings.TrimSpace(content), "+++") {
        return "", strings.TrimSpace(content), nil // No frontmatter
    }
    
    // Skip opening +++ and any trailing whitespace on that line
    idx := strings.Index(content, "+++")
    rest := content[idx+3:]
    if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
        rest = rest[nl+1:]
    }
    
    // Find closing +++
    closeIdx := strings.Index(rest, "\n+++")
    if closeIdx == -1 {
        return "", "", fmt.Errorf("unclosed TOML frontmatter")
    }
    
    tomlPart := rest[:closeIdx]
    bodyPart := strings.TrimSpace(rest[closeIdx+4:])
    
    return tomlPart, bodyPart, nil
}
```

**Required test cases:** (1) no frontmatter, (2) empty frontmatter, (3) frontmatter only no body, (4) BOM prefix, (5) Windows CRLF, (6) TOML syntax error, (7) `+++` in markdown body after closing delimiter, (8) trailing whitespace on `+++` lines, (9) normal happy path.

**Detection:** Table-driven test suite covering all 9 cases above.

---

### Pitfall 9: `--system` and `--profile` Flag Precedence Ambiguity

**What goes wrong:** User runs `fenec --profile coder --system custom.md`. Both provide a system prompt. Without clear precedence, the behavior depends on flag parse order, creating confusion.

**Why it happens:** Two new flags affecting the same thing (system prompt) with no documented interaction.

**Prevention:** Define and document a strict precedence chain:

```
1. --system <file>     (highest — explicit per-invocation override)
2. --profile <name>    (profile's markdown body)
3. ~/.config/fenec/system.md  (user's global default)
4. defaultSystemPrompt const  (built-in fallback)
```

`--system` with `--profile` uses the file's system prompt but keeps the profile's model/provider. Error on `--system <file>` when the file doesn't exist — don't silently fall through.

**Detection:** Test all 4 combinations: neither flag, --system only, --profile only, both flags together.

---

### Pitfall 10: ContextTracker Not Reset on `/clear` — Ghost Token Counts Trigger Truncation

**What goes wrong:** After `/clear`, `conv.Messages` has only the system prompt, but `ContextTracker.lastPromptEval` and `lastEval` still hold token counts from the pre-clear conversation. `ShouldTruncate()` returns `true` immediately. The first post-clear message pair gets truncated from a brand-new conversation.

**Why it happens:** `ContextTracker` has no `Reset()` method. Its counts are corrected by the next `StreamChat` response, but `ShouldTruncate` is checked AFTER the response — the first post-clear response may trigger truncation of the system prompt's companion user message.

**Prevention:** Add `Reset()` to ContextTracker and call it from `/clear`:

```go
func (ct *ContextTracker) Reset() {
    ct.lastPromptEval = 0
    ct.lastEval = 0
}
```

**Detection:** Test: fill context to 80%, `/clear`, send one message — verify no truncation warning appears.

---

### Pitfall 11: Hot-Reload Removes Provider Referenced by Active Profile

**What goes wrong:** User activates profile using `provider = "openai"`. They edit `config.toml` and rename the provider to `gpt`. Hot-reload fires, `providerRegistry.Update()` replaces all providers. The REPL's `r.provider` still holds the OLD provider object (Go GC keeps it alive, so it works), but `r.activeProvider` is `"openai"` which no longer exists in the registry. `/model` listing becomes confusing — active provider isn't in the list.

**Why it happens:** `providerRegistry.Update()` replaces the map atomically, but the REPL holds a direct `provider.Provider` pointer, not a name-based lookup. This is an existing edge case amplified by profiles, since profiles encode provider names in files.

**Prevention:** Accept stale references — the active provider instance continues working. Log a warning if `r.activeProvider` disappears from the registry after reload. Don't auto-switch mid-conversation.

**Detection:** Manual test: activate profile, rename provider in config, verify chat still works and no panic occurs.

---

### Pitfall 12: Session JSON Doesn't Record Profile — `/load` + `refreshSystemPrompt()` Clobbers Profile Prompt

**What goes wrong:** User activates profile "coder" with a custom system prompt. They save the session. Later, they `/load` it — messages (including the system prompt in `Messages[0]`) are restored, but `r.baseSystemPrompt` is still the default. Next call to `refreshSystemPrompt()` (e.g., after a Lua tool create event) replaces the loaded session's system prompt with the default + tool descriptions, destroying the profile's prompt.

**Why it happens:** `Session` struct has `ID`, `Model`, `Messages`, `TokenCount` — no `Profile` or `BaseSystemPrompt` field. On load, `baseSystemPrompt` isn't restored.

**Prevention:** Add `Profile string` field to `Session` and extract `baseSystemPrompt` on load:

```go
type Session struct {
    // ... existing fields
    Profile string `json:"profile,omitempty"` // Active profile name, empty if none
}
```

On `/load`: if session has a profile, reload that profile's system prompt into `baseSystemPrompt`. If no profile, extract base prompt from `Messages[0]` by stripping the `## Available Tools` section.

**Detection:** Test: activate profile, save, load, trigger `refreshSystemPrompt()` — verify system prompt still has profile content.

---

## Minor Pitfalls

### Pitfall 13: Profile Filenames with Special Characters

**What goes wrong:** `fenec profile create --name "my cool profile!"` creates `profiles/my cool profile!.md`. Spaces and special characters cause shell escaping issues.

**Prevention:** Restrict profile names to `[a-z0-9_-]`:

```go
var validProfileName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
```

Validate on create, reject with clear error message.

---

### Pitfall 14: Config Migration on Non-macOS Runs Unnecessarily

**What goes wrong:** On Linux, `os.UserConfigDir()` already returns `~/.config`. Migration logic checking for `~/Library/Application Support/fenec` wastes time and might hit edge cases on unexpected paths.

**Prevention:** Guard migration with `runtime.GOOS == "darwin"`. On Linux, `ConfigDir()` returns the standard XDG path with zero migration logic.

---

### Pitfall 15: `/clear` Confirmation and Help Text Missing

**What goes wrong:** `/clear` executes silently. User isn't sure it worked. `helpText` constant in `commands.go` doesn't include `/clear`.

**Prevention:** Print `"Conversation cleared. Previous session saved as {session-id}."` Update `helpText` to include `/clear - Reset conversation (saves current session first)`.

---

### Pitfall 16: Subcommand `fenec profile list` Triggers Pipe Mode Detection

**What goes wrong:** Running `profiles=$(fenec profile list)` triggers the pipe mode check (`!term.IsTerminal(os.Stdin)`). The code auto-enables pipe mode and tries to read stdin as a chat message instead of running the profile subcommand.

**Prevention:** Route subcommands BEFORE pipe detection and BEFORE flag parsing:

```go
func main() {
    // Subcommand routing — must happen before pipe detection
    if len(os.Args) > 1 && os.Args[1] == "profile" {
        handleProfileSubcommand(os.Args[2:])
        return  // Exit before pipe/interactive/flag logic
    }
    
    // ... existing: pflag.Parse(), pipe detection, config load, REPL
}
```

**Detection:** Test: `fenec profile list 2>/dev/null | head -1` returns a profile, not REPL output.

---

### Pitfall 17: Profile Frontmatter Validates Provider Name at Create Time but Provider May Not Exist Yet

**What goes wrong:** User creates a profile with `provider = "azure"` before adding an Azure provider to config.toml. Validation at create time rejects it. But the profile is supposed to be a static file — it should be valid to reference future providers.

**Prevention:** Validate provider/model at activation time (when `--profile` is used), not at create time. Create should validate syntax only (well-formed TOML, required fields present), not semantics.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation | Severity |
|-------------|---------------|------------|----------|
| `/clear` command | sync.Once prevents re-save (#1), empty overwrite (#2), ghost token counts (#10) | Replace sync.Once with resettable guard, save-before-clear sequence, add tracker.Reset() | Critical |
| Profile system prompt | Tool descriptions clobbered (#3), precedence ambiguity (#9) | Always go through refreshSystemPrompt(), define --system > --profile > system.md chain | Critical |
| Config path migration | fsnotify watches old dir (#4), incomplete migration (#6), non-macOS guard (#14) | Migrate inside ConfigDir() before return, move entire tree with os.Rename, guard with GOOS | Critical |
| CLI subcommands | pflag consumes args (#5), pipe mode interference (#16) | Route subcommands in os.Args before pflag.Parse() and before pipe detection | Critical |
| TOML frontmatter | No standard library, edge cases (#8) | Custom parser with 9-case table-driven test suite | Moderate |
| Profile + model switch | Context length stale (#7), session lacks profile (#12), stale provider (#11) | Extract shared switchModel(), add Profile to Session, accept stale refs with warning | Moderate |
| Profile naming | Special characters (#13), future providers (#17) | Regex validation on name, validate provider at activation not creation | Minor |
| UX feedback | Silent /clear, missing help (#15) | Print confirmation with saved session ID, update helpText | Minor |

## Sources

- Direct codebase audit: `internal/repl/repl.go` lines 40 (sync.Once), 779-789 (refreshSystemPrompt), 630-646 (autoSave) — HIGH confidence
- Direct codebase audit: `internal/config/config.go` lines 46-52 (ConfigDir using os.UserConfigDir) — HIGH confidence
- Direct codebase audit: `internal/config/watcher.go` lines 47-49 (watches parent directory of configPath) — HIGH confidence
- Direct codebase audit: `internal/session/store.go` lines 109-115 (AutoSave to _autosave.json) — HIGH confidence
- Direct codebase audit: `internal/chat/context.go` lines 1-92 (ContextTracker with no Reset method) — HIGH confidence
- Direct codebase audit: `main.go` lines 26-48 (pflag flag definitions, no subcommand routing) — HIGH confidence
- Go sync.Once documentation: no Reset method by design — HIGH confidence
- Hugo TOML frontmatter convention: `+++` delimiters — HIGH confidence
- pflag documentation: flag-only parser, no subcommand concept — HIGH confidence
- `os.UserConfigDir()` Go stdlib: returns `~/Library/Application Support` on macOS, `$XDG_CONFIG_HOME` or `~/.config` on Linux — HIGH confidence
- `os.Rename` Go stdlib: atomic on same filesystem, returns EXDEV error for cross-device — HIGH confidence

---
*Pitfalls research for: Fenec v1.3 — Profiles, Config Migration & CLI Subcommands*
*Researched: 2025-07-14*
