# Phase 3: Tool Execution - Research

**Researched:** 2026-04-11
**Domain:** Ollama tool calling API, shell execution with safety gates, agentic loop architecture
**Confidence:** HIGH

## Summary

Phase 3 transforms Fenec from a chat interface into an agentic system. The core work involves three interconnected pieces: (1) a tool registry that defines available tools and injects them into the system prompt and ChatRequest, (2) an agentic loop in the REPL that detects tool calls in streaming responses, dispatches them, feeds results back, and lets the model respond, and (3) a shell execution tool with timeout enforcement and a human approval gate for dangerous operations.

The Ollama API v0.20.5 (already in go.mod) provides native typed support for tool calling. The `ChatRequest.Tools` field accepts `[]api.Tool` definitions, the model responds with `api.Message.ToolCalls`, and tool results are returned as messages with `Role: "tool"`, `ToolName`, and `ToolCallID`. No new dependencies are needed -- all types exist in the current Ollama client. The session persistence layer already serializes `[]api.Message` which includes all tool-related fields.

Shell execution uses Go's `os/exec.CommandContext` with `context.WithTimeout` for configurable timeouts. The `WaitDelay` field (Go 1.20+) handles orphaned child processes. Dangerous command detection uses pattern matching against a configurable list of dangerous prefixes/patterns (`rm`, `sudo`, `chmod`, `mkfs`, `dd`, file write redirections, etc.) with user confirmation via the REPL's readline before execution.

**Primary recommendation:** Build a `tool` package with a `Registry` interface, implement `shell_exec` as the first built-in tool, and modify `StreamChat`/REPL to support the agentic tool-call loop. Keep the registry interface generic so Phase 4 (Lua tools) plugs in seamlessly.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TOOL-01 | Agent calls tools using structured function calling format and receives results | Ollama API v0.20.5 provides native `Tool`, `ToolCall`, `Message` types with `role:"tool"`. The agentic loop pattern sends tool definitions in `ChatRequest.Tools`, detects `ToolCalls` in response, dispatches, and feeds results back. |
| TOOL-02 | Available tools (built-in + Lua) are injected into the system prompt each turn | Tool Registry provides `ListTools() []api.Tool` for ChatRequest.Tools and `Describe() string` for system prompt injection. Both mechanisms ensure the model knows available tools. |
| TOOL-03 | Tool execution errors are returned to the model as structured error messages | Tool result messages use `Role: "tool"` with JSON-structured error content (`{"error": "message"}`), allowing the model to understand and recover from failures. |
| EXEC-01 | Agent can execute bash/shell commands and return stdout, stderr, and exit code | `os/exec.CommandContext` with `cmd.Stdout`/`cmd.Stderr` as `bytes.Buffer`, capturing exit code via `cmd.ProcessState.ExitCode()`. |
| EXEC-02 | Dangerous operations require user approval before execution | Pattern-based detection of dangerous commands (`rm -rf`, `sudo`, `>` redirects, `chmod`, etc.) with readline-based Y/n confirmation prompt before execution. |
| EXEC-03 | Shell commands have configurable timeout to prevent hangs | `context.WithTimeout` on CommandContext with `cmd.WaitDelay` for orphaned subprocess cleanup. Timeout reported as structured error to model. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/ollama/ollama/api | v0.20.5 | Tool calling types and chat API | Already in go.mod. Provides Tool, ToolCall, ToolCallFunction, ToolCallFunctionArguments, ToolFunction, ToolFunctionParameters, ToolProperty, ToolPropertiesMap -- all needed for tool definition and result handling. |
| os/exec | stdlib | Shell command execution | CommandContext provides context-based timeout and cancellation. WaitDelay handles orphaned subprocesses. No external library needed. |
| context | stdlib | Timeout and cancellation | WithTimeout for shell execution deadlines, WithCancel for REPL streaming interruption. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | Serialize tool results and arguments | Tool results sent as JSON strings in Message.Content. ToolCallFunctionArguments already handles JSON serialization. |
| log/slog | stdlib | Structured logging for tool dispatch | Log tool calls, execution results, timeouts, approval decisions for debugging. |
| syscall | stdlib | Process group management | Set Setpgid on shell commands so timeout kills the entire process tree, not just the parent shell. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pattern-based dangerous command detection | Full shell parser (mvdan.cc/sh) | A shell parser would catch all edge cases (aliases, variable expansion) but adds a dependency. Pattern matching is sufficient for the common cases (rm, sudo, chmod, etc.) and aligns with CLAUDE.md principle of avoiding unnecessary deps. Can upgrade later if needed. |
| Simple string prefix matching | Regex-based command analysis | Regex adds complexity without clear benefit for the initial set of dangerous commands. Simple prefix/substring checks are more readable and maintainable. |

