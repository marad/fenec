# Phase 15: Clear Command - Pattern Map

**Mapped:** 2026-04-15
**Files analyzed:** 5 (modified)
**Analogs found:** 5 / 5

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------|------|-----------|----------------|---------------|
| `internal/repl/repl.go` (add `handleClearCommand`) | controller | request-response | `internal/repl/repl.go` `handleSaveCommand()` (line 649) | exact |
| `internal/repl/repl.go` (add `/clear` to switch) | route | request-response | `internal/repl/repl.go` switch block (line 153) | exact |
| `internal/repl/commands.go` (add `/clear` to helpText) | config | — | `internal/repl/commands.go` `helpText` (line 36) | exact |
| `internal/chat/context.go` (add `Reset()` method) | service | transform | `internal/chat/context.go` `Update()` (line 21) | exact |
| `internal/repl/repl_test.go` (add clear tests) | test | — | `internal/repl/repl_test.go` existing tests (lines 232-389) | exact |
| `internal/chat/context_test.go` (add Reset test) | test | — | `internal/chat/context_test.go` existing tests (lines 17-21) | exact |

## Pattern Assignments

### `internal/repl/repl.go` — `handleClearCommand()` (controller, request-response)

**Analog:** `handleSaveCommand()` at lines 649–664

This is the primary analog. `handleClearCommand` follows the same structure: guard on `r.store`, sync conv→session, call `Store.Save()`, print feedback. The clear handler extends this with reset steps afterward.

**Save/sync pattern** (lines 649–664):
```go
// handleSaveCommand persists the current conversation to a named session file.
func (r *REPL) handleSaveCommand() {
	if r.store == nil {
		fmt.Fprintln(r.rl.Stdout(), "Session storage not available.")
		return
	}
	r.session.Messages = r.conv.Messages
	r.session.UpdatedAt = time.Now()
	if r.tracker != nil {
		r.session.TokenCount = r.tracker.TokenUsage()
	}
	if err := r.store.Save(r.session); err != nil {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Save failed: %v", err)))
		return
	}
	fmt.Fprintf(r.rl.Stdout(), "Session saved: %s (%d messages)\n", r.session.ID, len(r.session.Messages))
}
```

**Session+Conversation creation pattern** — from `NewREPL` (lines 64–83):
```go
	// Append tool descriptions to system prompt so the model knows what tools are available.
	if toolRegistry != nil {
		toolDesc := toolRegistry.Describe()
		if toolDesc != "" {
			systemPrompt = systemPrompt + "\n\n## Available Tools\n\n" + toolDesc
		}
	}

	conv := chat.NewConversation(model, systemPrompt)

	// Set context length from tracker if available.
	if tracker != nil {
		conv.ContextLength = tracker.Available()
	}

	sess := session.NewSession(model)
```

**refreshSystemPrompt pattern** (lines 779–790) — for rebuilding system prompt with tools:
```go
func (r *REPL) refreshSystemPrompt() {
	if len(r.conv.Messages) > 0 && r.conv.Messages[0].Role == "system" {
		prompt := r.baseSystemPrompt
		if r.registry != nil {
			toolDesc := r.registry.Describe()
			if toolDesc != "" {
				prompt = prompt + "\n\n## Available Tools\n\n" + toolDesc
			}
		}
		r.conv.Messages[0].Content = prompt
	}
}
```

**EnableThink pattern** (lines 800–802) — must preserve Think flag across clear:
```go
func (r *REPL) EnableThink() {
	r.conv.Think = true
}
```

**autoSave sync.Once pattern** (lines 630–646) — the `sync.Once` that needs resetting:
```go
func (r *REPL) autoSave() {
	r.autoSaved.Do(func() {
		if r.store == nil || r.session == nil {
			return
		}
		// Sync conversation messages to session.
		r.session.Messages = r.conv.Messages
		r.session.UpdatedAt = time.Now()
		if r.tracker != nil {
			r.session.TokenCount = r.tracker.TokenUsage()
		}
		if err := r.store.AutoSave(r.session); err != nil {
			// Best effort -- log but don't fail exit.
			fmt.Fprintf(os.Stderr, "auto-save failed: %v\n", err)
		}
	})
}
```

**Error output pattern** — consistent across all handlers:
```go
fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Save failed: %v", err)))
```

