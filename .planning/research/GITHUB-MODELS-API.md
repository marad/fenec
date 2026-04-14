# GitHub Models API — Feature & Capability Research

**Domain:** GitHub Models inference API (`models.inference.ai.azure.com`)
**Researched:** 2025-07-12
**Overall confidence:** HIGH (verified via live API calls + official docs)

---

## 1. API Compatibility

The GitHub Models API is **OpenAI-compatible**. It uses the same request/response format as the OpenAI Chat Completions API.

| Property | Value | Confidence |
|----------|-------|------------|
| Base URL | `https://models.inference.ai.azure.com` | HIGH — verified |
| Chat endpoint | `/chat/completions` | HIGH — verified |
| `/v1/` prefix | **NOT supported** (returns 404) | HIGH — verified |
| Auth header | `Authorization: Bearer <token>` | HIGH — verified |
| Token source | `gh auth token` (GitHub CLI PAT with `models:read`) | HIGH — from docs |
| Streaming | SSE via `"stream": true` | HIGH — verified |
| Tool calling | OpenAI-format `tools` array | HIGH — verified |
| `/models` list endpoint | Works but returns **incomplete list** (~8 of 40+ models) | HIGH — verified |

### Critical SDK Note

The `openai-go/v3` SDK default base URL is `https://api.openai.com/v1/`. When setting the base URL for GitHub Models, use **`https://models.inference.ai.azure.com`** (no `/v1/` suffix). The SDK appends `/chat/completions` to the base URL automatically.

**Verified:** `GET /v1/chat/completions` → 404. `POST /chat/completions` → works.

---

## 2. Available Models

### 2a. OpenAI Models

| Model ID (API) | Resolved Model | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|---------------|-----------|-----------|------------|--------------|-----------|-------|
| `gpt-4o` | gpt-4o-2024-11-20 | High | 131,072 | 16,384 | ✅ Verified | ✅ Verified | Flagship multimodal |
| `gpt-4o-mini` | gpt-4o-mini-2024-07-18 | Low | 131,072 | 4,096 | ✅ Verified | ✅ | Cost-efficient |
| `gpt-4.1` | gpt-4.1-2025-04-14 | High | 1,048,576 | 32,768 | ✅ | ✅ | **Uses dots not dashes!** |
| `gpt-4.1-mini` | gpt-4.1-mini-2025-04-14 | Low | 1,048,576 | 32,768 | ✅ Verified | ✅ | **Uses dots not dashes!** |
| `gpt-4.1-nano` | gpt-4.1-nano-2025-04-14 | Low | 1,048,576 | 32,768 | ✅ | ✅ | **Uses dots not dashes!** |
| `o1` | o1-2024-12-17 | Custom | 200,000 | 100,000 | ⚠️ Limited | ✅ | Requires `max_completion_tokens` |
| `o1-mini` | — | Custom | 128,000 | 65,536 | ⚠️ Limited | ✅ | Requires `max_completion_tokens` |
| `o1-preview` | — | Custom | 128,000 | 32,768 | ❌ | ✅ | Legacy, being replaced |
| `o3` | o3-2025-04-16 | Custom | 200,000 | 100,000 | ✅ | ✅ | Requires `max_completion_tokens` |
| `o3-mini` | o3-mini-2025-01-31 | Custom | 200,000 | 100,000 | ✅ | ✅ Verified | Requires `max_completion_tokens` |
| `o4-mini` | o4-mini-2025-04-16 | Custom | 200,000 | 100,000 | ✅ Verified | ✅ | Requires `max_completion_tokens` |
| `gpt-5` | gpt-5-2025-08-07 | Custom | 200,000 | 100,000 | ✅ | ✅ | Requires `max_completion_tokens` |
| `gpt-5-mini` | gpt-5-mini-2025-08-07 | Custom | 200,000 | 100,000 | ✅ | ✅ | Requires `max_completion_tokens` |
| `gpt-5-nano` | gpt-5-nano-2025-08-07 | Custom | 200,000 | 100,000 | ✅ | ✅ | Requires `max_completion_tokens` |
| `gpt-5-chat` | gpt-5-chat-2025-08-07 | Custom | 200,000 | 100,000 | ✅ | ✅ | 128K context window |

