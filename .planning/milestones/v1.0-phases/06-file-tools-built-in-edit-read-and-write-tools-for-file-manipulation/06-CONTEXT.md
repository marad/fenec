# Phase 6: File Tools - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Built-in Go tools for file manipulation: read_file, write_file, edit_file, and list_directory. These give the model reliable, structured file operations instead of relying on fragile shell escaping through shell_exec. Does not include search (agent uses shell_exec with grep/rg for that).

</domain>

<decisions>
## Implementation Decisions

### Tool set
- **D-01:** Four built-in tools: `read_file`, `write_file`, `edit_file`, `list_directory`
- **D-02:** No `search_files` tool -- the agent uses `shell_exec` with `grep` or `rg` for content search
- **D-03:** All four tools implement `tool.Tool` interface and register as built-in in `main.go`

### Safety model
- **D-04:** Writes within the current working directory require no approval
- **D-05:** Writes to paths outside the current working directory require user approval via the existing `ApproverFunc` pattern (same as ShellTool)
- **D-06:** Hard deny list blocks both reads and writes to sensitive paths: `/etc`, `/usr`, `/bin`, `/sbin`, `/boot`, `~/.ssh`, `~/.gnupg`
- **D-07:** Deny list check runs before approval -- denied paths are rejected outright, not sent to the approval prompt
- **D-08:** `read_file` and `list_directory` are unrestricted except for deny-listed paths (no approval for out-of-cwd reads)

### read_file
- **D-09:** Parameters: `path` (required), `offset` (optional, 0-based start line), `limit` (optional, max lines to read)
- **D-10:** Default behavior with no offset/limit: read entire file up to 1000 lines
- **D-11:** If file exceeds limit, truncate and include a truncation warning in the result
- **D-12:** Result JSON includes: `content`, `total_lines`, `lines_shown`, `truncated` (bool)
- **D-13:** Binary file detection: check first 512 bytes for null bytes. If binary, return error: "binary file detected, use shell_exec for binary inspection"

### write_file
- **D-14:** Parameters: `path` (required), `content` (required)
- **D-15:** Creates parent directories automatically if they don't exist (mkdir -p behavior)
- **D-16:** Overwrites existing files without prompting (approval only for out-of-cwd paths, per D-05)

### edit_file
- **D-17:** Search-and-replace model. Parameters: `path` (required), `old_text` (required, exact match), `new_text` (required)
- **D-18:** If `old_text` appears multiple times, replace first occurrence only
- **D-19:** If `old_text` not found in file, return error with message
- **D-20:** File must exist -- returns error if path doesn't exist (use `write_file` to create new files)
- **D-21:** On success, return JSON with: status "ok", `lines_changed` count, and a few surrounding context lines around the edit point

### list_directory
- **D-22:** Parameters: `path` (required)
- **D-23:** Each entry includes: `name`, `is_dir` (bool), `size` (bytes, 0 for directories)
- **D-24:** Result is JSON array of entries, sorted with directories first then files alphabetically

### Claude's Discretion
- Exact deny list matching logic (prefix match, symlink resolution)
- Working directory detection method (os.Getwd or passed in)
- Number of context lines to show in edit_file success response
- list_directory handling of permission errors on individual entries
- Error message wording for all failure cases
- Internal file I/O implementation details (buffered reads, atomic writes)

</decisions>

<specifics>
## Specific Ideas

- Reuse the closure-deferred approver pattern from ShellTool -- file tools take the same `ApproverFunc` callback
- The deny list should be a package-level function in internal/tool (similar to `IsDangerous` in safety.go) so it's testable independently
- edit_file's search-and-replace should preserve the file's original line endings (don't convert \r\n to \n)

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Tool system patterns
- `internal/tool/registry.go` -- Tool interface, Registry with Register/Dispatch/Describe
- `internal/tool/shell.go` -- ShellTool as reference for built-in tool design, ApproverFunc pattern
- `internal/tool/safety.go` -- IsDangerous function, pattern for path-based safety checks

### Integration points
- `main.go` -- Tool registration at startup (lines 87-141), approver wiring pattern
- `internal/repl/repl.go` -- System prompt tool injection, agentic loop dispatch

### Config
- `internal/config/config.go` -- Config resolution patterns (ToolsDir, SessionDir)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `tool.ApproverFunc`: Callback type for user approval -- reuse for out-of-cwd write approval
- `safety.IsDangerous()`: Pattern for deny-list checking -- extend or create parallel `IsDeniedPath()`
- `tool.Registry.Register()`: Registration for new built-in tools
- `errorJSON()` / `successJSON()` in create.go: JSON result formatting helpers

### Established Patterns
- Built-in tools implement `tool.Tool` interface (Name, Definition, Execute)
- Tool results are JSON strings returned to the model
- Approval callback is wired via closure in main.go (set after REPL creation)
- ShellResult pattern for structured JSON responses with metadata

### Integration Points
- `main.go` lines 87-141: Where new built-in tools register alongside shell_exec and self-extension tools
- System prompt rebuilds each turn from `registry.Describe()` -- new tools appear automatically
- REPL dispatch loop handles all tools uniformly via `registry.Dispatch()`

</code_context>

<deferred>
## Deferred Ideas

- File watching / change notifications -- future enhancement
- Diff display for edit operations in REPL output -- cosmetic, not needed for tool functionality
- Configurable deny list (user-editable) -- keep hard-coded for now

</deferred>

---

*Phase: 06-file-tools-built-in-edit-read-and-write-tools-for-file-manipulation*
*Context gathered: 2026-04-11*
