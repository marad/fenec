# Phase 1: Foundation - Research

**Researched:** 2026-04-11
**Domain:** Go CLI REPL with Ollama streaming chat and terminal markdown rendering
**Confidence:** HIGH

## Summary

Phase 1 builds a greenfield Go CLI application that connects to a local Ollama instance, provides a readline-based REPL for interactive chat, streams model responses token-by-token, and renders the final output with markdown formatting. The project has no existing code -- go.mod, source files, and test infrastructure must all be created from scratch.

The core technical challenge is **streaming markdown rendering**: tokens arrive one at a time from the Ollama callback, but markdown cannot be reliably rendered incrementally (code fences, tables, and list nesting require seeing the full block). The recommended approach is a two-phase strategy: print raw tokens during streaming for immediate feedback, then re-render the complete response with glamour once streaming completes. This matches how Ollama's own CLI works (it does NOT use glamour during streaming) and avoids the unsolved problem of incremental markdown parsing that even the Charm team has not shipped (glow PR #823 was closed without merging).

**Primary recommendation:** Stream raw tokens to stdout during generation, accumulate full response in a buffer, then replace the raw output with glamour-rendered markdown after the final token arrives. Use ANSI escape sequences (cursor-up + clear-line) to overwrite the raw output with formatted output.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Model-aware prompt showing active model name: `[gemma4]> `
- **D-02:** Single Enter sends message. Backslash at end of line continues to next line for multi-line input.
- **D-03:** Slash-prefix commands: `/quit`, `/model`, `/help`. Clear separation from messages sent to model.
- **D-04:** Non-modal key bindings -- each key always does one thing. Ctrl+C clears current input. Escape cancels active generation. Ctrl+D exits the application. No "press again to confirm" patterns.
- **D-05:** Animated thinking indicator (spinner/dots) shown between user pressing Enter and first token arriving. Indicator is replaced once streaming begins.
- **D-06:** Live progressive markdown rendering as tokens stream in. Code blocks, lists, and formatting render incrementally rather than waiting for response completion.
- **D-07:** Blank line separator between assistant response and next prompt. No horizontal rules or timestamps.
- **D-08:** Auto-page long responses that exceed terminal height. Pause with a "more" prompt (like less/more pager). User can press Enter to continue or q to stop.
- **D-09:** Default to first available model from Ollama -- no hardcoded default. Query Ollama for installed models at startup.
- **D-10:** `/model` with no args opens interactive numbered list. User picks by number. Shows which model is currently active.
- **D-11:** Conversation history preserved when switching models. New model sees all prior messages.
- **D-12:** Minimal model info -- active model shown only in the `[model]>` prompt. No extra status display.
- **D-13:** Startup banner: app name, version, help hint. Format: `fenec v0.1 -- type /help for commands`
- **D-14:** If Ollama is not running or unreachable, show a clear error message with fix instructions and exit. Do not start a broken REPL.
- **D-15:** System prompt loaded from markdown file at `~/.config/fenec/system.md`. If file doesn't exist, use a sensible default. Not a config key -- a standalone markdown file the user can edit directly.
- **D-16:** Connect to `localhost:11434` by default. `--host` flag to override.

### Claude's Discretion
- Thinking indicator animation style (spinner characters, frame rate)
- Exact glamour/lipgloss styling and color configuration
- Auto-pager implementation details (buffer strategy, key bindings beyond Enter/q)
- Internal message type structure
- Error message wording for non-connection errors
- readline configuration (history file location, completion settings)
- Default system prompt content when `~/.config/fenec/system.md` doesn't exist

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CHAT-01 | User can send messages and receive streaming responses token-by-token | Ollama `client.Chat()` with `ChatResponseFunc` callback provides per-token streaming. Accumulate `resp.Message.Content` in callback, print each chunk to stdout. |
| CHAT-04 | User can select which Ollama model to use (CLI flag or runtime command) | `client.List()` returns `ListResponse.Models` with model names. `--host` flag via `flag` package. `/model` slash command parsed in REPL loop. |
| CHAT-05 | Model responses render with markdown formatting and syntax-highlighted code blocks | `charm.land/glamour/v2` with `NewTermRenderer` renders markdown with syntax highlighting via Chroma. Apply after streaming completes. |
</phase_requirements>

