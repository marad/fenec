package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/marad/fenec/internal/provider"
	"github.com/marad/fenec/internal/provider/ollama"
)

// Config represents the top-level configuration loaded from config.toml.
type Config struct {
	DefaultProvider string                    `toml:"default_provider"`
	DefaultModel    string                    `toml:"default_model"`
	Providers       map[string]ProviderConfig `toml:"providers"`
}

// ProviderConfig represents the configuration for a single provider.
type ProviderConfig struct {
	Type         string `toml:"type"`
	URL          string `toml:"url"`
	APIKey       string `toml:"api_key"`
	DefaultModel string `toml:"default_model"`
}

// LoadConfig reads and parses a TOML config file at the given path.
// Environment variables in API key fields are resolved after parsing.
func LoadConfig(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Warn about unknown keys (typos in config).
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		for _, key := range undecoded {
			slog.Warn("unknown config key", "key", key)
		}
	}

	resolveEnvVars(&cfg)
	return &cfg, nil
}

// resolveEnvVars resolves $ENV_VAR references in provider API key fields.
// If the env var is not set, the API key is set to empty string and a warning is logged.
// If the API key is plaintext (non-empty, no $ prefix), a warning is logged.
func resolveEnvVars(cfg *Config) {
	for name, pc := range cfg.Providers {
		if strings.HasPrefix(pc.APIKey, "$") {
			envName := pc.APIKey[1:] // Strip the $ prefix.
			val := os.Getenv(envName)
			if val == "" {
				slog.Warn("env var not set for provider API key",
					"provider", name, "var", envName)
			}
			pc.APIKey = val
			cfg.Providers[name] = pc
		} else if pc.APIKey != "" {
			slog.Warn("API key in plaintext config is not recommended",
				"provider", name)
		}
	}
}

// DefaultConfig returns a Config with sensible defaults: a single Ollama
// provider at localhost:11434.
func DefaultConfig() *Config {
	return &Config{
		DefaultProvider: "ollama",
		Providers: map[string]ProviderConfig{
			"ollama": {
				Type: "ollama",
				URL:  DefaultHost,
			},
		},
	}
}

// WriteDefaultConfig writes the default configuration to the given path.
// If the file already exists, it is not overwritten.
func WriteDefaultConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, do not overwrite.
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	defaultTOML := `default_provider = "ollama"

[providers.ollama]
type = "ollama"
url = "http://localhost:11434"
`
	return os.WriteFile(path, []byte(defaultTOML), 0644)
}

// LoadOrCreateConfig loads the config from the given path. If the file does
// not exist, it writes the default config and returns it.
func LoadOrCreateConfig(path string) (*Config, error) {
	_, err := os.Stat(path)
	if err == nil {
		return LoadConfig(path)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking config file: %w", err)
	}

	// File does not exist: write default and return it.
	if writeErr := WriteDefaultConfig(path); writeErr != nil {
		return nil, fmt.Errorf("writing default config: %w", writeErr)
	}
	return DefaultConfig(), nil
}

// CreateProvider creates a provider.Provider instance from a ProviderConfig.
func CreateProvider(name string, cfg ProviderConfig) (provider.Provider, error) {
	switch cfg.Type {
	case "ollama":
		return ollama.New(cfg.URL)
	default:
		return nil, fmt.Errorf("unknown provider type %q for provider %q", cfg.Type, name)
	}
}
