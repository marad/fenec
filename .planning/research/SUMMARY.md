# Research Summary: GitHub Models Provider (v1.2)

**Project:** Fenec — Go CLI AI assistant
**Milestone:** v1.2 — `copilot` provider type targeting GitHub Models API
**Researched:** 2026-04-14 (ARCHITECTURE, RISKS, LIBRARIES) / 2026-04-12 (FEATURES)
**Overall Confidence:** HIGH — three of four files verified with live API calls

## Executive Summary

Adding a `copilot` provider to Fenec is a well-scoped integration task, not an architecture overhaul. GitHub Models API is OpenAI-compatible for every operation that matters — chat completions, tool calling, and SSE streaming — which means Fenec's existing `openai.Provider` does 80% of the work. The copilot provider is a thin wrapper that resolves a GitHub OAuth token, delegates all chat operations to the embedded openai provider, and replaces only model listing with a custom HTTP call to the GitHub Models catalog endpoint.

The single biggest technical constraint is that chat completions and model listing live on **different base URL paths** (`/inference` vs `/v1`) under `models.github.ai`, making it impossible to use one SDK client instance for both. The solution is settled: use `openai-go/v3` with `baseURL = https://models.github.ai/inference` for chat, and implement `ListModels()` as a direct `net/http` call to `https://models.github.ai/v1/models`. This is confirmed by three independent researchers and verified with live API calls.

The primary implementation risks are operational, not technical: aggressive rate limits (50 requests/day on high-tier models for free users), Azure Content Safety filters that are always-on and cannot be disabled, and `gh` CLI dependency that must fail gracefully with actionable error messages. None of these block the thin-wrapper approach, but all three require explicit error handling to avoid a poor user experience.

---

## Consensus Findings

The following are agreed upon by all researchers (or 3 of 4, with clear majority):

### The Definitive Base URLs

| Operation | URL | Method | Notes |
|-----------|-----|--------|-------|
| Chat completions | `https://models.github.ai/inference/chat/completions` | POST | SDK base URL: `https://models.github.ai/inference` |
| Streaming chat | Same URL, `"stream": true` in body | POST | Standard SSE, `data: [DONE]` terminator |
| Model listing | `https://models.github.ai/v1/models` | GET | Custom HTTP call, NOT via openai-go SDK |

**All three live-tested researchers confirmed these URLs work.** The `/inference/v1/chat/completions` alternate path also works but is not canonical.

### Authentication Flow

Token resolution priority (matches `gh` CLI ecosystem behavior, verified from `go-gh` source analysis):

1. `GH_TOKEN` env var — highest priority (CI/CD, Docker, Codespaces)
2. `GITHUB_TOKEN` env var — second priority (GitHub Actions auto-injection)
3. `gh auth token --hostname github.com` subprocess — reads system keyring

**Scope requirements:** Standard `gh auth login` OAuth tokens (scopes: `gist, read:org, repo, workflow`) are sufficient for GitHub Models API. No additional `models:read` scope is needed for OAuth tokens. PATs DO require `models:read`.

**Token lifetime:** GitHub OAuth tokens are long-lived (no automatic expiry). Fetch once at provider init — no refresh loop needed during a CLI session.

### openai-go/v3 Compatibility

| Feature | Compatible? | Notes |
|---------|-------------|-------|
| Chat completions | ✅ Yes | `option.WithBaseURL("https://models.github.ai/inference")` |
| Tool calling | ✅ Yes | Identical request/response format to OpenAI |
| SSE streaming | ✅ Yes | Standard format, works with `ssestream.Stream[ChatCompletionChunk]` |
| Model listing | ❌ No | Different path + non-standard response schema |
| `GetContextLength` | ❌ No (via SDK) | Use catalog `limits.max_input_tokens` via direct HTTP instead |

The existing `toOpenAITools()`, `toOpenAIMessages()`, `chatStreaming()`, and `extractReasoningContent()` methods in Fenec's openai provider work **unchanged** with GitHub Models.

### Model Listing: Catalog vs SDK

The `/v1/models` response uses `{"data": [...]}` structurally but individual model objects are GitHub-specific:

