package repl

import "strings"

// Command represents a parsed slash command.
type Command struct {
	Name string
	Args []string
}

// ParseCommand parses a slash command string.
// Returns nil if the input is not a slash command.
// Per D-03: slash-prefix commands start with /.
func ParseCommand(input string) *Command {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	parts := strings.Fields(trimmed)
	cmd := &Command{
		Name: parts[0],
	}
	if len(parts) > 1 {
		cmd.Args = parts[1:]
	}
	return cmd
}

// IsCommand returns true if the input starts with "/".
func IsCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// helpText is displayed when the user types /help.
const helpText = `Available commands:
  /help    - Show this help message
  /model              - List models or switch: /model [provider/]name
  /save    - Save current conversation to disk
  /load    - List and load a saved conversation
  /history - Show conversation stats (messages, tokens)
  /tools   - List all loaded tools with provenance
  /quit    - Exit fenec

Shortcuts:
  Ctrl+C  - Cancel active generation, or clear current input
  Ctrl+D  - Exit fenec
  \       - Continue input on next line (at end of line)

Tools:
  The agent can use tools to execute actions. Dangerous commands
  (rm, sudo, chmod, etc.) will prompt for your approval.`