**No new dependencies needed. Everything required is in stdlib + existing Ollama API.**

## Architecture Patterns

### Recommended Project Structure
```
internal/
  tool/
    registry.go       # Tool interface + Registry type
    shell.go           # shell_exec tool implementation
    shell_test.go      # Shell execution tests
    safety.go          # Dangerous command detection + approval
    safety_test.go     # Safety gate tests
  chat/
    client.go          # (existing) -- add Tools field support to StreamChat
    message.go         # (existing) -- add AddToolResult helper
    stream.go          # (existing) -- modify to detect tool calls
  repl/
    repl.go            # (existing) -- add agentic loop to sendMessage
```

### Pattern 1: Tool Registry
**What:** A registry that holds all available tools (built-in Go tools now, Lua tools in Phase 4). Provides both the Ollama API tool definitions and a dispatch mechanism.
**When to use:** Every ChatRequest includes the registry's tool list. Every tool call response is dispatched through the registry.
**Example:**
```go
// Source: Project architecture, verified against Ollama API v0.20.5 types

// Tool is the interface every tool must implement.
type Tool interface {
    // Name returns the tool's unique identifier (used in dispatch).
    Name() string
    // Definition returns the Ollama API tool definition.
    Definition() api.Tool
    // Execute runs the tool with the given arguments and returns a result string.
    // The context carries timeout/cancellation. The approver is called for dangerous ops.
    Execute(ctx context.Context, args api.ToolCallFunctionArguments) (string, error)
}

// Registry holds registered tools and dispatches calls.
type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry {
    return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
    r.tools[t.Name()] = t
}

// Tools returns api.Tools for injection into ChatRequest.
func (r *Registry) Tools() api.Tools {
    var tools api.Tools
    for _, t := range r.tools {
        tools = append(tools, t.Definition())
    }
    return tools
}

// Dispatch executes a tool call and returns the result as a string.
func (r *Registry) Dispatch(ctx context.Context, call api.ToolCall) (string, error) {
    t, ok := r.tools[call.Function.Name]
    if !ok {
        return "", fmt.Errorf("unknown tool: %s", call.Function.Name)
    }
    return t.Execute(ctx, call.Function.Arguments)
}

// Describe returns a human-readable description of all tools for the system prompt.
func (r *Registry) Describe() string {
    // Format tool names and descriptions for system prompt injection
}
```

### Pattern 2: Agentic Loop (Tool Call Cycle)
**What:** After the model streams a response, check if it contains tool calls. If so, execute each tool, append results as tool messages, and send the updated conversation back for the model's final answer. Repeat until the model responds without tool calls.
**When to use:** Every call to the model in the REPL.
**Example:**
```go
// Source: Ollama API v0.20.5 types + official tool calling docs

// The agentic loop in sendMessage (simplified):
for {
    msg, metrics, err := client.StreamChat(ctx, conv, onToken)
    // handle errors...

    // Check for tool calls in the response
    if len(msg.ToolCalls) == 0 {
        // No tool calls -- model gave a final text response
        conv.AddAssistant(msg.Content)
        break
    }

    // Model made tool calls -- add the assistant message (with tool calls) to history
    conv.Messages = append(conv.Messages, *msg)

    // Execute each tool call and add results
    for _, tc := range msg.ToolCalls {
        result, err := registry.Dispatch(ctx, tc)
        if err != nil {
            result = fmt.Sprintf(`{"error": %q}`, err.Error())
        }
        conv.Messages = append(conv.Messages, api.Message{
            Role:       "tool",
            Content:    result,
            ToolCallID: tc.ID,
        })
    }
    // Loop back -- send updated conversation to model for next response
}
```

