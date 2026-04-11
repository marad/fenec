# Architecture Research

**Domain:** Self-extending AI agent platform (Go + LuaJIT + Ollama)
**Researched:** 2026-04-11
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
                           Fenec Agent Platform
 ============================================================================

 +-----------+      +--------------------------------------------------+
 |           |      |                   Agent Core                      |
 |   CLI     |      |                                                  |
 |   REPL    +----->|  +------------+    +-----------+    +----------+ |
 |           |      |  | Conver-    |    | Agent     |    | System   | |
 | (stdin/   |<-----+  | sation    +--->| Loop      +--->| Prompt   | |
 |  stdout)  |      |  | Manager   |    | (ReAct)   |    | Builder  | |
 |           |      |  +-----+------+    +-----+-----+    +----------+ |
 +-----------+      |        |                 |                       |
                    |        v                 v                       |
                    |  +-----+------+    +-----+-----+                 |
                    |  | Message    |    | Tool      |                 |
                    |  | History    |    | Dispatcher|                 |
                    |  +------------+    +-----+-----+                 |
                    +-------------------------|------------------------+
                                              |
                         +--------------------+--------------------+
                         |                    |                    |
                    +----v-----+        +-----v----+        +-----v----+
                    | Ollama   |        | Built-in |        | Lua Tool |
                    | Client   |        | Tools    |        | Runtime  |
                    +----+-----+        +----------+        +-----+----+
                         |              | - bash   |              |
                    +----v-----+        | - write  |        +-----v----+
                    | Ollama   |        |   _tool  |        | Tool     |
                    | Server   |        +----------+        | Store    |
                    | (local)  |                            | (disk)   |
                    +----------+                            +----------+
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| CLI REPL | Read user input, display streamed responses, signal handling | `bufio.Scanner` on stdin, token-by-token stdout writes |
| Conversation Manager | Maintain message history, enforce context limits, build request payloads | Append-only message slice with rolling window truncation |
| Agent Loop | Drive the ReAct cycle: call LLM, check for tool calls, dispatch, feed results back, repeat until done | `for` loop with max-iteration guard |
| System Prompt Builder | Assemble system prompt with tool descriptions and instructions | Template that injects tool registry descriptions |
| Tool Dispatcher | Route tool calls from model output to correct handler, return results | Map of `name -> handler` with JSON argument parsing |
| Ollama Client | Send chat requests, stream responses, handle tool call responses | Official `github.com/ollama/ollama/api` package |
| Built-in Tools | Core tools implemented in Go (bash, write_tool) | Go functions conforming to a `Tool` interface |
| Lua Tool Runtime | Execute Lua scripts in a sandboxed VM, expose host APIs | `gopher-lua` with selective module loading |
| Tool Store | Persist Lua scripts to disk, load on startup, manage tool metadata | Directory of `.lua` files with JSON schema sidecars |
| Message History | Store conversation turns for context | In-memory slice of `api.Message`, no persistence needed initially |

## Recommended Project Structure

```
fenec/
+-- cmd/
|   +-- fenec/
|       +-- main.go              # Entry point, wire dependencies, start REPL
+-- internal/
|   +-- agent/
|   |   +-- agent.go             # Agent struct, ReAct loop, max iterations
|   |   +-- agent_test.go
|   |   +-- conversation.go      # Message history management, context window
|   |   +-- prompt.go            # System prompt assembly with tool descriptions
|   +-- cli/
|   |   +-- repl.go              # Input reading, response display, streaming
|   |   +-- repl_test.go
|   +-- llm/
|   |   +-- client.go            # Ollama client wrapper, chat method
|   |   +-- client_test.go
|   |   +-- types.go             # Request/response types if wrapping needed
|   +-- tool/
|   |   +-- registry.go          # Tool registration, lookup, listing
|   |   +-- registry_test.go
|   |   +-- tool.go              # Tool interface definition
|   |   +-- dispatch.go          # Parse tool calls, route to handlers
|   |   +-- dispatch_test.go
|   +-- tools/
|   |   +-- bash.go              # Built-in: execute shell commands
|   |   +-- bash_test.go
|   |   +-- writetool.go         # Built-in: write Lua tool to disk
|   |   +-- writetool_test.go
|   +-- lua/
|   |   +-- runtime.go           # gopher-lua VM setup, sandboxing
|   |   +-- runtime_test.go
|   |   +-- loader.go            # Load .lua files from tool store
|   |   +-- loader_test.go
|   |   +-- hostapi.go           # Go functions exposed to Lua scripts
+-- tools/                       # Lua tool store (persisted scripts)
|   +-- example.lua              # Example: agent-authored tool
|   +-- example.json             # Tool schema (name, description, params)
+-- go.mod
+-- go.sum
```

