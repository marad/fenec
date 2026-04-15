# Phase 18: Profile Flag - Research

**Researched:** 2025-07-24
**Domain:** CLI flag integration, profile loading, flag precedence in Go CLI app
**Confidence:** HIGH

## Summary

Phase 18 integrates the existing `internal/profile` package (built in Phase 16) into the CLI via a `--profile <name>` / `-P <name>` flag. The profile package already provides `Load(dir, name)` returning a `*Profile` with `Provider`, `ModelName`, and `SystemPrompt` fields. The config package provides `ProfilesDir()`. All building blocks exist — this phase wires them together in `main.go`.

The core challenge is implementing the precedence chain correctly: `--model` > profile > config default for models, and `--system` > profile > config default for prompts. These two chains must be independent so that `--system` and `--profile` compose (FLAG-04). The implementation inserts profile resolution as a new layer between config loading and the existing flag-override logic.

**Primary recommendation:** Insert profile loading after config loading but before model/prompt resolution in `main.go`. Use the profile's values as intermediate defaults that feed into the existing resolution chain, rather than restructuring the existing logic.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Model precedence is `--model` > profile's model > `cfg.DefaultModel` > first available. `--model` is a complete override — it resets both provider and model name back to defaults, it does NOT inherit the profile's provider.
- **D-02:** Prompt precedence is `--system` > profile's SystemPrompt > `config.LoadSystemPrompt()`. Each layer completely replaces the one below it (no blending).
- **D-03:** `--system` and `--profile` compose: `--system` overrides the profile's prompt while the profile's model still applies. Example: `fenec --profile coder --system ./custom.md` uses coder's model with custom.md prompt.
- **D-04:** Profile prompt completely replaces the default `system.md` (same pattern as `--system` per Phase 17 D-03). No combining or prepending.
- **D-05:** Profile prompt is optional — if the profile has a model but an empty markdown body, fall back to config default `system.md` for the prompt. This allows model-only profiles.
- **D-06:** Hard fail with clear error if `--profile` names a non-existent or unparseable profile. Same pattern as `--system` with missing file (Phase 17 D-01). User explicitly chose this profile; silent fallback would be confusing.
- **D-07:** Register as `--profile` / `-P` (uppercase P) using pflag `StringP`. Lowercase `-p` is taken by `--pipe`. Uppercase `-P` follows the pattern of a distinct flag and is easy to type.

### Agent's Discretion
- Profile loading uses `profile.Load(profileDir, name)` from the Phase 16 package — integration point is in main.go
- Profile resolution should happen early, before model and prompt resolution, so profile values can feed into the existing resolution chain
- Provider handling from profile's `Provider` field follows the same `providerRegistry.Get()` pattern as `--model` provider/model splitting

