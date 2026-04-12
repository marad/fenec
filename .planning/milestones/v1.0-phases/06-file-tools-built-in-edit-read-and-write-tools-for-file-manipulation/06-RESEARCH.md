# Phase 6: File Tools - Research

**Researched:** 2026-04-11
**Domain:** Go file I/O, path security, tool interface implementation
**Confidence:** HIGH

## Summary

Phase 6 adds four built-in Go tools (`read_file`, `write_file`, `edit_file`, `list_directory`) to the existing tool registry. The implementation is straightforward because the project already has a well-established `tool.Tool` interface, registration pattern, approval callback mechanism, and JSON result formatting helpers. No new dependencies are needed -- everything uses Go standard library (`os`, `path/filepath`, `bufio`, `strings`, `encoding/json`).

The primary complexity is the path safety model: a deny list for sensitive system paths, a working-directory check for write approval gating, and symlink-aware path resolution to prevent traversal attacks. Go 1.24+ provides `filepath.EvalSymlinks` for resolving symlinks before checking, which is sufficient for this use case since the deny list operates on resolved absolute paths and there is no TOCTOU risk for a single-user CLI tool.

**Primary recommendation:** Implement a shared `pathcheck` package (or functions in `internal/tool`) that provides `IsDeniedPath(path) bool` and `IsOutsideCWD(path) bool`. Each file tool calls these before executing. The four tools follow the exact same structural pattern as `ShellTool` and `CreateLuaTool` -- constructor, Name(), Definition(), Execute() with JSON results.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Four built-in tools: `read_file`, `write_file`, `edit_file`, `list_directory`
- **D-02:** No `search_files` tool -- the agent uses `shell_exec` with `grep` or `rg` for content search
- **D-03:** All four tools implement `tool.Tool` interface and register as built-in in `main.go`
- **D-04:** Writes within the current working directory require no approval
- **D-05:** Writes to paths outside the current working directory require user approval via the existing `ApproverFunc` pattern (same as ShellTool)
- **D-06:** Hard deny list blocks both reads and writes to sensitive paths: `/etc`, `/usr`, `/bin`, `/sbin`, `/boot`, `~/.ssh`, `~/.gnupg`
- **D-07:** Deny list check runs before approval -- denied paths are rejected outright, not sent to the approval prompt
- **D-08:** `read_file` and `list_directory` are unrestricted except for deny-listed paths (no approval for out-of-cwd reads)
- **D-09:** read_file parameters: `path` (required), `offset` (optional, 0-based start line), `limit` (optional, max lines to read)
- **D-10:** Default behavior with no offset/limit: read entire file up to 1000 lines
- **D-11:** If file exceeds limit, truncate and include a truncation warning in the result
- **D-12:** Result JSON includes: `content`, `total_lines`, `lines_shown`, `truncated` (bool)
- **D-13:** Binary file detection: check first 512 bytes for null bytes. If binary, return error
- **D-14:** write_file parameters: `path` (required), `content` (required)
- **D-15:** Creates parent directories automatically if they don't exist (mkdir -p behavior)
- **D-16:** Overwrites existing files without prompting (approval only for out-of-cwd paths, per D-05)
- **D-17:** edit_file: search-and-replace model. Parameters: `path` (required), `old_text` (required, exact match), `new_text` (required)
- **D-18:** If `old_text` appears multiple times, replace first occurrence only
- **D-19:** If `old_text` not found in file, return error with message
- **D-20:** File must exist -- returns error if path doesn't exist
- **D-21:** On success, return JSON with: status "ok", `lines_changed` count, and surrounding context lines
- **D-22:** list_directory parameters: `path` (required)
- **D-23:** Each entry includes: `name`, `is_dir` (bool), `size` (bytes, 0 for directories)
- **D-24:** Result is JSON array of entries, sorted with directories first then files alphabetically

### Claude's Discretion
- Exact deny list matching logic (prefix match, symlink resolution)
- Working directory detection method (os.Getwd or passed in)
- Number of context lines to show in edit_file success response
- list_directory handling of permission errors on individual entries
- Error message wording for all failure cases
- Internal file I/O implementation details (buffered reads, atomic writes)

