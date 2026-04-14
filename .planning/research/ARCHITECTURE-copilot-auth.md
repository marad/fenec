# Architecture: Copilot Provider — GitHub Models API Authentication

**Domain:** GitHub Models API integration for Fenec CLI
**Researched:** 2026-04-14
**Overall confidence:** HIGH (verified by live API testing)

## Executive Summary

The `copilot` provider type integrates Fenec with GitHub Models via the user's existing `gh` CLI authentication. The architecture is straightforward: retrieve a GitHub OAuth token from `gh auth token`, use it as Bearer auth against `https://models.github.ai`. The chat completions endpoint is fully OpenAI-compatible, so the existing `openai-go/v3` SDK can be reused. Model listing requires a custom HTTP call because the endpoint path differs from chat completions.

**Critical finding:** The old Azure endpoint (`https://models.inference.ai.azure.com`) is **deprecated as of July 17, 2025** and will be removed **October 17, 2025**. The correct endpoint is `https://models.github.ai`.

---

## 1. GitHub Models API Endpoint Architecture

### Endpoint Discovery (Verified Live)

| Purpose | URL | Method | Format |
|---------|-----|--------|--------|
| Chat completions | `https://models.github.ai/inference/chat/completions` | POST | OpenAI-compatible |
| Model listing | `https://models.github.ai/v1/models` | GET | OpenAI-ish (see notes) |
| ~~Legacy (DEPRECATED)~~ | ~~`https://models.inference.ai.azure.com`~~ | — | Sunset Oct 17, 2025 |

**Key architectural constraint:** Chat completions and model listing use **different base paths** (`/inference/` vs `/v1/`). This means a single `openai-go` SDK client cannot serve both endpoints.

### Chat Completions — Full OpenAI Compatibility ✅

Verified features:
- Standard `chat/completions` request/response format
- SSE streaming with `stream: true` (standard `data:` prefix, `[DONE]` terminator)
- Tool calling (function calling) — request and response format identical to OpenAI
- Token usage in response (`usage.prompt_tokens`, `usage.completion_tokens`)
- Content filter metadata (Azure-specific extra fields, safely ignored by SDK)

**Model ID format:** `publisher/model-name` (e.g., `openai/gpt-4o-mini`, `meta/llama-3.3-70b-instruct`)

### Model Listing — Partially Compatible ⚠️

The `/v1/models` endpoint returns `{"data": [...]}` which is structurally similar to OpenAI, but individual model objects differ:

| Field | OpenAI Format | GitHub Models Format |
|-------|--------------|---------------------|
| `id` | ✅ Present | ✅ Present (e.g., `openai/gpt-4o-mini`) |
| `object` | `"model"` | ❌ Missing |
| `created` | Unix timestamp | ❌ Missing |
| `owned_by` | String | ❌ Missing |
| `name` | ❌ N/A | ✅ Human-friendly name |
| `capabilities` | ❌ N/A | ✅ `["streaming", "tool-calling", ...]` |
| `limits` | ❌ N/A | ✅ `{"max_input_tokens": N, "max_output_tokens": N}` |
| `rate_limit_tier` | ❌ N/A | ✅ `"low"`, `"high"`, `"custom"` |

**Implication:** The `openai-go` SDK's `ListAutoPaging` will fail on this endpoint because the `Model` struct requires `id`, `created`, and `object` fields. Model listing must use a custom HTTP client.

**Bonus:** The `limits.max_input_tokens` field solves the context length problem — `GetContextLength()` can return real data instead of the OpenAI provider's current `return 0, nil`.

### Rate Limits (from response headers)

| Limit | Value | Period |
|-------|-------|--------|
| Requests | 20,000 | 60 seconds |
| Tokens | 2,000,000 | 60 seconds |

Rate limit headers follow standard format: `x-ratelimit-limit-requests`, `x-ratelimit-remaining-requests`, etc.

### Error Responses

| Scenario | HTTP Status | Body |
|----------|-------------|------|
| No auth header | 401 | `"Unauthorized"` (plain text) |
| Invalid token | 401 | `"Unauthorized"` (plain text) |
| Unknown model | 200* | `{"error":{"code":"unknown_model","message":"Unknown model: ..."}}`|
| Rate limited | 429 | Standard retry-after headers |

