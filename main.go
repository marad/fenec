package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
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

	// Create and run REPL.
	r, err := repl.NewREPL(client, defaultModel, systemPrompt, tracker, store, registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to start REPL: %v", err)))
		os.Exit(1)
	}
	defer r.Close()

	// Wire the approval function now that REPL is created.
	approver = r.ApproveCommand

	// Check for auto-saved session.
	if _, autoErr := store.LoadAutoSave(); autoErr == nil {
		fmt.Fprintln(os.Stdout, "Previous session auto-saved. Type /load to resume it.")
	}

	if err := r.Run(); err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
		os.Exit(1)
	}
}
