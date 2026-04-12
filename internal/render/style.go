package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

var (
	// modelStyle styles the model name in the prompt bracket.
	modelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A9B1D6"))

	// bannerStyle styles the startup banner.
	bannerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A9B1D6"))

	// errorStyle styles error messages.
	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Bold(true)

	// toolEventStyle styles tool lifecycle event banners.
	toolEventStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7AA2F7"))

	// toolCallStyle styles tool call indicators (muted gray).
	toolCallStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7089"))

	// thinkingStyle styles model thinking/reasoning output (dimmed).
	thinkingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565B73")).
		Italic(true)
)

// FormatPrompt returns the readline prompt string: [modelName]>
// Per D-01: model-aware prompt showing active model name.
// Per D-12: minimal model info -- only in the prompt.
func FormatPrompt(modelName string) string {
	return fmt.Sprintf("[%s]> ", modelStyle.Render(modelName))
}

// FormatBanner returns the startup banner.
// Per D-13: "fenec v0.1 -- type /help for commands"
func FormatBanner(version string) string {
	return bannerStyle.Render(fmt.Sprintf("fenec %s", version)) + " -- type /help for commands"
}

// FormatError returns a styled error message.
func FormatError(msg string) string {
	return errorStyle.Render("Error: ") + msg
}

// FormatToolCall returns a muted gray indicator for tool dispatch.
func FormatToolCall(name string, extra string) string {
	return toolCallStyle.Render("[tool: "+name+"]") + extra
}

// FormatToolResult returns a muted gray result indicator (debug mode only).
func FormatToolResult(result string) string {
	if len(result) <= 512 {
		return toolCallStyle.Render("[result: " + result + "]")
	}
	return toolCallStyle.Render(fmt.Sprintf("[result: %d bytes]", len(result)))
}

// FormatThinking returns the last maxLines non-empty lines of thinking content,
// styled in a muted italic style. Returns empty string if thinking is empty.
func FormatThinking(thinking string, maxLines int) string {
	if strings.TrimSpace(thinking) == "" {
		return ""
	}

	// Split into lines and filter out empty ones.
	allLines := strings.Split(thinking, "\n")
	var lines []string
	for _, l := range allLines {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return ""
	}

	// Take last maxLines.
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	label := thinkingStyle.Render("[thinking]")
	body := thinkingStyle.Render(strings.Join(lines, "\n"))
	return label + "\n" + body
}

// ThinkingStreamer displays a rolling window of the last N thinking lines,
// streaming them live as chunks arrive. Replaces the spinner during thinking.
type ThinkingStreamer struct {
	w           io.Writer
	maxLines    int
	lines       []string // rolling window of non-empty lines
	partial     string   // incomplete line (no trailing newline yet)
	drawnLines  int      // how many terminal lines we last drew (for erase)
	labelDrawn  bool
	mu          sync.Mutex
	stopped     bool
}

// NewThinkingStreamer creates a streamer that shows the last maxLines of
// thinking output, rewriting in place.
func NewThinkingStreamer(w io.Writer, maxLines int) *ThinkingStreamer {
	return &ThinkingStreamer{
		w:        w,
		maxLines: maxLines,
	}
}

// Push appends a thinking chunk and redraws the display.
func (ts *ThinkingStreamer) Push(chunk string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.stopped {
		return
	}

	// Combine partial line with new chunk.
	text := ts.partial + chunk
	parts := strings.Split(text, "\n")

	// Last element is the new partial (may be empty if chunk ended with \n).
	ts.partial = parts[len(parts)-1]

	// All elements except the last are complete lines.
	for _, line := range parts[:len(parts)-1] {
		if strings.TrimSpace(line) != "" {
			ts.lines = append(ts.lines, line)
			if len(ts.lines) > ts.maxLines {
				ts.lines = ts.lines[len(ts.lines)-ts.maxLines:]
			}
		}
	}

	ts.redraw()
}

// Finish freezes the display. After this, the thinking lines stay on screen
// and content streams below them. Returns the number of terminal lines occupied.
func (ts *ThinkingStreamer) Finish() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.stopped {
		return
	}
	ts.stopped = true

	// Flush any remaining partial line.
	if strings.TrimSpace(ts.partial) != "" {
		ts.lines = append(ts.lines, ts.partial)
		if len(ts.lines) > ts.maxLines {
			ts.lines = ts.lines[len(ts.lines)-ts.maxLines:]
		}
		ts.partial = ""
		ts.redraw()
	}

	// Move cursor below the drawn content so subsequent output doesn't overwrite.
	if ts.drawnLines > 0 {
		fmt.Fprint(ts.w, "\n")
	}
}

// Clear erases the thinking display entirely (used if no thinking was produced).
func (ts *ThinkingStreamer) Clear() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.stopped = true
	ts.erase()
}

// HasContent returns true if any thinking lines were received.
func (ts *ThinkingStreamer) HasContent() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.lines) > 0 || strings.TrimSpace(ts.partial) != ""
}

func (ts *ThinkingStreamer) erase() {
	if ts.drawnLines > 0 {
		// Move up and clear each line we drew.
		for i := 0; i < ts.drawnLines; i++ {
			fmt.Fprint(ts.w, "\033[A\033[K")
		}
		ts.drawnLines = 0
	}
	if ts.labelDrawn {
		fmt.Fprint(ts.w, "\033[A\033[K")
		ts.labelDrawn = false
	}
}

func (ts *ThinkingStreamer) redraw() {
	if len(ts.lines) == 0 {
		return
	}

	// Erase previous content.
	ts.erase()

	// Draw label + lines.
	label := thinkingStyle.Render("[thinking]")
	fmt.Fprintln(ts.w, label)
	ts.labelDrawn = true

	ts.drawnLines = 0
	for _, line := range ts.lines {
		fmt.Fprintln(ts.w, thinkingStyle.Render(line))
		ts.drawnLines++
	}
}

// FormatToolEvent returns a styled banner for tool lifecycle events.
// event is one of "created", "updated", "deleted".
func FormatToolEvent(event, name, desc string) string {
	switch event {
	case "created":
		return toolEventStyle.Render("New tool registered: "+name) + " -- " + strconv.Quote(desc)
	case "updated":
		return toolEventStyle.Render("Tool updated: "+name) + " -- " + strconv.Quote(desc)
	case "deleted":
		return toolEventStyle.Render("Tool removed: " + name)
	default:
		return ""
	}
}
