---
phase: 16-profile-package
plan: "01"
subsystem: profile
tags: [profile, parsing, toml, file-io]
dependency_graph:
  requires: []
  provides: [Profile, Frontmatter, ProfileSummary, Parse, Load, List, ProfilesDir]
  affects: [internal/config/config.go]
tech_stack:
  added: []
  patterns: ["+++‑delimited TOML frontmatter parsing", "provider/model split via SplitN", "path traversal protection"]
key_files:
  created:
    - internal/profile/profile.go
    - internal/profile/profile_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
decisions:
  - "Used line-by-line parsing to find +++ delimiters (simple, handles edge cases cleanly)"
  - "Path traversal protection rejects any name with /, \\, or . (strictest safe subset)"
  - "List() silently skips unparseable .md files (graceful degradation)"
metrics:
  duration: "~5min"
  completed: "2026-04-15"
  tasks_completed: 2
  tasks_total: 2
  test_count: 25
  files_changed: 4
requirements_completed: [PROF-01, PROF-02, PROF-03]
---

# Phase 16 Plan 01: Profile Package Summary

**One-liner:** TOML frontmatter parser with provider/model split, file I/O (Load/List), and path traversal protection for .md profile files.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Profile types and TOML frontmatter parsing | `62a5a87` (RED), `f1856f4` (GREEN) | `internal/profile/profile.go`, `internal/profile/profile_test.go` |
| 2 | File I/O — Load, List, ProfilesDir | `fc4a45f` (RED), `308c884` (GREEN) | `internal/profile/profile.go`, `internal/profile/profile_test.go`, `internal/config/config.go`, `internal/config/config_test.go` |

## Implementation Details

### Parse Function
- Splits content on `\n`, finds lines that are exactly `+++`
- Extracts TOML between first and second `+++` delimiters
- Decodes via `toml.Decode()` (BurntSushi/toml, already in go.mod)
- Splits `model` field on `/` using `strings.SplitN(model, "/", 2)` — matching main.go pattern
- Body after second `+++` is `strings.TrimSpace()`'d as SystemPrompt
- Descriptive error messages: "missing frontmatter" and "parsing frontmatter"

### Load Function
- Validates name against path traversal: rejects `/`, `\`, `.` characters
- Reads `{dir}/{name}.md`, calls Parse(), sets `profile.Name`
- Wraps errors with filename context

### List Function
- Reads directory entries, filters for `.md` suffix
- Silently skips directories, non-.md files, and unparseable files
- Returns sorted `[]ProfileSummary` by Name
- Non-existent directory returns empty slice (no error)

### ProfilesDir
- Follows exact ToolsDir() pattern: `filepath.Join(ConfigDir(), "profiles")`
- Does NOT create the directory

## Test Results

```
=== Profile package: 25 tests ===
TestParseFullProfile                    PASS
TestParseBareModel                      PASS
TestParseWithDescription                PASS
TestParseEmptyModel                     PASS
TestParseMissingFrontmatter             PASS
TestParseSingleDelimiterOnly            PASS
TestParseMalformedTOML                  PASS
TestParseEmptyBody                      PASS
TestParseBodyWhitespaceTrimmed          PASS
TestParseUnknownFieldsIgnored           PASS
TestParseModelSplitVariants (5 sub)     PASS
TestLoadExistingProfile                 PASS
TestLoadNonExistentProfile              PASS
TestLoadPathTraversalDotDot             PASS
TestLoadPathTraversalSlash              PASS
TestLoadPathTraversalBackslash          PASS
TestLoadPathTraversalDot                PASS
TestListWithProfiles                    PASS
TestListEmptyDirectory                  PASS
TestListNonExistentDirectory            PASS
TestListSkipsNonMdFiles                 PASS

=== Config package: TestProfilesDir ===
TestProfilesDir                         PASS

go vet ./internal/profile/ ./internal/config/  — clean
go build .                                     — success
```

## Deviations from Plan

None — plan executed exactly as written.

## TDD Gate Compliance

Both tasks followed RED → GREEN flow:
- Task 1: `test(16-01)` commit `62a5a87` (RED) → `feat(16-01)` commit `f1856f4` (GREEN) ✓
- Task 2: `test(16-01)` commit `fc4a45f` (RED) → `feat(16-01)` commit `308c884` (GREEN) ✓

No refactor phase needed — code was clean from the start.

## Known Stubs

None — all functions are fully implemented with real behavior.

## Self-Check: PASSED

- [x] `internal/profile/profile.go` exists (159 lines)
- [x] `internal/profile/profile_test.go` exists (232 lines, exceeds 100 line minimum)
- [x] `internal/config/config.go` contains ProfilesDir
- [x] `internal/config/config_test.go` contains TestProfilesDir
- [x] Commit `62a5a87` exists
- [x] Commit `f1856f4` exists
- [x] Commit `fc4a45f` exists
- [x] Commit `308c884` exists
