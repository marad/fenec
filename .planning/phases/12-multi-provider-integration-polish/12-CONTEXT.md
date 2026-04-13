# Phase 12: Multi-Provider Integration Polish - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning
**Source:** Gap closure from v1.1 milestone audit

<domain>
## Phase Boundary

Close the 3 integration gaps identified in `.planning/v1.1-MILESTONE-AUDIT.md`:

1. OpenAI adapter `chatStreaming()` never invokes `onThinking` — thinking content from `<think>` tags is stripped but not delivered to callers
2. `ProviderConfig.DefaultModel` is a dead field — parsed from TOML but no code path reads it
3. Hot-reload swaps registry providers but REPL's cached `r.provider` is stale until next `/model` or restart

After this phase, all 15 v1.1 requirements move from partial/pending to satisfied.

</domain>

<decisions>
## Implementation Decisions

### Gap 1: OpenAI streaming thinking delivery

- Parse `<think>...</think>` tags incrementally from content stream
- When content chunk arrives, check if we're inside a `<think>` tag — route to `onThinking` instead of `onToken`
- Handle tag boundaries that span chunk splits (accumulate partial tag, emit when complete)
- Also check SDK's `choice.Message.JSON.ExtraFields["reasoning_content"]` in non-streaming path (already working) and apply same pattern to streaming's accumulated message

### Gap 2: Per-provider `default_model` wiring

- In `main.go` model resolution: when `--model` is absent or has no model part, check the selected provider's `DefaultModel` field first, fall back to top-level `Config.DefaultModel`
- In REPL `/model provider` (no model) form: look up `providerRegistry.Get(providerName)` AND the provider's config `DefaultModel` (need to plumb ProviderConfig through or store defaults in registry)
- Simplest approach: extend `ProviderRegistry` to store each provider's `DefaultModel` alongside the Provider instance, add `DefaultModelFor(name) string` method

### Gap 3: Hot-reload REPL refresh

- REPL already holds `providerRegistry` after Phase 11
- Change REPL's `r.provider` from a cached field to a getter that pulls fresh from `r.providerRegistry.Get(r.activeProvider)` on each use
- Alternative: add a refresh callback that main.go's reload path invokes after `registry.Update()` to push new provider into REPL
- Recommended: convert `r.provider` to a method `r.currentProvider()` that always resolves via registry. Simpler, no callback plumbing.

### Claude's Discretion

- Exact parsing state machine for `<think>` tags across chunk boundaries
- Whether to preserve backward compatibility for REPL consumers that read `r.provider` directly (should be no external consumers — internal field)
- Whether to add `DefaultModelFor` to registry or separate accessor

</decisions>

<code_context>
## Existing Code Insights

### Files to Modify
- `internal/provider/openai/openai.go` — `chatStreaming()` at ~line 123, `extractThinkingFromContent()` helper
- `internal/provider/openai/openai_test.go` — add test for streaming thinking delivery
- `internal/config/registry.go` — add `DefaultModelFor(name) string` method, extend `Register` to accept default model
- `internal/config/toml.go` — `CreateProvider` returns both Provider and default model, or register-with-default helper
- `main.go` — update provider registration loop to pass `DefaultModel`, update `--model` resolution to check provider's default
- `internal/repl/repl.go` — convert `r.provider` cache to `r.currentProvider()` method OR add reload refresh hook

### Existing Patterns
- `ProviderRegistry` uses `sync.RWMutex` — any new methods follow the same locking pattern
- `/model` command already routes through registry — aligns with the refresh fix
- Ollama adapter delivers thinking chunks in real-time via `onThinking` — mirror that pattern

### Success Criteria (from Phase 12 roadmap goal)
1. OpenAI adapter `chatStreaming` delivers thinking content via `onThinking` callback
2. Per-provider `default_model` consulted when user gives provider without model
3. Hot-reload refresh affects REPL without requiring `/model` or restart
4. All 15 milestone requirements move from partial to satisfied in re-audit

</code_context>

<specifics>
## Specific Ideas

Example of thinking-in-stream fix:
```go
// Track whether we're inside <think> block
var inThink bool
var pending strings.Builder

for chunk := range stream {
    content := chunk.Choices[0].Delta.Content
    // State machine: detect <think> and </think> boundaries
    // Route content to onThinking if inThink, else onToken
}
```

Example of DefaultModel wiring:
```toml
[providers.ollama]
type = "ollama"
url = "http://localhost:11434"
default_model = "gemma4:latest"   # now honored
```

Example of REPL refresh approach (preferred):
```go
// Before:
func (r *REPL) sendMessage(...) {
    ... r.provider.StreamChat(...)
}

// After:
func (r *REPL) currentProvider() provider.Provider {
    return r.providerRegistry.Get(r.activeProvider)
}
func (r *REPL) sendMessage(...) {
    ... r.currentProvider().StreamChat(...)
}
```

</specifics>

<deferred>
## Deferred Ideas

None — this is a focused gap closure phase.

</deferred>
