package config

import (
	"errors"
	"testing"

	"github.com/marad/fenec/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegistryWithProviders(t *testing.T) (*ProviderRegistry, provider.Provider, provider.Provider) {
	t.Helper()
	reg := NewProviderRegistry()
	ollama := &mockProvider{name: "ollama"}
	openai := &mockProvider{name: "openai"}
	reg.Register("ollama", ollama)
	reg.Register("openai", openai)
	reg.SetDefault("ollama")
	return reg, ollama, openai
}

func TestResolveModel_BareModelKeepsCurrentProvider(t *testing.T) {
	reg, ollama, _ := newRegistryWithProviders(t)

	res, err := ResolveModel(reg, "gemma4", true, "", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "ollama", res.ProviderName)
	assert.Equal(t, ollama, res.Provider)
	assert.Equal(t, "gemma4", res.ModelName)
}

func TestResolveModel_ProviderSlashModelSwitchesProvider(t *testing.T) {
	reg, ollama, openai := newRegistryWithProviders(t)

	res, err := ResolveModel(reg, "openai/gpt-4o", true, "", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "openai", res.ProviderName)
	assert.Equal(t, openai, res.Provider)
	assert.Equal(t, "gpt-4o", res.ModelName)
}

func TestResolveModel_ProviderSlashEmptyUsesPerProviderDefault(t *testing.T) {
	reg, ollama, openai := newRegistryWithProviders(t)
	reg.SetDefaultModel("openai", "gpt-4o-mini")

	res, err := ResolveModel(reg, "openai/", true, "", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "openai", res.ProviderName)
	assert.Equal(t, openai, res.Provider)
	assert.Equal(t, "gpt-4o-mini", res.ModelName)
}

func TestResolveModel_ProviderSlashEmptyNoDefaultErrors(t *testing.T) {
	reg, ollama, _ := newRegistryWithProviders(t)

	_, err := ResolveModel(reg, "openai/", true, "gemma4", ollama, "ollama")
	require.Error(t, err)
	var noDefault *NoDefaultModelError
	require.True(t, errors.As(err, &noDefault))
	assert.Equal(t, "openai", noDefault.ProviderName)
}

func TestResolveModel_UnknownProviderErrors(t *testing.T) {
	reg, ollama, _ := newRegistryWithProviders(t)

	_, err := ResolveModel(reg, "nope/model", true, "", ollama, "ollama")
	require.Error(t, err)
	var unknown *UnknownProviderError
	require.True(t, errors.As(err, &unknown))
	assert.Equal(t, "nope", unknown.Name)
	assert.ElementsMatch(t, []string{"ollama", "openai"}, unknown.Available)
}

func TestResolveModel_NoFlagPrefersPerProviderDefault(t *testing.T) {
	reg, ollama, _ := newRegistryWithProviders(t)
	reg.SetDefaultModel("ollama", "gemma4")

	res, err := ResolveModel(reg, "", false, "ignored", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "ollama", res.ProviderName)
	assert.Equal(t, "gemma4", res.ModelName)
}

func TestResolveModel_NoFlagFallsBackToCfgDefault(t *testing.T) {
	reg, ollama, _ := newRegistryWithProviders(t)

	res, err := ResolveModel(reg, "", false, "gemma4", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "gemma4", res.ModelName)
}

func TestResolveModel_NoFlagNoDefaultsLeavesEmpty(t *testing.T) {
	reg, ollama, _ := newRegistryWithProviders(t)

	res, err := ResolveModel(reg, "", false, "", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "", res.ModelName, "caller falls through to ListModels")
}

func TestResolveModel_ExplicitEmptyModelKeepsCurrent(t *testing.T) {
	// Edge case: modelExplicit=true but modelArg="" (unusual but possible if
	// user passes --model ""). Should not error; caller falls through.
	reg, ollama, _ := newRegistryWithProviders(t)

	res, err := ResolveModel(reg, "", true, "gemma4", ollama, "ollama")
	require.NoError(t, err)
	assert.Equal(t, "ollama", res.ProviderName)
	assert.Equal(t, "", res.ModelName)
}