## Standard Stack

### Core (Phase 1 specific)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24+ (1.24.3 installed) | Application language | Installed on system. Ollama module requires 1.24 minimum. |
| github.com/ollama/ollama/api | v0.20.5 | Ollama client -- chat, streaming, model listing | Official Go client. `Chat()` method with `ChatResponseFunc` callback for streaming. `List()` for model discovery. Published Apr 9, 2026. |
| charm.land/glamour/v2 | v2.0.0 | Markdown rendering in terminal | Stylesheet-based markdown renderer with syntax highlighting via Chroma. Pure rendering (deterministic output). Published Mar 9, 2026. |
| charm.land/lipgloss/v2 | v2.0.2 | Terminal text styling | Style prompts, spinner text, status indicators. Automatic color downsampling to terminal capabilities. Published Mar 11, 2026. |
| github.com/chzyer/readline | v1.5.1 | REPL line editing | Pure Go readline with history, prompt customization, concurrent-safe `Stdout()` writer. 2,840+ importers. Stable. |
| github.com/briandowns/spinner | v1.23.2 | Thinking indicator animation | 90 charset options, goroutine-safe Start/Stop, prefix/suffix text. Published Jan 20, 2025. Lightweight, no framework dependency. |
| log/slog | stdlib | Structured logging | Zero-dependency structured logging from Go stdlib. |

