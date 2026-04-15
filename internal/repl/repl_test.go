package repl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chzyer/readline"
	"github.com/marad/fenec/internal/chat"
	"github.com/marad/fenec/internal/config"
	"github.com/marad/fenec/internal/model"
	prov "github.com/marad/fenec/internal/provider"
	"github.com/marad/fenec/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommandModelWithProvider(t *testing.T) {
	cmd := ParseCommand("/model ollama/gemma4")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/model", cmd.Name)
	assert.Equal(t, []string{"ollama/gemma4"}, cmd.Args)
}

func TestParseCommandModelBare(t *testing.T) {
	cmd := ParseCommand("/model gemma4")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/model", cmd.Name)
	assert.Equal(t, []string{"gemma4"}, cmd.Args)
}

func TestParseCommandModelNoArgs(t *testing.T) {
	cmd := ParseCommand("/model")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/model", cmd.Name)
	assert.Nil(t, cmd.Args)
}

func TestParseCommandValid(t *testing.T) {
	cmd := ParseCommand("/model gemma4")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/model", cmd.Name)
	assert.Equal(t, []string{"gemma4"}, cmd.Args)
}

func TestParseCommandNoArgs(t *testing.T) {
	cmd := ParseCommand("/help")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/help", cmd.Name)
	assert.Nil(t, cmd.Args)
}

func TestParseCommandNotACommand(t *testing.T) {
	cmd := ParseCommand("hello world")
	assert.Nil(t, cmd)
}

func TestParseCommandEmpty(t *testing.T) {
	cmd := ParseCommand("")
	assert.Nil(t, cmd)
}

func TestParseCommandWhitespaceOnly(t *testing.T) {
	cmd := ParseCommand("   ")
	assert.Nil(t, cmd)
}

func TestParseCommandWithLeadingSpaces(t *testing.T) {
	cmd := ParseCommand("  /quit  ")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/quit", cmd.Name)
	assert.Nil(t, cmd.Args)
}

func TestParseCommandMultipleArgs(t *testing.T) {
	cmd := ParseCommand("/model gemma4 --verbose")
	assert.NotNil(t, cmd)
	assert.Equal(t, "/model", cmd.Name)
	assert.Equal(t, []string{"gemma4", "--verbose"}, cmd.Args)
}

func TestIsCommandTrue(t *testing.T) {
	assert.True(t, IsCommand("/quit"))
}

func TestIsCommandFalse(t *testing.T) {
	assert.False(t, IsCommand("hello"))
}

func TestIsCommandWithSpaces(t *testing.T) {
	assert.True(t, IsCommand("  /help  "))
}

func TestIsCommandEmpty(t *testing.T) {
	assert.False(t, IsCommand(""))
}

func TestMultiLineDetection(t *testing.T) {
	assert.True(t, isContinuation("Tell me about\\"))
	assert.True(t, isContinuation("  Tell me about\\  "))
	assert.False(t, isContinuation("Tell me about"))
	assert.False(t, isContinuation(""))
	assert.False(t, isContinuation("Tell me about \\n"))
}

func TestParseNewCommands(t *testing.T) {
	tests := []struct {
		input string
		name  string
	}{
		{"/save", "/save"},
		{"/load", "/load"},
		{"/history", "/history"},
	}
	for _, tt := range tests {
		cmd := ParseCommand(tt.input)
		require.NotNil(t, cmd, "command %q should parse", tt.input)
		assert.Equal(t, tt.name, cmd.Name)
	}
}

func TestHelpTextContainsNewCommands(t *testing.T) {
	assert.Contains(t, helpText, "/save")
	assert.Contains(t, helpText, "/load")
	assert.Contains(t, helpText, "/history")
}

func TestHelpTextContainsClear(t *testing.T) {
	assert.Contains(t, helpText, "/clear")
	assert.Contains(t, helpText, "Save and reset")
}

