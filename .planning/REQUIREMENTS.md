# Requirements: Fenec

**Defined:** 2026-04-11
**Core Value:** An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Chat

- [x] **CHAT-01**: User can send messages and receive streaming responses token-by-token
- [x] **CHAT-02**: Agent maintains multi-turn conversation context across messages
- [x] **CHAT-03**: Agent manages context window -- tracks token usage and truncates when approaching model limits
- [x] **CHAT-04**: User can select which Ollama model to use (CLI flag or runtime command)
- [x] **CHAT-05**: Model responses render with markdown formatting and syntax-highlighted code blocks

### Session

- [x] **SESS-01**: User can save conversation to disk and resume later
- [x] **SESS-02**: Session auto-saves on exit to prevent data loss

### Tool System

- [x] **TOOL-01**: Agent calls tools using structured function calling format and receives results
- [x] **TOOL-02**: Available tools (built-in + Lua) are injected into the system prompt each turn
- [x] **TOOL-03**: Tool execution errors are returned to the model as structured error messages

### Execution

- [x] **EXEC-01**: Agent can execute bash/shell commands and return stdout, stderr, and exit code
- [x] **EXEC-02**: Dangerous operations (rm, sudo, writes) require user approval before execution
- [x] **EXEC-03**: Shell commands have configurable timeout to prevent hangs

### Lua Extensibility

- [x] **LUA-01**: Agent can write new Lua tools that persist to a tools directory on disk
- [x] **LUA-02**: Lua tools are loaded on startup and registered alongside built-in tools
- [x] **LUA-03**: New Lua tools become available immediately within the current session (hot-reload)
- [x] **LUA-04**: Lua execution is sandboxed -- no direct access to os, io, or debug modules
- [x] **LUA-05**: Lua tools are validated (syntax + schema) before persisting
- [x] **LUA-06**: Broken Lua tools are detected and reported, not silently loaded

### File Tools

- [ ] **FILE-01**: Agent can read files with offset/limit support and receive structured metadata (line count, truncation, binary detection)
- [ ] **FILE-02**: Agent can list directory contents with entry type, name, and size metadata
- [ ] **FILE-03**: Agent can write and edit files with automatic parent directory creation and CWD-based approval gating
- [ ] **FILE-04**: File operations on sensitive system paths (/etc, /usr, /bin, /sbin, /boot, ~/.ssh, ~/.gnupg) are blocked by a deny list

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
| CHAT-01 | Phase 1 | Complete |
| CHAT-02 | Phase 2 | Complete |
| CHAT-03 | Phase 2 | Complete |
| CHAT-04 | Phase 1 | Complete |
| CHAT-05 | Phase 1 | Complete |
| SESS-01 | Phase 2 | Complete |
| SESS-02 | Phase 2 | Complete |
| TOOL-01 | Phase 3 | Complete |
| TOOL-02 | Phase 3 | Complete |
| TOOL-03 | Phase 3 | Complete |
| EXEC-01 | Phase 3 | Complete |
| EXEC-02 | Phase 3 | Complete |
| EXEC-03 | Phase 3 | Complete |
| LUA-01 | Phase 5 | Complete |
| LUA-02 | Phase 4 | Complete |
| LUA-03 | Phase 5 | Complete |
| LUA-04 | Phase 4 | Complete |
| LUA-05 | Phase 5 | Complete |
| LUA-06 | Phase 4 | Complete |
| FILE-01 | Phase 6 | Planned |
| FILE-02 | Phase 6 | Planned |
| FILE-03 | Phase 6 | Planned |
| FILE-04 | Phase 6 | Planned |

**Coverage:**
- v1 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0

---
*Requirements defined: 2026-04-11*
*Last updated: 2026-04-11 after Phase 6 planning*
