---
phase: quick
plan: 260412-gan
type: execute
wave: 1
depends_on: []
files_modified:
  - main.go
  - go.mod
  - go.sum
autonomous: true
must_haves:
  truths:
    - "All flags use double-dash convention (--debug, --host, --pipe, --yolo, --line-by-line, --version)"
    - "Frequent flags have short forms: -d (debug), -y (yolo), -p (pipe)"
    - "--version / -v prints version and exits"
    - "Custom help output shows usage examples for interactive, pipe, and yolo modes"
    - "Help includes flag descriptions with short forms"
  artifacts:
    - path: "main.go"
      provides: "pflag-based flag parsing with custom help and version flag"
  key_links:
    - from: "main.go"
      to: "github.com/spf13/pflag"
      via: "import and flag definitions"
      pattern: "pflag\\."
---

<objective>
Switch CLI flag handling from stdlib `flag` to `pflag` for double-dash conventions, add custom help output with usage examples, and add a --version flag.

Purpose: Align CLI with standard Unix conventions (double-dash flags), provide helpful usage output for new users, and expose version information.
Output: Updated main.go with pflag integration, custom usage function, and --version flag.
</objective>

<execution_context>
@~/.claude/get-shit-done/workflows/execute-plan.md
@~/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@main.go
@internal/config/config.go
@internal/render/style.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add pflag dependency and switch all flag parsing</name>
  <files>main.go, go.mod, go.sum</files>
  <action>
1. Run `go get github.com/spf13/pflag` to add the dependency.

2. In main.go, replace `"flag"` import with `pflag "github.com/spf13/pflag"`.

3. Replace all flag definitions. Use pflag methods which support short forms:
   - `host := pflag.StringP("host", "H", "", "Ollama server address (default: localhost:11434)")` — use -H for host since pflag reserves -h for help
   - `pipeMode := pflag.BoolP("pipe", "p", false, "Read all stdin as a single message and send to model")`
   - `debugMode := pflag.BoolP("debug", "d", false, "Show tool call results and other debug output")`
   - `yoloMode := pflag.BoolP("yolo", "y", false, "Auto-approve all dangerous commands (use with caution)")`
   - `lineByLine := pflag.Bool("line-by-line", false, "In pipe mode, send each stdin line separately (default: batch)")` — no short form, less frequent flag
   - `showVersion := pflag.BoolP("version", "v", false, "Print version and exit")`

4. Replace `flag.Parse()` with `pflag.Parse()`.

5. Add version handling right after pflag.Parse():
   ```go
   if *showVersion {
       fmt.Printf("fenec %s\n", config.Version)
       os.Exit(0)
   }
   ```

6. Add custom usage function BEFORE pflag.Parse() call. Set `pflag.Usage` to a function that prints:
   ```
   fenec - AI assistant powered by local Ollama models

   Usage:
     fenec                    Start interactive chat
     echo "prompt" | fenec    Send piped input to model
     fenec --yolo             Auto-approve all tool commands

   Flags:
   ```
   Then call `pflag.PrintDefaults()` to print the flag table.

7. Run `go mod tidy` to clean up go.sum.
  </action>
  <verify>
    <automated>cd /home/marad/dev/fenec && go build -o /dev/null . && echo "Build OK"</automated>
  </verify>
  <done>
    - main.go uses pflag instead of stdlib flag
    - All flags use double-dash convention with appropriate short forms (-d, -y, -p, -H, -v)
    - `--version` / `-v` prints "fenec v0.1" and exits
    - `--help` / `-h` shows custom usage with examples and flag table
    - Project builds cleanly with no errors
  </done>
</task>

<task type="auto">
  <name>Task 2: Verify flag behavior end-to-end</name>
  <files>main.go</files>
  <action>
1. Run `go build -o $TMPDIR/fenec .` to produce a test binary.

2. Test --version: run `$TMPDIR/fenec --version` and `$TMPDIR/fenec -v` — both must print "fenec v0.1" and exit 0.

3. Test --help: run `$TMPDIR/fenec --help` — must show:
   - Header line with "fenec" and description
   - Usage examples section (interactive, pipe, yolo)
   - Flag table with descriptions and short forms

4. Test -h: run `$TMPDIR/fenec -h` — must show same help output as --help (pflag default behavior).

5. Test that unknown flags produce an error: run `$TMPDIR/fenec --bogus 2>&1; echo "exit: $?"` — should show error + usage and exit non-zero.

6. Clean up: `rm -f $TMPDIR/fenec`.

If any test fails, fix the issue in main.go and re-verify.
  </action>
  <verify>
    <automated>cd /home/marad/dev/fenec && go build -o $TMPDIR/fenec . && $TMPDIR/fenec --version && $TMPDIR/fenec --help > /dev/null && rm -f $TMPDIR/fenec && echo "All checks passed"</automated>
  </verify>
  <done>
    - `fenec --version` and `fenec -v` both print version and exit 0
    - `fenec --help` and `fenec -h` both show custom help with examples
    - Unknown flags produce error message and non-zero exit
  </done>
</task>

</tasks>

<verification>
- `go build .` succeeds
- `fenec --version` prints "fenec v0.1"
- `fenec --help` shows custom usage with examples and all flags with short forms
- `fenec -d`, `fenec -y`, `fenec -p` work as short forms for --debug, --yolo, --pipe
- Existing behavior unchanged: pipe auto-detection, yolo mode, debug mode all work as before
</verification>

<success_criteria>
All CLI flags use double-dash convention via pflag. Short forms -d, -y, -p, -H, -v work. Custom help shows usage examples. --version prints version and exits.
</success_criteria>

<output>
After completion, create `.planning/quick/260412-gan-improve-cli-help-output-and-flag-handlin/260412-gan-SUMMARY.md`
</output>
