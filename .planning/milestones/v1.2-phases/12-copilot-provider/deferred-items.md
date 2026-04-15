# Deferred Items — Phase 12 Copilot Provider

## Pre-existing Test Failures

### TestLoadSystemPromptFromFile (internal/config/config_test.go:42)

**Discovered during:** 12-02 Task 3 verification
**Description:** Test expects `LoadSystemPrompt` to return file contents ("Custom prompt") but gets the hardcoded default system prompt. The function appears to read a real `~/.config/fenec/system_prompt.txt` file that overrides the temp file path set in the test.
**Impact:** Unrelated to copilot provider. Does not affect any copilot functionality.
**Action:** Fix in a config-focused plan or quick task.
