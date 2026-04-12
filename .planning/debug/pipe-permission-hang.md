---
status: awaiting_human_verify
trigger: "pipe-permission-hang: When piping input to fenec, dangerous commands hang because permission prompt reads from pipe stdin"
created: 2026-04-12T00:00:00Z
updated: 2026-04-12T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED - REPL.ApproveCommand (repl.go:403) calls r.rl.Readline() which reads from stdin. In pipe mode, stdin is the pipe (not a terminal), so after the pipe content is consumed, Readline blocks forever waiting for more input that never arrives.
test: Code traced from main.go through to ApproveCommand
expecting: N/A - root cause confirmed
next_action: Implement fix: detect non-interactive stdin, add --yolo flag, modify approval flow

## Symptoms

expected: When running fenec in pipe mode, dangerous commands should either be auto-denied (with a clear message) or auto-approved if --yolo flag is set, without hanging.
actual: The program hangs when a dangerous command needs permission approval, because the permission prompt tries to read from stdin which is a pipe, not a terminal.
errors: No error messages — it just hangs/blocks waiting for input that never comes through the pipe.
reproduction: `echo "execute git add ." | fenec` — when the agent decides to run `git add .` which requires permission, the prompt blocks.
started: Design gap — permission system built for interactive terminal use only.

## Eliminated

## Evidence

- timestamp: 2026-04-12T00:01:00Z
  checked: main.go approval wiring
  found: Lines 91-98 create a closure that delegates to `r.ApproveCommand`. This is passed to ShellTool, WriteTool, EditTool. The `approver` variable is set on line 181 to `r.ApproveCommand`.
  implication: All dangerous command approval goes through REPL.ApproveCommand

- timestamp: 2026-04-12T00:01:30Z
  checked: REPL.ApproveCommand (repl.go:403-418)
  found: Uses r.rl.Readline() to read user input. In pipe mode, readline's stdin IS the pipe. After pipe content is exhausted (EOF), Readline may block or return EOF. The key issue: readline is configured with os.Stdin by default, so in pipe mode it reads from the pipe.
  implication: This is the root cause. The permission prompt hangs because readline tries to read from the pipe stdin.

- timestamp: 2026-04-12T00:02:00Z
  checked: main.go pipe mode (lines 186-192)
  found: Pipe mode is explicitly triggered with --pipe flag. RunPipe reads from os.Stdin. But the approval function still uses readline which also reads from stdin (the pipe).
  implication: Even in explicit pipe mode, approval hangs. Also, users might pipe input WITHOUT --pipe flag (e.g., `echo "hello" | fenec`) which would hit the interactive REPL but with a non-terminal stdin.

- timestamp: 2026-04-12T00:02:30Z
  checked: go.mod dependencies
  found: golang.org/x/term v0.42.0 already present as indirect dep. Can use term.IsTerminal(int(os.Stdin.Fd())) to detect non-interactive mode.
  implication: No new dependencies needed

## Resolution

root_cause: REPL.ApproveCommand (repl.go:403) uses r.rl.Readline() to prompt for Y/n approval. When stdin is a pipe, readline reads from the pipe which has already been consumed or will never provide the expected input, causing the program to hang indefinitely. Additionally, the program required --pipe flag to enter pipe mode; piping without the flag would attempt interactive REPL mode on a non-terminal stdin.
fix: |
  1. Added --yolo flag to auto-approve all dangerous commands
  2. Added terminal detection via golang.org/x/term.IsTerminal() to detect non-interactive stdin
  3. Auto-enable pipe mode when stdin is not a terminal (no need for --pipe flag)
  4. Three approval modes wired in main.go:
     - --yolo: auto-approve with stderr log message
     - Non-interactive (no --yolo): auto-deny with clear message directing user to --yolo
     - Interactive: delegate to REPL.ApproveCommand as before
verification: All 118 tests pass (112 existing + 6 new). New tests cover yolo approval, non-interactive denial, safe command passthrough, write tool integration with both modes, and nil-approver fallback.
files_changed:
  - main.go
  - go.mod
  - go.sum
  - internal/tool/approval_test.go
