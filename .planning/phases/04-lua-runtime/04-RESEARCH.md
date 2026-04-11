# Phase 4: Lua Runtime - Research

**Researched:** 2026-04-11
**Domain:** Embedded Lua VM (gopher-lua), script loading, sandboxing, tool registration
**Confidence:** HIGH

## Summary

Phase 4 adds Lua tool loading to Fenec's existing tool system. The current architecture has a clean `Tool` interface (Name/Definition/Execute) and a `Registry` that dispatches tool calls by name. Lua tools on disk must implement this same interface, loaded at startup from `~/.config/fenec/tools/`. The primary technical challenges are: (1) defining a metadata format that Lua scripts use to declare their tool schema (name, description, parameters), (2) sandboxing the Lua VM so scripts cannot access os/io/debug modules, and (3) reporting malformed scripts clearly rather than silently registering broken tools.

gopher-lua v1.1.2 is the established choice (per CLAUDE.md stack decisions). It provides `SkipOpenLibs` for sandboxing and `SetContext` for timeout enforcement. The LState is not goroutine-safe, but since Fenec executes tools sequentially in the REPL loop, a single LState (or create-per-execution) is sufficient for this phase. A pool pattern is deferred to when concurrent execution is needed.

**Primary recommendation:** Define Lua tools as scripts that return a metadata table (`name`, `description`, `parameters`, `execute`). Load each `.lua` file from the tools directory, execute it in a sandboxed LState, validate the returned table, and wrap it as a `tool.Tool` implementation registered in the existing Registry.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LUA-02 | Lua tools are loaded on startup and registered alongside built-in tools | LuaTool struct implements tool.Tool interface; loader scans tools directory on startup and calls registry.Register() for each valid script |
| LUA-04 | Lua execution is sandboxed -- no direct access to os, io, or debug modules | Use SkipOpenLibs + selective OpenBase/OpenTable/OpenString/OpenMath; context-based timeout; nil-out unsafe base functions (loadfile, dofile) |
| LUA-06 | Broken Lua tools are detected and reported, not silently loaded | Loader validates: syntax (DoFile error), return type (must be table), required fields (name, description, parameters, execute), parameter schema; reports errors with file path and reason |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/yuin/gopher-lua | v1.1.2 | Embedded Lua 5.1 VM | Already selected in CLAUDE.md. Pure Go, no cgo. Supports SkipOpenLibs for sandboxing, SetContext for timeouts, LGFunction for Go-Lua interop. Latest version confirmed. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/vadv/gopher-lua-libs/json | latest | JSON encode/decode in Lua scripts | Cherry-pick the json sub-package only. Preload with `json.Preload(L)` so Lua tools can `require("json")` to parse/encode JSON strings. Needed because tool arguments arrive as Go map values that get converted to Lua tables, and tool results are often JSON strings. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| vadv/gopher-lua-libs/json | Hand-rolled JSON functions | More control but duplicates well-tested code. Use the library. |
| Script returns metadata table | Lua comment header parsing | Comment headers are fragile (regex parsing) and cannot express parameter schemas. A returned table is idiomatic Lua and validates at runtime. |
| Single shared LState | Fresh LState per execution | Shared LState is faster (no init cost) but accumulates global state between tool runs. Fresh LState per execution is safer for sandboxing -- prevents one tool from polluting another. Start with fresh-per-execution; optimize to pooled if performance matters. |

**Installation:**
```bash
go get github.com/yuin/gopher-lua@v1.1.2
go get github.com/vadv/gopher-lua-libs/json
```

**Version verification:** gopher-lua v1.1.2 confirmed as latest via `go list -m -versions` (no newer version exists).

## Architecture Patterns

### Recommended Project Structure
```
internal/
  lua/
    sandbox.go       # Sandboxed LState creation (SkipOpenLibs + selective opens)
    loader.go        # Scan tools dir, load each .lua, validate metadata, return []LuaTool
    luatool.go       # LuaTool struct implementing tool.Tool interface
    convert.go       # Go<->Lua value conversion (ToolCallFunctionArguments <-> LTable)
    lua_test.go      # Tests for all of the above
```

### Pattern 1: Lua Tool File Format
**What:** Each Lua tool is a `.lua` file that returns a table with tool metadata and an execute function.
**When to use:** Every Lua tool file follows this format.
**Example:**
```lua
-- ~/.config/fenec/tools/word_count.lua
return {
    name = "word_count",
    description = "Count words in the given text",
    parameters = {
        { name = "text", type = "string", description = "Text to count words in", required = true }
    },
    execute = function(args)
        local text = args.text or ""
        local count = 0
        for _ in text:gmatch("%S+") do
            count = count + 1
        end
        return tostring(count)
    end
}
```

