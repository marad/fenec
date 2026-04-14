# Phase 7: Canonical Types - Research

**Researched:** 2026-04-12
**Domain:** Go type abstraction / provider decoupling
**Confidence:** HIGH

## Summary

Phase 7 introduces Fenec-owned types in a new `internal/model` package to replace direct usage of `github.com/ollama/ollama/api` types throughout the codebase. This is a pure refactoring phase -- no behavioral changes, no new features. The end result is that only the Ollama adapter (created in Phase 8) will import `ollama/api` types; all other packages use Fenec-native types.

The codebase currently has 30 Go files across 7 packages importing `ollama/api`, with ~310 individual type references. The most-used types are `api.Message` (62 refs), `api.ChatRequest`/`api.ChatResponse` (used only in `internal/chat`), `api.Tool` (22 refs), `api.ToolCallFunctionArguments` (42 refs combined with constructor), and `api.Metrics` (5 refs). The migration is straightforward because the Ollama types are simple structs with JSON tags -- no complex behavior needs replication.

**Primary recommendation:** Create `internal/model` with zero external dependencies, define canonical types mirroring the subset of Ollama types actually used, update all packages to import `internal/model` instead of `ollama/api`, and confine all Ollama-specific code to `internal/chat` (which becomes the Ollama adapter boundary in Phase 8).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion -- pure infrastructure phase.

Key constraints from research:
- 30 Go files across 6 packages (chat, tool, session, repl, lua, config) import `ollama/api`
- New `internal/model` package must have zero external dependencies
- Canonical types: Message, ToolDefinition, ToolCall, StreamMetrics (at minimum)
- Tool.Execute() signature changes from `api.ToolCallFunctionArguments` to `map[string]any`
- Tool.Definition() return type changes from `api.Tool` to canonical ToolDefinition
- Session persistence JSON must remain backward-compatible or include version migration
- All existing tests must pass after migration

### Claude's Discretion
All implementation choices are at Claude's discretion -- pure infrastructure phase.

### Deferred Ideas (OUT OF SCOPE)
None -- infrastructure phase stays within scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PROV-03 | User's existing Ollama workflow works exactly as before with zero configuration changes | Canonical types mirror Ollama types field-for-field. Session JSON backward compatibility preserved via identical JSON tags. Chat behavior unchanged -- only type wrappers change, not logic. |
</phase_requirements>

## Standard Stack

No new libraries needed. This phase uses only Go stdlib types.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| (none) | - | `internal/model` has zero external deps | By design -- canonical types must not couple to any provider |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | JSON marshal/unmarshal for canonical types | Session persistence, tool argument handling |
| iter | stdlib (Go 1.23+) | Range-over-function iterators | Only if replicating ordered map behavior from `ToolPropertiesMap` |

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── model/           # NEW: Canonical types (zero external deps)
│   ├── message.go   # Message type
│   ├── tool.go      # ToolDefinition, ToolCall, ToolCallArguments
│   └── metrics.go   # StreamMetrics (subset of Ollama Metrics)
├── chat/            # Becomes the Ollama adapter boundary
│   ├── client.go    # ChatService interface uses canonical types
│   ├── stream.go    # Converts between ollama/api <-> model types internally
│   └── message.go   # Conversation uses model.Message
├── tool/            # Uses model.ToolDefinition, model.ToolCall
├── session/         # Uses model.Message for persistence
├── lua/             # Uses map[string]any for tool args
└── repl/            # Uses canonical types only
```

### Pattern 1: Canonical Type Definition
**What:** Define minimal structs mirroring the Ollama types actually used, with identical JSON tags for serialization compatibility.
**When to use:** For every Ollama type referenced outside `internal/chat`.

The canonical types needed, based on actual codebase usage:

```go
// internal/model/message.go
package model

// Message represents a single message in a conversation.
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content"`
    Thinking   string     `json:"thinking,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}
```

Key observations from the Ollama `api.Message`:
- `Images []ImageData` -- NOT used anywhere in Fenec. Omit.
- `ToolName string` -- NOT used anywhere in Fenec. Omit.
- `Role`, `Content`, `Thinking`, `ToolCalls`, `ToolCallID` -- all actively used.

