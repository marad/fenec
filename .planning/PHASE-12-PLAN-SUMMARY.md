# Phase 12: Copilot Provider — Plan Summary

**Goal**: Users can chat with GitHub Models using `type = "copilot"` in config, with automatic auth from `gh` CLI
**Requirements**: COPILOT-01, COPILOT-02, COPILOT-03, COPILOT-04, COPILOT-05, COPILOT-09

## What This Phase Builds

A new `internal/provider/copilot/` package that wraps the existing `openai.Provider` with GitHub-specific token resolution. The copilot provider is NOT built from scratch — it embeds `openai.New("https://models.github.ai/inference", token)` and delegates all chat/streaming/tool calling to it.

## Plans

### Plan 12-01: Provider Skeleton + Token Resolution

**Creates:**
- `internal/provider/copilot/token.go` — `resolveToken()` function implementing the priority chain:
  1. `GH_TOKEN` env var (highest priority — CI/CD, Docker, Codespaces)
  2. `GITHUB_TOKEN` env var (GitHub Actions auto-injection)
  3. `gh auth token --hostname github.com` subprocess (system keyring)
- `internal/provider/copilot/copilot.go` — Provider struct:
  - Wraps `*openai.Provider` as `inner` field
  - `New()` constructor: calls resolveToken(), creates inner openai provider with `https://models.github.ai/inference` base URL
  - `Name()` → returns `"copilot"`
  - `StreamChat()` → delegates to `p.inner.StreamChat()` (unchanged)
  - Temporary stubs for `ListModels()`, `GetContextLength()`, `Ping()` that delegate to inner (replaced in Phase 13)
  - Default model: `openai/gpt-4o-mini`

**Modifies:**
- `internal/config/toml.go` — add `case "copilot": return copilotProvider.New()` to `CreateProvider()` switch

**Key technical decisions:**
- Token fetched once at provider init (GitHub OAuth tokens are long-lived, no refresh needed)
- `exec.LookPath("gh")` to detect gh CLI presence (safe in Go 1.19+)
- `cmd.Output()` captures stdout only; stderr available via ExitError for error messages
- No `go-gh/v2` dependency — 30 lines of code vs 20+ transitive packages
- Actionable error messages:
  - gh not found: "copilot provider requires the GitHub CLI (gh). Install: https://cli.github.com"
  - gh not authenticated: "GitHub CLI is not authenticated. Run: gh auth login"

### Plan 12-02: Tests + Error Handling

**Creates:**
- `internal/provider/copilot/token_test.go` — Unit tests for resolveToken():
  - GH_TOKEN env var takes priority over GITHUB_TOKEN
  - GITHUB_TOKEN used when GH_TOKEN unset
  - Falls through to gh CLI when no env vars set
  - gh not installed → error with install URL
  - gh not authenticated (exit code 1) → error with "gh auth login"
  - gh returns empty token → error
- `internal/provider/copilot/copilot_test.go` — Unit tests for Provider:
  - Name() returns "copilot"
  - StreamChat delegates to inner openai provider
  - Provider satisfies provider.Provider interface (compile-time check)

**Key constraint:** Token resolution uses os/exec subprocess — tests must mock LookPath and Command execution. Use an interface or test helper to inject mock behavior.

## Success Criteria

1. `[providers.copilot] type = "copilot"` in config (no url/api_key) creates a working provider
2. Auth resolves via GH_TOKEN → GITHUB_TOKEN → `gh auth token` priority chain
3. Missing/unauthenticated `gh` CLI produces actionable error with remediation steps
4. Streaming chat and tool calling work identically to the openai provider

## Technical Context from Research

- Chat endpoint: `https://models.github.ai/inference` (openai-go SDK base URL)
- Auth: Bearer token via `option.WithAPIKey(token)` — standard openai-go pattern
- Token format: `gho_*` prefix (GitHub OAuth)
- `gh auth token` exit codes: 0=success, 1=general error, 4=not authenticated
- Streaming, tool calling, message format all identical to OpenAI — no adaptation needed
- Content filter errors (code: `content_filter`) are Azure-specific but harmless — SDK handles standard error fields
- No new Go dependencies needed
