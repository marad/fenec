# Milestones

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
