# Phase 17: System Flag - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 17-system-flag
**Areas discussed:** Error handling (expanded to cover validation, override behavior, flag design)

---

## Error Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Hard fail | Exit with error message if file not found/unreadable | ✓ |
| Warn and fallback | Print warning to stderr, use default system prompt anyway | |
| You decide | Agent's discretion | |

**User's choice:** Hard fail — user explicitly asked for this file, silently ignoring is confusing
**Notes:** None

## Content Validation

| Option | Description | Selected |
|--------|-------------|----------|
| No validation | Read file as-is, any text content is valid | ✓ |
| Require non-empty | Reject empty files with an error | |
| You decide | Agent's discretion | |

**User's choice:** No validation — consistent with how system.md works today
**Notes:** None

## Override Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Skip default entirely | --system replaces default system.md completely | ✓ |
| Prepend to default | --system content added before default system.md | |
| You decide | Agent's discretion | |

**User's choice:** Skip default system.md entirely — clean override, no blending
**Notes:** Phase 18 will handle --system + --profile composability (FLAG-04)

## Short Flag

| Option | Description | Selected |
|--------|-------------|----------|
| --system / -s | Follows pflag short flag convention | ✓ |
| --system only | No short flag | |
| You decide | Agent's discretion | |

**User's choice:** --system / -s — follows existing convention (-m, -p, -d, -y, -v)
**Notes:** None

---

## Agent's Discretion

- Helper function placement (config package vs inline in main.go)
- Exact error message wording
- Empty string flag value handling

## Deferred Ideas

None — discussion stayed within phase scope
