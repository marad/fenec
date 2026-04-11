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
const defaultSystemPrompt = `You are Fenec, a helpful AI assistant running locally via Ollama. Be concise and direct in your responses. Use markdown formatting when it improves clarity.`

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
