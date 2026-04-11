# Feature Landscape

**Domain:** Personal AI agent platform (CLI, local models, self-extending via LuaJIT)
**Researched:** 2026-04-11

## Table Stakes

Features users expect from any CLI AI agent in 2026. Missing = product feels broken or abandoned.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Streaming response output | Every CLI agent streams tokens as they arrive. Blocking until complete feels broken. Users expect to see the model "thinking" in real time. | Medium | Ollama API supports streaming natively. Must handle partial token rendering, line wrapping, and interrupt (Ctrl+C) cleanly. |
| Conversation context management | Users expect multi-turn conversation that remembers what was said. Without this, every message is isolated and the agent feels stateless. | Medium | At minimum: sliding window of recent messages. Must track token usage against model context limits and truncate/summarize older turns. Ollama models vary in context window size (8K-128K). |
| Structured tool calling with result feedback | The model must be able to call tools AND see the results in a follow-up turn. One-shot tool calling without feeding results back is useless for multi-step reasoning. | High | This is the core loop: model proposes tool call -> engine executes -> result injected into conversation -> model reasons about result. Ollama supports tool calling via OpenAI-compatible API for supported models. |
| Bash/shell command execution | Every serious CLI agent can run shell commands. This is the fundamental "hands" of the agent. Users expect it from day one. | Low | Execute command, capture stdout/stderr/exit code, return to model. Must handle timeouts and output truncation for large outputs. |
| Human approval for dangerous operations | Post-2024, users expect agents to ask before running destructive commands (rm -rf, sudo, network operations). Running arbitrary commands without confirmation is a dealbreaker. | Medium | Risk classification: auto-approve reads, confirm writes/deletes, block or require explicit approval for destructive operations. Pattern-match on command content. Claude Code, Codex CLI, and every major agent does this. |
| Tool discovery in system prompt | The model must know what tools are available. Standard practice is injecting tool descriptions into the system prompt or using the model's native function-calling schema. | Low | List all registered tools (built-in + Lua) with name, description, parameter schema. Inject into system prompt on each turn. |
| Graceful error handling and reporting | When a tool fails, the model should see the error and reason about it, not crash. Users expect resilience. | Low | Return structured errors from tool execution back to the model conversation. Include error type, message, and any relevant context. |
| Session persistence (conversation save/load) | Users expect to be able to resume conversations. Losing all context on exit feels like a toy. | Medium | Save conversation history to disk (JSON or similar). Load on startup or via explicit command. Enables the agent to pick up where it left off. |
| Configurable model selection | Users with Ollama have multiple models installed. They expect to pick which model to use, not be hardcoded to one. | Low | CLI flag or config file for model name. Query Ollama for available models. Default to a sensible choice but allow override. |
| Markdown/code rendering in terminal | Model responses contain code blocks, lists, headers. Rendering raw markdown looks terrible. Users expect formatted output. | Medium | Use a terminal markdown renderer (glamour or similar). Syntax highlighting for code blocks. Must handle wide/narrow terminals gracefully. |

## Differentiators

Features that set Fenec apart. Not universally expected, but provide competitive advantage. The self-extension capability is Fenec's primary differentiator.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Self-extending via LuaJIT tool authoring | The agent writes new Lua tools that persist to disk and become available in future sessions. This is Fenec's core value proposition -- the agent grows more capable over time through use. No other local-first CLI agent does this with an embedded scripting runtime. | High | Model generates Lua code -> validated -> saved to tools directory -> loaded on next session (or hot-loaded). Must provide a clear Lua API for tools (register name, description, parameters, handler function). Sandbox the Lua execution environment. |
| Lua tool hot-reloading | New tools become available immediately within the current session, not just on restart. The agent writes a tool and can use it in the same conversation. | Medium | Watch tools directory or explicitly reload after write. Re-inject updated tool list into system prompt for subsequent turns. |
| Tool composition and chaining | Agent can use the output of one tool as input to another within a single reasoning sequence. Multi-step tool use without human intervention for approved operations. | Medium | This emerges naturally from the tool-call-result-feedback loop if implemented correctly. The model reasons about results and decides next tool call. Not a separate feature, but the architecture must support it cleanly. |
| Lua tool library ecosystem | Ship a curated set of useful Lua tools out of the box (file operations, HTTP requests, JSON parsing, text processing). Gives the agent immediate utility beyond bash and provides examples for self-authored tools. | Medium | Start with 5-10 high-value built-in Lua tools. These also serve as templates the agent references when writing new tools. |
| Tool validation and testing | Before persisting a new Lua tool, validate it: syntax check, dry-run, schema validation. Prevents the agent from polluting its tool library with broken tools. | Medium | Lua syntax check is cheap. Can also run a simple test invocation. Reject tools that fail validation and explain the error to the model so it can fix and retry. |
| Conversation summarization for long sessions | Automatically compress older conversation turns into summaries when approaching context limits. Preserves important context while staying within token budgets. | High | Requires a secondary model call (or same model) to generate summaries. Must decide what to keep verbatim vs. summarize. Complex to get right -- too aggressive loses context, too conservative wastes tokens. |
| System prompt templating and customization | Let users define custom system prompts, persona, and behavioral rules. Personal assistant should behave how the user wants. | Low | Config file with system prompt template. Support variable interpolation (tool list, date, user name). |
| Ollama model capability detection | Automatically detect which models support tool calling vs. plain chat. Adjust behavior accordingly (structured tool calling vs. prompt-based tool parsing). | Medium | Query Ollama model metadata. Some models support native function calling, others need prompt engineering to output structured tool calls. Graceful degradation for models without native tool support. |

## Anti-Features

