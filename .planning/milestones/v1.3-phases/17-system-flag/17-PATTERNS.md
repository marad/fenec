# Phase 17: System Flag - Pattern Map

**Mapped:** 2025-07-18
**Files analyzed:** 1 modified file (main.go), 0 new files
**Analogs found:** 1 / 1 (self-analog — the file being modified IS the pattern source)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `main.go` | CLI entry point | request-response (flag → file-I/O → pass-through) | `main.go` (self — existing flag + system prompt patterns) | exact |

## Pattern Assignments

### `main.go` (CLI entry point, flag → file-I/O → pass-through)

**Analog:** `main.go` itself — this is a modification, not a new file. All patterns come from the existing code in the same file.

**Imports pattern** (lines 1-23) — no new imports needed beyond `os` (already imported):
```go
import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	pflag "github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	feneclua "github.com/marad/fenec/internal/lua"
	"github.com/marad/fenec/internal/provider"
	"github.com/marad/fenec/internal/render"
	"github.com/marad/fenec/internal/repl"
	"github.com/marad/fenec/internal/session"
	"github.com/marad/fenec/internal/tool"
)
```
Note: `os.ReadFile` is in stdlib `os` package, already imported. No new imports required.

---

**Flag definition pattern** (lines 27-32) — all flags use `pflag.XxxP()` at top of `main()`:
```go
modelName := pflag.StringP("model", "m", "", "Model to use (provider/model or just model name)")
pipeMode := pflag.BoolP("pipe", "p", false, "Read all stdin as a single message and send to model")
debugMode := pflag.BoolP("debug", "d", false, "Show tool call results and other debug output")
yoloMode := pflag.BoolP("yolo", "y", false, "Auto-approve all dangerous commands (use with caution)")
lineByLine := pflag.Bool("line-by-line", false, "In pipe mode, send each stdin line separately (default: batch)")
showVersion := pflag.BoolP("version", "v", false, "Print version and exit")
```
**Copy this pattern:** Add `systemFile := pflag.StringP("system", "s", "", "...")` in this block (after line 32).

---

**Usage/help text pattern** (lines 34-46) — hand-crafted examples followed by `pflag.PrintDefaults()`:
```go
pflag.Usage = func() {
	fmt.Fprintf(os.Stderr, `fenec - AI assistant powered by local Ollama models

Usage:
  fenec                    Start interactive chat
  fenec --model gemma4     Use a specific model
  echo "prompt" | fenec    Send piped input to model
  fenec --yolo             Auto-approve all tool commands

Flags:
`)
	pflag.PrintDefaults()
}
```
**Copy this pattern:** Add a `--system` example line (e.g., `fenec --system prompt.md  Use a custom system prompt`) in the Usage block.

---

**System prompt loading — the block to replace** (lines 174-180):
```go
// Load system prompt (per D-15).
systemPrompt, err := config.LoadSystemPrompt()
if err != nil {
	fmt.Fprintln(os.Stderr, render.FormatError(
		fmt.Sprintf("Failed to load system prompt: %v", err)))
	os.Exit(1)
}
```
**Replace with:** Conditional branch — if `*systemFile != ""`, use `os.ReadFile(*systemFile)` with hard-fail error handling; else fall through to `config.LoadSystemPrompt()`.

---

**Error handling pattern** (lines 70-73, 77-80, 124-127, 157-159, 167-169, 177-179) — consistent throughout `main()`:
```go
fmt.Fprintln(os.Stderr, render.FormatError(
	fmt.Sprintf("Failed to <description>: %v", err)))
os.Exit(1)
```
**Copy this exact pattern** for the `--system` file read error case. Use `render.FormatError` + `fmt.Sprintf` + `os.Stderr` + `os.Exit(1)`.

---

**REPL integration point** (line 291) — `systemPrompt` is passed as a string parameter, no changes needed:
```go
r, err := repl.NewREPL(p, defaultModel, activeProviderName, systemPrompt, tracker, store, toolRegistry, providerRegistry)
```
The REPL stores this as `baseSystemPrompt` and appends tool descriptions (lines 64-74 of `repl.go`). **No REPL changes needed** — the `systemPrompt` variable just needs to contain the right content before this call.

---

**`config.LoadSystemPrompt()` reference** (`internal/config/config.go` lines 119-134) — the function being conditionally bypassed:
```go
func LoadSystemPrompt() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return defaultSystemPrompt, nil
	}

	path := filepath.Join(dir, "system.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSystemPrompt, nil
		}
		return "", err
	}
	return string(data), nil
}
```
**Key difference:** `LoadSystemPrompt()` falls back to `defaultSystemPrompt` on missing file. The `--system` flag path must NOT fall back — it must hard-fail per D-01. Use bare `os.ReadFile` without fallback logic.

## Shared Patterns

### Error Handling (fatal exit)
**Source:** `main.go` lines 70-73 (representative of 6+ identical usages)
**Apply to:** The `--system` file read error branch
```go
fmt.Fprintln(os.Stderr, render.FormatError(
	fmt.Sprintf("Failed to <description>: %v", err)))
os.Exit(1)
```

### Flag Definition Convention
**Source:** `main.go` lines 27-32
**Apply to:** The new `--system` / `-s` flag
- Use `pflag.StringP` for flags with short aliases
- Return pointer, dereference with `*` when reading
- Place in the flag definition block before `pflag.Parse()`

### `os.ReadFile` for file content
**Source:** `internal/config/config.go` line 126
**Apply to:** Reading the `--system` file
```go
data, err := os.ReadFile(path)
// ...
return string(data), nil
```
Same `os.ReadFile` → `string(data)` pattern used by `LoadSystemPrompt()`. The only difference is error handling (hard fail vs fallback).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| — | — | — | All changes are to `main.go` which is its own analog. No new files needed. |

## Testing Patterns

### Existing config test patterns (`internal/config/config_test.go`)
**Source:** `internal/config/config_test.go` lines 1-35
```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSystemPromptDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	prompt, err := LoadSystemPrompt()
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "Fenec")
}

func TestLoadSystemPromptFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	fenecDir := filepath.Join(tmpDir, ".config", "fenec")
	require.NoError(t, os.MkdirAll(fenecDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(fenecDir, "system.md"), []byte("Custom prompt"), 0644))

	prompt, err := LoadSystemPrompt()
	assert.NoError(t, err)
	assert.Equal(t, "Custom prompt", prompt)
}
```
**Apply if:** A helper function is created in `config` package (agent's discretion). If implementation stays inline in `main.go`, primary verification is integration testing (`fenec --system <file>`).

**Test conventions:**
- `t.TempDir()` for temp directories (auto-cleaned)
- `t.Setenv("HOME", tmpDir)` to isolate config path lookups
- `require.NoError` for precondition setup, `assert.NoError` / `assert.Equal` for assertions
- `testify/assert` + `testify/require` — never raw `if err != nil { t.Fatal(err) }`

## Metadata

**Analog search scope:** `main.go`, `internal/config/config.go`, `internal/repl/repl.go`, `internal/config/config_test.go`
**Files scanned:** 4 primary source files + test file
**Pattern extraction date:** 2025-07-18
