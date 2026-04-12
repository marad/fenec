# Quick Task 260412-gan: Improve CLI help output and flag handling - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Task Boundary

Replace Go's stdlib `flag` package with `pflag` for double-dash flag convention. Add custom help output with usage examples. Add `--version` flag.

</domain>

<decisions>
## Implementation Decisions

### Flag style
- Switch from stdlib `flag` to `github.com/spf13/pflag`
- Double-dash for all flags: `--debug`, `--host`, `--pipe`, `--yolo`, `--line-by-line`
- Short forms for frequent flags: `-d` (debug), `-y` (yolo), `-p` (pipe), `-h` (host)
- Note: pflag uses `-h` for help by default — pick a different short form for host or skip it

### Help output
- Custom usage function (not pflag's default)
- Include 2-3 usage examples: interactive mode, pipe mode, yolo mode
- Show flag descriptions with short forms
- Show version in help header

### Version flag
- Add `--version` / `-v` flag that prints version and exits

### Claude's Discretion
- Exact help text formatting and layout
- Which short form to use for host if -h conflicts with help
- Whether to group flags by category in help output

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<canonical_refs>
## Canonical References

No external specs — requirements fully captured in decisions above

</canonical_refs>