```go
// internal/model/tool.go
package model

// ToolDefinition describes a tool available for model use.
// Mirrors the JSON schema format used by Ollama and OpenAI.
type ToolDefinition struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Parameters  ToolFunctionParameters `json:"parameters"`
}

type ToolFunctionParameters struct {
    Type       string                    `json:"type"`
    Required   []string                  `json:"required,omitempty"`
    Properties map[string]ToolProperty   `json:"properties"`
}

type ToolProperty struct {
    Type        PropertyType `json:"type,omitempty"`
    Description string       `json:"description,omitempty"`
    Enum        []any        `json:"enum,omitempty"`
}

type PropertyType []string

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
    ID       string           `json:"id,omitempty"`
    Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
    Index     int            `json:"index"`
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}
```

```go
// internal/model/metrics.go
package model

// StreamMetrics holds token usage metrics from a streaming response.
type StreamMetrics struct {
    PromptEvalCount int `json:"prompt_eval_count,omitempty"`
    EvalCount       int `json:"eval_count,omitempty"`
}
```

### Pattern 2: Simplify ToolCallFunctionArguments to map[string]any
**What:** Replace the Ollama `ToolCallFunctionArguments` ordered map with a plain `map[string]any`.
**When to use:** Everywhere tool arguments are used.

The Ollama `ToolCallFunctionArguments` is an ordered map with methods: `Get(key)`, `Set(key, value)`, `All()`, `Len()`, `ToMap()`. In Fenec, tool implementations only use `Get(key)` to extract individual arguments, and `All()` in `ArgsToLuaTable`. Insertion order is not meaningful for tool execution.

Replacing with `map[string]any` means:
- `args.Get("path")` becomes `args["path"]` (or a helper function)
- `args.All()` becomes `for k, v := range args`
- `api.NewToolCallFunctionArguments()` becomes `make(map[string]any)`
- `args.Set("key", val)` becomes `args["key"] = val`

This is a significant simplification. The CONTEXT.md explicitly calls out this change.

### Pattern 3: Simplify ToolPropertiesMap to map[string]ToolProperty
**What:** Replace the Ollama `ToolPropertiesMap` ordered map with `map[string]ToolProperty`.
**When to use:** In `ToolFunctionParameters.Properties`.

Similar to ToolCallFunctionArguments -- the ordered map is unnecessary for Fenec's use. Tool definitions are built programmatically and property order doesn't affect model behavior.

### Pattern 4: Adapter Conversion in internal/chat
**What:** The `internal/chat` package (specifically `stream.go` and `client.go`) remains the Ollama adapter. It converts between Fenec canonical types and Ollama API types at the boundary.
**When to use:** StreamChat converts `[]model.Message` to `[]api.Message` before calling Ollama, and converts `api.Message` back to `model.Message` in the response.

```go
// Conversion functions in internal/chat (adapter boundary)

func toOllamaMessages(msgs []model.Message) []api.Message { ... }
func fromOllamaMessage(msg api.Message) model.Message { ... }
func toOllamaTools(defs []model.ToolDefinition) api.Tools { ... }
func fromOllamaMetrics(m api.Metrics) model.StreamMetrics { ... }
```

### Pattern 5: Session JSON Backward Compatibility
**What:** Session JSON files use `api.Message` serialization format. The canonical `model.Message` must produce identical JSON.
**When to use:** Session save/load operations.

The `api.Message` JSON tags are:
- `"role"`, `"content"`, `"thinking,omitempty"`, `"images,omitempty"`, `"tool_calls,omitempty"`, `"tool_name,omitempty"`, `"tool_call_id,omitempty"`

The canonical `model.Message` uses identical tags for the fields that exist. Fields omitted from the canonical type (`images`, `tool_name`) were never populated by Fenec, so they appear as `null` or absent in JSON. This means existing session files will deserialize correctly into the canonical type -- absent/null fields are handled by `omitempty` and Go's zero values.

**Critical verification:** The `api.Message` has a custom `UnmarshalJSON` method. We need to verify that standard `encoding/json` unmarshaling works for the canonical type by checking that `api.Message`'s custom unmarshaler doesn't do anything special that Fenec relies on.

