---
phase: 12-copilot-provider
plan: 01
status: complete
subsystem: provider
tags: [copilot, github-models, provider, auth]
dependency_graph:
  requires: [internal/provider/openai]
  provides: [internal/provider/copilot]
  affects: [internal/config/toml.go]
tech_stack:
  added: []
  patterns: [delegation-wrapper, injectable-functions]
key_files:
  created:
    - internal/provider/copilot/token.go
    - internal/provider/copilot/copilot.go
  modified:
    - internal/config/toml.go
decisions:
  - Copilot provider wraps openai.Provider with delegation — no duplicated API logic
  - Token resolution uses injectable functions (resolveTokenWith) for testability
  - ListModels/Ping/GetContextLength delegate to inner openai.Provider as stubs for Phase 12
metrics:
  duration: 1min
  completed: "2026-04-14T12:54:19Z"
  tasks: 2
  files: 3
---

# Phase 12 Plan 01: Copilot Provider Package Summary

JWT-free GitHub Models provider wrapping openai.Provider with automatic token resolution via GH_TOKEN, GITHUB_TOKEN, or `gh auth token` CLI.

## What Was Built

### Token Resolution (`internal/provider/copilot/token.go`)
- `resolveTokenWith()` — injectable version checking GH_TOKEN → GITHUB_TOKEN → `gh auth token`
- `resolveToken()` — production wrapper using real `exec.LookPath` and `exec.Command`
- Actionable error messages: install URL for missing gh CLI, `gh auth login` for unauthenticated state, empty token detection

### Copilot Provider (`internal/provider/copilot/copilot.go`)
- `Provider` struct wrapping `*openaiProvider.Provider` at `https://models.github.ai/inference`
- `New()` constructor — no arguments, resolves token automatically
- `Name()` returns `"copilot"`, `DefaultModel()` returns `"openai/gpt-4o-mini"`
- `StreamChat` delegates to inner provider (tool calling works identically to openai)
- `ListModels`, `Ping`, `GetContextLength` delegate as stubs (replaced in Phase 13)
- Compile-time `var _ provider.Provider = (*Provider)(nil)` check

### Config Integration (`internal/config/toml.go`)
- Added `case "copilot"` to `CreateProvider` switch calling `copilotProvider.New()`
- Added import alias `copilotProvider`
- No url or api_key fields needed — copilot provider is self-contained

## Commits

| Hash | Message |
|------|---------|
| eec951b | feat(provider): add copilot provider wrapping GitHub Models API |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

| File | Line | Stub | Resolution |
|------|------|------|------------|
| copilot.go | ListModels | Delegates to inner openai ListModels | Phase 13: catalog HTTP call |
| copilot.go | Ping | Delegates to inner openai Ping | Phase 13: catalog fetch |
| copilot.go | GetContextLength | Delegates to inner openai GetContextLength | Phase 13: catalog data |

These stubs are intentional — they provide working behavior via delegation. Phase 13 replaces them with GitHub Models catalog API calls.

## Verification

```
✓ go build ./...                                    — exit 0
✓ go vet ./internal/provider/copilot/...            — exit 0
✓ var _ provider.Provider interface check compiles
✓ All acceptance criteria grep checks passed (11/11)
```

## Self-Check: PASSED
