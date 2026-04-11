package render

import (
	"fmt"
	"strconv"

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
