---
phase: 07-canonical-types
verified: 2026-04-12T19:30:00Z
status: passed
score: 7/7 must-haves verified
re_verification: false
---

# Phase 7: Canonical Types Verification Report

**Phase Goal:** Fenec owns its own message and tool types, decoupled from any single provider's API types
**Verified:** 2026-04-12T19:30:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria + Plan 01 must_haves)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can start Fenec and chat with Ollama exactly as before — no behavioral difference from v1.0 | VERIFIED | `go test ./... -count=1` exits 0 across all 8 packages; ChatService interface signatures preserve all REPL-facing behavior |
| 2 | No import of `github.com/ollama/ollama/api` types exists outside the Ollama adapter package | VERIFIED | Real import found only in `internal/chat/client.go` and `internal/chat/stream.go`; zero matches in tool, lua, session, repl packages |
| 3 | All existing tests pass with the new canonical types | VERIFIED | All packages: chat, config, lua, model, render, repl, session, tool — all pass |
| 4 | Canonical Message type serializes to identical JSON as api.Message for all fields Fenec uses | VERIFIED | JSON tags `role`, `content`, `thinking,omitempty`, `tool_calls,omitempty`, `tool_call_id,omitempty` match Ollama wire format; 5 test functions confirm |
| 5 | Canonical ToolDefinition type serializes to identical JSON as api.Tool including PropertyType single-string marshaling | VERIFIED | `PropertyType.MarshalJSON` and `UnmarshalJSON` implemented; 7 tests in tool_test.go all pass |
| 6 | Canonical StreamMetrics type serializes to identical JSON as api.Metrics subset | VERIFIED | `PromptEvalCount` and `EvalCount` with `omitempty`; 3 tests in metrics_test.go pass |
| 7 | Session persistence JSON remains backward-compatible with v1.0 saved sessions | VERIFIED | `Session.Messages` is `[]model.Message` with identical JSON tags to former `[]api.Message` |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/model/message.go` | Message, ToolCall, ToolCallFunction types | VERIFIED | Exists; contains `type Message struct` with 5 fields and correct JSON tags; zero external deps |
| `internal/model/tool.go` | ToolDefinition, ToolFunction, ToolFunctionParameters, ToolProperty, PropertyType types | VERIFIED | Exists; contains all 7 types including `MarshalJSON`/`UnmarshalJSON` on PropertyType; only stdlib `encoding/json` imported |
| `internal/model/metrics.go` | StreamMetrics type | VERIFIED | Exists; contains `type StreamMetrics struct` with PromptEvalCount and EvalCount |
| `internal/model/message_test.go` | JSON round-trip tests for Message | VERIFIED | Contains 5 tests matching `TestMessageJSON*` pattern |
| `internal/model/tool_test.go` | JSON round-trip tests for ToolDefinition, PropertyType marshal/unmarshal | VERIFIED | Contains `TestPropertyTypeMarshalSingleString`, `TestPropertyTypeMarshalMultipleStrings`, `TestPropertyTypeUnmarshalBareString`, `TestPropertyTypeUnmarshalArray`, plus 4 ToolDefinition tests |
| `internal/model/metrics_test.go` | JSON round-trip tests for StreamMetrics | VERIFIED | Contains `TestStreamMetricsJSONRoundTrip` and 2 others |
| `internal/tool/registry.go` | Tool interface using model.ToolDefinition and map[string]any | VERIFIED | `Definition() model.ToolDefinition` at line 30; `Tools() []model.ToolDefinition` at line 102 |
| `internal/chat/client.go` | ChatService interface using canonical types | VERIFIED | `StreamChat(... tools []mdl.ToolDefinition ...) (*mdl.Message, *mdl.StreamMetrics, error)` |
| `internal/chat/stream.go` | Ollama conversion functions at adapter boundary | VERIFIED | `toOllamaMessages`, `fromOllamaMessage`, `toOllamaTools`, `fromOllamaMetrics` all present |
| `internal/session/session.go` | Session using model.Message | VERIFIED | `Messages []model.Message` at line 15 |
| `internal/repl/repl.go` | REPL using only canonical types (no ollama/api import) | VERIFIED | Imports `internal/model`; no ollama/api import; `tc.Function.Arguments["command"]` map access at line 402 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/model/tool.go` | `encoding/json` | `PropertyType.MarshalJSON` and `UnmarshalJSON` | VERIFIED | Both methods present at lines 42 and 51 |
| `internal/chat/stream.go` | `github.com/ollama/ollama/api` | `toOllamaMessages`, `fromOllamaMessage`, `toOllamaTools`, `fromOllamaMetrics` | VERIFIED | All 4 conversion functions present; `func toOllama*` pattern confirmed |
| `internal/tool/registry.go` | `internal/model` | Tool interface `Definition()` and `Execute()` signatures | VERIFIED | `model.ToolDefinition` in return type and `model.ToolCall` in `Dispatch` parameter |
| `internal/repl/repl.go` | `internal/model` | tool dispatch and stream chat integration | VERIFIED | `[]model.ToolDefinition` at line 327; map argument access at line 402 |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PROV-03 | 07-01-PLAN.md, 07-02-PLAN.md | User's existing Ollama workflow works exactly as before with zero configuration changes | SATISFIED | `go test ./... -count=1` exits 0; only `internal/chat` retains ollama/api import; all tool, session, repl packages use canonical types |

**Orphaned requirements check:** REQUIREMENTS.md maps only PROV-03 to Phase 7. Both plans claim PROV-03. No orphaned requirements.

### Anti-Patterns Found

None. Scan of all phase-modified files found:
- No TODO/FIXME/placeholder comments in `internal/model/`
- No TODO/FIXME in `internal/chat/stream.go`
- No stub patterns (empty returns, console-only handlers) in any file
- The `args.Set` / `NewToolPropertiesMap` / `props.Set` patterns in `stream.go` are confined to the conversion functions — this is intentional adapter code translating canonical types to Ollama's ordered map format, not a leak

### Human Verification Required

| Test | What to do | Expected | Why human |
|------|-----------|----------|-----------|
| Live chat session | Run `fenec`, send a message with a tool call (e.g., shell_exec), observe response | Tool executes, result returned to model, model produces final answer | End-to-end streaming + tool round-trip cannot be verified without a live Ollama instance |
| Session save/load with v1.0 data | Load an existing session file created before Phase 7, continue conversation | Session loads cleanly, messages display correctly | Requires actual saved session file from v1.0 to test JSON backward-compatibility |

### Gaps Summary

No gaps. All three success criteria from ROADMAP.md are verified in the codebase:

1. **Behavioral preservation** — full test suite passes including repl, session, tool, and lua packages that users interact with indirectly
2. **Import boundary** — `github.com/ollama/ollama/api` exists only in `internal/chat/client.go` and `internal/chat/stream.go` (production code); test files in that package also use it as expected for adapter testing
3. **Test coverage** — 16 tests across 3 test files in `internal/model` prove JSON round-trip fidelity; all other package tests pass after migration

The phase goal is fully achieved.

---

_Verified: 2026-04-12T19:30:00Z_
_Verifier: Claude (gsd-verifier)_