### Structure Rationale

- **cmd/fenec/**: Single binary entry point. Wires all dependencies via constructor injection and starts the REPL. Minimal code -- just composition.
- **internal/agent/**: Owns the ReAct loop and conversation state. This is the brain -- it calls the LLM, interprets responses, and decides whether to dispatch tools or return to the user.
- **internal/cli/**: Owns terminal I/O only. Decoupled from agent logic so the agent can be tested without a terminal.
- **internal/llm/**: Thin wrapper around the Ollama client. Isolates the external dependency behind an interface so tests can mock it and a future provider swap is trivial.
- **internal/tool/**: The tool system core -- interface definition, registry, and dispatch. No concrete tools live here, just the framework.
- **internal/tools/**: Concrete built-in tool implementations. Each tool is a separate file implementing the `Tool` interface.
- **internal/lua/**: LuaJIT runtime management. Handles VM lifecycle, sandboxing, loading scripts, and exposing host APIs to Lua.
- **tools/**: On-disk directory where agent-authored Lua tools persist. Each tool is a `.lua` file paired with a `.json` schema file.

## Architectural Patterns

### Pattern 1: ReAct Agent Loop

**What:** A loop where the model reasons about what to do, acts (calls tools), observes results, and repeats until it has a final answer or hits a safety limit.
**When to use:** Always -- this is the core execution model for the agent.
**Trade-offs:** Simple to implement and debug. The loop is explicit Go code (not a graph or state machine), so standard debugging tools work. Risk of infinite loops requires a max-iteration guard.

**Example:**
```go
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
    a.conversation.AddUser(userMessage)

    for i := 0; i < a.maxIterations; i++ {
        resp, err := a.llm.Chat(ctx, a.buildRequest())
        if err != nil {
            return "", fmt.Errorf("llm chat: %w", err)
        }

        a.conversation.AddAssistant(resp.Message)

        if len(resp.Message.ToolCalls) == 0 {
            return resp.Message.Content, nil // Done -- model gave final answer
        }

        for _, tc := range resp.Message.ToolCalls {
            result, err := a.tools.Dispatch(ctx, tc)
            if err != nil {
                a.conversation.AddToolResult(tc, fmt.Sprintf("error: %v", err))
                continue
            }
            a.conversation.AddToolResult(tc, result)
        }
    }

    return "", fmt.Errorf("agent exceeded max iterations (%d)", a.maxIterations)
}
```

### Pattern 2: Interface-Based Tool Registry

**What:** Tools implement a common interface. The registry maps tool names to handlers. The dispatcher looks up and invokes tools by name from model output.
**When to use:** Always -- this is how built-in Go tools and Lua-backed tools share a unified dispatch path.
**Trade-offs:** Clean separation between tool framework and tool implementations. Adding a new tool means implementing the interface and registering it. Slightly more boilerplate than a raw function map, but much more testable and extensible.

**Example:**
```go
// Tool interface -- every tool (Go or Lua) implements this
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage  // JSON Schema for the model
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry holds all available tools
type Registry struct {
    tools map[string]Tool
}

func (r *Registry) Register(t Tool) {
    r.tools[t.Name()] = t
}

func (r *Registry) All() []Tool {
    // Returns all tools for system prompt generation
}

func (r *Registry) Dispatch(ctx context.Context, tc api.ToolCall) (string, error) {
    t, ok := r.tools[tc.Function.Name]
    if !ok {
        return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
    }
    argsJSON, _ := json.Marshal(tc.Function.Arguments)
    return t.Execute(ctx, argsJSON)
}
```

### Pattern 3: Lua Tool as Interface Adapter

**What:** Each Lua script on disk is wrapped in a Go struct that implements the `Tool` interface. The Lua runtime executes the script when `Execute` is called.
**When to use:** For all agent-authored tools. The adapter pattern lets Lua tools be first-class citizens in the same registry as Go tools.
**Trade-offs:** Lua tools have slightly higher invocation overhead (VM setup per call or pooled VMs). But they enable the core value proposition -- self-extension. The adapter keeps the rest of the system unaware whether a tool is Go or Lua.

**Example:**
```go
type LuaTool struct {
    name        string
    description string
    params      json.RawMessage
    scriptPath  string
    runtime     *LuaRuntime
}

func (lt *LuaTool) Name() string              { return lt.name }
func (lt *LuaTool) Description() string       { return lt.description }
func (lt *LuaTool) Parameters() json.RawMessage { return lt.params }

func (lt *LuaTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    return lt.runtime.ExecScript(ctx, lt.scriptPath, args)
}
```

### Pattern 4: Constructor Injection (No Framework)

**What:** Dependencies are passed explicitly through constructors. No DI container, no magic -- just function parameters.
**When to use:** Always in Go. Wire or Uber fx are overkill for this project size.
**Trade-offs:** More explicit wiring in `main.go`, but every dependency is visible and testable. Go's implicit interface satisfaction means swapping implementations for tests is trivial.

**Example:**
```go
// In cmd/fenec/main.go
func main() {
    ollamaClient := llm.NewClient("http://localhost:11434", "gemma3")
    luaRuntime := lua.NewRuntime(lua.WithSandbox(true))
    toolRegistry := tool.NewRegistry()

    // Register built-in tools
    toolRegistry.Register(tools.NewBash())
    toolRegistry.Register(tools.NewWriteTool("./tools", toolRegistry, luaRuntime))

    // Load persisted Lua tools
    luaTools, _ := lua.LoadToolsFromDir("./tools", luaRuntime)
    for _, lt := range luaTools {
        toolRegistry.Register(lt)
    }

    agent := agent.New(ollamaClient, toolRegistry)
    repl := cli.NewREPL(agent)
    repl.Run()
}
```

## Data Flow

### Request Flow (Single Turn)

```
User types message
    |
    v
CLI REPL reads line (bufio.Scanner)
    |
    v
Agent.Run(ctx, message)
    |
    v
Conversation Manager appends user message to history
    |
    v
System Prompt Builder assembles:
  - Base system prompt (role, behavior instructions)
  - Tool descriptions from Registry.All()
    |
    v
Agent builds ChatRequest{Model, Messages, Tools}
    |
    v
Ollama Client.Chat(ctx, req, streamFn)
    |
    v
Ollama server runs inference, streams tokens
    |
    v
streamFn callback receives ChatResponse chunks
    |
    +---> If streaming content: CLI prints tokens as received
    |
    +---> If Done=true: check for ToolCalls
              |
              +---> No ToolCalls: return content to CLI (turn complete)
              |
              +---> Has ToolCalls: enter tool dispatch loop
                        |
                        v
                    For each ToolCall:
                      Registry.Dispatch(ctx, toolCall)
                        |
                        +---> Go tool: execute directly
                        +---> Lua tool: LuaRuntime.ExecScript()
                        |
                        v
                    Append tool results to conversation history
                        |
                        v
                    Loop back to Agent builds ChatRequest
                    (next iteration of ReAct loop)
```

### Self-Extension Flow (Agent Creates New Tool)

```
Model decides it needs a tool that doesn't exist
    |
    v
Model calls write_tool with:
  - name: "tool_name"
  - description: "what it does"
  - parameters: {JSON schema}
  - script: "lua source code"
    |
    v
write_tool handler:
  1. Validates Lua syntax (compile check, no execute)
  2. Writes script to tools/tool_name.lua
  3. Writes schema to tools/tool_name.json
  4. Creates LuaTool adapter
  5. Registers new tool in Registry (live, no restart)
  6. Returns success message to model
    |
    v
Model now sees new tool in next iteration's tool list
    |
    v
Model can call the new tool immediately in the same session
```

### Key Data Flows

1. **Chat flow:** User message -> conversation history -> system prompt + tools -> Ollama -> streamed response -> display to user. Straightforward request-response with streaming.
2. **Tool call flow:** Model response with ToolCalls -> dispatcher looks up handler -> execute -> result string -> append to history as tool role message -> re-prompt model. This is the ReAct inner loop.
3. **Self-extension flow:** Model calls write_tool -> Lua script persisted to disk -> adapter created -> registered in live registry -> immediately available. The tool store on disk is the persistence layer.
4. **Startup flow:** main.go wires deps -> loads Lua tools from disk -> registers all tools -> starts REPL. Tool discovery happens once at startup, plus dynamically via write_tool.

## Scaling Considerations

| Concern | At Personal Use | At Power Use (many tools) | At Multi-Session |
|---------|----------------|--------------------------|------------------|
| Context window | Not a concern -- local models handle single conversations fine | Tool descriptions consume tokens; need tool-list pruning or summarization | Add conversation persistence (SQLite) |
| Lua VM overhead | Negligible -- create per call | Pool VMs to avoid repeated setup | Pool with per-session isolation |
| Tool registry size | 5-20 tools, fine as a map | 50+ tools: model struggles to pick correctly; add tool categories or selection | Same issue; consider tool-use history to rank |
| Response latency | Dominated by model inference time | Same -- tool execution is fast relative to LLM | Same |

### Scaling Priorities

1. **First bottleneck: Context window consumption.** As the agent accumulates tools, their descriptions eat into the context window. Mitigation: keep descriptions terse, and later implement a tool selection pre-pass where the model picks relevant tools before the main prompt.
2. **Second bottleneck: Tool selection accuracy.** With many Lua tools, the model may hallucinate tool names or pick wrong tools. Mitigation: good naming conventions, clear descriptions, and consider categorization.

## Anti-Patterns

### Anti-Pattern 1: Graph-Based Agent Orchestration

**What people do:** Implement the agent loop as a state machine graph (nodes and edges) like LangGraph.
**Why it's wrong:** For a single-agent system, this adds abstraction without value. The ReAct loop is a simple `for` loop in Go. Graph abstractions obscure control flow and make debugging harder. Go already has `for`, `if`, and goroutines -- use them.
**Do this instead:** Plain Go loop with explicit tool dispatch. Use a graph only if you need multi-agent orchestration (Fenec does not).

### Anti-Pattern 2: Unbounded Agent Loop

**What people do:** Let the agent loop run indefinitely without a max iteration limit.
**Why it's wrong:** Models can enter reasoning loops, repeatedly calling the same tool, or oscillating between tools. Without a guard, this burns compute and never returns.
**Do this instead:** Set `maxIterations` (start with 10). Log each iteration. Return an error if exceeded. The user can always re-prompt.

### Anti-Pattern 3: Unsandboxed Lua Execution

**What people do:** Give the Lua VM full access to `os`, `io`, `debug`, and `require`.
**Why it's wrong:** The agent writes Lua code. An AI-authored script with `os.execute("rm -rf /")` should not be possible. Even without malice, bugs in agent-authored code could corrupt state.
**Do this instead:** Create the LState with `SkipOpenLibs: true`. Open only `base`, `table`, `string`, `math`. Provide host-controlled APIs (HTTP, file read within a sandbox directory) through registered Go functions.

### Anti-Pattern 4: Storing Tool State in the Lua VM

**What people do:** Keep tool state (counters, caches, accumulated data) inside the Lua VM between calls.
**Why it's wrong:** VM state is fragile -- if you pool or recreate VMs, state is lost. It creates invisible coupling between tool invocations.
**Do this instead:** Tools should be stateless functions. If a tool needs persistence, it writes to a file or passes data back through the conversation. The conversation history is the state.

### Anti-Pattern 5: Prompt-Injection-Style Tool Definitions

**What people do:** Describe tools only in the system prompt text rather than using Ollama's structured `Tools` field.
**Why it's wrong:** Ollama's API has first-class `Tools` support with JSON Schema. Using structured tool definitions gives the model a formal contract, not a natural language suggestion. Models trained for tool calling expect the structured format.
**Do this instead:** Use `api.Tool` structs in `ChatRequest.Tools`. Also describe tools in the system prompt for models that benefit from both.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Ollama | HTTP API via `github.com/ollama/ollama/api` | Official Go client. Streaming via callback. Default `http://localhost:11434`. |
| Shell (bash) | `os/exec.Command` | Built-in tool. Run with timeout context. Capture stdout+stderr. |
| File system | `os` / `io/fs` | Tool store reads/writes. Sandbox Lua to specific directories. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| CLI <-> Agent | Method call: `agent.Run(ctx, msg) -> (string, error)` | CLI owns I/O, Agent owns logic. Agent streams via callback for real-time display. |
| Agent <-> LLM Client | Method call: `client.Chat(ctx, req, fn)` | Behind an interface for testability. Mock in tests. |
| Agent <-> Tool Registry | Method call: `registry.Dispatch(ctx, toolCall)` | Registry is injected into Agent. Agent never knows tool implementations. |
| Tool Registry <-> Lua Runtime | Method call: `runtime.ExecScript(ctx, path, args)` | LuaTool adapter bridges the gap. Registry sees Tool interface only. |
| write_tool <-> Tool Registry | Method call: `registry.Register(newTool)` | write_tool has a reference to the registry for live registration. |
| write_tool <-> Lua Runtime | Method call: `runtime.CompileCheck(script)` | Syntax validation before persisting. Does not execute. |

## Build Order (Dependencies)

Components must be built in an order that respects their dependencies. The following sequence ensures each component can be tested independently as it is built.

```
Phase 1: Foundation
  tool.Tool interface + Registry       (no deps -- pure Go)
  llm.Client interface + Ollama impl   (depends only on ollama/api)
  cli.REPL                             (depends only on stdin/stdout)

Phase 2: Agent Core
  agent.Conversation                   (depends on api.Message types)
  agent.PromptBuilder                  (depends on tool.Registry)
  agent.Agent (ReAct loop)             (depends on llm.Client, tool.Registry, Conversation)

Phase 3: Built-in Tools
  tools.Bash                           (implements tool.Tool, uses os/exec)
  Wire CLI -> Agent -> Ollama          (first working chat without tools)
  Wire tool dispatch into agent loop   (first working tool calls)

Phase 4: Lua Runtime
  lua.Runtime                          (depends on gopher-lua, sandboxing)
  lua.Loader                           (depends on Runtime, reads disk)
  lua.LuaTool adapter                  (implements tool.Tool, uses Runtime)

Phase 5: Self-Extension
  tools.WriteTool                      (depends on Registry, Runtime, file I/O)
  Startup Lua loading in main.go       (depends on Loader, Registry)
  Full self-extension loop working     (agent can create + use Lua tools)
```

**Build order rationale:** The Tool interface and Registry are pure abstractions with no external deps -- they can be built and tested first. The LLM client wraps an external service but is simple. The Agent loop integrates these two, forming the core. Built-in tools prove the tool system works. Lua is layered on top since it is the most complex component (VM, sandboxing, file I/O). Self-extension comes last because it requires everything else to be working.

## Sources

- [Ollama Go API package](https://pkg.go.dev/github.com/ollama/ollama/api) -- Official Go client with Chat, Tool, Message structs (HIGH confidence)
- [Ollama tool calling docs](https://docs.ollama.com/capabilities/tool-calling) -- Tool definition format, conversation loop pattern (HIGH confidence)
- [Implementing LLM Tool-calling with Go and Ollama](https://dev.to/calvinmclean/how-to-implement-llm-tool-calling-with-go-and-ollama-237g) -- Practical Go implementation of the tool calling loop (MEDIUM confidence)
- [Go AI Agent Library architecture analysis](https://www.vitaliihonchar.com/insights/go-ai-agent-library) -- ReAct loop in Go, rejection of graph abstractions (MEDIUM confidence)
- [Building Effective Agents (Anthropic)](https://www.anthropic.com/research/building-effective-agents) -- Augmented LLM patterns, workflow vs agent patterns, tool engineering (HIGH confidence)
- [gopher-lua](https://github.com/yuin/gopher-lua) -- Lua 5.1 VM in Go, selective module loading for sandboxing (HIGH confidence)
- [gopher-lua sandbox discussion](https://github.com/yuin/gopher-lua/issues/55) -- SkipOpenLibs + selective OpenXXX for security (MEDIUM confidence)
- [Self-Learning AI Agent Architecture](https://www.contextstudios.ai/blog/how-to-build-a-self-learning-ai-agent-system-our-actual-architecture) -- Persistent tool/skill files, file-based memory (MEDIUM confidence)
- [Context Window Management](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) -- Message history strategies, rolling buffers (HIGH confidence)
- [Go Project Layout](https://go.dev/doc/modules/layout) -- Official Go module layout guidance (HIGH confidence)
- [Go AI Agent Frameworks 2026](https://reliasoftware.com/blog/golang-ai-agent-frameworks) -- Ecosystem overview of Go agent frameworks (MEDIUM confidence)

---
*Architecture research for: Fenec -- self-extending AI agent platform*
*Researched: 2026-04-11*
