# Feature Landscape

**Domain:** CLI AI assistant — profile/persona system, config migration, conversation management
**Researched:** 2025-07-14
**Confidence:** HIGH (based on analysis of aichat, llm CLI, mods, fabric source code + Fenec codebase)

## Competitive Landscape Summary

Five tools define the ecosystem for CLI AI assistants with profile-like systems:

| Tool | Language | Profile Concept | Storage Format | System Prompt Override |
|------|----------|----------------|----------------|----------------------|
| **aichat** | Rust | "Roles" — markdown files with YAML frontmatter in `roles/` dir | `<name>.md` with `---` YAML frontmatter | `.role <name>` REPL command or `-r` flag |
| **llm** (Simon Willison) | Python | "Templates" — YAML files combining prompt + model + options | YAML files in `templates/` dir | `-s/--system` flag inline, `--save` to persist |
| **mods** (Charm) | Go | "Roles" — YAML list of system messages in config file | Inline in `mods.yaml` config under `roles:` key | `--role <name>` flag |
| **fabric** | Go | "Patterns" — directories with `system.md` files | `~/.config/fabric/patterns/<name>/system.md` | `--pattern <name>` / `-p` flag |
| **Fenec** (planned) | Go | "Profiles" — markdown files with TOML frontmatter | `~/.config/fenec/profiles/<name>.md` | `--system <file>` flag + `--profile` flag |

---

## Table Stakes

Features users expect from any CLI tool with a profile/persona system. Missing = product feels incomplete or broken.

### Profile/Persona Features

| Feature | Why Expected | Complexity | Existing Dependency | Notes |
|---------|--------------|------------|-------------------|-------|
| **Named profile selection via flag** (`--profile <name>` / `-P`) | aichat (`-r`), mods (`--role`), fabric (`-p`) all support this. Users expect CLI flag activation. | Low | pflag already wired | Just a new string flag + lookup in profiles dir |
| **Profile = system prompt + model override** | aichat roles have prompt + model in frontmatter. llm templates have prompt + model. This is the minimum useful combination. | Low | Existing `LoadSystemPrompt()` + `--model` flag logic | Profile overrides defaults for both |
| **Profile listing** (`fenec profile list`) | aichat (`.role` lists roles), llm (`llm templates`), mods (`--list-roles`). Users need to discover what's available. | Low | `os.ReadDir()` on profiles dir | Just enumerate `profiles/*.md` |
| **Ad-hoc system prompt override** (`--system <file>`) | llm (`-s`), fabric patterns. Essential for one-off prompt injection without creating a saved profile. | Low | Existing `LoadSystemPrompt()` function | Replaces `system.md` for this invocation only |
| **Profile stored as human-editable files** | aichat = `.md` files, llm = YAML files, fabric = `system.md` in dirs. Users expect to `$EDITOR` their profiles directly. | Low | Already using `system.md` for default prompt | Natural extension of existing pattern |
| **Conversation reset** (`/clear`) | aichat (`.empty session`). Every REPL with session context needs a way to start fresh without quitting. | Low | Existing `chat.Conversation` + `session.Session` types | Reset messages to just system prompt |

### Config Path Features

| Feature | Why Expected | Complexity | Existing Dependency | Notes |
|---------|--------------|------------|-------------------|-------|
| **~/.config/fenec as canonical path** | CLI convention — aichat uses `~/.config/aichat`, mods uses `~/.config/mods`. macOS `~/Library/Application Support/` is for GUI apps. CLI tools that use it feel wrong. | Low | Existing `config.ConfigDir()` uses `os.UserConfigDir()` which returns `~/Library/Application Support/` on macOS | Change to hardcode `~/.config/fenec` |
| **Auto-migration from old path** | Data loss = unacceptable. Users who have sessions, config, tools at the old path must not lose them. | Medium | All existing code goes through `config.ConfigDir()` | One-time move on startup |

---

## Differentiators

Features that set Fenec apart. Not universally expected, but add significant value.

