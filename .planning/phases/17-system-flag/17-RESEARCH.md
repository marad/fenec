# Phase 17: System Flag - Research

**Researched:** 2025-07-18
**Domain:** CLI flag addition / Go pflag / file I/O
**Confidence:** HIGH

## Summary

Phase 17 adds a `--system <file>` / `-s <file>` flag to override the system prompt for a single invocation. This is a surgically small feature — the entire change touches one file (`main.go`) in two places: flag definition and system prompt loading. The REPL already accepts `systemPrompt` as a parameter and appends tool descriptions independently, so zero downstream changes are needed.

The codebase has a clear, established pattern for this exact kind of work: pflag defines the flag, `os.ReadFile` reads the content, and the existing error-handling pattern (`render.FormatError` → stderr → `os.Exit(1)`) handles failure. The only design decision is where to put the file-reading logic — inline in `main.go` (recommended) or as a config helper.

**Primary recommendation:** Implement entirely in `main.go` with a conditional branch at lines 174–180. If `--system` is set and non-empty, read the file with `os.ReadFile`; otherwise fall through to `config.LoadSystemPrompt()`. Add a unit-testable helper function for the file-reading+error logic if desired, but given the 5-line implementation, inline is cleaner.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Hard fail — if `--system <file>` is provided and the file doesn't exist or can't be read, exit with a clear error message (non-zero exit code). User explicitly requested this file; silently falling back would be confusing.
- **D-02:** No content validation — read file as-is, any text content is valid. Consistent with how `config.LoadSystemPrompt()` handles `system.md` today.
- **D-03:** `--system` completely replaces the default system prompt — skip `config.LoadSystemPrompt()` entirely when this flag is set. No blending or prepending.
- **D-04:** Tool descriptions remain appended by the REPL regardless of prompt source — `baseSystemPrompt` in REPL receives the override content, then tool descriptions are added as usual.
- **D-05:** Register as `--system` / `-s` using pflag `StringP` — follows existing short flag convention (`-m`, `-p`, `-d`, `-y`, `-v`).

### Agent's Discretion
- Whether to add a helper function in `config` package or handle file reading inline in `main.go`
- Exact error message wording
- Whether `--system ""` (empty string flag value) is treated as "not provided" or as an error

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FLAG-01 | `--system <file>` flag reads file and uses content as system prompt for one invocation | Fully supported — pflag.StringP for flag, os.ReadFile for content, conditional bypass of config.LoadSystemPrompt(), REPL accepts systemPrompt param unchanged |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Flag parsing (`--system`) | CLI entry point (`main.go`) | — | All flags defined in main(), pflag handles parsing |
| File reading | CLI entry point (`main.go`) | `config` package (alternative) | Simple os.ReadFile; config package is optional but not required |
| System prompt injection | REPL (`internal/repl`) | — | NewREPL already receives systemPrompt string and appends tool descriptions |
| Tool description append | REPL (`internal/repl`) | — | `refreshSystemPrompt()` and `NewREPL()` handle this — no changes needed |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/pflag` | v1.0.10 | CLI flag parsing | Already in use for all flags [VERIFIED: go.mod] |
| `os` (stdlib) | Go 1.25.8 | `os.ReadFile` for file content, `os.Exit` for errors | Already used throughout `main.go` [VERIFIED: source] |

### Supporting
No additional libraries needed. Everything is already in the project.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Inline file read in main.go | `config.LoadSystemPromptFromFile(path)` helper | Adds a function for 3 lines of code; unnecessary indirection for this case |

**Installation:**
No new dependencies. Zero `go get` commands.

## Architecture Patterns

### System Architecture Diagram

```
CLI invocation
    │
    ▼
pflag.Parse()
    │
    ├── --system <file> provided?
    │       │
    │       YES → os.ReadFile(file) → systemPrompt
    │       │        └─ error? → render.FormatError → stderr → os.Exit(1)
    │       │
    │       NO → config.LoadSystemPrompt() → systemPrompt
    │                └─ error? → render.FormatError → stderr → os.Exit(1)
    │
    ▼
repl.NewREPL(..., systemPrompt, ...)
    │
    ├── baseSystemPrompt = systemPrompt  (stored for refresh)
    ├── toolDesc = toolRegistry.Describe()
    └── conv.Messages[0] = systemPrompt + "\n\n## Available Tools\n\n" + toolDesc
