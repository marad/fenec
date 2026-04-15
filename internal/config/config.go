package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// DefaultHost is the default Ollama server address.
	// Per D-16: Connect to localhost:11434 by default.
	DefaultHost = "http://localhost:11434"

	// Version is the application version.
	Version = "v0.1"

	// AppName is the application name.
	AppName = "fenec"
)

// defaultSystemPrompt is used when ~/.config/fenec/system.md does not exist.
const defaultSystemPrompt = `You are Fenec, a helpful AI assistant running locally via Ollama. Be concise and direct in your responses. Use markdown formatting when it improves clarity. You have access to tools that let you execute shell commands when needed to help the user.

## Lua Tool Format

When creating or updating Lua tools via create_lua_tool or update_lua_tool, the code MUST return a table with this exact structure:

` + "```" + `lua
return {
    name = "tool_name",
    description = "What this tool does",
    parameters = {
        { name = "param1", type = "string", description = "Param description", required = true }
    },
    execute = function(args)
        local value = args.param1 or ""
        return "result: " .. value
    end
}
` + "```" + `

Rules: the script must return a table (not call a function). The table must have name (string), description (string), and execute (function). Parameters is optional. The execute function receives an args table and must return a string.`

// ConfigDir returns the fenec configuration directory path.
// Always returns ~/.config/fenec on all platforms (per CFG-01).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "fenec"), nil
}

// legacyConfigDir returns the old macOS config path, or empty string on non-darwin.
func legacyConfigDir() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "fenec")
}

// MigrateIfNeeded migrates config data from the legacy macOS path
// (~/Library/Application Support/fenec) to the new path (~/.config/fenec).
// It is a no-op on non-macOS platforms or if no legacy data exists.
// Prints a confirmation message to stderr on successful migration (per CFG-02, CFG-03).
func MigrateIfNeeded() {
	legacy := legacyConfigDir()
	if legacy == "" {
		return // Not macOS, nothing to migrate
	}

	newDir, err := ConfigDir()
	if err != nil {
		return // Can't determine new path, skip silently
	}

	doMigrate(legacy, newDir, os.Stderr)
}

// doMigrate performs the actual directory migration from legacy to newDir.
// Exported to tests via package-private access. Accepts io.Writer for testable stderr output.
// Skips silently if: legacy doesn't exist, newDir already exists, or rename fails.
func doMigrate(legacy, newDir string, w io.Writer) {
	// Check if legacy directory exists.
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return // No legacy data, nothing to migrate
	}

	// Don't overwrite if new path already exists.
	if _, err := os.Stat(newDir); err == nil {
		return // New path exists, don't clobber
	}

	// Ensure parent directory (~/.config/) exists with standard permissions.
	if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
		fmt.Fprintf(w, "fenec: failed to create config directory: %v\n", err)
		return
	}

	// Atomic rename — instant on same APFS volume, no partial state risk.
	if err := os.Rename(legacy, newDir); err != nil {
		fmt.Fprintf(w, "fenec: failed to migrate config: %v\n", err)
		return
	}

	// CFG-03: User feedback on stderr.
	fmt.Fprintf(w, "fenec: migrated config from %s to %s\n", legacy, newDir)
}

// LoadSystemPrompt reads the system prompt from {ConfigDir}/system.md.
// Per D-15: If the file doesn't exist, returns a sensible default.
// Returns error only for permission or I/O issues (not for missing file).
func LoadSystemPrompt() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return defaultSystemPrompt, nil
	}

	path := filepath.Join(dir, "system.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSystemPrompt, nil
		}
		return "", err
	}
	return string(data), nil
}

// SessionDir returns the path to the session storage directory.
// Located at {ConfigDir}/sessions/.
// Creates the directory if it doesn't exist.
func SessionDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	sessDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		return "", err
	}

	return sessDir, nil
}

// ToolsDir returns the path to the Lua tools directory.
// Located at {ConfigDir}/tools/.
// Does NOT create the directory -- it may not exist until Phase 5.
func ToolsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tools"), nil
}

// ProfilesDir returns the path to the profiles directory.
// Located at {ConfigDir}/profiles/.
// Does NOT create the directory -- it may not exist until the user creates a profile.
func ProfilesDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles"), nil
}

// HistoryFile returns the path to the readline history file.
// Located at {ConfigDir}/history.
// Creates the config directory if it doesn't exist.
func HistoryFile() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	// Create the config directory if it doesn't exist.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(dir, "history"), nil
}
