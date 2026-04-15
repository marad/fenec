# Project Research Summary

**Project:** Fenec v1.3 — Profiles, Subcommands & Config Migration
**Domain:** CLI AI assistant — profile/persona system, config path standardization, conversation management
**Researched:** 2025-07-18
**Confidence:** HIGH

## Executive Summary

Fenec v1.3 adds a profile system (named persona files with TOML frontmatter), CLI subcommands for profile management, macOS config path migration from `~/Library/Application Support/fenec` to `~/.config/fenec`, new `--system`/`--profile` CLI flags, and a `/clear` REPL command. The competitive landscape (aichat, llm CLI, mods, fabric) confirms these are table-stakes features for CLI AI tools — every major competitor has profile/role equivalents, and `~/.config/` is the universal CLI convention. The good news: Fenec's existing stack already handles everything. **Zero new dependencies are needed.** BurntSushi/toml already supports string decoding for frontmatter, pflag supports scoped FlagSets for subcommands, and Go stdlib covers config migration, editor integration, and path resolution.

The recommended approach is a dependency-ordered build: config path migration first (every feature depends on correct paths), then the independent `/clear` command, then the profile data model, followed by flag integration and subcommand routing. This order was derived from both the architecture dependency graph and pitfall analysis — building out of order risks features operating against wrong paths or incomplete data models. The architecture is conservative: one new package (`internal/profile/`), modifications to three existing files, and manual subcommand routing instead of Cobra.

The primary risks are subtle state management bugs, not architectural complexity. The `/clear` command has three interacting pitfalls (sync.Once preventing re-save, empty session overwriting valuable data, ghost token counts triggering false truncation). Config migration must happen inside `ConfigDir()` before it returns, or fsnotify will watch the wrong directory. Profile activation must go through `refreshSystemPrompt()` or tool descriptions silently vanish. All pitfalls have verified prevention strategies with concrete code patterns. The overall risk is low because every feature touches a narrow surface area and the existing codebase is well-structured.

## Key Findings

### Recommended Stack

Zero new dependencies. All v1.3 features build on the existing dependency set plus Go stdlib. This is a direct result of good prior stack choices.

**Core technologies (all existing):**
- **BurntSushi/toml v1.6.0:** TOML frontmatter parsing via `toml.Decode(string, &struct)` — already in go.mod, only `DecodeFile()` used today but `Decode()` is first-class API
- **spf13/pflag v1.0.10:** Subcommand routing via `pflag.NewFlagSet()` per subcommand — already in go.mod, scoped flag parsing confirmed
- **Go stdlib `os/exec`:** `$EDITOR` integration for `profile create`/`edit` — standard `cmd.Run()` with stdio wiring
- **Go stdlib `os`:** Config migration via `os.Rename()` (atomic on same filesystem), `os.UserHomeDir()` for XDG path construction
- **Go stdlib `runtime`:** `runtime.GOOS == "darwin"` guard for macOS-only migration

**What NOT to add:** Cobra (overkill for 3 subcommands), adrg/xdg (10 lines of stdlib replaces it), any frontmatter parser library (trivially 25 lines), mitchellh/go-homedir (deprecated, stdlib covers it).

### Expected Features

**Must have (table stakes):**
- `--profile <name>` flag — every competitor has this (aichat `-r`, mods `--role`, fabric `-p`)
- Profile = system prompt + model override — minimum useful profile definition
- `fenec profile list` — users need to discover available profiles
- `--system <file>` flag — ad-hoc prompt override without creating a saved profile
- Human-editable profile files — markdown with TOML frontmatter, `$EDITOR`-friendly
- `/clear` REPL command — reset conversation without quitting
- `~/.config/fenec` as canonical path — CLI convention, `~/Library/Application Support/` is for GUI apps
- Auto-migration from old path — no data loss on upgrade

