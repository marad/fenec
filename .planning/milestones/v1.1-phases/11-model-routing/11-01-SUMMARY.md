---
plan: 11-01
phase: 11-model-routing
status: complete
commit: 683caa1
---

# Plan 11-01 Summary: Wire ProviderRegistry into REPL and main.go

## What was built

Wired the ProviderRegistry into main.go and the REPL to enable cross-provider model targeting via `--model provider/model` CLI flag and `/model provider/model` REPL command.

## Tasks completed

### Task 1: DefaultName() and main.go model routing
- Added `DefaultName() string` to `ProviderRegistry` (thread-safe, returns `r.defaultProvider`)
- Added 3 registry tests covering default name, update, and empty cases
- Rewrote `--model` flag handling: splits on `/` via `strings.SplitN`, looks up provider, exits with error listing available providers if not found
- Removed wasteful `ListModels` validation for `--model` flag — provider errors naturally at runtime
- Added `activeProviderName` string variable tracking current provider name in main.go
- Updated flag help text to "Model to use (provider/model or just model name)"

### Task 2: REPL registry wiring and /model switching
- Added `providerRegistry *config.ProviderRegistry` and `activeProvider string` to REPL struct
- Updated `NewREPL` signature: added `activeProvider string` and `providerRegistry *config.ProviderRegistry` params, renamed `registry` to `toolRegistry` for disambiguation
- Rewrote `handleModelCommand` to accept `args []string`:
  - With `provider/model`: switches provider via registry, updates model, context length, prompt
  - With bare `model`: switches model within current provider
  - With no args: placeholder message (listing implemented in Plan 02)
- Updated `/model` dispatch in `Run()` to pass `cmd.Args`
- Updated helpText: `/model - List models or switch: /model [provider/]name`
- Added 3 tests: `TestParseCommandModelWithProvider`, `TestParseCommandModelBare`, `TestParseCommandModelNoArgs`

## Verification

- `go build ./...` ✓
- `go test ./internal/config/ -run TestRegistryDefaultName` ✓ (3 tests pass)
- `go test ./internal/repl/ -run TestParseCommand` ✓ (all tests pass)

## Key decisions

- No ListModels validation on `--model` flag: avoids blocking startup with slow ListModels call
- Conversation history preserved across all switches (no reset)
- Context length updated from new provider on switch; tracker recalibrates naturally on next completion
- Empty `--model` with `cfg.DefaultModel` set uses config default_model