### Deferred Ideas (OUT OF SCOPE)
- Profile listing command (`fenec --list-profiles` or similar) — deferred to Phase 19
- Profile creation/editing commands — deferred to Phase 19
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FLAG-02 | `--profile <name>` / `-P <name>` flag activates a named profile at launch (loads model + prompt) | pflag `StringP` pattern established in codebase; `profile.Load()` API ready; integration point identified in `main.go` lines 27-33 (flag def) and 82+ (post-config-load) |
| FLAG-03 | `--model` flag overrides profile's model setting (priority: `--model` > profile > config default) | Existing `--model` resolution at lines 132-152 already handles provider/model splitting; profile values inject as intermediate defaults before this block |
| FLAG-04 | `--system` and `--profile` are composable (`--system` overrides prompt, profile's model still applies) | Model and prompt resolution are already separate code blocks (lines 132-174 vs 176-194); profile values inject independently into each chain |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `--profile` flag parsing | CLI (main.go) | — | pflag registration and parsing happens in main() |
| Profile file loading | Profile package | Config package | `profile.Load()` does parsing; `config.ProfilesDir()` resolves path |
| Model precedence resolution | CLI (main.go) | — | All model/provider logic lives in main.go (lines 132-174) |
| Prompt precedence resolution | CLI (main.go) | Config package | main.go orchestrates; `config.LoadSystemPrompt()` provides default |
| Error reporting | CLI (main.go) | Render package | `render.FormatError()` + `os.Exit(1)` for hard failures |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/spf13/pflag | v1.0.10 | CLI flag parsing | Already used for all flags in main.go [VERIFIED: go.mod] |
| github.com/BurntSushi/toml | v1.6.0 | TOML frontmatter parsing | Used by profile.Parse() and config package [VERIFIED: go.mod] |
| github.com/stretchr/testify | v1.11.1 | Test assertions | Used in all test files [VERIFIED: go.mod] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| internal/profile | n/a | Profile loading and parsing | `Load(dir, name)` for profile activation |
| internal/config | n/a | Config dir resolution, provider registry | `ProfilesDir()`, `ProviderRegistry.Get()` |
| internal/render | n/a | Error formatting | `FormatError()` for user-facing errors |

No new dependencies needed. All required libraries are already in go.mod. [VERIFIED: go.mod and codebase inspection]

## Architecture Patterns

### System Architecture Diagram

```
CLI Input: fenec --profile coder --model gemma4 --system ./custom.md
                    │                    │              │
                    ▼                    │              │
         ┌──────────────────┐            │              │
         │ pflag.Parse()    │            │              │
         │ profileName="coder"           │              │
         │ modelName="gemma4"            │              │
         │ systemFile="./custom.md"      │              │
         └──────────┬───────┘            │              │
                    ▼                    │              │
         ┌──────────────────┐            │              │
         │ Config Loading   │            │              │
         │ cfg.DefaultModel │            │              │
         │ cfg.DefaultProvider           │              │
         │ providerRegistry │            │              │
         └──────────┬───────┘            │              │
                    ▼                    │              │
         ┌──────────────────┐            │              │
         │ Profile Loading  │ ◄── NEW    │              │
         │ profile.Load()   │            │              │
         │ → prof.Provider  │            │              │
         │ → prof.ModelName │            │              │
         │ → prof.SystemPrompt           │              │
         └──────────┬───────┘            │              │
                    │                    │              │
            ┌───────┴──────────┐         │              │
            ▼                  ▼         │              │
   ┌─────────────────┐  ┌──────────────┐│              │
   │ Model Resolution│  │Prompt Resoln ││              │
   │                 │  │              ││              │
   │ --model set?    │  │ --system set?│◄──────────────┘
   │  YES→use --model│◄─┘  YES→use file│
   │  NO→profile?    │  │  NO→profile? │
   │   YES→use prof  │  │   YES→use it │
   │   NO→cfg default│  │   NO→system.md│
   │   NONE→list[0]  │  │              │
   └────────┬────────┘  └──────┬───────┘
            │                  │
            ▼                  ▼
   ┌──────────────────────────────────┐
   │ repl.NewREPL(provider, model,   │
   │   activeProvider, systemPrompt, │
   │   ...)                          │
   └──────────────────────────────────┘
```

### Pattern 1: Profile Loading as Intermediate Default Layer

**What:** Load profile after config but before model/prompt resolution. Profile values become intermediate defaults that existing resolution logic naturally overrides.

**When to use:** When adding a new layer to an existing precedence chain.

**Example:**
```go
// After config loading, before model resolution:
var prof *profile.Profile
if *profileName != "" {
    profileDir, err := config.ProfilesDir()
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to resolve profiles directory: %v", err)))
        os.Exit(1)
    }
    prof, err = profile.Load(profileDir, *profileName)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Profile %q: %v", *profileName, err)))
        os.Exit(1)
    }
}
```
[VERIFIED: matches `profile.Load()` API from `internal/profile/profile.go` lines 96-114]

### Pattern 2: Provider Override from Profile (follows --model pattern)

**What:** When a profile specifies `provider/model`, resolve the provider from the registry using the same `providerRegistry.Get()` pattern as `--model`.

**When to use:** Profile has a non-empty `Provider` field.

**Example:**
```go
// Apply profile's model/provider as intermediate defaults
// (before --model override check)
if prof != nil {
    if prof.Provider != "" {
        namedProvider, ok := providerRegistry.Get(prof.Provider)
        if !ok {
            fmt.Fprintf(os.Stderr, "Profile %q: provider %q not found.\n", *profileName, prof.Provider)
            os.Exit(1)
        }
        p = namedProvider
        activeProviderName = prof.Provider
    }
    if prof.ModelName != "" {
        *modelName = prof.ModelName
    }
}
```
[VERIFIED: matches `providerRegistry.Get()` API from `internal/config/registry.go` lines 41-46 and `--model` pattern from `main.go` lines 134-148]

### Pattern 3: Independent Model and Prompt Resolution Chains

**What:** Model and prompt are resolved in separate code blocks. Profile injects into each independently, allowing `--system` to override prompt while profile's model still applies.

**When to use:** Composable flag behavior (FLAG-04).

**Key insight:** The existing code already separates model resolution (lines 132-174) from prompt resolution (lines 176-194). Profile values inject into each block independently. No restructuring needed.

### Pattern 4: D-01 Complete Override Semantics for --model

**What:** Per D-01, `--model` is a complete override — it resets both provider and model name. If a profile set `provider=ollama`, `model=gemma4`, and user passes `--model gpt-4o`, the result should use the *default* provider with `gpt-4o`, NOT the profile's provider.

**When to use:** When `--model` flag is set AND a profile was loaded.

**Implementation:** Profile values apply *before* the `--model` block. The existing `--model` block at lines 132-152 already handles this correctly — when `--model` is bare (no `/`), it uses whatever `p` is (default provider). When `--model` has a `/`, it explicitly resolves the provider. In both cases, `--model` logic needs to *reset* the provider back to the default if the profile had changed it.

**Critical detail:** The current `--model` block checks `if *modelName != ""` (line 133). If profile set `*modelName` and then `--model` also sets it, we need to distinguish "user passed --model" from "profile set modelName". Use `pflag.CommandLine.Changed("model")` to detect if `--model` was explicitly passed.

### Anti-Patterns to Avoid
- **Merging/blending prompts:** D-02 says each layer completely replaces the one below. Never concatenate profile prompt with default prompt.
- **Inheriting profile's provider when --model is bare:** D-01 says `--model` is a complete override. If user says `--model gpt-4o` with `--profile coder` (which sets ollama provider), the result should use the config's default provider, not ollama.
- **Silent fallback for invalid profiles:** D-06 requires hard fail. Never silently fall back to default behavior.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Profile file parsing | Custom TOML+markdown parser | `profile.Parse()` from Phase 16 | Already handles +++, TOML decode, provider/model split |
| Profile file I/O with security | Manual filepath construction | `profile.Load()` from Phase 16 | Already has path traversal protection |
| Config dir resolution | Hardcoded paths | `config.ProfilesDir()` from Phase 16 | Platform-aware, consistent with other dirs |
| Provider lookup | Manual map access | `providerRegistry.Get()` | Thread-safe, consistent error pattern |
| Flag detection | String comparison | `pflag.CommandLine.Changed("model")` | Official pflag API for detecting explicit flag usage |

**Key insight:** All the building blocks exist. This phase is pure integration — wiring existing APIs together in `main.go`.

## Common Pitfalls

### Pitfall 1: Confusing "profile set model" with "--model flag set"
**What goes wrong:** After profile sets `*modelName`, the existing `--model` block at line 133 sees `*modelName != ""` and enters the model override path — even though `--model` wasn't explicitly passed.
**Why it happens:** Profile values are injected into the same `*modelName` variable used by `--model` flag.
**How to avoid:** Use `pflag.CommandLine.Changed("model")` to detect if `--model` was explicitly passed by the user. Only enter the `--model` override path when the flag was explicitly set.
**Warning signs:** `fenec --profile coder` (where coder sets model=ollama/gemma4) acts differently from expected because it enters the `--model` branch.

### Pitfall 2: Provider leaking from profile to --model override
**What goes wrong:** Profile sets provider to "ollama". User passes `--model gpt-4o` (bare, no provider prefix). The `--model` block sees `p` is already ollama from the profile and keeps it — but user expected default provider behavior.
**Why it happens:** D-01 says `--model` is a complete override that resets provider. But the profile already changed `p` and `activeProviderName`.
**How to avoid:** When `--model` is explicitly set (via Changed check), reset `p` and `activeProviderName` back to config defaults before applying `--model` logic. Or: apply profile model/provider only when `--model` was NOT explicitly passed.
**Warning signs:** `fenec --profile coder --model gpt-4o` uses ollama provider instead of default.

### Pitfall 3: Empty profile prompt treated as override
**What goes wrong:** Profile with empty body (model-only profile) sets systemPrompt to "" which replaces the default system prompt, resulting in no system prompt.
**Why it happens:** D-05 says empty body should fall back to config default. But naive implementation checks `if prof.SystemPrompt != ""` vs. checking profile existence alone.
**How to avoid:** Only override system prompt when `prof.SystemPrompt != ""`. An empty body means "no prompt opinion" — fall through to config default.
**Warning signs:** `fenec --profile minimal` (model-only profile) has no system prompt.

### Pitfall 4: Profile loading before ProfilesDir is resolved
**What goes wrong:** `config.ProfilesDir()` could fail if config dir isn't set up.
**Why it happens:** Config dir creation happens lazily in some paths.
**How to avoid:** Call `config.ProfilesDir()` after config loading (which ensures config dir exists). ProfilesDir returns the path without creating it — the directory may not exist, which is fine until `profile.Load()` tries to read from it.
**Warning signs:** Cryptic error about missing directory instead of "profile not found".

### Pitfall 5: Help text not updated
**What goes wrong:** User doesn't know `--profile` / `-P` exists because `pflag.Usage` and help examples don't mention it.
**Why it happens:** Forgetting to update the custom Usage function at lines 35-48.
**How to avoid:** Add `--profile` to both the usage examples and the Flags section.
**Warning signs:** Running `fenec --help` doesn't show the profile flag.

## Code Examples

### Flag Registration (matches existing pattern)
```go
// Source: main.go existing pattern (lines 27-33)
profileName := pflag.StringP("profile", "P", "", "Activate a named profile (loads model + prompt)")
```
[VERIFIED: follows exact `pflag.StringP` pattern from main.go lines 27-33]

### Profile Loading and Error Handling
```go
// Source: profile.Load() API from internal/profile/profile.go lines 96-114
// Source: error pattern from main.go lines 178-184 (--system error handling)
var prof *profile.Profile
if *profileName != "" {
    profileDir, err := config.ProfilesDir()
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to resolve profiles directory: %v", err)))
        os.Exit(1)
    }
    prof, err = profile.Load(profileDir, *profileName)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Profile %q: %v", *profileName, err)))
        os.Exit(1)
    }
}
```

### Model Precedence with --model Detection
```go
// Source: pflag.Changed API from github.com/spf13/pflag
// Detect if --model was explicitly passed (vs. set by profile)
modelExplicit := pflag.CommandLine.Changed("model")
```
[VERIFIED: pflag.FlagSet.Changed() method exists in spf13/pflag v1.0.10 — standard API for detecting explicit flag usage]

### Profile Model/Provider Application
```go
// Apply profile model/provider ONLY if --model was not explicitly passed
if prof != nil && !modelExplicit {
    if prof.Provider != "" {
        namedProvider, ok := providerRegistry.Get(prof.Provider)
        if !ok {
            fmt.Fprintf(os.Stderr, "Profile %q: provider %q not found. Available providers:\n", *profileName, prof.Provider)
            for _, n := range providerRegistry.Names() {
                fmt.Fprintf(os.Stderr, "  - %s\n", n)
            }
            os.Exit(1)
        }
        p = namedProvider
        activeProviderName = prof.Provider
    }
    if prof.ModelName != "" {
        *modelName = prof.ModelName
    }
}
```

### Profile Prompt Application (independent from model)
```go
// Prompt precedence: --system > profile > config default
var systemPrompt string
if *systemFile != "" {
    // --system flag: highest priority (existing code)
    data, err := os.ReadFile(*systemFile)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to read system prompt file: %v", err)))
        os.Exit(1)
    }
    systemPrompt = string(data)
} else if prof != nil && prof.SystemPrompt != "" {
    // Profile prompt: middle priority (NEW)
    systemPrompt = prof.SystemPrompt
} else {
    // Config default: lowest priority (existing code)
    var err error
    systemPrompt, err = config.LoadSystemPrompt()
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to load system prompt: %v", err)))
        os.Exit(1)
    }
}
```

### Usage Text Update
```go
// Source: main.go lines 35-48 (existing Usage function)
pflag.Usage = func() {
    fmt.Fprintf(os.Stderr, `fenec - AI assistant powered by local Ollama models

Usage:
  fenec                    Start interactive chat
  fenec --model gemma4     Use a specific model
  fenec --profile coder    Activate a named profile
  echo "prompt" | fenec    Send piped input to model
  fenec --yolo             Auto-approve all tool commands
  fenec --system prompt.md  Use a custom system prompt

Flags:
`)
    pflag.PrintDefaults()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Config-only model selection | `--model` flag + config default | Phase 12/Quick task | Flag overrides config |
| Config-only system prompt | `--system` flag + config default | Phase 17 | Flag overrides config |
| Two-layer precedence (flag > config) | Three-layer precedence (flag > profile > config) | Phase 18 (this phase) | Profile adds middle layer |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pflag.CommandLine.Changed("model")` returns true only when `--model` was explicitly passed by user | Code Examples | If Changed() doesn't work as expected, model precedence breaks — would need alternative detection method |

**Note:** A1 is standard pflag API behavior but tagged ASSUMED because it wasn't runtime-verified in this session. The API is well-documented in spf13/pflag.

## Open Questions

1. **Should `--profile` with `--model provider/model` syntax override both provider AND model from profile?**
   - What we know: D-01 says `--model` is a complete override. The existing `--model` block already handles `provider/model` splitting.
   - What's unclear: Nothing — D-01 is clear that `--model` resets both. The existing code handles this.
   - Recommendation: No action needed. Existing `--model` block handles this case.

2. **Should profile loading happen before or after provider health check?**
   - What we know: Health check is at lines 154-162. Profile may change the active provider.
   - What's unclear: If profile changes the provider, the health check needs to ping the profile's provider, not the config default.
   - Recommendation: Profile loading should happen BEFORE the health check so the correct provider is pinged. This means reordering: config → profile → model flag → health check → model list → prompt.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go standard `go test`) |