Features to explicitly NOT build. These are tempting but wrong for Fenec's scope and philosophy.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Cloud/remote model provider support | Fenec is local-first. Adding OpenAI/Anthropic/Google API support fragments testing, adds API key management complexity, and dilutes the local-first value proposition. Ollama already provides access to dozens of models. | Support Ollama exclusively. If users want cloud models, Ollama proxies exist. Revisit only if there's overwhelming demand. |
| Web UI or GUI | A web interface is a completely different product with auth, state management, frontend build pipeline, and browser compatibility concerns. It would double the codebase for marginal benefit when the target user is comfortable in a terminal. | Stay CLI. Invest in excellent terminal UX (markdown rendering, streaming, colors, progress indicators). |
| Multi-user support | This is a personal assistant. Multi-user adds auth, session isolation, permission models, and data separation. Massive complexity for a single-user tool. | Single user. Single config directory. No auth layer. |
| MCP server/client protocol | MCP is powerful but adds enormous protocol complexity (JSON-RPC, capability negotiation, transport layers, auth). Fenec's LuaJIT extension model is simpler and more aligned with self-extension. Adopting MCP is a future milestone, not a foundation concern. | Use Fenec's own tool registration API. If MCP interop is needed later, implement it as a bridge tool, not as the core protocol. |
| Multi-agent orchestration | Multi-agent systems (planner + executor + evaluator) are complex to build and debug. A single capable agent with good tools is more reliable and predictable for a personal assistant. | Single agent with multi-step tool use. The agent plans and executes within one conversation loop. Add multi-agent only if specific use cases demand it. |
| RAG / vector database integration | Embedding pipelines, vector stores, chunking strategies, and retrieval tuning are each individually complex. Not needed for the foundation when conversation context and tool results provide sufficient grounding. | Use conversation history and tool results for context. If knowledge retrieval is needed later, implement it as a Lua tool that searches files. |
| Autonomous background execution | Agents running unsupervised in the background making decisions is a safety and trust problem for a v1 personal assistant. Users need to build trust with the agent first. | Interactive only. The human is always in the loop. Every tool call is visible. Consider background tasks as a much later enhancement with extensive safety work. |
| Plugin marketplace / sharing platform | Building distribution infrastructure (registry, versioning, trust/signing, dependency resolution) is a product in itself. Way too early. | Tools are Lua files in a directory. Users can share by copying files. That's enough for now. |

## Feature Dependencies

```
Ollama Integration
  -> Streaming Response Output
  -> Configurable Model Selection
  -> Model Capability Detection

Conversation Context Management
  -> Session Persistence (save/load relies on having a context model)
  -> Conversation Summarization (extends context management)

Tool System (registration + discovery)
  -> Tool Discovery in System Prompt
  -> Bash Tool (first built-in tool)
  -> LuaJIT Integration (second tool type)
     -> Lua Tool Library (built-in Lua tools)
     -> Self-Extension (agent writes new Lua tools)
        -> Tool Validation (gate before persisting)
        -> Tool Hot-Reloading (make new tools immediately available)

Structured Tool Calling + Result Feedback
  -> Tool Composition and Chaining (emerges from the feedback loop)
  -> Human Approval Gate (intercepts between call and execution)

Markdown Rendering (independent, can be added anytime)
System Prompt Templating (independent, can be added anytime)
```

## MVP Recommendation

Prioritize these for the first working version, in dependency order:

1. **Ollama integration with streaming** -- without this, nothing works. Connect to Ollama, send messages, stream responses to the terminal.
2. **Conversation context management** -- multi-turn conversation. Track message history, manage token limits with a simple truncation strategy.
3. **Tool system with structured calling and result feedback** -- the core agent loop. Model sees tools, proposes calls, engine executes, results feed back.
4. **Bash tool** -- first concrete tool. Immediate utility. Low complexity given the tool system exists.
5. **Human approval for tool execution** -- safety gate before bash execution. Simple confirm/deny prompt. Non-negotiable for a tool that runs shell commands.
6. **LuaJIT integration** -- embed Lua runtime, register Lua scripts as tools, execute them through the tool system.
7. **Self-extension** -- the crown jewel. Agent writes Lua tools that persist. This is what makes Fenec unique.

Defer:
- **Session persistence**: Valuable but not needed for the agent to function. Add in the phase after core is working.
- **Markdown rendering**: Nice polish but not functional. Add when the core loop is solid.
- **Conversation summarization**: Complex to implement well. Use simple truncation first, upgrade later.
- **Tool hot-reloading**: Self-extension works fine with "restart to pick up new tools" initially.
- **Model capability detection**: Start by targeting one model that supports tool calling well (Gemma 4). Generalize later.

## Sources

- Ollama tool calling documentation: https://docs.ollama.com/capabilities/tool-calling
- Codex CLI approval system: https://developers.openai.com/codex/agent-approvals-security
- Google ADK safety patterns: https://google.github.io/adk-docs/safety/
- LangChain context engineering: https://docs.langchain.com/oss/python/langchain/context-engineering
- MCP specification: https://modelcontextprotocol.io/specification/2025-11-25
- JetBrains context management research: https://blog.jetbrains.com/research/2025/12/efficient-context-management/
- Letta agent memory patterns: https://www.letta.com/blog/agent-memory
- InfoQ CLI agent patterns: https://www.infoq.com/articles/ai-agent-cli/
- Darwin Godel Machine (self-improving agents): https://sakana.ai/dgm/
- VentureBeat Memento-Skills framework: https://venturebeat.com/orchestration/new-framework-lets-ai-agents-rewrite-their-own-skills-without-retraining-the-underlying-model
- NVIDIA sandboxing guidance: https://developer.nvidia.com/blog/practical-security-guidance-for-sandboxing-agentic-workflows-and-managing-execution-risk/