```json
{
  "id": "openai/gpt-4o-mini",
  "name": "OpenAI GPT-4o mini",
  "capabilities": ["streaming", "tool-calling"],
  "limits": { "max_input_tokens": 131072, "max_output_tokens": 4096 },
  "rate_limit_tier": "low"
}
```

Missing required OpenAI fields (`created`, `object`, `owned_by`) mean `openai-go` SDK's `ListAutoPaging` will fail. Implement `ListModels()` with a direct `net/http` call. **Bonus:** The `limits.max_input_tokens` field finally gives `GetContextLength()` real data to return.

### Recommended Default Model

**`openai/gpt-4o-mini`** — Low rate-limit tier (150 RPD free), available to all Copilot plan levels, supports both tool calling and streaming, widely tested. Good balance of capability and daily quota.

---

## Resolved Discrepancies

### Endpoint URL (Critical)

| Researcher | URL Referenced | Status |
|-----------|---------------|--------|
| FEATURES.md | `https://models.inference.ai.azure.com` (implicit) | ❌ Wrong — old Azure endpoint |
| ARCHITECTURE-copilot-auth.md | `https://models.github.ai` | ✅ Correct — verified live |
| LIBRARIES.md | `https://models.github.ai/inference` | ✅ Correct — verified from 3 official sources |
| RISKS.md | `https://models.github.ai` | ✅ Correct — verified live, deprecation headers confirmed |

**Resolution: Use `https://models.github.ai`.**

FEATURES.md was written for the broader v1.1 multi-provider milestone (scoped to an earlier research cycle) and references the old Azure endpoint in its context examples. The three v1.2-specific researchers all independently confirmed the endpoint migration. The old endpoint (`models.inference.ai.azure.com`) was deprecated July 17, 2025 and **shuts down October 17, 2025**. This is not a minor version difference — building against the old endpoint guarantees a hard failure within months.

The change blog post is confirmed: https://github.blog/changelog/2025-07-17-deprecation-of-azure-endpoint-for-github-models/

### Model Name Format

- **Old endpoint:** Short names, e.g., `gpt-4o-mini`
- **New endpoint:** Publisher-prefixed names, e.g., `openai/gpt-4o-mini`

Both short and publisher-prefixed names work for inference on the new endpoint, but the catalog returns publisher-prefixed IDs. Fenec should use publisher-prefixed names throughout (they compose naturally with `copilot/openai/gpt-4o-mini` in the `provider/model` routing).

### `/v1/models` Response Format

RISKS.md and LIBRARIES.md both verified this independently but describe it slightly differently. The authoritative description: the response IS `{"data": [...]}` (not a bare array), but items lack `created`, `object`, and `owned_by` fields that the openai-go SDK requires. Result: the SDK's model listing code fails; custom HTTP parsing is required.

---

## Implementation Blueprint

### Architecture Decision: Thin Wrapper

```
copilot.Provider
├── inner: *openai.Provider          // Embedded — handles StreamChat, Ping
│         baseURL: models.github.ai/inference
│         apiKey: gh token
│
└── catalog: []ghModel (cached)       // Custom — handles ListModels, GetContextLength
          fetched from: models.github.ai/v1/models
          auth: Bearer <gh token>
```

```go
// internal/provider/copilot/copilot.go

const (
    inferenceBaseURL = "https://models.github.ai/inference"
    modelsURL        = "https://models.github.ai/v1/models"
)

type Provider struct {
    inner      *openai.Provider   // delegates StreamChat, Ping
    token      string
    httpClient *http.Client
    mu         sync.RWMutex
    catalog    []ghModel          // lazy-loaded, cached for session
}

func (p *Provider) Name() string { return "copilot" }
// ListModels  → custom HTTP GET to modelsURL, parse {data:[...]}
// GetContextLength → return catalog[model].Limits.MaxInputTokens
// StreamChat  → delegate to p.inner.StreamChat (unchanged)
// Ping        → call fetchCatalog (validates auth + connectivity)
```

### Token Resolution

