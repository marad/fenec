package config

import (
	"os"
	"path/filepath"
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
// On Linux: ~/.config/fenec/
// On macOS: ~/Library/Application Support/fenec/
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// LoadSystemPrompt reads the system prompt from ~/.config/fenec/system.md.
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