### Deferred Ideas (OUT OF SCOPE)
- File watching / change notifications -- future enhancement
- Diff display for edit operations in REPL output -- cosmetic, not needed for tool functionality
- Configurable deny list (user-editable) -- keep hard-coded for now
</user_constraints>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os` | 1.25.8 (installed) | File I/O: Open, ReadFile, WriteFile, MkdirAll, Stat, ReadDir | All file operations use stdlib. No external file library needed. |
| Go stdlib `path/filepath` | 1.25.8 | Abs, EvalSymlinks, Clean, Rel, Dir for path resolution and safety | Standard path manipulation. EvalSymlinks resolves symlinks before deny-list check. |
| Go stdlib `bufio` | 1.25.8 | Line-by-line reading with Scanner for offset/limit support | Efficient line counting without loading entire file into memory. |
| Go stdlib `encoding/json` | 1.25.8 | JSON result marshaling | Same pattern as existing tools (ShellResult.ToJSON, errorJSON, successJSON). |
| Go stdlib `strings` | 1.25.8 | String replacement for edit_file (strings.Replace with count=1) | First-occurrence-only replacement per D-18. |
| Go stdlib `os/user` | 1.25.8 | Resolve ~ in deny list paths to actual home directory | Needed for `~/.ssh` and `~/.gnupg` deny list entries. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/ollama/ollama/api | v0.20.5 | Tool, ToolProperty, ToolCallFunctionArguments types | Already in go.mod. Tool definition and argument handling. |
| github.com/stretchr/testify | v1.11.1 | Test assertions | Already in go.mod. assert/require for unit tests. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `filepath.EvalSymlinks` for symlink resolution | `os.Root` (Go 1.24+) | os.Root is for constraining operations within a directory. Our design allows reads anywhere except deny-listed paths, so Root is too restrictive. EvalSymlinks + prefix match is the right tool here. |
| `bufio.Scanner` for line reading | `os.ReadFile` + `strings.Split` | ReadFile loads entire file into memory. Scanner is more efficient for large files and naturally supports offset/limit without loading everything. However, for files under the 1000-line default limit, either approach is fine. |
| `strings.Replace(content, old, new, 1)` for edit | Regexp-based replacement | Exact match is simpler and matches D-17. Regex would add complexity and the model sends exact text. |

**Installation:**
```bash
# No new dependencies needed -- all stdlib + existing go.mod entries
```

## Architecture Patterns

### Recommended Project Structure
```
internal/tool/
    read.go          # ReadFileTool
    read_test.go
    write.go         # WriteFileTool
    write_test.go
    edit.go          # EditFileTool
    edit_test.go
    listdir.go       # ListDirTool
    listdir_test.go
    pathcheck.go     # IsDeniedPath, IsOutsideCWD, resolveAndCheck helpers
    pathcheck_test.go
    safety.go        # Existing IsDangerous (unchanged)
    shell.go         # Existing ShellTool (unchanged)
    registry.go      # Existing Registry (unchanged)
    create.go        # Existing CreateLuaTool (unchanged)
    update.go        # Existing UpdateLuaTool (unchanged)
    delete.go        # Existing DeleteLuaTool (unchanged)
```

### Pattern 1: Tool Implementation (same as existing tools)

**What:** Each file tool is a struct implementing `tool.Tool` with Name(), Definition(), Execute().
**When to use:** All four tools follow this exact pattern.
**Example:**
```go
// Source: internal/tool/shell.go and create.go (existing codebase patterns)
type ReadFileTool struct {
    approver ApproverFunc // Only needed for write tools, but included for consistency
}

func NewReadFileTool() *ReadFileTool {
    return &ReadFileTool{}
}

func (r *ReadFileTool) Name() string { return "read_file" }

func (r *ReadFileTool) Definition() api.Tool {
    props := api.NewToolPropertiesMap()
    props.Set("path", api.ToolProperty{
        Type:        api.PropertyType{"string"},
        Description: "Absolute or relative path to the file to read",
    })
    props.Set("offset", api.ToolProperty{
        Type:        api.PropertyType{"integer"},
        Description: "Start reading from this line number (0-based). Optional.",
    })
    props.Set("limit", api.ToolProperty{
        Type:        api.PropertyType{"integer"},
        Description: "Maximum number of lines to read. Optional, defaults to 1000.",
    })

    return api.Tool{
        Type: "function",
        Function: api.ToolFunction{
            Name:        "read_file",
            Description: "Read the contents of a file. Returns the file content with line count metadata.",
            Parameters: api.ToolFunctionParameters{
                Type:       "object",
                Required:   []string{"path"},
                Properties: props,
            },
        },
    }
}
```

### Pattern 2: Path Safety Check (new, shared across tools)

**What:** Centralized functions for deny-list checking and CWD boundary detection.
**When to use:** Called by all four tools before executing file operations.
**Example:**
```go
// Source: Design based on existing safety.go pattern

