# Fenec — Code Review Report

**Date:** 2026-04-24  
**Scope:** Full codebase (~13,400 LOC across 60+ Go source files)  
**Version:** v0.1 (tag v1.3)

---

## 1. Executive Summary

Fenec is a well-structured CLI AI assistant written in Go with a clean internal architecture. The codebase demonstrates strong separation of concerns, a solid provider abstraction, and thoughtful security-by-default in tool execution. Test coverage is good overall (most packages > 75%), with clear areas for improvement. The main issues found are concentrated in the REPL layer (code duplication, low test coverage) and a few security/robustness edge cases.

**Overall Quality:** ★★★★☆ — Solid, production-capable code with room for polish.

---

## 2. Architecture Assessment

### Strengths

- **Clean package layering:** `model` → `provider` → `chat` → `tool` → `repl` → `main`. Dependencies flow one way; no cycles.
- **Provider abstraction** (`provider.Provider` interface) is well-designed — Ollama, OpenAI, and Copilot implementations are cleanly separated. The Copilot provider elegantly composes `openai.Provider` rather than duplicating logic.
- **Canonical model types** (`internal/model`) decouple all packages from vendor-specific types. Conversion happens only at the adapter boundary (e.g., `toOllamaMessages`, `toOpenAIMessages`).
- **Tool registry pattern** is extensible and clean. Built-in vs Lua provenance tracking is a nice touch.
- **Lua sandboxing** is done correctly: `SkipOpenLibs: true`, selective safe library loading, `dofile`/`loadfile` nilled out. Fresh LState per execution prevents cross-call state pollution.
- **Config hot-reload** via fsnotify with debouncing is a nice UX feature done correctly.
- **Atomic file writes** for sessions prevent corruption on crash.

### Concerns

- **`main.go` is 425 lines** with complex setup orchestration. The "approver closure" pattern (declaring `var approver` then assigning it post-REPL creation) works but is fragile. Consider extracting a `cmd` or `app` package.
- **The `repl` package is the largest and least testable** at 22.5% coverage. The `REPL` struct accumulates many responsibilities: readline, streaming, tool dispatch, session management, context tracking, spinner lifecycle.

---

## 3. Code Quality Findings

### 3.1 Critical Issues

**None found.** No data races, no panics in normal paths, no security holes in the default configuration.

### 3.2 High-Priority Issues

#### H-1: `sendMessage()` duplicates the entire streaming/spinner pattern

`repl.go` lines 250–360 contain the agentic tool loop, and lines 360–410 duplicate nearly the same streaming pattern for the max-tool-rounds summary. This is ~60 lines of copy-pasted code with minor variable name differences (`sp2`, `thinkingStarted2`, `contentStarted2`).

**Recommendation:** Extract a `streamResponse(ctx, req, rl) (content string, msg, metrics, err)` helper.

#### H-2: Context tracker `TruncateOldest` has a logic error in proportional estimation

```go
totalBefore := len(conv.Messages) + removeCount // message count before removal (for ratio)
```

This calculates `totalBefore` *after* the removal (it adds `removeCount` to the already-shortened slice). The intent is correct but the naming and position are confusing. More importantly, the proportional estimate is very rough — after several removals in a loop, the estimate drifts significantly from reality.

**Recommendation:** Calculate `totalBefore` *before* the `append` splice. Consider adding a comment about the estimation being intentionally coarse (corrected by next `PromptEvalCount`).

#### H-3: `client.go` is an empty file

`internal/chat/client.go` contains only `package chat`. This is dead weight and should either be populated or removed.

#### H-4: `.golangci.yml` is incompatible with current golangci-lint

The config file is missing a `version` field required by recent golangci-lint versions, making `golangci-lint run` fail outright. Linting is effectively non-functional.

**Recommendation:** Add `version: "2"` (or appropriate version) to the config, or regenerate from `golangci-lint config init`.

### 3.3 Medium-Priority Issues

#### M-1: `Describe()` in tool registry iterates a map without sorting

`Registry.Describe()` iterates `r.tools` (a `map[string]Tool`) for system prompt injection. Map iteration order in Go is non-deterministic, meaning the tool list in the system prompt changes between runs. This can cause unnecessary context window churn (model sees "different" system prompts).

**Recommendation:** Sort tool names before building the description, similar to how `ToolInfo()` already sorts.

#### M-2: OpenAI provider silently falls back to non-streaming when tools are present

```go
func (p *Provider) StreamChat(...) {
    if len(req.Tools) > 0 {
        return p.chatNonStreaming(...)
    }
```

This means the user sees no token-by-token streaming during the entire tool-calling phase — the content arrives all at once. For long responses, this creates a perception of hanging. The comment says "tool call arguments arrive as a complete JSON string in non-streaming mode" but OpenAI's streaming API does support tool calls (arguments come in `delta.tool_calls[].function.arguments` chunks).

**Recommendation:** Document this limitation visibly or implement streaming tool call argument assembly.

#### M-3: Shell command safety bypass via encoding tricks

`IsDangerous` checks for literal patterns like `"rm "`. An attacker (or hallucinating model) can bypass with:
- `bash -c 'rm -rf /'` — `bash` is not in the dangerous list
- `env rm -rf /` — `env` is not detected
- `$(rm -rf /)` — `$(` is handled as a separator but the inner command isn't re-checked for separators recursively
- `echo | xargs rm -rf /` — `xargs` is handled, but nested separators after it aren't

This is defense-in-depth (the user still gets a `/bin/sh -c` execution), and dangerous commands prompt for approval. But the safety layer gives a false sense of security.