**User feedback pattern** — from handleSaveCommand (line 663) and handleLoadCommand (lines 740–741):
```go
// handleSaveCommand feedback:
fmt.Fprintf(r.rl.Stdout(), "Session saved: %s (%d messages)\n", r.session.ID, len(r.session.Messages))

// handleLoadCommand feedback:
fmt.Fprintf(r.rl.Stdout(), "Loaded session %s (%d messages, model: %s)\n",
    loaded.ID, len(loaded.Messages), loaded.Model)
```

---

### `internal/repl/repl.go` — Switch statement routing (route, request-response)

**Analog:** Run() switch at lines 153–171

**Command routing pattern** (lines 153–171):
```go
			switch cmd.Name {
			case "/quit":
				return nil
			case "/help":
				fmt.Fprintln(r.rl.Stdout(), helpText)
			case "/model":
				r.handleModelCommand(cmd.Args)
			case "/save":
				r.handleSaveCommand()
			case "/load":
				r.handleLoadCommand()
			case "/history":
				r.handleHistoryCommand()
			case "/tools":
				r.handleToolsCommand()
			default:
				fmt.Fprintf(r.rl.Stdout(), "Unknown command: %s. Type /help for available commands.\n", cmd.Name)
			}
```

Add `/clear` case following the same single-line dispatch pattern as `/save`:
```go
			case "/clear":
				r.handleClearCommand()
```

---

### `internal/repl/commands.go` — helpText update (config)

**Analog:** `helpText` constant at lines 36–52

**Help text pattern** (lines 36–52):
```go
const helpText = `Available commands:
  /help    - Show this help message
  /model              - List models or switch: /model [provider/]name
  /save    - Save current conversation to disk
  /load    - List and load a saved conversation
  /history - Show conversation stats (messages, tokens)
  /tools   - List all loaded tools with provenance
  /quit    - Exit fenec

Shortcuts:
  Ctrl+C  - Cancel active generation, or clear current input
  Ctrl+D  - Exit fenec
  \       - Continue input on next line (at end of line)

Tools:
  The agent can use tools to execute actions. Dangerous commands
  (rm, sudo, chmod, etc.) will prompt for your approval.`
```

Insert `/clear` line after `/history` and before `/tools`, following the same alignment convention:
```
  /clear   - Save and reset conversation (start fresh)
```

---

### `internal/chat/context.go` — `Reset()` method (service, transform)

**Analog:** `Update()` method at lines 21–24

**Existing method pattern** (lines 21–24):
```go
// Update records the latest token counts from Ollama Metrics.
func (ct *ContextTracker) Update(promptEvalCount, evalCount int) {
	ct.lastPromptEval = promptEvalCount
	ct.lastEval = evalCount
}
```

`Reset()` follows the same pattern — a method on `*ContextTracker` that sets the two counter fields. Place it directly after `Update()`:
```go
// Reset zeroes the token counters so a fresh conversation starts clean.
func (ct *ContextTracker) Reset() {
	ct.lastPromptEval = 0
	ct.lastEval = 0
}
```

---

### `internal/repl/repl_test.go` — Clear command tests (test)

**Analog:** Existing test patterns in the same file

**Test imports pattern** (lines 1–22):
```go
package repl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chzyer/readline"
	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	"github.com/marad/fenec/internal/model"
	prov "github.com/marad/fenec/internal/provider"
	"github.com/marad/fenec/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**newTestREPL helper** (lines 232–265) — builds minimal REPL with bytes.Buffer output capture. Clear tests will need to extend this to include `store`, `session`, `tracker`, and `baseSystemPrompt` fields:
```go
func newTestREPL(
	t *testing.T,
	p prov.Provider,
	registry *config.ProviderRegistry,
	activeProvider, activeModel string,
) (*REPL, *bytes.Buffer) {
	t.Helper()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { stdinR.Close(); stdinW.Close() })

	var buf bytes.Buffer
	rl, rlErr := readline.NewEx(&readline.Config{
		Stdin:  stdinR,
		Stdout: &buf,
		Stderr: &buf,
	})
	if rlErr != nil {
		t.Skipf("readline init failed (no TTY?): %v", rlErr)
	}
	t.Cleanup(func() { rl.Close() })

	r := &REPL{
		provider:         p,
		providerRegistry: registry,
		activeProvider:   activeProvider,
		conv:             chat.NewConversation(activeModel, ""),
		rl:               rl,
	}
	return r, &buf
}
```

**Test assertion style** — uses `assert`/`require` from testify, never `suite`. Examples:
```go
// Simple assertion (line 129):
assert.Contains(t, helpText, "/save")

