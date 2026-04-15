# Phase 18: Profile Flag - Context

**Gathered:** 2025-07-24
**Status:** Ready for planning

<domain>
## Phase Boundary

User can activate a named profile at launch via `--profile <name>` / `-P <name>`, loading both model and system prompt with proper flag precedence. Depends on Phase 16 (profile package) and Phase 17 (system flag).

Requirements: FLAG-02, FLAG-03, FLAG-04
</domain>

<decisions>
## Implementation Decisions

### Flag Precedence Chain
- **D-01:** Model precedence is `--model` > profile's model > `cfg.DefaultModel` > first available. `--model` is a complete override — it resets both provider and model name back to defaults, it does NOT inherit the profile's provider.
- **D-02:** Prompt precedence is `--system` > profile's SystemPrompt > `config.LoadSystemPrompt()`. Each layer completely replaces the one below it (no blending).
- **D-03:** `--system` and `--profile` compose: `--system` overrides the profile's prompt while the profile's model still applies. Example: `fenec --profile coder --system ./custom.md` uses coder's model with custom.md prompt.

### Profile Loading Behavior
- **D-04:** Profile prompt completely replaces the default `system.md` (same pattern as `--system` per Phase 17 D-03). No combining or prepending.
- **D-05:** Profile prompt is optional — if the profile has a model but an empty markdown body, fall back to config default `system.md` for the prompt. This allows model-only profiles.

### Error Handling
- **D-06:** Hard fail with clear error if `--profile` names a non-existent or unparseable profile. Same pattern as `--system` with missing file (Phase 17 D-01). User explicitly chose this profile; silent fallback would be confusing.

### Flag Design
- **D-07:** Register as `--profile` / `-P` (uppercase P) using pflag `StringP`. Lowercase `-p` is taken by `--pipe`. Uppercase `-P` follows the pattern of a distinct flag and is easy to type.

### Agent's Discretion
- Profile loading uses `profile.Load(profileDir, name)` from the Phase 16 package — integration point is in main.go
- Profile resolution should happen early, before model and prompt resolution, so profile values can feed into the existing resolution chain
- Provider handling from profile's `Provider` field follows the same `providerRegistry.Get()` pattern as `--model` provider/model splitting
</decisions>

<deferred>
## Deferred Ideas

- Profile listing command (`fenec --list-profiles` or similar) — deferred to Phase 19
- Profile creation/editing commands — deferred to Phase 19
</deferred>

<canonical_refs>
## Canonical References

No external specs or ADRs. All decisions derive from:
- `.planning/ROADMAP.md` — Phase 18 section (goal, success criteria)
- `.planning/REQUIREMENTS.md` — FLAG-02, FLAG-03, FLAG-04
- `.planning/phases/17-system-flag/17-CONTEXT.md` — Prior decisions on `--system` behavior (D-01 through D-05)
- `internal/profile/profile.go` — Profile package API (Load, Parse, List, types)
- `main.go` lines 27-33 (flag definitions), 132-170 (model resolution), 178-193 (prompt resolution)
</canonical_refs>
