# Roadmap: Fenec

## Overview

Fenec delivers a self-extending AI agent platform in five phases. Foundation establishes a working chat connection to local Ollama models with streaming output. Conversation adds multi-turn context management and session persistence. Tool Execution wires up the agentic loop -- structured tool calling, dispatch, and a bash tool with safety gates. Lua Runtime embeds a sandboxed scripting engine that loads user-authored tools alongside built-ins. Self-Extension completes the vision: the agent writes, validates, and hot-reloads its own Lua tools, growing more capable over time.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation** - Streaming chat with a local Ollama model, formatted output, model selection
- [x] **Phase 2: Conversation** - Multi-turn context management and persistent sessions (completed 2026-04-11)
- [ ] **Phase 3: Tool Execution** - Structured tool calling, bash command execution, and safety gates
- [ ] **Phase 4: Lua Runtime** - Sandboxed Lua scripting engine that loads tools from disk
- [ ] **Phase 5: Self-Extension** - Agent authors, validates, and hot-reloads its own Lua tools

## Phase Details

### Phase 1: Foundation
**Goal**: User can chat with a local Ollama model and see well-formatted streaming responses
**Depends on**: Nothing (first phase)
**Requirements**: CHAT-01, CHAT-04, CHAT-05
**Success Criteria** (what must be TRUE):
  1. User can type a message and see the response stream in token-by-token
  2. User can select which Ollama model to use via CLI flag or runtime command
  3. Model responses display with markdown formatting and syntax-highlighted code blocks
**Plans**: 3 plans

Plans:
- [x] 01-01-PLAN.md — Project scaffold, Go module, Ollama client with streaming chat
- [x] 01-02-PLAN.md — Markdown rendering, spinner, lipgloss styles, config/system prompt
- [x] 01-03-PLAN.md — REPL loop, slash commands, pager, main.go wiring, end-to-end verification

### Phase 2: Conversation
**Goal**: User can have sustained multi-turn conversations that survive application restarts
**Depends on**: Phase 1
**Requirements**: CHAT-02, CHAT-03, SESS-01, SESS-02
**Success Criteria** (what must be TRUE):
  1. Agent maintains conversation context -- earlier messages inform later responses across multiple turns
  2. Agent tracks token usage and truncates old messages when approaching model context limits
  3. User can save a conversation to disk and resume it in a later session
  4. Conversation auto-saves on exit so no data is lost on unexpected quit
**Plans**: 3 plans

Plans:
- [x] 02-01-PLAN.md — Context window management: StreamChat metrics capture, Show API context length, ContextTracker truncation
- [x] 02-02-PLAN.md — Session persistence: Session type, file-based Store with atomic writes, auto-save support
- [x] 02-03-PLAN.md — REPL integration: /save, /load, /history commands, auto-save on exit, main.go wiring

### Phase 3: Tool Execution
**Goal**: Agent can call tools, execute shell commands, and handle errors -- with human approval for dangerous operations
**Depends on**: Phase 2
**Requirements**: TOOL-01, TOOL-02, TOOL-03, EXEC-01, EXEC-02, EXEC-03
**Success Criteria** (what must be TRUE):
  1. Agent outputs structured tool calls that are parsed, dispatched, and their results fed back into the conversation
  2. Available tools are listed in the system prompt so the model knows what it can call
  3. Agent can execute a shell command and receive stdout, stderr, and exit code
  4. Dangerous operations (rm, sudo, file writes) prompt the user for approval before executing
  5. Shell commands that exceed a configurable timeout are killed and the timeout is reported to the model
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD

### Phase 4: Lua Runtime
**Goal**: Lua scripts on disk are loaded as first-class tools alongside Go built-ins
**Depends on**: Phase 3
**Requirements**: LUA-02, LUA-04, LUA-06
**Success Criteria** (what must be TRUE):
  1. Lua tools placed in the tools directory are loaded on startup and appear in the system prompt alongside built-in tools
  2. Lua execution is sandboxed -- scripts cannot access os, io, or debug modules directly
  3. Broken or malformed Lua tools are detected on load and reported to the user, not silently registered
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD

### Phase 5: Self-Extension
**Goal**: The agent can author new Lua tools that persist, validate, and become immediately usable -- the platform grows its own capabilities
**Depends on**: Phase 4
**Requirements**: LUA-01, LUA-03, LUA-05
**Success Criteria** (what must be TRUE):
  1. Agent can write a new Lua tool that is saved to the tools directory and persists across sessions
  2. Newly written Lua tools are validated (syntax check + schema check) before being persisted -- invalid tools are rejected with a clear error
  3. New Lua tools become available immediately in the current session without restart (hot-reload)
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete | 2026-04-11 |
| 2. Conversation | 3/3 | Complete   | 2026-04-11 |
| 3. Tool Execution | 0/0 | Not started | - |
| 4. Lua Runtime | 0/0 | Not started | - |
| 5. Self-Extension | 0/0 | Not started | - |
