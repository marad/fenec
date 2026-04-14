# Milestones

## v1.2 GitHub Models Provider (Shipped: 2026-04-14)

**Phases completed:** 2 phases, 4 plans, 4 tasks

**Key accomplishments:**

- GitHub Models `copilot` provider with zero-config auth: resolves token via GH_TOKEN → GITHUB_TOKEN → `gh auth token` priority chain with actionable error messages for missing/unauthenticated gh CLI
- Copilot provider wraps `openai.Provider` via delegation — full streaming, tool calling, and model routing work identically to the existing OpenAI adapter with no duplicated logic
- Catalog HTTP client with lazy double-checked locking cache fetching 40+ models from `https://models.github.ai/v1/models` — real context lengths from `limits.max_input_tokens` (e.g., gpt-4o-mini=131072, gpt-4.1=1048576)
- Ping validates auth and connectivity via single catalog fetch — no chat round-trip needed; 401 returns auth-specific error, network error returns connectivity error
- `/model` REPL correctly groups catalog entries under `copilot` heading with publisher-prefixed IDs (`openai/gpt-4o-mini`, `meta/llama-3.3-70b-instruct`)

---

## v1.1 Multi-Provider Support (Shipped: 2026-04-14)

**Phases completed:** 5 phases, 9 plans, 18 tasks

**Key accomplishments:**

- Fenec-owned Message, ToolDefinition, and StreamMetrics types with PropertyType custom JSON marshaling and full round-trip test coverage
- Full type decoupling from ollama/api -- only internal/chat retains the import as adapter boundary with 4 conversion functions
- Provider interface with 5 methods and Ollama adapter, moving all Ollama-specific code behind internal/provider/ollama while REPL and main.go consume only the abstract Provider
- TOML config loading with $ENV_VAR API key resolution, provider registry, and config-driven main.go startup replacing hardcoded Ollama
- fsnotify config file watcher with 100ms debounce, directory-level watch for editor atomic saves, and main.go reload callback that rebuilds providers on config change
- OpenAI-compatible Provider adapter with streaming/non-streaming dispatch, tool call argument parsing, and factory wiring via openai-go v3 SDK
- 26 unit tests for OpenAI adapter covering streaming SSE, non-streaming tool calls, thinking extraction, model listing, ping, metrics, and config factory wiring

---

## v1.0 Fenec Platform Foundation (Shipped: 2026-04-12)

**Phases completed:** 6 phases, 14 plans, 29 tasks

**Key accomplishments:**

- Ollama client wrapper with streaming chat, model listing, and conversation management using chatAPI interface pattern for testability
- Glamour markdown rendering with dark theme, braille spinner, lipgloss-styled prompt/banner, and system prompt loading from ~/.config/fenec/system.md
- Interactive REPL wiring chat engine and rendering into a runnable fenec binary with slash commands, streaming chat, and model selection
- StreamChat metrics capture, Show API context length discovery, and ContextTracker with threshold-based truncation
- File-based session persistence with atomic writes, auto-save, and JSON serialization using Ollama's api.Message type
- Context tracking, /save /load /history commands, and sync.Once auto-save wired into REPL and main.go
- Tool registry with generic interface for extensibility, shell_exec tool with timeout enforcement and dangerous-command safety gates using ApproverFunc callback pattern
- Agentic loop wired in REPL with StreamChat tool passing, tool call dispatch/result feeding, dangerous command approval via readline, and shell_exec registered in main.go
- Sandboxed gopher-lua VM with whitelist-only library access and LuaTool type implementing tool.Tool for executing Lua scripts as agent tools
- Directory scanner that loads .lua tools at startup with partial-success semantics and descriptive error reporting for broken scripts
- Three self-extension built-in tools (create/update/delete_lua_tool) with full Lua validation pipeline and Registry provenance tracking
- Self-extension tools wired into main.go with banner notifications, /tools command, and system prompt hot-reload for end-to-end tool lifecycle
- Path deny-list with symlink resolution, ReadFileTool with offset/limit/binary detection, ListDirTool with dirs-first sorting
- WriteFileTool with mkdir -p and approval gating, EditFileTool with first-occurrence replace preserving line endings, all four file tools wired into main.go

---
