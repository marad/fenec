# Phase 18: Profile Flag - Discussion Log

**Date:** 2025-07-24
**Mode:** Interactive discuss
**Duration:** ~5 minutes

## Areas Discussed

### 1. Flag Precedence Chain (selected by user)

**Gray area:** How `--model`, `--system`, `--profile`, and config defaults compose, including provider handling.

**Key question 1:** When `--profile` provides `provider/model` and `--model` provides a bare model, should `--model` inherit the profile's provider?
- **Decision:** No — `--model` is a complete override (both provider and model reset to defaults). Matches current behavior. → **D-01**

**Key question 2:** Do `--system` and `--profile` compose?
- **Decision:** Yes — `--system` overrides prompt, profile's model still applies. → **D-03**

**Key question 3:** Does profile prompt replace or combine with default `system.md`?
- **Decision:** Complete replacement. → **D-04**

**Key question 4:** What about profiles with model but no prompt body?
- **Decision:** Fall back to config default `system.md`. Allows model-only profiles. → **D-05**

**Key question 5:** Error handling for invalid profile names?
- **Decision:** Hard fail with clear error, same pattern as `--system`. → **D-06**

## Areas Not Discussed (settled by prior phases or ROADMAP)
- Error handling was briefly confirmed as part of the precedence chain discussion
- `-P` short flag availability verified (lowercase `-p` taken by `--pipe`)
- Profile package API (Phase 16) already well-defined, no ambiguity