### Pattern 2: Sandboxed LState Factory
**What:** Create a minimal Lua VM with only safe libraries open.
**When to use:** Every time a Lua tool is loaded or executed.
**Example:**
```go
// Source: gopher-lua README + issue #27 sandboxing guidance
func NewSandboxedState(ctx context.Context) *lua.LState {
    L := lua.NewState(lua.Options{SkipOpenLibs: true})

    // Open only safe libraries
    for _, pair := range []struct {
        n string
        f lua.LGFunction
    }{
        {lua.LoadLibName, lua.OpenPackage}, // needed for require()
        {lua.BaseLibName, lua.OpenBase},
        {lua.TabLibName, lua.OpenTable},
        {lua.StringLibName, lua.OpenString},
        {lua.MathLibName, lua.OpenMath},
    } {
        if err := L.CallByParam(lua.P{
            Fn:      L.NewFunction(pair.f),
            NRet:    0,
            Protect: true,
        }, lua.LString(pair.n)); err != nil {
            panic(err) // should never fail for built-in libs
        }
    }

    // Remove unsafe base functions that remain even after selective open
    for _, name := range []string{"dofile", "loadfile"} {
        L.SetGlobal(name, lua.LNil)
    }

    // Set context for timeout enforcement
    L.SetContext(ctx)

    // Preload JSON module (cherry-picked from gopher-lua-libs)
    ljson.Preload(L)

    return L
}
```

### Pattern 3: LuaTool Implementing tool.Tool
**What:** A Go struct that wraps a loaded Lua script's metadata and calls its execute function.
**When to use:** For each validated Lua tool file.
**Example:**
```go
type LuaTool struct {
    name        string
    description string
    params      []LuaParam
    scriptPath  string
    proto       *lua.FunctionProto // pre-compiled bytecode for reuse
}

type LuaParam struct {
    Name        string
    Type        string // "string", "number", "boolean"
    Description string
    Required    bool
}

func (lt *LuaTool) Name() string { return lt.name }

func (lt *LuaTool) Definition() api.Tool {
    props := api.NewToolPropertiesMap()
    var required []string
    for _, p := range lt.params {
        props.Set(p.Name, api.ToolProperty{
            Type:        api.PropertyType{p.Type},
            Description: p.Description,
        })
        if p.Required {
            required = append(required, p.Name)
        }
    }
    return api.Tool{
        Type: "function",
        Function: api.ToolFunction{
            Name:        lt.name,
            Description: lt.description,
            Parameters: api.ToolFunctionParameters{
                Type:       "object",
                Required:   required,
                Properties: props,
            },
        },
    }
}

func (lt *LuaTool) Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error) {
    // Create fresh sandboxed state for each execution
    L := NewSandboxedState(ctx)
    defer L.Close()

    // Load the pre-compiled script
    fn := L.NewFunctionFromProto(lt.proto)
    L.Push(fn)
    if err := L.PCall(0, 1, nil); err != nil {
        return "", fmt.Errorf("lua tool %s: script error: %w", lt.name, err)
    }

    // Get the returned table and call execute
    tbl := L.CheckTable(-1)
    executeFn := L.GetField(tbl, "execute")
    if executeFn.Type() != lua.LTFunction {
        return "", fmt.Errorf("lua tool %s: execute is not a function", lt.name)
    }

    // Convert Go args to Lua table
    argsTable := argsToLuaTable(L, args)

    if err := L.CallByParam(lua.P{
        Fn:      executeFn,
        NRet:    1,
        Protect: true,
    }, argsTable); err != nil {
        return "", fmt.Errorf("lua tool %s: execution error: %w", lt.name, err)
    }

    result := L.Get(-1)
    L.Pop(1)
    return lua.LVAsString(result), nil
}
```

### Pattern 4: Loader with Error Reporting
**What:** Scan the tools directory, attempt to load each `.lua` file, report errors for malformed ones, return only valid tools.
**When to use:** At startup, before REPL creation.
**Example:**
```go
type LoadError struct {
    Path   string
    Reason string
}

type LoadResult struct {
    Tools  []*LuaTool
    Errors []LoadError
}

func LoadTools(toolsDir string) (*LoadResult, error) {
    result := &LoadResult{}

    entries, err := os.ReadDir(toolsDir)
    if err != nil {
        if os.IsNotExist(err) {
            return result, nil // no tools directory is fine
        }
        return nil, err
    }

    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
            continue
        }
        path := filepath.Join(toolsDir, entry.Name())
        tool, err := loadSingleTool(path)
        if err != nil {
            result.Errors = append(result.Errors, LoadError{Path: path, Reason: err.Error()})
            continue
        }
        result.Tools = append(result.Tools, tool)
    }
    return result, nil
}
```

