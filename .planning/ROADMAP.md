# Roadmap: Fenec

## Milestones

- ✅ **v1.0 Fenec Platform Foundation** — Phases 1-6 (shipped 2026-04-12)
- ✅ **v1.1 Multi-Provider Support** — Phases 7-11 (shipped 2026-04-14)
- ✅ **v1.2 GitHub Models Provider** — Phases 12-13 (shipped 2026-04-14)
- 🚧 **v1.3 Profiles & Config** — Phases 14-19 (in progress)

## Phases

<details>
<summary>v1.0 Fenec Platform Foundation (Phases 1-6) -- SHIPPED 2026-04-12</summary>

- [x] Phase 1: Foundation (3/3 plans) -- completed 2026-04-11
- [x] Phase 2: Conversation (3/3 plans) -- completed 2026-04-11
- [x] Phase 3: Tool Execution (2/2 plans) -- completed 2026-04-11
- [x] Phase 4: Lua Runtime (2/2 plans) -- completed 2026-04-11
- [x] Phase 5: Self-Extension (2/2 plans) -- completed 2026-04-11
- [x] Phase 6: File Tools (2/2 plans) -- completed 2026-04-12

See `.planning/milestones/v1.0-ROADMAP.md` for full phase details.

</details>

<details>
<summary>✅ v1.1 Multi-Provider Support (Phases 7-11) — SHIPPED 2026-04-14</summary>

- [x] Phase 7: Canonical Types (2/2 plans) — completed 2026-04-12
- [x] Phase 8: Provider Abstraction (1/1 plan) — completed 2026-04-12
- [x] Phase 9: Configuration (2/2 plans) — completed 2026-04-13
- [x] Phase 10: OpenAI-Compatible Client (2/2 plans) — completed 2026-04-13
- [x] Phase 11: Model Routing (2/2 plans) — completed 2026-04-14

See `.planning/milestones/v1.1-ROADMAP.md` for full phase details.

</details>

<details>
<summary>✅ v1.2 GitHub Models Provider (Phases 12-13) — SHIPPED 2026-04-14</summary>

- [x] Phase 12: Copilot Provider (2/2 plans) — completed 2026-04-14
- [x] Phase 13: Model Catalog (2/2 plans) — completed 2026-04-14

See `.planning/milestones/v1.2-ROADMAP.md` for full phase details.

</details>

### 🚧 v1.3 Profiles & Config (Phases 14-19)

**Milestone Goal:** Named assistant profiles with custom system prompts, models, and providers — plus config path standardization and conversation reset.

- [x] **Phase 14: Config Path Migration** - Standardize config to `~/.config/fenec` with automatic macOS migration (completed 2026-04-15)
- [x] **Phase 15: Clear Command** - `/clear` REPL command resets conversation mid-session without data loss (completed 2026-04-15)
- [x] **Phase 16: Profile Package** - Profile data model with TOML frontmatter parsing, file I/O, and validation (1 plan) (completed 2026-04-15)
- [x] **Phase 17: System Flag** - `--system <file>` flag for ad-hoc system prompt override (completed 2026-04-15)
- [x] **Phase 18: Profile Flag** - `--profile <name>` flag with model/prompt priority chain (completed 2026-04-15)
- [ ] **Phase 19: Profile Subcommands** - `fenec profile list/create/edit` CLI subcommands

## Phase Details

### Phase 14: Config Path Migration
**Goal**: Config directory lives at `~/.config/fenec` on all platforms with automatic migration from legacy macOS path
**Depends on**: Phase 13
**Requirements**: CFG-01, CFG-02, CFG-03
**Success Criteria** (what must be TRUE):
  1. Fresh install on any platform creates config at `~/.config/fenec` (not `~/Library/Application Support/fenec`)
  2. Existing macOS user's data auto-migrates from `~/Library/Application Support/fenec` to `~/.config/fenec` on first run
  3. User sees migration feedback message on stderr confirming successful migration
  4. All existing features (sessions, tools, config, system.md) work identically after migration
**Plans:** 1/1 plans complete
Plans:
- [x] 14-01-PLAN.md — ConfigDir standardization, migration logic, and main.go wiring (TDD)

### Phase 15: Clear Command
**Goal**: User can reset conversation mid-session without losing previous context or breaking REPL state
**Depends on**: Phase 14
**Requirements**: CONV-01, CONV-02, CONV-03
**Success Criteria** (what must be TRUE):
  1. User types `/clear` in REPL and conversation resets to initial state (only system prompt remains)
  2. Previous conversation auto-saves to named file before clear — no data loss
  3. System prompt and tool descriptions remain functional after clear (tools still callable)
  4. Token tracking resets — no phantom truncation on fresh conversation after clear
