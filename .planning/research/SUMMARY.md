# Project Research Summary

**Project:** Fenec
**Domain:** Local-first AI agent platform (Go + Lua + Ollama)
**Researched:** 2026-04-11
**Confidence:** HIGH

## Executive Summary

Fenec is a personal AI agent platform built in Go that connects to local Ollama models for inference, uses structured tool calling to act on the user's behalf, and differentiates itself through self-extension: the agent writes Lua scripts that persist as new tools, growing more capable over time. The stack is mature and well-supported — Go 1.24+, the official Ollama Go client, gopher-lua for embedded scripting, and Charm libraries for terminal rendering.

The dominant architecture pattern is a ReAct loop (reason-act-observe) with an interface-based tool registry that unifies Go built-in tools and Lua script tools behind the same contract. This is standard across every major agent framework and maps cleanly to the Go interface pattern.

Three critical risks require early mitigation: (1) Ollama silently truncates context windows, destroying tool definitions mid-conversation; (2) gopher-lua has no built-in sandboxing — all system libraries load by default; (3) local models produce unreliable tool call JSON that requires defensive parsing. All three have known solutions that must be implemented from Phase 1.

## Key Findings

### Recommended Stack

The stack avoids frameworks (no LangChainGo) in favor of direct Ollama integration. gopher-lua (Lua 5.1, not actual LuaJIT) is the correct choice — it's pure Go, no cgo required, and has strong sandboxing support via selective module loading.

**Core technologies:**
- **Go 1.24+** (target 1.26): Single-binary deployment, strong concurrency, excellent Lua embedding story
- **github.com/ollama/ollama/api v0.20.x**: Official client with native Tool, ToolCall, Message types — guaranteed API compatibility
- **github.com/yuin/gopher-lua v1.1.2**: Dominant Lua-in-Go library (2,345+ importers), Lua 5.1 with goto, 20% faster than alternatives
- **charm.land/glamour/v2**: Markdown rendering for model responses
- **github.com/chzyer/readline v1.5.x**: Line editing, history, completion for the REPL

**Key anti-recommendation:** Do NOT use LangChainGo — it's a multi-provider abstraction that fights a single-provider custom tool architecture. The Ollama API maps directly onto the agentic loop without any framework.

**Key clarification:** PROJECT.md says "LuaJIT" but there is no maintained LuaJIT binding for Go. gopher-lua implements Lua 5.1 and is the correct choice. Update PROJECT.md terminology.

### Expected Features

**Must have (table stakes):**
- Streaming response output (every CLI agent does this)
- Multi-turn conversation with context management
- Structured tool calling with result feedback loop
- Bash/shell command execution
- Human approval for dangerous operations
- Tool discovery in system prompt
- Graceful error handling (tool failures returned to model)
- Session persistence (save/load conversations)
- Configurable model selection
- Markdown/code rendering in terminal

**Should have (competitive):**
- Self-extending via Lua tool authoring (primary differentiator)
- Lua tool hot-reloading (use new tools immediately)
- Tool health monitoring (detect broken Lua tools)

**Defer (v2+):**
- MCP protocol support (enormous complexity, not needed for v1)
- Multi-agent orchestration
- RAG/knowledge base integration
- Email/notes integrations (explicitly out of scope per PROJECT.md)

### Architecture Approach

The system follows a ReAct loop pattern: CLI REPL receives user input, conversation manager builds context, system prompt builder injects available tools, the agent loop sends to Ollama, parses response for tool calls, dispatches via tool registry, feeds results back, and repeats until the model produces a final text response.

**Major components:**
1. **CLI REPL** — User input/output via readline
2. **Conversation Manager** — Message history, context window tracking, truncation
3. **Agent Loop (ReAct)** — Core reason-act-observe cycle with max iteration guard
4. **System Prompt Builder** — Injects tool definitions from registry
5. **Tool Registry** — Interface-based registry unifying Go and Lua tools
6. **Tool Dispatcher** — Parses tool calls, routes to handlers, returns results
7. **LLM Client** — Ollama API wrapper with streaming and explicit num_ctx
8. **Lua Runtime** — Sandboxed gopher-lua VM pool with Go host API
9. **Tool Persistence** — Disk storage for agent-authored Lua tools

### Critical Pitfalls