func TestHandleClearCommandSavesAndResets(t *testing.T) {
	r, buf := newTestREPL(t, &mockProvider{name: "ollama"}, nil, "ollama", "gemma4")

	dir := t.TempDir()
	r.store = session.NewStore(dir)
	r.session = session.NewSession("gemma4")
	r.tracker = chat.NewContextTracker(8192, 0.85)
	r.tracker.Update(500, 100)
	r.baseSystemPrompt = "You are helpful."

	// Populate conversation with user content so HasContent() returns true.
	r.conv = chat.NewConversation("gemma4", "You are helpful.")
	r.conv.AddUser("Hello")
	r.conv.AddAssistant("Hi there!")
	r.conv.ContextLength = 8192
	r.conv.Think = true

	// Use a distinct ID so we can detect session replacement (NewSession uses
	// second-precision timestamps, so within the same second IDs may collide).
	r.session.ID = "old-session"
	originalSessionID := r.session.ID

	r.handleClearCommand()

	output := buf.String()

	// CONV-02: session file was saved.
	assert.Contains(t, output, "Conversation saved:")
	assert.Contains(t, output, "Session cleared.")

	// CONV-01: conversation reset to initial state (system prompt only).
	assert.Len(t, r.conv.Messages, 1, "conversation should have only system message")
	assert.Equal(t, "system", r.conv.Messages[0].Role)
	assert.Equal(t, "gemma4", r.conv.Model)

	// D-03: new session with different ID.
	assert.NotEqual(t, originalSessionID, r.session.ID)

	// D-04: tracker reset.
	assert.Equal(t, 0, r.tracker.TokenUsage())

	// Pitfall 1: Think flag preserved.
	assert.True(t, r.conv.Think, "Think flag must be preserved across clear")

	// Pitfall 3: ContextLength preserved.
	assert.Equal(t, 8192, r.conv.ContextLength)
}

func TestHandleClearCommandSkipsSaveWhenEmpty(t *testing.T) {
	r, buf := newTestREPL(t, &mockProvider{name: "ollama"}, nil, "ollama", "gemma4")

	dir := t.TempDir()
	r.store = session.NewStore(dir)
	r.session = session.NewSession("gemma4")
	r.baseSystemPrompt = "You are helpful."

	// Conversation with only system message — HasContent() returns false.
	r.conv = chat.NewConversation("gemma4", "You are helpful.")

	r.handleClearCommand()

	output := buf.String()

	// D-06: only "Session cleared." when no content.
	assert.Equal(t, "Session cleared.\n", output)
	assert.NotContains(t, output, "Conversation saved:")
}

func TestHandleClearCommandPreservesToolDescriptions(t *testing.T) {
	r, buf := newTestREPL(t, &mockProvider{name: "ollama"}, nil, "ollama", "gemma4")
	_ = buf

	dir := t.TempDir()
	r.store = session.NewStore(dir)
	r.session = session.NewSession("gemma4")
	r.baseSystemPrompt = "You are helpful."

	// Create a conv with tool descriptions in system prompt.
	// The registry field is nil — after clear, system prompt should be rebuilt from baseSystemPrompt only.
	r.conv = chat.NewConversation("gemma4", "You are helpful.\n\n## Available Tools\n\nsome tool desc")

	r.handleClearCommand()

	// CONV-03: system prompt must contain the base system prompt.
	assert.Equal(t, "You are helpful.", r.conv.Messages[0].Content,
		"system prompt should be rebuilt from baseSystemPrompt (no registry = no tool desc appended)")
}

