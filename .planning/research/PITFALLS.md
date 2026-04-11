# Pitfalls Research

**Domain:** AI agent platform (Go + LuaJIT + Ollama, self-extending via Lua tools)
**Researched:** 2026-04-11
**Confidence:** HIGH (verified across official docs, GitHub issues, and multiple community sources)

## Critical Pitfalls

### Pitfall 1: Ollama Silent Context Truncation Destroys Tool Calling

**What goes wrong:**
Ollama defaults to a 2048-4096 token context window. When the system prompt (with tool definitions), conversation history, and tool results exceed this, Ollama silently discards the oldest tokens in FIFO order. The system prompt containing tool definitions gets truncated first, meaning the model suddenly "forgets" tools exist mid-conversation. Tool calling stops working with no error message. The agent appears to understand you but can no longer use tools, and there is zero indication this has happened.

**Why it happens:**
Ollama's default num_ctx is conservative (2048) to work on low-memory hardware. Tool definitions in the system prompt can easily consume 500-1500 tokens for even a modest tool set. Add a few conversation turns with tool call/response pairs, and you blow past the limit within 3-4 exchanges. As the agent self-extends and accumulates more Lua tools, the tool definitions section of the system prompt grows continuously, accelerating the problem.

**How to avoid:**
- Set num_ctx explicitly in every API request (minimum 8192 for basic use, 32768+ recommended for agentic workflows with tools).
- Track token usage: count system prompt tokens + conversation tokens before each request. Warn or summarize when approaching 75% of the context window.
- Implement conversation compaction: summarize older turns when approaching the limit rather than letting Ollama silently truncate.
- For self-extending tools: implement a tool pagination or selection strategy so only relevant tools are included in the system prompt for a given task, rather than dumping all tools every time.

**Warning signs:**
- Agent stops calling tools after working correctly for several turns
- Agent starts "hallucinating" answers to questions it previously would have used tools for
- Agent responds "I don't have access to tools" when they are defined
- Longer conversations degrade faster than short ones

**Phase to address:**
Phase 1 (Ollama integration). This must be solved from the first API call. If deferred, every feature built on top will have intermittent, impossible-to-debug failures.

---

### Pitfall 2: Unsandboxed Lua Execution Enables Arbitrary System Access

**What goes wrong:**
The agent writes Lua scripts that persist to disk and execute in future sessions. gopher-lua loads all built-in libraries by default, including `io` (file system access), `os` (command execution, environment variables), and `loadfile`/`dofile` (load arbitrary code). A model-generated Lua script -- whether from a hallucination, prompt injection, or honest mistake -- can read/write arbitrary files, execute system commands, or load malicious code from the network. Since scripts persist, a single bad script becomes a permanent backdoor.