**Should have (differentiators):**
- TOML frontmatter (`+++` delimiters) — consistent with Fenec's TOML-everywhere approach
- Provider override in profiles — pin both provider AND model (e.g., `copilot/claude-sonnet-4`)
- Interactive `fenec profile create` — opens `$EDITOR` with scaffold template
- `fenec profile edit <name>` — convenience wrapper for editing profiles
- Migration feedback message — "Migrated config from X → Y"

**Defer (v2+):**
- Profile switching mid-session via REPL command (needs conversation context decisions)
- `FENEC_CONFIG_DIR` env var override
- Profile inheritance/composition (over-engineering)
- Automatic profile selection based on context (fragile, surprising)

### Architecture Approach

The architecture adds one new internal package and modifies three existing files. Profiles are markdown files with TOML frontmatter stored in `~/.config/fenec/profiles/`. The startup flow gains two new insertion points: migration runs inside `ConfigDir()` before it returns, and profile/system-prompt resolution slots between config loading and REPL creation. Subcommand routing uses pre-pflag `os.Args` dispatch — not Cobra. The `/clear` command resets conversation state through existing REPL internals.

**Major components:**
1. **`internal/profile/`** (NEW) — Profile struct, TOML frontmatter parsing, Load/List/Create, file I/O, `$EDITOR` integration (~200 LOC)
2. **`internal/config/config.go`** (MODIFIED) — `ConfigDir()` returns `~/.config/fenec` on macOS, `MigrateIfNeeded()` with atomic directory rename, `ProfilesDir()` helper
3. **`main.go`** (MODIFIED) — Pre-pflag subcommand dispatch, `--profile`/`--system` flags, migration call on startup, priority-chain resolution for model and prompt
4. **`internal/repl/repl.go`** (MODIFIED) — `/clear` handler with save-before-clear, resettable auto-save guard, context tracker reset

**Key patterns:**
- **Flag layering:** `config.toml defaults < profile settings < CLI flags` (standard Unix convention)
- **Filename-as-identity:** Profile name derived from filename, not stored in content
- **Priority chain:** `--model` > profile model > config default; `--system` > profile prompt > `system.md` > hardcoded default

### Critical Pitfalls

1. **`/clear` breaks `sync.Once` auto-save** — `sync.Once` has no `Reset()`. After `/clear`, the new conversation is never auto-saved. **Fix:** Replace with `bool` + `sync.Mutex` guard that `/clear` can reset.

2. **`/clear` overwrites previous session** — Without save-before-clear, `_autosave.json` gets empty conversation. **Fix:** Persist current session to named file FIRST, then create fresh session, then reset.

3. **Profile clobbers tool descriptions** — Replacing `baseSystemPrompt` without calling `refreshSystemPrompt()` makes tools invisible to the model. **Fix:** Always update `baseSystemPrompt` then call `refreshSystemPrompt()`. Never write `conv.Messages[0]` directly.

4. **fsnotify watches wrong directory after migration** — If migration runs after `ConfigDir()` returns, the watcher watches the old path. **Fix:** Migration must happen INSIDE `ConfigDir()` before it returns.

5. **pflag.Parse() consumes subcommand arguments** — `fenec profile create` fails because pflag sees "profile" as unknown positional arg. **Fix:** Route subcommands via `os.Args` BEFORE `pflag.Parse()`.

6. **ContextTracker ghost token counts** — After `/clear`, stale token counts trigger truncation on the first new message. **Fix:** Add `ContextTracker.Reset()` method, call from `/clear`.

## Implications for Roadmap

Based on research, suggested phase structure (6 phases, matching the dependency graph identified in architecture and features research):

### Phase 1: Config Path Migration
**Rationale:** Foundational — every other feature reads/writes through `ConfigDir()`. Must be correct before anything is built on top. Both architecture and features research independently identified this as the first dependency.
**Delivers:** `~/.config/fenec` as canonical path on macOS, automatic migration of all existing data (config, sessions, tools, history, system.md), user feedback message.
**Addresses:** Table stake: `~/.config/fenec` canonical path, auto-migration from old path.
**Avoids:** Pitfall #4 (fsnotify watches wrong dir) — migration inside `ConfigDir()`; Pitfall #6 (incomplete migration) — move entire directory tree; Pitfall #14 (non-macOS guard).