| Feature | Value Proposition | Complexity | Existing Dependency | Notes |
|---------|-------------------|------------|-------------------|-------|
| **TOML frontmatter in profile markdown** | Fenec already uses TOML config. TOML frontmatter (`+++` delimiters, Hugo convention) keeps the ecosystem consistent. aichat uses YAML frontmatter, but Fenec's TOML-everywhere story is cleaner. | Low | Existing `BurntSushi/toml` dependency | Parse `+++...+++` then TOML decode header |
| **Profile includes provider override** | Most tools only override model. Fenec's `provider/model` syntax means profiles can pin both provider AND model — e.g., a "code-review" profile always uses `copilot/claude-sonnet-4`. | Low | Existing `providerRegistry.Get()` | Add `provider` field to frontmatter or use existing `provider/model` syntax in the `model` field |
| **Interactive profile creation** (`fenec profile create`) | aichat and llm both let you edit roles/templates, but neither has a guided creation flow. A subcommand that opens `$EDITOR` with a template scaffold is more discoverable. | Medium | Needs subcommand routing (pflag `Args()` or simple arg parsing) | Opens editor with pre-filled template |
| **Profile edit subcommand** (`fenec profile edit <name>`) | llm has `llm templates edit`. Opens the file in `$EDITOR` directly. Saves users from remembering the profile directory path. | Low | `os.Getenv("EDITOR")` + `exec.Command` | Convenience wrapper |
| **Migration with user feedback** | Print a clear message: "Migrated config from ~/Library/Application Support/fenec → ~/.config/fenec". Silent migrations confuse users when they go looking for their files. | Low | `fmt.Fprintf(os.Stderr, ...)` | One log line at startup |

---

## Anti-Features

Features to explicitly NOT build for v1.3.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Profile inheritance/composition** | aichat doesn't have it, fabric doesn't have it. Adds complexity with unclear benefit. "Profile A extends Profile B" is over-engineering for a personal tool. | Keep profiles flat and independent. Users can copy-paste between profile files. |
| **Profile stored in main config TOML** | mods puts roles inline in its YAML config. This couples profile management to config editing and makes per-profile files impossible. Fenec correctly plans separate files. | One file per profile in `profiles/` dir. |
| **GUI profile editor / TUI form** | Out of scope per PROJECT.md (CLI only). A TUI form for profile fields would be scope creep. | `$EDITOR` with a template scaffold. |
| **Cloud profile sync** | Personal tool, single user. No cloud. | Profiles are plain files — users can use git/syncthing/dotfiles themselves. |
| **Automatic profile selection based on context** | "Smart" routing that picks a profile based on working directory or prompt content is fragile and surprising. | Explicit `--profile` flag or REPL command. User is in control. |
| **`--system` accepting inline strings** | llm does `-s 'inline prompt text'`. For Fenec, `--system` should accept a **file path** only, matching the existing `system.md` file-based pattern. Inline strings encourage long unwieldy commands and are hard to reuse. | `--system path/to/prompt.md` reads from file. For inline, users can create a profile. |
| **cobra migration for subcommands** | Fenec uses pflag directly. Adding cobra just for `fenec profile *` subcommands is heavy. pflag + `pflag.Args()` positional arg parsing is sufficient for 3 subcommands. | Parse `os.Args` / `pflag.Args()` for `profile` subcommand routing. If subcommand count grows past 5-6 in future milestones, reconsider cobra then. |
| **XDG_CONFIG_HOME env var override** | aichat supports `AICHAT_CONFIG_DIR` env var. Nice-to-have but not v1.3 scope. Hardcoding `~/.config/fenec` handles 99% of cases. | Can add `FENEC_CONFIG_DIR` env var in a future milestone if requested. |
| **Profile applied mid-session via REPL command** | aichat supports `.role <name>` in REPL to switch roles mid-conversation. Adds complexity around what happens to existing conversation context. | Defer to v1.4. For now, profiles are launch-time only via `--profile` flag. |

---

## Feature Dependencies