```

### Recommended Change Locations

```
main.go
├── Line 27-32 area  — Add: systemFile := pflag.StringP("system", "s", "", "...")
├── Line 34-46       — Update: pflag.Usage help text to include --system example
└── Line 174-180     — Replace: conditional system prompt loading
```

### Pattern 1: Flag Definition (existing pattern)
**What:** All flags use pflag package-level functions at top of main()
**When to use:** Every new CLI flag
**Example:**
```go
// Source: main.go lines 27-32 [VERIFIED: codebase]
modelName := pflag.StringP("model", "m", "", "Model to use (provider/model or just model name)")
// New flag follows same pattern:
systemFile := pflag.StringP("system", "s", "", "File to use as system prompt")
```

### Pattern 2: Error Handling (existing pattern)
**What:** Format error → stderr → exit 1
**When to use:** All fatal errors in main()
**Example:**
```go
// Source: main.go lines 177-179 [VERIFIED: codebase]
fmt.Fprintln(os.Stderr, render.FormatError(
    fmt.Sprintf("Failed to load system prompt: %v", err)))
os.Exit(1)
```

### Pattern 3: Conditional System Prompt Loading (new)
**What:** Check if `--system` flag was provided; if yes, read file; if no, use default path
**When to use:** Replacing lines 174-180 in main.go
**Example:**
```go
// New pattern for system prompt loading
var systemPrompt string
if *systemFile != "" {
    data, err := os.ReadFile(*systemFile)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to read system prompt file: %v", err)))
        os.Exit(1)
    }
    systemPrompt = string(data)
} else {
    var err error
    systemPrompt, err = config.LoadSystemPrompt()
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to load system prompt: %v", err)))
        os.Exit(1)
    }
}
```

### Anti-Patterns to Avoid
- **Modifying REPL to read the file:** The REPL should remain agnostic to prompt source. `main.go` is the integration point — pass the resolved string.
- **Falling back silently on file error:** D-01 explicitly requires hard fail. Do NOT catch the error and fall back to default.
- **Validating file content:** D-02 says any text content is valid. Don't check for empty files, specific formats, etc.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File reading | Custom buffered reader | `os.ReadFile` | One-shot full read is correct for prompt files (small) |
| Flag parsing | `os.Args` manual parsing | `pflag.StringP` | Already used for all flags, handles `--system=file` and `--system file` syntax |

**Key insight:** This feature is so small that the main risk is over-engineering it, not under-engineering.

## Common Pitfalls

### Pitfall 1: Short Flag Collision
**What goes wrong:** `-s` is already used by another flag
**Why it happens:** Not checking existing short flags
**How to avoid:** Verified existing short flags: `-m` (model), `-p` (pipe), `-d` (debug), `-y` (yolo), `-v` (version). `-s` is available. [VERIFIED: main.go lines 27-32]
**Warning signs:** pflag panic at startup about duplicate flag

### Pitfall 2: Empty String Treated as "Provided"
**What goes wrong:** `pflag.StringP` returns `""` as default — so checking `*systemFile != ""` correctly treats unprovided flag as "not set". But `--system ""` (explicit empty string) also passes as "not set".
**Why it happens:** pflag doesn't distinguish "flag not passed" from "flag passed with empty value" via the pointer API
**How to avoid:** This is acceptable behavior per agent's discretion. `--system ""` being treated as "not provided" is sensible — there's no valid use case for an empty system prompt file path. Document this choice.
**Warning signs:** N/A — this is a design choice, not a bug.

### Pitfall 3: Relative vs Absolute Paths
**What goes wrong:** User passes `--system ./prompt.md` and it resolves relative to CWD
**Why it happens:** `os.ReadFile` uses CWD-relative paths by default
**How to avoid:** This is actually correct behavior — users expect `./prompt.md` to be CWD-relative. No special handling needed. Do NOT resolve against config dir.
**Warning signs:** N/A — standard Go file I/O behavior.

### Pitfall 4: Forgetting to Update Help Text
**What goes wrong:** `--system` flag works but doesn't appear in usage examples
**Why it happens:** `pflag.PrintDefaults()` will auto-show the flag, but the hand-crafted Usage section (lines 34-46) also needs an example line
**How to avoid:** Add a usage example like `fenec --system prompt.md  Use a custom system prompt`
**Warning signs:** Running `fenec --help` and not seeing the flag in the examples section

## Code Examples

### Complete Implementation (main.go changes)
```go
// Source: Derived from existing patterns in main.go [VERIFIED: codebase]

// 1. Add flag definition (near line 32)
systemFile := pflag.StringP("system", "s", "", "File to use as system prompt for this session")

// 2. Update Usage function (add to examples block)
// fenec --system prompt.md  Use a custom system prompt

