package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	pflag "github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	feneclua "github.com/marad/fenec/internal/lua"
	"github.com/marad/fenec/internal/provider"
	"github.com/marad/fenec/internal/render"
	"github.com/marad/fenec/internal/repl"
	"github.com/marad/fenec/internal/session"
	"github.com/marad/fenec/internal/tool"
)

func main() {
	// Parse flags.
	modelName := pflag.StringP("model", "m", "", "Model to use (provider/model or just model name)")
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
  fenec -m ollama/gemma4   Use a specific provider and model
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

	// Load or create config file.
	configDir, err := config.ConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to resolve config directory: %v", err)))
		os.Exit(1)
	}
	configPath := filepath.Join(configDir, "config.toml")
	cfg, err := config.LoadOrCreateConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Failed to load config: %v", err)))
		os.Exit(1)
	}

	// Build provider registry from config.
	providerRegistry := config.NewProviderRegistry()
	for name, pc := range cfg.Providers {
		p, err := config.CreateProvider(name, pc)
		if err != nil {
			slog.Error("failed to create provider", "name", name, "error", err)
			continue
		}
		providerRegistry.Register(name, p)
	}
	providerRegistry.SetDefault(cfg.DefaultProvider)

	// Start config file watcher for hot-reload (CONF-04).
	configWatcher, err := config.NewConfigWatcher(configPath, func() {
		newCfg, err := config.LoadConfig(configPath)
		if err != nil {
			slog.Error("config reload failed, keeping old config", "error", err)
			return
		}
		// Rebuild all providers from new config.
		newProviders := make(map[string]provider.Provider)
		for name, pc := range newCfg.Providers {
			newP, createErr := config.CreateProvider(name, pc)
			if createErr != nil {
				slog.Error("failed to create provider on reload", "name", name, "error", createErr)
				continue
			}
			newProviders[name] = newP
		}
		providerRegistry.Update(newProviders, newCfg.DefaultProvider)
		slog.Info("config reloaded", "providers", len(newProviders))
	})
	if err != nil {
		// Non-fatal: hot-reload is a convenience, not critical.
		slog.Warn("config watcher failed to start, hot-reload disabled", "error", err)
	} else {
		defer configWatcher.Stop()
	}

	// Get default provider.
	p, err := providerRegistry.Default()
	if err != nil {
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("No default provider available: %v", err)))
		os.Exit(1)
	}
	activeProviderName := providerRegistry.DefaultName()

	// Handle --model flag: supports provider/model syntax for cross-provider targeting.
	var defaultModel string
	if *modelName != "" {
		if strings.Contains(*modelName, "/") {
			// provider/model syntax: resolve provider and model separately.
			parts := strings.SplitN(*modelName, "/", 2)
			providerName, modelPart := parts[0], parts[1]
			resolvedProvider, ok := providerRegistry.Get(providerName)
			if !ok {
				fmt.Fprintf(os.Stderr, "Unknown provider %q. Available providers: %s\n",
					providerName, strings.Join(providerRegistry.Names(), ", "))
				os.Exit(1)
			}
			p = resolvedProvider
			activeProviderName = providerName
			defaultModel = modelPart
		} else {
			// No prefix: use default provider with the given model name.
			defaultModel = *modelName
		}
	} else if cfg.DefaultModel != "" {
		// No --model flag but config has a default_model.
		defaultModel = cfg.DefaultModel
	}

	// Health check: if the selected provider is unreachable, show error and exit.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Ping(ctx); err != nil {
		providerURL := cfg.Providers[activeProviderName].URL
		fmt.Fprintln(os.Stderr, render.FormatError(
			fmt.Sprintf("Cannot connect to provider %q at %s. Is it running?\n\nDetails: %v", activeProviderName, providerURL, err)))
		os.Exit(1)
	}

	// If no model was specified, pick the first available from the provider.
	if defaultModel == "" {
		models, err := p.ListModels(ctx)
		if err != nil || len(models) == 0 {
			fmt.Fprintln(os.Stderr, render.FormatError(
				"No models available. Pull one with: ollama pull gemma4"))
			os.Exit(1)
		}
		defaultModel = models[0]
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
	toolRegistry := tool.NewRegistry()

	// Create approval function that will be set after REPL creation.
	// We need the REPL instance for readline access, so use a closure.
	var approver tool.ApproverFunc

	shellTool := tool.NewShellTool(30*time.Second, func(command string) bool {
		if approver != nil {
			return approver(command)
		}
		return false
	})
	toolRegistry.Register(shellTool)

	// Register file manipulation tools.
	readTool := tool.NewReadFileTool()
	toolRegistry.Register(readTool)

	writeTool := tool.NewWriteFileTool(func(desc string) bool {
		if approver != nil {
			return approver(desc)
		}
		return false
	})
	toolRegistry.Register(writeTool)

	editTool := tool.NewEditFileTool(func(desc string) bool {
		if approver != nil {
			return approver(desc)
		}
		return false
	})
	toolRegistry.Register(editTool)

	listDirTool := tool.NewListDirTool()
	toolRegistry.Register(listDirTool)

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
				toolRegistry.RegisterLua(t)
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
		createTool := tool.NewCreateLuaTool(toolsDir, toolRegistry, notifier)
		toolRegistry.Register(createTool)
		updateTool := tool.NewUpdateLuaTool(toolsDir, toolRegistry, notifier)
		toolRegistry.Register(updateTool)
		deleteTool := tool.NewDeleteLuaTool(toolsDir, toolRegistry, notifier)
		toolRegistry.Register(deleteTool)
	} else {
		slog.Warn("self-extension tools disabled: no tools directory available")
	}

	// Create and run REPL.
	r, err := repl.NewREPL(p, defaultModel, systemPrompt, tracker, store, toolRegistry)
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