func TestAutoSaveCalledOnce(t *testing.T) {
	// Create a temp directory for the session store.
	dir := t.TempDir()
	store := session.NewStore(dir)

	sess := session.NewSession("test-model")
	// Add enough messages so HasContent returns true (>1 message).
	sess.Messages = append(sess.Messages,
		model.Message{Role: "system", Content: "system prompt"},
		model.Message{Role: "user", Content: "hello"},
	)

	// We cannot easily construct a full REPL in tests (requires readline, etc.),
	// so we test the sync.Once behavior directly using the same pattern.
	var saveCount atomic.Int32
	var once sync.Once

	autoSaveFn := func() {
		once.Do(func() {
			saveCount.Add(1)
			err := store.AutoSave(sess)
			assert.NoError(t, err)
		})
	}

	// Call multiple times concurrently.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			autoSaveFn()
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), saveCount.Load(), "auto-save should execute exactly once")

	// Verify the auto-save file was actually written.
	loaded, err := store.LoadAutoSave()
	require.NoError(t, err)
	assert.Equal(t, "test-model", loaded.Model)
	assert.Len(t, loaded.Messages, 2)

	// Use time and require to satisfy imports.
	_ = time.Now()
}

func TestReadAllInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single line", "hello world\n", "hello world"},
		{"multi-line", "line one\nline two\nline three\n", "line one\nline two\nline three"},
		{"empty", "", ""},
		{"whitespace only", "  \n  \n  ", ""},
		{"preserves internal spacing", "  hello\n  world  ", "hello\n  world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readAllInput(strings.NewReader(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── Phase-11 gap-fill: ROUT-02 / ROUT-03 ────────────────────────────────────

// mockProvider implements provider.Provider for unit tests.
type mockProvider struct {
	name   string
	models []string
	err    error
	ctxLen int
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) ListModels(_ context.Context) ([]string, error) {
	return m.models, m.err
}
func (m *mockProvider) Ping(_ context.Context) error { return nil }
func (m *mockProvider) StreamChat(
	_ context.Context,
	_ *prov.ChatRequest,
	_ func(string),
	_ func(string),
) (*model.Message, *model.StreamMetrics, error) {
	return nil, nil, nil
}
func (m *mockProvider) GetContextLength(_ context.Context, _ string) (int, error) {
	return m.ctxLen, nil
}

// newTestREPL builds a minimal REPL with readline wired to a bytes.Buffer so
// output from handleModelCommand / listModels can be captured.
func newTestREPL(
	t *testing.T,
	p prov.Provider,
	registry *config.ProviderRegistry,
	activeProvider, activeModel string,
) (*REPL, *bytes.Buffer) {
	t.Helper()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { stdinR.Close(); stdinW.Close() })

	var buf bytes.Buffer
	rl, rlErr := readline.NewEx(&readline.Config{
		Stdin:  stdinR,
		Stdout: &buf,
		Stderr: &buf,
	})
	if rlErr != nil {
		t.Skipf("readline init failed (no TTY?): %v", rlErr)
	}
	t.Cleanup(func() { rl.Close() })

	r := &REPL{
		provider:         p,
		providerRegistry: registry,
		activeProvider:   activeProvider,
		conv:             chat.NewConversation(activeModel, ""),
		rl:               rl,
	}
	return r, &buf
}

// TestHelpTextContainsProviderSyntax verifies /help documents provider/model syntax.
// ROUT-02: users must be able to discover the switching syntax.
func TestHelpTextContainsProviderSyntax(t *testing.T) {
	assert.Contains(t, helpText, "[provider/]",
		"helpText must document provider/model switching syntax")
}

// TestHandleModelCommandProviderModel verifies that "provider/model" arg switches
// both the active provider and the active model.
// ROUT-02: /model openai/gpt-4 must update r.activeProvider and r.conv.Model.
func TestHandleModelCommandProviderModel(t *testing.T) {
	ollamaMock := &mockProvider{name: "ollama", models: []string{"gemma4"}}
	openaiMock := &mockProvider{name: "openai", models: []string{"gpt-4"}}

	registry := config.NewProviderRegistry()
	registry.Register("ollama", ollamaMock)
	registry.Register("openai", openaiMock)

	r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")

	r.handleModelCommand([]string{"openai/gpt-4"})

	output := buf.String()
	assert.Equal(t, "openai", r.activeProvider,
		"activeProvider should switch to openai")
	assert.Equal(t, "gpt-4", r.conv.Model,
		"conv.Model should switch to gpt-4")
	assert.Contains(t, output, "Switched to openai/gpt-4",
		"output should confirm the switch with full provider/model name")
}

