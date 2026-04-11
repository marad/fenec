package render

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdownBasic(t *testing.T) {
	output, err := RenderMarkdown("# Hello\n\nworld", 80)
	assert.NoError(t, err)
	assert.Contains(t, output, "Hello")
	assert.Contains(t, output, "world")
}

func TestRenderMarkdownCodeBlock(t *testing.T) {
	md := "```go\nfmt.Println(\"hello\")\n```"
	output, err := RenderMarkdown(md, 80)
	assert.NoError(t, err)
	assert.Contains(t, output, "Println")
}

func TestRenderMarkdownDefaultWidth(t *testing.T) {
	// Passing width=0 should not panic and should default to 80.
	output, err := RenderMarkdown("# Test\n\nSome content here.", 0)
	assert.NoError(t, err)
	assert.Contains(t, output, "Test")
}

func TestCountLines(t *testing.T) {
	assert.Equal(t, 3, CountLines("a\nb\nc"))
	assert.Equal(t, 0, CountLines(""))
	assert.Equal(t, 1, CountLines("no newline"))
}

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

func TestOverwriteRawOutput(t *testing.T) {
	var buf bytes.Buffer
	rendered := "formatted output"
	OverwriteRawOutput(&buf, 5, rendered)

	output := buf.String()
	// Should contain cursor-up escape sequence for 5 lines
	assert.Contains(t, output, "\033[5A")
	// Should contain clear-to-end-of-screen escape sequence
	assert.Contains(t, output, "\033[J")
	// Should contain the rendered content
	assert.Contains(t, output, "formatted output")
}

func TestFormatError(t *testing.T) {
	errMsg := FormatError("something went wrong")
	assert.Contains(t, errMsg, "Error:")
	assert.Contains(t, errMsg, "something went wrong")
}