```go
func resolveToken() (string, error) {
    if t := os.Getenv("GH_TOKEN"); t != ""     { return t, nil }
    if t := os.Getenv("GITHUB_TOKEN"); t != "" { return t, nil }
    return tokenFromGhCLI()
}

func tokenFromGhCLI() (string, error) {
    ghPath, err := exec.LookPath("gh")
    if err != nil {
        return "", fmt.Errorf("copilot provider requires gh CLI — install from https://cli.github.com")
    }
    cmd := exec.Command(ghPath, "auth", "token", "--hostname", "github.com")
    out, err := cmd.Output()
    // ... handle exit errors with actionable messages
}
```

**Do NOT add `go-gh/v2` as a dependency** — it pulls 20+ transitive packages (survey, glamour, lipgloss, etc.) for a function achievable in ~30 lines.

### Config Integration

```toml
# ~/.config/fenec/config.toml
[providers.copilot]
type = "copilot"
# No url or api_key — auto-resolved from gh CLI or env vars
```

```go
// config/toml.go — add one case to CreateProvider:
case "copilot":
    return copilotProvider.New()   // No args needed
```

### Model Listing Implementation

```go
func (p *Provider) fetchCatalog(ctx context.Context) ([]ghModel, error) {
    // Double-checked locking for goroutine-safe lazy init
    p.mu.RLock()
    if p.catalog != nil { defer p.mu.RUnlock(); return p.catalog, nil }
    p.mu.RUnlock()

    p.mu.Lock()
    defer p.mu.Unlock()
    if p.catalog != nil { return p.catalog, nil }

    req, _ := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
    req.Header.Set("Authorization", "Bearer "+p.token)

    resp, err := p.httpClient.Do(req)
    // ... decode {"data": [...]} into []ghModel
    // cache in p.catalog
}
```

### Error Messages (User-Facing)

| Condition | Message |
|-----------|---------|
| `gh` not found | `"copilot provider requires the GitHub CLI (gh). Install: https://cli.github.com"` |
| `gh` not authenticated | `"GitHub CLI is not authenticated. Run: gh auth login"` |
| API 401 | `"GitHub token is invalid or expired. Run 'gh auth login' to refresh."` (if `GH_TOKEN` set, add: "If GH_TOKEN is set, ensure it has models:read scope") |
| API 429 | `"Rate limited. Try again in N seconds."` (parse `x-ratelimit-reset-requests` header) |
| Daily limit | `"Daily request limit reached. Resets at midnight UTC."` |
| Content filter | `"This request was blocked by content safety filters."` (code: `content_filter`) |
| Unknown model | Surface model name from error JSON (`code: unknown_model`) |

### o1/o3/reasoning Model Handling

Models like `o1`, `o1-mini`, `o3`, `o4-mini` do not support streaming. The `github/gh-models` CLI explicitly disables streaming for these:

```go
// In buildParams() or at StreamChat entry point:
if isReasoningModel(model) {
    // Disable streaming, use non-streaming path
}
```

Detection: check catalog `capabilities` array for absence of `"streaming"`, or match model name patterns.

---

## Risk Register

