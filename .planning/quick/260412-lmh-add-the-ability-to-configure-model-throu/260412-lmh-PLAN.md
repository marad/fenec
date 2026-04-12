---
phase: quick-260412-lmh
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - main.go
  - main_test.go
autonomous: true
requirements: [CLI-MODEL-FLAG]
must_haves:
  truths:
    - "User can specify a model via --model or -m flag"
    - "When --model is given, fenec uses that model instead of auto-selecting the first available"
    - "When --model specifies a model not available in Ollama, fenec exits with a clear error and lists available models"
    - "When --model is omitted, existing behavior is preserved (first available model selected)"
  artifacts:
    - path: "main.go"
      provides: "--model / -m flag parsing and model selection logic"
  key_links:
    - from: "main.go"
      to: "internal/repl/repl.go"
      via: "defaultModel passed to NewREPL"
      pattern: "repl.NewREPL.*defaultModel"
---

<objective>
Add a --model (-m) CLI flag that lets the user specify which Ollama model to use for the chat session.

Purpose: Currently fenec auto-selects the first model from `ollama list`. Users with multiple models installed need a way to pick a specific one without using the interactive `/model` command after startup.

Output: Updated main.go with --model flag, validation against available models, and clear error messages.
</objective>

<context>
@main.go
@internal/config/config.go
@internal/chat/client.go
@internal/repl/repl.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add --model flag with validation</name>
  <files>main.go</files>
  <action>
Add a new pflag to main.go:

```go
modelName := pflag.StringP("model", "m", "", "Ollama model to use (default: first available)")
```

Add it after the existing `host` flag definition (line 24 area) to keep flags logically grouped.

Update the Usage function to include the new flag in the examples section. Add an example like:
```
  fenec --model gemma4    Use a specific model
```

After the existing model selection block (lines 83-89 where `defaultModel := models[0]`), add model flag handling:

1. If `*modelName` is not empty:
   - Check if the specified model exists in the `models` slice
   - If found, use it as `defaultModel`
   - If NOT found, print an error to stderr that includes: the requested model name, and a list of available models. Then exit with code 1. Format: "Model "X" not found. Available models:\n  - model1\n  - model2\n\nPull it with: ollama pull X"
2. If `*modelName` is empty, keep existing behavior (`defaultModel = models[0]`)

The validation should be case-sensitive since Ollama model names are case-sensitive.
  </action>
  <verify>
    <automated>cd /home/marad/dev/fenec && go build -o /dev/null . && echo "BUILD OK"</automated>
  </verify>
  <done>
  - `fenec --model gemma4` starts with gemma4 model
  - `fenec -m gemma4` works as shorthand
  - `fenec --model nonexistent` exits with error listing available models
  - `fenec` without --model still picks first available model
  - `fenec --help` shows the new --model flag
  </done>
</task>

</tasks>

<verification>
- `go build ./...` compiles without errors
- `fenec --help` shows --model flag with description
- Manual test: `fenec --model <valid-model>` starts chat with that model shown in prompt
- Manual test: `fenec --model nonexistent-model-xyz` exits with clear error listing available models
</verification>

<success_criteria>
The --model / -m flag is functional: valid model names are accepted and used, invalid names produce a helpful error with the list of available models, and omitting the flag preserves the existing auto-select behavior.
</success_criteria>

<output>
After completion, create `.planning/quick/260412-lmh-add-the-ability-to-configure-model-throu/260412-lmh-SUMMARY.md`
</output>