### Dev/Test
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/stretchr/testify | v1.11.1 | Test assertions | `assert` and `require` packages for readable test assertions. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| briandowns/spinner | Hand-rolled goroutine spinner | Spinner library handles terminal cleanup, multiple charsets, and edge cases. Not worth hand-rolling for 3 lines of setup. |
| glamour post-render | mdterm (streaming markdown) | mdterm has 1 star, 3 commits, no releases. Far too immature. Glamour is battle-tested (used by GitHub CLI, GitLab CLI). |
| chzyer/readline | ergochat/readline (maintained fork) | ergochat fork has more recent fixes but less adoption. chzyer is stable and sufficient. Switch only if bugs are hit. |
| Two-phase render | Full incremental markdown render | Charmbracelet closed their own streaming markdown PR (glow #823) citing it as unsolved. Two-phase is the pragmatic choice. |

**Installation:**
```bash
go mod init github.com/user/fenec
go get github.com/ollama/ollama@v0.20.5
go get charm.land/glamour/v2@v2.0.0
go get charm.land/lipgloss/v2@v2.0.2
go get github.com/chzyer/readline@v1.5.1
go get github.com/briandowns/spinner@v1.23.2
go get github.com/stretchr/testify@v1.11.1
```

## Architecture Patterns

### Recommended Project Structure
```
fenec/
├── main.go              # Entry point: flag parsing, client init, run REPL
├── go.mod
├── go.sum
├── internal/
│   ├── chat/
│   │   ├── client.go    # Ollama client wrapper (Chat, List models)
│   │   ├── client_test.go
│   │   ├── message.go   # Message types, conversation history
│   │   └── stream.go    # Streaming callback, token accumulation
│   ├── repl/
│   │   ├── repl.go      # REPL loop: readline, command dispatch
│   │   ├── repl_test.go
│   │   ├── commands.go  # Slash command handlers (/quit, /model, /help)
│   │   └── pager.go     # Auto-paging for long responses
│   ├── render/
│   │   ├── render.go    # Glamour markdown rendering, two-phase strategy
│   │   ├── render_test.go
│   │   ├── spinner.go   # Thinking indicator management
│   │   └── style.go     # Lipgloss style definitions
│   └── config/
│       ├── config.go    # System prompt loading, host configuration
│       └── config_test.go
├── Taskfile.yml         # Build, test, lint, run tasks
└── .golangci.yml        # Linter configuration
```

### Pattern 1: Streaming Chat with Two-Phase Rendering
**What:** Accumulate tokens during streaming, re-render with glamour after completion.
**When to use:** Every chat response.
**Example:**
```go
// Source: Ollama API docs (https://pkg.go.dev/github.com/ollama/ollama/api)
func (c *ChatClient) StreamChat(ctx context.Context, messages []api.Message, model string) (*api.Message, error) {
    var fullContent strings.Builder
    
    req := &api.ChatRequest{
        Model:    model,
        Messages: messages,
    }

    // Phase 1: Stream raw tokens to terminal
    err := c.client.Chat(ctx, req, func(resp api.ChatResponse) error {
        if resp.Message.Content != "" {
            fmt.Print(resp.Message.Content) // Raw output during streaming
            fullContent.WriteString(resp.Message.Content)
        }
        return nil
    })
    if err != nil {
        return nil, err
    }

    // Phase 2: Replace raw output with glamour-rendered markdown
    rendered := renderMarkdown(fullContent.String())
    overwriteOutput(rendered)

    return &api.Message{
        Role:    "assistant",
        Content: fullContent.String(),
    }, nil
}
```

### Pattern 2: ANSI Overwrite for Post-Stream Rendering
**What:** Count lines of raw output, move cursor up, clear, write rendered output.
**When to use:** After streaming completes, to replace raw text with formatted markdown.
**Example:**
```go
func overwriteOutput(rawLineCount int, rendered string) {
    // Move cursor up to start of raw output
    fmt.Printf("\033[%dA", rawLineCount)
    // Clear from cursor to end of screen
    fmt.Print("\033[J")
    // Print rendered markdown
    fmt.Print(rendered)
}
```

### Pattern 3: Ollama Client Initialization with Host Override
**What:** Support `--host` flag and fallback to OLLAMA_HOST env var or default localhost.
**When to use:** Application startup.
**Example:**
```go
// Source: Ollama API docs (https://pkg.go.dev/github.com/ollama/ollama/api)
func newOllamaClient(hostFlag string) (*api.Client, error) {
    if hostFlag != "" {
        u, err := url.Parse(hostFlag)
        if err != nil {
            return nil, fmt.Errorf("invalid host URL: %w", err)
        }
        return api.NewClient(u, http.DefaultClient), nil
    }
    // Falls back to OLLAMA_HOST env var, then localhost:11434
    return api.ClientFromEnvironment()
}
```

### Pattern 4: Readline REPL with Concurrent Streaming Output
**What:** Use readline's `Stdout()` writer for streaming output to avoid corrupting the prompt.
**When to use:** During the REPL loop when streaming responses while readline is active.
**Example:**
```go
// Source: chzyer/readline docs (https://pkg.go.dev/github.com/chzyer/readline)
rl, _ := readline.NewEx(&readline.Config{
    Prompt:          "[gemma4]> ",
    HistoryFile:     filepath.Join(configDir, "history"),
    InterruptPrompt: "^C",
    EOFPrompt:       "exit",
})
defer rl.Close()

for {
    line, err := rl.Readline()
    if err == readline.ErrInterrupt {
        continue // D-04: Ctrl+C clears current input
    }
    if err == io.EOF {
        break // D-04: Ctrl+D exits
    }
    // Process line...
    // Use rl.Stdout() for writing streaming output
}
```

### Pattern 5: Spinner for Thinking Indicator
**What:** Show animated spinner between user Enter and first token.
**When to use:** After sending message, before first streaming token arrives.
**Example:**
```go
// Source: briandowns/spinner docs (https://pkg.go.dev/github.com/briandowns/spinner)
s := spinner.New(spinner.CharSets[11], 80*time.Millisecond) // Braille dots
s.Suffix = " Thinking..."
s.Writer = rl.Stdout() // Write to readline's stdout to avoid corruption
s.Start()

// In the streaming callback, on first token:
var once sync.Once
callback := func(resp api.ChatResponse) error {
    once.Do(func() {
        s.Stop()
        // Clear spinner line
    })
    fmt.Fprint(rl.Stdout(), resp.Message.Content)
    fullContent.WriteString(resp.Message.Content)
    return nil
}
```

### Pattern 6: Slash Command Dispatch
**What:** Parse `/command` prefix and dispatch to handlers.
**When to use:** In the REPL loop before sending to model.
**Example:**
```go
func handleInput(line string) (handled bool) {
    line = strings.TrimSpace(line)
    if !strings.HasPrefix(line, "/") {
        return false
    }
    parts := strings.Fields(line)
    switch parts[0] {
    case "/quit":
        os.Exit(0)
    case "/model":
        handleModelCommand(parts[1:])
    case "/help":
        printHelp()
    default:
        fmt.Printf("Unknown command: %s\n", parts[0])
    }
    return true
}
```

### Anti-Patterns to Avoid
- **Do NOT attempt incremental glamour rendering during streaming:** Glamour requires complete markdown blocks. Partial code fences, incomplete lists, and mid-table tokens produce broken output. Even Charmbracelet has not solved this (glow PR #823 closed Sep 2025).
- **Do NOT write to os.Stdout directly while readline is active:** Use `rl.Stdout()` to prevent prompt corruption from concurrent writes.
- **Do NOT use bubbletea for the REPL:** It is a full TUI framework and overkill for a readline-based chat. The project constraints explicitly exclude it for now.
- **Do NOT hardcode a default model name:** Use `client.List()` to discover available models at runtime (D-09).

## D-06 Reconciliation: Progressive Rendering

User decision D-06 requests "live progressive markdown rendering as tokens stream in." Research shows this is an unsolved problem at production quality:

- Charmbracelet's own streaming markdown PR for glow (#823) was closed without merge (Sep 2025), citing need for new infrastructure.
- mdterm (the only Go library attempting this) has 1 star and 3 commits -- not production ready.
- Ollama's own CLI does NOT render markdown during streaming -- it outputs raw text.

**Recommended interpretation of D-06:** Stream raw tokens live (providing immediate feedback), then post-render the complete response with glamour. This gives the user the "live" feel of seeing content appear token-by-token while ensuring reliable, correctly-formatted markdown output. The brief re-render at the end (using ANSI cursor movement to overwrite raw text) is nearly imperceptible for short responses and clearly beneficial for long ones.

**If the user insists on true incremental rendering:** The fallback approach is to accumulate tokens until markdown block boundaries (blank lines, fence closings) and render completed blocks incrementally with glamour. This is more complex and prone to edge cases but achievable for common markdown structures.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Terminal spinner animation | Custom goroutine with ticker and print | `briandowns/spinner` | Handles terminal cleanup, cursor hide/show, 90 charsets, writer integration. Edge cases around terminal reset on crash. |
| Markdown to ANSI rendering | Custom markdown parser with ANSI codes | `charm.land/glamour/v2` | Markdown parsing is deceptively complex (nested lists, fenced code, tables, links). Glamour uses goldmark + Chroma for correct rendering. |
| Readline with history | `bufio.Scanner` + manual history | `github.com/chzyer/readline` | History persistence, concurrent-safe stdout, Ctrl+C/Ctrl+D handling, prompt refresh -- all non-trivial to implement correctly. |
| Terminal text styling | Manual ANSI escape codes | `charm.land/lipgloss/v2` | Color downsampling to terminal capabilities, cross-platform support, composable styles. |
| Flag parsing | Custom argument parsing | `flag` stdlib | Simple, sufficient for `--host` and `--model` flags. No need for cobra with 2 flags. |

**Key insight:** The terminal is a hostile environment for output formatting. Character widths, ANSI escape handling, color support detection, and cursor management are all fraught with edge cases. Use libraries that handle these.

## Common Pitfalls

### Pitfall 1: Ollama Not Running at Startup
**What goes wrong:** Application panics or shows cryptic connection error.
**Why it happens:** Ollama is an external runtime dependency that may not be started.
**How to avoid:** Call `client.List()` at startup as a health check. If it returns an error, print a clear message: "Cannot connect to Ollama at {host}. Is it running? Start with: ollama serve" and exit with non-zero status (D-14).
**Warning signs:** `connection refused` or timeout errors from the HTTP client.

### Pitfall 2: Readline Prompt Corruption During Streaming
**What goes wrong:** Streaming output interleaves with the readline prompt, producing garbled terminal state.
**Why it happens:** Writing to os.Stdout while readline is waiting for input on the same terminal.
**How to avoid:** Always use `rl.Stdout()` for writing output during the REPL session. The readline library manages terminal state and refreshes the prompt correctly.
**Warning signs:** Prompt appears mid-response, input characters mixed with output.

### Pitfall 3: Context Cancellation Not Stopping Stream
**What goes wrong:** User presses Escape to cancel generation, but tokens keep arriving.
**Why it happens:** The Ollama `client.Chat()` call must be given a cancellable context, and the context must be cancelled.
**How to avoid:** Use `context.WithCancel()` for each chat call. Wire Escape key to call the cancel function. The Chat callback should also check `ctx.Err()` and return an error if cancelled.
**Warning signs:** Escape key seems to do nothing, response continues after cancellation.

### Pitfall 4: Glamour Rendering Width Mismatch
**What goes wrong:** Rendered markdown wraps awkwardly or is too narrow/wide for the terminal.
**Why it happens:** Glamour's `WithWordWrap()` defaults may not match the current terminal width.
**How to avoid:** Query terminal width with `os.Stdout.Fd()` and `unix.IoctlGetWinsize` (or `golang.org/x/term`) and pass it to `glamour.WithWordWrap(termWidth)`. Re-query on SIGWINCH for terminal resize.
**Warning signs:** Text wraps mid-word or leaves large right margins.

### Pitfall 5: Empty Model List from Ollama
**What goes wrong:** `client.List()` returns zero models, application has nothing to default to.
**Why it happens:** Ollama is running but no models have been pulled.
**How to avoid:** Check `len(models) == 0` after listing. Print a helpful message: "No models installed. Pull one with: ollama pull gemma4" and exit.
**Warning signs:** Index-out-of-range panic when trying to select first model.

### Pitfall 6: Multi-line Input Handling with Backslash Continuation
**What goes wrong:** Backslash at end of line is sent literally to the model instead of continuing input.
**Why it happens:** readline returns the full line including the trailing backslash. Must be handled explicitly in the REPL loop.
**How to avoid:** After `rl.Readline()`, check if the trimmed line ends with `\`. If so, strip the backslash, change prompt to a continuation indicator (e.g., `... `), and accumulate lines until one doesn't end with `\`.
**Warning signs:** Model receives messages with trailing backslashes.

### Pitfall 7: Ollama Module Dependency Weight
**What goes wrong:** `go get github.com/ollama/ollama` pulls in hundreds of transitive dependencies including CUDA bindings and ML frameworks.
**Why it happens:** The Ollama module's go.mod includes the full server dependency tree.
**How to avoid:** Import ONLY `github.com/ollama/ollama/api` -- never import the root module. Go's module system only downloads what the imported packages actually need. The `/api` subpackage is lightweight.
**Warning signs:** go.sum grows to thousands of entries, build takes unusually long.

## Code Examples

### Complete Ollama Streaming Chat Call
```go
// Source: https://pkg.go.dev/github.com/ollama/ollama/api (v0.20.5)
package chat

import (
    "context"
    "fmt"
    "strings"

    "github.com/ollama/ollama/api"
)

type ChatResponseFunc = api.ChatResponseFunc

func StreamChat(ctx context.Context, client *api.Client, model string, messages []api.Message) (*api.Message, error) {
    var content strings.Builder

    req := &api.ChatRequest{
        Model:    model,
        Messages: messages,
    }

    err := client.Chat(ctx, req, func(resp api.ChatResponse) error {
        if resp.Message.Content != "" {
            content.WriteString(resp.Message.Content)
        }
        if resp.Done {
            // Final response -- contains metrics (eval count, duration, etc.)
        }
        return ctx.Err() // Stop early if context cancelled
    })
    if err != nil {
        return nil, err
    }

    return &api.Message{
        Role:    "assistant",
        Content: content.String(),
    }, nil
}
```

### Listing Available Models
```go
// Source: https://pkg.go.dev/github.com/ollama/ollama/api (v0.20.5)
func ListModels(ctx context.Context, client *api.Client) ([]string, error) {
    resp, err := client.List(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list models: %w", err)
    }

    names := make([]string, 0, len(resp.Models))
    for _, m := range resp.Models {
        names = append(names, m.Name)
    }
    return names, nil
}
```

### Glamour Markdown Rendering
```go
// Source: https://pkg.go.dev/charm.land/glamour/v2 (v2.0.0)
package render

import (
    "charm.land/glamour/v2"
)

func RenderMarkdown(content string, width int) (string, error) {
    r, err := glamour.NewTermRenderer(
        glamour.WithStandardStyle("dark"),
        glamour.WithWordWrap(width),
    )
    if err != nil {
        return "", err
    }
    return r.Render(content)
}
```

### Lipgloss Styled Prompt
```go
// Source: https://pkg.go.dev/charm.land/lipgloss/v2 (v2.0.2)
package render

import (
    "fmt"
    "charm.land/lipgloss/v2"
)

var (
    modelStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#7D56F4")).
        Bold(true)

    promptStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FAFAFA"))
)

func FormatPrompt(modelName string) string {
    return fmt.Sprintf("[%s]> ", modelStyle.Render(modelName))
}
```

### System Prompt Loading
```go
package config

import (
    "os"
    "path/filepath"
)

const defaultSystemPrompt = `You are Fenec, a helpful AI assistant running locally. Be concise and direct.`

func LoadSystemPrompt() (string, error) {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return defaultSystemPrompt, nil
    }

    path := filepath.Join(configDir, "fenec", "system.md")
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return defaultSystemPrompt, nil
        }
        return "", err
    }
    return string(data), nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `github.com/charmbracelet/glamour` | `charm.land/glamour/v2` | Mar 2026 | Vanity import path, "pure" deterministic rendering, no `WithAutoStyle()`. Must explicitly set style. |