// 3. Replace system prompt loading block (lines 174-180)
var systemPrompt string
if *systemFile != "" {
    data, err := os.ReadFile(*systemFile)
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to read system prompt file: %v", err)))
        os.Exit(1)
    }
    systemPrompt = string(data)
} else {
    var err error
    systemPrompt, err = config.LoadSystemPrompt()
    if err != nil {
        fmt.Fprintln(os.Stderr, render.FormatError(
            fmt.Sprintf("Failed to load system prompt: %v", err)))
        os.Exit(1)
    }
}
```

### Testing Pattern (config helper approach, if chosen)
```go
// If a testable helper is desired in config package:
func LoadSystemPromptFromFile(path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", fmt.Errorf("reading system prompt file: %w", err)
    }
    return string(data), nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Only `system.md` config file | `system.md` + `--system` flag override | Phase 17 | Per-invocation prompt customization |

**Upcoming (Phase 18):** `--profile` flag will add another prompt source. FLAG-04 specifies `--system` overrides profile's prompt while keeping profile's model. This phase lays groundwork.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| — | — | — | — |

**All claims in this research were verified from codebase inspection — no user confirmation needed.**

## Open Questions

1. **`--system ""` behavior**
   - What we know: pflag returns empty string for both "not provided" and "provided with empty string". Checking `*systemFile != ""` treats both as "not provided".
   - What's unclear: Whether this needs explicit handling (e.g., `pflag.CommandLine.Changed("system")` to detect if flag was explicitly passed)
   - Recommendation: Treat empty as "not provided" — simplest, no valid use case for empty path. This is in agent's discretion per CONTEXT.md.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go conventions) |
| Quick run command | `go test ./internal/config/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FLAG-01a | `--system <file>` reads file and returns content as system prompt | unit | `go test ./internal/config/... -run TestLoadSystemPromptFromFile -x` | ❌ Wave 0 (if config helper approach) |
| FLAG-01b | `--system` with nonexistent file produces error and exits | unit | `go test ./internal/config/... -run TestLoadSystemPromptFromFile_NotExist -x` | ❌ Wave 0 (if config helper approach) |
| FLAG-01c | Without `--system`, default behavior unchanged | unit | `go test ./internal/config/... -run TestLoadSystemPrompt -x` | ✅ Existing |
| FLAG-01d | Tool descriptions still appended regardless of prompt source | integration | Manual — verify REPL still calls `toolRegistry.Describe()` (code review) | manual-only |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] Test for file reading success case (if config helper approach is used)
- [ ] Test for file reading failure case (nonexistent file)
- [ ] If inline approach: tests live in `main_test.go` or are manual integration tests

*(Note: If implementation stays inline in `main.go` with `os.ReadFile`, the testable surface is minimal — `os.ReadFile` is stdlib and doesn't need testing. The primary verification is integration: run `fenec --system <file>` and confirm behavior.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes (file path) | OS file permissions; no path traversal concern since user explicitly provides the path |
| V6 Cryptography | no | — |

### Known Threat Patterns for CLI file flags

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via `--system` | Information Disclosure | Not applicable — user explicitly provides the path; this is expected behavior, not an attack vector. User already has shell access. |
| Symlink following | Tampering | Not applicable — `os.ReadFile` follows symlinks, which is correct for a user-provided path. The user controls their own filesystem. |

**Security note:** This flag reads a file chosen by the user who is already running the binary. There is no privilege boundary being crossed. Standard `os.ReadFile` is appropriate.

## Sources

### Primary (HIGH confidence)
- `main.go` — All flag definitions, system prompt loading, error handling patterns [VERIFIED: direct file inspection]
- `internal/config/config.go` — `LoadSystemPrompt()` function [VERIFIED: direct file inspection]
- `internal/repl/repl.go` — `NewREPL()` signature, `baseSystemPrompt` field, `refreshSystemPrompt()` [VERIFIED: direct file inspection]
- `go.mod` — pflag v1.0.10, Go 1.25.8 [VERIFIED: direct file inspection]
- `.planning/phases/17-system-flag/17-CONTEXT.md` — Decisions D-01 through D-05 [VERIFIED: direct file inspection]

### Secondary (MEDIUM confidence)
- None needed — all research is codebase-derived

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, all patterns verified in codebase
- Architecture: HIGH — single-file change with clear insertion points identified
- Pitfalls: HIGH — verified short flag availability, edge cases documented

**Research date:** 2025-07-18
**Valid until:** Indefinite (stable Go stdlib + pflag patterns)
