---
phase: 07-canonical-types
plan: 01
subsystem: api
tags: [go, json, types, canonical, serialization]

# Dependency graph
requires: []
provides:
  - "Canonical Message, ToolCall, ToolCallFunction types (internal/model)"
  - "Canonical ToolDefinition, ToolFunction, ToolFunctionParameters, ToolProperty, PropertyType types"
  - "Canonical StreamMetrics type"
  - "PropertyType custom JSON marshal/unmarshal (single-string and array forms)"
affects: [07-02-canonical-types, 08-provider-abstraction]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Canonical types with zero external dependencies", "Custom JSON marshal for polymorphic type representation"]

key-files:
  created:
    - internal/model/message.go
    - internal/model/tool.go
    - internal/model/metrics.go
    - internal/model/message_test.go
    - internal/model/tool_test.go
    - internal/model/metrics_test.go
  modified: []

key-decisions:
  - "Used map[string]ToolProperty for Properties instead of ordered map -- simpler, sufficient for JSON schema"
  - "Used map[string]any for ToolCallFunction.Arguments instead of ToolCallFunctionArguments ordered type"
  - "Omitted Images and ToolName fields from Message -- Fenec never uses them"

patterns-established:
  - "internal/model as the canonical types package with zero external dependencies"
  - "TDD for type definitions: failing JSON tests first, then implementation"

requirements-completed: [PROV-03]

# Metrics
duration: 2min
completed: 2026-04-12
---

# Phase 07 Plan 01: Canonical Types Summary

**Fenec-owned Message, ToolDefinition, and StreamMetrics types with PropertyType custom JSON marshaling and full round-trip test coverage**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-12T18:54:47Z
- **Completed:** 2026-04-12T18:56:41Z
- **Tasks:** 1
- **Files modified:** 6

## Accomplishments
- Created `internal/model` package with canonical types decoupled from `ollama/api`
- Implemented PropertyType custom JSON marshaling: single-element as bare string, multi-element as array
- Full JSON round-trip test coverage with 16 tests covering all serialization behaviors
- Zero external dependencies in the model package (only `encoding/json` from stdlib)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create canonical types in internal/model package** (TDD)
   - RED: `f6a003e` (test: add failing tests for canonical model types)
   - GREEN: `17da4c0` (feat: implement canonical model types in internal/model)

## Files Created/Modified
- `internal/model/message.go` - Message, ToolCall, ToolCallFunction types
- `internal/model/tool.go` - ToolDefinition, ToolFunction, ToolFunctionParameters, ToolProperty, PropertyType types with custom JSON marshal
- `internal/model/metrics.go` - StreamMetrics type
- `internal/model/message_test.go` - 5 tests for Message JSON round-trip and omitempty
- `internal/model/tool_test.go` - 7 tests for PropertyType marshal/unmarshal and ToolDefinition JSON
- `internal/model/metrics_test.go` - 3 tests for StreamMetrics JSON round-trip and omitempty

## Decisions Made
- Used `map[string]ToolProperty` for Properties instead of Ollama's `*ToolPropertiesMap` ordered map -- a regular map is simpler and sufficient since JSON schema property order does not matter
- Used `map[string]any` for ToolCallFunction.Arguments instead of Ollama's `ToolCallFunctionArguments` ordered type -- standard Go map provides the same functionality for argument access
- Omitted `Images []ImageData` and `ToolName string` from Message -- Fenec never uses image input or the tool name field

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Canonical types ready for Phase 07 Plan 02: migrating all Ollama type references to use `internal/model`
- All type shapes match the JSON wire format of Ollama API types for seamless adapter conversion
- PropertyType custom marshaling ensures tool definitions serialize identically to Ollama format

## Self-Check: PASSED

All 6 files verified present. Both commit hashes (f6a003e, 17da4c0) found in git log.

---
*Phase: 07-canonical-types*
*Completed: 2026-04-12*