```
Config path migration → Must happen BEFORE any other feature (all paths depend on ConfigDir())
  │
  ├── --system flag (reads file from disk, needs correct config context)
  │
  ├── Profile storage (profiles/ dir lives under ConfigDir())
  │     │
  │     ├── --profile flag (loads profile from profiles/ dir)
  │     │
  │     ├── fenec profile list (enumerates profiles/ dir)
  │     │
  │     ├── fenec profile create (writes to profiles/ dir)
  │     │
  │     └── fenec profile edit (opens file from profiles/ dir)
  │
  └── /clear command (independent of config path, but listed here for completeness)

Subcommand routing → Required for `fenec profile *` commands
  │
  ├── fenec profile create
  ├── fenec profile list
  └── fenec profile edit
```

### Dependency Notes

1. **Config path migration is foundational.** Every feature reads/writes to `ConfigDir()`. Changing the path must happen first and be tested thoroughly before building on top.

2. **`--system` flag is independent of profiles.** It's a simpler feature (read file, use as system prompt) and should be built before the profile system since profiles build on the same prompt-loading pattern.

3. **`/clear` is fully independent.** It only touches `chat.Conversation` internals. Can be built in any order.

4. **Subcommand routing is a prerequisite for `fenec profile *`.** But `--profile` flag works with existing pflag patterns.

---

## Expected User Workflows

### Creating a Profile

```bash
# Option A: guided creation (opens $EDITOR with scaffold)
$ fenec profile create coder
# Editor opens with:
# +++
# model = "copilot/claude-sonnet-4"
# +++
# You are a senior Go developer. Be concise. Prefer table-driven tests.

# Option B: manual creation (power user)
$ cat > ~/.config/fenec/profiles/coder.md << 'EOF'
+++
model = "copilot/claude-sonnet-4"
+++
You are a senior Go developer. Be concise. Prefer table-driven tests.
EOF
```

### Switching Profiles

```bash
# At launch
$ fenec --profile coder
$ fenec -P coder

# Combined with other flags
$ fenec -P coder --debug
$ fenec -P coder --model ollama/gemma4  # --model overrides profile's model

# List available profiles
$ fenec profile list
  coder       copilot/claude-sonnet-4
  writer      ollama/gemma4
  reviewer    copilot/claude-sonnet-4
```

### Ad-hoc System Prompt Override

```bash
# One-off system prompt from file
$ fenec --system ~/prompts/sql-expert.md

# Works with pipe mode
$ cat schema.sql | fenec --system ~/prompts/sql-expert.md --pipe

# --system and --profile are mutually exclusive (fail fast with error)
```

### Config Migration Experience

```bash
$ fenec
# First run after update:
# "Migrated configuration from ~/Library/Application Support/fenec to ~/.config/fenec"
# Then normal startup continues

# Subsequent runs: no message, no migration check needed
# (migration is idempotent — if new path exists, skip)
```

### Clearing Conversation Mid-Session

```
You> /clear
Conversation cleared. Starting fresh.

You> (new conversation, system prompt preserved)
```

---

## MVP Recommendation

**Prioritize in this order:**

1. **Config path migration** — Foundational. Unblocks everything else. Do first.
2. **`/clear` REPL command** — Trivial, independent, instant user value.
3. **`--system <file>` flag** — Simple flag, establishes the pattern for prompt loading from arbitrary files.
4. **Profile file format + `--profile` flag** — Core profile feature. Parse TOML frontmatter, load prompt, override model/provider.
5. **`fenec profile list`** — Enumerate profiles directory. Needed for discoverability.
6. **`fenec profile create`** — Scaffold template, open in `$EDITOR`.
7. **`fenec profile edit <name>`** — Open existing profile in `$EDITOR`.

**Defer:**
- `/profile` REPL command for mid-session switching (v1.4 — needs conversation context decisions)
- `FENEC_CONFIG_DIR` env var override (future — if requested)
- Profile composition/inheritance (likely never needed)

---

## Detailed Feature Specifications

### Profile File Format

```markdown
+++
model = "copilot/claude-sonnet-4"
+++
You are a senior Go developer...
```

**Frontmatter fields (all optional):**

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `model` | string | config default | Can be bare model name or `provider/model` syntax |