*Note: Unknown model returns 200 with error JSON, not 404. The SDK will need to handle this.

---

## 2. Token Retrieval Architecture

### How `gh auth token` Works

**Command:** `gh auth token`
**Output:** Token on stdout, followed by newline (`\n`)
**Token format:** `gho_*` prefix (OAuth App user-to-server token)
**Token storage:** System keyring (macOS Keychain, Linux secret-service, Windows Credential Manager)

**Exit codes:**
| Code | Meaning |
|------|---------|
| 0 | Success, token printed to stdout |
| 1 | General error (not authenticated, hostname not found) |
| 4 | Authentication required (gh not logged in) |

**Error output:** Goes to stderr (e.g., `"no oauth token found for nonexistent.example.com"`)

**Flags used:**
- `--hostname <host>` — specify GitHub host (default: `github.com`)
- `--user <user>` — specify account (default: active account)

### Token Scope Requirements

**Verified experimentally:** A standard `gh auth login` token with scopes `gist, read:org, repo, workflow` is sufficient for GitHub Models API access. **No special `models:read` or similar scope is required.** Any authenticated GitHub user token works.

### Token Resolution Priority (from `go-gh` source code analysis)

The `gh` ecosystem resolves tokens in this order:

1. **`GH_TOKEN` env var** — highest priority for `github.com`
2. **`GITHUB_TOKEN` env var** — second priority for `github.com`
3. **Config file** (`~/.config/gh/hosts.yml` → `oauth_token` field)
4. **System keyring** via `gh auth token --secure-storage`

For enterprise hosts, `GH_ENTERPRISE_TOKEN` and `GITHUB_ENTERPRISE_TOKEN` are checked instead.

### Recommended Token Retrieval Strategy for Fenec

**Do NOT use `go-gh/v2` as a library dependency.** It pulls 20+ transitive dependencies (survey, glamour, lipgloss, gojq, etc.) for a function that can be implemented in ~30 lines.

**Instead, implement the same priority chain directly:**

```go
package copilot

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// resolveToken retrieves a GitHub token using the same priority chain
// as the gh CLI ecosystem:
// 1. GH_TOKEN env var
// 2. GITHUB_TOKEN env var  
// 3. `gh auth token` subprocess (reads keyring)
func resolveToken() (string, error) {
    // Priority 1: GH_TOKEN (matches gh CLI behavior)
    if token := os.Getenv("GH_TOKEN"); token != "" {
        return token, nil
    }
    
    // Priority 2: GITHUB_TOKEN (CI/CD environments like GitHub Actions)
    if token := os.Getenv("GITHUB_TOKEN"); token != "" {
        return token, nil
    }
    
    // Priority 3: gh CLI keyring
    return tokenFromGhCLI()
}

func tokenFromGhCLI() (string, error) {
    ghPath, err := exec.LookPath("gh")
    if err != nil {
        return "", fmt.Errorf("copilot provider requires the GitHub CLI (gh): %w\n"+
            "Install from https://cli.github.com", err)
    }
    
    cmd := exec.Command(ghPath, "auth", "token", "--hostname", "github.com")
    output, err := cmd.Output()
    if err != nil {
        // Check if it's an exit error to provide better messages
        if exitErr, ok := err.(*exec.ExitError); ok {
            stderr := strings.TrimSpace(string(exitErr.Stderr))
            return "", fmt.Errorf("gh auth failed (exit %d): %s\n"+
                "Run 'gh auth login' to authenticate", exitErr.ExitCode(), stderr)
        }
        return "", fmt.Errorf("failed to run gh auth token: %w", err)
    }
    
    token := strings.TrimSpace(string(output))
    if token == "" {
        return "", fmt.Errorf("gh auth token returned empty result\n"+
            "Run 'gh auth login' to authenticate")
    }
    
    return token, nil
}
```

### Why This Priority Chain Matters

| Scenario | Token Source | Why |
|----------|-------------|-----|
| Developer workstation | `gh auth token` (keyring) | Standard `gh auth login` flow |
| GitHub Actions | `GITHUB_TOKEN` env var | Auto-injected by Actions runtime |
| Codespaces | `GH_TOKEN` or `GITHUB_TOKEN` | Auto-injected |
| CI with explicit token | `GH_TOKEN` env var | User sets in CI config |
| Docker / headless | `GH_TOKEN` env var | No keyring available |

