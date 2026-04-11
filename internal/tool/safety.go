package tool

import "strings"

// ApproverFunc is a callback for user approval of dangerous commands.
// Returns true to allow execution, false to deny.
type ApproverFunc func(command string) bool

// dangerousPatterns contains substrings that indicate a potentially destructive command.
var dangerousPatterns = []string{
	"rm ", "rm\t", "rmdir ",
	"sudo ",
	"chmod ", "chown ",
	"mkfs", "dd ",
	"> ", ">> ",
	"mv ",
	"kill ", "killall ", "pkill ",
	"reboot", "shutdown",
	"systemctl ",
	"apt ", "dnf ", "pacman ",
}

// IsDangerous checks whether a shell command contains any dangerous patterns.
func IsDangerous(command string) bool {
	cmd := strings.TrimSpace(command)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}
	return false
}