**Recommendation:** Add `bash`, `sh`, `env`, `nohup`, `eval` to the command-boundary patterns. Document that the safety check is a heuristic, not a security boundary.

#### M-4: No input validation on model names

`/model some/model` and `--model` accept arbitrary strings. A model name like `../../etc/passwd` would be passed directly to the provider API. While this is unlikely to cause harm (the provider rejects unknown models), it's good hygiene to validate.

#### M-5: Session ID is a timestamp with second resolution

```go
ID: now.Format("2006-01-02T15-04-05"),
```

If `/clear` and the subsequent session creation happen within the same second, the new session ID collides with the saved session, overwriting it. This is unlikely in practice but is a latent bug.

**Recommendation:** Append a short random suffix or use millisecond resolution.

#### M-6: `edit_file` replaces only the first occurrence

The tool description says "First occurrence will be replaced" which is clear. However, if the old_text matches in multiple places, the user/model has no way to target a specific occurrence (e.g., by line number). This limits the utility for repetitive code patterns.

### 3.4 Low-Priority / Style Issues

#### L-1: Inconsistent error handling for tool arguments

Some tools return `("", error)` for missing arguments, while others return `(errorJSON(...), nil)`. The pattern varies even within the same file:
- `read_file`, `write_file`, `edit_file`: return Go errors for missing args, JSON errors for path/access issues
- `create_lua_tool`, `delete_lua_tool`: return Go errors for missing args, JSON for validation

This is actually a reasonable pattern (Go errors = programming bugs, JSON errors = model-correctable mistakes), but it's inconsistently applied and undocumented.

**Recommendation:** Add a comment to the `Tool` interface documenting the convention.

#### L-2: `maxOutput` truncation in ShellTool is byte-based, not rune-safe

```go
if len(out.Stdout) > maxOutput {
    out.Stdout = out.Stdout[:maxOutput] + "\n... (truncated)"
}
```

If `maxOutput` falls mid-UTF8 sequence, the JSON marshaling may produce invalid JSON.

**Recommendation:** Use `utf8.ValidString` truncation or truncate at the last valid rune boundary.

#### L-3: `render` package test coverage at 56%

The render package has styling functions that are mostly trivial, but the spinner and some formatting logic would benefit from tests.

#### L-4: No graceful shutdown on SIGTERM

Only `os.Interrupt` (SIGINT/Ctrl+C) is handled. `SIGTERM` (from `kill` or container stop) won't trigger auto-save.

#### L-5: `copilot` provider test takes 15 seconds

The copilot test suite takes 15s (vs <1s for other packages), likely due to real HTTP timeout waits. This slows the feedback loop.

---

## 4. Test Coverage Summary

| Package | Coverage | Assessment |
|---------|----------|------------|
| `model` | 91.7% | ✅ Excellent |
| `profile` | 91.5% | ✅ Excellent |
| `ollama` | 88.5% | ✅ Very good |
| `openai` | 88.1% | ✅ Very good |
| `chat` | 85.7% | ✅ Good |
| `lua` | 85.8% | ✅ Good |
| `tool` | 85.5% | ✅ Good |
| `config` | 78.3% | 🔶 Adequate |
| `session` | 77.2% | 🔶 Adequate |
| `copilot` | 76.6% | 🔶 Adequate |
| `render` | 56.1% | 🔸 Low |
| `profilecmd` | 49.4% | 🔸 Low |
| `repl` | 22.5% | 🔴 Needs attention |

**Key gap:** The `repl` package contains the core interaction logic (agentic loop, session management, streaming) but is only 22.5% covered. This is the highest-risk area for regressions.

---

## 5. Security Assessment

| Area | Rating | Notes |
|------|--------|-------|
| Lua sandboxing | ✅ Strong | No os/io/debug; dofile/loadfile removed; fresh state per execution |
| File path deny list | ✅ Good | Symlink resolution, fail-closed on error, covers ~/.ssh, ~/.gnupg, /etc, /usr |
| Shell command safety | 🔶 Heuristic | Pattern-based, bypassable (see M-3), but gated by user approval |
| API key storage | 🔶 Adequate | Config warns on plaintext keys, supports $ENV_VAR references |
| Session data | ✅ Good | Atomic writes, no sensitive data in session files |
| Copilot token storage | 🔶 Adequate | hosts.json written with 0600 perms, good |

---

## 6. Recommendations (Prioritized)

1. **Fix `.golangci.yml`** — linting is currently broken (H-4)
2. **Extract streaming helper in REPL** to eliminate duplication (H-1)
3. **Sort tool descriptions** for deterministic system prompts (M-1)
4. **Remove empty `client.go`** (H-3)
5. **Add SIGTERM handling** for graceful shutdown (L-4)
6. **Improve REPL test coverage** — even integration-level tests with a mock provider would help
7. **Document error return conventions** on the `Tool` interface (L-1)
8. **Add `bash`, `sh`, `env` to dangerous command patterns** (M-3)
9. **Fix session ID collision potential** (M-5)
10. **Speed up copilot tests** with shorter timeouts or mock clocks (L-5)

---

## 7. Positive Highlights

- **Well-documented design decisions** — comments reference decision IDs (D-01, D-04, etc.) making it easy to trace intent
- **Comprehensive Copilot provider** — device flow auth, token caching, automatic retry on 404 is production-quality
- **Profile system** is elegantly designed — TOML frontmatter + markdown body, composable with --system and --model flags
- **Config hot-reload** with fsnotify + debounce is a premium UX feature done right
- **The Lua self-extension system** (create/update/delete tools at runtime) is the project's unique value proposition and it's well-implemented with proper validation, sandboxing, and system prompt refresh
- **All tests pass** — no flaky tests, no skipped tests
