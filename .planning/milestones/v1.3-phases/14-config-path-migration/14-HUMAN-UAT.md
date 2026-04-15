---
status: partial
phase: 14-config-path-migration
source: [14-VERIFICATION.md]
started: 2026-04-15T06:45:00Z
updated: 2026-04-15T06:45:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. End-to-End Migration on macOS
expected: Create test files at `~/Library/Application Support/fenec/`, remove `~/.config/fenec`, run binary. Legacy dir disappears, new dir has all files, stderr shows migration message with source and destination paths.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
