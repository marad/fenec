---
status: awaiting_human_verify
trigger: "The `\"dd \"` pattern in fenec's `dangerousPatterns` list causes false positives for commands containing \"add\""
created: 2026-04-12T00:00:00Z
updated: 2026-04-12T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — `strings.Contains` causes false positives for "dd " (matches inside "add "), and "reboot"/"shutdown" can match inside filenames
test: n/a — root cause confirmed
expecting: n/a
next_action: Awaiting human verification of fix

## Symptoms

expected: `git add .` should NOT be flagged as dangerous
actual: `git add .` triggers the dangerous command approval prompt because `strings.Contains("git add .", "dd ")` returns true
errors: No errors — incorrect classification as dangerous
reproduction: Any command containing "add" followed by a space
started: Since dangerousPatterns list was created

## Eliminated

## Evidence

- timestamp: 2026-04-12
  checked: All 18 patterns in dangerousPatterns list for substring false-positive risk
  found: |
    HIGH RISK: "dd " matches inside "add " — triggers on `git add .`, `npm add`, `yarn add`, `useradd`
    MODERATE RISK: "reboot" (no trailing space) matches inside filenames like "reboot_handler.py"
    MODERATE RISK: "shutdown" (no trailing space) matches inside filenames like "shutdown_graceful.py"
    LOW RISK: "rm " could match inside "inform ", "firmware " (rare in practice)
    LOW RISK: "kill " could match inside "skill " (rare in practice)
  implication: The fix must use word-boundary-aware matching. Patterns represent commands, so they should only match at command position (start of string, or after a command separator)

## Resolution

root_cause: `IsDangerous` uses naive `strings.Contains` for pattern matching. The pattern "dd " matches as a substring inside "add " (and similar words). The function has no concept of word boundaries or command position.
fix: Replaced naive `strings.Contains` with command-boundary-aware matching. Introduced `dangerousPattern` struct with `commandBoundary` flag. Patterns representing commands (rm, dd, kill, sudo, reboot, etc.) require boundary matching — they must appear at start of string or after a shell separator (|, ;, &&, ||, $(), backtick, xargs). Operator patterns (>, >>) retain simple substring matching. Added 43 new test cases covering false-positive prevention and true-positive preservation.
verification: All 53 IsDangerous tests pass. Full test suite passes with zero regressions. Key verified behaviors: `git add .` no longer flagged, `dd if=/dev/zero of=/dev/sda` still flagged, dd after pipe/semicolon/&&/||/$() still flagged, `inform`, `pseudo`, `adapt`, `skill`, `overkill` no longer false-positive, `reboot`/`shutdown` only match at command position not in filenames.
files_changed: [internal/tool/safety.go, internal/tool/safety_test.go]