// Test with REPL setup (lines 277–296):
func TestHandleModelCommandProviderModel(t *testing.T) {
	ollamaMock := &mockProvider{name: "ollama", models: []string{"gemma4"}}
	openaiMock := &mockProvider{name: "openai", models: []string{"gpt-4"}}

	registry := config.NewProviderRegistry()
	registry.Register("ollama", ollamaMock)
	registry.Register("openai", openaiMock)

	r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")

	r.handleModelCommand([]string{"openai/gpt-4"})

	output := buf.String()
	assert.Equal(t, "openai", r.activeProvider, ...)
	assert.Contains(t, output, "Switched to openai/gpt-4", ...)
}
```

**Session store test setup** — from TestAutoSaveCalledOnce (lines 134–136):
```go
	dir := t.TempDir()
	store := session.NewStore(dir)
```

---

### `internal/chat/context_test.go` — Reset test (test)

**Analog:** Existing tests in the same file

**Test pattern** (lines 17–21):
```go
func TestContextTrackerUpdateSetsTokenCounts(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	ct.Update(500, 100)
	assert.Equal(t, 600, ct.TokenUsage())
}
```

The Reset test follows the same pattern: create tracker, set some values, call Reset, assert zeroed:
```go
func TestContextTrackerResetZeroesCounters(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	ct.Update(500, 100)
	ct.Reset()
	assert.Equal(t, 0, ct.TokenUsage())
}
```

---

## Shared Patterns

### Output Writing
**Source:** All REPL handlers (`repl.go`)
**Apply to:** `handleClearCommand()`
```go
// Standard output: use r.rl.Stdout() — never os.Stdout directly in handlers
fmt.Fprintln(r.rl.Stdout(), "message")
fmt.Fprintf(r.rl.Stdout(), "template: %s\n", value)
```

### Error Formatting
**Source:** `internal/repl/repl.go` lines 660, 675, 730
**Apply to:** `handleClearCommand()` save error path
```go
fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Save failed: %v", err)))
```

### Nil Guard on Optional Fields
**Source:** `internal/repl/repl.go` — used consistently across handlers
**Apply to:** `handleClearCommand()` for `r.store`, `r.session`, `r.tracker`, `r.registry`
```go
// Guard pattern for store (handleSaveCommand line 650-652):
if r.store == nil {
    fmt.Fprintln(r.rl.Stdout(), "Session storage not available.")
    return
}

// Guard pattern for tracker (handleSaveCommand line 657-658):
if r.tracker != nil {
    r.session.TokenCount = r.tracker.TokenUsage()
}

// Guard pattern for registry (refreshSystemPrompt line 782-787):
if r.registry != nil {
    toolDesc := r.registry.Describe()
    if toolDesc != "" {
        prompt = prompt + "\n\n## Available Tools\n\n" + toolDesc
    }
}
```

### Conversation ↔ Session Sync
**Source:** `internal/repl/repl.go` lines 636–639 (autoSave) and lines 654–658 (handleSaveCommand)
**Apply to:** `handleClearCommand()` save step
```go
r.session.Messages = r.conv.Messages
r.session.UpdatedAt = time.Now()
if r.tracker != nil {
    r.session.TokenCount = r.tracker.TokenUsage()
}
```

### sync.Once Reset by Value Replacement
**Source:** REPL struct definition (line 40: `autoSaved sync.Once`)
**Apply to:** `handleClearCommand()` after session reset
```go
// sync.Once has no Reset() — replace with zero value to re-arm
r.autoSaved = sync.Once{}
```

### Test Helper Extension
**Source:** `newTestREPL()` at repl_test.go lines 232–265
**Apply to:** Clear command tests need store, session, tracker, baseSystemPrompt
```go
// Extend the returned REPL with additional fields after creation:
r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")
dir := t.TempDir()
r.store = session.NewStore(dir)
r.session = session.NewSession("gemma4")
r.tracker = chat.NewContextTracker(8192, 0.85)
r.baseSystemPrompt = "test system prompt"
// Then populate conversation with messages for content-check tests
```

## No Analog Found

No files in this phase lack a close analog. All changes are modifications to existing files with well-established patterns already in the codebase.

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| — | — | — | All files have exact analogs |

## Metadata

**Analog search scope:** `internal/repl/`, `internal/chat/`, `internal/session/`
**Files scanned:** 8 (repl.go, commands.go, repl_test.go, context.go, context_test.go, message.go, session.go, store.go)
**Pattern extraction date:** 2026-04-15
