package repl

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marad/fenec/internal/model"
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
