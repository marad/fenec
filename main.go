package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	pflag "github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	feneclua "github.com/marad/fenec/internal/lua"
	"github.com/marad/fenec/internal/provider/ollama"
	"github.com/marad/fenec/internal/render"
	"github.com/marad/fenec/internal/repl"
	"github.com/marad/fenec/internal/session"
	"github.com/marad/fenec/internal/tool"
)

func main() {
	// Parse flags (per D-16: --host flag to override default).
	host := pflag.StringP("host", "H", "", "Ollama server address (default: localhost:11434)")
	modelName := pflag.StringP("model", "m", "", "Ollama model to use (default: first available)")
	pipeMode := pflag.BoolP("pipe", "p", false, "Read all stdin as a single message and send to model")
	debugMode := pflag.BoolP("debug", "d", false, "Show tool call results and other debug output")
	yoloMode := pflag.BoolP("yolo", "y", false, "Auto-approve all dangerous commands (use with caution)")
	lineByLine := pflag.Bool("line-by-line", false, "In pipe mode, send each stdin line separately (default: batch)")
	showVersion := pflag.BoolP("version", "v", false, "Print version and exit")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `fenec - AI assistant powered by local Ollama models

Usage:
  fenec                    Start interactive chat
  fenec --model gemma4     Use a specific model
  echo "prompt" | fenec    Send piped input to model
  fenec --yolo             Auto-approve all tool commands

Flags:
`)
		pflag.PrintDefaults()
	}

	pflag.Parse()

	if *showVersion {
		fmt.Printf("fenec %s\n", config.Version)
		os.Exit(0)
	}

	// Detect whether stdin is a terminal (interactive) or a pipe/redirect.
	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	// Auto-enable pipe mode when stdin is not a terminal and --pipe was not explicitly set.
	if !interactive && !*pipeMode {
		*pipeMode = true
	}

	// Determine host.
	ollamaHost := config.DefaultHost
	if *host != "" {
		ollamaHost = *host
	}

	// Create Ollama provider.
	p, err := ollama.New(ollamaHost)
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to create provider: %v", err)))
		os.Exit(1)
	}

	// Health check (per D-14: if Ollama unreachable, show error with fix instructions and exit).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Ping(ctx); err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Cannot connect to Ollama at %s. Is it running? Start with: ollama serve\n\nDetails: %v", ollamaHost, err)))
		os.Exit(1)
	}

	// Get available models and select first (per D-09).
	models, err := p.ListModels(ctx)
	if err != nil || len(models) == 0 {
		fmt.Fprintln(os.Stderr, render.FormatError(
			"No models available. Pull one with: ollama pull gemma4"))
		os.Exit(1)
	}
	defaultModel := models[0]

	// Handle --model flag: validate and override default model selection.
	if *modelName != "" {
		found := false
		for _, m := range models {
			if m == *modelName {
				found = true
				defaultModel = m
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Model %q not found. Available models:\n", *modelName)
			for _, m := range models {
				fmt.Fprintf(os.Stderr, "  - %s\n", m)
			}
			fmt.Fprintf(os.Stderr, "\nPull it with: ollama pull %s\n", *modelName)
			os.Exit(1)
		}
	}

	// Load system prompt (per D-15).
	systemPrompt, err := config.LoadSystemPrompt()
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to load system prompt: %v", err)))
		os.Exit(1)
	}

	// Query model's context window size.
	ctxLen, err := p.GetContextLength(ctx, defaultModel)
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

	// Register file manipulation tools.
	readTool := tool.NewReadFileTool()
	registry.Register(readTool)

	writeTool := tool.NewWriteFileTool(func(desc string) bool {
		if approver != nil {
			return approver(desc)
		}
		return false
	})
	registry.Register(writeTool)

	editTool := tool.NewEditFileTool(func(desc string) bool {
		if approver != nil {
			return approver(desc)
		}
		return false
	})
	registry.Register(editTool)

	listDirTool := tool.NewListDirTool()
	registry.Register(listDirTool)

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

	// Register self-extension tools only when toolsDir is resolved.
	// Without a valid tools directory, create/update/delete would write to CWD.
	if toolsDir != "" {
		createTool := tool.NewCreateLuaTool(toolsDir, registry, notifier)
		registry.Register(createTool)
		updateTool := tool.NewUpdateLuaTool(toolsDir, registry, notifier)
		registry.Register(updateTool)
		deleteTool := tool.NewDeleteLuaTool(toolsDir, registry, notifier)
		registry.Register(deleteTool)
	} else {
		slog.Warn("self-extension tools disabled: no tools directory available")
	}

	// Create and run REPL.
	r, err := repl.NewREPL(p, defaultModel, systemPrompt, tracker, store, registry)
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to start REPL: %v", err)))
		os.Exit(1)
	}
	defer r.Close()

	// Wire the approval function and REPL reference now that REPL is created.
	if *yoloMode {
		// --yolo: auto-approve everything, works in both interactive and pipe modes.
		approver = func(command string) bool {
			fmt.Fprintf(os.Stderr, "[yolo] auto-approved: %s\n", command)
			return true
		}
	} else if !interactive {
		// Non-interactive (pipe) without --yolo: auto-deny with clear message.
		approver = func(command string) bool {
			fmt.Fprintf(os.Stderr, "[denied] %s — non-interactive mode. Use --yolo to auto-approve.\n", command)
			return false
		}
	} else {
		// Interactive terminal: prompt user as normal.
		approver = r.ApproveCommand
	}
	replRef = r
	r.SetDebug(*debugMode)

	// Enable thinking only in interactive mode — in pipe mode the input is
	// complete and thinking wastes the model's token budget on planning.
	if !*pipeMode {
		r.EnableThink()
	}

	// Pipe mode: read stdin line-by-line, send to model, exit on EOF.
	if *pipeMode {
		if err := r.RunPipe(os.Stdin, *lineByLine); err != nil {
			fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
			os.Exit(1)
		}
		return
	}

	// Check for auto-saved session.
	if _, autoErr := store.LoadAutoSave(); autoErr == nil {
		fmt.Fprintln(os.Stdout, "Previous session auto-saved. Type /load to resume it.")
	}

	if err := r.Run(); err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(err.Error()))
		os.Exit(1)
	}
}