| # | Risk | Severity | Likelihood | Mitigation |
|---|------|----------|------------|------------|
| 1 | **Old endpoint used** — building against deprecated `models.inference.ai.azure.com` | 🔴 Critical | None if research is followed | Use `models.github.ai` exclusively. Never reference old URL in code or docs. |
| 2 | **SDK base URL mismatch** — can't use one SDK instance for both chat + model listing | 🔴 Critical | Certain without this fix | Use SDK only for `/inference` (chat). Direct HTTP for `/v1/models`. |
| 3 | **Rate limits** — 50 RPD on high-tier models (free tier) | 🟡 High | Very likely for active users | Show `x-ratelimit-remaining-requests` in debug mode. Clear messages on 429. Default to low-tier model (`gpt-4o-mini`). |
| 4 | **Azure Content Safety filters** — always-on, cannot be disabled | 🟡 High | Likely for power users | Catch `content_filter` error code explicitly. Show a human-readable message. Don't retry filtered requests. |
| 5 | **Token expiry or revocation mid-session** | 🟡 High | Possible | On 401, show: "token may have been revoked, restart fenec and run gh auth login". Do NOT auto-retry (adds complexity, harms session UX). |
| 6 | **`gh` CLI not installed** | 🟢 Medium | Common for new users | `exec.LookPath("gh")` at provider init — fail fast with install link before any API call. |
| 7 | **`GH_TOKEN` env var is a PAT without `models:read`** | 🟢 Medium | Uncommon | On 401, check if `GH_TOKEN` is set and hint about required scope. |
| 8 | **Model requires higher Copilot plan** (o1, o3, gpt-5, deepseek-r1 need Pro+) | 🟡 High | Likely for users on free tier | Don't filter catalog — show all models. Handle plan-restriction errors gracefully. |
| 9 | **Empty tool call responses** from models without `tool-calling` capability | 🟢 Medium | Possible | Check `capabilities` field; warn if selected model lacks `"tool-calling"`. Handle empty `tool_calls: []` without crashing. |
| 10 | **API breaking changes** (public preview, migrated once already) | 🟡 High | Possible | Make base URLs configurable in TOML (`url` field can override default). Document the config key. |

---

## Requirements Implications

### Must Be In Scope (v1.2)

1. **New `copilot` provider type** in `config/toml.go` — `case "copilot": return copilotProvider.New()`
2. **Token resolution** — `GH_TOKEN` → `GITHUB_TOKEN` → `gh auth token`, with actionable error messages for every failure mode
3. **Chat/streaming/tool calling via openai provider embedding** — no new implementation needed, reuse existing
4. **Custom model listing** — direct HTTP GET to `https://models.github.ai/v1/models`, parse `{"data":[...]}` with GitHub-specific schema
5. **`GetContextLength()` from catalog** — return `limits.max_input_tokens` (solves the current `return 0, nil` problem in the openai provider for all GitHub Models)
6. **`Ping()` via catalog fetch** — validates auth and connectivity in one call
7. **Content filter error handling** — catch `content_filter` error code, surface a user-friendly message
8. **Rate limit messaging** — 429 errors must show how long to wait, daily exhaustion must be distinguishable from per-minute limits
9. **`gh` not-installed and not-authenticated detection** — fail at init, not at first API call

### Should Be In Scope (adds polish, low complexity)

- **Capability warning** — warn when selected model lacks `"tool-calling"` in catalog
- **Rate limit tier in model listing** — show `rate_limit_tier` (low/high/custom) in `/model` REPL output
- **Configurable base URL** — allow `url` override in TOML for forward-compatibility if GitHub migrates endpoints again
- **Optional org attribution** — `org` field in TOML switches base URL to `https://models.github.ai/orgs/{org}/inference`

### Out of Scope (v1.2)

- **Automatic token refresh on 401** — adds complexity, not needed for long-lived OAuth tokens; handle as clear error
- **Adding `go-gh/v2` dependency** — 20+ transitive deps for 30 lines of code; implement token resolution inline
- **`X-GitHub-Api-Version` header** — not required for inference; add only if a specific feature demands it
- **Custom-tier model support (o1, o3, gpt-5)** — requires Copilot Pro+; handle gracefully but don't optimize for
- **At-scale / organization billing integration** — out of scope for personal dev tool use case
- **Catalog caching to disk** — in-memory cache per session is sufficient; disk cache adds complexity for minimal gain

---

## Open Questions

1. **Does `GetContextLength()` need a fallback when the model isn't in the catalog?** The current openai provider returns `0, nil` for unknown models. Should copilot provider do the same (return 0 as "unknown, use provider default") or error? **Recommendation:** Return `0, nil` — consistent with existing behavior, non-fatal.

2. **Should `Ping()` fail if catalog fetch fails with a non-auth error (e.g., network timeout)?** The `fetchCatalog` approach makes `Ping()` dependent on model listing availability, which may be overly strict. **Recommendation:** If catalog fetch fails on a 5xx or network error, return an error from `Ping()`. On 401, return a specific auth error.