**Why it happens:**
gopher-lua has no built-in sandboxing. The library explicitly delegates sandboxing to the host application (confirmed in GitHub issue #27 and #11). Developers often defer sandboxing to "later" because the initial development loop of "write script, test script" feels safe. But the moment the model writes and persists a script, the attack surface is open.

**How to avoid:**
- Use `lua.OpenXXX` functions selectively instead of `OpenLibs()`. Only open `base`, `table`, `string`, `math`. Never open `io`, `os`, `debug`, or `loadfile`/`dofile`.
- Create a whitelist environment: start with an empty global table and explicitly register only safe functions.
- Expose Go-implemented functions for controlled I/O: the Lua script calls `fenec.read_file(path)` which goes through Go validation (path allowlisting, size limits) rather than Lua's native `io.open`.
- Validate all agent-generated Lua before persisting: parse the AST or at minimum scan for forbidden function calls (`os.execute`, `io.open`, `loadfile`, `dofile`, `load`, `rawset`, `rawget`, `debug.*`).
- Set execution time limits via instruction counting (gopher-lua supports this via context cancellation).
- Set memory limits on LState creation (use `Options{RegistrySize, CallStackSize}` to cap resource usage).

**Warning signs:**
- Lua scripts that import/require unexpected modules
- Scripts containing `os.execute`, `io.open`, or `loadfile` calls
- Scripts that write to paths outside the designated tools directory
- Unexplained file changes or network activity during tool execution

**Phase to address:**
Phase 1 (LuaJIT integration). Sandboxing must be the very first thing built, before any script execution. Retrofitting sandboxing after scripts already exist is much harder than building it from the start.

---

### Pitfall 3: Bash Tool Enables Arbitrary Command Execution Without Guardrails

**What goes wrong:**
The built-in bash tool lets the model execute shell commands. Without constraints, the model can `rm -rf /`, install packages, exfiltrate data, modify system configs, or execute network commands. Even well-intentioned commands can be destructive -- a model trying to "clean up" might delete important files. Combined with the self-extension capability, the model could write a Lua tool that wraps shell access in a way that bypasses any bash-tool-specific restrictions.

**Why it happens:**
Using `exec.Command("bash", "-c", userInput)` in Go passes the entire string to bash for interpretation, enabling shell metacharacters, pipes, redirects, and command chaining. The model is not adversarial, but it is unreliable -- it will occasionally generate destructive commands through misunderstanding, hallucination, or overly aggressive problem-solving.

**How to avoid:**
- Never use `sh -c` or `bash -c` with model-generated strings. Use `exec.Command(program, arg1, arg2, ...)` with separate arguments when possible.
- Implement a confirmation workflow for destructive operations: classify commands and require human approval for anything that modifies the system (writes, deletes, installs, network access).
- Maintain a command allowlist for auto-approved operations (ls, cat, grep, find, head, tail, wc) and require confirmation for everything else.
- Set execution timeouts (5-30 seconds default).
- Run commands in a restricted environment: use a chroot, container, or at minimum a restricted PATH.
- Log every command with full arguments before execution.
- Rate-limit command execution to prevent runaway loops.

**Warning signs:**
- Model generating commands with `sudo`, `rm -rf`, `chmod 777`, `curl | bash`, or `>` redirects to system files
- Commands that chain with `&&` or `;` to sneak in extra operations
- Model attempting to install packages or modify system state
- Repeated failed commands followed by escalating attempts

**Phase to address:**
Phase 1 (bash tool). The bash tool should ship with restrictions from day one. A "just log and execute everything" approach, even for personal use, will bite you the first time the model hallucinates a destructive command.

---

### Pitfall 4: Tool Call Parsing Fragility with Local Models

**What goes wrong:**
Local models (especially smaller ones like 8B parameter variants) produce malformed tool calls: wrong JSON syntax, hallucinated function names, missing required parameters, parameters of wrong types, or tool calls embedded in natural language instead of structured format. The agent crashes, silently ignores the tool call, or enters an error loop. Gemma 4 specifically uses a custom format (`<|tool_call>call:FUNCTION_NAME{...}<tool_call|>`) that differs from OpenAI's format, and Ollama's tool call parser has known issues with certain models in v0.20.0+.

**Why it happens:**
Local models have less reliable structured output than cloud models (GPT-4, Claude). They were not trained with the same volume of tool-calling examples. The model's tool call format depends on how Ollama's template parses the model's output, and template mismatches cause tool calls to be dropped entirely. With more tools in the prompt, smaller models increasingly confuse tool schemas or generate calls to non-existent tools.

**How to avoid:**
- Implement defensive parsing: wrap all tool call parsing in error recovery. Never assume the model output will be well-formed JSON.
- Add a validation layer between raw model output and tool dispatch: verify function name exists, all required parameters present, types match schema.
- Implement graceful degradation: when a tool call fails to parse, feed the error back to the model with context ("Your tool call was malformed: [specific error]. Please try again with this format: [example]").
- Test with the specific Ollama model version you ship with. Tool calling behavior varies significantly between model versions and quantization levels.
- Use Ollama's native `/api/chat` endpoint for tool calling rather than the OpenAI-compatible `/v1` endpoint, as the native endpoint handles tool calls more reliably.
- Consider a retry budget: allow 2-3 parsing retries before giving up on a tool call.

**Warning signs:**
- JSON parse errors in tool call responses
- Model wrapping tool calls in markdown code blocks or natural language
- Model calling tools that don't exist (hallucinated tool names)
- Tool calls that work with one model but fail with another
- Tool calls breaking after Ollama or model updates

**Phase to address:**
Phase 1 (tool system). Build the validation and error recovery layer from the start. It is not optional when using local models.

---

### Pitfall 5: Agent Loop Hangs and Runaway Execution

**What goes wrong:**
The agent enters an infinite loop: calling a tool, getting an error, retrying with the same arguments, getting the same error, retrying again. Or oscillating: tool A produces output that triggers tool B, which produces output that triggers tool A. Or retry storms: tool fails, both the agent retry logic and Go-level retry logic fire, multiplying requests. The agent burns resources indefinitely while the user waits. For a CLI app, this manifests as a hung terminal with no output.

**Why it happens:**
Agent loops are the most common failure mode in production AI agents. The model has no inherent concept of "I'm stuck" -- it sees each turn independently. Without explicit progress tracking, the model genuinely believes each retry might succeed. Local models are especially prone to this because they have less sophisticated reasoning about failure recovery.

**How to avoid:**
- Implement hard limits: max_tool_calls per turn (e.g., 10), max_turns per conversation exchange (e.g., 20), total execution timeout (e.g., 2 minutes).
- Deduplicate tool calls: if the same function + same arguments is called twice in a row, inject a reflection prompt ("You called [tool] with the same arguments and got the same error. Try a different approach or explain the problem to the user.").
- Track progress explicitly: maintain a set of "completed actions" and "failed actions" in the agent context.
- Implement circuit breakers: after N consecutive tool errors, stop the loop and ask the user for guidance.
- Always provide a user interrupt mechanism (Ctrl+C handler that cleanly terminates the current agent loop and returns to the REPL prompt).
- For non-retriable errors (permission denied, file not found), classify the error and tell the model not to retry.

**Warning signs:**
- Same tool call appearing 3+ times in sequence
- Agent response time growing unboundedly
- CPU/memory usage spiking during agent execution
- User unable to get a response because the agent is "thinking"

**Phase to address:**
Phase 1 (tool execution engine). The loop control and circuit breaker must be in the execution engine from the start. Every feature built on top inherits these protections.

---

### Pitfall 6: Self-Extension Without Validation Creates Persistent Bad Tools

**What goes wrong:**
The agent writes a Lua tool that has a bug, a security flaw, or simply does not work correctly. This tool persists to disk. On next startup, it is loaded and presented to the model as an available tool. The model calls it, it fails or produces wrong results, and the model may even try to "fix" it by writing another broken version. Over time, the tools directory accumulates broken or redundant tools that pollute the system prompt with noise and waste context window tokens. Worse: a subtly broken tool that returns incorrect results (rather than erroring) will silently corrupt the agent's behavior.

**Why it happens:**
The "write tool, persist to disk, load on startup" pattern has no quality gate. The model is not a reliable programmer -- it generates code that looks correct but has edge cases, type errors, or logic bugs. Without testing, these persist as "capabilities" the model trusts.

**How to avoid:**
- Implement a tool validation pipeline: after the agent writes a Lua script, run it through syntax validation (gopher-lua can parse without executing), then a basic smoke test (execute with test inputs and check it does not crash).
- Add a tool staging area: new tools go to a "pending" directory, not the active tools directory. Only promote after validation.
- Include a tool metadata file alongside each Lua script: description, input schema, output schema, creation date, author (agent vs human), test status.
- Implement tool versioning: when the agent "fixes" a tool, keep the old version. Allow rollback.
- Set a maximum tool count: if the tools directory grows beyond N tools, prompt the user to review and prune.
- Include tool health monitoring: track tool call success/failure rates. Flag tools with high failure rates for review.

**Warning signs:**
- Tools directory growing without corresponding user benefit
- Model frequently calling tools that error out
- Multiple versions of the same tool (bash_v1.lua, bash_v2.lua, bash_fixed.lua)
- System prompt growing so large it degrades model performance (circles back to Pitfall 1)

**Phase to address:**
Phase 2 or 3 (self-extension). The self-extension feature should not ship without at minimum syntax validation and a staging workflow. "Write directly to active tools" is the path to an unusable tools directory within a week.

---

### Pitfall 7: gopher-lua LState Lifecycle Mismanagement

**What goes wrong:**
gopher-lua's LState is not goroutine-safe. Sharing an LState across goroutines causes data races and crashes. Creating a new LState for every Lua tool invocation wastes memory and time (each LState allocates registry and callstack buffers). LState context leaks (via NewThread/coroutines) cause gradual memory growth that manifests as the process slowly consuming more RAM over hours of use.

**Why it happens:**
Go developers naturally reach for goroutines and shared state. gopher-lua's API does not enforce single-goroutine access, so it silently corrupts rather than erroring. The context leak in NewThread (GitHub discussion #437) is especially insidious because it only manifests under sustained use.

**How to avoid:**
- Implement an LState pool (gopher-lua's README shows the pattern): reuse LStates instead of creating new ones, protect the pool with a sync.Mutex.
- Never share an LState across goroutines. One LState per goroutine, communicate via channels.
- Configure LState options for your use case: use auto-growing registry and callstack (`Options{RegistrySize: 256, RegistryMaxSize: 0, RegistryGrowStep: 32}`) to balance memory and performance.
- If running the same Lua scripts repeatedly, share compiled bytecode between LStates (bytecode is read-only and safe to share).
- Ensure proper LState.Close() on every path (including error paths). Use defer.
- Monitor Go process RSS over time during development. A slow leak indicates LState or context issues.

**Warning signs:**
- Process memory growing steadily over long sessions
- Random panics or data corruption during concurrent tool execution
- "Weird" Lua behavior where global state from one script leaks into another
- Performance degradation over time within a single session

**Phase to address:**
Phase 1 (LuaJIT integration). The LState management pattern must be established from the first Lua execution. Retrofitting pooling onto a "just create a new LState" approach requires touching every call site.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoded Ollama URL (localhost:11434) | Faster initial development | Cannot support remote Ollama, Docker setups, or custom ports | Never -- use a config value from the start, it costs nothing |
| Dumping all tools into system prompt | Simple implementation | Context window bloat as tools grow, model confusion from too many options | Only with fewer than 5 tools. Must address before self-extension ships |
| String concatenation for prompt building | Quick to write | Impossible to test, debug, or modify. Prompt injection surface | Never -- use a template system (Go's text/template is fine) |
| `OpenLibs()` for Lua environment | All Lua features available | Full system access from Lua scripts, security nightmare | Never -- always use selective OpenXXX from day one |
| Single-file tool storage (no metadata) | Tools are just .lua files | No versioning, no schema, no test status, no way to manage growth | Only in Phase 1 prototyping. Must add metadata before self-extension |
| Synchronous Ollama calls (no streaming) | Simpler implementation | Terrible UX -- user stares at blank terminal for 5-30 seconds | Only acceptable for tool call responses (not visible to user). Chat responses must stream |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Ollama API | Using the OpenAI-compatible `/v1/chat/completions` endpoint for tool calling | Use Ollama's native `/api/chat` endpoint -- tool call parsing is more reliable, especially for Gemma 4 |
| Ollama API | Not setting `num_ctx` in requests, relying on model defaults | Always pass `num_ctx` explicitly in every request. Default of 2048 is unusable for agent workflows |
| Ollama API | Assuming model is loaded and ready | Handle cold start: first request after idle may take 10-30 seconds. Set `keep_alive` parameter to prevent model unloading during a session |
| Ollama streaming | Parsing each NDJSON chunk independently for tool calls | Accumulate streaming chunks and parse tool calls from the complete response. Partial tool call JSON in a single chunk is malformed |
| gopher-lua | Registering Go functions that panic on bad input from Lua | Wrap every Go function exposed to Lua with error handling. Lua scripts will pass unexpected types, nil values, and wrong argument counts |
| gopher-lua | Using global variables in Lua scripts for state | Globals leak between script executions within the same LState. Use local variables or pass state explicitly via function arguments |
| Gemma 4 on Ollama | Expecting OpenAI-format tool calls | Gemma 4 uses its own tool call format (`<\|tool_call>...<tool_call\|>`). Ollama translates this, but verify your Ollama version handles it correctly |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| New LState per tool call | Increasing latency, memory growth | LState pool with reuse | Noticeable after 20+ tool calls in a session |
| Loading all Lua files on startup | Slow startup, wasted memory | Lazy loading: parse tool metadata on startup, load bytecode on first call | Noticeable at 50+ tools |
| Full conversation history in every request | Token count explodes, context truncation | Conversation compaction: summarize old turns, keep recent N turns verbatim | Breaks at 5-10 tool-heavy conversation turns |
| Unbounded tool output in context | Single tool returning huge output fills context window | Truncate tool output to a max token count (e.g., 2000 tokens). Summarize if needed | Breaks when bash tool runs `cat` on a large file |
| Synchronous tool execution | Agent blocks waiting for slow tool (network request, large file) | Set per-tool execution timeouts. Run tool execution in goroutine with context cancellation | First time a Lua tool makes a slow network call |
| Ollama cold start on every session | 10-30 second delay on first message | Send a lightweight "ping" request on startup to warm the model. Configure `keep_alive` to prevent unloading | Every time the user launches the CLI after Ollama has been idle |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Agent-written Lua scripts with `os.execute()` access | Arbitrary command execution persisted as a "tool" | Whitelist Lua environment, never expose `os` or `io` libraries |
| Bash tool accepting raw model output as shell command | Command injection via shell metacharacters (`;`, `&&`, `\|`, backticks) | Use exec.Command with separate args, or validate/allowlist commands |
| Tool output fed back to model without sanitization | Indirect prompt injection: a file or command output contains instructions that hijack the model | Sanitize tool outputs, strip known injection patterns, consider output length limits |
| Lua tools directory writable by the running process without constraints | Model writes a tool that modifies or deletes other tools, or writes to paths outside tools dir | Validate write paths, use a dedicated tools directory with no path traversal |
| No authentication on Ollama API (default) | Any local process can send requests to Ollama and get responses from the loaded model | Not critical for personal use, but be aware if running on shared machines |
| Persistent Lua tools not reviewed after creation | Subtly malicious or broken tools accumulate, become "trusted" by default | Implement tool review workflow, at minimum log tool creation with diff |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No streaming for chat responses | User stares at blank terminal for 5-30 seconds, thinks app is frozen | Stream tokens as they arrive, show a "thinking" indicator during tool execution |
| No indication when tools are being called | User sees silence, doesn't know if agent is working or stuck | Print tool call indicators: "[Calling bash: ls -la ...]" with a spinner |
| Tool errors shown as raw tracebacks | User sees gopher-lua internals or Go panics | Catch and format errors: "Tool 'file_reader' failed: file not found at /path" |
| No way to interrupt a long-running agent loop | User must Ctrl+C the entire process, losing conversation state | Handle SIGINT gracefully: cancel current tool execution, return to REPL prompt |
| All tools shown in system prompt create confusion | Model calls irrelevant tools for simple tasks, or gets overwhelmed | Implement tool relevance filtering, or let model see a tool index before requesting full schemas |
| No conversation persistence between sessions | User loses all context when exiting the CLI | Save conversation history to disk, offer session resume |

## "Looks Done But Isn't" Checklist

- [ ] **Ollama integration:** Often missing explicit `num_ctx` setting -- verify context window is adequate (8192+ minimum) and that tool definitions + conversation fit within it
- [ ] **Tool calling:** Often missing error recovery for malformed tool calls -- verify the agent gracefully handles unparseable model output without crashing
- [ ] **Bash tool:** Often missing execution timeouts -- verify commands that hang (e.g., `cat /dev/urandom`) get killed after timeout
- [ ] **Lua sandbox:** Often missing restriction of built-in libraries -- verify `os.execute`, `io.open`, `loadfile`, `dofile` are not available in agent-written scripts
- [ ] **Streaming:** Often missing partial line handling -- verify that streamed markdown renders correctly and doesn't produce garbled terminal output
- [ ] **Agent loop:** Often missing termination conditions -- verify the agent cannot call tools indefinitely (set max_tool_calls, max_turns, total_timeout)
- [ ] **Self-extension:** Often missing syntax validation before persist -- verify a Lua syntax error in agent-written code does not persist a broken tool
- [ ] **Tool discovery:** Often missing token budget accounting -- verify adding 10+ tools doesn't blow past the context window
- [ ] **Error handling:** Often missing Go-to-Lua bridge error handling -- verify passing nil or wrong types from Lua to Go-registered functions does not panic
- [ ] **Conversation history:** Often missing compaction -- verify a 20-turn conversation with tool calls doesn't silently lose its system prompt to context truncation

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Context truncation breaking tools | LOW | Set `num_ctx` higher, implement token counting, restart conversation |
| Unsandboxed Lua scripts already persisted | MEDIUM | Audit all scripts in tools directory, add sandbox, re-validate existing scripts against new restrictions |
| Broken tool accumulated in tools dir | LOW | Delete or move to quarantine directory, implement validation pipeline going forward |
| LState memory leak in long session | LOW | Restart the CLI process. Fix LState lifecycle for next session |
| Agent loop consuming resources | LOW | Ctrl+C (if handler works), implement circuit breaker for next time |
| Malicious command executed via bash tool | HIGH | Assess damage, restore from backup. Prevention is the only real strategy here |
| Model generating prompt injection via tool output | MEDIUM | Sanitize tool outputs, add output filtering layer, consider using separate model context for tool results vs conversation |
| Tools directory bloated with redundant scripts | MEDIUM | Manual review and pruning session, implement tool health metrics going forward |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Silent context truncation | Phase 1 (Ollama integration) | Token counting test: fill context to 90%, verify no silent truncation and warning is emitted |
| Unsandboxed Lua execution | Phase 1 (LuaJIT integration) | Security test: Lua script attempts `os.execute`, `io.open` -- verify they fail |
| Bash command injection | Phase 1 (bash tool) | Security test: model output containing `; rm -rf /` does not execute the second command |
| Tool call parsing fragility | Phase 1 (tool system) | Fault injection test: feed malformed JSON as tool call, verify graceful error and retry |
| Agent loop hangs | Phase 1 (tool execution engine) | Stress test: create a tool that always errors, verify agent stops after max retries |
| Self-extension without validation | Phase 2-3 (self-extension) | Test: agent writes syntactically invalid Lua, verify it does not persist to active tools |
| LState lifecycle mismanagement | Phase 1 (LuaJIT integration) | Memory test: execute 100 Lua tools, verify RSS is stable (not growing linearly) |
| Cold start latency | Phase 1 (Ollama integration) | UX test: launch CLI, first response arrives within acceptable time (model pre-warming) |
| Tool output prompt injection | Phase 2 (tool integration hardening) | Test: tool returns output containing "ignore previous instructions", verify model behavior unchanged |
| Tools directory bloat | Phase 3 (self-extension maturity) | Metric: track tool count, system prompt token count, tool success rates over time |

## Sources

- [Ollama context length documentation](https://docs.ollama.com/context-length)
- [Ollama tool calling documentation](https://docs.ollama.com/capabilities/tool-calling)
- [Ollama streaming tool calling blog](https://ollama.com/blog/streaming-tool)
- [gopher-lua GitHub repository](https://github.com/yuin/gopher-lua)
- [gopher-lua sandbox discussion (issue #11)](https://github.com/yuin/gopher-lua/issues/11)
- [gopher-lua filesystem restriction (issue #27)](https://github.com/yuin/gopher-lua/issues/27)
- [gopher-lua LState pooling (issue #335)](https://github.com/yuin/gopher-lua/issues/335)
- [gopher-lua memory optimizations (issue #197)](https://github.com/yuin/gopher-lua/issues/197)
- [gopher-lua context leak discussion (#437)](https://github.com/yuin/gopher-lua/discussions/437)
- [Gemma 4 function calling documentation](https://ai.google.dev/gemma/docs/capabilities/text/function-calling-gemma4)
- [Gemma 4 tool calling with Ollama compatibility issues](https://github.com/anomalyco/opencode/issues/20995)
- [Go command injection prevention (Semgrep)](https://semgrep.dev/docs/cheat-sheets/go-command-injection)
- [Go command injection (Snyk)](https://snyk.io/blog/understanding-go-command-injection-vulnerabilities/)
- [Agent infinite loop failure mode (Medium)](https://medium.com/@komalbaparmar007/llm-tool-calling-in-production-rate-limits-retries-and-the-infinite-loop-failure-mode-you-must-2a1e2a1e84c8)
- [Infinite agent loop patterns](https://www.agentpatterns.tech/en/failures/infinite-loop)
- [Ollama Gemma 4 tool calling broken in v0.20.0](https://www.gemma4.wiki/ollama/gemma-4-ollama-chat-completion)
- [OWASP Top 10 for LLM Applications](https://owasp.org/www-project-top-10-for-large-language-model-applications/)
- [Ollama model cold start and keep-alive](https://myangle.net/ollama-keep-model-loaded-in-memory/)
- [Lua sandboxing techniques](http://lua-users.org/wiki/SandBoxes)
- [Ollama num_ctx silent truncation (issue #2714)](https://github.com/ollama/ollama/issues/2714)

---
*Pitfalls research for: Fenec -- Go + LuaJIT + Ollama AI agent platform with self-extension*
*Researched: 2026-04-11*
