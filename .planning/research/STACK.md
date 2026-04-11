# Stack Research

**Domain:** Go-based AI agent platform with Lua extensibility and local Ollama inference
**Researched:** 2026-04-11
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.24+ (target 1.26) | Application language | Single-binary deployment, strong concurrency for streaming, excellent embedding story for Lua. Go 1.26 is current stable (1.26.2 released Apr 7, 2026). Use 1.24 as minimum since the Ollama module requires it. |
| github.com/ollama/ollama/api | v0.20.x | Ollama client — chat, streaming, tool calling | The official Go client used by the Ollama CLI itself. Provides typed ChatRequest/ChatResponse with native Tool, ToolCall, and Message types. Guaranteed API compatibility since Ollama dogfoods it. No lighter alternative supports tool calling with the same type safety. |
| github.com/yuin/gopher-lua | v1.1.2 | Embedded Lua VM | The dominant Lua-in-Go library (2,345+ importers). Implements Lua 5.1 with goto from 5.2. 20% faster than Shopify's go-lua. Clean API for exposing Go functions to Lua via LGFunction. Supports selective module loading for sandboxing. |
| log/slog | stdlib (Go 1.21+) | Structured logging | Standard library, zero dependencies. Sufficient performance for a CLI agent. Avoids pulling in zerolog/zap for a tool that does not need extreme throughput logging. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/vadv/gopher-lua-libs | latest | Lua stdlib extensions (JSON, HTTP, filesystem, YAML) | Provides pre-built Go-backed Lua modules so agent-authored Lua scripts can parse JSON, make HTTP requests, read files. Cherry-pick modules -- do not import the entire library. |
| charm.land/glamour/v2 | v2.x | Markdown rendering in terminal | Render model responses with syntax highlighting, lists, code blocks. Makes streaming chat output readable. |
| charm.land/lipgloss/v2 | v2.x | Terminal styling | Style prompts, status indicators, tool call output. Lightweight companion to glamour. |
| github.com/chzyer/readline | v1.5.x | Line editing for REPL | Provides history, multi-line input, completion support for the CLI chat. Mature (most popular Go readline). Use this over bubbletea for a simple REPL -- bubbletea is overkill until you need a full TUI. |
| github.com/stretchr/testify | v1.9.x | Test assertions and mocking | Standard Go test helper. Use assert/require packages. Do not use suite -- it adds unnecessary structure. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| golangci-lint | Linting | Use default config plus govet, errcheck, staticcheck, gosimple. Run in CI and pre-commit. |
| goreleaser | Binary releases | Produces cross-platform single binaries. Configure for linux/darwin amd64/arm64. |
| Taskfile (go-task) | Task runner | Cleaner than Makefiles for Go projects. Define build, test, lint, run tasks. |
| Ollama (local) | Model serving | Required runtime dependency. Agent connects to localhost:11434 by default. |

## Installation

