# Phase 05: Self-Extension - Research

**Researched:** 2026-04-11
**Domain:** Lua tool lifecycle management (create, update, delete, hot-reload)
**Confidence:** HIGH

## Summary

Phase 5 is a pure application-level feature built on top of Phase 4's Lua runtime. No new external libraries are needed. The core work is implementing three new built-in Go tools (`create_lua_tool`, `update_lua_tool`, `delete_lua_tool`), adding an `Unregister` method to the registry, adding a `/tools` slash command, and ensuring hot-reload works correctly.

The existing codebase provides nearly all the building blocks: `CompileFile` for syntax validation, `NewLuaToolFromProto` for schema validation, `Registry.Register` for runtime registration, and `registry.Tools()` which is called fresh on every `sendMessage` call. The main gaps are: (1) Registry has no `Unregister` method, (2) the system prompt tool description text is built once at REPL construction and won't reflect hot-reload changes, and (3) `config.ToolsDir()` does not create the directory.

**Primary recommendation:** Build three built-in tool structs following the ShellTool pattern, add `Unregister` to Registry, fix system prompt staleness by updating the conversation's system message when tools change, and wire everything through main.go like `ShellTool`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Single `create_lua_tool` built-in Go tool. Model calls it with the full Lua source code as an argument.
- **D-02:** Internally: writes code to a temp staging file, compiles with `CompileFile()`, validates schema with `NewLuaToolFromProto()`. If valid, moves to `~/.config/fenec/tools/` and registers in the registry. If invalid, returns error details.
- **D-03:** On success: returns tool name, description, and parameter list as confirmation. On failure: returns error message (syntax error with line number, or schema error like "no `execute` function found"). No temp file path in the response.
- **D-04:** `create_lua_tool` rejects if a tool with the same name already exists -- returns an error. Must use `update_lua_tool` to replace.
- **D-05:** `update_lua_tool` built-in for replacing existing Lua tools. Same validation flow as create. Rejects if tool doesn't exist.
- **D-06:** `delete_lua_tool` built-in for removing Lua tools from disk and unregistering them.
- **D-07:** All three tools (`create`, `update`, `delete`) are built-in Go tools alongside `shell_exec`.
- **D-08:** No user approval required for tool creation, update, or delete. The Lua sandbox provides safety guarantees.
- **D-09:** Distinct banner notification on tool creation: `New tool registered: word_count -- "Count words in text"` -- stands out from regular tool call output.
- **D-10:** Similar banners for update and delete events.
- **D-11:** Newly created or updated tools are registered in the running registry immediately. Available to the model on the next agentic turn (system prompt rebuilt each turn from `registry.Describe()`).
- **D-12:** Deleted tools are unregistered immediately -- model no longer sees them.
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

### Deferred Ideas (OUT OF SCOPE)
- Advanced sandboxing (resource limits, network restrictions, filesystem scoping) -- future milestone
- Tool versioning / rollback -- add to backlog if needed
- Tool sharing / export between fenec instances -- out of scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LUA-01 | Agent can write new Lua tools that persist to a tools directory on disk | `create_lua_tool` built-in writes validated Lua to `~/.config/fenec/tools/`. `config.ToolsDir()` provides path. Must auto-create directory on first write. |
| LUA-03 | New Lua tools become available immediately within the current session (hot-reload) | `registry.Register()` adds to live registry. `registry.Tools()` is called fresh each `sendMessage()` turn. System prompt needs updating -- see Architecture Patterns. |
| LUA-05 | Lua tools are validated (syntax + schema) before persisting | `CompileFile()` validates syntax with line-number errors. `NewLuaToolFromProto()` validates schema (name, description, execute function). Both already exist from Phase 4. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/yuin/gopher-lua | v1.1.2 (in go.mod) | Lua compilation and validation | Already used. `CompileFile()` and `NewLuaToolFromProto()` are the validation backbone. |
| github.com/ollama/ollama/api | v0.20.5 (in go.mod) | Tool definitions and arguments | Already used. New tools implement `tool.Tool` with `api.Tool` definitions. |
| charm.land/lipgloss/v2 | v2.0.2 (in go.mod) | Banner styling | Already used for prompt and error styling. New banner styles for tool events. |
| os (stdlib) | Go 1.25 | File I/O for tool persistence | `os.WriteFile`, `os.Remove`, `os.MkdirAll`, `os.CreateTemp`. |

