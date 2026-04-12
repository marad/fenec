---
status: testing
phase: 05-self-extension
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md]
started: 2026-04-11T18:30:00Z
updated: 2026-04-11T20:35:00Z
---

## Current Test

number: 6
name: Validation errors shown to model for self-correction
expected: |
  When create_lua_tool receives invalid Lua, the error includes line numbers
  or descriptive messages. The model can see the error and retry with corrected code.
awaiting: user response

## Tests

### 1. /tools command lists loaded tools
expected: Run fenec. Type `/tools`. Output shows all registered tools sorted by name, each tagged `[built-in]` or `[lua]`. Built-in tools include at least: create_lua_tool, delete_lua_tool, shell_exec, update_lua_tool. Any Lua tools loaded from ~/.fenec/tools/ show as `[lua]`.
result: pass

### 2. Create a Lua tool via conversation
expected: Ask the model to create a simple tool (e.g., "create a tool that counts words in text"). The model calls create_lua_tool. A muted blue banner appears: "New tool registered: word_count" (or similar). The file appears in ~/.fenec/tools/. `/tools` now shows it tagged `[lua]`.
result: pass
note: Required streaming tool call fix (361c74c). After fix, model successfully creates tools. First attempt often fails validation but model retries and succeeds — banner appears, tool persists to disk.

### 3. Hot-reload — new tool available immediately
expected: After creating a tool in test 2, ask the model to USE it in the same session (e.g., "count the words in 'hello world'"). The model calls the newly created tool and returns a result. No restart needed.
result: pass
note: Verified via pipe mode. Tool was called immediately after creation. Lua runtime error in the tool (model-authored Lua was buggy) but the tool WAS dispatched — hot-reload works. Model handled error gracefully.

### 4. Update a Lua tool via conversation
expected: Ask the model to update the tool from test 2 (e.g., "update word_count to also return character count"). The model calls update_lua_tool. A banner appears: "Tool updated: word_count". The tool file on disk is replaced. Using the tool reflects the new behavior.
result: skipped
reason: gemma4:e2b (5B) struggles to write valid Lua consistently. Update path tested via unit tests (7 tests passing). Needs a larger model for reliable E2E.

### 5. Delete a Lua tool via conversation
expected: Ask the model to delete the tool (e.g., "delete the word_count tool"). The model calls delete_lua_tool. A banner appears: "Tool removed: word_count". `/tools` no longer lists it. The file is removed from ~/.fenec/tools/.
result: pass
note: Verified via pipe mode. Banner "Tool removed: greet" displayed. `/tools` confirmed tool gone. File removed from disk.

### 6. Validation errors shown to model for self-correction
expected: When create_lua_tool receives invalid Lua, the error includes line numbers or descriptive messages. The model can see the error and retry with corrected code.
result: pass
note: Verified via pipe mode. First attempt returned {"error":"validation error: ...script must return a table, got nil"}. Model saw error, retried with corrected Lua, succeeded on second attempt. Self-correction loop works.

## Summary

total: 6
passed: 5
issues: 0
pending: 0
skipped: 1
blocked: 0

## Gaps
