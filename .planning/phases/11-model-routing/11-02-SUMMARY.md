---
plan: 11-02
phase: 11-model-routing
status: complete
commit: be3d59a
---

# Plan 11-02 Summary: /model Listing with Provider-Grouped Display

## What was built

Implemented the `/model` no-arg listing command that discovers models from all configured providers in parallel and displays them grouped by provider, with the active model highlighted.

## Tasks completed

### Task 1: Render helpers for provider-grouped listing
- Added `providerHeaderStyle` using muted color `#6B7089` (matches existing `toolCallStyle`)
- `FormatProviderHeader(name string) string` — returns `## name` styled header
- `FormatModelEntry(name string, active bool) string` — active: `  -> name`, inactive: `     name` (spaces for alignment)
- `FormatProviderError(name string, err string) string` — muted `  (unreachable: ...)` inline
- Tests: `TestFormatProviderHeader`, `TestFormatModelEntryActive`, `TestFormatModelEntryInactive`, `TestFormatProviderError`

### Task 2: Parallel provider discovery in /model no-arg branch
- Added `providerModels` struct (name, models, err)
- Added `listModels()` helper on REPL
- Parallel discovery: `sync.WaitGroup` + goroutines, one per provider, with 5-second `context.WithTimeout`
- Results collected in pre-allocated slice indexed by provider name position
- Display iterates providers in sorted order (from `Names()`):
  - Prints provider header
  - On error: inline `FormatProviderError` and continues
  - On empty: prints `  (no models)` 
  - Otherwise: each model with `FormatModelEntry`, active flag = `providerName == r.activeProvider && model == r.conv.Model`
  - Blank line between provider sections
- Updated `handleModelCommand` no-arg branch to call `r.listModels()`

## Verification

- `go build ./...` ✓
- `go test ./internal/render/ -run "TestFormatProvider|TestFormatModelEntry"` ✓ (4 tests pass)
- `go test ./internal/repl/ -v` ✓ (all tests pass)
- `go test ./internal/repl/ ./internal/render/ -v` ✓ (all tests pass)

## Output format

```
## ollama
  -> gemma4
     llama3.2

## lmstudio
  (unreachable: connection refused)

## openai
     gpt-4o
     gpt-5
```