### Supporting
No new dependencies required. Everything builds on existing packages.

### Alternatives Considered
None applicable -- all decisions are locked to existing stack.

**Installation:**
No new packages needed.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── tool/
│   ├── registry.go      # Add Unregister(), Has(), ToolInfo type
│   ├── shell.go          # Existing (reference pattern)
│   ├── create.go         # NEW: CreateLuaTool built-in
│   ├── update.go         # NEW: UpdateLuaTool built-in
│   ├── delete.go         # NEW: DeleteLuaTool built-in
│   ├── safety.go         # Existing
│   └── approval.go       # Existing (not found -- just safety.go)
├── lua/
│   ├── luatool.go        # Existing (add CompileSource helper)
│   ├── loader.go         # Existing
│   ├── sandbox.go        # Existing
│   └── convert.go        # Existing
├── repl/
│   ├── repl.go           # Add /tools handler, system prompt refresh
│   └── commands.go       # Add /tools to command list and help text
├── render/
│   └── style.go          # Add tool event banner styles
└── config/
    └── config.go         # Existing (ToolsDir already works)
```

### Pattern 1: Built-in Tool Implementation (following ShellTool)
**What:** Each new built-in tool is a struct implementing `tool.Tool` interface
**When to use:** For all three new tools
**Example:**
```go
// Source: internal/tool/shell.go pattern
type CreateLuaTool struct {
    toolsDir string
    registry *Registry
    notifier func(event, name, desc string)
}

func (c *CreateLuaTool) Name() string { return "create_lua_tool" }

func (c *CreateLuaTool) Definition() api.Tool {
    props := api.NewToolPropertiesMap()
    props.Set("code", api.ToolProperty{
        Type:        api.PropertyType{"string"},
        Description: "Complete Lua tool source code",
    })
    return api.Tool{
        Type: "function",
        Function: api.ToolFunction{
            Name:        "create_lua_tool",
            Description: "Create a new Lua tool...",
            Parameters: api.ToolFunctionParameters{
                Type:     "object",
                Required: []string{"code"},
                Properties: props,
            },
        },
    }
}

func (c *CreateLuaTool) Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error) {
    // 1. Extract code from args
    // 2. Write to temp file
    // 3. CompileFile() for syntax check
    // 4. NewLuaToolFromProto() for schema check
    // 5. Check registry for name collision (D-04)
    // 6. MkdirAll on toolsDir
    // 7. Write final file to toolsDir/<name>.lua
    // 8. Register in registry
    // 9. Call notifier
    // 10. Return success JSON
}
```

### Pattern 2: Registry Unregister for Delete Support
**What:** Add `Unregister(name string) bool` to Registry
**When to use:** For `delete_lua_tool` and `update_lua_tool` (unregister old, register new)
**Example:**
```go
// Source: pattern derived from existing Registry.Register
func (r *Registry) Unregister(name string) bool {
    _, ok := r.tools[name]
    if ok {
        delete(r.tools, name)
    }
    return ok
}

// Has checks if a tool is registered (for create vs update distinction)
func (r *Registry) Has(name string) bool {
    _, ok := r.tools[name]
    return ok
}
```

### Pattern 3: Notifier Callback for Banner Output
**What:** Tool event notifications via callback, same pattern as ShellTool's ApproverFunc
**When to use:** To display banners when tools are created/updated/deleted
**Example:**
```go
// Notifier callback type -- injected from REPL like ApproverFunc
type ToolEventNotifier func(event string, toolName string, description string)