| `github.com/charmbracelet/lipgloss` | `charm.land/lipgloss/v2` | Mar 2026 | Vanity import path, new Cursed Renderer. `NewStyle()` replaces `lipgloss.Style{}`. |
| Ollama SDK `v0.5.x` | `v0.20.x` | Early 2026 | Major version jump. `Think` field added to ChatRequest. `ToolCallID` added to Message. Logprobs support added. |
| `go get github.com/jmorganca/ollama` | `go get github.com/ollama/ollama` | 2024 | Module path changed from jmorganca to ollama organization. |

**Deprecated/outdated:**
- `glamour.WithAutoStyle()`: Removed in v2. Use `WithStandardStyle("dark")` or `WithStandardStyle("light")` explicitly.
- `glamour.WithColorProfile()`: Removed in v2. Color adaptation is now the responsibility of lipgloss.
- `github.com/charmbracelet/*` import paths: All Charm v2 libraries use `charm.land/*` vanity domain.

## Open Questions

1. **Escape Key Detection During Streaming**
   - What we know: readline captures Escape but only when readline is actively reading. During streaming, readline is not reading input.
   - What's unclear: Whether we need raw terminal input reading alongside readline, or if readline provides a mechanism for background key capture.
   - Recommendation: During streaming, temporarily switch to raw terminal mode (using `golang.org/x/term`) to capture Escape. Restore readline's terminal state after streaming completes. Alternatively, use Ctrl+C (SIGINT) for cancellation during streaming since readline is not reading -- this is simpler and well-understood.

