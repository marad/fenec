package copilot

import (
	"os"
	"testing"

	"github.com/marad/fenec/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderImplementsInterface(t *testing.T) {
	// Compile-time check: Provider must satisfy provider.Provider.
	// This duplicates the var _ line in copilot.go as an explicit test.
	var _ provider.Provider = (*Provider)(nil)
}

func TestNewWithGHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "gho_test_token_for_new")
	t.Setenv("GITHUB_TOKEN", "")

	p, err := New()
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestProviderName(t *testing.T) {
	t.Setenv("GH_TOKEN", "gho_test_token_for_name")
	t.Setenv("GITHUB_TOKEN", "")

	p, err := New()
	require.NoError(t, err)
	assert.Equal(t, "copilot", p.Name())
}

func TestProviderDefaultModel(t *testing.T) {
	t.Setenv("GH_TOKEN", "gho_test_token_for_model")
	t.Setenv("GITHUB_TOKEN", "")

	p, err := New()
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-4o-mini", p.DefaultModel())
}

func TestNewWithoutTokenFailsWhenNoGh(t *testing.T) {
	// Only meaningful when gh CLI is not installed or not authenticated.
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")

	_, err := New()
	if err == nil {
		t.Skip("gh CLI is installed and authenticated — cannot test no-token failure path")
	}
	assert.Error(t, err)
}
