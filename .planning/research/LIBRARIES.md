# GitHub Models API + openai-go/v3 SDK Compatibility

**Project:** Fenec AI Assistant — GitHub Models Provider
**Researched:** 2025-07-15
**Overall Confidence:** HIGH — verified against official GitHub repos, docs, and SDK source code

---

## Executive Summary

**Yes, `openai-go/v3` works with GitHub Models API**, but with important caveats. The GitHub Models API at `https://models.github.ai/inference` is OpenAI-compatible for **chat completions, tool calling, and streaming**. The Python OpenAI SDK sample in GitHub's official `github/codespaces-models` repo confirms this pattern — `base_url="https://models.github.ai/inference"` with `api_key=GITHUB_TOKEN`.

The same approach works in Go with `option.WithBaseURL("https://models.github.ai/inference")` and `option.WithAPIKey(ghToken)`.

**Critical finding:** The endpoint has changed from the old `models.inference.ai.azure.com` to **`models.github.ai`**. The project context mentions the old URL — it must be updated.

**Model listing is NOT compatible** with the OpenAI SDK's `/models` endpoint. GitHub uses a separate catalog API at `https://models.github.ai/catalog/models` that returns a plain JSON array (not OpenAI's `{"data": [...], "object": "list"}` pagination format).

---

## 1. Base URL Format

### Answer: `https://models.github.ai/inference`

**Confidence: HIGH** — Verified from 3 official sources:
- `github/codespaces-models` Python OpenAI SDK sample: `endpoint = "https://models.github.ai/inference"`
- `github/gh-models` CLI source: `defaultInferenceRoot = "https://models.github.ai"`, `defaultInferencePath = "inference/chat/completions"`
- GitHub REST API docs curl example: `https://models.github.ai/inference/chat/completions`

### How it maps in openai-go/v3

The SDK builds URLs as `BaseURL.Parse(path)`:
- Default: `https://api.openai.com/v1/` + `chat/completions` → `https://api.openai.com/v1/chat/completions`
- GitHub Models: `https://models.github.ai/inference/` + `chat/completions` → `https://models.github.ai/inference/chat/completions` ✅

The SDK's `WithBaseURL()` auto-appends a trailing `/` if missing, so both forms work:
```go
option.WithBaseURL("https://models.github.ai/inference")   // works (SDK adds /)
option.WithBaseURL("https://models.github.ai/inference/")  // also works
```

### ⚠️ Old endpoint is DEPRECATED

The project currently references `https://models.inference.ai.azure.com`. This is the **old Azure-hosted endpoint**. GitHub has migrated to `models.github.ai`. Zero code search results found for the old endpoint on GitHub, confirming it's been superseded.

**Source:** `github/gh-models` config uses `models.github.ai` exclusively; all official documentation and samples reference `models.github.ai`.

---

## 2. Authentication

### Answer: Bearer token via `option.WithAPIKey(token)` ✅

**Confidence: HIGH** — Verified from official samples and REST API docs.

```go
// This is the correct pattern — already used in Fenec's openai provider
option.WithAPIKey(ghToken)
// SDK sends: Authorization: Bearer <ghToken>
```

### Token requirements:
- **Fine-grained PAT** with `models:read` permission (the docs say `models:read` scope)
- **`gh auth token`** also works — GitHub CLI tokens include the necessary scopes
- **GitHub Actions `GITHUB_TOKEN`** works with `permissions: models: read`

---

## 3. Custom Headers

### Answer: No custom headers required for inference; optional for full API compliance

**Confidence: HIGH** — Verified by comparing official samples.

The `github/codespaces-models` curl and Python samples do **NOT** include `X-GitHub-Api-Version` or `Accept: application/vnd.github+json` headers for inference calls. The basic curl sample only sends:
```
Content-Type: application/json
Authorization: Bearer $GITHUB_TOKEN
```

The REST API docs **recommend** (not require) additional headers:
```
Accept: application/vnd.github+json        # "recommended"
X-GitHub-Api-Version: 2026-03-10           # "optional, but required for some features"
```

The `github/gh-models` CLI sends custom Azure user-agent headers but NOT `X-GitHub-Api-Version`:
```
x-ms-useragent: github-cli-models
x-ms-user-agent: github-cli-models
```

### Recommendation for Fenec

Start with NO custom headers — the SDK's default `Content-Type: application/json` and `Authorization: Bearer TOKEN` are sufficient. Add API version header only if needed for specific features:

```go
// Only if needed later:
option.WithHeader("X-GitHub-Api-Version", "2026-03-10")
```

**NOTE:** The `Copilot-Integration-Id` header is for the Copilot Chat API (`api.githubcopilot.com`), NOT for GitHub Models. These are different APIs.

---

## 4. Model Listing

### Answer: NOT compatible with openai-go/v3 `ListAutoPaging` ❌

**Confidence: HIGH** — Verified from GitHub REST API docs and openai-go source code.

**The problem:** Two incompatibilities:

| Aspect | OpenAI SDK expects | GitHub Models provides |
|--------|-------------------|----------------------|
| **Endpoint** | `GET {baseURL}/models` → `GET .../inference/models` | `GET https://models.github.ai/catalog/models` (different path) |
| **Response format** | `{"data": [...], "object": "list"}` | Plain JSON array `[...]` |
| **Item schema** | `{"id": "...", "created": ..., "owned_by": "..."}` | `{"id": "openai/gpt-4.1", "name": "OpenAI GPT-4.1", "publisher": "OpenAI", "capabilities": [...], ...}` |

### Catalog response example (from REST API docs):
```json
[
  {
    "id": "openai/gpt-4.1",
    "name": "OpenAI GPT-4.1",
    "publisher": "OpenAI",
    "registry": "azure-openai",
    "summary": "gpt-4.1 outperforms gpt-4o across the board...",
    "capabilities": ["streaming", "tool-calling"],
    "limits": {
      "max_input_tokens": 1048576,
      "max_output_tokens": 32768
    },
    "rate_limit_tier": "high",
    "supported_input_modalities": ["text", "image", "audio"],
    "supported_output_modalities": ["text"]
  }
]
```

### Recommendation for Fenec

Implement model listing with a **direct HTTP call** to the catalog API, bypassing the openai-go SDK's `Models.ListAutoPaging()`:

```go
// Separate HTTP call to catalog
resp, _ := http.Get("https://models.github.ai/catalog/models")
var models []struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Capabilities []string `json:"capabilities"`
}
json.NewDecoder(resp.Body).Decode(&models)
```

The catalog endpoint does NOT require authentication (based on `github/gh-models` source — `ListModels` only sets `Content-Type`, not `Authorization`). However, to be safe, include the Bearer token.

**Bonus:** The catalog provides useful metadata (`capabilities`, `limits.max_input_tokens`, `rate_limit_tier`) that OpenAI's `/models` doesn't expose. This can power `GetContextLength()` properly — unlike OpenAI where it returns 0.

---

## 5. Model ID Format

### Answer: `{publisher}/{model_name}` format

**Confidence: HIGH** — Verified from docs and samples.

| Example | Publisher | Model |
|---------|-----------|-------|
| `openai/gpt-4.1` | openai | gpt-4.1 |
| `openai/gpt-4o-mini` | openai | gpt-4o-mini |
| `deepseek/deepseek-v3-0324` | deepseek | deepseek-v3-0324 |
| `ai21-labs/ai21-jamba-1.5-large` | ai21-labs | ai21-jamba-1.5-large |

This differs from the OpenAI API where models are just `gpt-4o-mini`. Fenec users will need to specify the full `publisher/model` ID.

---

## 6. Tool Calling

### Answer: Fully compatible with OpenAI format ✅

**Confidence: HIGH** — Verified from REST API docs and official Python OpenAI SDK sample.

The GitHub Models API supports the same tool calling interface as OpenAI:
- `tools` array with `type: "function"` entries
- `tool_choice`: `auto`, `required`, `none`
- Response includes `tool_calls` with `id`, `function.name`, `function.arguments`
- Tool result messages use `role: "tool"` with `tool_call_id`

The `github/codespaces-models/samples/python/openai/tools.py` sample demonstrates end-to-end tool calling using the standard OpenAI Python SDK — the exact same patterns Fenec already uses in `openai.go`.

**Fenec's existing `toOpenAITools()` and tool call response parsing will work unchanged.**

### Model capability check

Not all models support tool calling. The catalog API exposes `"capabilities": ["streaming", "tool-calling"]` per model. Fenec could use this to warn users when selecting models without tool support.

---

## 7. Streaming

### Answer: Standard SSE format, same as OpenAI ✅

**Confidence: HIGH** — Verified from REST API docs, curl samples, and `github/gh-models` SSE implementation.

- Request: `"stream": true` in request body
- Response: Server-Sent Events with `data: {...json...}` lines
- Stream termination: `data: [DONE]`
- Chunk format: `ChatCompletionChunk` with `choices[].delta.content`

The `github/gh-models` SSE reader is forked from the Azure OpenAI SDK and parses the standard SSE format. The openai-go SDK's `ssestream.Stream[ChatCompletionChunk]` uses the same format.

**Fenec's existing `chatStreaming()` method will work unchanged.**

### Note on o1 models

The `github/gh-models` CLI explicitly disables streaming for `o1`, `o1-mini`, and `o1-preview` models:
```go
if req.Model == "o1-mini" || req.Model == "o1-preview" || req.Model == "o1" {
    req.Stream = false
}
```
Fenec may want to add similar logic if supporting reasoning models.

---

## 8. Rate Limits

**Confidence: HIGH** — From official docs.

| Tier | RPM | RPD (Free) | RPD (Copilot Business) | Tokens/req |
|------|-----|-----------|----------------------|------------|
| Low | 15 | 150 | 300 | 8K in, 4K out |
| High | 10 | 50 | 100 | 16K in, 8K out |
| Embedding | 15 | 150 | 300 | 8K in, 8K out |

Rate limit headers: `x-ratelimit-timeremaining`, standard `Retry-After`

The openai-go SDK has `option.WithMaxRetries(2)` which handles transient errors, but won't auto-handle rate limits with proper backoff. Custom 429 handling may be needed.