**Plans:** 1 plan
Plans:
- [x] 15-01-PLAN.md — /clear command with auto-save, state reset, and test coverage

### Phase 16: Profile Package
**Goal**: Profile data model and file I/O enable creating, loading, and listing named profiles with TOML frontmatter and markdown system prompts
**Depends on**: Phase 14
**Requirements**: PROF-01, PROF-02, PROF-03
**Success Criteria** (what must be TRUE):
  1. User can create a `.md` file in `~/.config/fenec/profiles/` with `+++`-delimited TOML frontmatter and markdown body
  2. Profile TOML frontmatter `model` field correctly parses `provider/model` syntax
  3. Profile markdown body (after frontmatter) is extracted as the system prompt text
  4. Profile loading handles edge cases gracefully: missing frontmatter, empty body, malformed TOML
**Plans**: 1 plan
Plans:
- [x] 16-01-PLAN.md — Profile types, TOML frontmatter parsing, file I/O (Load/List), and ProfilesDir helper

### Phase 17: System Flag
**Goal**: User can override the system prompt for a single invocation via a file path flag
**Depends on**: Phase 14
**Requirements**: FLAG-01
**Success Criteria** (what must be TRUE):
  1. `fenec --system <file>` reads file content and uses it as the system prompt for that session
  2. Tool descriptions remain functional when using `--system` override (tools still callable)
  3. Without `--system`, default system prompt behavior is unchanged
**Plans:** 1/1 plans complete
Plans:
- [x] 17-01-PLAN.md — --system/-s flag with conditional system prompt loading

### Phase 18: Profile Flag
**Goal**: User can activate a named profile at launch, loading both model and system prompt with proper flag precedence
**Depends on**: Phase 16, Phase 17
**Requirements**: FLAG-02, FLAG-03, FLAG-04
**Success Criteria** (what must be TRUE):
  1. `fenec --profile <name>` / `fenec -P <name>` loads the named profile's system prompt and model
  2. `--model` flag overrides profile's model setting (priority: `--model` > profile > config default)
  3. `--system` and `--profile` compose: `--system` overrides prompt while profile's model still applies
  4. Invalid profile name produces clear error message
**Plans:** 1/1 plans complete
Plans:
- [x] 18-01-PLAN.md — --profile/-P flag with model/prompt precedence chains

### Phase 19: Profile Subcommands
**Goal**: User can manage profiles through dedicated CLI subcommands without interfering with normal REPL operation
**Depends on**: Phase 16
**Requirements**: PROF-04, PROF-05, PROF-06
**Success Criteria** (what must be TRUE):
  1. `fenec profile list` displays available profiles with name and model
  2. `fenec profile create <name>` scaffolds a new profile and opens `$EDITOR` with template
  3. `fenec profile edit <name>` opens existing profile in `$EDITOR`
  4. Subcommands route correctly via pre-pflag `os.Args` dispatch — no interference with normal `fenec` invocation
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-11 |
| 2. Conversation | v1.0 | 3/3 | Complete | 2026-04-11 |
| 3. Tool Execution | v1.0 | 2/2 | Complete | 2026-04-11 |
| 4. Lua Runtime | v1.0 | 2/2 | Complete | 2026-04-11 |
| 5. Self-Extension | v1.0 | 2/2 | Complete | 2026-04-11 |
| 6. File Tools | v1.0 | 2/2 | Complete | 2026-04-12 |
| 7. Canonical Types | v1.1 | 2/2 | Complete | 2026-04-12 |
| 8. Provider Abstraction | v1.1 | 1/1 | Complete | 2026-04-12 |
| 9. Configuration | v1.1 | 2/2 | Complete | 2026-04-13 |
| 10. OpenAI-Compatible Client | v1.1 | 2/2 | Complete | 2026-04-13 |
| 11. Model Routing | v1.1 | 2/2 | Complete | 2026-04-14 |
| 12. Copilot Provider | v1.2 | 2/2 | Complete   | 2026-04-14 |
| 13. Model Catalog | v1.2 | 2/2 | Complete   | 2026-04-14 |
| 14. Config Path Migration | v1.3 | 1/1 | Complete    | 2026-04-15 |
| 15. Clear Command | v1.3 | 0/1 | Not started | - |
| 16. Profile Package | v1.3 | 1/1 | Complete   | 2026-04-15 |
| 17. System Flag | v1.3 | 1/1 | Complete    | 2026-04-15 |
| 18. Profile Flag | v1.3 | 1/1 | Complete    | 2026-04-15 |
| 19. Profile Subcommands | v1.3 | 0/? | Not started | - |
