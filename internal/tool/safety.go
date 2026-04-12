package tool

import "strings"

// ApproverFunc is a callback for user approval of dangerous commands.
// Returns true to allow execution, false to deny.
type ApproverFunc func(command string) bool

// dangerousPattern describes a pattern to match, and whether it must appear
// at a command boundary (start of input or after a shell separator like |, ;, &&).
type dangerousPattern struct {
	pattern         string
	commandBoundary bool // if true, pattern must appear at start of command position
}

// dangerousPatterns contains patterns that indicate a potentially destructive command.
// Patterns with commandBoundary=true only match when they appear as a standalone
// command (not as a substring of another word like "add" matching "dd").
var dangerousPatterns = []dangerousPattern{
	{pattern: "rm ", commandBoundary: true},
	{pattern: "rm\t", commandBoundary: true},
	{pattern: "rmdir ", commandBoundary: true},
	{pattern: "sudo ", commandBoundary: true},
	{pattern: "chmod ", commandBoundary: true},
	{pattern: "chown ", commandBoundary: true},
	{pattern: "mkfs", commandBoundary: true},
	{pattern: "dd ", commandBoundary: true},
	{pattern: "> ", commandBoundary: false},
	{pattern: ">> ", commandBoundary: false},
	{pattern: "mv ", commandBoundary: true},
	{pattern: "kill ", commandBoundary: true},
	{pattern: "killall ", commandBoundary: true},
	{pattern: "pkill ", commandBoundary: true},
	{pattern: "reboot", commandBoundary: true},
	{pattern: "shutdown", commandBoundary: true},
	{pattern: "systemctl ", commandBoundary: true},
	{pattern: "apt ", commandBoundary: true},
	{pattern: "dnf ", commandBoundary: true},
	{pattern: "pacman ", commandBoundary: true},
}

// commandSeparators are shell constructs that introduce a new command.
// A pattern at a command boundary must either be at the start of the string or
// appear immediately after one of these (possibly with spaces).
// Includes shell operators and command-wrapping utilities like xargs.
var commandSeparators = []string{"|", ";", "&&", "||", "$(", "`", "xargs "}

// IsDangerous checks whether a shell command contains any dangerous patterns.
func IsDangerous(command string) bool {
	cmd := strings.TrimSpace(command)
	for _, dp := range dangerousPatterns {
		if dp.commandBoundary {
			if containsAtCommandBoundary(cmd, dp.pattern) {
				return true
			}
		} else {
			if strings.Contains(cmd, dp.pattern) {
				return true
			}
		}
	}
	return false
}

// containsAtCommandBoundary checks if pattern appears in cmd at a position
// where a shell command could start: at the beginning of the string, or
// after a command separator (|, ;, &&, ||, $( , `).
func containsAtCommandBoundary(cmd, pattern string) bool {
	// Check if command starts with the pattern (after trimming)
	if strings.HasPrefix(cmd, pattern) {
		return true
	}

	// Check after each command separator
	for _, sep := range commandSeparators {
		// Find all occurrences of the separator
		remaining := cmd
		for {
			idx := strings.Index(remaining, sep)
			if idx < 0 {
				break
			}
			// Move past the separator and any whitespace
			after := remaining[idx+len(sep):]
			after = strings.TrimLeft(after, " \t")
			if strings.HasPrefix(after, pattern) {
				return true
			}
			// Continue searching after this separator
			remaining = remaining[idx+len(sep):]
		}
	}

	return false
}
