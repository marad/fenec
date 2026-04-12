# Phase 2: Conversation - Research

**Researched:** 2026-04-11
**Domain:** Multi-turn conversation management, context window tracking, session persistence
**Confidence:** HIGH

## Summary

Phase 2 adds four capabilities to the existing Fenec REPL: multi-turn conversation context (already partially working via `Conversation.Messages`), token usage tracking with context window management, session save/load to disk, and auto-save on exit. The existing codebase provides strong foundations -- `chat.Conversation` already accumulates `[]api.Message` and `repl.REPL` already has signal handling infrastructure.

The key technical challenges are: (1) obtaining the model's context window limit via the Ollama Show API to know *when* to truncate, (2) using `PromptEvalCount` and `EvalCount` from `ChatResponse.Metrics` (returned on the final streaming chunk where `Done == true`) to track actual token usage rather than relying on inaccurate character-based estimates, and (3) implementing atomic JSON file persistence with graceful shutdown hooks.

**Primary recommendation:** Use Ollama's actual token counts from streaming metrics for tracking, query the model's `context_length` via `/api/show` at startup, implement client-side truncation (drop oldest non-system messages) before hitting the context limit, persist sessions as JSON files using atomic write (temp file + rename), and wire auto-save into both normal exit paths and signal handlers.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CHAT-02 | Agent maintains multi-turn conversation context across messages | Conversation struct already accumulates messages; need to verify context is sent correctly and previous messages influence responses |
| CHAT-03 | Agent manages context window -- tracks token usage and truncates when approaching model limits | Use `ChatResponse.Metrics.PromptEvalCount` + `EvalCount` for tracking; query model `context_length` via Show API; implement client-side truncation before hitting limit |
| SESS-01 | User can save conversation to disk and resume later | JSON serialization of `api.Message` slice + metadata; `/save` and `/load` slash commands; atomic file writes |
| SESS-02 | Session auto-saves on exit to prevent data loss | SIGTERM/SIGINT handler + defer in Run() for normal exit; auto-save to a known location |
</phase_requirements>

## Standard Stack

### Core (already in go.mod)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/ollama/ollama/api | v0.20.5 | Chat API, Show API, Metrics types | Already in use. Show method provides model context_length. ChatResponse.Metrics provides actual token counts. |
| encoding/json | stdlib | Session serialization | Standard library JSON. api.Message has JSON tags and custom MarshalJSON/UnmarshalJSON. Zero new dependencies. |
| os/signal | stdlib | Graceful shutdown, auto-save trigger | Already used in repl.go for SIGINT. Extend to also trigger auto-save. |

### Supporting (no new dependencies needed)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| os | stdlib | File I/O, temp files, atomic rename | Session persistence with atomic writes (TempFile + Rename). |
| path/filepath | stdlib | Cross-platform path construction | Session file paths under config directory. |
| time | stdlib | Timestamps in session metadata | Session creation/modification timestamps. |
| fmt, strconv | stdlib | User-facing messages | Token count display, session listing. |

### No New Dependencies Required

This phase requires zero new Go modules. Everything is covered by the existing `go.mod` dependencies (Ollama API types, JSON stdlib) and Go standard library. This is intentional -- the CLAUDE.md explicitly recommends against databases for conversation storage and suggests JSON file persistence.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  chat/
    message.go        # Existing: Conversation struct (MODIFY: add context tracking)
    client.go         # Existing: Client, chatAPI interface (MODIFY: add Show method)
    stream.go         # Existing: StreamChat (MODIFY: capture Metrics from final chunk)
    context.go        # NEW: Context window tracking and truncation logic
  session/
    session.go        # NEW: Session type, metadata, serialization
    store.go          # NEW: File-based session persistence (save/load/list/autosave)
  config/
    config.go         # Existing (MODIFY: add SessionDir helper)
  repl/
    repl.go           # Existing (MODIFY: add /save, /load, /history commands, auto-save hooks)
    commands.go       # Existing (MODIFY: add new command entries)