1. **Ollama silent context truncation** — Default 2048 context window is far too small. Set num_ctx explicitly (32768+ for agentic use). Track token usage and implement conversation compaction.
2. **Unsandboxed Lua execution** — gopher-lua loads os, io, debug by default. Use SkipOpenLibs and selectively load only safe modules. Expose host capabilities through registered Go functions.
3. **Tool call parsing fragility** — Local models produce malformed JSON, hallucinate tool names, format calls inconsistently. Defensive parsing with error recovery is mandatory.
4. **Agent infinite loops** — Without max_tool_calls, deduplication, and circuit breakers, the agent enters infinite retry loops. Must be in the execution engine from day one.
5. **LState is not goroutine-safe** — gopher-lua requires an LState pool pattern with proper lifecycle management. One LState per tool execution, not shared.
6. **Self-extension without validation** — Model-generated Lua scripts need syntax validation, schema validation, and smoke testing before persisting. Broken tools waste context window tokens.

## Implications for Roadmap

### Phase 1: Foundation and Interfaces
**Rationale:** Everything depends on core contracts — Tool interface, Ollama client, CLI shell. Zero cross-dependencies; can build in parallel.
**Delivers:** Project skeleton, Tool interface + Registry, Ollama client wrapper (with explicit num_ctx), basic streaming REPL
**Addresses:** Streaming output, model selection, markdown rendering
**Avoids:** Silent context truncation (by setting num_ctx from first call)

### Phase 2: Agent Core and Chat Loop
**Rationale:** Integrates foundation pieces into a working product. First usable artifact: multi-turn streaming chat.
**Delivers:** Conversation manager, system prompt builder, ReAct loop with max-iteration guard
**Addresses:** Multi-turn conversation, context management, tool discovery in prompt
**Avoids:** Agent infinite loops (max iterations from day one)

### Phase 3: Built-in Tools and Safety
**Rationale:** First agentic interaction — model calls tools, sees results, reasons about them.
**Delivers:** Bash tool with timeout/approval, tool dispatch wired into agent loop, error handling
**Addresses:** Bash execution, human approval, graceful error handling, tool calling with result feedback
**Avoids:** Command injection (structured execution, not raw shell interpolation)

### Phase 4: Lua Runtime and Tool Loading
**Rationale:** Most complex new component. Requires tool system to be stable before adding scripting layer.
**Delivers:** Sandboxed gopher-lua VM, LState pool, Go host API, Lua-to-Tool adapter, tool loader from disk
**Addresses:** Lua integration, tool loading
**Avoids:** Unsandboxed execution, LState goroutine safety issues

### Phase 5: Self-Extension
**Rationale:** The crown jewel, built last because it needs everything else working.
**Delivers:** write_tool built-in, validation pipeline (syntax + schema + smoke test), hot-reload, metadata tracking
**Addresses:** Self-extending via Lua authoring, hot-reloading, tool health monitoring
**Avoids:** Persistent broken tools (validation pipeline)

### Phase Ordering Rationale

- Each phase depends on the previous — no phase can be built without its predecessor
- Foundation (1) → Integration (2) → First tools (3) → Scripting layer (4) → Self-extension (5) follows a strict dependency chain
- Safety concerns (context management, sandboxing, approval gates) are addressed in the phase where they first become relevant, not deferred
- The MVP is usable after Phase 3 (chat + bash tools). Phase 4-5 add the differentiating feature.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 4:** Lua host API design — which Go functions to expose, LState pooling nuances, sandbox module enumeration
- **Phase 5:** Validation pipeline design, tool metadata format, system prompt engineering for Lua authoring quality

Phases with standard patterns (skip deep research):
- **Phase 1:** Standard Go interfaces + well-documented Ollama client
- **Phase 2:** Well-documented ReAct loop pattern
- **Phase 3:** Straightforward os/exec + confirmation prompt

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All libraries verified on pkg.go.dev with current versions |
| Features | HIGH | Cross-referenced against Claude Code, Codex CLI, Gemini CLI, OpenCode, Aider |
| Architecture | HIGH | ReAct loop is the dominant pattern across all agent frameworks |
| Pitfalls | HIGH | Verified via official Ollama docs, gopher-lua GitHub issues, community reports |

**Overall confidence:** HIGH

### Gaps to Address

- **Gemma 4 tool calling reliability:** Active compatibility issues with Ollama v0.20.0. Needs verification at implementation time.
- **Lua host API surface:** Which Go functions to expose to Lua scripts requires design work during Phase 4 planning.
- **Tool metadata format:** Whether to use .json sidecar files or embed schema in Lua source needs a design decision.
- **Conversation compaction strategy:** Simple truncation works for MVP, but summarization will be needed for real-world usage.

---
*Research completed: 2026-04-11*
*Ready for roadmap: yes*