### Pattern 3: Shell Execution with Safety Gate
**What:** The shell_exec tool runs commands via `/bin/sh -c`, captures stdout/stderr/exit_code, enforces timeout, and requires approval for dangerous commands.
**When to use:** When the model calls the `shell_exec` tool.
**Example:**
```go
// Source: Go stdlib os/exec, verified against Go 1.26 docs

func executeShell(ctx context.Context, command string, timeout time.Duration) (*ShellResult, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
    cmd.WaitDelay = 5 * time.Second // Kill orphaned children after 5s grace

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()

    exitCode := 0
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return &ShellResult{
                Stdout:   stdout.String(),
                Stderr:   stderr.String(),
                ExitCode: -1,
                TimedOut: true,
            }, nil
        }
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        } else {
            return nil, err
        }
    }

    return &ShellResult{
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
        ExitCode: exitCode,
    }, nil
}
```

### Pattern 4: Dangerous Command Detection
**What:** Before executing a shell command, check it against a list of dangerous patterns. If matched, prompt the user for confirmation.
**When to use:** Inside the shell_exec tool, before running the command.
**Example:**
```go
// Dangerous command patterns -- check command string against these
var dangerousPatterns = []string{
    "rm ",        // File deletion
    "rm\t",       // rm with tab
    "rmdir ",     // Directory removal
    "sudo ",      // Privilege escalation
    "chmod ",     // Permission changes
    "chown ",     // Ownership changes
    "mkfs",       // Filesystem creation (destructive)
    "dd ",        // Raw disk operations
    "> ",         // File write redirect (overwrite)
    ">> ",        // File append redirect
    "mv ",        // File move (can overwrite)
    "kill ",      // Process termination
    "killall ",   // Mass process termination
    "pkill ",     // Pattern-based process kill
    "reboot",     // System reboot
    "shutdown",   // System shutdown
    "systemctl ", // Service management
    "apt ",       // Package management
    "dnf ",       // Package management
    "pacman ",    // Package management
}

func isDangerous(command string) bool {
    cmd := strings.TrimSpace(command)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(cmd, pattern) {
            return true
        }
    }
    return false
}
```

### Anti-Patterns to Avoid
- **Blocking the REPL during tool execution without feedback:** Always show the user what tool is being called and its progress. Print tool call info before execution.
- **Swallowing tool errors:** Never silently fail. Return structured error messages to the model so it can adapt.
- **Unbounded tool call loops:** Set a maximum number of tool call iterations per user message (e.g., 10) to prevent infinite loops where the model keeps calling tools.
- **Executing commands without shell:** Always use `/bin/sh -c` rather than exec.Command with split args. The model generates shell commands, not tokenized argument arrays.
- **Ignoring ToolCallID:** The Ollama API uses ToolCall.ID and Message.ToolCallID to correlate calls with results. Always set these correctly.
- **Adding tool definitions to system prompt text AND ChatRequest.Tools:** Use ChatRequest.Tools for the structured definitions (Ollama uses these to format the prompt template). Add a brief natural-language summary to the system prompt only if the model needs extra guidance on when/how to use tools.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tool call JSON schema | Custom JSON schema builder | api.Tool + api.ToolFunctionParameters + api.ToolPropertiesMap | Ollama API v0.20.5 has typed builders (NewToolPropertiesMap, ToolProperty.Set). Using them ensures schema compatibility. |
| Shell argument parsing | Custom command tokenizer | `/bin/sh -c "command"` | The model outputs shell syntax. Parsing it yourself introduces bugs. Let the shell handle it. |
| Process timeout | Manual timer + kill goroutine | exec.CommandContext + context.WithTimeout | Go stdlib handles this correctly including edge cases with orphaned processes (WaitDelay). |
| JSON serialization of tool args | Manual map construction | api.ToolCallFunctionArguments.ToMap() | The ordered map type handles JSON round-tripping correctly. |
| Tool call correlation | Custom ID generation | Use ToolCall.ID from the model's response | Ollama generates the IDs. Your tool result message just references the same ID. |

**Key insight:** The Ollama API v0.20.5 has evolved significantly -- it now has ordered maps for tool properties and arguments, proper ToolCallID correlation, and typed tool definitions. The blog post examples using `map[string]api.ToolFunctionProperty` are outdated. Use `ToolPropertiesMap` with `.Set()`.