```

### Pattern 1: Token Tracking via Ollama Metrics (not estimation)

**What:** Capture actual token counts from `ChatResponse.Metrics` instead of using character-based estimation heuristics.

**When to use:** Every chat completion. The final streaming chunk (where `Done == true`) includes `PromptEvalCount` (input tokens) and `EvalCount` (output tokens).

**Why:** Character-based estimates (1 token ~= 4 chars) are unreliable across models and languages. Ollama provides exact counts. Use them.

**How it works in the current streaming code:**

The existing `StreamChat` method processes chunks via the callback `func(resp api.ChatResponse) error`. The final chunk has `Done: true` and populated `Metrics`. The current code only reads `resp.Message.Content`. It needs to also capture `resp.PromptEvalCount` and `resp.EvalCount` from that final chunk.

```go
// In StreamChat callback, capture metrics from the final chunk:
var metrics api.Metrics
err := c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
    if resp.Message.Content != "" {
        content.WriteString(resp.Message.Content)
        if onToken != nil {
            onToken(resp.Message.Content)
        }
    }
    if resp.Done {
        metrics = resp.Metrics
    }
    return ctx.Err()
})
```

**Return value change:** `StreamChat` should return the metrics alongside the message, so the caller can update the conversation's token budget.

### Pattern 2: Client-Side Context Window Management

**What:** Track cumulative token usage and proactively truncate old messages before hitting the model's context limit. Do NOT rely on Ollama's silent server-side truncation.

**When to use:** Before every `StreamChat` call, check if the conversation is approaching the context limit.

**Why Ollama's built-in truncation is insufficient:**
- Ollama's `truncate` parameter defaults to `true` and silently drops oldest messages from the front
- There is NO response field indicating truncation occurred
- The user has no visibility into lost context
- Ollama may over-truncate (known issue #11885)
- Client-side truncation gives the user control and visibility

**Strategy:**
1. At startup, query the model's `context_length` via `Client.Show()` + `model_info["{family}.context_length"]`
2. After each completion, update a running token count using `PromptEvalCount + EvalCount` from Metrics
3. Before each new request, estimate whether the next turn will exceed the limit
4. If approaching the limit (e.g., 90% threshold), drop the oldest non-system user/assistant message pairs
5. Inform the user when truncation occurs (e.g., "[context: dropped 3 oldest messages to stay within 8192 token limit]")
6. Set `Truncate: boolPtr(false)` on `ChatRequest` to disable Ollama's silent truncation in favor of our explicit management

**Token budget tracking approach:**
- After each call, store `PromptEvalCount` (this was the total input tokens Ollama processed for ALL messages)
- The running total is simply: last `PromptEvalCount` + last `EvalCount` = tokens consumed by the conversation so far
- When this exceeds the threshold, truncate from the front (skip system message)

```go
type ContextTracker struct {
    maxTokens      int     // From model's context_length
    threshold      float64 // e.g., 0.85 = truncate at 85% capacity
    lastPromptEval int     // Last PromptEvalCount from Metrics
    lastEval       int     // Last EvalCount from Metrics
}

