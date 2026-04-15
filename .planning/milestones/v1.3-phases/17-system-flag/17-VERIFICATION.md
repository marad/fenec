---
phase: 17-system-flag
verified: 2026-04-15T10:27:35Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run fenec --system <file> with a valid prompt file against a live model and verify the system prompt affects model behavior"
    expected: "Model responds according to the custom system prompt content, not the default system prompt"
    why_human: "Requires live Ollama server and model interaction to confirm system prompt is used in generation"
  - test: "Run fenec --system <file> and invoke a tool (e.g., ask the model to list files) to confirm tools still work"
    expected: "Tool calls succeed — model sees tool descriptions and can invoke them"
    why_human: "Requires live Ollama server to confirm tool descriptions are appended and tools are callable"
---

# Phase 17: System Flag Verification Report

**Phase Goal:** User can override the system prompt for a single invocation via a file path flag
**Verified:** 2026-04-15T10:27:35Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `fenec --system <file>` reads file content and uses it as the system prompt for that session | ✓ VERIFIED | Flag defined at line 33 (`pflag.StringP("system", "s", ...)`), file read at line 179 (`os.ReadFile(*systemFile)`), prompt set at line 185 (`systemPrompt = string(data)`), passed to REPL at line 305 |
| 2 | Tool descriptions remain functional when using `--system` override (tools still callable) | ✓ VERIFIED | REPL stores `baseSystemPrompt` (line 42/66/96) and appends tool descriptions (lines 68-72) regardless of prompt source. `refreshSystemPrompt()` (line 847) rebuilds from base + tools. Source-agnostic. |
| 3 | Without `--system`, default system prompt behavior is unchanged | ✓ VERIFIED | Else branch at lines 186-194 calls `config.LoadSystemPrompt()` — identical to original code path |
| 4 | Invalid/missing file path with `--system` produces clear error and non-zero exit | ✓ VERIFIED | Error handler at lines 180-183 with `FormatError` and `os.Exit(1)`. Behavioral test: `fenec --system /nonexistent.md` exits code 1 with `Error: Failed to read system prompt file: open ...` |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `main.go` | `--system/-s` flag definition, conditional system prompt loading | ✓ VERIFIED | Contains `pflag.StringP("system", "s"` (line 33), `if *systemFile != ""` conditional (line 178), `os.ReadFile(*systemFile)` (line 179), and default fallback via `config.LoadSystemPrompt()` (line 188) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `main.go` (flag parse, line 33) | `main.go` (system prompt loading, line 178) | `if *systemFile != ""` conditional | ✓ WIRED | Flag variable `systemFile` directly referenced in conditional at line 178 |
| `main.go` (systemPrompt variable, line 177) | `repl.NewREPL` (line 305) | Parameter pass-through | ✓ WIRED | `systemPrompt` passed as 4th arg to `repl.NewREPL(p, defaultModel, activeProviderName, systemPrompt, ...)` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `main.go` | `systemPrompt` (line 177) | `os.ReadFile(*systemFile)` or `config.LoadSystemPrompt()` | Yes — file content or config-based prompt | ✓ FLOWING |
| `internal/repl/repl.go` | `baseSystemPrompt` (line 42) | `systemPrompt` param from `NewREPL` (line 47→66→96) | Yes — stores base and appends tool descriptions (lines 68-72) | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build succeeds | `go build ./...` | Exit 0, no errors | ✓ PASS |
| Config tests pass (regression) | `go test ./internal/config/...` | `ok` (cached) | ✓ PASS |
| `--system` flag in help output | `fenec --help 2>&1 \| grep system` | `-s, --system string   File to use as system prompt for this session` | ✓ PASS |
| Usage example in help | `fenec --help 2>&1 \| grep "system prompt.md"` | `fenec --system prompt.md  Use a custom system prompt` | ✓ PASS |
| Invalid file exits non-zero | `fenec --system /tmp/nonexistent.md` | Exit 1, `Error: Failed to read system prompt file: open /tmp/nonexistent.md: no such file or directory` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-----------|-------------|--------|----------|
| FLAG-01 | 17-01-PLAN.md | `--system <file>` flag reads file and uses content as system prompt for one invocation | ✓ SATISFIED | Flag defined, file read, prompt passed to REPL, error handling present. REQUIREMENTS.md traceability table marks FLAG-01 as Complete for Phase 17. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None found | — | — |

No TODOs, FIXMEs, placeholders, empty implementations, or stub patterns detected in `main.go`.

### Human Verification Required

### 1. System Prompt Override Affects Model Behavior

**Test:** Create a file with custom system prompt (e.g., "You are a pirate. Respond in pirate speak.") and run `fenec --system pirate.md`. Ask a question and observe the response.
**Expected:** Model responds using pirate language/style, demonstrating the custom system prompt is active — not the default prompt.
**Why human:** Requires live Ollama server and model interaction to confirm system prompt influences generation.

### 2. Tools Remain Functional with `--system` Override

**Test:** Run `fenec --system <custom-prompt-file>` and ask the model to perform a tool action (e.g., "list files in the current directory").
**Expected:** Model invokes the tool (e.g., `shell_exec` or `list_dir`) and returns results. Tool descriptions are visible in the system prompt.
**Why human:** Requires live Ollama server to confirm tool descriptions are appended to custom prompt and tools are callable.

### Gaps Summary

No automated gaps found. All 4 must-haves verified through code analysis and behavioral spot-checks. Two human verification items remain: confirming the system prompt actually affects model behavior and tools remain functional during a live session with `--system` override.

---

_Verified: 2026-04-15T10:27:35Z_
_Verifier: the agent (gsd-verifier)_