```bash
# Initialize module
go mod init github.com/marad/fenec

# Core dependencies
go get github.com/ollama/ollama@latest
go get github.com/yuin/gopher-lua@v1.1.2

# Lua standard library extensions (import selectively)
go get github.com/vadv/gopher-lua-libs@latest

# Terminal UI
go get charm.land/glamour/v2@latest
go get charm.land/lipgloss/v2@latest

# REPL
go get github.com/chzyer/readline@latest

# Dev dependencies
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/go-task/task/v3/cmd/task@latest
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| github.com/ollama/ollama/api | github.com/openai/openai-go/v3 (pointed at Ollama) | If you need to support multiple backends (OpenAI, Anthropic, Ollama) behind one interface. The OpenAI-compatible endpoint supports tool calling. Trade-off: you lose Ollama-specific features (model management, Think/reasoning control) and gain portability. For a local-first personal agent, the native client is better. |
| github.com/ollama/ollama/api | github.com/rozoomcool/go-ollama-sdk | If the official module's transitive dependency weight becomes a problem. This is a lightweight typed client. Trade-off: smaller community, possibly slower to adopt new Ollama features like streaming tool calls. |
| github.com/yuin/gopher-lua | github.com/Shopify/go-lua | If you need Lua 5.2 compatibility specifically. Trade-off: 20% slower, no coroutine support, missing string pattern matching. Not worth it for this project. |
| github.com/yuin/gopher-lua | Embedded JavaScript (goja) | If your target users know JS better than Lua. Trade-off: heavier runtime, less predictable performance. Lua's simplicity is a feature for agent-authored code -- smaller surface area means fewer ways the LLM can write broken scripts. |
| charm.land/glamour/v2 | Plain fmt.Println | For the initial MVP, plain output is fine. Add glamour when streaming responses look ugly without markdown rendering. |
| github.com/chzyer/readline | charm.land/bubbletea/v2 | When you want a full TUI with panels, status bar, history pane. PROJECT.md explicitly puts TUI out of scope for now. Start with readline REPL, upgrade to bubbletea in a later milestone. |
| log/slog | github.com/rs/zerolog | If you find slog's performance insufficient or need structured JSON logs for a log aggregator. Unlikely for a personal CLI tool. |
| github.com/stretchr/testify | Standard testing only | Testify is not strictly necessary -- Go's testing package works fine. But assert/require reduce boilerplate significantly. Low cost, high value. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| LangChainGo (github.com/tmc/langchaingo) | Massive abstraction layer designed for multi-provider, multi-chain orchestration. Pulls in 10+ provider SDKs. You are building a single-provider (Ollama) agent with a custom tool system -- LangChain's abstractions will fight your architecture rather than help it. | Direct Ollama API client + custom tool registry |
| github.com/spf13/cobra | Overkill for a single-command REPL application. Cobra is for kubectl-style CLI tools with many subcommands. Fenec is `fenec` (run) or `fenec chat` at most. | Simple flag parsing with stdlib `flag` package, or a thin wrapper. Add cobra later only if you grow beyond 3-4 subcommands. |
| github.com/spf13/viper | Heavy config library with many transitive deps. A personal agent needs a config file and env vars, not Consul/etcd/remote config support. | TOML/YAML file parsed with encoding/json or github.com/BurntSushi/toml. Environment variable overrides via os.Getenv. |
| cgo-based LuaJIT bindings (e.g., aarzilli/golua) | Requires cgo, which breaks cross-compilation and complicates builds. The PROJECT.md says "LuaJIT" but gopher-lua (pure Go, Lua 5.1) gives you the extensibility benefits without the build complexity. True LuaJIT is only worth it if Lua script performance is a bottleneck -- for tool scripts that mostly call Go functions, it will not be. | github.com/yuin/gopher-lua (pure Go Lua 5.1 VM) |
| Database (SQLite, Postgres) for conversation storage | Premature. Start with in-memory conversation history. Persist to JSON files if needed. A database adds migration complexity before you know what schema you need. | In-memory []Message slice, optional JSON file persistence |
| MCP (Model Context Protocol) client | MCP is a server-side protocol for exposing tools to LLM hosts. Fenec IS the host. You might want to be an MCP server later (so other tools can use your Lua tools), but you do not need an MCP client. | Custom tool registry with a simple interface |

## Stack Patterns

**For the tool calling loop:**
- Use the Ollama native API with Tools field on ChatRequest
- Parse ToolCalls from ChatResponse.Message.ToolCalls
- Execute the tool (built-in Go handler or Lua script via gopher-lua)
- Append tool result as a Message with Role "tool" and the ToolCallID
- Send the updated messages back for the model's final response
- This is the standard agentic loop pattern and maps directly onto the Ollama API types

**For Lua sandboxing:**
- Create LState with `lua.Options{SkipOpenLibs: true}`
- Selectively open only safe libraries: base, table, string, math
- Do NOT open os, io, or debug libraries by default
- Expose controlled Go functions (e.g., `fenec.http_get`, `fenec.read_file`) that you audit
- Set execution timeouts via context cancellation on the LState

**For self-extension (agent writes new tools):**
- Agent outputs Lua source code in a structured format
- Fenec validates the script (syntax check via gopher-lua parse, then sandboxed test run)
- Script saved to `~/.fenec/tools/` with a metadata header (name, description, parameters)
- On startup, scan the tools directory, parse metadata, register each as an available tool
- Tool definitions injected into the system prompt so the model sees them

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| github.com/ollama/ollama v0.20.x | Go 1.24+ | Ollama module requires Go 1.24.1 minimum per its go.mod. This sets our floor. |
| github.com/yuin/gopher-lua v1.1.2 | Go 1.20+ | No strict minimum documented, but v1.1.x works with modern Go. |
| charm.land/glamour/v2 | Go 1.23+ | Charm v2 libraries use vanity import paths (charm.land, not github.com/charmbracelet). |
| charm.land/lipgloss/v2 | Go 1.23+ | Same vanity domain pattern as glamour. |
| github.com/chzyer/readline v1.5.x | Go 1.18+ | Stable, minimal dependencies. |
| Ollama server v0.20+ | Gemma 4 (all sizes) | Gemma 4 launched Apr 2, 2026 with day-0 Ollama support. Tool calling works via /api/chat. |
| Gemma 4 (26B MoE, 31B Dense) | Tool calling | Native function calling support. The E2B/E4B edge models also support it but with lower quality. Recommend 26B MoE for best quality-to-resource ratio. |

## Key Technical Notes

### Ollama API Dependency Weight
The `github.com/ollama/ollama` module is the full Ollama codebase (~80+ transitive dependencies). Go only compiles what you import, so the binary size impact is limited to the `api` package and its direct deps. However, `go mod tidy` will download more than you need. This is an acceptable trade-off for guaranteed API compatibility. If it becomes painful, the OpenAI-compatible endpoint via `github.com/openai/openai-go/v3` is a viable escape hatch.

### LuaJIT Naming Clarification
The PROJECT.md references "LuaJIT" but the recommended stack uses gopher-lua (pure Go Lua 5.1). This is intentional. True LuaJIT requires cgo and a C compiler, breaking Go's cross-compilation story. Gopher-lua provides the same extensibility model (Lua 5.1 scripts, Go function bindings) without the build complexity. The performance difference is irrelevant -- tool scripts spend their time in Go-implemented functions, not in Lua computation.

### Streaming Architecture
The Ollama API streams responses via a callback function (`ChatResponseFunc`). Each callback invocation delivers a partial response. For the REPL, print each chunk as it arrives. For tool calls, the model will emit a complete Message with ToolCalls in the final streaming chunk (where `Done: true`). Design the chat loop to buffer tool calls and dispatch them after streaming completes.

## Sources

- [Ollama Go API package](https://pkg.go.dev/github.com/ollama/ollama/api) -- v0.20.5, types verified (HIGH confidence)
- [Ollama tool calling docs](https://docs.ollama.com/capabilities/tool-calling) -- official documentation (HIGH confidence)
- [Ollama OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility) -- endpoint and feature matrix (HIGH confidence)
- [Tool calling Go implementation](https://dev.to/calvinmclean/how-to-implement-llm-tool-calling-with-go-and-ollama-237g) -- code patterns verified against API types (MEDIUM confidence)
- [GopherLua GitHub](https://github.com/yuin/gopher-lua) -- v1.1.2, features and API verified (HIGH confidence)
- [GopherLua sandboxing](https://github.com/yuin/gopher-lua/issues/27) -- SkipOpenLibs approach (MEDIUM confidence)
- [gopher-lua-libs](https://github.com/vadv/gopher-lua-libs) -- Lua stdlib extensions (MEDIUM confidence)
- [Ollama Go SDK comparison](https://www.glukhov.org/post/2025/10/using-ollama-in-go/) -- alternative client analysis (MEDIUM confidence)
- [Charm v2 libraries](https://charm.land/libs/) -- import paths changed to charm.land vanity domain (HIGH confidence)
- [Bubbletea v2](https://pkg.go.dev/charm.land/bubbletea/v2) -- v2 import path verified (HIGH confidence)
- [Glamour v2](https://pkg.go.dev/charm.land/glamour/v2) -- markdown renderer (HIGH confidence)
- [Go 1.26 release](https://go.dev/blog/go1.26) -- current stable version (HIGH confidence)
- [Gemma 4 + Ollama](https://ghost.codersera.com/blog/how-to-run-gemma-4-with-ollama-setup-guide/) -- day-0 support, tool calling confirmed (HIGH confidence)
- [Go slog vs zerolog](https://leapcell.io/blog/high-performance-structured-logging-in-go-with-slog-and-zerolog) -- logging comparison (MEDIUM confidence)
- [Go CLI comparison](https://blog.jetbrains.com/go/2025/11/10/go-language-trends-ecosystem-2025/) -- Cobra/urfave ecosystem context (MEDIUM confidence)

---
*Stack research for: Go-based AI agent platform (Fenec)*
*Researched: 2026-04-11*