### Phase 2: /clear REPL Command
**Rationale:** Fully independent of all other features — only touches REPL internals. Quick win that delivers immediate user value. BUT has 3 interacting critical pitfalls that must be handled carefully.
**Delivers:** `/clear` command that resets conversation, saves previous session, resets token tracking, updates help text.
**Addresses:** Table stake: conversation reset without quitting.
**Avoids:** Pitfall #1 (sync.Once) — resettable auto-save guard; Pitfall #2 (empty overwrite) — save-before-clear sequence; Pitfall #10 (ghost tokens) — ContextTracker.Reset(); Pitfall #15 (UX) — confirmation message with session ID.

### Phase 3: Profile Package
**Rationale:** Core data model needed by both `--profile` flag (Phase 5) and profile subcommands (Phase 6). Building the package in isolation enables thorough testing of TOML frontmatter parsing edge cases before integration.
**Delivers:** `internal/profile/` package — Profile struct, `Load()`, `List()`, `Create()`, frontmatter parser with edge case handling, profile name validation.
**Addresses:** Differentiator: TOML frontmatter format, provider override in profiles.
**Avoids:** Pitfall #8 (frontmatter edge cases) — table-driven tests for 9 cases; Pitfall #13 (special characters) — `[a-z0-9_-]` validation; Pitfall #17 (provider validation timing) — validate at activation, not creation.

### Phase 4: --system Flag
**Rationale:** Simpler than `--profile` (just reads a file and uses as system prompt), establishes the prompt-override pattern that profiles build on. Good warmup for the more complex profile flag integration.
**Delivers:** `--system <file>` CLI flag that overrides the system prompt for one invocation.
**Addresses:** Table stake: ad-hoc system prompt override.
**Avoids:** Pitfall #9 (precedence ambiguity) — define `--system` > profile > `system.md` > default chain explicitly.

### Phase 5: --profile Flag
**Rationale:** Uses profile package from Phase 3. Modifies the startup flow in main.go for model/provider/prompt resolution. More complex integration than `--system` because it affects both model and prompt. Must come after the profile package is proven.
**Delivers:** `--profile <name>` / `-P` flag, priority-chain resolution for model and prompt, composable with `--model` and `--system` overrides.
**Addresses:** Table stake: named profile selection via flag, profile = prompt + model override.
**Avoids:** Pitfall #3 (tool descriptions clobbered) — use `refreshSystemPrompt()`; Pitfall #7 (stale context length) — extract shared `switchModel()` method; Pitfall #9 (precedence) — documented priority chain; Pitfall #12 (session lacks profile) — add Profile field to Session struct.

### Phase 6: Profile Subcommands
**Rationale:** Most invasive to `main.go` structure — adds subcommand routing. Benefits from having profile loading already tested via `--profile` (Phase 5). Requires pre-pflag dispatch pattern.
**Delivers:** `fenec profile list`, `fenec profile create <name>`, `fenec profile edit <name>` — with `$EDITOR` integration.
**Addresses:** Table stakes: profile listing, differentiators: interactive creation, edit subcommand.
**Avoids:** Pitfall #5 (pflag consumes args) — route via `os.Args` before `pflag.Parse()`; Pitfall #16 (pipe mode interference) — subcommand dispatch before pipe detection.

### Phase Ordering Rationale

- **Dependency-driven:** Config migration → profile package → flags → subcommands follows the strict dependency graph both architecture and features research identified independently
- **Risk-front-loaded:** Phases 1-2 address the most critical pitfalls (data loss, silent state corruption) early when the codebase is still simple
- **Progressive integration:** Each phase touches one more layer — Phase 1 is config-only, Phase 2 is REPL-only, Phase 3 is new package, Phases 4-5 modify main.go incrementally, Phase 6 restructures main.go routing
- **Test confidence builds:** Profile package is proven in isolation (Phase 3) before being wired into startup flow (Phase 5), so integration bugs are easier to isolate