3. **How should the `/model` REPL command display GitHub Models?** Models have a compound `publisher/model` format. When a user runs `/model` inside a copilot session, should it display `openai/gpt-4o-mini` or `gpt-4o-mini`? **Recommendation:** Display full `publisher/model` IDs — this is what inference accepts and what the catalog returns.

4. **Token caching across provider reinitializations** — If `fenec` supports hot-reload of provider config, does a new `copilot.New()` call always shell out to `gh auth token`? **Recommendation:** Yes — the subprocess costs ~50-100ms but ensures the token is always fresh. Only cache within a single `Provider` instance's lifetime.

5. **Rate limit header discrepancy** — RISKS.md noted that live API responses show 20,000 RPM in headers but docs say 10-15 RPM for free tier. This may reflect Copilot Business limits (the test account). Don't hardcode rate limit values — always read from response headers.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Endpoint URLs | HIGH | Verified with live `curl` calls on 2026-04-14. Two independent researchers confirmed. Official GitHub blog post confirms deprecation. |
| Auth flow | HIGH | Live tested. `go-gh` source analyzed for priority chain. OAuth token scopes verified empirically. |
| openai-go/v3 compatibility | HIGH | Chat/tools/streaming verified against official `github/codespaces-models` Python samples (same pattern). SDK source code analyzed for URL construction. |
| Model listing approach | HIGH | `ListAutoPaging` failure confirmed from SDK source. Catalog response format verified live. `{"data":[...]}` wrapper confirmed. |
| Rate limits | MEDIUM | Headers verified live, but observed values don't match docs (likely plan-dependent). 429 behavior not directly tested. |
| Content filter behavior | HIGH | Error format verified live with intentional policy violation. |
| Token expiry / 401 mid-session | MEDIUM | OAuth token lifetime from docs. Mid-session revocation behavior extrapolated — not directly tested. |

**Overall confidence: HIGH.** The core implementation path is proven. Remaining uncertainty is operational (rate limit tiers, plan-dependent behavior) — none of it blocks implementation.

### Gaps to Address During Implementation

- **Verify `/v1/models` response format is stable** — The catalog endpoint is the newer of the two APIs. Confirm the `{"data":[...]}` wrapper and `capabilities`/`limits` fields are present in production before relying on them.
- **Test with Copilot Free account** — All live testing was done with a Business/higher account. Free tier behavior (especially model availability filtering and rate limits) needs validation.
- **Confirm o1/o3 streaming behavior** — The `gh-models` CLI disables streaming for o1 family. Verify this is still true for the new endpoint; model capability flags in catalog should indicate this.

---

## Sources

### Primary (HIGH confidence — verified live)
- [github/gh-models source — azure_client_config.go](https://github.com/github/gh-models/blob/main/internal/azuremodels/azure_client_config.go) — canonical endpoint URLs
- [github/gh-models source — azure_client.go](https://github.com/github/gh-models/blob/main/internal/azuremodels/azure_client.go) — auth headers, SSE streaming, org endpoints
- [github/codespaces-models — Python OpenAI SDK samples](https://github.com/github/codespaces-models/tree/main/samples/python/openai) — base URL, auth, tool calling, streaming patterns
- [GitHub REST API — Models Inference](https://docs.github.com/en/rest/models/inference) — official API spec
- [GitHub REST API — Models Catalog](https://docs.github.com/en/rest/models/catalog) — catalog response format
- [GitHub Deprecation Blog](https://github.blog/changelog/2025-07-17-deprecation-of-azure-endpoint-for-github-models/) — confirmed sunset Oct 17, 2025
- openai-go/v3 SDK source (v3.31.0, local) — URL construction, pagination format, retry behavior
- Live API calls on 2026-04-14 — endpoints, error responses, rate limit headers, tool calling

### Secondary (MEDIUM confidence)
- [GitHub Models Quickstart](https://docs.github.com/en/github-models/quickstart) — endpoint migration overview
- [tigillo/githubmodels-go](https://github.com/tigillo/githubmodels-go) — community Go client confirming patterns
- go-gh/v2 source analysis — token resolution priority chain

---

*Research completed: 2026-04-14*
*Milestone: v1.2 — copilot provider*
*Ready for roadmap: yes*