### Pattern 5: Go-to-Lua Value Conversion
**What:** Convert ToolCallFunctionArguments to an LTable, and Lua return values back to Go strings.
**When to use:** Every tool execution boundary.
**Example:**
```go
func argsToLuaTable(L *lua.LState, args api.ToolCallFunctionArguments) *lua.LTable {
    tbl := L.NewTable()
    for key, val := range args.All() {
        switch v := val.(type) {
        case string:
            L.SetField(tbl, key, lua.LString(v))
        case float64:
            L.SetField(tbl, key, lua.LNumber(v))
        case bool:
            L.SetField(tbl, key, lua.LBool(v))
        case nil:
            L.SetField(tbl, key, lua.LNil)
        default:
            // For complex types (maps, slices), JSON-encode then set as string
            b, _ := json.Marshal(v)
            L.SetField(tbl, key, lua.LString(string(b)))
        }
    }
    return tbl
}
```

### Anti-Patterns to Avoid
- **Opening all libs then removing dangerous ones:** Use SkipOpenLibs + selective open instead. Removing after the fact risks missing a dangerous function.
- **Parsing Lua comments for metadata:** Fragile regex parsing. Use the idiomatic Lua pattern of returning a table.
- **Shared mutable LState across tool executions:** Global state leaks between tools. Use fresh LState per execution with pre-compiled bytecode for speed.
- **Silent error swallowing during tool load:** Every malformed script must produce a visible error message with the file path and specific reason.
- **Storing LState reference in LuaTool:** LState is not reusable across invocations safely. Store the compiled FunctionProto instead.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON in Lua | Custom JSON encoder/decoder exposed to Lua | `github.com/vadv/gopher-lua-libs/json` cherry-pick | JSON parsing has many edge cases (unicode, escaping, nested structures). The library handles them. |
| Lua syntax checking | Custom parser or regex validation | `L.DoFile()` / `L.LoadFile()` error return | gopher-lua's own parser gives precise syntax errors with line numbers. |
| Type conversion Go<->Lua | Manual string formatting | Structured `argsToLuaTable` function using LValue types | The Ollama API sends typed arguments (string, float64, bool). Proper conversion preserves types. |

**Key insight:** gopher-lua's built-in error handling (`PCall`, `Protect: true`, `ApiError` with line numbers) provides better error reporting than anything hand-rolled. Use protected calls everywhere.

## Common Pitfalls

### Pitfall 1: Forgetting to Close LState
**What goes wrong:** Memory leak when creating fresh LState per execution without closing.
**Why it happens:** Go's GC does not clean up gopher-lua internal state.
**How to avoid:** Always `defer L.Close()` immediately after creation.
**Warning signs:** Memory growth over time with many tool executions.

### Pitfall 2: Opening LoadLib Without Restricting require
**What goes wrong:** If you open the package library (needed for `require("json")`), Lua scripts can `require("io")` or `require("os")` if those modules are preloaded.
**Why it happens:** `OpenPackage` exposes the `require` function which checks `package.preload`.
**How to avoid:** Only preload modules you explicitly want available (json). Do NOT call `L.OpenLibs()` anywhere. The SkipOpenLibs approach means os/io/debug are never loaded, so `require("os")` will fail even with package library open.
**Warning signs:** A Lua script successfully calling `require("os")`.

### Pitfall 3: Not Nil-ing Unsafe Base Functions
**What goes wrong:** Even with selective library opening, `OpenBase` exposes `dofile` and `loadfile` which can load arbitrary files.
**Why it happens:** These are part of the base library, not os/io.
**How to avoid:** After opening base, set `dofile` and `loadfile` to nil: `L.SetGlobal("dofile", lua.LNil)`.
**Warning signs:** A Lua script loading external files.

### Pitfall 4: LState Context Performance Impact
**What goes wrong:** Using `L.SetContext()` adds overhead to every Lua instruction check.
**Why it happens:** gopher-lua checks the context on every instruction when a context is set.
**How to avoid:** Only set context during execution, not during loading/compilation. For short tool scripts the overhead is negligible, but be aware for computational scripts.
**Warning signs:** Noticeable slowdown on Lua-heavy workloads.

