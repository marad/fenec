# Risks, Edge Cases & Gotchas: GitHub Models API Integration

**Project:** Fenec v1.2 — Copilot Provider
**Researched:** 2026-04-14
**Method:** Live API testing against both endpoints + official docs analysis
**Overall confidence:** HIGH (findings verified with live API calls)

---

## ⛔ CRITICAL: Endpoint Deprecation In Progress

**The planned base URL `https://models.inference.ai.azure.com` is deprecated.**

The old endpoint returns these headers on every response (verified live):
```
deprecation: Fri, 17 July 2025 00:00:00 GMT
sunset: Fri, 17 Oct 2025 00:00:00 GMT
link: <https://github.blog/changelog/2025-07-17-deprecation-of-azure-endpoint-for-github-models/>; rel="deprecation"
link: <https://github.models.ai/inference>; rel="alternate"
```

**Timeline:**
- **July 17, 2025**: Officially deprecated
- **October 17, 2025**: Will stop returning valid responses
- **New endpoint**: `https://models.github.ai`

**Source:** [GitHub Blog Changelog](https://github.blog/changelog/2025-07-17-deprecation-of-azure-endpoint-for-github-models/) — *"As of July 17, 2025 usage of https://models.inference.ai.azure.com for GitHub Models inference and embeddings is officially deprecated. This change follows the launch of the GitHub Models API on May 15, 2025."*

**Recommendation:** Build against `https://models.github.ai` from day one. Do NOT use the old Azure endpoint.

**Confidence:** HIGH — verified with live API calls to both endpoints.

---

## 1. `gh` CLI Not Installed

**What happens:** `exec.Command("gh", "auth", "token")` fails with `exec: "gh": executable file not found in $PATH`.

**How to detect:**
```go
_, err := exec.LookPath("gh")
if err != nil {
    return fmt.Errorf("copilot provider requires the GitHub CLI (gh). Install: https://cli.github.com")
}
```

**Error to show:** Clear message with install link. This is the most common failure for new users.

**Edge case:** `gh` might be installed but not in PATH (e.g., installed via GUI installer on macOS but shell not reloaded). The `LookPath` check handles this correctly.

**Confidence:** HIGH — standard Go `exec` behavior.

---

## 2. `gh auth token` Failures

**Tested scenarios:**

| Scenario | Exit Code | Stderr | Token Output |
|----------|-----------|--------|--------------|
| Authenticated | 0 | (empty) | `gho_XXXX...` (OAuth token) |
| Not logged in | 1 | `not logged into any github.com account` | (empty) |
| Token expired | 0 then API returns 401 | (empty) | Stale token string |

**Key finding:** `gh auth token` returns a token with prefix `gho_` (GitHub OAuth). This is NOT a PAT — it's an OAuth token managed by `gh`.

**Edge cases:**
- **Multiple accounts:** `gh auth token` returns the active account's token. Use `gh auth token --hostname github.com` to be explicit.
- **GH_TOKEN env var:** If `GH_TOKEN` is set, `gh auth token` returns that value instead of the keyring token. This can silently use a PAT that may lack `models:read` scope.
- **Token rotation:** GitHub OAuth tokens can be refreshed/rotated by `gh`. Token obtained at init may become invalid during a long session. Consider re-fetching on 401.
- **gh installed but never used:** Exit code 1, clear error message.

**Error to show:**
```
copilot: gh CLI is not authenticated. Run: gh auth login
```

**Recommendation:** Fetch token at provider init. If API returns 401, re-fetch once before showing auth error. Store a `getToken func() (string, error)` rather than a static token string.

**Confidence:** HIGH — tested live.

---

## 3. Rate Limits

### Free Tier Rate Limits (Verified from docs)

| Tier | Metric | Copilot Free | Copilot Pro | Copilot Business | Copilot Enterprise |
|------|--------|-------------|-------------|------------------|-------------------|
| **Low** | RPM | 15 | 15 | 15 | 20 |
| | RPD | 150 | 150 | 300 | 450 |
| | Tokens/req | 8K in, 4K out | 8K in, 4K out | 8K in, 4K out | 8K in, 8K out |
| | Concurrent | 5 | 5 | 5 | 8 |
| **High** | RPM | 10 | 10 | 10 | 15 |
| | RPD | 50 | 50 | 100 | 150 |
| | Tokens/req | 8K in, 4K out | 8K in, 4K out | 8K in, 4K out | 16K in, 8K out |
| | Concurrent | 2 | 2 | 2 | 4 |
| **Custom** (o1/o3/gpt-5) | RPM | N/A | 1 | 2 | 2 |
| | RPD | N/A | 8 | 10 | 12 |

**Low tier models:** gpt-4.1-mini, gpt-4.1-nano, gpt-4o-mini, phi-4, mistral-small, ministral-3b, etc.
**High tier models:** gpt-4.1, gpt-4o, llama-3.1-405B, llama-4-scout, deepseek-v3, etc.
**Custom tier:** o1, o3, o3-mini, o4-mini, gpt-5 family, deepseek-r1, grok-3 — most have Copilot Pro minimum.

### Rate Limit Response Headers (Verified live)

```
x-ratelimit-limit-requests: 20000
x-ratelimit-remaining-requests: 19999
x-ratelimit-limit-tokens: 2000000
x-ratelimit-remaining-tokens: 1999962
x-ratelimit-renewalperiod-requests: 60
x-ratelimit-renewalperiod-tokens: 60
x-ratelimit-reset-requests: 3
x-ratelimit-reset-tokens: 1
x-ratelimit-abusepenalty-active: False
```

**Note:** The response headers show much higher limits (20K RPM) than the documented free tier. This may be because the account has Copilot Business, or because the docs describe playground-level limits while the API enforces different limits. The exact enforcement is opaque.

### 429 Handling

When rate limited, expect HTTP 429 with:
- `x-ratelimit-reset-requests` header (seconds until reset)
- `x-ratelimit-renewalperiod-requests` header (renewal window in seconds)
- No `Retry-After` header was observed (unlike standard OpenAI API)

**Recommendation:**
- Implement retry with backoff using `x-ratelimit-reset-requests` header
- The openai-go SDK already has `WithMaxRetries(2)` configured — verify it handles 429 correctly
- Show remaining requests in verbose/debug mode
- **With only 50-150 RPD on high-tier models**, users WILL hit daily limits. Show a clear message: "Daily rate limit reached. Resets at midnight UTC."

**Confidence:** HIGH for header format (verified live). MEDIUM for 429 response body format (couldn't trigger 429 in testing).

---

## 4. Model Availability by Account Type

**Key finding:** Model availability is NOT uniform. Some models require Copilot Pro or higher.

**Verified from docs:** Custom-tier models (o1, o3, gpt-5, deepseek-r1, grok-3) show "N/A" for Copilot Free. This means:
- **Copilot Free users** cannot use o1, o3, o4-mini, gpt-5 family, grok-3, deepseek-r1
- **Copilot Pro+** can access all models

**Edge case:** A user may see a model in `/v1/models` listing but get a permission error when trying to use it. The API error format for this case needs handling.

**Recommendation:**
- Don't filter the models list — show all available models
- Handle the "model not available for your plan" error gracefully
- Consider showing rate limit tier in `/model` listing (available from API)

**Confidence:** HIGH — verified from official docs rate limit table.

---

## 5. API Stability & Production Readiness

**Official position (from docs):** *"GitHub Models is designed to allow for learning, experimentation and proof-of-concept activities... and is not designed for production use cases."*

**However:** GitHub now offers paid "at-scale" usage for organizations with production-grade rate limits. The free tier is explicitly experimental.

**What this means for Fenec:**
- The API WILL work for personal/development use
- Rate limits are the main constraint, not stability
- Azure Content Safety filters are ALWAYS on and CANNOT be disabled
- The endpoint has migrated once already (Azure → github.ai) — could change again
- API is in "public preview" — breaking changes possible without warning

**Recommendation:**
- Document in Fenec that copilot provider is for personal/dev use
- Handle API changes gracefully (version the base URL in config)
- Allow users to override the base URL in config for forward-compatibility

**Confidence:** HIGH — directly from official docs.

---

## 6. Token Scopes

### OAuth Tokens (from `gh auth login`)

**Verified live:** A standard `gh auth login` token with scopes `gist, read:org, repo, workflow` successfully authenticates to the GitHub Models API. **No additional `models:read` scope is needed for OAuth tokens.**

Token format: `gho_` prefix (GitHub OAuth).

### Personal Access Tokens (PATs)

The docs explicitly state: *"Create a GitHub personal access token. The token needs to have `models:read` permissions."*

**Edge case:** If a user has `GH_TOKEN` env var set to a PAT without `models:read`, `gh auth token` will return that PAT, and API calls will fail with 401. The error message from the API is:
```json
{"error":{"code":"unauthorized","message":"Bad credentials","details":"Bad credentials"}}
```

This is indistinguishable from a completely invalid token, making debugging hard.

**Recommendation:**
- On 401 error, check if `GH_TOKEN` env var is set and suggest: "If GH_TOKEN is set, ensure it has `models:read` scope"
- Prefer `gh auth token` over manual token configuration

**Confidence:** HIGH — OAuth tested live. PAT scope requirement from official docs.

---

## 7. Error Response Format

### Error format is OpenAI-compatible with Azure extensions

**Standard errors (verified live):**
```json
{
  "error": {
    "message": "descriptive message",
    "type": "invalid_request_error",
    "param": "parameter_name",
    "code": "error_code"
  }
}
```

**Known error codes (verified):**

| HTTP | Code | Example |
|------|------|---------|
| 400 | `unknown_model` | `"Unknown model: nonexistent-xyz"` |
| 400 | `empty_array` | `"Invalid 'messages': empty array"` |
| 400 | `invalid_value` | `"max_tokens is too large: 50000"` |
| 400 | `content_filter` | Azure content safety triggered (see below) |
| 401 | `unauthorized` | `"Bad credentials"` |

### Content Filter Errors (Azure-specific, verified live)

When Azure Content Safety blocks a request, the error includes an `innererror` field not present in standard OpenAI errors:
```json
{
  "error": {
    "message": "The response was filtered due to the prompt triggering Azure OpenAI's content management policy...",
    "type": null,
    "param": "prompt",
    "code": "content_filter",
    "status": 400,
    "innererror": {
      "code": "ResponsibleAIPolicyViolation",
      "content_filter_result": {
        "violence": {"filtered": true, "severity": "medium"},
        "hate": {"filtered": false, "severity": "safe"},
        ...
      }
    }
  }
}
```

**This is an extra field.** The openai-go SDK will parse the standard `error` fields but may not expose `innererror`. The error `message` is descriptive enough for user display.

### Streaming Chunks Include Content Filter Results

Every SSE chunk includes `content_filter_results` in each choice. This is extra data not present in standard OpenAI streams:
```json
{"choices":[{"content_filter_results":{"hate":{"filtered":false,"severity":"safe"},...},"delta":{"content":"Hello"}}]}
```

**Impact:** The openai-go SDK should ignore unknown fields, but the extra data increases payload size. No action needed unless SSE parsing breaks.

**Recommendation:**
- The openai-go SDK handles the standard error fields correctly
- For content filter errors, display the `message` field — it's user-readable
- Add special handling for code `content_filter` to show a gentler message like "This request was blocked by content safety filters"

**Confidence:** HIGH — all verified with live API calls.

---

## 8. Retry Behavior

### SDK Built-in Retries

The existing openai-go provider already sets `option.WithMaxRetries(2)`. The SDK retries on:
- 429 (Rate Limit)
- 500, 502, 503, 504 (Server errors)
- Connection errors

### Rate Limit Retry Headers

From live testing, the API returns:
- `x-ratelimit-reset-requests: N` — seconds until request limit resets
- `x-ratelimit-reset-tokens: N` — seconds until token limit resets
- `x-ratelimit-renewalperiod-requests: 60` — renewal window (60s)

**No `Retry-After` header** was observed in 200 responses. The 429 response likely includes `Retry-After` but this couldn't be verified without triggering a rate limit.

**Recommendation:**
- Keep `WithMaxRetries(2)` — the SDK handles retry backoff
- For 429 specifically, parse `x-ratelimit-reset-requests` to show user: "Rate limited. Try again in N seconds."
- For daily limit exhaustion, there's no automatic retry — show: "Daily request limit reached"
- Do NOT aggressively retry — with only 50-150 RPD, each retry consumes quota

**Confidence:** MEDIUM — retry headers verified from 200 responses. 429 behavior extrapolated.

---

## 9. Context Length & Token Limits

### Critical Mismatch: Docs vs API

**The docs** state free tier limits of 8K input / 4K output tokens per request.

**The API** `/v1/models` endpoint reports much higher limits:

| Model | max_input_tokens | max_output_tokens |
|-------|-----------------|------------------|
| gpt-4.1 | 1,048,576 | 32,768 |
| gpt-4o | 131,072 | 16,384 |
| gpt-4o-mini | 131,072 | 4,096 |
| deepseek-r1 | 128,000 | 4,096 |
| llama-4-scout | 10,000,000 | 4,096 |
| gpt-5 | 200,000 | 100,000 |

**Verified live:** A request with 3,014 prompt tokens succeeded on gpt-4o-mini. A request with `max_tokens: 50000` was rejected with a clear error referencing the model limit (16,384), not the documented 4K free tier limit.

**Theory:** The docs may describe free playground limits, while API access gets the model's actual limits. OR the docs are outdated since the May 2025 API launch. OR paid-tier accounts get full model limits.

**Recommendation:**
- Use the `/v1/models` endpoint's `limits` field for `GetContextLength()` — this is accurate
- Don't hardcode context limits from the docs
- Cache model limits at startup (they don't change per-session)

**Confidence:** HIGH for API-reported limits (verified live). LOW for doc-stated limits (may be outdated or plan-dependent).

---

## 10. Known Breaking Changes & Deprecations

### Active: Endpoint Migration (CRITICAL)

| Date | Event |
|------|-------|
| May 15, 2025 | New GitHub Models API launched at `models.github.ai` |
| July 17, 2025 | Old endpoint `models.inference.ai.azure.com` deprecated |
| **October 17, 2025** | **Old endpoint will stop working** |

### API Path Structure Changed

**Old endpoint (`models.inference.ai.azure.com`):**
- `POST /chat/completions` — chat (OpenAI-compatible path)
- `GET /models` — model listing (Azure AI format: bare JSON array)

**New endpoint (`models.github.ai`):**
- `POST /inference/chat/completions` — chat
- `GET /v1/models` — model listing (OpenAI-like format: `{data: [...]}`)

**The paths are different and incompatible.** The openai-go SDK cannot use a single base URL for both endpoints on the new API.

### Model Name Format Changed

| Endpoint | Model Name Format | Example |
|----------|-------------------|---------|
| Old (`models.inference.ai.azure.com`) | Short name | `gpt-4o-mini` |
| New (`models.github.ai`) | Publisher-prefixed | `openai/gpt-4o-mini` |

**Verified:** Both short and prefixed names work for inference on the new endpoint. The `/v1/models` listing returns prefixed names.

### Upcoming Concerns

- The free tier is "public preview" — subject to change
- GitHub recently launched paid "at-scale" usage — free tier limits may tighten
- No API versioning header observed (unlike `api.github.com` which uses `X-GitHub-Api-Version`)

**Confidence:** HIGH — deprecation verified from both response headers and blog post.

---

## Architectural Implications

### Problem: openai-go SDK Base URL Mismatch

The openai-go SDK constructs URLs as `baseURL + "/chat/completions"` and `baseURL + "/models"`.

On `models.github.ai`:
- Chat completions: `POST /inference/chat/completions` → needs baseURL = `https://models.github.ai/inference`
- Model listing: `GET /v1/models` → needs baseURL = `https://models.github.ai/v1`

**These are different base URLs.** The SDK can only use one.

**Options:**
1. **Use `/inference` as base URL** for the SDK (chat works), implement model listing separately via direct HTTP to `/v1/models`
2. **Create two SDK clients** — one for inference, one for model listing
3. **Skip SDK model listing** — implement `ListModels` with a direct HTTP call to the GitHub Models `/v1/models` endpoint

**Recommendation:** Option 3. Use the SDK with baseURL `https://models.github.ai/inference` for chat. Implement `ListModels()` as a direct HTTP GET to `https://models.github.ai/v1/models` with custom JSON parsing. This also gives access to the richer model metadata (capabilities, limits, rate_limit_tier).

### Model Name Mapping

The `/v1/models` endpoint returns `openai/gpt-4o-mini` but inference accepts both `gpt-4o-mini` and `openai/gpt-4o-mini`.

**Recommendation:** List models with their full prefixed names (matches the `copilot/openai/gpt-4o-mini` pattern in Fenec). The `/` delimiter already works in Fenec's `provider/model` routing.

### Tool Calling Compatibility

**Verified live:** Tool calling works identically to OpenAI API. The response format matches:
```json
{"tool_calls": [{"function": {"name": "get_weather", "arguments": "{\"city\":\"Tokyo\"}"}, "id": "call_XXX", "type": "function"}]}
```

Not all models support tool calling. The `/v1/models` `capabilities` field indicates support:
- Models with `"tool-calling"` in capabilities: 23 of 43
- Models without: 20 of 43 (embeddings, older models, reasoning-only models)

**Edge case verified:** Meta Llama-4-Scout with tools returns empty `tool_calls: []` and empty content — it silently fails rather than erroring. The code must handle empty responses gracefully.

---

## Summary: Risk Priority Matrix

| # | Risk | Severity | Likelihood | Mitigation |
|---|------|----------|------------|------------|
| 1 | **Endpoint deprecation** — building against dead URL | 🔴 Critical | Certain | Use `models.github.ai` from day one |
| 2 | **SDK base URL mismatch** — can't use single URL for models+chat | 🔴 Critical | Certain | Separate model listing from SDK client |
| 3 | **Rate limits** — 50 RPD on high-tier models | 🟡 High | Very likely | Show clear messages, track remaining |
| 4 | **Content filter blocking** — Azure safety filters always on | 🟡 High | Likely | Handle `content_filter` error code gracefully |
| 5 | **Token expiry mid-session** | 🟡 High | Possible | Re-fetch token on 401, retry once |
| 6 | **Model requires higher Copilot plan** | 🟡 High | Likely | Handle auth/plan errors clearly |
| 7 | **`gh` not installed** | 🟢 Medium | Common | Detect at init, show install link |
| 8 | **GH_TOKEN env var overrides `gh auth token`** | 🟢 Medium | Uncommon | Hint in 401 error message |
| 9 | **Empty tool call responses** from non-OpenAI models | 🟢 Medium | Possible | Handle empty content + empty tool_calls |
| 10 | **API breaking changes** (public preview) | 🟡 High | Possible | Make base URL configurable in TOML |

---

## Appendix: Verified API Endpoints

All verified with live `curl` calls on 2026-04-14 using `gh auth token` (OAuth, scopes: gist, read:org, repo, workflow).

### New Endpoint (`models.github.ai`) — USE THIS

| Method | Path | Status | Notes |
|--------|------|--------|-------|
| GET | `/v1/models` | ✅ 200 | OpenAI-like format, 43 models, rich metadata |
| POST | `/inference/chat/completions` | ✅ 200 | Chat completions (OpenAI-compatible body) |
| POST | `/inference/chat/completions` (stream) | ✅ 200 | SSE streaming works |
| POST | `/inference/v1/chat/completions` | ✅ 200 | Also works (alternate path) |
| GET | `/inference/models` | ❌ 404 | — |
| POST | `/v1/chat/completions` | ❌ 404 | — |
| GET | `/models` | ❌ 404 | — |

### Old Endpoint (`models.inference.ai.azure.com`) — DEPRECATED

| Method | Path | Status | Notes |
|--------|------|--------|-------|
| GET | `/models` | ✅ 200 | Azure AI format (bare array), only 8 models |
| POST | `/chat/completions` | ✅ 200 | Works but returns deprecation headers |

### Error Responses Verified

| Input | HTTP | Error Code | Error Message |
|-------|------|------------|---------------|
| Invalid token | 401 | `unauthorized` | `"Bad credentials"` |
| Unknown model | 400 | `unknown_model` | `"Unknown model: X"` |
| Empty messages | 400 | `empty_array` | `"Invalid 'messages': empty array"` |
| max_tokens too high | 400 | `invalid_value` | `"max_tokens is too large: N"` |
| Content safety violation | 400 | `content_filter` | `"response was filtered due to..."` |