// deniedPrefixes are resolved at init time with home directory expansion.
var deniedPrefixes []string

func init() {
    home, _ := os.UserHomeDir()
    deniedPrefixes = []string{
        "/etc", "/usr", "/bin", "/sbin", "/boot",
        filepath.Join(home, ".ssh"),
        filepath.Join(home, ".gnupg"),
    }
}

// IsDeniedPath checks if a path (after resolving symlinks and making absolute)
// falls within any denied prefix.
func IsDeniedPath(path string) (bool, error) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return true, err // Deny on resolution failure
    }
    // Resolve symlinks to prevent traversal via symlink into denied dir.
    resolved, err := filepath.EvalSymlinks(absPath)
    if err != nil {
        // For non-existent paths (write_file creating new files),
        // resolve the parent directory instead.
        resolved, err = filepath.EvalSymlinks(filepath.Dir(absPath))
        if err != nil {
            return true, err
        }
        resolved = filepath.Join(resolved, filepath.Base(absPath))
    }
    for _, prefix := range deniedPrefixes {
        if resolved == prefix || strings.HasPrefix(resolved, prefix+string(filepath.Separator)) {
            return true, nil
        }
    }
    return false, nil
}

// IsOutsideCWD checks if a resolved path is outside the current working directory.
func IsOutsideCWD(path string) (bool, error) {
    cwd, err := os.Getwd()
    if err != nil {
        return true, err // Fail closed
    }
    absPath, err := filepath.Abs(path)
    if err != nil {
        return true, err
    }
    rel, err := filepath.Rel(cwd, absPath)
    if err != nil {
        return true, err
    }
    return strings.HasPrefix(rel, ".."), nil
}
```

### Pattern 3: Integer Argument Extraction

**What:** Safely extracting optional integer parameters from `ToolCallFunctionArguments`.
**When to use:** `read_file` offset and limit parameters.
**Example:**
```go
// JSON unmarshaling produces float64 for numbers. Convert safely.
func getOptionalInt(args api.ToolCallFunctionArguments, key string, defaultVal int) (int, error) {
    val, ok := args.Get(key)
    if !ok {
        return defaultVal, nil
    }
    switch v := val.(type) {
    case float64:
        return int(v), nil
    case json.Number:
        n, err := v.Int64()
        if err != nil {
            return 0, fmt.Errorf("argument '%s' must be an integer", key)
        }
        return int(n), nil
    default:
        return 0, fmt.Errorf("argument '%s' must be an integer, got %T", key, val)
    }
}
```

### Pattern 4: JSON Result Format (reuse existing helpers)

**What:** Return JSON strings as tool results, reusing `errorJSON()` from create.go.
**When to use:** All tool results.
**Example:**
```go
// read_file result
type ReadResult struct {
    Content    string `json:"content"`
    TotalLines int    `json:"total_lines"`
    LinesShown int    `json:"lines_shown"`
    Truncated  bool   `json:"truncated"`
}

// edit_file result
type EditResult struct {
    Status       string `json:"status"`
    LinesChanged int    `json:"lines_changed"`
    Context      string `json:"context"`  // Surrounding lines around edit point
}

// list_directory entry
type DirEntry struct {
    Name  string `json:"name"`
    IsDir bool   `json:"is_dir"`
    Size  int64  `json:"size"`
}
```

### Pattern 5: Approver Wiring (same closure pattern as ShellTool)

**What:** Write tools (`write_file`, `edit_file`) take an `ApproverFunc` via the same closure-deferred pattern used by ShellTool in main.go.
**When to use:** Out-of-CWD write approval.
**Example:**
```go
// In main.go, same pattern as shellTool:
readTool := tool.NewReadFileTool()
registry.Register(readTool)

writeTool := tool.NewWriteFileTool(func(desc string) bool {
    if approver != nil {
        return approver(desc)
    }
    return false
})
registry.Register(writeTool)

editTool := tool.NewEditFileTool(func(desc string) bool {
    if approver != nil {
        return approver(desc)
    }
    return false
})
registry.Register(editTool)