**Parsing rules:**
- If file starts with `+++`, parse TOML between first and second `+++`
- Everything after second `+++` is the system prompt (trimmed)
- If no `+++` delimiters, entire file is the system prompt (backward-compat with plain `system.md`)
- Empty prompt section → use default system prompt (profile only overrides model)

### Precedence Rules

```
Highest priority → Lowest priority:

--model flag        > profile model   > config default_model  > first available
--system flag       > profile prompt  > ~/.config/fenec/system.md > hardcoded default
--profile activates both model + prompt from the profile file
--system + --profile = error (conflicting prompt sources — fail fast)
```

### /clear Implementation

**What it resets:**
- `conv.Messages` → reset to just `[system_message]`
- `session` → create new `session.Session` with fresh ID + timestamp
- `tracker` → reset token counts
- Auto-save of old session before clearing (if has content)

**What it preserves:**
- System prompt (including tool descriptions)
- Active model + provider
- Debug mode, yolo mode
- REPL readline history

### Config Path Migration

**Migration algorithm (macOS only):**
```
oldDir = os.UserConfigDir() + "/fenec"        // ~/Library/Application Support/fenec
newDir = os.Getenv("HOME") + "/.config/fenec" // ~/.config/fenec

if runtime.GOOS != "darwin" {
    // Linux already uses ~/.config via os.UserConfigDir()
    // Just change ConfigDir() to always return ~/.config/fenec
    return
}

if !exists(oldDir) {
    return  // Nothing to migrate
}

if exists(newDir) {
    return  // Already migrated (or user manually created it)
}

os.MkdirAll(filepath.Dir(newDir), 0755)
os.Rename(oldDir, newDir)  // Atomic move on same filesystem
log("Migrated configuration: %s → %s", oldDir, newDir)
```

**Files affected:**
- `config.toml` (main config)
- `system.md` (system prompt)
- `sessions/` (saved sessions)
- `tools/` (Lua tools)
- `history` (readline history)

### Subcommand Routing (without cobra)

```go
// In main.go, before pflag.Parse():
args := os.Args[1:]
if len(args) > 0 && args[0] == "profile" {
    handleProfileSubcommand(args[1:])
    return
}
// Then normal pflag.Parse() for flags
```

This is the simplest pattern for adding subcommands to a pflag-based CLI. It handles:
- `fenec profile list`
- `fenec profile create <name>`
- `fenec profile edit <name>`

---

## Sources

- **aichat** role system: `src/config/role.rs` — roles are `.md` files with YAML frontmatter (`---`), stored in `roles/` dir under config. Built-in roles embedded via `rust_embed`. Fields: model, temperature, top_p, use_tools. [HIGH confidence — read source directly]
- **aichat** config dir: `src/config/mod.rs` — uses `XDG_CONFIG_HOME` or `dirs::config_dir()`. Supports `AICHAT_CONFIG_DIR` env override. [HIGH confidence — read source]
- **aichat** clear: `.empty session` REPL command calls `empty_session()` which calls `clear_messages()`. [HIGH confidence — read source]
- **llm** templates: YAML files in `templates/` dir. Created via `--save` flag or `llm templates edit`. Support prompt, system prompt, model, options, schema, tools. [HIGH confidence — official docs at llm.datasette.io]
- **llm** system prompt: `-s/--system` flag for inline system prompt text. Can be saved to template. [HIGH confidence — official docs]
- **mods** roles: YAML list of system messages under `roles:` key in config file. `--role <name>` flag. Each role message loaded via `loadMsg()` which can load from file paths. Uses cobra for CLI. [HIGH confidence — read source]
- **fabric** patterns: `~/.config/fabric/patterns/<name>/system.md` directory structure. `--pattern` flag. Go-based. [HIGH confidence — README + source]
- **TOML frontmatter**: `+++` delimiter convention from Hugo/Zola static site generators. Well-established, unambiguous. [HIGH confidence — widely documented]
- **Go `os.UserConfigDir()`**: Returns `~/Library/Application Support` on macOS, `~/.config` on Linux. [HIGH confidence — Go stdlib docs]