Checked: The custom `UnmarshalJSON` on `api.Message` handles backward compatibility for Ollama's own format changes (e.g., tool call format evolution). Since Fenec only saves messages it creates, and these use the current format, standard `encoding/json` unmarshaling is sufficient.

### Anti-Patterns to Avoid
- **Wrapping Ollama types instead of owning types:** Do NOT create `type Message = api.Message` type aliases. This defeats the purpose -- the canonical types must be fully independent.
- **Converter explosion:** Do NOT create per-field converters. Use simple struct-to-struct conversion functions.
- **Partial migration:** Do NOT leave some packages using `api.Message` and others using `model.Message`. The migration must be complete in one phase.
- **Over-engineering the types:** Do NOT add methods or behavior to canonical types that don't exist in the current usage. Keep them as plain data structs with JSON tags.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Ordered map for tool properties | Custom ordered map type | `map[string]ToolProperty` | Property order is irrelevant for JSON schema; models don't care about key ordering |
| Ordered map for tool arguments | Custom ordered map type | `map[string]any` | Argument order is irrelevant for tool dispatch; simplifies all tool implementations |
| JSON serialization compatibility | Custom marshal/unmarshal | Identical JSON struct tags | Go's `encoding/json` handles this correctly when tags match |
| Type conversion helpers | Generic reflection-based converter | Simple hand-written struct copying | Only 4-5 conversion functions needed; reflection adds complexity for no gain |

## Common Pitfalls

### Pitfall 1: JSON Tag Mismatch Breaking Session Load
**What goes wrong:** If canonical `model.Message` has different JSON tags than `api.Message`, existing saved sessions fail to load.
**Why it happens:** Subtle differences like `"tool_calls"` vs `"toolCalls"` or missing `omitempty`.
**How to avoid:** Copy JSON tags verbatim from `api.Message`. Write a test that unmarshals a known session JSON into `model.Message`.
**Warning signs:** Session load tests failing after migration.

### Pitfall 2: ToolCallFunctionArguments.Get() vs Map Access
**What goes wrong:** The Ollama `ToolCallFunctionArguments.Get(key)` returns `(any, bool)`, exactly like map access. But code using it via `args.Get("key")` needs to change to `args["key"]`.
**Why it happens:** Every single tool's `Execute` method uses `args.Get("key")` -- there are 8 built-in tools plus LuaTool.
**How to avoid:** Create a helper function `func GetArg(args map[string]any, key string) (any, bool)` to minimize code changes. Or simply change to map indexing with ok-check.
**Warning signs:** Tool argument extraction returning zero values without error.

### Pitfall 3: PropertyType Custom JSON Marshaling
**What goes wrong:** The Ollama `PropertyType` is `[]string` with custom `MarshalJSON`/`UnmarshalJSON` that outputs a single string when the slice has one element (e.g., `"string"` instead of `["string"]`). If the canonical type doesn't replicate this, tool definitions sent to models may have wrong JSON format.
**Why it happens:** JSON schema expects `"type": "string"` not `"type": ["string"]`.
**How to avoid:** Replicate the `MarshalJSON`/`UnmarshalJSON` on the canonical `PropertyType`. It's 10 lines of code.
**Warning signs:** Models failing to parse tool definitions.

### Pitfall 4: Conversation.AddRawMessage Signature Change
**What goes wrong:** `AddRawMessage(msg api.Message)` changes to `AddRawMessage(msg model.Message)`. The caller in `repl.go` passes `*msg` (dereferenced from `*api.Message` returned by `StreamChat`). After migration, `StreamChat` returns `*model.Message`, so this works -- but verify the dereference still makes sense.
**Why it happens:** Pointer/value semantics can be subtle during type migrations.
**How to avoid:** Verify all call sites of `AddRawMessage` after changing the signature.
**Warning signs:** Compile errors about type mismatch.

### Pitfall 5: ChatService Interface Migration
**What goes wrong:** The `ChatService` interface in `client.go` uses `api.Tools` and returns `*api.Message, *api.Metrics`. Changing this interface signature affects all implementers AND all callers.
**Why it happens:** Interface changes ripple through the dependency graph.
**How to avoid:** Change the interface to use canonical types. Update the `Client` implementation (which does the actual Ollama API calls) to convert at the boundary. Update callers (REPL) to use canonical types.
**Warning signs:** Compilation failures cascading across packages.