---

## 9. Integration Pattern for Fenec

### Recommended approach: Reuse openai provider with minimal changes

The current `openai.New(baseURL, apiKey)` already supports custom base URLs. For GitHub Models:

```go
// In config/toml.go, the existing CreateProvider can handle this:
// config.toml:
// [providers.github-models]
// type = "openai"
// url = "https://models.github.ai/inference"
// api_key = "$GITHUB_TOKEN"

provider, _ := openaiProvider.New("https://models.github.ai/inference", ghToken)
```

### What needs to change

1. **Model listing** — Override `ListModels()` to call catalog API instead of `/models`
2. **Context length** — Use catalog `limits.max_input_tokens` for `GetContextLength()`
3. **Provider name** — Return `"github-models"` instead of `"openai"`
4. **Token sourcing** — Support `gh auth token` command for API key

### What works unchanged

- `StreamChat()` — both streaming and non-streaming paths ✅
- `buildParams()` — same request format ✅
- `toOpenAIMessages()` — same message format ✅
- `toOpenAITools()` — same tool calling format ✅
- `extractReasoningContent()` — works if GitHub Models proxies DeepSeek reasoning ✅
- `extractThinkingFromContent()` — works for `<think>` tag parsing ✅

### Implementation options

**Option A: Thin wrapper around openai.Provider (recommended)**
```go
type GitHubModelsProvider struct {
    *openai.Provider        // embed for chat/streaming/tools
    token     string
    catalogURL string
}

func (p *GitHubModelsProvider) Name() string { return "github-models" }
func (p *GitHubModelsProvider) ListModels(ctx context.Context) ([]string, error) {
    // Direct HTTP call to catalog API
}
func (p *GitHubModelsProvider) GetContextLength(ctx context.Context, model string) (int, error) {
    // Use catalog limits.max_input_tokens
}
```

**Option B: New provider type in config ("github-models")**
Add `case "github-models":` to `CreateProvider()` in `toml.go`.

---

## 10. Org-attributed Endpoint

For organizations with paid GitHub Models access, there's also an org-attributed endpoint:
```
POST https://models.github.ai/orgs/{org}/inference/chat/completions
```

This could be supported via a config option:
```toml
[providers.github-models]
type = "github-models"
url = "https://models.github.ai/inference"
api_key = "$GITHUB_TOKEN"
# org = "my-org"  # optional, for org-attributed billing
```

If `org` is set, base URL becomes `https://models.github.ai/orgs/{org}/inference`.

---

## Sources

| Source | Confidence | Key Info |
|--------|-----------|----------|
| [github/codespaces-models/samples/python/openai/basic.py](https://github.com/github/codespaces-models/blob/main/samples/python/openai/basic.py) | HIGH | Base URL, auth pattern with OpenAI SDK |
| [github/codespaces-models/samples/python/openai/tools.py](https://github.com/github/codespaces-models/blob/main/samples/python/openai/tools.py) | HIGH | Tool calling compatibility |
| [github/codespaces-models/samples/python/openai/streaming.py](https://github.com/github/codespaces-models/blob/main/samples/python/openai/streaming.py) | HIGH | Streaming compatibility |
| [github/codespaces-models/samples/curl/basic.sh](https://github.com/github/codespaces-models/blob/main/samples/curl/basic.sh) | HIGH | Minimal headers needed |
| [github/gh-models/internal/azuremodels/azure_client_config.go](https://github.com/github/gh-models/blob/main/internal/azuremodels/azure_client_config.go) | HIGH | Endpoint URLs |
| [github/gh-models/internal/azuremodels/azure_client.go](https://github.com/github/gh-models/blob/main/internal/azuremodels/azure_client.go) | HIGH | Auth headers, SSE streaming, org endpoints |
| [github/gh-models/internal/azuremodels/types.go](https://github.com/github/gh-models/blob/main/internal/azuremodels/types.go) | HIGH | Catalog response format |
| [GitHub REST API — Models Inference](https://docs.github.com/en/rest/models/inference) | HIGH | Full API spec, tool calling, streaming params |
| [GitHub REST API — Models Catalog](https://docs.github.com/en/rest/models/catalog) | HIGH | Catalog response format |
| [GitHub Models Quickstart](https://docs.github.com/en/github-models/quickstart) | HIGH | Endpoint migration to models.github.ai |
| openai-go/v3 SDK source (v3.31.0, local) | HIGH | URL construction, pagination format |
| [tigillo/githubmodels-go](https://github.com/tigillo/githubmodels-go) | MEDIUM | Community Go client (confirms patterns) |

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Model listing incompatibility | Medium | Implement separate catalog HTTP call |
| Rate limits (50 RPD for high-tier free) | High | Display rate limit info, graceful error handling |
| API is in public preview | Medium | Feature-flag the provider, handle endpoint changes |
| Old endpoint (`models.inference.ai.azure.com`) deprecated | Low | Use `models.github.ai` only |
| `X-GitHub-Api-Version` may become required | Low | Add header proactively |
| o1 models don't support streaming | Low | Detect and fall back to non-streaming |
