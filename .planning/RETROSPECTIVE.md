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

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 102 | 6 | First milestone — established GSD workflow patterns |
| v1.1 | 57 | 5 | Provider abstraction layer; introduced milestone audit before complete |

### Cumulative Quality

| Milestone | Go LOC | Test Files | Packages |
|-----------|--------|------------|----------|
| v1.0 | 6,970 | 15+ | 7 |
| v1.1 | 10,335 (4,499 prod + 5,836 test) | 20+ | 10 |

### Top Lessons (Verified Across Milestones)

1. Validate user preferences for visible features before implementation
2. Interface-driven tool design enables painless extensibility
3. Generate VERIFICATION.md immediately after execution — retroactive generation causes audit overhead
4. Compile-time interface guards are cheap and catch real bugs early
