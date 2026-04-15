# Phase 17: System Flag - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

User can override the system prompt for a single invocation via `fenec --system <file>` / `fenec -s <file>`. The file content completely replaces the default system prompt (from `system.md` or built-in default). Tool descriptions are still appended by the REPL as usual.

</domain>

<decisions>
## Implementation Decisions

### Error Handling
- **D-01:** Hard fail — if `--system <file>` is provided and the file doesn't exist or can't be read, exit with a clear error message (non-zero exit code). User explicitly requested this file; silently falling back would be confusing.
- **D-02:** No content validation — read file as-is, any text content is valid. Consistent with how `config.LoadSystemPrompt()` handles `system.md` today.

### Override Behavior
- **D-03:** `--system` completely replaces the default system prompt — skip `config.LoadSystemPrompt()` entirely when this flag is set. No blending or prepending.
- **D-04:** Tool descriptions remain appended by the REPL regardless of prompt source — `baseSystemPrompt` in REPL receives the override content, then tool descriptions are added as usual.

### Flag Design
- **D-05:** Register as `--system` / `-s` using pflag `StringP` — follows existing short flag convention (`-m`, `-p`, `-d`, `-y`, `-v`).

### Agent's Discretion
- Whether to add a helper function in `config` package or handle file reading inline in `main.go`
- Exact error message wording
- Whether `--system ""` (empty string flag value) is treated as "not provided" or as an error

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — FLAG-01 defines acceptance criteria for `--system` flag

### Roadmap
- `.planning/ROADMAP.md` §Phase 17 — Success criteria including tool functionality preservation

### Key Source Files
- `main.go` — Flag definitions (pflag) and system prompt loading at line 174-175
- `internal/config/config.go` — `LoadSystemPrompt()` function (lines 116-135)
- `internal/repl/repl.go` — `NewREPL()` receives systemPrompt, stores `baseSystemPrompt`, appends tool descriptions (lines 47-96)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `config.LoadSystemPrompt()` — Reads `system.md` from config dir; returns default if missing. `--system` bypasses this entirely.
- `pflag.StringP()` — Existing pattern for all flags in `main.go`
- `repl.NewREPL(..., systemPrompt, ...)` — Already accepts system prompt as parameter; no REPL changes needed

### Established Patterns
- All flags defined at top of `main()` using pflag, parsed before any logic
- System prompt flows: `main.go` loads → passes to `NewREPL()` → stored as `baseSystemPrompt` → tool descriptions appended
- Error formatting uses `render.FormatError()` with `fmt.Fprintln(os.Stderr, ...)` then `os.Exit(1)`

### Integration Points
- `main.go` line 174-175 — Insert `--system` file read before or instead of `config.LoadSystemPrompt()`
- `pflag` block at top of `main()` — Add `--system` / `-s` flag definition
- `pflag.Usage` function — Add `--system` to help text

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 17-system-flag*
*Context gathered: 2026-04-15*
