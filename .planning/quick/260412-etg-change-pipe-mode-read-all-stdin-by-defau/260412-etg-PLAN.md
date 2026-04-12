---
phase: quick
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - main.go
  - internal/repl/repl.go
  - internal/repl/repl_test.go
autonomous: true
requirements: []
must_haves:
  truths:
    - "Piped stdin is read in full and sent as a single message by default"
    - "The --line-by-line flag restores old per-line behavior"
    - "Empty stdin produces no messages in either mode"
    - "Slash commands in line-by-line mode still work"
  artifacts:
    - path: "main.go"
      provides: "--line-by-line CLI flag wired to RunPipe"
      contains: "lineByLine"
    - path: "internal/repl/repl.go"
      provides: "RunPipe with batch-default and line-by-line option"
      contains: "ReadAll"
  key_links:
    - from: "main.go"
      to: "internal/repl/repl.go"
      via: "RunPipe(os.Stdin, *lineByLine)"
      pattern: "RunPipe.*lineByLine"
---

<objective>
Change pipe mode so that stdin is read entirely and sent as a single message by default. Add a --line-by-line flag to preserve the old per-line behavior.

Purpose: When piping multi-line content (e.g., a file) into fenec, the model should receive the full context at once rather than getting fragmented line-by-line messages. The old behavior remains available behind a flag for cases where per-line processing is desired.

Output: Updated RunPipe, new CLI flag, tests for both modes.
</objective>

<execution_context>
@~/.claude/get-shit-done/workflows/execute-plan.md
@~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@main.go
@internal/repl/repl.go
@internal/repl/repl_test.go

<interfaces>
From internal/repl/repl.go (current RunPipe signature):
```go
func (r *REPL) RunPipe(input io.Reader) error
```

From main.go (current call site):
```go
if err := r.RunPipe(os.Stdin); err != nil {
```

Key REPL methods used in RunPipe:
```go
r.rl.Stdout() io.Writer        // output destination
r.sendMessage(input string)     // sends user input to model
r.handleToolsCommand()          // /tools handler
r.handleHistoryCommand()        // /history handler
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Update RunPipe to support batch and line-by-line modes</name>
  <files>internal/repl/repl.go</files>
  <action>
Change the RunPipe method signature from `RunPipe(input io.Reader) error` to `RunPipe(input io.Reader, lineByLine bool) error`.

When `lineByLine` is false (the new default):
1. Read ALL of stdin using `io.ReadAll(input)` into a single string.
2. Trim whitespace from the full content.
3. If the result is empty, return nil (no message to send).
4. Print `> ` followed by a truncated preview of the input (first 100 chars, with "..." if truncated) to show the user what was received.
5. Call `r.sendMessage(fullContent)` once.
6. Print a trailing newline.
7. Return nil.

When `lineByLine` is true, keep the EXACT existing scanner-based behavior (the current implementation of RunPipe) — iterate lines, skip blanks, handle /quit, handle slash commands, echo each line with `> `, call sendMessage per line.

The io import is already present. Add "io" usage for io.ReadAll (it is already imported at the top of the file).
  </action>
  <verify>
    <automated>cd /home/marad/dev/fenec && go build ./...</automated>
  </verify>
  <done>RunPipe accepts lineByLine bool parameter. Default path (lineByLine=false) reads all stdin at once. Old path (lineByLine=true) processes line-by-line.</done>
</task>

<task type="auto">
  <name>Task 2: Add --line-by-line flag and wire to RunPipe</name>
  <files>main.go</files>
  <action>
In main.go's flag definitions (after the existing `yoloMode` flag), add:

```go
lineByLine := flag.Bool("line-by-line", false, "In pipe mode, send each stdin line as a separate message (default: send all stdin as one message)")
```

Update the `--pipe` flag's description to reflect the new default behavior:

```go
pipeMode := flag.Bool("pipe", false, "Read all stdin as a single message and send to model, exit on EOF")
```

Update the RunPipe call site (around line 213) to pass the flag value:

```go
if err := r.RunPipe(os.Stdin, *lineByLine); err != nil {
```
  </action>
  <verify>
    <automated>cd /home/marad/dev/fenec && go build -o /dev/null . && echo "hello world" | go run . --help 2>&1 | grep -q "line-by-line" && echo "OK"</automated>
  </verify>
  <done>--line-by-line flag exists in CLI help. RunPipe call passes the flag value. Project compiles cleanly.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Add tests for both RunPipe modes</name>
  <files>internal/repl/repl_test.go</files>
  <behavior>
    - Test: readAllInput helper reads full io.Reader content as single trimmed string
    - Test: readAllInput returns empty string for empty/whitespace-only input
    - Test: readAllInput preserves internal newlines (multi-line content stays multi-line)
  </behavior>
  <action>
Since RunPipe depends on the full REPL struct (readline, sendMessage, etc.), testing it end-to-end in a unit test is impractical. Instead, extract the stdin-reading logic for the batch mode into a package-level helper function in repl.go:

```go
// readAllInput reads the entire content of r as a single string, trimming
// leading/trailing whitespace. Returns empty string if input is empty or
// whitespace-only.
func readAllInput(r io.Reader) (string, error) {
    data, err := io.ReadAll(r)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(data)), nil
}
```

Then use this helper in RunPipe's batch path:
```go
content, err := readAllInput(input)
if err != nil { return fmt.Errorf("reading stdin: %w", err) }
if content == "" { return nil }
```

In repl_test.go, add tests:

```go
func TestReadAllInput(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"single line", "hello world\n", "hello world"},
        {"multi-line", "line one\nline two\nline three\n", "line one\nline two\nline three"},
        {"empty", "", ""},
        {"whitespace only", "  \n  \n  ", ""},
        {"preserves internal spacing", "  hello\n  world  ", "hello\n  world"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := readAllInput(strings.NewReader(tt.input))
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

Add `"strings"` to the test file imports if not already present.
  </action>
  <verify>
    <automated>cd /home/marad/dev/fenec && go test ./internal/repl/ -run TestReadAllInput -v</automated>
  </verify>
  <done>readAllInput helper is tested for single-line, multi-line, empty, and whitespace-only inputs. All tests pass.</done>
</task>

</tasks>

<verification>
- `go build ./...` compiles without errors
- `go test ./internal/repl/ -v` passes all tests including new TestReadAllInput
- `echo "hello" | go run . --help` shows --line-by-line in usage
- `go vet ./...` reports no issues
</verification>

<success_criteria>
- Piping content into fenec reads all stdin as one message by default
- `--line-by-line` flag restores old per-line behavior
- readAllInput helper is unit tested
- All existing tests continue to pass
</success_criteria>

<output>
After completion, create `.planning/quick/260412-etg-change-pipe-mode-read-all-stdin-by-defau/260412-etg-SUMMARY.md`
</output>