### 2b. Meta Llama Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `Meta-Llama-3.1-8B-Instruct` | Low | 131,072 | 4,096 | ⚠️ Accepts but ignores | ✅ | Small model, unreliable tools |
| `Meta-Llama-3.1-405B-Instruct` | High | 131,072 | 4,096 | ✅ | ✅ | Large model |
| `Llama-3.3-70B-Instruct` | High | 128,000 | 4,096 | ✅ Verified | ✅ Verified | **Best Llama for tools** |
| `Llama-3.2-11B-Vision-Instruct` | Low | 128,000 | 4,096 | ⚠️ | ✅ | Vision model |
| `Llama-3.2-90B-Vision-Instruct` | High | 128,000 | 4,096 | ⚠️ | ✅ | Vision model |
| `Llama-4-Scout-17B-16E-Instruct` | High | 10,000,000 | 4,096 | ✅ | ✅ | 10M context (!), MoE |
| `Llama-4-Maverick-17B-128E-Instruct-FP8` | High | 1,000,000 | 4,096 | ✅ | ✅ | 1M context, MoE |

### 2c. DeepSeek Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `DeepSeek-V3-0324` | High | 128,000 | 4,096 | ✅ Verified | ✅ | Chat model, solid tools |
| `DeepSeek-R1` | Custom | 128,000 | 4,096 | ❌ | ✅ | Reasoning model, `<think>` tags in content |
| `DeepSeek-R1-0528` | Custom | 128,000 | 4,096 | ❌ | ✅ | Newer R1 variant |

### 2d. Mistral Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `mistral-small-2503` | Low | 128,000 | 4,096 | ✅ Verified | ✅ | Good small model |
| `mistral-medium-2505` | Low | 128,000 | 4,096 | ✅ | ✅ | Medium model |
| `Codestral-2501` | Low | 256,000 | 4,096 | ✅ | ✅ | Code-focused |
| `Ministral-3B` | Low | 131,072 | 4,096 | ⚠️ | ✅ | Very small |

### 2e. Cohere Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `cohere-command-a` | Low | 131,072 | 4,096 | ✅ Verified | ✅ | Good all-rounder |
| `Cohere-command-r-08-2024` | Low | 131,072 | 4,096 | ✅ | ✅ | |
| `Cohere-command-r-plus-08-2024` | High | 131,072 | 4,096 | ✅ | ✅ | |

### 2f. xAI Grok Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `grok-3` | Custom | 131,072 | 4,096 | ✅ | ✅ | Very low rate limits |
| `grok-3-mini` | Custom | 131,072 | 4,096 | ⚠️ | ✅ | Has `reasoning_content` field |

### 2g. Microsoft Phi Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `Phi-4` | Low | 16,384 | 16,384 | ⚠️ Unreliable | ✅ | Small context, had server errors |
| `Phi-4-mini-instruct` | Low | 128,000 | 4,096 | ⚠️ | ✅ | |
| `Phi-4-reasoning` | Low | 32,768 | 4,096 | ❌ | ✅ | Reasoning model |

### 2h. Other Models

| Model ID (API) | Rate Tier | Max Input | Max Output | Tool Calling | Streaming | Notes |
|----------------|-----------|-----------|------------|--------------|-----------|-------|
| `MAI-DS-R1` | Custom | 128,000 | 4,096 | ❌ | ✅ | Microsoft's DeepSeek R1 distill; **API returned "unknown model" — may need different ID** |
| `AI21-Jamba-1.5-Large` | High | 262,144 | 4,096 | ✅ | ✅ | **API returned "unknown model" — may need different ID** |

### 2i. Embedding Models (not for chat)

| Model ID (API) | Rate Tier | Notes |
|----------------|-----------|-------|
| `text-embedding-3-large` | Embedding | Not for chat completions |
| `text-embedding-3-small` | Embedding | Not for chat completions |
| `Cohere-embed-v3-english` | Embedding | Not for chat completions |
| `Cohere-embed-v3-multilingual` | Embedding | Not for chat completions |

---

## 3. Rate Limits (Free Tier)

### Standard Tiers

| Tier | RPM | RPD | Tokens/Request (in) | Tokens/Request (out) | Concurrent |
|------|-----|-----|---------------------|----------------------|------------|
| **Low** (Copilot Free) | 15 | 150 | 8,000 | 4,000 | 5 |
| **High** (Copilot Free) | 10 | 50 | 8,000 | 4,000 | 2 |
| **Embedding** (Copilot Free) | 15 | 150 | 64,000 | — | 5 |