### Research Flags

Phases with standard patterns (skip `/gsd-research-phase`):
- **Phase 1 (Config Migration):** Well-documented `os.Rename()` behavior, migration logic is ~25 lines, Go stdlib only
- **Phase 2 (/clear):** All APIs verified in source, but implementation must follow exact save-before-clear sequence from pitfalls research
- **Phase 3 (Profile Package):** Frontmatter parsing is straightforward string splitting + existing `toml.Decode()`
- **Phase 4 (--system flag):** Trivial flag addition with file read — established pflag pattern
- **Phase 6 (Subcommands):** Simple `os.Args` routing, `$EDITOR` integration is standard `os/exec` pattern

Phases that may benefit from `/gsd-research-phase`:
- **Phase 5 (--profile flag):** Most complex integration — touches startup flow, model resolution, provider switching, context tracker updates, session persistence. Multiple interacting pitfalls. Worth a phase research pass to map exact code insertion points and test scenarios.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All APIs verified via `go doc`, existing deps confirmed in `go.mod`, zero new dependencies |
| Features | HIGH | Competitive analysis of 4 real tools (aichat, llm, mods, fabric) with source code review |
| Architecture | HIGH | Based on direct codebase audit — all integration points, line numbers, and APIs verified in source |
| Pitfalls | HIGH | All 17 pitfalls derived from codebase audit with specific line references and verified Go stdlib behavior |

**Overall confidence:** HIGH

### Gaps to Address

- **Cross-device rename fallback:** STACK.md says same-filesystem rename is guaranteed (both paths under `$HOME`), but ARCHITECTURE.md includes a `copyDirRecursive` fallback. Decide during Phase 1 whether to implement the fallback or trust same-FS assumption. Recommendation: skip fallback — `$HOME` subdirectories are always same filesystem on macOS.
- **`--system` + `--profile` interaction:** FEATURES.md originally said these should be mutually exclusive (error). ARCHITECTURE.md and PITFALLS.md say they should be composable (`--system` overrides prompt, keeps profile's model). **Go with composable** — the architecture/pitfalls position is more flexible and avoids artificial restrictions.
- **Session profile field:** Pitfall #12 identifies that Session JSON needs a `Profile` field for `/load` to restore the correct system prompt. This isn't mentioned in FEATURES.md or ARCHITECTURE.md. Address during Phase 5 implementation — add the field to Session struct.
- **`sync.Once` replacement approach:** PITFALLS.md suggests `bool` + `sync.Mutex`. ARCHITECTURE.md suggests `r.autoSaved = sync.Once{}` (zero-value reset). The `bool` + `Mutex` approach is safer and more explicit. Use that.

## Sources

### Primary (HIGH confidence)
- `go doc github.com/BurntSushi/toml Decode` — confirmed `func Decode(data string, v any) (MetaData, error)`
- `go doc github.com/spf13/pflag FlagSet` — confirmed `NewFlagSet()` with scoped parsing
- `go doc os UserConfigDir` — confirmed macOS returns `~/Library/Application Support`
- `go doc os UserHomeDir` — confirmed returns `$HOME` on Unix/macOS
- Direct codebase audit: `internal/config/config.go`, `internal/repl/repl.go`, `internal/chat/`, `internal/session/`, `main.go` — all integration points verified with line numbers
- Competitive source code: aichat (`src/config/role.rs`), mods (Go source), fabric (Go source)

### Secondary (MEDIUM confidence)
- Hugo/Zola TOML frontmatter `+++` convention — well-established but informal standard
- llm CLI docs at `llm.datasette.io` — official documentation, not source code

---
*Research completed: 2025-07-18*
*Ready for roadmap: yes*
