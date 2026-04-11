package render

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

var (
	// modelStyle styles the model name in the prompt bracket.
	modelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	// bannerStyle styles the startup banner.
	bannerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	// errorStyle styles error messages.
	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Bold(true)
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