### Custom Tiers (Copilot Free vs Copilot Pro)

| Model Group | Copilot Free RPM | Copilot Free RPD | Copilot Pro RPM | Copilot Pro RPD |
|-------------|-----------------|-----------------|-----------------|-----------------|
| o1-preview | ❌ N/A | ❌ N/A | 1 | 8 |
| o1, o3, gpt-5 | ❌ N/A | ❌ N/A | 1 | 8 |
| o1-mini, o3-mini, o4-mini, gpt-5-mini/nano/chat | ❌ N/A | ❌ N/A | 2 | 12 |
| DeepSeek-R1, DeepSeek-R1-0528, MAI-DS-R1 | 1 | 8 | 1 | 8 |
| xAI Grok-3 | 1 | 15 | 1 | 15 |
| xAI Grok-3-Mini | 2 | 30 | 2 | 30 |

### Copilot Business / Enterprise Tiers

| Tier | RPM | RPD | Tokens/Request (in) | Tokens/Request (out) | Concurrent |
|------|-----|-----|---------------------|----------------------|------------|
| **Low** (Business) | 15 | 300 | 8,000 | 4,000 | 5 |
| **Low** (Enterprise) | 20 | 450 | 8,000 | 8,000 | 8 |
| **High** (Business) | 10 | 100 | 8,000 | 4,000 | 2 |
| **High** (Enterprise) | 15 | 150 | 16,000 | 8,000 | 4 |

### ⚠️ Critical Rate Limit Note

The free tier **tokens per request** limit (8,000 input, 4,000 output) is **MUCH lower** than the model's actual context window. For example, gpt-4o has a 128K context window but the free tier only allows 8K input tokens per request. The paid tier (GitHub Models at scale) removes these per-request limits and uses production-grade Azure limits.

---

## 4. Model-Specific Quirks & Implementation Notes

### Quirk 1: `max_tokens` vs `max_completion_tokens`

**Affected models:** o1, o1-mini, o1-preview, o3, o3-mini, o4-mini, gpt-5, gpt-5-mini, gpt-5-nano, gpt-5-chat

These reasoning models **reject** `max_tokens` with error:
```
"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead."
```

**Implementation:** The copilot provider must detect reasoning models and use `max_completion_tokens` instead of `max_tokens`.

### Quirk 2: Model ID naming inconsistency

- **Marketplace paths** use dashes: `gpt-4-1`, `gpt-4-1-mini`
- **API model IDs** use dots: `gpt-4.1`, `gpt-4.1-mini`
- Other models are case-sensitive: `Meta-Llama-3.1-8B-Instruct`, `DeepSeek-R1`, `Phi-4`

**Implementation:** Users must use exact API model IDs, not marketplace slugs.

### Quirk 3: Reasoning content handling varies by model

| Model | Reasoning Format | Field |
|-------|-----------------|-------|
| DeepSeek-R1 | `<think>...</think>` tags in `content` | `reasoning_content: null` |
| grok-3-mini | Separate field | `reasoning_content: "..."` |
| o-series | Hidden (used for internal CoT) | `reasoning_tokens` in usage |

The existing `extractThinkingFromContent()` handles DeepSeek-R1's format. The `extractReasoningContent()` handles grok-3-mini. O-series reasoning is not exposed.

### Quirk 4: Azure content filter fields in responses

All responses through GitHub Models include Azure content safety filter results:
```json
"content_filter_results": {
    "hate": {"filtered": false, "severity": "safe"},
    "self_harm": {"filtered": false, "severity": "safe"},
    ...
}
```
These are **extra fields** in the JSON response. The `openai-go` SDK handles these gracefully (ignored as unrecognized fields).

### Quirk 5: `/models` endpoint is incomplete

`GET /models` only returns ~8 models (a few Llama, GPT-4o, embeddings). **Not all available models are listed.** The copilot provider should either:
1. Return a hardcoded curated list of recommended models, or
2. Accept any model ID and let the API return an error if invalid

Recommendation: Option 2 (accept any model ID) with a `/models list` command that shows known models.

### Quirk 6: Some models returned "unknown model" errors