### Pitfall 6: Test Mock Types
**What goes wrong:** Test files have mock types (like `mockAPI` in `client_test.go`, `dummyTool` in `registry_test.go`) that implement interfaces using Ollama types. These all need updating.
**Why it happens:** Tests are implementation-heavy; they construct Ollama response objects directly.
**How to avoid:** Update test mocks systematically. The `mockAPI` in `chat/client_test.go` is special -- it mocks the internal `chatAPI` interface which stays Ollama-specific (it's the adapter boundary). Only the `ChatService` interface and its callers change.
**Warning signs:** Test compilation failures.

## Code Examples

### Canonical Message Type
```go
// internal/model/message.go
package model

// Message represents a single message in a conversation.
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content"`
    Thinking   string     `json:"thinking,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}
```

### Canonical ToolDefinition Type
```go
// internal/model/tool.go
package model

import "encoding/json"

// ToolDefinition describes a tool available for model use.
type ToolDefinition struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Parameters  ToolFunctionParameters `json:"parameters"`
}

type ToolFunctionParameters struct {
    Type       string                  `json:"type"`
    Required   []string                `json:"required,omitempty"`
    Properties map[string]ToolProperty `json:"properties"`
}

type ToolProperty struct {
    Type        PropertyType `json:"type,omitempty"`
    Description string       `json:"description,omitempty"`
    Enum        []any        `json:"enum,omitempty"`
}

// PropertyType can be a single string or array of strings in JSON.
// Marshals as "string" for single-element, ["string", "null"] for multiple.
type PropertyType []string

func (pt PropertyType) MarshalJSON() ([]byte, error) {
    if len(pt) == 1 {
        return json.Marshal(pt[0])
    }
    return json.Marshal([]string(pt))
}

func (pt *PropertyType) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err == nil {
        *pt = PropertyType{s}
        return nil
    }
    var ss []string
    if err := json.Unmarshal(data, &ss); err != nil {
        return err
    }
    *pt = ss
    return nil
}
```

### Canonical ToolCall and Arguments
```go
// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
    ID       string           `json:"id,omitempty"`
    Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
    Index     int            `json:"index"`
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}
```

### Canonical StreamMetrics
```go
// internal/model/metrics.go
package model

// StreamMetrics holds token usage metrics from a streaming response.
type StreamMetrics struct {
    PromptEvalCount int `json:"prompt_eval_count,omitempty"`
    EvalCount       int `json:"eval_count,omitempty"`
}
```

### Updated Tool Interface
```go
// internal/tool/registry.go (changed signatures)

type Tool interface {
    Name() string
    Definition() model.ToolDefinition
    Execute(ctx context.Context, args map[string]any) (string, error)
}
```

### Updated ChatService Interface
```go
// internal/chat/client.go (changed signatures)

type ChatService interface {
    ListModels(ctx context.Context) ([]string, error)
    Ping(ctx context.Context) error
    StreamChat(ctx context.Context, conv *Conversation, tools []model.ToolDefinition, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error)
    GetContextLength(ctx context.Context, model string) (int, error)
}
```

### Adapter Conversion Example (in internal/chat)
```go
// internal/chat/stream.go -- conversion at the boundary

func toOllamaMessages(msgs []model.Message) []api.Message {
    out := make([]api.Message, len(msgs))
    for i, m := range msgs {
        out[i] = api.Message{
            Role:       m.Role,
            Content:    m.Content,
            Thinking:   m.Thinking,
            ToolCallID: m.ToolCallID,
        }
        for _, tc := range m.ToolCalls {
            args := api.NewToolCallFunctionArguments()
            for k, v := range tc.Function.Arguments {
                args.Set(k, v)
            }
            out[i].ToolCalls = append(out[i].ToolCalls, api.ToolCall{
                ID: tc.ID,
                Function: api.ToolCallFunction{
                    Index:     tc.Function.Index,
                    Name:      tc.Function.Name,
                    Arguments: args,
                },
            })
        }
    }
    return out
}

func fromOllamaMessage(msg api.Message) model.Message {
    m := model.Message{
        Role:       msg.Role,
        Content:    msg.Content,
        Thinking:   msg.Thinking,
        ToolCallID: msg.ToolCallID,
    }
    for _, tc := range msg.ToolCalls {
        m.ToolCalls = append(m.ToolCalls, model.ToolCall{
            ID: tc.ID,
            Function: model.ToolCallFunction{
                Index:     tc.Function.Index,
                Name:      tc.Function.Name,
                Arguments: tc.Function.Arguments.ToMap(),
            },
        })
    }
    return m
}
```

### Updated Tool Implementation Example
```go
// internal/tool/read.go (after migration)

func (r *ReadFileTool) Definition() model.ToolDefinition {
    return model.ToolDefinition{
        Type: "function",
        Function: model.ToolFunction{
            Name:        "read_file",
            Description: "Read the contents of a file...",
            Parameters: model.ToolFunctionParameters{
                Type:     "object",
                Required: []string{"path"},
                Properties: map[string]model.ToolProperty{
                    "path": {
                        Type:        model.PropertyType{"string"},
                        Description: "Absolute or relative path to the file to read",
                    },
                    "offset": {
                        Type:        model.PropertyType{"integer"},
                        Description: "Start reading from this line number (0-based). Optional.",
                    },
                    "limit": {
                        Type:        model.PropertyType{"integer"},
                        Description: "Maximum number of lines to read. Optional, defaults to 1000.",
                    },
                },
            },
        },
    }
}

func (r *ReadFileTool) Execute(_ context.Context, args map[string]any) (string, error) {
    pathVal, ok := args["path"]
    if !ok {
        return "", fmt.Errorf("missing required argument: path")
    }
    path, ok := pathVal.(string)
    // ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct `api.ToolCallFunctionArguments` (ordered map) | `map[string]any` | This phase | Simpler tool implementations, no ordered map dependency |
| `api.NewToolPropertiesMap()` (ordered map) | `map[string]ToolProperty` literal | This phase | Cleaner tool definitions, no factory function needed |
| `api.Tools` type alias | `[]model.ToolDefinition` slice | This phase | Standard Go slice, no special type |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.9.x |
| Config file | None (Go test conventions) |
| Quick run command | `go test ./internal/model/... ./internal/tool/... ./internal/chat/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements --> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROV-03-a | Canonical types serialize to same JSON as Ollama types | unit | `go test ./internal/model/... -run TestJSON -count=1` | Wave 0 |
| PROV-03-b | Session load/save works with canonical types | unit | `go test ./internal/session/... -count=1` | Existing (needs migration) |
| PROV-03-c | All tool definitions produce valid JSON schema | unit | `go test ./internal/tool/... -count=1` | Existing (needs migration) |
| PROV-03-d | StreamChat returns canonical types correctly | unit | `go test ./internal/chat/... -count=1` | Existing (needs migration) |
| PROV-03-e | Full test suite passes (no regression) | integration | `go test ./... -count=1` | Existing |

### Sampling Rate
- **Per task commit:** `go test ./... -count=1`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/model/message_test.go` -- covers PROV-03-a: JSON round-trip for Message
- [ ] `internal/model/tool_test.go` -- covers PROV-03-a: JSON round-trip for ToolDefinition, PropertyType marshal/unmarshal
- [ ] `internal/model/metrics_test.go` -- covers PROV-03-a: JSON round-trip for StreamMetrics

## Migration Inventory

### Files by Package (import changes needed)

| Package | Files | Source | Test | Nature of Change |
|---------|-------|--------|------|------------------|
| internal/model | 3 | NEW | NEW | Create canonical types |
| internal/chat | 4 | message.go, client.go, stream.go, context.go | client_test.go, stream_test.go, context_test.go | Interface + Conversation migrate to canonical; stream.go adds conversion layer |
| internal/tool | 8 | registry.go, shell.go, read.go, write.go, edit.go, listdir.go, create.go, delete.go, update.go | registry_test.go, shell_test.go, read_test.go, write_test.go, edit_test.go, listdir_test.go, create_test.go | Tool interface + all implementations change to canonical types |
| internal/session | 1 | session.go | session_test.go, store_test.go | Session.Messages type changes |
| internal/lua | 2 | luatool.go, convert.go | luatool_test.go | LuaTool + ArgsToLuaTable change to canonical types |
| internal/repl | 1 | repl.go | repl_test.go | Uses canonical types via chat.ChatService |
| root | 0 | main.go | - | No ollama/api import (uses packages that abstract it) |

### Type Reference Counts (files needing change)
| Ollama Type | Count | Canonical Replacement |
|-------------|-------|-----------------------|
| api.Message | 62 | model.Message |
| api.Tool | 22 | model.ToolDefinition |
| api.ToolCallFunctionArguments | 42 | map[string]any |
| api.ToolCall | 7 | model.ToolCall |
| api.ToolCallFunction | 5 | model.ToolCallFunction |
| api.ToolFunction | 11 | model.ToolFunction |
| api.ToolFunctionParameters | 10 | model.ToolFunctionParameters |
| api.ToolProperty | 14 | model.ToolProperty |
| api.PropertyType | 14 | model.PropertyType |
| api.NewToolPropertiesMap | 9 | (eliminated -- map literal) |
| api.Tools | 6 | []model.ToolDefinition |
| api.Metrics | 5 | model.StreamMetrics |
| api.ChatRequest | 26 | stays in chat (adapter) |
| api.ChatResponse | 25 | stays in chat (adapter) |
| api.ChatResponseFunc | 19 | stays in chat (adapter) |
| api.ListResponse | 12 | stays in chat (adapter) |
| api.ShowRequest/Response | 16 | stays in chat (adapter) |
| api.Client/NewClient/etc | 5 | stays in chat (adapter) |
| api.ThinkValue | 1 | stays in chat (adapter) |
| api.ListModelResponse | 3 | stays in chat (adapter) |

### Boundary Analysis
Types that STAY in internal/chat (Ollama adapter):
- `api.ChatRequest`, `api.ChatResponse`, `api.ChatResponseFunc` -- only used inside stream.go
- `api.Client`, `api.ClientFromEnvironment`, `api.NewClient` -- only used inside client.go
- `api.ListResponse`, `api.ListModelResponse` -- only used inside client.go
- `api.ShowRequest`, `api.ShowResponse` -- only used inside client.go
- `api.ThinkValue` -- only used inside stream.go
- The internal `chatAPI` interface -- stays Ollama-specific

Types that MIGRATE to canonical:
- Everything used by the `ChatService` interface signature (public boundary)
- Everything used by `Tool` interface (tool package public boundary)
- Everything used by `Session` struct (session package)
- Everything used by `Conversation` struct (chat package, but public)

## Open Questions

1. **getOptionalInt helper function**
   - What we know: `getOptionalInt` in `tool/read.go` takes `api.ToolCallFunctionArguments` as parameter. With `map[string]any`, the signature changes to `getOptionalInt(args map[string]any, key string, defaultVal int)`.
   - What's unclear: Whether to keep as a standalone function or move to a shared util.
   - Recommendation: Keep in `tool/read.go` as-is, just change the argument type. It's only used by read.go.

2. **successJSON helper in tool/create.go**
   - What we know: `successJSON` takes a `Tool` interface, calls `Definition()`, then accesses `def.Function.Parameters.Properties.All()` to iterate property names.
   - What's unclear: With `map[string]ToolProperty` replacing `ToolPropertiesMap`, iteration changes from `props.All()` to `for name := range props`.
   - Recommendation: Simple change -- iterate the map directly.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: All 30+ Go files read and analyzed for Ollama API type usage
- `go doc github.com/ollama/ollama/api` -- verified all type shapes: Message, Tool, ToolCall, ToolCallFunctionArguments, Metrics, PropertyType, ToolPropertiesMap
- Existing test suite: all 7 packages pass (verified via `go test ./...`)

### Secondary (MEDIUM confidence)
- JSON backward compatibility analysis: based on JSON tag comparison between `api.Message` and proposed `model.Message` -- high confidence but should be validated with a round-trip test

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies, pure Go types
- Architecture: HIGH - straightforward type extraction pattern, well-understood boundary
- Pitfalls: HIGH - all identified from direct codebase analysis, not speculation

**Research date:** 2026-04-12
**Valid until:** Indefinite (internal refactoring, no external API changes affect this)
