# Phase 11: Model Routing - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Unified model selection across providers. The `--model` CLI flag accepts `provider/model` or bare `modelname` forms. The `/model` REPL command shows models grouped by provider, highlights the active one, and switches providers when given `provider/model`. Auto-discovery fetches model lists from each configured provider.

</domain>

<decisions>
## Implementation Decisions

### `--model` Flag Behavior

- `--model provider/model` — split on first `/`, resolve to provider by name, pass model to it
- `--model modelname` (no prefix) — use `default_provider` from config + this modelname
- Unknown provider name in prefix → error with list of available providers
- No `--model` flag at all → use `default_provider` + `default_model` from config
- `:` is NOT a delimiter — Ollama tags like `gemma4:latest` must work

### `/model` REPL Command Behavior

- `/model` (no arg) — list models grouped by provider with subheadings:
  ```
  ## ollama
  → gemma4
    llama3.2

  ## lmstudio
    qwen3-14b

  ## openai
    gpt-4o
    gpt-5
  ```
  - `→` arrow prefix marks the active model
  - Provider subheadings in muted style for grouping
- `/model provider/model` — switch to that provider + model
  - Conversation history preserved (canonical types work across providers)
  - Show confirmation line after switch
- `/model modelname` (no prefix) — switch model within current provider (no provider change)
- Unknown provider or model → error message, no change

### Model Discovery

- Cache per-provider model list in memory during session
- Refresh when user runs `/model` with no args (fetch fresh from each provider)
- No disk cache, no TTL — simple in-memory

### Conversation Continuity

- Switching provider keeps conversation intact (canonical `model.Message` works universally)
- No reset, no warning — just works
- Tools remain registered across switches; if model doesn't support tool calling, user learns at tool-invocation time

### Claude's Discretion

- Exact wording of confirmation/error messages
- Caching struct shape
- Whether to parallelize ListModels calls across providers (recommended for latency)
- How REPL displays muted provider headers (lipgloss styling)
- Whether discovery happens eagerly at startup or lazily on first `/model`

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/registry.go` — ProviderRegistry with Names(), Get(name), Default(), SetDefault(name) methods
- `internal/provider/provider.go` — Provider.ListModels(ctx) already required by interface
- `internal/provider/ollama/ollama.go` — ListModels implementation exists
- `internal/provider/openai/openai.go` — ListModels implementation exists
- `internal/repl/repl.go` — `handleModelCommand` already exists for single-provider `/model <name>` selection

### Established Patterns
- REPL slash commands dispatched via switch statement in handleCommand
- Model selection persists in `conv.Model` field on Conversation
- ContextTracker re-fetches context length after model change
- lipgloss styles for muted text (already used for tool output)

### Integration Points
- `main.go` parses `--model` flag — needs to split provider/model
- `main.go` passes single Provider to REPL — Phase 11 needs to pass registry instead
- REPL needs access to ProviderRegistry to switch providers
- REPL's `handleModelCommand` needs upgrading to handle `provider/model` form

### New/Changed Types
- `REPL` struct gains a `*config.ProviderRegistry` field alongside or replacing its single `Provider`
- Conversation may need to track which provider produced it (for display only)

</code_context>

<specifics>
## Specific Ideas

Example `/model` output when active is `ollama/gemma4` and lmstudio is configured but offline:

```
## ollama
→ gemma4
  llama3.2
  qwen3-14b

## lmstudio
  (unreachable: connection refused)

## openai
  gpt-4o
  gpt-5
  gpt-4o-mini
```

- Unreachable providers show an inline error instead of blocking the whole listing

</specifics>

<deferred>
## Deferred Ideas

- Disk-cached model lists with TTL — keep it simple, refresh on /model
- Fuzzy model name matching — exact match only for now
- Model favorites / recent selections — future enhancement
- Per-provider model metadata (context length, capabilities) — future enhancement

</deferred>
