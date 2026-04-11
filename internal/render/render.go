package render

import (
	"fmt"
	"io"
	"strings"

	"charm.land/glamour/v2"
)

// RenderMarkdown renders the given markdown string to styled terminal output.
// width sets the word wrap column (pass terminal width).
// Uses glamour with "dark" style (NOT WithAutoStyle -- removed in v2).
func RenderMarkdown(content string, width int) (string, error) {
	if width <= 0 {
		width = 80
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", fmt.Errorf("creating glamour renderer: %w", err)
	}

	rendered, err := r.Render(content)
	if err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}

	// Glamour's styles add leading/trailing blank lines around blocks.
	// Trim them so the caller controls spacing.
	rendered = strings.TrimRight(rendered, "\n")

	return rendered, nil
}

// OverwriteRawOutput moves cursor up by rawLineCount lines, clears to end of screen,
// then writes the rendered content. Used after streaming completes to replace
// raw text with glamour-formatted markdown.
func OverwriteRawOutput(w io.Writer, rawLineCount int, rendered string) {
	// Move cursor up to start of raw output
	fmt.Fprintf(w, "\033[%dA", rawLineCount)
	// Clear from cursor to end of screen
	fmt.Fprint(w, "\033[J")
	// Write the rendered content
	fmt.Fprint(w, rendered)
}

// CountLines returns the number of newlines in s, plus 1 if s is non-empty.
// Used to count raw output lines for OverwriteRawOutput.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
