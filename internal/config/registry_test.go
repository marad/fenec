package config

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a minimal provider.Provider for testing the registry.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) ListModels(_ context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockProvider) Ping(_ context.Context) error { return nil }
func (m *mockProvider) StreamChat(_ context.Context, _ *provider.ChatRequest, _ func(string), _ func(string)) (*model.Message, *model.StreamMetrics, error) {
	return nil, nil, nil
}
func (m *mockProvider) GetContextLength(_ context.Context, _ string) (int, error) {
	return 4096, nil
}

func TestRegistryGetDefault(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("ollama", &mockProvider{name: "ollama"})
	reg.Register("openai", &mockProvider{name: "openai"})
	reg.SetDefault("ollama")

	p, err := reg.Default()
	require.NoError(t, err)
	assert.Equal(t, "ollama", p.Name())
}

func TestRegistryGetByName(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("ollama", &mockProvider{name: "ollama"})

	p, ok := reg.Get("ollama")
	assert.True(t, ok)
	assert.Equal(t, "ollama", p.Name())

	_, ok = reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistryUpdate(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("old", &mockProvider{name: "old"})
	reg.SetDefault("old")

	// Update replaces all providers.
	newProviders := map[string]provider.Provider{
		"new1": &mockProvider{name: "new1"},
		"new2": &mockProvider{name: "new2"},
	}
	reg.Update(newProviders, nil, "new1")

	// Old provider is gone.
	_, ok := reg.Get("old")
	assert.False(t, ok)

	// New providers are accessible.
	p, ok := reg.Get("new1")
	assert.True(t, ok)
	assert.Equal(t, "new1", p.Name())

	p, ok = reg.Get("new2")
	assert.True(t, ok)
	assert.Equal(t, "new2", p.Name())

	// Default is updated.
	def, err := reg.Default()
	require.NoError(t, err)
	assert.Equal(t, "new1", def.Name())
}

func TestRegistryNames(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("charlie", &mockProvider{name: "charlie"})
	reg.Register("alpha", &mockProvider{name: "alpha"})
	reg.Register("bravo", &mockProvider{name: "bravo"})

	names := reg.Names()
	expected := []string{"alpha", "bravo", "charlie"}
	assert.Equal(t, expected, names)

	// Verify it is sorted.
	assert.True(t, sort.StringsAreSorted(names))
}

func TestRegistryDefaultNotFound(t *testing.T) {
	reg := NewProviderRegistry()
	reg.SetDefault("nonexistent")

	_, err := reg.Default()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistryDefaultName(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("ollama", &mockProvider{name: "ollama"})
	reg.SetDefault("ollama")

	assert.Equal(t, "ollama", reg.DefaultName())
}

func TestRegistryDefaultNameAfterUpdate(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("old", &mockProvider{name: "old"})
	reg.SetDefault("old")

	newProviders := map[string]provider.Provider{
		"new1": &mockProvider{name: "new1"},
	}
	reg.Update(newProviders, nil, "new1")

	assert.Equal(t, "new1", reg.DefaultName())
}

func TestRegistryDefaultNameEmpty(t *testing.T) {
	reg := NewProviderRegistry()

	assert.Equal(t, "", reg.DefaultName())
}

func TestRegistryRegisterWithDefault(t *testing.T) {
	reg := NewProviderRegistry()
	reg.RegisterWithDefault("ollama", &mockProvider{name: "ollama"}, "gemma4")
	reg.RegisterWithDefault("openai", &mockProvider{name: "openai"}, "gpt-4")
	reg.RegisterWithDefault("local", &mockProvider{name: "local"}, "")

	// RegisterWithDefault stores the provider.
	p, ok := reg.Get("ollama")
	assert.True(t, ok)
	assert.Equal(t, "ollama", p.Name())

	// DefaultModelFor returns the stored default model.
	assert.Equal(t, "gemma4", reg.DefaultModelFor("ollama"))
	assert.Equal(t, "gpt-4", reg.DefaultModelFor("openai"))

	// Empty default model is not stored.
	assert.Equal(t, "", reg.DefaultModelFor("local"))
}

func TestRegistryDefaultModelForUnknown(t *testing.T) {
	reg := NewProviderRegistry()

	// Unknown provider returns empty string.
	assert.Equal(t, "", reg.DefaultModelFor("nonexistent"))
}

func TestRegistryUpdateWithDefaultModels(t *testing.T) {
	reg := NewProviderRegistry()
	reg.RegisterWithDefault("ollama", &mockProvider{name: "ollama"}, "gemma4")

	// Verify initial state.
	assert.Equal(t, "gemma4", reg.DefaultModelFor("ollama"))

	// Update replaces providers and default models.
	newProviders := map[string]provider.Provider{
		"new1": &mockProvider{name: "new1"},
		"new2": &mockProvider{name: "new2"},
	}
	newDefaultModels := map[string]string{
		"new1": "model-a",
		"new2": "model-b",
	}
	reg.Update(newProviders, newDefaultModels, "new1")

	// Old default model is gone.
	assert.Equal(t, "", reg.DefaultModelFor("ollama"))

	// New default models are accessible.
	assert.Equal(t, "model-a", reg.DefaultModelFor("new1"))
	assert.Equal(t, "model-b", reg.DefaultModelFor("new2"))

	// Providers are replaced too.
	_, ok := reg.Get("ollama")
	assert.False(t, ok)
	p, ok := reg.Get("new1")
	assert.True(t, ok)
	assert.Equal(t, "new1", p.Name())
}

func TestRegistryConcurrentAccess(t *testing.T) {
	reg := NewProviderRegistry()
	reg.RegisterWithDefault("initial", &mockProvider{name: "initial"}, "default-model")
	reg.SetDefault("initial")

	var wg sync.WaitGroup

	// 10 readers doing Get/Default/DefaultModelFor.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				reg.Get("initial")
				reg.Default()
				reg.Names()
				reg.DefaultModelFor("initial")
			}
		}()
	}

	// 2 writers doing Update with default models.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				reg.Update(map[string]provider.Provider{
					"updated": &mockProvider{name: "updated"},
				}, map[string]string{
					"updated": "new-model",
				}, "updated")
			}
		}()
	}

	// 2 writers doing RegisterWithDefault.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				reg.RegisterWithDefault("dynamic", &mockProvider{name: "dynamic"}, "dyn-model")
			}
		}()
	}

	wg.Wait()
}