// In main.go, wire it after REPL creation:
var notifier tool.ToolEventNotifier
// ... create tools with notifier ...
notifier = func(event, name, desc string) {
    fmt.Fprintln(os.Stdout, render.FormatToolEvent(event, name, desc))
}
```

### Pattern 4: System Prompt Staleness Fix
**What:** The system prompt text describing tools is set once at REPL construction. New tools won't appear in the text description.
**When to use:** After any tool create/update/delete

**Critical finding:** The system prompt is embedded in `conv.Messages[0]` as the first message. It includes tool descriptions via `registry.Describe()`. This text is stale after hot-reload. However, the actual tool schemas in `ChatRequest.Tools` are always fresh because `registry.Tools()` is called on every `sendMessage()`.

**Options (Claude's discretion):**
1. **Do nothing** -- The API `Tools` field is what the model uses for structured tool calling. The system prompt text is supplementary. Most models prioritize the schema over free-text descriptions.
2. **Update system message** -- Rebuild `conv.Messages[0].Content` with fresh `registry.Describe()` after each tool event. This keeps text and schema in sync.

**Recommendation:** Option 2 is safer. If the model sees a tool in the schema but not in the system prompt text, it may be confused. This is a simple rebuild:
```go
// After tool event, update system message:
func (r *REPL) refreshSystemPrompt() {
    if len(r.conv.Messages) > 0 && r.conv.Messages[0].Role == "system" {
        // Rebuild with fresh tool descriptions
        toolDesc := r.registry.Describe()
        r.conv.Messages[0].Content = r.baseSystemPrompt + "\n\n## Available Tools\n\n" + toolDesc
    }
}
```
This requires storing the base system prompt (before tool descriptions) as a REPL field.

### Pattern 5: Temp File Staging Flow (D-02)
**What:** Write code to temp, validate, then move to final location
**When to use:** In create_lua_tool and update_lua_tool Execute methods
**Example:**
```go
// Use os.CreateTemp in system temp dir (Claude's discretion)
tmpFile, err := os.CreateTemp("", "fenec-tool-*.lua")
if err != nil { return "", fmt.Errorf("failed to create temp file: %w", err) }
defer os.Remove(tmpFile.Name()) // Cleanup on any path

if _, err := tmpFile.Write([]byte(code)); err != nil {
    tmpFile.Close()
    return "", fmt.Errorf("failed to write temp file: %w", err)
}
tmpFile.Close()

// Validate
proto, err := lua.CompileFile(tmpFile.Name())
if err != nil { return "", err } // Returns syntax error with line number

lt, err := lua.NewLuaToolFromProto(proto, tmpFile.Name())
if err != nil { return "", err } // Returns schema error

// Move to final location
finalPath := filepath.Join(toolsDir, lt.Name() + ".lua")
os.MkdirAll(toolsDir, 0755)
os.Rename(tmpFile.Name(), finalPath) // or copy if cross-device
```

### Pattern 6: Tool Provenance Tagging (D-14)
**What:** `/tools` needs to distinguish built-in from Lua tools
**When to use:** For `/tools` command output
**Options:**
1. Add an `IsBuiltIn() bool` method to `tool.Tool` interface -- breaking change, affects LuaTool
2. Track provenance in Registry (e.g., `Register(t Tool)` vs `RegisterLua(t Tool)`)
3. Type-assert: `_, isLua := t.(*lua.LuaTool)` -- simple, works with current code
4. Add a separate set in Registry tracking which names are Lua tools

**Recommendation:** Option 3 (type assertion) is simplest and requires no interface changes. The Registry already stores `Tool` values -- type-asserting to `*lua.LuaTool` is idiomatic Go. However, this creates an import cycle (tool -> lua -> tool). Better: use a `Provenance` method on a new interface, or track in Registry.

**Revised recommendation:** Add a `builtIn` set to Registry:
```go
type Registry struct {
    tools   map[string]Tool
    builtIn map[string]bool // true for built-in tools
}

func (r *Registry) Register(t Tool) {
    r.tools[t.Name()] = t
    r.builtIn[t.Name()] = true
}

func (r *Registry) RegisterLua(t Tool) {
    r.tools[t.Name()] = t
    // not in builtIn set
}

