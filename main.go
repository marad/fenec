package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	feneclua "github.com/marad/fenec/internal/lua"
	"github.com/marad/fenec/internal/render"
	"github.com/marad/fenec/internal/repl"
	"github.com/marad/fenec/internal/session"
	"github.com/marad/fenec/internal/tool"
)

func main() {
	// Parse flags (per D-16: --host flag to override default).
	host := flag.String("host", "", "Ollama server address (default: localhost:11434)")
	flag.Parse()

	// Determine host.
	ollamaHost := config.DefaultHost
	if *host != "" {
		ollamaHost = *host
	}

	// Create Ollama client.
	client, err := chat.NewClient(ollamaHost)
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to create client: %v", err)))
		os.Exit(1)
	}

	// Health check (per D-14: if Ollama unreachable, show error with fix instructions and exit).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Cannot connect to Ollama at %s. Is it running? Start with: ollama serve\n\nDetails: %v", ollamaHost, err)))
		os.Exit(1)
	}

	// Get available models and select first (per D-09).
	models, err := client.ListModels(ctx)
	if err != nil || len(models) == 0 {
		fmt.Fprintln(os.Stderr, render.FormatError(
			"No models available. Pull one with: ollama pull gemma4"))
		os.Exit(1)
	}
	defaultModel := models[0]

	// Load system prompt (per D-15).
	systemPrompt, err := config.LoadSystemPrompt()
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to load system prompt: %v", err)))
		os.Exit(1)
	}

	// Query model's context window size.
	ctxLen, err := client.GetContextLength(ctx, defaultModel)
	if err != nil {
		// Non-fatal: use fallback.
		ctxLen = 4096
	}

	// Create context tracker (85% threshold triggers truncation).
	tracker := chat.NewContextTracker(ctxLen, 0.85)

	// Create session store.
	sessDir, err := config.SessionDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to create session directory: %v", err)))
		os.Exit(1)
	}
	store := session.NewStore(sessDir)

	// Create tool registry with shell_exec tool.
	registry := tool.NewRegistry()

	// Create approval function that will be set after REPL creation.
	// We need the REPL instance for readline access, so use a closure.
	var approver tool.ApproverFunc

	shellTool := tool.NewShellTool(30*time.Second, func(command string) bool {
		if approver != nil {
			return approver(command)
		}
		return false
	})
	registry.Register(shellTool)

	// Load Lua tools from tools directory.
	toolsDir, err := config.ToolsDir()
	if err != nil {
		slog.Warn("failed to resolve tools directory", "error", err)
	} else {
		result, err := feneclua.LoadTools(toolsDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(
				fmt.Sprintf("Failed to scan tools directory: %v", err)))
			// Non-fatal: continue without Lua tools.
		} else {
			for _, t := range result.Tools {
				registry.RegisterLua(t)
			}
			if len(result.Tools) > 0 {
				slog.Info("loaded Lua tools", "count", len(result.Tools))
			}
			for _, e := range result.Errors {
				fmt.Fprintln(os.Stderr, render.FormatError(
					fmt.Sprintf("Lua tool load error: %s", e.Error())))
			}
		}
	}

	// Create self-extension tool notifier.
	// Uses replRef closure to refresh system prompt after REPL creation.
	var replRef *repl.REPL
	notifier := func(event, name, desc string) {
		fmt.Fprintln(os.Stdout, render.FormatToolEvent(event, name, desc))
		if replRef != nil {
			replRef.RefreshSystemPrompt()
		}
	}

	// Register self-extension tools (built-in, alongside shell_exec).
	createTool := tool.NewCreateLuaTool(toolsDir, registry, notifier)
	registry.Register(createTool)
	updateTool := tool.NewUpdateLuaTool(toolsDir, registry, notifier)
	registry.Register(updateTool)
	deleteTool := tool.NewDeleteLuaTool(toolsDir, registry, notifier)
	registry.Register(deleteTool)

	// Create and run REPL.
	r, err := repl.NewREPL(client, defaultModel, systemPrompt, tracker, store, registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to start REPL: %v", err)))
		os.Exit(1)
	}
	defer r.Close()

	// Wire the approval function and REPL reference now that REPL is created.
	approver = r.ApproveCommand
	replRef = r

	// Check for auto-saved session.
	if _, autoErr := store.LoadAutoSave(); autoErr == nil {
		fmt.Fprintln(os.Stdout, "Previous session auto-saved. Type /load to resume it.")
	}

	if err := r.Run(); err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
		os.Exit(1)
	}
}
