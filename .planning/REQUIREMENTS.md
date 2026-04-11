# Requirements: Fenec

**Defined:** 2026-04-11
**Core Value:** An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Chat

- [ ] **CHAT-01**: User can send messages and receive streaming responses token-by-token
- [ ] **CHAT-02**: Agent maintains multi-turn conversation context across messages
- [ ] **CHAT-03**: Agent manages context window -- tracks token usage and truncates when approaching model limits
- [ ] **CHAT-04**: User can select which Ollama model to use (CLI flag or runtime command)
- [x] **CHAT-05**: Model responses render with markdown formatting and syntax-highlighted code blocks

### Session

- [ ] **SESS-01**: User can save conversation to disk and resume later
- [ ] **SESS-02**: Session auto-saves on exit to prevent data loss

### Tool System

- [ ] **TOOL-01**: Agent calls tools using structured function calling format and receives results
- [ ] **TOOL-02**: Available tools (built-in + Lua) are injected into the system prompt each turn
- [ ] **TOOL-03**: Tool execution errors are returned to the model as structured error messages

### Execution

- [ ] **EXEC-01**: Agent can execute bash/shell commands and return stdout, stderr, and exit code
- [ ] **EXEC-02**: Dangerous operations (rm, sudo, writes) require user approval before execution
- [ ] **EXEC-03**: Shell commands have configurable timeout to prevent hangs

### Lua Extensibility

- [ ] **LUA-01**: Agent can write new Lua tools that persist to a tools directory on disk
- [ ] **LUA-02**: Lua tools are loaded on startup and registered alongside built-in tools
- [ ] **LUA-03**: New Lua tools become available immediately within the current session (hot-reload)
- [ ] **LUA-04**: Lua execution is sandboxed -- no direct access to os, io, or debug modules
- [ ] **LUA-05**: Lua tools are validated (syntax + schema) before persisting
- [ ] **LUA-06**: Broken Lua tools are detected and reported, not silently loaded

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Integrations

- **INTG-01**: Agent can read and tag emails
- **INTG-02**: Agent can search through and organize notes

### Advanced Chat

- **ACHAT-01**: Conversation summarization when approaching context limits (beyond simple truncation)
- **ACHAT-02**: Full TUI with panels, history sidebar, status indicators

### Protocol Support

- **PROT-01**: MCP (Model Context Protocol) support for external tool integration

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Cloud/remote model providers | Local-first with Ollama, no cloud dependencies |
| Multi-user support | Personal assistant, single user |
| Web or GUI interface | CLI only for v1 |
| Multi-agent orchestration | Single agent is sufficient for v1 |
| RAG/knowledge base | Adds significant complexity, defer to future milestone |
| OAuth/authentication | Personal tool, no auth needed |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CHAT-01 | Phase 1 | Pending |
| CHAT-02 | Phase 2 | Pending |
| CHAT-03 | Phase 2 | Pending |
| CHAT-04 | Phase 1 | Pending |
| CHAT-05 | Phase 1 | Complete |
| SESS-01 | Phase 2 | Pending |
| SESS-02 | Phase 2 | Pending |
| TOOL-01 | Phase 3 | Pending |
| TOOL-02 | Phase 3 | Pending |
| TOOL-03 | Phase 3 | Pending |
| EXEC-01 | Phase 3 | Pending |
| EXEC-02 | Phase 3 | Pending |
| EXEC-03 | Phase 3 | Pending |
| LUA-01 | Phase 5 | Pending |
| LUA-02 | Phase 4 | Pending |
| LUA-03 | Phase 5 | Pending |
| LUA-04 | Phase 4 | Pending |
| LUA-05 | Phase 5 | Pending |
| LUA-06 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0

---
*Requirements defined: 2026-04-11*
*Last updated: 2026-04-11 after roadmap creation*
