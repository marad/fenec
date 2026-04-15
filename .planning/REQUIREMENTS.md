# Requirements: Fenec v1.3

**Defined:** 2025-07-18
**Core Value:** An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## v1.3 Requirements

Requirements for Profiles & Config milestone. Each maps to roadmap phases.

### Config Path

- [ ] **CFG-01**: Config directory is `~/.config/fenec` on all platforms (macOS switches from `~/Library/Application Support/fenec`)
- [ ] **CFG-02**: Existing data auto-migrates from `~/Library/Application Support/fenec` to `~/.config/fenec` on macOS first run
- [ ] **CFG-03**: User sees migration feedback message on stderr after successful migration

### Conversation

- [ ] **CONV-01**: User can type `/clear` in REPL to reset conversation to initial state
- [ ] **CONV-02**: Previous session auto-saves to named file before clear (no data loss)
- [ ] **CONV-03**: System prompt and tool descriptions preserved after clear (tools remain functional)

### Profiles

- [x] **PROF-01**: User can create profile files as markdown with `+++`-delimited TOML frontmatter in `~/.config/fenec/profiles/`
- [x] **PROF-02**: Profile TOML frontmatter supports `model` field for model/provider override (using existing `provider/model` syntax)
- [x] **PROF-03**: Profile markdown body (after frontmatter) becomes the system prompt for the session
- [ ] **PROF-04**: User can list available profiles with name and model via `fenec profile list`
- [ ] **PROF-05**: User can scaffold a new profile via `fenec profile create <name>` (opens `$EDITOR` with template)
- [ ] **PROF-06**: User can edit an existing profile via `fenec profile edit <name>` (opens `$EDITOR`)

### CLI Flags

- [ ] **FLAG-01**: `--system <file>` flag reads file and uses content as system prompt for one invocation
- [ ] **FLAG-02**: `--profile <name>` / `-P <name>` flag activates a named profile at launch (loads model + prompt)
- [ ] **FLAG-03**: `--model` flag overrides profile's model setting (priority: `--model` > profile > config default)
- [ ] **FLAG-04**: `--system` and `--profile` are composable (`--system` overrides prompt, profile's model still applies)

## Future Requirements

### Session Profiles

- **SESS-01**: User can switch profiles mid-session via `/profile <name>` REPL command
- **SESS-02**: Profile switch preserves or optionally clears conversation context

### Config Flexibility

- **CFGX-01**: `FENEC_CONFIG_DIR` environment variable overrides default config path
- **CFGX-02**: `XDG_CONFIG_HOME` respected on Linux for config path resolution

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Profile switching mid-session (`/profile`) | Needs conversation context decisions — defer to v1.4 |
| `FENEC_CONFIG_DIR` env var override | `~/.config/fenec` covers 99% of cases — add if requested |
| Profile inheritance/composition | Over-engineering for a personal tool — copy-paste suffices |
| `--system` accepting inline strings | File-based pattern preferred — reusable and consistent with `system.md` |
| Cobra migration for subcommands | pflag + `os.Args` routing sufficient for 3 subcommands |
| Automatic profile selection by context | Fragile and surprising behavior — explicit `--profile` is better |
| GUI/TUI profile editor | CLI only — `$EDITOR` integration is the right approach |
| Cloud profile sync | Personal tool, single user — users can use git/dotfiles |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CFG-01 | Phase 14 | Pending |
| CFG-02 | Phase 14 | Pending |
| CFG-03 | Phase 14 | Pending |
| CONV-01 | Phase 15 | Pending |
| CONV-02 | Phase 15 | Pending |
| CONV-03 | Phase 15 | Pending |
| PROF-01 | Phase 16 | Complete |
| PROF-02 | Phase 16 | Complete |
| PROF-03 | Phase 16 | Complete |
| FLAG-01 | Phase 17 | Pending |
| FLAG-02 | Phase 18 | Pending |
| FLAG-03 | Phase 18 | Pending |
| FLAG-04 | Phase 18 | Pending |
| PROF-04 | Phase 19 | Pending |
| PROF-05 | Phase 19 | Pending |
| PROF-06 | Phase 19 | Pending |

**Coverage:**
- v1.3 requirements: 16 total
- Mapped to phases: 16
- Unmapped: 0 ✓

---
*Requirements defined: 2025-07-18*
*Last updated: 2025-07-18 after initial definition*