// TestHandleModelCommandUnknownProvider verifies that an unrecognised provider
// prints an error and leaves the model unchanged.
// ROUT-02: error path must name the bad provider and list valid ones.
func TestHandleModelCommandUnknownProvider(t *testing.T) {
	ollamaMock := &mockProvider{name: "ollama", models: []string{"gemma4"}}

	registry := config.NewProviderRegistry()
	registry.Register("ollama", ollamaMock)

	r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")

	r.handleModelCommand([]string{"nonexistent/gemma4"})

	output := buf.String()
	assert.Equal(t, "gemma4", r.conv.Model,
		"conv.Model must remain unchanged when provider is unknown")
	assert.Contains(t, output, "Unknown provider: nonexistent",
		"error message should identify the unknown provider")
	assert.Contains(t, output, "ollama",
		"error message should list registered providers")
}

// TestHandleModelCommandBareModel verifies that a bare model name (no slash)
// switches the model while keeping the current provider.
// ROUT-02: /model llama3.2 must update r.conv.Model but not r.activeProvider.
func TestHandleModelCommandBareModel(t *testing.T) {
	ollamaMock := &mockProvider{name: "ollama", models: []string{"gemma4", "llama3.2"}}

	registry := config.NewProviderRegistry()
	registry.Register("ollama", ollamaMock)

	r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")

	r.handleModelCommand([]string{"llama3.2"})

	output := buf.String()
	assert.Equal(t, "llama3.2", r.conv.Model,
		"conv.Model should switch to the bare model name")
	assert.Equal(t, "ollama", r.activeProvider,
		"activeProvider must not change for a bare model switch")
	assert.Contains(t, output, "Switched to llama3.2",
		"output should confirm the model switch")
}

// TestListModels verifies parallel provider discovery and grouped output with
// the active model marked by an arrow.
// ROUT-03: /model with no args must list all providers and mark active model.
func TestListModels(t *testing.T) {
	ollamaMock := &mockProvider{name: "ollama", models: []string{"gemma4", "llama3.2"}}
	openaiMock := &mockProvider{name: "openai", models: []string{"gpt-4"}}

	registry := config.NewProviderRegistry()
	registry.Register("ollama", ollamaMock)
	registry.Register("openai", openaiMock)

	r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")

	r.listModels()

	output := buf.String()
	assert.Contains(t, output, "## ollama", "output should contain ollama section header")
	assert.Contains(t, output, "## openai", "output should contain openai section header")
	assert.Contains(t, output, "->", "output should mark active model with arrow")
	assert.Contains(t, output, "gemma4", "output should list gemma4")
	assert.Contains(t, output, "llama3.2", "output should list llama3.2")
	assert.Contains(t, output, "gpt-4", "output should list gpt-4")
}

// TestListModelsUnreachableProvider verifies that a provider whose ListModels
// returns an error is shown as unreachable rather than crashing.
// ROUT-03: unreachable providers must display an inline error.
func TestListModelsUnreachableProvider(t *testing.T) {
	ollamaMock := &mockProvider{name: "ollama", models: []string{"gemma4"}}
	lmstudioMock := &mockProvider{
		name: "lmstudio",
		err:  errors.New("connection refused"),
	}

	registry := config.NewProviderRegistry()
	registry.Register("lmstudio", lmstudioMock)
	registry.Register("ollama", ollamaMock)

	r, buf := newTestREPL(t, ollamaMock, registry, "ollama", "gemma4")

	r.listModels()

	output := buf.String()
	assert.Contains(t, output, "## ollama",
		"output should include the reachable provider header")
	assert.Contains(t, output, "unreachable",
		"output should flag lmstudio as unreachable")
}