listTool := tool.NewListDirTool()
registry.Register(listTool)
```

### Anti-Patterns to Avoid

- **Loading entire file for read_file:** Use `bufio.Scanner` with line counting for offset/limit. Do not `os.ReadFile` + `strings.Split` for potentially large files.
- **Resolving paths after operations:** Always resolve and check paths BEFORE any file I/O. The deny list and CWD check must run first.
- **Converting \r\n to \n in edit_file:** Per CONTEXT.md specifics, edit_file must preserve original line endings. Use `strings.Replace` on the raw file content, not on split lines.
- **Using os.Root for file tool operations:** `os.Root` constrains operations to a single directory. Our design intentionally allows reads from anywhere (except denied paths) and writes with approval outside CWD. Root is the wrong abstraction here.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Binary detection | Custom magic number table | Null byte check in first 512 bytes | D-13 specifies this exact approach. Simple, fast, sufficient. |
| Path safety | Ad-hoc string checks per tool | Shared `IsDeniedPath`/`IsOutsideCWD` functions | Same logic in 4 tools = single testable package-level function. |
| JSON result formatting | Manual string concatenation | `json.Marshal` with typed structs | Existing pattern from ShellResult, errorJSON. Prevents escaping bugs. |
| Line-by-line reading | Manual byte counting | `bufio.Scanner` with line counter | Scanner handles line endings (\n, \r\n) correctly. |
| Home directory in deny list | Hardcoded path string | `os.UserHomeDir()` at init | Cross-platform (Linux/macOS). |

**Key insight:** The four file tools are structurally identical to existing tools. The only new concern is the path safety layer, which is a small shared module.

## Common Pitfalls

### Pitfall 1: Symlink Traversal Past Deny List
**What goes wrong:** A symlink at `/home/user/link -> /etc/passwd` bypasses a naive deny-list check that only examines the provided path string.
**Why it happens:** Checking the raw path before resolving symlinks.
**How to avoid:** Call `filepath.EvalSymlinks` on the absolute path BEFORE checking against the deny list. If EvalSymlinks fails for a non-existent file (e.g., write_file creating a new file), resolve the parent directory instead and append the filename.
**Warning signs:** Tests that use symlinks pointing into denied directories passing unexpectedly.

### Pitfall 2: JSON Number Types from Ollama
**What goes wrong:** `args.Get("offset")` returns `float64` (Go's default JSON number type), not `int`. Direct type assertion to `int` panics or fails silently.
**Why it happens:** JSON has no integer type. Go's `encoding/json` unmarshals numbers as `float64` by default.
**How to avoid:** Type-assert to `float64` first, then convert to `int`. Handle the case where the model sends a string number too (defensive coding).
**Warning signs:** Test with `makeToolArgs(map[string]interface{}{"offset": 10})` -- the 10 becomes float64 after JSON roundtrip.

### Pitfall 3: Line Ending Corruption in edit_file
**What goes wrong:** File opened with `\r\n` line endings (Windows-origin files on Linux) gets line endings changed to `\n` after edit_file.
**Why it happens:** Many Go text processing approaches split on `\n` and rejoin, silently dropping `\r`.
**How to avoid:** Read the file as raw bytes, use `strings.Replace(content, oldText, newText, 1)` on the full content string. Do not split into lines for the replacement operation.
**Warning signs:** Files gaining or losing bytes after edit_file without intentional content change.

### Pitfall 4: Race on CWD Detection
**What goes wrong:** If `os.Getwd()` is called during tool execution but the working directory was changed by a shell_exec tool call in a previous round, the CWD check may produce unexpected results.
**Why it happens:** `os.Getwd()` returns the process's current working directory, which could theoretically change.
**How to avoid:** This is very unlikely for a single-user CLI. The agent runs shell commands in child processes (`exec.Command`) which do not change the parent's CWD. Document this assumption. No mitigation needed.
**Warning signs:** Only if someone adds a `cd` built-in tool that changes the process CWD.

### Pitfall 5: Deny List Partial Prefix Match
**What goes wrong:** Denying `/etc` also denies `/etcetera` if using naive `strings.HasPrefix`.
**Why it happens:** The prefix `/etc` is a substring of `/etcetera`.
**How to avoid:** Check for exact match OR prefix with trailing separator: `resolved == prefix || strings.HasPrefix(resolved, prefix + "/")`.
**Warning signs:** Legitimate paths starting with a deny-list entry being rejected.

### Pitfall 6: edit_file Context Line Count
**What goes wrong:** Showing too many or too few context lines around the edit point in the success response, consuming unnecessary tokens or providing insufficient feedback to the model.
**Why it happens:** This is a discretionary choice. Too many lines waste model context; too few don't help the model verify its edit.
**How to avoid:** Use 3 lines before and after the edit point. This is the standard diff context size (matching `diff -U3` convention).
**Warning signs:** Model making repeated edit_file calls because it can't verify the first one worked.

## Code Examples

Verified patterns from the existing codebase:

### Binary File Detection (D-13)
```go
// Check first 512 bytes for null bytes
func isBinaryFile(path string) (bool, error) {
    f, err := os.Open(path)
    if err != nil {
        return false, err
    }
    defer f.Close()

    buf := make([]byte, 512)
    n, err := f.Read(buf)
    if err != nil && err != io.EOF {
        return false, err
    }
    for i := 0; i < n; i++ {
        if buf[i] == 0 {
            return true, nil
        }
    }
    return false, nil
}
```

### Line-Based File Reading with Offset/Limit (D-09 through D-12)
```go
func readFileLines(path string, offset, limit int) (*ReadResult, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    var lines []string
    lineNum := 0
    totalLines := 0

    for scanner.Scan() {
        totalLines++
        if lineNum >= offset && len(lines) < limit {
            lines = append(lines, scanner.Text())
        }
        lineNum++
    }
    if err := scanner.Err(); err != nil {
        return nil, err
    }

    return &ReadResult{
        Content:    strings.Join(lines, "\n"),
        TotalLines: totalLines,
        LinesShown: len(lines),
        Truncated:  totalLines > offset+limit,
    }, nil
}
```

### Search-and-Replace Edit (D-17 through D-21)
```go
func editFileReplace(path, oldText, newText string) (*EditResult, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    original := string(content)
    if !strings.Contains(original, oldText) {
        return nil, fmt.Errorf("text not found in file")
    }

    // Replace first occurrence only (per D-18)
    updated := strings.Replace(original, oldText, newText, 1)

    if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
        return nil, err
    }

    // Count changed lines
    oldLines := strings.Count(oldText, "\n")
    newLines := strings.Count(newText, "\n")
    linesChanged := abs(newLines - oldLines) + 1

    // Extract context around edit point
    editIdx := strings.Index(original, oldText)
    context := extractContext(updated, editIdx, 3) // 3 lines before/after

    return &EditResult{
        Status:       "ok",
        LinesChanged: linesChanged,
        Context:      context,
    }, nil
}
```

### Tool Registration in main.go (existing pattern)
```go
// Source: main.go lines 87-141
// File tools follow exact same pattern as shellTool and self-extension tools:

