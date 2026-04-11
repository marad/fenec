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
	"time"

	"github.com/chzyer/readline"

	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	"github.com/marad/fenec/internal/render"
	"github.com/marad/fenec/internal/session"
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
	tracker   *chat.ContextTracker // Context window tracking
	store     *session.Store       // Session persistence
	session   *session.Session     // Current session
	autoSaved sync.Once            // Ensures auto-save runs only once
}

// NewREPL creates a REPL connected to the given chat service.
func NewREPL(client chat.ChatService, model string, systemPrompt string, tracker *chat.ContextTracker, store *session.Store) (*REPL, error) {
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

	// Set context length from tracker if available.
	if tracker != nil {
		conv.ContextLength = tracker.Available()
	}

	sess := session.NewSession(model)

	r := &REPL{
		client:  client,
		conv:    conv,
		rl:      rl,
		sigCh:   make(chan os.Signal, 1),
		tracker: tracker,
		store:   store,
		session: sess,
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
	defer r.autoSave()

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
			case "/save":
				r.handleSaveCommand()
			case "/load":
				r.handleLoadCommand()
			case "/history":
				r.handleHistoryCommand()
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
	r.autoSave()
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
	msg, metrics, err := r.client.StreamChat(ctx, r.conv, func(token string) {
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

	// Update context tracking and handle truncation.
	if r.tracker != nil && metrics != nil {
		r.tracker.Update(metrics.PromptEvalCount, metrics.EvalCount)

		if r.tracker.ShouldTruncate() {
			removed := r.tracker.TruncateOldest(r.conv)
			if removed > 0 {
				fmt.Fprintf(r.rl.Stdout(), "\n[context: dropped %d oldest messages to stay within %d token limit]\n",
					removed, r.tracker.Available())
			}
		}
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

// autoSave persists the current session to the auto-save file.
// Uses sync.Once to ensure only a single save occurs, even if called from
// both defer in Run() and Close().
func (r *REPL) autoSave() {
	r.autoSaved.Do(func() {
		if r.store == nil || r.session == nil {
			return
		}
		// Sync conversation messages to session.
		r.session.Messages = r.conv.Messages
		r.session.UpdatedAt = time.Now()
		if r.tracker != nil {
			r.session.TokenCount = r.tracker.TokenUsage()
		}
		if err := r.store.AutoSave(r.session); err != nil {
			// Best effort -- log but don't fail exit.
			fmt.Fprintf(os.Stderr, "auto-save failed: %v\n", err)
		}
	})
}

// handleSaveCommand persists the current conversation to a named session file.
func (r *REPL) handleSaveCommand() {
	if r.store == nil {
		fmt.Fprintln(r.rl.Stdout(), "Session storage not available.")
		return
	}
	r.session.Messages = r.conv.Messages
	r.session.UpdatedAt = time.Now()
	if r.tracker != nil {
		r.session.TokenCount = r.tracker.TokenUsage()
	}
	if err := r.store.Save(r.session); err != nil {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Save failed: %v", err)))
		return
	}
	fmt.Fprintf(r.rl.Stdout(), "Session saved: %s (%d messages)\n", r.session.ID, len(r.session.Messages))
}

// handleLoadCommand lists saved sessions and lets the user select one to restore.
func (r *REPL) handleLoadCommand() {
	if r.store == nil {
		fmt.Fprintln(r.rl.Stdout(), "Session storage not available.")
		return
	}

	sessions, err := r.store.List()
	if err != nil {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Failed to list sessions: %v", err)))
		return
	}

	// Include auto-save if it exists.
	autoSave, autoErr := r.store.LoadAutoSave()
	hasAutoSave := autoErr == nil && autoSave != nil

	if len(sessions) == 0 && !hasAutoSave {
		fmt.Fprintln(r.rl.Stdout(), "No saved sessions found.")
		return
	}

	fmt.Fprintln(r.rl.Stdout(), "Saved sessions:")
	if hasAutoSave {
		fmt.Fprintf(r.rl.Stdout(), "  0. [auto-save] %s (%d messages, %s)\n",
			autoSave.Model, len(autoSave.Messages), autoSave.UpdatedAt.Format("2006-01-02 15:04"))
	}
	for i, s := range sessions {
		fmt.Fprintf(r.rl.Stdout(), "  %d. %s - %s (%d messages, %s)\n",
			i+1, s.ID, s.Model, s.MessageCount, s.UpdatedAt.Format("2006-01-02 15:04"))
	}

	// Read selection.
	origPrompt := r.rl.Config.Prompt
	maxNum := len(sessions)
	minNum := 1
	if hasAutoSave {
		minNum = 0
	}
	r.rl.SetPrompt(fmt.Sprintf("Select session [%d-%d]: ", minNum, maxNum))

	selection, err := r.rl.Readline()
	r.rl.SetPrompt(origPrompt)
	if err != nil {
		return
	}

	selection = strings.TrimSpace(selection)
	if selection == "" {
		return
	}

	num, err := strconv.Atoi(selection)
	if err != nil || num < minNum || num > maxNum {
		fmt.Fprintf(r.rl.Stdout(), "Invalid selection: %s\n", selection)
		return
	}

	var loaded *session.Session
	if num == 0 && hasAutoSave {
		loaded = autoSave
	} else {
		loaded, err = r.store.Load(sessions[num-1].ID)
		if err != nil {
			fmt.Fprintln(r.rl.Stdout(), render.FormatError(fmt.Sprintf("Failed to load session: %v", err)))
			return
		}
	}

	// Restore conversation state.
	r.conv.Messages = loaded.Messages
	r.conv.SetModel(loaded.Model)
	r.session = loaded
	r.rl.SetPrompt(render.FormatPrompt(loaded.Model))
	fmt.Fprintf(r.rl.Stdout(), "Loaded session %s (%d messages, model: %s)\n",
		loaded.ID, len(loaded.Messages), loaded.Model)
}

// handleHistoryCommand displays conversation statistics.
func (r *REPL) handleHistoryCommand() {
	msgCount := len(r.conv.Messages)
	fmt.Fprintf(r.rl.Stdout(), "Messages: %d\n", msgCount)
	if r.tracker != nil {
		fmt.Fprintf(r.rl.Stdout(), "Tokens used: %d / %d (%.0f%%)\n",
			r.tracker.TokenUsage(), r.tracker.Available(),
			float64(r.tracker.TokenUsage())/float64(r.tracker.Available())*100)
	}
	fmt.Fprintf(r.rl.Stdout(), "Session: %s\n", r.session.ID)
}