## Common Pitfalls

### Pitfall 1: Outdated Ollama API Type Names
**What goes wrong:** Code using `ToolFunctionProperty` or `map[string]ToolFunctionProperty` from older blog posts won't compile. The v0.20.5 API uses `ToolProperty` and `*ToolPropertiesMap` (an ordered map, not a regular Go map).
**Why it happens:** The Ollama API types were refactored between v0.6 and v0.20 to use ordered maps for deterministic JSON serialization.
**How to avoid:** Always reference the actual types in `api/types.go` at v0.20.5. Use `api.NewToolPropertiesMap()` and `pm.Set(key, api.ToolProperty{...})`.
**Warning signs:** Compilation errors mentioning `ToolFunctionProperty` or type mismatch on `Properties` field.

### Pitfall 2: Streaming Tool Calls Accumulation
**What goes wrong:** Tool calls may arrive across multiple streaming chunks. If you only check the final chunk, you might miss tool calls or get partial data.
**Why it happens:** Ollama's incremental parser streams tool calls as they are recognized. The `ToolCalls` field on `resp.Message` may be populated on intermediate chunks.
**How to avoid:** In the streaming callback, accumulate `resp.Message.ToolCalls` in addition to content. When streaming completes (`Done=true`), use the accumulated tool calls for dispatch. Alternatively, collect the full assistant message and check its ToolCalls after streaming ends.
**Warning signs:** Tool calls being silently dropped, especially with models that output thinking tokens before tool calls.

### Pitfall 3: Shell Command Timeout vs Process Group
**What goes wrong:** `exec.CommandContext` kills the parent process on timeout, but child processes (spawned by the shell) continue running.
**Why it happens:** By default, the shell command's children inherit the parent PID but not process group termination.
**How to avoid:** Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and use `cmd.WaitDelay` to handle cleanup. The WaitDelay mechanism (Go 1.20+) provides a grace period for orphaned I/O pipes.
**Warning signs:** Test processes hanging after timeout, or zombie processes accumulating during long sessions.

### Pitfall 4: Infinite Tool Call Loop
**What goes wrong:** The model keeps making tool calls in a loop -- each result triggers another tool call, consuming context and time.
**Why it happens:** The model may not realize it has enough information, or it may be stuck in a pattern (e.g., repeatedly checking if a file was created).
**How to avoid:** Set a maximum iteration count (e.g., 10 tool call rounds per user message). When exceeded, force the model to respond with text by omitting tools from the next request or adding a message telling it to summarize.
**Warning signs:** Console filling with repeated tool call/result cycles for a single user input.

### Pitfall 5: Gemma 4 Tool Calling Reliability
**What goes wrong:** Gemma 4 may produce malformed tool calls, fail to use tools when it should, or produce tool calls with incorrect argument types.
**Why it happens:** STATE.md notes: "Gemma 4 tool calling reliability has active compatibility issues with Ollama v0.20.0." Local models have lower tool calling reliability than cloud models.
**How to avoid:** Implement robust parsing that handles malformed tool calls gracefully (log warning, return error to model). Do not assume arguments will always have the expected types -- validate and coerce. Consider testing with multiple models (llama3.2, qwen3) during development.
**Warning signs:** JSON parse errors in tool call arguments, empty function names, or missing required arguments.

### Pitfall 6: Missing Assistant Message in Tool Call History
**What goes wrong:** When the model makes a tool call, its message (containing the tool_calls field) must be added to the conversation BEFORE the tool result messages. If omitted, the model loses context about what it asked for.
**Why it happens:** It's tempting to only add the tool result and skip the assistant's tool-call message.
**How to avoid:** Always append the complete assistant message (with ToolCalls populated) to conversation history, then append tool result messages after it.
**Warning signs:** Model repeating tool calls it already made, or not understanding tool results.

## Code Examples

Verified patterns from the Ollama API v0.20.5 source:

### Building a Tool Definition
```go
// Source: Ollama API v0.20.5 api/types.go -- verified against actual types

props := api.NewToolPropertiesMap()
props.Set("command", api.ToolProperty{
    Type:        api.PropertyType{"string"},
    Description: "The shell command to execute",
})

shellTool := api.Tool{
    Type: "function",
    Function: api.ToolFunction{
        Name:        "shell_exec",
        Description: "Execute a shell command and return stdout, stderr, and exit code",
        Parameters: api.ToolFunctionParameters{
            Type:     "object",
            Required: []string{"command"},
            Properties: props,
        },
    },
}
```

### Sending Tools in ChatRequest
```go
// Source: Ollama API v0.20.5 -- ChatRequest.Tools field

req := &api.ChatRequest{
    Model:    conv.Model,
    Messages: conv.Messages,
    Tools:    registry.Tools(),  // api.Tools ([]api.Tool)
    Truncate: boolPtr(false),
}
if conv.ContextLength > 0 {
    req.Options = map[string]any{"num_ctx": conv.ContextLength}
}
```

### Constructing a Tool Result Message
```go
// Source: Ollama API v0.20.5 -- Message.Role "tool", Message.ToolCallID

resultMsg := api.Message{
    Role:       "tool",
    Content:    `{"stdout": "hello\n", "stderr": "", "exit_code": 0}`,
    ToolCallID: toolCall.ID,
}
conv.Messages = append(conv.Messages, resultMsg)
```

### Reading Tool Call Arguments
```go
// Source: Ollama API v0.20.5 -- ToolCallFunctionArguments

for _, tc := range msg.ToolCalls {
    args := tc.Function.Arguments
    // Use Get for individual values
    if cmdVal, ok := args.Get("command"); ok {
        command, _ := cmdVal.(string)
        // execute command...
    }
    // Or convert to regular map
    argMap := args.ToMap()
}
```

### Shell Result as JSON for Model
```go
// Source: Project design -- structured tool results

type ShellResult struct {
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exit_code"`
    TimedOut bool   `json:"timed_out,omitempty"`
}