func (ct *ContextTracker) ShouldTruncate() bool {
    total := ct.lastPromptEval + ct.lastEval
    return total >= int(float64(ct.maxTokens)*ct.threshold)
}
```

### Pattern 3: Session Persistence as JSON Files

**What:** Save conversation state (messages + metadata) to JSON files in the config directory.

**When to use:** On `/save` command, on `/quit`, on SIGINT/SIGTERM, and on Ctrl+D (EOF).

**File format:**
```json
{
    "id": "2026-04-11T14-30-00",
    "model": "gemma4:latest",
    "created_at": "2026-04-11T14:30:00Z",
    "updated_at": "2026-04-11T15:45:00Z",
    "messages": [
        {"role": "system", "content": "..."},
        {"role": "user", "content": "..."},
        {"role": "assistant", "content": "..."}
    ],
    "token_count": 4521
}
```

**Key design decisions:**
- Use `api.Message` directly for the messages array -- it has proper JSON tags and custom marshal/unmarshal
- Session ID derived from creation timestamp (human-readable, sortable, unique enough for single-user)
- Store in `~/.config/fenec/sessions/` (using `config.ConfigDir()`)
- One JSON file per session: `{id}.json`
- Auto-save file: `_autosave.json` (single file, overwritten each time)

### Pattern 4: Atomic File Writes

**What:** Write to a temp file in the same directory, then rename to the target path.

**When to use:** Every session save operation. Prevents data loss if the process crashes mid-write.

```go
func atomicWriteJSON(path string, v any) error {
    dir := filepath.Dir(path)
    f, err := os.CreateTemp(dir, ".fenec-session-*.tmp")
    if err != nil {
        return fmt.Errorf("creating temp file: %w", err)
    }
    tmpPath := f.Name()

    // Cleanup on failure
    defer func() {
        if tmpPath != "" {
            os.Remove(tmpPath)
        }
    }()

    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    if err := enc.Encode(v); err != nil {
        f.Close()
        return fmt.Errorf("encoding JSON: %w", err)
    }
    if err := f.Sync(); err != nil {
        f.Close()
        return fmt.Errorf("syncing file: %w", err)
    }
    if err := f.Close(); err != nil {
        return fmt.Errorf("closing temp file: %w", err)
    }

    if err := os.Rename(tmpPath, path); err != nil {
        return fmt.Errorf("renaming temp file: %w", err)
    }
    tmpPath = "" // Prevent deferred cleanup
    return nil
}
```

### Pattern 5: Auto-Save on Exit

**What:** Save the current conversation automatically on all exit paths.

**When to use:** Normal exit (Ctrl+D, `/quit`), signal-based exit (SIGINT when not streaming, SIGTERM).

**Implementation approach:**

The existing `REPL.Run()` method has a loop that returns `nil` on EOF or `/quit`. Add auto-save logic:

1. **Normal exit:** `defer r.autoSave()` at the top of `Run()`
2. **Signal exit:** The existing SIGINT goroutine in `NewREPL` handles Ctrl+C for streaming cancellation. Extend it: if NOT streaming and second Ctrl+C arrives (or SIGTERM), trigger auto-save before exiting.
3. **`REPL.Close()`:** Already called via `defer r.Close()` in `main.go`. Add auto-save here as the final safety net.

The auto-save should be idempotent (safe to call multiple times) and non-blocking (best-effort, log errors but don't fail the exit).

### Anti-Patterns to Avoid

- **Estimating tokens from character count:** Inaccurate. Use Ollama's actual `PromptEvalCount`/`EvalCount` from Metrics.
- **Relying on Ollama's silent truncation:** No feedback to user. Disable it (`Truncate: false`) and manage client-side.
- **Using a database for session storage:** Premature. JSON files are simpler, human-readable, and sufficient for a single-user CLI tool. CLAUDE.md explicitly says "In-memory []Message slice, optional JSON file persistence."
- **Blocking on auto-save during signal handler:** Signal handlers must be fast. Save in a goroutine with a short timeout if needed, but prefer synchronous save in `Close()` which runs in the main goroutine.
- **Saving sessions with no messages:** Don't create session files for empty conversations. Check `len(messages) > 1` (more than just system prompt) before saving.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token counting | Character-based estimator | `ChatResponse.Metrics.PromptEvalCount` and `EvalCount` | Ollama provides exact counts per request. No estimation needed. |
| Model context limit discovery | Hardcoded constants per model | `Client.Show()` -> `model_info["{family}.context_length"]` | Different models have different limits (4k, 8k, 32k, 128k). Query at runtime. |
| JSON serialization of messages | Custom serialization format | `encoding/json` with `api.Message` (has JSON tags + custom marshal) | Ollama's types are already JSON-serializable. |
| Atomic file writes | `os.WriteFile` directly | Temp file + `os.Rename` pattern | Prevents corruption on crash mid-write. Standard POSIX atomicity guarantee. |
| Session file format | Custom binary or TOML | Plain JSON | Human-readable, debuggable, matches the `api.Message` JSON format. |

**Key insight:** Ollama already provides the two hardest pieces -- exact token counts (via Metrics) and model context limits (via Show API). The phase is primarily about wiring these existing capabilities into the conversation flow and adding file I/O.

## Common Pitfalls

### Pitfall 1: Silent Context Truncation by Ollama
**What goes wrong:** Ollama's default behavior (`truncate: true`) silently drops oldest messages when the conversation exceeds the context window. The user thinks the model "forgot" earlier context, but it was actually truncated server-side with no indication.
**Why it happens:** `ChatRequest.Truncate` defaults to `true`. No response field indicates truncation occurred.
**How to avoid:** Set `Truncate: boolPtr(false)` on every ChatRequest. Manage truncation client-side with user-visible feedback.
**Warning signs:** Model responses that ignore earlier context. `PromptEvalCount` suddenly much lower than expected.

### Pitfall 2: Context Length Defaults Vary by VRAM
**What goes wrong:** Assuming a fixed default context length (e.g., 4096). Ollama dynamically adjusts defaults based on available VRAM: under 24 GB = 4k, 24-48 GB = 32k, 48+ GB = 256k.
**Why it happens:** The default is not a fixed number -- it depends on the machine's GPU.
**How to avoid:** Always query the model's actual context_length via the Show API. Do not hardcode.
**Warning signs:** Truncation happening earlier or later than expected on different machines.

### Pitfall 3: Metrics Only on Final Streaming Chunk
**What goes wrong:** Trying to read `PromptEvalCount`/`EvalCount` from intermediate streaming chunks -- they're zero.
**Why it happens:** Ollama populates Metrics only on the final chunk where `Done == true`.
**How to avoid:** Capture metrics in the streaming callback specifically when `resp.Done == true`.
**Warning signs:** Token counts always showing 0.

### Pitfall 4: Not Setting num_ctx on ChatRequest
**What goes wrong:** The model uses whatever context size Ollama decided at load time, which may not match what you queried via Show.
**Why it happens:** STATE.md already documents this: "Set num_ctx explicitly from first Ollama call to avoid silent context truncation."
**How to avoid:** Include `"num_ctx": contextLength` in `ChatRequest.Options` on every call.
**Warning signs:** Inconsistent behavior between sessions or after model reloads.

### Pitfall 5: Crash During Session Write Corrupts File
**What goes wrong:** If the process is killed while writing a session JSON file, the file is left in a partial/corrupted state and cannot be loaded.
**Why it happens:** `os.WriteFile` is not atomic -- it writes directly to the target path.
**How to avoid:** Use the atomic write pattern: create temp file in same directory, write, sync, close, then `os.Rename`.
**Warning signs:** Session files that fail to parse on load.

### Pitfall 6: model_info Key Prefix Varies by Model Family
**What goes wrong:** Looking for `"context_length"` as a direct key in `model_info`, when the actual key is `"{family}.context_length"` (e.g., `"gemma3.context_length"`, `"llama.context_length"`).
**Why it happens:** Ollama prefixes `model_info` keys with the model architecture family name.
**How to avoid:** Iterate `model_info` keys looking for a suffix of `.context_length`, or use the `Details.Family` field to construct the key.
**Warning signs:** Context length always returning 0 or nil.

### Pitfall 7: Auto-Save Races with Signal Handler
**What goes wrong:** Both `defer autoSave()` in `Run()` and the signal handler try to save simultaneously, corrupting the file.
**Why it happens:** SIGINT arrives, signal goroutine saves, then deferred cleanup also saves.
**How to avoid:** Use `sync.Once` for auto-save -- first call wins, subsequent calls are no-ops.
**Warning signs:** Corrupted auto-save files, race detector warnings.

## Code Examples

### Querying Model Context Length via Show API

```go
// Source: Ollama Go API docs + /api/show response format
func (c *Client) GetContextLength(ctx context.Context, model string) (int, error) {
    resp, err := c.api.Show(ctx, &api.ShowRequest{Model: model})
    if err != nil {
        return 0, fmt.Errorf("show model %s: %w", model, err)
    }

    // model_info keys are prefixed with the family name, e.g. "gemma3.context_length"
    for key, val := range resp.ModelInfo {
        if strings.HasSuffix(key, ".context_length") {
            switch v := val.(type) {
            case float64:
                return int(v), nil
            case int:
                return v, nil
            }
        }
    }

    // Fallback: conservative default
    return 4096, nil
}
```

### Capturing Metrics from Streaming Response

```go
// Modified StreamChat that returns metrics alongside the message
func (c *Client) StreamChat(ctx context.Context, conv *Conversation, onToken func(string)) (*api.Message, *api.Metrics, error) {
    var content strings.Builder
    var metrics api.Metrics

    req := &api.ChatRequest{
        Model:    conv.Model,
        Messages: conv.Messages,
        Options:  map[string]any{"num_ctx": conv.ContextLength},
    }
    // Disable server-side truncation -- we manage it client-side
    f := false
    req.Truncate = &f

    err := c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
        if resp.Message.Content != "" {
            content.WriteString(resp.Message.Content)
            if onToken != nil {
                onToken(resp.Message.Content)
            }
        }
        if resp.Done {
            metrics = resp.Metrics
        }
        return ctx.Err()
    })

    // ... error handling same as current ...

    return &api.Message{
        Role:    "assistant",
        Content: content.String(),
    }, &metrics, nil
}
```

### Session File Structure

```go
// Session represents a saved conversation.
type Session struct {
    ID        string        `json:"id"`
    Model     string        `json:"model"`
    CreatedAt time.Time     `json:"created_at"`
    UpdatedAt time.Time     `json:"updated_at"`
    Messages  []api.Message `json:"messages"`
    TokenCount int          `json:"token_count"` // Last known total token usage
}
```

### Context Window Truncation

```go
// TruncateOldest removes the oldest non-system message pairs until
// the estimated token count is below the threshold.
// Returns the number of messages removed.
func (conv *Conversation) TruncateOldest(currentTokens, maxTokens int, threshold float64) int {
    limit := int(float64(maxTokens) * threshold)
    if currentTokens <= limit {
        return 0
    }

    removed := 0
    // Find first non-system message
    start := 0
    for start < len(conv.Messages) && conv.Messages[start].Role == "system" {
        start++
    }

    // Remove pairs (user + assistant) from the front
    for currentTokens > limit && start+1 < len(conv.Messages) {
        // Remove two messages (user + assistant pair)
        conv.Messages = append(conv.Messages[:start], conv.Messages[start+2:]...)
        removed += 2
        // Rough estimate: reduce by proportion of messages removed
        // (Actual count will be corrected by next PromptEvalCount)
        currentTokens = currentTokens * (len(conv.Messages)) / (len(conv.Messages) + 2)
    }

    return removed
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hardcoded 4096 context | Dynamic VRAM-based defaults (4k/32k/256k) | Ollama recent versions | Must query actual limit, not assume 4096 |
| No token counts in API | Metrics embedded in ChatResponse | Ollama v0.x+ | Can track exact usage without estimation |
| `Truncate` defaults to true silently | Still defaults to true, but field exists to disable | Ollama v0.20+ | Set to false for client-side control |
| Character-based token estimation | Server-reported PromptEvalCount/EvalCount | Always available | No need for tokenizer libraries in Go |

## Open Questions

1. **How does PromptEvalCount behave with Ollama's KV cache?**
   - What we know: Ollama caches key-value pairs between requests. If the conversation prefix hasn't changed, it reuses the cache.
   - What's unclear: Does `PromptEvalCount` reflect the total prompt tokens or only newly evaluated tokens? If cached, does it still report the full count?
   - Recommendation: Test empirically with a multi-turn conversation. If cached calls report fewer tokens, fall back to accumulating counts manually. LOW confidence on this specific behavior.

2. **What happens when Show API is unavailable for a model?**
   - What we know: Works for standard models (gemma, llama, etc.). Custom Modelfiles should also work.
   - What's unclear: Remote/proxy models may not support Show. Edge case for local-first tool.
   - Recommendation: Fall back to a conservative default (4096) if Show fails or returns no context_length. Log a warning.

3. **Session file size for long conversations**
   - What we know: JSON files with many messages can grow large. A 128k context conversation could produce 100+ KB JSON files.
   - What's unclear: Whether this matters for a personal tool. Probably fine.
   - Recommendation: No compression for v1. If sessions grow unwieldy, address in v2.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None -- Go convention (go test ./...) |
| Quick run command | `go test ./internal/chat/ ./internal/session/ -v -count=1` |
| Full suite command | `go test -race -cover ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CHAT-02 | Multi-turn context: messages accumulate and are sent to model | unit | `go test ./internal/chat/ -run TestConversation -v -count=1` | Partial (message.go has Conversation but no dedicated multi-turn test) |
| CHAT-03 | Token tracking and truncation when approaching limit | unit | `go test ./internal/chat/ -run TestContext -v -count=1` | No -- Wave 0 |
| SESS-01 | Save/load session to/from disk | unit | `go test ./internal/session/ -run TestSession -v -count=1` | No -- Wave 0 |
| SESS-02 | Auto-save on exit | unit + integration | `go test ./internal/session/ -run TestAutoSave -v -count=1` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/chat/ ./internal/session/ -v -count=1`
- **Per wave merge:** `go test -race -cover ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/chat/context_test.go` -- covers CHAT-03 (token tracking, truncation logic, threshold behavior)
- [ ] `internal/session/session_test.go` -- covers SESS-01 (serialization, save/load, atomic writes)
- [ ] `internal/session/store_test.go` -- covers SESS-01, SESS-02 (file persistence, listing, auto-save)
- [ ] `internal/chat/message_test.go` -- covers CHAT-02 (multi-turn message accumulation, verify full history sent)
- [ ] Framework install: None -- testify already in go.mod

## Sources

### Primary (HIGH confidence)
- [Ollama Go API package](https://pkg.go.dev/github.com/ollama/ollama/api) -- ChatRequest, ChatResponse, Metrics, ShowRequest, ShowResponse types verified
- [Ollama API docs: Show model details](https://docs.ollama.com/api-reference/show-model-details) -- model_info keys including context_length, family prefix pattern
- [Ollama API docs: Usage](https://docs.ollama.com/api/usage) -- PromptEvalCount, EvalCount available in final streaming chunk
- [Ollama context length docs](https://docs.ollama.com/context-length) -- VRAM-based defaults (4k/32k/256k)

### Secondary (MEDIUM confidence)
- [Ollama issue #14259](https://github.com/ollama/ollama/issues/14259) -- Silent truncation behavior, no response field for truncation indication
- [Ollama issue #11885](https://github.com/ollama/ollama/issues/11885) -- Over-truncation with large context windows
- [Go atomic file write patterns](https://michael.stapelberg.ch/posts/2017-01-28-golang_atomically_writing/) -- TempFile + Rename approach
- [Go graceful shutdown patterns](https://victoriametrics.com/blog/go-graceful-shutdown/) -- signal.NotifyContext, cleanup patterns

### Tertiary (LOW confidence)
- PromptEvalCount behavior with KV cache -- unclear from docs, needs empirical testing

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib + existing Ollama dependency, zero new modules
- Architecture: HIGH -- well-understood patterns (JSON persistence, signal handling), clear Ollama API types
- Pitfalls: HIGH -- documented Ollama issues (silent truncation, VRAM defaults), verified against multiple sources
- Context tracking via Metrics: MEDIUM -- API types confirmed, but KV cache interaction with PromptEvalCount not fully documented

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable domain, Ollama API unlikely to break)
