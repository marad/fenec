package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tomlContent := `
default_provider = "ollama"
default_model = "gemma4"

[providers.ollama]
type = "ollama"
url = "http://localhost:11434"

[providers.openai]
type = "openai"
url = "https://api.openai.com/v1"
api_key = "$OPENAI_API_KEY"
default_model = "gpt-4o"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(tomlContent), 0644))

	// Set the env var so resolution doesn't produce warnings.
	t.Setenv("OPENAI_API_KEY", "test-key-123")

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "ollama", cfg.DefaultProvider)
	assert.Equal(t, "gemma4", cfg.DefaultModel)
	assert.Len(t, cfg.Providers, 2)

	ollamaCfg, ok := cfg.Providers["ollama"]
	require.True(t, ok)
	assert.Equal(t, "ollama", ollamaCfg.Type)
	assert.Equal(t, "http://localhost:11434", ollamaCfg.URL)

	openaiCfg, ok := cfg.Providers["openai"]
	require.True(t, ok)
	assert.Equal(t, "openai", openaiCfg.Type)
	assert.Equal(t, "https://api.openai.com/v1", openaiCfg.URL)
	assert.Equal(t, "test-key-123", openaiCfg.APIKey) // Resolved from env var.
	assert.Equal(t, "gpt-4o", openaiCfg.DefaultModel)
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.toml")
	require.Error(t, err)
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("this is = not [valid toml"), 0644))

	_, err := LoadConfig(path)
	require.Error(t, err)
}

func TestResolveEnvVars(t *testing.T) {
	t.Setenv("FOO_KEY", "secret123")

	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"test": {APIKey: "$FOO_KEY"},
		},
	}
	resolveEnvVars(cfg)

	assert.Equal(t, "secret123", cfg.Providers["test"].APIKey)
}

func TestResolveEnvVarsMissing(t *testing.T) {
	// Ensure the env var is not set.
	t.Setenv("MISSING_VAR", "")
	os.Unsetenv("MISSING_VAR")

	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"test": {APIKey: "$MISSING_VAR"},
		},
	}
	resolveEnvVars(cfg)

	assert.Equal(t, "", cfg.Providers["test"].APIKey)
}

func TestPlaintextKeyWarning(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"test": {APIKey: "sk-literal-key"},
		},
	}
	resolveEnvVars(cfg)

	// Plaintext key should be preserved as-is.
	assert.Equal(t, "sk-literal-key", cfg.Providers["test"].APIKey)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "ollama", cfg.DefaultProvider)
	assert.Len(t, cfg.Providers, 1)

	ollama, ok := cfg.Providers["ollama"]
	require.True(t, ok)
	assert.Equal(t, "ollama", ollama.Type)
	assert.Equal(t, "http://localhost:11434", ollama.URL)
}

func TestWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fenec", "config.toml")

	err := WriteDefaultConfig(path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, `default_provider = "ollama"`)
	assert.Contains(t, content, `[providers.ollama]`)
	assert.Contains(t, content, `type = "ollama"`)
	assert.Contains(t, content, `url = "http://localhost:11434"`)
}

func TestWriteDefaultConfigNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write custom content first.
	customContent := `default_provider = "custom"`
	require.NoError(t, os.WriteFile(path, []byte(customContent), 0644))

	// WriteDefaultConfig should not overwrite.
	err := WriteDefaultConfig(path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, customContent, string(data))
}

func TestLoadOrCreateConfig(t *testing.T) {
	t.Run("non-existent creates file and returns default", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "fenec", "config.toml")

		cfg, err := LoadOrCreateConfig(path)
		require.NoError(t, err)

		// Should return default config.
		assert.Equal(t, "ollama", cfg.DefaultProvider)
		assert.Len(t, cfg.Providers, 1)

		// File should have been created.
		_, err = os.Stat(path)
		assert.NoError(t, err)
	})

	t.Run("existing file loads it", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")

		tomlContent := `
default_provider = "custom"

[providers.custom]
type = "ollama"
url = "http://custom:1234"
`
		require.NoError(t, os.WriteFile(path, []byte(tomlContent), 0644))

		cfg, err := LoadOrCreateConfig(path)
		require.NoError(t, err)
		assert.Equal(t, "custom", cfg.DefaultProvider)
	})
}

func TestCreateProviderOllama(t *testing.T) {
	p, err := CreateProvider("test-ollama", ProviderConfig{
		Type: "ollama",
		URL:  "http://localhost:11434",
	})
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "ollama", p.Name())
}

func TestCreateProviderOpenAI(t *testing.T) {
	p, err := CreateProvider("test-openai", ProviderConfig{
		Type:   "openai",
		URL:    "http://localhost:1234/v1",
		APIKey: "test-key",
	})
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "openai", p.Name())
}

func TestCreateProviderOpenAINoAPIKey(t *testing.T) {
	p, err := CreateProvider("lmstudio", ProviderConfig{
		Type: "openai",
		URL:  "http://localhost:1234/v1",
	})
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "openai", p.Name())
}

func TestCreateProviderUnknownType(t *testing.T) {
	_, err := CreateProvider("test-unknown", ProviderConfig{
		Type: "unknown",
		URL:  "http://localhost:1234",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider type")
}