---

## 3. Provider Architecture

### Design: Compose `openai.Provider` + Custom Model Listing

The copilot provider should **embed/wrap** the existing `openai.Provider` for chat operations, adding:
1. Token resolution at init time
2. Custom model listing via HTTP
3. Context length from model metadata

```
┌──────────────────────────────────────────┐
│            copilot.Provider              │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  openai.Provider (embedded)        │  │  ← Handles StreamChat, Ping
│  │  baseURL: models.github.ai/infer.  │  │
│  │  apiKey: gh token                  │  │
│  └────────────────────────────────────┘  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  Custom model catalog              │  │  ← ListModels, GetContextLength
│  │  HTTP GET models.github.ai/v1/    │  │
│  │  Cached in-memory                  │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

### Recommended Implementation Structure

```go
// internal/provider/copilot/copilot.go
package copilot

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"

    "github.com/marad/fenec/internal/model"
    "github.com/marad/fenec/internal/provider"
    openaiProvider "github.com/marad/fenec/internal/provider/openai"
)

const (
    inferenceBaseURL = "https://models.github.ai/inference"
    modelsURL        = "https://models.github.ai/v1/models"
)

var _ provider.Provider = (*Provider)(nil)

type Provider struct {
    inner      *openaiProvider.Provider  // Delegates StreamChat, etc.
    token      string
    httpClient *http.Client
    
    mu         sync.RWMutex
    catalog    []ghModel               // Cached model list
}

// ghModel represents a model from the GitHub Models API.
type ghModel struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Capabilities []string `json:"capabilities"`
    Limits       struct {
        MaxInputTokens  int `json:"max_input_tokens"`
        MaxOutputTokens int `json:"max_output_tokens"`
    } `json:"limits"`
    RateLimitTier string `json:"rate_limit_tier"`
}

type modelsResponse struct {
    Data []ghModel `json:"data"`
}

func New() (*Provider, error) {
    token, err := resolveToken()
    if err != nil {
        return nil, fmt.Errorf("copilot: %w", err)
    }
    
    inner, err := openaiProvider.New(inferenceBaseURL, token)
    if err != nil {
        return nil, fmt.Errorf("copilot: creating openai client: %w", err)
    }
    
    return &Provider{
        inner:      inner,
        token:      token,
        httpClient: &http.Client{},
    }, nil
}

func (p *Provider) Name() string { return "copilot" }

func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
    catalog, err := p.fetchCatalog(ctx)
    if err != nil {
        return nil, err
    }
    
    var names []string
    for _, m := range catalog {
        names = append(names, m.ID)
    }
    return names, nil
}

func (p *Provider) Ping(ctx context.Context) error {
    _, err := p.fetchCatalog(ctx)
    return err
}

func (p *Provider) GetContextLength(ctx context.Context, modelName string) (int, error) {
    catalog, err := p.fetchCatalog(ctx)
    if err != nil {
        return 0, err
    }
    for _, m := range catalog {
        if m.ID == modelName {
            return m.Limits.MaxInputTokens, nil
        }
    }
    return 0, nil // Unknown model, use default
}

func (p *Provider) StreamChat(ctx context.Context, req *provider.ChatRequest,
    onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
    return p.inner.StreamChat(ctx, req, onToken, onThinking)
}

func (p *Provider) fetchCatalog(ctx context.Context) ([]ghModel, error) {
    p.mu.RLock()
    if p.catalog != nil {
        defer p.mu.RUnlock()
        return p.catalog, nil
    }
    p.mu.RUnlock()
    
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // Double-check after acquiring write lock
    if p.catalog != nil {
        return p.catalog, nil
    }
    
    req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+p.token)
    
    resp, err := p.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching models: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("models API returned %d", resp.StatusCode)
    }
    
    var result modelsResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decoding models: %w", err)
    }
    
    p.catalog = result.Data
    return p.catalog, nil
}
```

### Config Integration

```toml
# ~/.config/fenec/config.toml

