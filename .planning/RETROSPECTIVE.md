# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Fenec Platform Foundation

**Shipped:** 2026-04-12
**Phases:** 6 | **Plans:** 14 | **Commits:** 102

### What Was Built
- Streaming CLI chat with local Ollama models (Gemma 4 support)
- Multi-turn conversation management with context tracking and auto-truncation
- Session persistence with atomic writes and auto-save
- Tool system: registry, agentic loop, shell_exec with dangerous-command approval
- Sandboxed Lua runtime with startup loading and error reporting
- Self-extension: agent writes, validates, and hot-reloads its own Lua tools
- File tools (read/write/edit/list) with path deny-list and CWD approval gating

### What Worked
- TDD approach in plans kept implementation quality high — tests caught real issues early
- Wave-based parallel execution reduced wall-clock time for multi-plan phases
- ApproverFunc callback pattern scaled well across tool types (shell, write, edit)
- Interface-driven design (tool.Tool, ChatService) made adding new tools trivial

### What Was Inefficient
- Phase 01 glamour rendering was built then never wired — wasted effort on a feature the user didn't want
- PageOutput auto-pager was implemented and tested but never connected to the REPL
- Both were cleaned up as tech debt, but the effort to build them was unnecessary

### Patterns Established
- All tools implement `tool.Tool` interface (Name, Definition, Execute)
- ApproverFunc callback for permission gating (deferred via closure in main.go)
- Tool results are JSON strings returned to the model
- Path safety: IsDeniedPath before IsOutsideCWD (deny-before-approve ordering)
- Registry provenance tracking (built-in vs Lua) for safe self-extension

### Key Lessons
1. Ask about user preferences for UI/rendering BEFORE building — the glamour deviation could have been caught in discuss phase
2. Pure Go dependencies (gopher-lua instead of cgo LuaJIT) pay off for build simplicity
3. Closure-based wiring in main.go (approver, notifier, replRef) is clean but fragile if initialization order changes

### Cost Observations
- Model mix: ~80% opus (planning + execution), ~20% sonnet (verification + integration checks)
- Sessions: 1 (full autonomous run)
- Notable: Autonomous mode completed entire milestone in a single session

---

## Milestone: v1.1 — Multi-Provider Support

**Shipped:** 2026-04-14
**Phases:** 5 | **Plans:** 9 | **Commits:** 57

### What Was Built
- `internal/model` canonical types package — decoupled all packages from `ollama/api` with one clean adapter boundary
- `provider.Provider` 5-method interface + Ollama adapter — pluggable provider system without changing REPL or tools
- TOML config loading with `$ENV_VAR` API key resolution, provider registry, config-driven `main.go`
- fsnotify hot-reload watcher with 100ms debounce — live config changes without restart
- OpenAI-compatible adapter (openai-go v3) — full streaming SSE, non-streaming tool call fallback, thinking extraction
- `--model provider/model` CLI flag + `/model` REPL command with parallel provider-grouped model listing

### What Worked
- Phase-by-phase type decoupling (Phase 7) before the interface (Phase 8) before config (Phase 9) was the right order — each phase built cleanly on the last
- Compile-time interface guards (`var _ provider.Provider = (*Provider)(nil)`) caught mismatches immediately without needing tests
- The `provider/model` slash delimiter was intuitive and required zero special casing in the CLI flag parser
- Parallel `listModels()` with `context.WithTimeout(5s)` pattern worked well for discovery — unreachable providers degrade gracefully

### What Was Inefficient
- Phase 11 VERIFICATION.md was not generated after execution — required a retroactive `gsd-verify-work` pass to close the milestone. The artifact gap caused an unnecessary `gaps_found` → re-audit cycle.
- CONF-04 active-provider hot-reload gap: `r.provider` in REPL is not auto-refreshed after config reload. Known limitation that needs a `currentProvider()` accessor refactor in a future phase.

### Patterns Established
- Provider implementations live under `internal/provider/<name>/` with a compile-time interface check at package top
- `internal/config/toml.go` `CreateProvider()` factory is the single extension point for new provider types
- `ProviderRegistry` pointer is shared between watcher closure and REPL — hot-reload wires through `Update()` only
- All provider-specific SDK imports stay inside the adapter package — canonical types cross the boundary