### Pitfall 5: Lua Number Precision Loss
**What goes wrong:** Lua 5.1 uses float64 for all numbers. Integer arguments from the model may lose precision for very large integers.
**Why it happens:** JSON numbers from Ollama API are parsed as float64, then converted to LNumber (also float64). For typical tool arguments this is not a problem, but be aware.
**How to avoid:** For tool results, always return strings from Lua. The model interprets string results fine.
**Warning signs:** Large integer arguments (>2^53) behaving unexpectedly.

### Pitfall 6: Config Directory Not Existing
**What goes wrong:** `LoadTools` fails because `~/.config/fenec/tools/` does not exist yet.
**Why it happens:** No tools have been created yet (Phase 5 creates them).
**How to avoid:** Treat missing directory as empty (zero tools, no error). The loader should `os.IsNotExist` check and return an empty result.
**Warning signs:** Startup failure when tools directory is absent.

## Code Examples

### Complete Sandboxed Execution Flow
```go
// Source: gopher-lua README, issue #27, issue #11
func executeLuaTool(proto *lua.FunctionProto, args api.ToolCallFunctionArguments, timeout time.Duration) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    L := NewSandboxedState(ctx)
    defer L.Close()

    // Execute the compiled script to get the tool table
    fn := L.NewFunctionFromProto(proto)
    L.Push(fn)
    if err := L.PCall(0, 1, nil); err != nil {
        return "", fmt.Errorf("script load error: %w", err)
    }

    toolTable := L.CheckTable(-1)
    L.Pop(1)

    // Get the execute function
    executeFn := L.GetField(toolTable, "execute")
    if executeFn.Type() != lua.LTFunction {
        return "", fmt.Errorf("execute field is not a function")
    }

    // Convert args and call
    argsTable := argsToLuaTable(L, args)
    if err := L.CallByParam(lua.P{
        Fn:      executeFn,
        NRet:    1,
        Protect: true,
    }, argsTable); err != nil {
        return "", fmt.Errorf("execution error: %w", err)
    }

    result := L.Get(-1)
    L.Pop(1)

    if result == lua.LNil {
        return "", nil
    }
    return lua.LVAsString(result), nil
}
```

### Pre-compiling for Reuse
```go
// Source: gopher-lua README - "Sharing Lua byte code between LStates"
func compileLuaFile(path string) (*lua.FunctionProto, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := bufio.NewReader(file)
    chunk, err := parse.Parse(reader, path)
    if err != nil {
        return nil, fmt.Errorf("syntax error: %w", err)
    }

    proto, err := lua.Compile(chunk, path)
    if err != nil {
        return nil, fmt.Errorf("compile error: %w", err)
    }
    return proto, nil
}
```

### Validation of Tool Metadata Table
```go
// Source: project-specific pattern derived from tool.Tool interface requirements
func validateToolTable(L *lua.LState, tbl *lua.LTable, path string) error {
    // Check required string fields
    for _, field := range []string{"name", "description"} {
        val := L.GetField(tbl, field)
        if val == lua.LNil || val.Type() != lua.LTString {
            return fmt.Errorf("%s: missing or non-string '%s' field", path, field)
        }
    }

    // Check execute is a function
    executeFn := L.GetField(tbl, "execute")
    if executeFn == lua.LNil || executeFn.Type() != lua.LTFunction {
        return fmt.Errorf("%s: missing or non-function 'execute' field", path)
    }

    // Check parameters is a table (may be empty)
    params := L.GetField(tbl, "parameters")
    if params != lua.LNil && params.Type() != lua.LTTable {
        return fmt.Errorf("%s: 'parameters' must be a table", path)
    }

    return nil
}
```