func (r *ShellResult) ToJSON() string {
    // Truncate very long output to avoid blowing up context
    const maxOutput = 4096
    stdout := r.Stdout
    if len(stdout) > maxOutput {
        stdout = stdout[:maxOutput] + "\n... (truncated)"
    }
    stderr := r.Stderr
    if len(stderr) > maxOutput {
        stderr = stderr[:maxOutput] + "\n... (truncated)"
    }
    
    result := ShellResult{
        Stdout:   stdout,
        Stderr:   stderr,
        ExitCode: r.ExitCode,
        TimedOut: r.TimedOut,
    }
    b, _ := json.Marshal(result)
    return string(b)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `map[string]api.ToolFunctionProperty` for tool params | `*api.ToolPropertiesMap` with ordered map | Ollama v0.20+ (2025) | Must use NewToolPropertiesMap() + Set(), not map literals |
| Tool calls only in final non-streaming response | Streaming tool calls via incremental parser | Ollama May 2025 | Tool calls can arrive mid-stream; must accumulate |
| No ToolCallID correlation | ToolCall.ID + Message.ToolCallID | Ollama v0.20+ | Tool results must reference the correct call ID |
| exec.Command manual timeout | exec.CommandContext + WaitDelay | Go 1.20 | Cleaner timeout with orphan process cleanup |

**Deprecated/outdated:**
- `ToolFunctionProperty` type: Replaced by `ToolProperty` with `ToolPropertiesMap`
- Manual `syscall.Kill(-pid, syscall.SIGKILL)` for process groups: Use `WaitDelay` instead for most cases
- Blog post patterns using `map[string]any` for ToolCall arguments: v0.20.5 uses `ToolCallFunctionArguments` (ordered map) with Get/Set/ToMap methods

## Open Questions

1. **Streaming Tool Call Accumulation Strategy**
   - What we know: Tool calls arrive in streaming chunks on `resp.Message.ToolCalls`. The current `StreamChat` only accumulates `Content`.
   - What's unclear: Whether Ollama sends partial tool calls across chunks (name in one chunk, arguments in another) or complete tool calls per chunk.
   - Recommendation: Accumulate the complete Message from the final `Done=true` chunk, which should have the fully-assembled ToolCalls. If intermediate chunks have partial tool calls, defer to the final chunk. Test empirically with Gemma 4.

2. **Approval Gate UX During Streaming**
   - What we know: The shell tool needs user approval for dangerous commands. The REPL uses readline for input.
   - What's unclear: How to prompt for approval in the middle of a tool dispatch cycle without disrupting the streaming output flow.
   - Recommendation: After the model's streaming response completes and before tool execution, print the tool call info and prompt for approval. The streaming output is already done at this point. Use a simple Y/n readline prompt.

3. **Tool Call Output Display**
   - What we know: Users need to see what tools are being called and their results.
   - What's unclear: The exact format for displaying tool calls and results in the REPL.
   - Recommendation: Print a concise indicator like `[tool: shell_exec] command: ls -la` before execution and `[result: exit_code=0, 5 lines]` after. Keep it minimal -- the model's final response will summarize.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed -- Go convention |
| Quick run command | `go test ./internal/tool/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TOOL-01 | Tool call dispatch and result feedback | unit | `go test ./internal/tool/ -run TestRegistryDispatch -v` | Wave 0 |
| TOOL-02 | Tool definitions injected into system prompt / ChatRequest | unit | `go test ./internal/tool/ -run TestRegistryTools -v` | Wave 0 |
| TOOL-03 | Error results returned as structured messages | unit | `go test ./internal/tool/ -run TestDispatchError -v` | Wave 0 |
| EXEC-01 | Shell execution captures stdout/stderr/exit_code | unit | `go test ./internal/tool/ -run TestShellExec -v` | Wave 0 |
| EXEC-02 | Dangerous command detection and approval | unit | `go test ./internal/tool/ -run TestDangerous -v` | Wave 0 |
| EXEC-03 | Shell timeout kills process and reports | unit | `go test ./internal/tool/ -run TestShellTimeout -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/tool/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before verification

### Wave 0 Gaps
- [ ] `internal/tool/registry_test.go` -- covers TOOL-01, TOOL-02, TOOL-03
- [ ] `internal/tool/shell_test.go` -- covers EXEC-01, EXEC-03
- [ ] `internal/tool/safety_test.go` -- covers EXEC-02

## Sources

### Primary (HIGH confidence)
- Ollama API v0.20.5 source: `/home/marad/go/pkg/mod/github.com/ollama/ollama@v0.20.5/api/types.go` -- Tool, ToolCall, ToolCallFunction, ToolCallFunctionArguments, ToolProperty, ToolPropertiesMap, ToolFunctionParameters, ToolFunction, Message (ToolCalls, ToolName, ToolCallID) types verified directly
- Ollama API v0.20.5 source: `/home/marad/go/pkg/mod/github.com/ollama/ollama@v0.20.5/api/client.go` -- Chat method and ChatResponseFunc verified
- [Go os/exec documentation](https://pkg.go.dev/os/exec) -- CommandContext, Cancel, WaitDelay fields verified for Go 1.26
- [Ollama tool calling docs](https://docs.ollama.com/capabilities/tool-calling) -- tool definition schema, role "tool" messages
- [Ollama streaming tool calls blog](https://ollama.com/blog/streaming-tool) -- incremental parser, streaming tool call support since May 2025

### Secondary (MEDIUM confidence)
- [Go tool calling implementation guide](https://dev.to/calvinmclean/how-to-implement-llm-tool-calling-with-go-and-ollama-237g) -- Agentic loop pattern (note: uses older API types, code patterns verified but type names differ from v0.20.5)
- [Ollama chat API docs](https://docs.ollama.com/api/chat) -- Request/response format
- [Go os/exec patterns](https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/) -- Process group management, stdout/stderr capture

### Tertiary (LOW confidence)
- Gemma 4 tool calling reliability -- known issues noted in STATE.md but no definitive fix documented. Needs empirical testing at implementation time.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All types verified directly in Ollama v0.20.5 source code and Go stdlib
- Architecture: HIGH - Agentic loop pattern is well-established, Ollama API types map directly to the pattern
- Pitfalls: HIGH - Type name changes verified in source, streaming behavior from official blog, process timeout from Go docs
- Shell safety: MEDIUM - Dangerous command list is a design decision; patterns are standard but completeness is debatable

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- Ollama API types unlikely to change within a minor version)
