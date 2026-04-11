package repl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
