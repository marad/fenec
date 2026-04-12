# Phase 5: Self-Extension - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

The agent can author new Lua tools that persist, validate, and become immediately usable -- the platform grows its own capabilities. Covers creation, update, deletion, and hot-reload of Lua tools. Does not include advanced sandboxing beyond what Phase 4 established.

Requirements: LUA-01, LUA-03, LUA-05

</domain>

<decisions>
## Implementation Decisions

### Tool creation mechanism
- **D-01:** Single `create_lua_tool` built-in Go tool. Model calls it with the full Lua source code as an argument.
- **D-02:** Internally: writes code to a temp staging file, compiles with `CompileFile()`, validates schema with `NewLuaToolFromProto()`. If valid, moves to `~/.config/fenec/tools/` and registers in the registry. If invalid, returns error details.
- **D-03:** On success: returns tool name, description, and parameter list as confirmation. On failure: returns error message (syntax error with line number, or schema error like "no `execute` function found"). No temp file path in the response.
- **D-04:** `create_lua_tool` rejects if a tool with the same name already exists -- returns an error. Must use `update_lua_tool` to replace.

### Tool update and delete
- **D-05:** `update_lua_tool` built-in for replacing existing Lua tools. Same validation flow as create. Rejects if tool doesn't exist.
- **D-06:** `delete_lua_tool` built-in for removing Lua tools from disk and unregistering them.
- **D-07:** All three tools (`create`, `update`, `delete`) are built-in Go tools alongside `shell_exec`.

### User oversight
- **D-08:** No user approval required for tool creation, update, or delete. The Lua sandbox provides safety guarantees.
- **D-09:** Distinct banner notification on tool creation: `✦ New tool registered: word_count -- "Count words in text"` — stands out from regular tool call output.
- **D-10:** Similar banners for update and delete events.

### Hot-reload
- **D-11:** Newly created or updated tools are registered in the running registry immediately. Available to the model on the next agentic turn (system prompt rebuilt each turn from `registry.Describe()`).
- **D-12:** Deleted tools are unregistered immediately -- model no longer sees them.

### /tools command
- **D-13:** `/tools` slash command lists all loaded tools in a flat list.
- **D-14:** Each entry tagged `[built-in]` or `[lua]` to show provenance. Shows name and description.

### Claude's Discretion
- Temp staging directory location (system temp, config sibling, etc.)
- Temp file cleanup strategy
- Exact banner styling (colors, characters)
- `/tools` output formatting details
- `update_lua_tool` and `delete_lua_tool` argument shape
- Error message wording
- Tools directory auto-creation on first write

</decisions>

<specifics>
## Specific Ideas

- Banner should use muted styling, not bold saturated colors
- The create/validate/save flow uses the filesystem as state -- temp file holds the intermediate result, no in-memory state tracking needed between tool calls

</specifics>

<canonical_refs>
## Canonical References

### Lua tool system
- `internal/lua/loader.go` -- `LoadTools()`, `CompileFile()` for compilation and directory scanning
- `internal/lua/luatool.go` -- `NewLuaToolFromProto()` for schema validation, `LuaTool` type and `Execute()` method
- `internal/lua/sandbox.go` -- `NewSandboxedState()` sandbox setup, safe library list
- `internal/lua/convert.go` -- Go-to-Lua value conversion for tool arguments

### Tool registry
- `internal/tool/registry.go` -- `Tool` interface, `Registry` with `Register()`, `Dispatch()`, `Describe()`, `Tools()`
- `internal/tool/shell.go` -- `ShellTool` as reference for built-in tool design

### REPL integration
- `internal/repl/repl.go` -- System prompt tool injection (lines 58-64), agentic loop, tool dispatch display
- `internal/repl/commands.go` -- Slash command parsing and handler registration

### Config
- `internal/config/config.go` -- `ToolsDir()` returns `~/.config/fenec/tools/` (does not create directory)

### Test fixtures
- `internal/lua/testdata/` -- Example Lua tool files (word_count.lua, validation failure cases)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `lua.CompileFile()`: Syntax validation -- already returns parse errors with line numbers
- `lua.NewLuaToolFromProto()`: Schema validation -- checks name, description, execute fields
- `tool.Registry.Register()`: Runtime registration of new tools
- `tool.Registry.Describe()`: System prompt generation -- rebuilds each turn so new tools appear automatically
- `repl.ParseCommand()`: Slash command infrastructure for adding `/tools`

### Established Patterns
- Built-in tools implement `tool.Tool` interface (Name, Definition, Execute)
- Tool calls displayed as `[tool: name]` / `[result: N bytes]` in REPL output
- ShellTool uses an approval callback for dangerous commands -- same pattern could wire notifications
- Registry is injected into REPL at construction, available as `r.registry`

### Integration Points
- `main.go` (lines 99-121): Tool registration at startup -- new built-in tools wire in here
- REPL `sendMessage()`: Tool dispatch loop -- new tools participate automatically via registry
- System prompt: Tool descriptions appended each turn via `registry.Describe()` -- no special handling needed for hot-reload

</code_context>

<deferred>
## Deferred Ideas

- Advanced sandboxing (resource limits, network restrictions, filesystem scoping) -- future milestone
- Tool versioning / rollback -- add to backlog if needed
- Tool sharing / export between fenec instances -- out of scope

</deferred>

---

*Phase: 05-self-extension*
*Context gathered: 2026-04-11*