default_provider = "copilot"

[providers.copilot]
type = "copilot"
# No url or api_key needed — auto-resolved from gh CLI
```

Changes to `config/toml.go`:

```go
func CreateProvider(name string, cfg ProviderConfig) (provider.Provider, error) {
    switch cfg.Type {
    case "ollama":
        return ollama.New(cfg.URL)
    case "openai":
        return openaiProvider.New(cfg.URL, cfg.APIKey)
    case "copilot":
        return copilotProvider.New()  // No args needed
    default:
        return nil, fmt.Errorf("unknown provider type %q for provider %q", cfg.Type, name)
    }
}
```

---

## 4. Token Lifecycle & Caching

### Token Lifetime

GitHub OAuth tokens from `gh auth login` are **long-lived** (no automatic expiry). They persist until:
- User explicitly runs `gh auth logout`
- User revokes the token on github.com (Settings → Applications)
- Token is rotated via `gh auth refresh`

**Implication:** Fetching the token once at provider init time is safe. No refresh loop needed.

### Caching Strategy

**Recommended: Fetch once at provider creation, never refresh during session.**

Rationale:
1. `gh auth token` shells out to the system keyring — costs ~50-100ms
2. Token doesn't expire during a CLI session (sessions last minutes to hours)
3. If token is revoked mid-session, the API returns 401 — handle as a retryable error
4. Re-fetching on 401 adds complexity without real benefit for a CLI tool

**For the model catalog:** Cache in-memory with lazy fetch (first call populates, subsequent calls return cached). The model list doesn't change during a session.

### 401 Error Handling

If the API returns 401 during a session (token was revoked externally):

```go
// In StreamChat error handling:
// "copilot: authentication failed — your GitHub token may have been revoked.
//  Run 'gh auth login' to re-authenticate, then restart fenec."
```

Don't auto-retry with a fresh token — it adds complexity and the user needs to know their session is broken.

---

## 5. Error Handling Matrix

### Provider Initialization Errors

| Condition | Detection | User Message |
|-----------|-----------|-------------|
| `gh` not installed | `exec.LookPath("gh")` fails | "copilot provider requires the GitHub CLI (gh). Install from https://cli.github.com" |
| `gh` not authenticated | `gh auth token` exits 1 | "GitHub CLI is not authenticated. Run 'gh auth login' to authenticate." |
| `gh` installed but old | Token works but API fails | Handle at API call level, not init |
| `GH_TOKEN` set but invalid | Token resolves but API returns 401 | "GitHub token is invalid. Check your GH_TOKEN environment variable." |

### Runtime Errors

| Condition | Detection | Handling |
|-----------|-----------|---------|
| Token revoked mid-session | 401 from API | Clear error message, suggest re-login |
| Rate limited | 429 + retry-after header | The openai-go SDK has `MaxRetries(2)` built in |
| Model not available | Error JSON in response | Surface model name in error |
| Network error | HTTP client error | Standard network error handling |

---

## 6. Security Considerations

### Token Handling

1. **Never log the token.** Use `slog` carefully — the token must not appear in debug output.
2. **Never store the token on disk.** It lives only in memory for the session duration.
3. **Environment variable precedence is intentional.** `GH_TOKEN` and `GITHUB_TOKEN` are standard in the `gh` ecosystem. Users expect these to work.
4. **`exec.LookPath` is safe in Go 1.19+.** No need for `cli/safeexec` — the current directory exploit was fixed in the Go stdlib.

### Subprocess Safety

```go
// GOOD: Use LookPath to get absolute path, then exec it
ghPath, err := exec.LookPath("gh")
cmd := exec.Command(ghPath, "auth", "token", "--hostname", "github.com")

// GOOD: Use cmd.Output() which captures stdout only
output, err := cmd.Output()  // stderr available via ExitError.Stderr

