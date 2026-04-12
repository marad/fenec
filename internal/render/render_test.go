package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatPrompt(t *testing.T) {
	prompt := FormatPrompt("gemma4")
	assert.Contains(t, prompt, "gemma4")
	assert.Contains(t, prompt, "]>")
}

func TestFormatBanner(t *testing.T) {
	banner := FormatBanner("v0.1")
	assert.Contains(t, banner, "fenec")
	assert.Contains(t, banner, "v0.1")
	assert.Contains(t, banner, "/help")
}

func TestFormatError(t *testing.T) {
	errMsg := FormatError("something went wrong")
	assert.Contains(t, errMsg, "Error:")
	assert.Contains(t, errMsg, "something went wrong")
}

func TestFormatThinkingEmpty(t *testing.T) {
	assert.Equal(t, "", FormatThinking("", 3))
	assert.Equal(t, "", FormatThinking("   \n  \n  ", 3))
}

func TestFormatThinkingFewerLinesThanMax(t *testing.T) {
	result := FormatThinking("Line one\nLine two", 3)
	assert.Contains(t, result, "[thinking]")
	assert.Contains(t, result, "Line one")
	assert.Contains(t, result, "Line two")
}

func TestFormatThinkingTruncatesToLastN(t *testing.T) {
	input := "First\nSecond\nThird\nFourth\nFifth"
	result := FormatThinking(input, 3)
	assert.Contains(t, result, "[thinking]")
	assert.NotContains(t, result, "First")
	assert.NotContains(t, result, "Second")
	assert.Contains(t, result, "Third")
	assert.Contains(t, result, "Fourth")
	assert.Contains(t, result, "Fifth")
}

func TestFormatThinkingContainsLabel(t *testing.T) {
	result := FormatThinking("Some reasoning here", 3)
	assert.Contains(t, result, "[thinking]")
	assert.Contains(t, result, "Some reasoning here")
}

func TestFormatThinkingSkipsEmptyLines(t *testing.T) {
	input := "Line A\n\n\nLine B\n\nLine C\n\nLine D\nLine E"
	result := FormatThinking(input, 3)
	// Should show last 3 non-empty lines: C, D, E
	assert.NotContains(t, result, "Line A")
	assert.NotContains(t, result, "Line B")
	assert.Contains(t, result, "Line C")
	assert.Contains(t, result, "Line D")
	assert.Contains(t, result, "Line E")
}