func (r *Registry) IsBuiltIn(name string) bool {
    return r.builtIn[name]
}
```

### Anti-Patterns to Avoid
- **Recompiling from disk on execute:** LuaTool stores pre-compiled `FunctionProto`. After registration, never re-read the file. The in-memory proto is canonical.
- **Shared LState across tool operations:** Fresh sandboxed LState per validation run. Phase 4 decision.
- **Modifying registry during iteration:** Don't iterate and unregister simultaneously. The agentic loop dispatches one tool at a time, so this isn't actually a risk.
- **Cross-device rename:** `os.Rename` fails across filesystem boundaries (e.g., /tmp on different mount than ~/.config). Use write-then-remove instead of rename for maximum portability.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Lua syntax validation | Custom parser | `lua.CompileFile()` → `parse.Parse` + `glua.Compile` | Already handles line numbers in errors, battle-tested |
| Lua schema validation | Custom field checks | `lua.NewLuaToolFromProto()` | Already validates name, description, execute, parameters |
| Tool registration | Custom map management | `tool.Registry.Register()` / `.Unregister()` | Central dispatch, Describe, Tools -- all wired |
| Temp file management | Manual fd tracking | `os.CreateTemp` + `defer os.Remove` | Stdlib handles race-free unique names |
| Directory creation | Check-then-create | `os.MkdirAll` | Atomic, handles existing dirs, creates parents |

**Key insight:** Phase 4 built all the hard infrastructure. Phase 5 is plumbing -- connecting user intent (model calls) to existing validation and registration APIs.

## Common Pitfalls

### Pitfall 1: Cross-Device Rename Failure
**What goes wrong:** `os.Rename` from `/tmp/fenec-tool-xxx.lua` to `~/.config/fenec/tools/word_count.lua` fails with `EXDEV` (invalid cross-device link) if /tmp is on a different filesystem.
**Why it happens:** Common on Linux systems where /tmp is tmpfs.
**How to avoid:** Use `os.WriteFile` to write directly to the final path (after validation from temp), or copy bytes then remove temp. Alternatively, create temp in the same directory as final destination.
**Warning signs:** "invalid cross-device link" error in production but not in tests (test TempDir is on same fs).

### Pitfall 2: System Prompt Staleness
**What goes wrong:** Model sees tool in `ChatRequest.Tools` schema but not in the system prompt text. May cause inconsistent behavior.
**Why it happens:** System prompt is built once at REPL construction. Hot-reload updates registry but not the text.
**How to avoid:** Store base system prompt separately. After any tool event, rebuild `conv.Messages[0].Content` with fresh `registry.Describe()`.
**Warning signs:** Model says "I don't have a tool called X" even though it was just created.

### Pitfall 3: Name Collision Between Built-in and Lua Tools
**What goes wrong:** Agent creates a Lua tool named `shell_exec`, overriding the built-in.
**Why it happens:** Registry uses a flat map -- same name overwrites.
**How to avoid:** In `create_lua_tool.Execute()`, check if the name matches any existing tool (not just Lua tools). Reject names that collide with built-in tools.
**Warning signs:** Built-in tool stops working after agent creates a tool.

### Pitfall 4: File Naming Doesn't Match Tool Name
**What goes wrong:** Agent creates `word_count` tool but file is named `my_tool.lua`. On restart, loader creates `LuaTool` from file but the internal name is `word_count`. If another tool is saved as `word_count.lua`, they collide on disk.
**Why it happens:** File name and tool name are independent in the current loader.
**How to avoid:** Always name the file `<tool_name>.lua`. The `create_lua_tool` sets the filename from the validated tool's `Name()`, not from user input.
**Warning signs:** Duplicate tool files with different names but same internal tool name.

### Pitfall 5: Partial Write on Disk Full
**What goes wrong:** Tool written to temp validates fine, but final write to tools dir fails mid-write, leaving a corrupt file.
**Why it happens:** Disk full, permission error, or crash during write.
**How to avoid:** Write to temp in the same directory, then rename (atomic on same fs). Or write full content with `os.WriteFile` (which is write-then-close, not streaming).
**Warning signs:** Truncated Lua files in tools directory after crash.

### Pitfall 6: Update Without Refreshing Proto
**What goes wrong:** `update_lua_tool` updates the file on disk and re-registers, but the old `LuaTool` with stale `FunctionProto` is still in the registry.
**Why it happens:** Registry.Register just overwrites the map entry, but if the same pointer is reused, the old proto stays.
**How to avoid:** Create a completely new `LuaTool` from the new source, then register it. Never mutate the existing tool.
**Warning signs:** Updated tool still executes old code within the session.

## Code Examples

### Creating a Tool (Full Flow)
```go
// Source: Derived from existing CompileFile + NewLuaToolFromProto patterns
func (c *CreateLuaTool) Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error) {
    codeVal, ok := args.Get("code")
    if !ok {
        return "", fmt.Errorf("missing required argument: code")
    }
    code, ok := codeVal.(string)
    if !ok {
        return "", fmt.Errorf("argument 'code' must be a string")
    }

    // Write to temp file for CompileFile
    tmpFile, err := os.CreateTemp("", "fenec-tool-*.lua")
    if err != nil {
        return "", fmt.Errorf("failed to create staging file: %w", err)
    }
    tmpPath := tmpFile.Name()
    defer os.Remove(tmpPath)

    if _, err := tmpFile.WriteString(code); err != nil {
        tmpFile.Close()
        return "", fmt.Errorf("failed to write staging file: %w", err)
    }
    tmpFile.Close()

    // Syntax validation
    proto, err := lua.CompileFile(tmpPath)
    if err != nil {
        return formatError(err), nil // Return error as result, not Go error
    }

    // Schema validation
    lt, err := lua.NewLuaToolFromProto(proto, tmpPath)
    if err != nil {
        return formatError(err), nil
    }

    // Name collision check (D-04)
    if c.registry.Has(lt.Name()) {
        return fmt.Sprintf("Tool '%s' already exists. Use update_lua_tool to replace it.", lt.Name()), nil
    }

    // Ensure tools directory exists
    if err := os.MkdirAll(c.toolsDir, 0755); err != nil {
        return "", fmt.Errorf("failed to create tools directory: %w", err)
    }

    // Write final file (named by tool name, not temp name)
    finalPath := filepath.Join(c.toolsDir, lt.Name()+".lua")
    if err := os.WriteFile(finalPath, []byte(code), 0644); err != nil {
        return "", fmt.Errorf("failed to write tool file: %w", err)
    }

    // Re-compile from final path so scriptPath is correct
    proto, _ = lua.CompileFile(finalPath)
    lt, _ = lua.NewLuaToolFromProto(proto, finalPath)

    // Register and notify
    c.registry.RegisterLua(lt)
    if c.notifier != nil {
        c.notifier("created", lt.Name(), lt.Definition().Function.Description)
    }

    return formatSuccess(lt), nil
}
```

### Validation Error as Tool Result (Not Go Error)
```go
// Source: Pattern from ShellTool -- return structured info, not Go errors
// Validation errors should be returned as the tool result string so the
// model can see them and try again. Only infrastructure failures should
// be Go errors.
func formatError(err error) string {
    return fmt.Sprintf(`{"error": %q}`, err.Error())
}