// BAD: Don't use cmd.CombinedOutput() — stderr might leak into token
// BAD: Don't use shell expansion: exec.Command("sh", "-c", "gh auth token")
```

---

## 7. Available Models (as of 2026-04-14)

43 models available, key ones for Fenec users:

| Model | Context | Tool Calling | Tier |
|-------|---------|--------------|------|
| `openai/gpt-4.1` | 1,048,576 | ✅ | high |
| `openai/gpt-4.1-mini` | 1,048,576 | ✅ | low |
| `openai/gpt-4o` | 131,072 | ✅ | high |
| `openai/gpt-4o-mini` | 131,072 | ✅ | low |
| `meta/llama-3.3-70b-instruct` | 131,072 | ❌ | high |
| `deepseek/deepseek-r1` | 128,000 | ✅ | custom |
| `mistral-ai/mistral-small-2503` | 128,000 | ✅ | low |

Rate limit tiers: `low` (generous free tier), `high` (higher limits), `custom` (may require pay-as-you-go).

---

## 8. Decision: go-gh Library vs. os/exec

### Recommendation: Use `os/exec` directly. Do NOT add `go-gh/v2` dependency.

**Reasons:**

| Factor | `go-gh/v2` | `os/exec` direct |
|--------|-----------|-----------------|
| Dependencies | 20+ packages (survey, glamour, lipgloss, gojq, etc.) | 0 new dependencies |
| Binary size impact | Significant | None |
| Token resolution | `auth.TokenForHost("github.com")` — one line | ~30 lines implementing same chain |
| Env var support | Built-in `GH_TOKEN`/`GITHUB_TOKEN` fallback | Manual (trivial to implement) |
| Maintenance | Tracks gh CLI updates automatically | Must update if gh changes behavior |
| Testability | Harder to mock (reads real config) | Easy to mock (interface around exec) |

The `go-gh` auth package internally does exactly what our `resolveToken()` function does: check env vars, then shell out to `gh auth token`. There's no magic. Importing 20+ transitive dependencies for 30 lines of logic is wrong.

---

## 9. Streaming Compatibility Verification

The GitHub Models streaming format is **identical to OpenAI's SSE format**:

```
data: {"choices":[{"delta":{"content":"Hello"},...}],"model":"gpt-4o-mini-2024-07-18","object":"chat.completion.chunk",...}
data: {"choices":[{"delta":{"content":" world"},...}],...}
data: [DONE]
```

Extra Azure-specific fields in chunks (`content_filter_results`, `prompt_filter_results`) are safely ignored by the openai-go SDK. The existing `chatStreaming` and `chatNonStreaming` methods in `openai.Provider` will work without modification.

**Tool calling in non-streaming mode** also works identically — same `tool_calls` array structure with `function.name` and `function.arguments` as JSON string.

---

## 10. Implementation Checklist

1. **Create `internal/provider/copilot/` package**
   - `token.go` — `resolveToken()` with env → gh CLI fallback chain
   - `copilot.go` — Provider struct, wraps `openai.Provider`
   - `models.go` — Custom model catalog fetch from `/v1/models`
   - `copilot_test.go` — Mock token resolution, mock HTTP for model listing

2. **Update `config/toml.go`**
   - Add `case "copilot"` to `CreateProvider` switch

3. **Test matrix**
   - `gh` installed + authenticated → works
   - `gh` not installed → clear error
   - `gh` installed but not authenticated → clear error
   - `GH_TOKEN` env var set → uses it, skips `gh` CLI
   - Invalid token → 401 with helpful message
   - Model listing → returns 43 models with correct IDs
   - Chat completion → streams correctly
   - Tool calling → works in non-streaming mode
   - GetContextLength → returns real values from model metadata

---

## Sources

| Source | Confidence | What It Verified |
|--------|-----------|-----------------|
| Live API testing against `models.github.ai` | HIGH | Endpoint paths, auth, response formats, streaming, tool calling |
| `gh auth token --help` + exit code testing | HIGH | Token retrieval behavior, output format, error codes |
| `gh auth status` output | HIGH | Token type (`gho_*`), scopes, storage (keyring) |
| `go-gh/v2@v2.13.0` source code (`pkg/auth/auth.go`) | HIGH | Token resolution priority chain, env vars checked |
| `openai-go/v3@v3.31.0` source code | HIGH | SDK URL resolution, path construction, base URL handling |
| GitHub Blog changelog (2025-07-17) | HIGH | Azure endpoint deprecation, new endpoint URL |
| Response headers from `models.github.ai` | HIGH | Rate limits, deprecation notices, region info |
