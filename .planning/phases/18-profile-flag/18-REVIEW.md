---
phase: 18-profile-flag
reviewed: 2026-04-15T12:30:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - main.go
  - internal/profile/profile.go
findings:
  critical: 0
  warning: 1
  info: 0
  total: 1
status: issues_found
---

# Phase 18: Code Review Report

**Reviewed:** 2026-04-15T12:30:00Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found

## Summary

Phase 18 introduces the `--profile/-P` flag integration, allowing users to activate named profiles that specify model and system prompt configurations. The implementation adds profile loading logic to main.go and uses the existing `internal/profile` package for profile parsing.

The changes implement a three-layer precedence system:
- **Model precedence:** `--model` flag > profile > config default
- **Prompt precedence:** `--system` flag > profile > config default

The code uses `pflag.CommandLine.Changed("model")` to distinguish between explicit `--model` flag usage and profile-derived model names, which is a correct approach to prevent profile settings from being misinterpreted as flag overrides.

**Key observation:** One potential logic issue was identified related to model provider resolution when no explicit model is specified. The code quality is generally good with proper error handling and clear separation of concerns.

## Warnings

### WR-01: Profile Model Application Logic May Skip When Both Provider and Model Are Empty

**File:** `main.go:155-171`

**Issue:** The profile model/provider application logic (lines 155-171) only executes when `prof != nil && !modelExplicit` is true. However, within that block, the provider override (lines 156-167) checks `if prof.Provider != ""`, and the model override (lines 168-170) checks `if prof.ModelName != ""`. 

If a profile exists but both `prof.Provider` and `prof.ModelName` are empty (which is valid per the profile package — profiles can have empty frontmatter), then the profile loading succeeds, but neither provider nor model gets applied. This is likely the intended behavior for "prompt-only" profiles, but it's not immediately obvious from reading the code.

The more concerning scenario is if `prof.Provider != ""` but `prof.ModelName == ""`. In this case, the provider gets switched (line 165-166), but no model name is set, potentially leaving `*modelName` empty and relying on the fallback to `cfg.DefaultModel` at line 193-194.

However, there's a subtle issue: If the profile specifies a provider but no model name, and the user has not passed `--model`, then:
1. Line 166 sets `activeProviderName = prof.Provider`
2. Lines 168-170 are skipped (because `prof.ModelName == ""`)
3. Line 192-194 checks `if *modelName == "" && cfg.DefaultModel != ""` and sets `*modelName = cfg.DefaultModel`

**The issue:** The `cfg.DefaultModel` is set in the context of the original default provider, not the profile's provider. If the profile switches the provider but doesn't specify a model, the code will use the config's default model name with the profile's provider, which may not exist on that provider.

**Example scenario:**
- Config has `defaultProvider = "ollama"` and `defaultModel = "gemma4"`
- Profile specifies `provider = "openai"` but no model field
- Result: Code tries to use model "gemma4" (from config) with provider "openai", which likely doesn't have that model

**Fix:** Add validation after profile provider switching to ensure the model name is appropriate for the switched provider, or require profiles that specify a provider to also specify a model:

```go
// Apply profile's model and provider as intermediate defaults (FLAG-02, FLAG-03).
// Only when --model was NOT explicitly passed — per D-01, --model is a complete override.
modelExplicit := pflag.CommandLine.Changed("model")
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
		
		// If profile switches provider but doesn't specify model, this is likely an error
		if prof.ModelName == "" {
			fmt.Fprintf(os.Stderr, "Profile %q: provider specified but no model name given. Profiles that set a provider must also set a model.\n", *profileName)
			os.Exit(1)
		}
	}
	if prof.ModelName != "" {
		*modelName = prof.ModelName
	}
}
```

Alternatively, document this as intentional behavior if profiles are allowed to switch providers without specifying models (though this seems like an edge case that's more likely to be a user error than intentional).

---

_Reviewed: 2026-04-15T12:30:00Z_
_Reviewer: gsd-code-reviewer_
_Depth: standard_