readTool := tool.NewReadFileTool()
registry.Register(readTool)

writeTool := tool.NewWriteFileTool(func(desc string) bool {
    if approver != nil { return approver(desc) }
    return false
})
registry.Register(writeTool)

editTool := tool.NewEditFileTool(func(desc string) bool {
    if approver != nil { return approver(desc) }
    return false
})
registry.Register(editTool)

listTool := tool.NewListDirTool()
registry.Register(listTool)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `filepath.EvalSymlinks` + prefix check | `os.Root` (Go 1.24+) for directory-constrained ops | Go 1.24 (Feb 2025) | os.Root prevents TOCTOU races. Not applicable here because our tools intentionally allow access outside CWD (with approval). |
| Manual `ioutil.ReadFile` | `os.ReadFile` (Go 1.16+) | Go 1.16 (Feb 2021) | `io/ioutil` is deprecated. Use `os.ReadFile` / `os.WriteFile` directly. |
| `ioutil.ReadDir` | `os.ReadDir` (Go 1.16+) | Go 1.16 (Feb 2021) | Returns `[]fs.DirEntry` which is more efficient (no stat per entry for type check). |

**Deprecated/outdated:**
- `io/ioutil`: Entire package deprecated since Go 1.16. Use `os` and `io` equivalents.
- `filepath.Walk`: Replaced by `filepath.WalkDir` (Go 1.16+) for better performance. Not needed here since list_directory is non-recursive.

## Open Questions

1. **Approval prompt wording for file writes**
   - What we know: ShellTool shows `[dangerous command] <command>` and prompts `Allow? [y/N]`
   - What's unclear: Should file write approval show the full path, or path + operation type?
   - Recommendation: Show `[write outside cwd] /absolute/path/to/file` for consistency with the shell pattern. Claude's discretion per CONTEXT.md.