| Quick run command | `go test ./internal/profile/ -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FLAG-02 | `--profile` loads named profile's model and prompt | integration | `go test ./internal/profile/ -run TestLoad -count=1 -v` | ✅ (Load tests exist in profile_test.go) |
| FLAG-02 | Invalid profile name produces error | unit | `go test ./internal/profile/ -run "TestLoad.*Traversal\|TestLoadNon" -count=1 -v` | ✅ (error path tests exist) |
| FLAG-03 | Model precedence chain works correctly | integration | Manual / new test needed | ❌ Wave 0 — needs integration test in main_test.go or dedicated test |
| FLAG-04 | `--system` + `--profile` compose correctly | integration | Manual / new test needed | ❌ Wave 0 — needs integration test |

### Sampling Rate
- **Per task commit:** `go test ./internal/profile/ -count=1 && go build .`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] Integration test for flag precedence (model: `--model` > profile > config)
- [ ] Integration test for prompt composability (`--system` + `--profile`)
- [ ] Integration test for invalid profile error

**Note:** Since all changes are in `main.go` (which has no unit tests currently), and the profile package already has comprehensive tests, the most practical validation is: (1) `go build .` compiles, (2) profile package tests pass, (3) manual smoke test of flag combinations, and (4) targeted new tests for precedence logic if extracted into a testable function.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | `profile.Load()` validates name against path traversal (rejects `/`, `\`, `.`) [VERIFIED: profile.go line 97] |
| V6 Cryptography | no | — |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via profile name | Elevation of Privilege | `profile.Load()` rejects names with `/`, `\`, `.` — already implemented in Phase 16 |
| Large/malformed profile file | Denial of Service | BurntSushi/toml handles gracefully; profile files are user-owned |

No new security concerns introduced. All path traversal protection is already in `profile.Load()`. [VERIFIED: profile.go lines 97-99]

## Sources

### Primary (HIGH confidence)
- `internal/profile/profile.go` — Profile package API: Parse, Load, List, types [VERIFIED: direct code inspection]
- `internal/profile/profile_test.go` — Comprehensive test coverage [VERIFIED: direct code inspection, all tests pass]
- `internal/config/config.go` — ProfilesDir(), LoadSystemPrompt(), ConfigDir() [VERIFIED: direct code inspection]
- `internal/config/registry.go` — ProviderRegistry.Get(), Default(), Names() [VERIFIED: direct code inspection]
- `main.go` — Flag definitions, model resolution, prompt resolution [VERIFIED: direct code inspection]
- `.planning/phases/18-profile-flag/18-CONTEXT.md` — Locked decisions D-01 through D-07 [VERIFIED: direct file read]
- `go.mod` — All dependency versions confirmed [VERIFIED: direct file read]

### Secondary (MEDIUM confidence)
- `pflag.CommandLine.Changed()` — spf13/pflag standard API for detecting explicit flag usage [ASSUMED: standard documented API, not runtime-verified]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod, all APIs verified by code inspection
- Architecture: HIGH — integration point clearly identified, existing patterns well-understood
- Pitfalls: HIGH — each pitfall derived from reading the actual code flow and CONTEXT.md decisions

**Research date:** 2025-07-24
**Valid until:** 2025-08-24 (stable — Go CLI, no external API changes expected)
