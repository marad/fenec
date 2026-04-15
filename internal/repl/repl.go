package repl

import (
	"bufio"
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
	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	"github.com/marad/fenec/internal/render"
	"github.com/marad/fenec/internal/session"
	"github.com/marad/fenec/internal/tool"
)

// REPL manages the interactive chat loop.
type REPL struct {
	provider         provider.Provider
	providerRegistry *config.ProviderRegistry
	activeProvider   string
	conv             *chat.Conversation
	rl               *readline.Instance
	mu               sync.Mutex           // Protects streaming state
	streaming        bool                 // True while streaming a response
	cancelFn         context.CancelFunc   // For cancelling active generation via Ctrl+C
	sigCh            chan os.Signal        // SIGINT channel for cleanup
	tracker          *chat.ContextTracker // Context window tracking
	store            *session.Store       // Session persistence
	session          *session.Session     // Current session
	autoSaved        sync.Once            // Ensures auto-save runs only once
	registry         *tool.Registry       // Tool registry for agentic loop
	baseSystemPrompt string               // System prompt before tool descriptions (for refresh)
	debug            bool                 // Show tool results when true
}

// NewREPL creates a REPL connected to the given chat service.
func NewREPL(p provider.Provider, model string, activeProvider string, systemPrompt string, tracker *chat.ContextTracker, store *session.Store, toolRegistry *tool.Registry, providerRegistry *config.ProviderRegistry) (*REPL, error) {
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

	// Store base system prompt before tool descriptions are appended.
	// Needed for refreshSystemPrompt after hot-reload events.
	basePrompt := systemPrompt

	// Append tool descriptions to system prompt so the model knows what tools are available.
	if toolRegistry != nil {
		toolDesc := toolRegistry.Describe()
		if toolDesc != "" {
			systemPrompt = systemPrompt + "\n\n## Available Tools\n\n" + toolDesc
		}
	}

	conv := chat.NewConversation(model, systemPrompt)

	// Set context length from tracker if available.
	if tracker != nil {
		conv.ContextLength = tracker.Available()
	}

	sess := session.NewSession(model)

	r := &REPL{
		provider:         p,
		providerRegistry: providerRegistry,
		activeProvider:   activeProvider,
		conv:             conv,
		rl:               rl,
		sigCh:            make(chan os.Signal, 1),
		tracker:          tracker,
		store:            store,
		session:          sess,
		registry:         toolRegistry,
		baseSystemPrompt: basePrompt,
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
				r.handleModelCommand(cmd.Args)
			case "/save":
				r.handleSaveCommand()
			case "/load":
				r.handleLoadCommand()
			case "/history":
				r.handleHistoryCommand()
			case "/tools":
				r.handleToolsCommand()
			case "/clear":
				r.handleClearCommand()
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

// readAllInput reads the entire content of r as a single string, trimming
// leading/trailing whitespace. Returns empty string if input is empty or
// whitespace-only.
func readAllInput(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// RunPipe reads from input and sends to the model, printing responses to stdout.
// When lineByLine is false (default), all stdin is read at once and sent as a
// single message. When lineByLine is true, each line is sent as a separate message.
// Exits on EOF.
func (r *REPL) RunPipe(input io.Reader, lineByLine bool) error {
	if !lineByLine {
		return r.runPipeBatch(input)
	}
	return r.runPipeLineByLine(input)
}

// runPipeBatch reads all of stdin at once and sends it as a single message.
func (r *REPL) runPipeBatch(input io.Reader) error {
	content, err := readAllInput(input)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	if content == "" {
		return nil
	}

	// Show a truncated preview of what was received.
	preview := content
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	fmt.Fprintf(r.rl.Stdout(), "> %s\n", preview)

	r.sendMessage(content)
	fmt.Fprintln(r.rl.Stdout())
	return nil
}

// runPipeLineByLine reads stdin line-by-line, sending each as a separate message.
func (r *REPL) runPipeLineByLine(input io.Reader) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/quit" {
			return nil
		}
		// Handle slash commands inline.
		if IsCommand(line) {
			cmd := ParseCommand(line)
			if cmd != nil {
				switch cmd.Name {
				case "/tools":
					r.handleToolsCommand()
				case "/help":
					fmt.Fprintln(r.rl.Stdout(), helpText)
				case "/history":
					r.handleHistoryCommand()
				default:
					fmt.Fprintf(r.rl.Stdout(), "Unsupported in pipe mode: %s\n", cmd.Name)
				}
			}
			continue
		}
		fmt.Fprintf(r.rl.Stdout(), "> %s\n", line)
		r.sendMessage(line)
		fmt.Fprintln(r.rl.Stdout())
	}
	return scanner.Err()
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

const maxToolRounds = 10

// sendMessage sends user input to the model and handles streaming output.
// Implements the agentic loop: when the model returns tool calls, dispatch each,
// feed results back, and re-send until the model responds with text only.
func (r *REPL) sendMessage(input string) {
	r.conv.AddUser(input)

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

	// Get tool definitions for ChatRequest (nil if no registry).
	var tools []model.ToolDefinition
	if r.registry != nil {
		tools = r.registry.Tools()
	}

	for round := 0; round < maxToolRounds; round++ {
		sp := render.NewSpinner(r.rl.Stdout())
		sp.Start()

		var content strings.Builder
		thinkingStarted := false
		contentStarted := false

		// Build provider request from conversation state.
		req := &provider.ChatRequest{
			Model:         r.conv.Model,
			Messages:      r.conv.Messages,
			Tools:         tools,
			Think:         r.conv.Think,
			ContextLength: r.conv.ContextLength,
		}

		// Stream the response.
		msg, metrics, err := r.provider.StreamChat(ctx, req, func(token string) {
			if !contentStarted {
				contentStarted = true
				sp.Stop()
				if thinkingStarted {
					fmt.Fprint(r.rl.Stdout(), "\n")
				}
			}
			fmt.Fprint(r.rl.Stdout(), token)
			content.WriteString(token)
		}, func(chunk string) {
			if !thinkingStarted {
				sp.Stop()
				fmt.Fprintln(r.rl.Stdout(), render.FormatThinkingLabel())
				thinkingStarted = true
			}
			fmt.Fprint(r.rl.Stdout(), render.FormatThinkingChunk(chunk))
		})

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

		// Check for tool calls.
		if len(msg.ToolCalls) == 0 {
			// No tool calls -- final text response.
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
			return
		}

		// Model made tool calls -- add assistant message (with ToolCalls) to history.
		r.conv.AddRawMessage(*msg)

		// Execute each tool call.
		for _, tc := range msg.ToolCalls {
			// Print muted tool call indicator.
			extra := ""
			if cmdVal, ok := tc.Function.Arguments["command"]; ok {
				extra = fmt.Sprintf(" %v", cmdVal)
			}
			fmt.Fprintf(r.rl.Stdout(), "\n%s\n", render.FormatToolCall(tc.Function.Name, extra))

			result, err := r.registry.Dispatch(ctx, tc)
			if err != nil {
				result = fmt.Sprintf(`{"error": %q}`, err.Error())
			}

			// Show tool result only in debug mode.
			if r.debug {
				fmt.Fprintf(r.rl.Stdout(), "%s\n", render.FormatToolResult(result))
			}

			// Add tool result to conversation.
			r.conv.AddToolResult(tc.ID, result)
		}

		// Update context tracking after tool round.
		if r.tracker != nil && metrics != nil {
			r.tracker.Update(metrics.PromptEvalCount, metrics.EvalCount)
		}

		// Loop back for next round.
	}

	// If we hit max rounds, force a text response by omitting tools.
	fmt.Fprintf(r.rl.Stdout(), "\n[max tool rounds (%d) reached, requesting summary]\n", maxToolRounds)
	r.conv.AddUser("Please summarize what you've done so far. Do not make any more tool calls.")

	sp2 := render.NewSpinner(r.rl.Stdout())
	sp2.Start()
	var content strings.Builder
	thinkingStarted2 := false
	contentStarted2 := false

	summaryReq := &provider.ChatRequest{
		Model:         r.conv.Model,
		Messages:      r.conv.Messages,
		Think:         r.conv.Think,
		ContextLength: r.conv.ContextLength,
	}
	msg, _, err := r.provider.StreamChat(ctx, summaryReq, func(token string) {
		if !contentStarted2 {
			contentStarted2 = true
			sp2.Stop()
			if thinkingStarted2 {
				fmt.Fprint(r.rl.Stdout(), "\n")
			}
		}
		fmt.Fprint(r.rl.Stdout(), token)
		content.WriteString(token)
	}, func(chunk string) {
		if !thinkingStarted2 {
			sp2.Stop()
			fmt.Fprintln(r.rl.Stdout(), render.FormatThinkingLabel())
			thinkingStarted2 = true
		}
		fmt.Fprint(r.rl.Stdout(), render.FormatThinkingChunk(chunk))
	})
	sp2.Stop()

	if err != nil {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError(err.Error()))
		return
	}
	if content.Len() > 0 {
		r.conv.AddAssistant(content.String())
	}
	_ = msg
}