2. **Auto-Pager for Long Responses (D-08)**
   - What we know: User wants "more"-style paging for long responses. Terminal height is detectable.
   - What's unclear: Whether to implement a custom pager or pipe to system `less`. Custom pager needs raw terminal mode for key capture.
   - Recommendation: Implement a simple custom pager that counts rendered lines vs terminal height. When threshold reached, pause output and show `--more--` prompt. Enter continues, q stops. This avoids shelling out to `less` which would lose ANSI styling context.

3. **Spinner Writer Compatibility with Readline**
   - What we know: `briandowns/spinner` accepts a custom `io.Writer`. Readline provides `rl.Stdout()`.
   - What's unclear: Whether spinner's cursor-hide/show ANSI codes interact well with readline's terminal management.
   - Recommendation: Test early. If conflicts arise, implement a minimal hand-rolled spinner using `rl.Stdout()` directly (just a goroutine with a ticker printing braille characters).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed -- Go's testing uses convention (`*_test.go` files) |
| Quick run command | `go test ./...` |
| Full suite command | `go test -race -cover ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CHAT-01 | Streaming chat sends messages and receives token-by-token responses | integration | `go test ./internal/chat/ -run TestStreamChat -x` | No -- Wave 0 |
| CHAT-01 | Token accumulation produces complete response | unit | `go test ./internal/chat/ -run TestTokenAccumulation` | No -- Wave 0 |
| CHAT-04 | Model listing returns available models | unit | `go test ./internal/chat/ -run TestListModels` | No -- Wave 0 |
| CHAT-04 | /model command switches active model | unit | `go test ./internal/repl/ -run TestModelCommand` | No -- Wave 0 |
| CHAT-04 | --host flag overrides default connection | unit | `go test ./internal/chat/ -run TestClientHost` | No -- Wave 0 |
| CHAT-05 | Markdown rendering produces styled output | unit | `go test ./internal/render/ -run TestRenderMarkdown` | No -- Wave 0 |
| CHAT-05 | Code blocks have syntax highlighting | unit | `go test ./internal/render/ -run TestCodeBlockRendering` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./...`
- **Per wave merge:** `go test -race -cover ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `go.mod` -- module initialization
- [ ] `Taskfile.yml` -- task runner with build, test, lint targets
- [ ] `.golangci.yml` -- linter configuration
- [ ] `internal/chat/client_test.go` -- covers CHAT-01 (streaming), CHAT-04 (model list, host)
- [ ] `internal/repl/repl_test.go` -- covers CHAT-04 (slash commands)
- [ ] `internal/render/render_test.go` -- covers CHAT-05 (markdown rendering)
- [ ] `internal/config/config_test.go` -- covers system prompt loading
- [ ] Test helper: mock Ollama client interface for unit tests without running Ollama

## Sources

### Primary (HIGH confidence)
- [Ollama Go API package v0.20.5](https://pkg.go.dev/github.com/ollama/ollama/api) -- Chat, List, Message types, streaming callback pattern
- [Ollama API client.go source](https://github.com/ollama/ollama/blob/main/api/client.go) -- NewClient and ClientFromEnvironment constructors
- [Glamour v2 package](https://pkg.go.dev/charm.land/glamour/v2) -- v2.0.0, TermRenderer API, style options
- [Glamour v2 Upgrade Guide](https://github.com/charmbracelet/glamour/blob/main/UPGRADE_GUIDE_V2.md) -- Breaking changes: removed WithAutoStyle, pure rendering
- [Lipgloss v2 package](https://pkg.go.dev/charm.land/lipgloss/v2) -- v2.0.2, NewStyle API, color handling
- [chzyer/readline package](https://pkg.go.dev/github.com/chzyer/readline) -- v1.5.1, Config, Stdout(), concurrent writes
- [briandowns/spinner package](https://pkg.go.dev/github.com/briandowns/spinner) -- v1.23.2, CharSets, Start/Stop API

### Secondary (MEDIUM confidence)
- [Glow streaming markdown PR #823](https://github.com/charmbracelet/glow/pull/823) -- Closed Sep 2025, confirms incremental markdown rendering is unsolved at production quality
- [Ollama CLI cmd.go](https://github.com/ollama/ollama/blob/main/cmd/cmd.go) -- Ollama's own CLI does NOT use glamour, uses raw text streaming with word wrap
- [Ollama streaming docs](https://docs.ollama.com/capabilities/streaming) -- Streaming patterns (Python/JS examples, Go pattern inferred from API types)
- [simonw/llm PR #571](https://github.com/simonw/llm/pull/571) -- Rich library "accumulate and re-render" pattern for streaming markdown in Python

### Tertiary (LOW confidence)
- [mdterm library](https://github.com/mkozhukh/mdterm) -- Streaming terminal markdown for Go, too immature (1 star, 3 commits) for production use

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- All packages verified on pkg.go.dev with exact versions and publish dates
- Architecture: HIGH -- Patterns based on verified API types and official documentation
- Streaming strategy: HIGH -- Validated by examining Ollama's own CLI approach and Charmbracelet's closed PR
- Pitfalls: HIGH -- Derived from documented library behaviors and known integration patterns
- D-06 progressive rendering: MEDIUM -- Two-phase approach is well-understood but user decision may need reconciliation

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- all libraries are released versions)