### Key Lessons
1. Generate VERIFICATION.md immediately after execution (run `gsd-verify-work` before moving to next phase) — retroactive generation adds overhead to milestone audit
2. When two phases share a concern (OAIC-04: Phase 10 infra + Phase 11 UX), document the explicit split in CONTEXT.md — it was done well here and the integration checker confirmed it
3. Hot-reload patterns need a REPL accessor pattern, not just registry mutation — if the active provider can change externally, the consumer needs a way to re-fetch it

### Cost Observations
- Model mix: ~70% opus (planning + architecture phases), ~30% sonnet (verification + integration checks)
- Sessions: Multiple (one per major phase group)
- Notable: Phase 7 canonical types refactor was zero-regression — TDD approach with failing tests first caught 3 real issues in type conversion

---

## Milestone: v1.2 — GitHub Models Provider

**Shipped:** 2026-04-14
**Phases:** 2 | **Plans:** 4 | **Commits:** ~15

### What Was Built
- `internal/provider/copilot/` package wrapping `openai.Provider` via delegation — GitHub Models via `gh auth token`, no API key needed
- Token resolution chain (GH_TOKEN → GITHUB_TOKEN → `gh auth token`) with injectable functions for testability
- Catalog HTTP client with lazy double-checked locking cache — fetches 40+ models from `https://models.github.ai/v1/models`
- Real `max_input_tokens` context lengths from catalog (e.g., gpt-4o-mini=131072, gpt-4.1=1048576)
- Catalog-backed `Ping()` — validates auth and connectivity with a single catalog fetch; no chat round-trip
- Verified `/model` REPL grouping works correctly without code changes — `Name()="copilot"` + `ListModels()` integration was free

### What Worked
- Delegation pattern (copilot wraps openai) was the right call — streaming and tool calling "just worked" with zero adapter code
- `fetchCatalogFrom(ctx, url)` test injection pattern (separate URL-parameterized method) gave full mock HTTP coverage without test-only struct fields
- Cache-seeding pattern in tests (`fetchCatalogFrom` then calling public methods) is clean and reusable
- Phase 12 stubs that delegated to `inner` provider meant Phase 13 could be purely additive — no regressions

### What Was Inefficient
- The openai-go SDK's `ListAutoPaging` was found incompatible with GitHub Models catalog schema only during implementation — a quick API test before planning would have saved one iteration
- Phase 12 shipped stub `Ping()` that still made a direct HTTP call — this was then replaced in Phase 13. Planning the catalog integration together would have been cleaner.

### Patterns Established
- Provider delegation pattern: `copilot.Provider` embeds `*openai.Provider` as `inner` — all protocol-level operations delegated
- Injectable functions for subprocess testing: `resolveTokenWith(lookPathFn, commandFn)` pattern avoids full mock frameworks
- `fetchCatalogFrom` pattern: public API method uses const URL, test helper accepts URL — no test-only struct fields
- `var _ provider.Provider = (*Provider)(nil)` compile-time guard at package top (consistent with v1.1 convention)

### Key Lessons
1. Catalog compatibility is not guaranteed by openai-go SDK even for OpenAI-compatible endpoints — validate response schema early, especially for list operations
2. Two-phase stubs (delegate in P12, replace in P13) added a clean separation but created unnecessary intermediate code — when the final implementation is known upfront, build it in phase 1
3. `exec.ExitError` cannot be constructed directly in Go — real subprocess (`sh -c exit N`) is the pragmatic mock for exit code testing
4. `/model` REPL grouping is provider-agnostic by design — adding a new provider requires no REPL changes if `Name()` and `ListModels()` are correct

### Cost Observations
- Model mix: ~100% sonnet (small, well-defined phases with clear patterns from v1.1)
- Sessions: 1
- Notable: v1.2 was the fastest milestone yet — 2 phases, clear patterns from v1.1, delegation approach eliminated ~200 lines of boilerplate

---

## Milestone: v1.3 — Profiles & Config

**Shipped:** 2026-04-15
**Phases:** 6 | **Plans:** 6 | **Commits:** 47