2. **edit_file preserving file permissions**
   - What we know: `os.WriteFile` takes a permission mode. Original file may have specific permissions.
   - What's unclear: Should edit_file preserve the original file's permission bits?
   - Recommendation: Read original permissions with `os.Stat` before writing, pass them to `os.WriteFile`. This is a small detail but prevents permission changes.

3. **bufio.Scanner line length limit**
   - What we know: `bufio.Scanner` has a default token size of 64KB per line. Very long lines (minified JS, large data files) could exceed this.
   - What's unclear: Whether to increase the buffer or let it fail.
   - Recommendation: Increase scanner buffer to 1MB with `scanner.Buffer(make([]byte, 1024*1024), 1024*1024)`. This handles most real-world files without excessive memory use.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None needed (Go test runner) |
| Quick run command | `go test ./internal/tool/ -run TestFile -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map

Since no formal requirement IDs were provided for Phase 6, tests map to the decision IDs from CONTEXT.md:

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-06 | Deny list blocks sensitive paths | unit | `go test ./internal/tool/ -run TestIsDeniedPath -v` | Wave 0 |
| D-07 | Deny list runs before approval | unit | `go test ./internal/tool/ -run TestDenyBeforeApproval -v` | Wave 0 |
| D-08 | read_file unrestricted except deny list | unit | `go test ./internal/tool/ -run TestReadFile -v` | Wave 0 |
| D-09/D-10 | read_file offset/limit/defaults | unit | `go test ./internal/tool/ -run TestReadFileOffset -v` | Wave 0 |
| D-11/D-12 | Truncation with metadata | unit | `go test ./internal/tool/ -run TestReadFileTruncation -v` | Wave 0 |
| D-13 | Binary file detection | unit | `go test ./internal/tool/ -run TestBinaryDetect -v` | Wave 0 |
| D-14/D-15 | write_file creates dirs | unit | `go test ./internal/tool/ -run TestWriteFile -v` | Wave 0 |
| D-04/D-05 | CWD approval gating | unit | `go test ./internal/tool/ -run TestWriteApproval -v` | Wave 0 |
| D-17/D-18 | edit_file first occurrence replace | unit | `go test ./internal/tool/ -run TestEditFile -v` | Wave 0 |
| D-19 | edit_file text not found error | unit | `go test ./internal/tool/ -run TestEditFileNotFound -v` | Wave 0 |
| D-22/D-23/D-24 | list_directory sorted output | unit | `go test ./internal/tool/ -run TestListDir -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/tool/ -v -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- `internal/tool/pathcheck.go` + `pathcheck_test.go` -- deny list and CWD check logic
- `internal/tool/read.go` + `read_test.go` -- ReadFileTool
- `internal/tool/write.go` + `write_test.go` -- WriteFileTool
- `internal/tool/edit.go` + `edit_test.go` -- EditFileTool
- `internal/tool/listdir.go` + `listdir_test.go` -- ListDirTool

None -- existing test infrastructure (Go test runner + testify) covers all needs. No framework install required.

## Sources

### Primary (HIGH confidence)
- Existing codebase: `internal/tool/shell.go`, `safety.go`, `create.go`, `registry.go` -- established patterns for Tool interface, ApproverFunc, JSON results, test helpers
- Existing codebase: `main.go` lines 87-141 -- tool registration and approver wiring pattern
- [Go os.Root blog post](https://go.dev/blog/osroot) -- traversal-resistant file APIs in Go 1.24, confirmed os.Root is not the right fit for this design
- [Ollama API types.go](https://pkg.go.dev/github.com/ollama/ollama/api) -- `PropertyType{"integer"}` confirmed for integer tool parameters, verified in Ollama test files
- [Go filepath.EvalSymlinks](https://pkg.go.dev/path/filepath#EvalSymlinks) -- symlink resolution for deny-list checking

### Secondary (MEDIUM confidence)
- [Go path traversal prevention](https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/) -- filepath.Clean and EvalSymlinks patterns
- [Binary file detection](https://groups.google.com/g/golang-nuts/c/YeLL7L7SwWs) -- null byte detection approach

### Tertiary (LOW confidence)
- None. All findings verified against codebase or official Go documentation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All stdlib, no new dependencies, verified against existing go.mod
- Architecture: HIGH - Direct extension of established tool patterns in the codebase, all reference files read
- Pitfalls: HIGH - Path safety and JSON type issues well-documented in Go ecosystem, verified against API source code

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- no moving parts, all Go stdlib)