func formatSuccess(lt *lua.LuaTool) string {
    // Return confirmation with tool metadata
    def := lt.Definition()
    var params []string
    for key := range def.Function.Parameters.Properties.All() {
        params = append(params, key)
    }
    return fmt.Sprintf(`{"status":"created","name":%q,"description":%q,"parameters":%v}`,
        def.Function.Name, def.Function.Description, params)
}
```

### Registry Additions
```go
// Source: Pattern extension of existing Registry
func (r *Registry) Unregister(name string) bool {
    _, ok := r.tools[name]
    if ok {
        delete(r.tools, name)
        delete(r.builtIn, name)
    }
    return ok
}

func (r *Registry) Has(name string) bool {
    _, ok := r.tools[name]
    return ok
}
```

### Banner Styling
```go
// Source: Follows existing style.go patterns with muted colors per user preference
var toolEventStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#7AA2F7")) // Muted blue, not bold

func FormatToolEvent(event, name, desc string) string {
    // D-09 format: "New tool registered: word_count -- "Count words in text""
    switch event {
    case "created":
        return toolEventStyle.Render("New tool registered: "+name) + " -- " + strconv.Quote(desc)
    case "updated":
        return toolEventStyle.Render("Tool updated: "+name) + " -- " + strconv.Quote(desc)
    case "deleted":
        return toolEventStyle.Render("Tool removed: "+name)
    }
    return ""
}
```

### /tools Slash Command
```go
// Source: Follows existing handleModelCommand pattern
func (r *REPL) handleToolsCommand() {
    if r.registry == nil {
        fmt.Fprintln(r.rl.Stdout(), "No tool registry available.")
        return
    }
    // ToolInfo returns name, description, isBuiltIn for each tool
    info := r.registry.ToolInfo()
    if len(info) == 0 {
        fmt.Fprintln(r.rl.Stdout(), "No tools loaded.")
        return
    }
    for _, t := range info {
        tag := "[lua]"
        if t.BuiltIn {
            tag = "[built-in]"
        }
        fmt.Fprintf(r.rl.Stdout(), "  %s %s -- %s\n", tag, t.Name, t.Description)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| System prompt text-only tool descriptions | Ollama `ChatRequest.Tools` structured schema | Ollama v0.5+ (2024) | Models use schema for structured calling, text is supplementary |
| Reload from disk on each execution | Pre-compiled FunctionProto in memory | Phase 4 (current) | Performance, consistency -- already the pattern |

**No deprecated/outdated patterns apply** -- Phase 5 builds on Phase 4's current architecture.

## Open Questions

1. **System Prompt Refresh Strategy**
   - What we know: System prompt text is stale after hot-reload. API `Tools` field is fresh.
   - What's unclear: Whether models behave differently when text and schema disagree.
   - Recommendation: Refresh the system prompt text on tool events (low cost, high safety). Store base prompt as REPL field.

2. **Temp File Location**
   - What we know: `os.CreateTemp("")` uses system temp. Cross-device rename may fail.
   - What's unclear: Whether to use system temp or tools dir as temp location.
   - Recommendation: Use `os.CreateTemp("", ...)` for staging, then `os.WriteFile` to final destination (avoids rename entirely). Clean up temp with `defer os.Remove`.

3. **CompileFile vs CompileSource**
   - What we know: Current `CompileFile` opens a file path. The staging flow writes to temp then calls `CompileFile`.
   - What's unclear: Whether adding a `CompileSource(code string, name string)` helper is worth it.
   - Recommendation: Stick with `CompileFile` on the temp file. It's already tested and the temp file is needed anyway (D-02 says "writes code to a temp staging file"). Adding CompileSource is unnecessary indirection.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go convention: `go test ./...`) |
| Quick run command | `go test ./internal/tool/... ./internal/lua/... ./internal/repl/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LUA-01 | create_lua_tool writes valid Lua to tools dir | unit | `go test ./internal/tool/... -run TestCreateLuaTool -count=1` | No -- Wave 0 |
| LUA-01 | create_lua_tool rejects duplicate names (D-04) | unit | `go test ./internal/tool/... -run TestCreateLuaToolDuplicate -count=1` | No -- Wave 0 |
| LUA-01 | update_lua_tool replaces existing tool on disk | unit | `go test ./internal/tool/... -run TestUpdateLuaTool -count=1` | No -- Wave 0 |
| LUA-01 | delete_lua_tool removes file from disk | unit | `go test ./internal/tool/... -run TestDeleteLuaTool -count=1` | No -- Wave 0 |
| LUA-03 | Registry.Register makes tool available via Tools() | unit | `go test ./internal/tool/... -run TestRegistryRegister -count=1` | Yes (existing) |
| LUA-03 | Registry.Unregister removes tool from Tools() | unit | `go test ./internal/tool/... -run TestRegistryUnregister -count=1` | No -- Wave 0 |
| LUA-05 | Syntax errors return line numbers | unit | `go test ./internal/tool/... -run TestCreateSyntaxError -count=1` | No -- Wave 0 |
| LUA-05 | Schema errors (missing execute) are reported | unit | `go test ./internal/tool/... -run TestCreateSchemaError -count=1` | No -- Wave 0 |
| LUA-05 | Valid Lua compiles and registers | unit | `go test ./internal/tool/... -run TestCreateLuaToolSuccess -count=1` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/tool/... ./internal/lua/... ./internal/repl/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/tool/create_test.go` -- covers LUA-01, LUA-05 (create path)
- [ ] `internal/tool/update_test.go` -- covers LUA-01 (update path)
- [ ] `internal/tool/delete_test.go` -- covers LUA-01 (delete path)
- [ ] `internal/tool/registry_test.go` -- add Unregister, Has, ToolInfo tests (extends existing file)

## Sources

### Primary (HIGH confidence)
- `internal/tool/registry.go` -- Registry API, no Unregister method exists
- `internal/tool/shell.go` -- Reference implementation for built-in tools
- `internal/lua/luatool.go` -- CompileFile, NewLuaToolFromProto API
- `internal/lua/loader.go` -- LoadTools pattern
- `internal/repl/repl.go` -- System prompt built once at line 58-64, registry.Tools() called per sendMessage at line 240-242
- `internal/config/config.go` -- ToolsDir() returns path without creating directory
- `internal/render/style.go` -- Existing lipgloss styles (muted color palette)
- `go.mod` -- All dependencies already present, no new packages needed

### Secondary (MEDIUM confidence)
- `os.Rename` cross-device limitation is well-documented Go stdlib behavior
- gopher-lua `parse.Parse` accepts `io.Reader` -- verified in luatool.go line 42

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No new dependencies, everything verified in go.mod and existing code
- Architecture: HIGH - All building blocks exist, patterns clear from ShellTool reference
- Pitfalls: HIGH - Cross-device rename and system prompt staleness verified by code inspection

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- no external dependency changes expected)
