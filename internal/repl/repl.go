package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"github.com/chzyer/readline"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	"github.com/marad/fenec/internal/render"
)

// REPL manages the interactive chat loop.
type REPL struct {
	client    chat.ChatService
	conv      *chat.Conversation
	rl        *readline.Instance
	mu        sync.Mutex         // Protects streaming state
	streaming bool               // True while streaming a response
	cancelFn  context.CancelFunc // For cancelling active generation via Ctrl+C
	sigCh     chan os.Signal      // SIGINT channel for cleanup
}

// NewREPL creates a REPL connected to the given chat service.
func NewREPL(client chat.ChatService, model string, systemPrompt string) (*REPL, error) {
	historyFile, err := config.HistoryFile()
	if err != nil {
		// Non-fatal: proceed without history.
		historyFile = ""
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          render.FormatPrompt(model),
		HistoryFile:     historyFile,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, fmt.Errorf("creating readline: %w", err)
	}

	conv := chat.NewConversation(model, systemPrompt)

	r := &REPL{
		client: client,
		conv:   conv,
		rl:     rl,
		sigCh:  make(chan os.Signal, 1),
	}

	// Ctrl+C / SIGINT handling (per D-04).
	// When streaming, cancel the active generation.
	// When not streaming, readline's default InterruptPrompt handles it (clears input).
	signal.Notify(r.sigCh, os.Interrupt)
	go func() {
		for range r.sigCh {
			r.mu.Lock()
			if r.streaming && r.cancelFn != nil {
				r.cancelFn()
			}
			r.mu.Unlock()
		}
	}()

	return r, nil
}

// Run starts the interactive REPL loop. Blocks until exit.
func (r *REPL) Run() error {
	// Print startup banner (per D-13).
	fmt.Fprintln(r.rl.Stdout(), render.FormatBanner(config.Version))
	// Blank line separator (per D-07).
	fmt.Fprintln(r.rl.Stdout())

	for {
		line, err := r.rl.Readline()
		if err == readline.ErrInterrupt {
			// Per D-04: Ctrl+C clears input when not streaming.
			continue
		}
		if err == io.EOF {
			// Per D-04: Ctrl+D exits.
			return nil
		}
		if err != nil {
			return fmt.Errorf("readline error: %w", err)
		}

		// Handle multi-line input (per D-02).
		input := r.handleMultiLine(line)

		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			continue
		}

		// Check for slash commands (per D-03).
		if IsCommand(trimmed) {
			cmd := ParseCommand(trimmed)
			if cmd == nil {
				continue
			}
			switch cmd.Name {
			case "/quit":
				return nil
			case "/help":
				fmt.Fprintln(r.rl.Stdout(), helpText)
			case "/model":
				r.handleModelCommand()
			default:
				fmt.Fprintf(r.rl.Stdout(), "Unknown command: %s. Type /help for available commands.\n", cmd.Name)
			}
			continue
		}

		// Send message to model.
		r.sendMessage(input)

		// Blank line separator (per D-07).
		fmt.Fprintln(r.rl.Stdout())
	}
}

// Close cleans up REPL resources.
func (r *REPL) Close() {
	signal.Stop(r.sigCh)
	close(r.sigCh)
	r.rl.Close()
}

// handleMultiLine accumulates lines ending with backslash continuation (per D-02).
func (r *REPL) handleMultiLine(firstLine string) string {
	line := firstLine
	if !isContinuation(line) {
		return line
	}

	var buf strings.Builder
	buf.WriteString(strings.TrimSuffix(strings.TrimSpace(line), "\\"))
	buf.WriteString("\n")

	// Set continuation prompt.
	origPrompt := r.rl.Config.Prompt
	r.rl.SetPrompt("... ")

	for {
		next, err := r.rl.Readline()
		if err != nil {
			break
		}
		if isContinuation(next) {
			buf.WriteString(strings.TrimSuffix(strings.TrimSpace(next), "\\"))
			buf.WriteString("\n")
		} else {
			buf.WriteString(next)
			break
		}
	}

	// Restore original prompt.
	r.rl.SetPrompt(origPrompt)
	return buf.String()
}

// isContinuation returns true if the line ends with a backslash (continuation marker).
func isContinuation(line string) bool {
	return strings.HasSuffix(strings.TrimSpace(line), "\\")
}

// sendMessage sends user input to the model and handles streaming output.
func (r *REPL) sendMessage(input string) {
	r.conv.AddUser(input)

	// Create cancellable context for this generation.
	ctx, cancel := context.WithCancel(context.Background())

	r.mu.Lock()
	r.streaming = true
	r.cancelFn = cancel
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.streaming = false
		r.cancelFn = nil
		r.mu.Unlock()
		cancel()
	}()

	// Start thinking spinner (per D-05).
	sp := render.NewSpinner(r.rl.Stdout())
	sp.Start()

	// Use FirstTokenNotifier to stop spinner on first token.
	var content strings.Builder
	notifier := chat.NewFirstTokenNotifier(func() {
		sp.Stop()
	})

	// Stream the response — tokens print directly as they arrive.
	msg, err := r.client.StreamChat(ctx, r.conv, func(token string) {
		notifier.Notify()
		fmt.Fprint(r.rl.Stdout(), token)
		content.WriteString(token)
	})

	// Ensure spinner is stopped (in case no tokens arrived).
	sp.Stop()

	if err != nil {
		if ctx.Err() == context.Canceled {
			fmt.Fprintln(r.rl.Stdout(), "\n[generation cancelled]")
			if msg != nil && msg.Content != "" {
				r.conv.AddAssistant(msg.Content)
			}
			return
		}
		fmt.Fprintln(r.rl.Stdout(), render.FormatError(err.Error()))
		return
	}

	if content.Len() > 0 {
		r.conv.AddAssistant(content.String())
	}
}

// handleModelCommand implements the /model interactive selection (per D-09, D-10).
func (r *REPL) handleModelCommand() {
	ctx := context.Background()
	models, err := r.client.ListModels(ctx)
	if err != nil {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Failed to list models: %v", err)))
		return
	}

	if len(models) == 0 {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError("No models available. Pull one with: ollama pull gemma4"))
		return
	}

	// Print numbered list with current model marked.
	fmt.Fprintln(r.rl.Stdout(), "Available models:")
	currentModel := r.conv.Model
	for i, m := range models {
		marker := "  "
		if m == currentModel {
			marker = "* "
		}
		fmt.Fprintf(r.rl.Stdout(), "  %s%d. %s\n", marker, i+1, m)
	}

	// Read selection.
	origPrompt := r.rl.Config.Prompt
	r.rl.SetPrompt(fmt.Sprintf("Select model [1-%d]: ", len(models)))

	selection, err := r.rl.Readline()
	r.rl.SetPrompt(origPrompt)

	if err != nil {
		return // User cancelled with Ctrl+C or Ctrl+D.
	}

	selection = strings.TrimSpace(selection)
	if selection == "" {
		return
	}

	num, err := strconv.Atoi(selection)
	if err != nil || num < 1 || num > len(models) {
		fmt.Fprintf(r.rl.Stdout(), "Invalid selection: %s. Enter a number between 1 and %d.\n", selection, len(models))
		return
	}

	selectedModel := models[num-1]
	r.conv.SetModel(selectedModel) // Per D-11: history preserved.
	r.rl.SetPrompt(render.FormatPrompt(selectedModel))
	fmt.Fprintf(r.rl.Stdout(), "Switched to %s\n", selectedModel)
}