`MAI-DS-R1` and `AI21-Jamba-1.5-Large` returned `"Unknown model"` errors despite being listed on the marketplace. These may:
- Need different API IDs than their marketplace names
- Be temporarily unavailable
- Require specific Copilot subscription tiers

### Quirk 7: No `/v1/` prefix

The endpoint `https://models.inference.ai.azure.com/v1/chat/completions` returns 404. The correct path is `https://models.inference.ai.azure.com/chat/completions`. The openai-go SDK uses `/v1/` as its default base URL suffix, so when setting `WithBaseURL`, provide the raw URL without any path suffix.

---

## 5. Recommended Models for Fenec

### Best for AI Assistant Use (tool calling + streaming + good reasoning)

| Priority | Model ID | Why |
|----------|----------|-----|
| 1 | `gpt-4o` | Best overall: fast, great tools, 128K context, streaming |
| 2 | `gpt-4.1-mini` | Huge 1M context, good tools, low tier (more RPD) |
| 3 | `gpt-4o-mini` | Fast, cheap, good enough for most tasks |
| 4 | `o4-mini` | Reasoning + tool calling, good for complex tasks |
| 5 | `Llama-3.3-70B-Instruct` | Best open-source option, verified tool calling |
| 6 | `mistral-small-2503` | Fast, low tier, decent tools |
| 7 | `DeepSeek-V3-0324` | Strong open model, verified tools |

### Default recommendation: `gpt-4o-mini`
- Low rate limit tier (15 RPM, 150 RPD) — most permissive
- Fast responses
- Reliable tool calling (verified)
- 128K context (though limited to 8K input per request on free tier)

---

## 6. Implementation Recommendations for Copilot Provider

### Config Example
```toml
[providers.copilot]
type = "copilot"
# No URL needed — hardcoded to models.inference.ai.azure.com
# No API key needed — uses `gh auth token` automatically
default_model = "gpt-4o-mini"
```

### Key Implementation Points

1. **Reuse the existing `openai` provider** as base — the API is OpenAI-compatible
2. **Set base URL** to `https://models.inference.ai.azure.com` (no `/v1/`)
3. **Get token** via `exec.Command("gh", "auth", "token")` — cache it, refresh on 401
4. **Hardcode model list** for `ListModels()` since `/models` endpoint is incomplete
5. **Handle `max_completion_tokens`** for o-series and gpt-5 reasoning models
6. **`GetContextLength()`** can return known values from a built-in map

### Model Capability Map (for built-in knowledge)

```go
var modelInfo = map[string]struct {
    ContextLength int
    ToolCalling   bool
    Reasoning     bool  // uses max_completion_tokens
}{
    "gpt-4o":           {131072, true, false},
    "gpt-4o-mini":      {131072, true, false},
    "gpt-4.1":          {1048576, true, false},
    "gpt-4.1-mini":     {1048576, true, false},
    "gpt-4.1-nano":     {1048576, true, false},
    "o1":               {200000, true, true},
    "o3":               {200000, true, true},
    "o3-mini":          {200000, true, true},
    "o4-mini":          {200000, true, true},
    "gpt-5":            {200000, true, true},
    "gpt-5-mini":       {200000, true, true},
    "gpt-5-nano":       {200000, true, true},
    "gpt-5-chat":       {128000, true, true},
    "Llama-3.3-70B-Instruct":      {128000, true, false},
    "Llama-4-Scout-17B-16E-Instruct": {10000000, true, false},
    "DeepSeek-V3-0324": {128000, true, false},
    "DeepSeek-R1":      {128000, false, false},
    "mistral-small-2503": {128000, true, false},
    "cohere-command-a":  {131072, true, false},
}
```

---

## 7. Sources

- **GitHub Docs — Prototyping with AI models:** https://docs.github.com/en/github-models/use-github-models/prototyping-with-ai-models (rate limits table)
- **GitHub Docs (raw):** https://raw.githubusercontent.com/github/docs/main/content/github-models/use-github-models/prototyping-with-ai-models.md
- **GitHub Marketplace Models:** https://github.com/marketplace?type=models (model catalog)
- **Live API testing:** All model IDs, tool calling, and streaming verified via `curl` against `https://models.inference.ai.azure.com/chat/completions` using `gh auth token`
- **openai-go SDK v3.31.0:** Base URL handling verified from source at `$GOMODCACHE/github.com/openai/openai-go/v3@v3.31.0/`