// ApproveCommand prompts the user for Y/n confirmation of a dangerous command.
// Returns true if approved, false if denied.
func (r *REPL) ApproveCommand(command string) bool {
	fmt.Fprintf(r.rl.Stdout(), "[dangerous command] %s\n", command)

	origPrompt := r.rl.Config.Prompt
	r.rl.SetPrompt("Allow? [y/N]: ")

	response, err := r.rl.Readline()
	r.rl.SetPrompt(origPrompt)

	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// providerModels holds the result of listing models from a single provider.
type providerModels struct {
	name   string
	models []string
	err    error
}

// listModels fetches models from all registered providers in parallel and prints
// them grouped by provider, with the active model marked by an arrow.
func (r *REPL) listModels() {
	if r.providerRegistry == nil {
		fmt.Fprintln(r.rl.Stdout(), render.FormatError("Provider registry not available."))
		return
	}

	names := r.providerRegistry.Names()
	if len(names) == 0 {
		fmt.Fprintln(r.rl.Stdout(), "No providers configured.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := make([]providerModels, len(names))
	var wg sync.WaitGroup

	for i, name := range names {
		wg.Add(1)
		go func(idx int, providerName string) {
			defer wg.Done()
			results[idx].name = providerName
			p, ok := r.providerRegistry.Get(providerName)
			if !ok {
				results[idx].err = fmt.Errorf("provider not found")
				return
			}
			models, err := p.ListModels(ctx)
			results[idx].models = models
			results[idx].err = err
		}(i, name)
	}

	wg.Wait()

	for i, res := range results {
		fmt.Fprintln(r.rl.Stdout(), render.FormatProviderHeader(res.name))
		if res.err != nil {
			fmt.Fprintln(r.rl.Stdout(), render.FormatProviderError(res.name, res.err.Error()))
		} else if len(res.models) == 0 {
			fmt.Fprintln(r.rl.Stdout(), "  (no models)")
		} else {
			for _, m := range res.models {
				active := res.name == r.activeProvider && m == r.conv.Model
				fmt.Fprintln(r.rl.Stdout(), render.FormatModelEntry(m, active))
			}
		}
		if i < len(results)-1 {
			fmt.Fprintln(r.rl.Stdout())
		}
	}
}

// handleModelCommand implements the /model command.
// With no args: shows models grouped by provider across all registered providers.
// With args: switches provider and/or model based on "provider/model" or bare "model" syntax.
func (r *REPL) handleModelCommand(args []string) {
	if len(args) == 0 {
		r.listModels()
		return
	}

	target := args[0]
	if idx := strings.Index(target, "/"); idx != -1 {
		// "provider/model" syntax: switch both provider and model.
		parts := strings.SplitN(target, "/", 2)
		providerName, modelName := parts[0], parts[1]

		if r.providerRegistry == nil {
			fmt.Fprintln(r.rl.Stdout(), render.FormatError("Provider registry not available."))
			return
		}
		newProvider, ok := r.providerRegistry.Get(providerName)
		if !ok {
			fmt.Fprintf(r.rl.Stdout(), "Unknown provider: %s. Available: %s\n",
				providerName, strings.Join(r.providerRegistry.Names(), ", "))
			return
		}

		r.provider = newProvider
		r.activeProvider = providerName
		r.conv.SetModel(modelName)

		// Update context length from new provider (5s timeout matches listModels convention).
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if ctxLen, err := newProvider.GetContextLength(ctxTimeout, modelName); err == nil && ctxLen > 0 {
			r.conv.ContextLength = ctxLen
		}

		r.rl.SetPrompt(render.FormatPrompt(modelName))
		fmt.Fprintf(r.rl.Stdout(), "Switched to %s/%s\n", providerName, modelName)
	} else {
		// Bare model name: stay on current provider, switch model only.
		modelName := target
		r.conv.SetModel(modelName)

		// Update context length from current provider (5s timeout matches listModels convention).
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if ctxLen, err := r.provider.GetContextLength(ctxTimeout, modelName); err == nil && ctxLen > 0 {
			r.conv.ContextLength = ctxLen
		}

		r.rl.SetPrompt(render.FormatPrompt(modelName))
		fmt.Fprintf(r.rl.Stdout(), "Switched to %s\n", modelName)
	}
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

// handleClearCommand saves the current conversation (if non-empty) and resets
// to a fresh state. Implements /clear per CONV-01, CONV-02, CONV-03.
func (r *REPL) handleClearCommand() {
	saved := false
	var savedID string
	var msgCount int

	// Step 1: Save if conversation has user content (D-01, D-02).
	if r.store != nil && r.session != nil {
		r.session.Messages = r.conv.Messages
		r.session.UpdatedAt = time.Now()
		if r.tracker != nil {
			r.session.TokenCount = r.tracker.TokenUsage()
		}
		if r.session.HasContent() {
			if err := r.store.Save(r.session); err != nil {
				fmt.Fprintln(r.rl.Stdout(), render.FormatError(
					fmt.Sprintf("Save failed: %v", err)))
				// Continue with clear despite save failure — don't trap user.
			} else {
				saved = true
				savedID = r.session.ID
				msgCount = len(r.session.Messages)
			}
		}
	}

	// Capture flags that must survive the reset (Pitfall 1, Pitfall 3).
	thinkEnabled := r.conv.Think
	var contextLength int
	if r.tracker != nil {
		contextLength = r.tracker.Available()
	} else {
		contextLength = r.conv.ContextLength
	}

	// Step 2: Build full system prompt with tool descriptions (CONV-03).
	systemPrompt := r.baseSystemPrompt
	if r.registry != nil {
		toolDesc := r.registry.Describe()
		if toolDesc != "" {
			systemPrompt = systemPrompt + "\n\n## Available Tools\n\n" + toolDesc
		}
	}

	// Step 3: Create fresh conversation and session (D-03).
	r.conv = chat.NewConversation(r.conv.Model, systemPrompt)
	r.conv.Think = thinkEnabled
	r.conv.ContextLength = contextLength
	r.session = session.NewSession(r.conv.Model)

	// Step 4: Reset tracker and auto-save guard (D-04, Pitfall 2).
	if r.tracker != nil {
		r.tracker.Reset()
	}
	r.autoSaved = sync.Once{}

	// Step 5: User feedback (D-05, D-06).
	if saved {
		fmt.Fprintf(r.rl.Stdout(), "Conversation saved: %s (%d messages). Session cleared.\n",
			savedID, msgCount)
	} else {
		fmt.Fprintln(r.rl.Stdout(), "Session cleared.")
	}
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

// handleToolsCommand lists all registered tools with provenance tags and descriptions.
func (r *REPL) handleToolsCommand() {
	if r.registry == nil {
		fmt.Fprintln(r.rl.Stdout(), "No tool registry available.")
		return
	}
	info := r.registry.ToolInfo()
	if len(info) == 0 {
		fmt.Fprintln(r.rl.Stdout(), "No tools loaded.")
		return
	}
	for _, t := range info {
		tag := "[lua]"
		if t.BuiltIn {
			tag = "[built-in]"
		}
		fmt.Fprintf(r.rl.Stdout(), "  %-10s %s -- %s\n", tag, t.Name, t.Description)
	}
}

// refreshSystemPrompt rebuilds the system prompt message in the conversation
// with current tool descriptions from the registry. Called after tool lifecycle
// events so the model sees updated tools on the next turn.
func (r *REPL) refreshSystemPrompt() {
	if len(r.conv.Messages) > 0 && r.conv.Messages[0].Role == "system" {
		prompt := r.baseSystemPrompt
		if r.registry != nil {
			toolDesc := r.registry.Describe()
			if toolDesc != "" {
				prompt = prompt + "\n\n## Available Tools\n\n" + toolDesc
			}
		}
		r.conv.Messages[0].Content = prompt
	}
}

// SetDebug enables or disables debug output (e.g., tool result display).
func (r *REPL) SetDebug(on bool) {
	r.debug = on
}

// EnableThink enables model thinking/reasoning output on the conversation.
// When enabled, thinking content is captured and the last 3 lines are
// displayed in muted style before each response.
func (r *REPL) EnableThink() {
	r.conv.Think = true
}

// RefreshSystemPrompt is the exported wrapper for refreshSystemPrompt.
// Called from the tool event notifier callback in main.go after tool
// create/update/delete events.
func (r *REPL) RefreshSystemPrompt() {
	r.refreshSystemPrompt()
}