### ToolsDir Config Function
```go
// Add to internal/config/config.go
func ToolsDir() (string, error) {
    dir, err := ConfigDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, "tools"), nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OpenLibs + remove dangerous funcs | SkipOpenLibs + selective open | Always recommended for gopher-lua | Safer: whitelist vs blacklist approach |
| Lua comment-based metadata | Script returns metadata table | Common pattern in plugin systems | Runtime-validated, supports complex schemas |
| Shared LState with mutex | Fresh LState per execution or pool | Ongoing recommendation | Prevents cross-tool state pollution |
| parse.Parse syntax check only | Compile to FunctionProto (syntax + compile check) | gopher-lua bytecode sharing feature | Catches more errors and enables reuse |

**Deprecated/outdated:**
- `setfenv` is a Lua 5.1 feature that gopher-lua supports, but for sandboxing in Go, controlling which libraries are opened is more effective than environment manipulation.

## Open Questions

1. **Execution timeout value**
   - What we know: ShellTool uses 30 seconds. Lua tools should be faster (no subprocess).
   - What's unclear: What is the right default timeout for Lua execution?
   - Recommendation: Use 10 seconds default. Lua tools making HTTP calls (via future exposed Go functions) may need longer, but that is a Phase 5 concern. For Phase 4, Lua tools only do computation with string/math/table operations.

2. **Should the loader create the tools directory?**
   - What we know: `config.SessionDir()` creates its directory with `MkdirAll`. Tools directory does not exist yet.
   - What's unclear: Should Phase 4 create `~/.config/fenec/tools/` on startup, or wait until Phase 5 when the agent writes tools?
   - Recommendation: Do NOT create it in Phase 4. Treat missing directory as zero tools. Phase 5 will create it when the agent writes its first tool.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none (Go default test runner) |
| Quick run command | `go test ./internal/lua/... -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LUA-02 | Lua tools loaded on startup, registered alongside built-ins | unit | `go test ./internal/lua/... -run TestLoadTools -v -count=1` | Wave 0 |
| LUA-02 | LuaTool.Definition() produces valid api.Tool | unit | `go test ./internal/lua/... -run TestLuaToolDefinition -v -count=1` | Wave 0 |
| LUA-02 | LuaTool.Execute() calls Lua function and returns result | unit | `go test ./internal/lua/... -run TestLuaToolExecute -v -count=1` | Wave 0 |
| LUA-04 | Sandboxed LState has no os/io/debug access | unit | `go test ./internal/lua/... -run TestSandbox -v -count=1` | Wave 0 |
| LUA-04 | dofile/loadfile are nil in sandbox | unit | `go test ./internal/lua/... -run TestSandboxUnsafeFunctions -v -count=1` | Wave 0 |
| LUA-04 | Context timeout cancels long-running scripts | unit | `go test ./internal/lua/... -run TestExecutionTimeout -v -count=1` | Wave 0 |
| LUA-06 | Malformed Lua (syntax error) produces LoadError | unit | `go test ./internal/lua/... -run TestLoadBrokenSyntax -v -count=1` | Wave 0 |
| LUA-06 | Missing required fields produce LoadError | unit | `go test ./internal/lua/... -run TestLoadMissingFields -v -count=1` | Wave 0 |
| LUA-06 | Valid tools load alongside broken ones (partial success) | unit | `go test ./internal/lua/... -run TestLoadMixedTools -v -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/lua/... -v -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/lua/lua_test.go` -- covers LUA-02, LUA-04, LUA-06 (all tests)
- [ ] `internal/lua/testdata/` -- test fixture Lua scripts (valid, broken syntax, missing fields, sandbox escape attempts)
- [ ] Framework install: `go get github.com/yuin/gopher-lua@v1.1.2` -- dependency not yet in go.mod

## Sources

### Primary (HIGH confidence)
- [gopher-lua GitHub README](https://github.com/yuin/gopher-lua) -- SkipOpenLibs, LGFunction, DoFile, SetContext API
- [gopher-lua pkg.go.dev](https://pkg.go.dev/github.com/yuin/gopher-lua) -- Full API reference: LState, Options, all type definitions, library openers, FunctionProto compilation
- [gopher-lua-libs/json pkg.go.dev](https://pkg.go.dev/github.com/vadv/gopher-lua-libs/json) -- Preload(L) function for cherry-picked JSON module
- [Ollama API types.go](file:///home/marad/go/pkg/mod/github.com/ollama/ollama@v0.20.5/api/types.go) -- Tool, ToolFunction, ToolCallFunctionArguments, ToolProperty verified from source
- Existing codebase: `internal/tool/registry.go` -- Tool interface, Registry.Register(), Registry.Dispatch()

### Secondary (MEDIUM confidence)
- [gopher-lua issue #27](https://github.com/yuin/gopher-lua/issues/27) -- Sandboxing via SkipOpenLibs + selective open, nil-out unsafe functions
- [gopher-lua issue #11](https://github.com/yuin/gopher-lua/issues/11) -- Sandbox environment replacement approach
- [gopher-lua issue #5](https://github.com/yuin/gopher-lua/issues/5) -- LState goroutine safety, pool pattern

### Tertiary (LOW confidence)
- None. All findings verified against primary or secondary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- gopher-lua v1.1.2 is locked per CLAUDE.md, API verified from pkg.go.dev
- Architecture: HIGH -- Tool interface is well-defined in existing code, patterns derived from official gopher-lua documentation
- Pitfalls: HIGH -- Sandboxing guidance from multiple gopher-lua issues and Lua security documentation, verified against API

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- gopher-lua has not released since v1.1.2, API is settled)