### What Was Built
- `~/.config/fenec` config path standardization with automatic macOS migration from `~/Library/Application Support/fenec`
- `/clear` REPL command with auto-save, ContextTracker.Reset(), and token tracking reset
- `internal/profile` package — TOML frontmatter parsing, Load/List/Parse API, path traversal protection
- `--system/-s` flag for ad-hoc system prompt override per invocation
- `--profile/-P` flag with three-layer precedence chains (model: `--model` > profile > config; prompt: `--system` > profile > config)
- `fenec profile list/create/edit` subcommands via `internal/profilecmd` package with `$EDITOR` integration

### What Worked
- `pflag.CommandLine.Changed("model")` guard was the critical insight for correct precedence — prevents profile model from leaking when user explicitly sets `--model`
- Pre-pflag `os.Args` dispatch for subcommands avoided needing Cobra for just 3 commands — clean and simple
- Hugo-style `+++`-delimited TOML frontmatter is familiar to developers and trivial to parse
- Dir-injection pattern in profilecmd made testing possible without touching real filesystem config
- Discuss phase with 7 decisions (D-01 through D-07) caught the `--model` + profile interaction edge cases before any code was written

### What Was Inefficient
- Requirements for Phases 14 and 15 (CFG-01 through CFG-03, CONV-01 through CONV-03) were not checked off in REQUIREMENTS.md despite both phases being complete — discovered during milestone completion. Executors should update both the traceability table AND the checkbox list.
- Phase 15 progress table showed "0/1" plans complete despite being fully shipped — stale tracking artifact from a prior session
- UI gate regex matching "ui" in words like "profile" caused false positives — should use word-boundary matching

### Patterns Established
- Three-layer precedence for flags: CLI flag > profile setting > config default, using `pflag.Changed()` to distinguish explicit from default
- Profile file format: `+++`-delimited TOML frontmatter with `model` (provider/model syntax) + markdown body as system prompt
- Pre-pflag subcommand dispatch: `os.Args[1]` check before `pflag.Parse()` for subcommand routing
- `$EDITOR` integration: `strings.Fields()` to support multi-word editors, fallback to `vi`
- Path traversal protection: `strings.ContainsAny(name, "/\\.")` for profile names

### Key Lessons
1. Flag precedence design deserves a dedicated discuss-phase decision — the `--model` + `--profile` interaction had 3 subtle edge cases that were only caught through structured questioning
2. Executors need to update BOTH requirement checkboxes AND traceability table status — one without the other creates confusion at milestone completion
3. Empty-body fallthrough (D-05) is important for model-only profiles — profiles that only set a model should not override the default system prompt
4. `$EDITOR` with `strings.Fields()` handles editors like `code --wait` correctly — simple string split is better than shell parsing

### Cost Observations
- Model mix: ~60% opus (planning + execution for Phases 17-19), ~40% sonnet (research + verification)
- Sessions: Multiple (Phases 14-15 in prior session, 16 in another, 17-19 in this session)
- Notable: Phase 18 discuss phase was the most valuable — 7 decisions prevented 3 implementation bugs

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 102 | 6 | First milestone — established GSD workflow patterns |
| v1.1 | 57 | 5 | Provider abstraction layer; introduced milestone audit before complete |
| v1.2 | 15 | 2 | Delegation pattern; fastest milestone — built on v1.1 provider system |
| v1.3 | 47 | 6 | Profiles & config; discuss phase proved critical for flag precedence |

### Cumulative Quality

| Milestone | Go LOC | Test Files | Packages |
|-----------|--------|------------|----------|
| v1.0 | 6,970 | 15+ | 7 |
| v1.1 | 10,335 (4,499 prod + 5,836 test) | 20+ | 10 |
| v1.2 | ~11,300 | 22+ | 11 |
| v1.3 | ~12,400 | 24+ | 13 |

### Top Lessons (Verified Across Milestones)

1. Validate user preferences for visible features before implementation
2. Interface-driven tool design enables painless extensibility
3. Generate VERIFICATION.md immediately after execution — retroactive generation causes audit overhead
4. Compile-time interface guards are cheap and catch real bugs early
5. Delegation wrapping pattern avoids duplicating protocol-level code — prefer `inner.Method()` over re-implementing
6. Flag precedence requires explicit `Changed()` guards — implicit defaults silently override intended behavior
7. Pre-pflag dispatch is sufficient for a handful of subcommands — Cobra overhead not justified until 5+ subcommands
