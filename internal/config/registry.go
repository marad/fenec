package config

import (
	"fmt"
	"sort"
	"sync"

	"github.com/marad/fenec/internal/provider"
)

// ProviderRegistry is a thread-safe map of provider names to Provider instances.
// It supports concurrent reads and exclusive writes via sync.RWMutex.
type ProviderRegistry struct {
	mu              sync.RWMutex
	providers       map[string]provider.Provider
	defaultProvider string
	defaultModels   map[string]string
}

// NewProviderRegistry returns an initialized registry with an empty providers map.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers:     make(map[string]provider.Provider),
		defaultModels: make(map[string]string),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(name string, p provider.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// RegisterWithDefault adds a provider and its per-provider default model to the registry.
func (r *ProviderRegistry) RegisterWithDefault(name string, p provider.Provider, defaultModel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
	if defaultModel != "" {
		r.defaultModels[name] = defaultModel
	}
}

// DefaultModelFor returns the per-provider default model, or empty string if not set.
func (r *ProviderRegistry) DefaultModelFor(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultModels[name]
}

// SetDefault sets the name of the default provider.
func (r *ProviderRegistry) SetDefault(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultProvider = name
}

// Get returns the provider with the given name and whether it exists.
func (r *ProviderRegistry) Get(name string) (provider.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// Default returns the default provider, or an error if it is not found.
func (r *ProviderRegistry) Default() (provider.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[r.defaultProvider]
	if !ok {
		return nil, fmt.Errorf("default provider %q not found", r.defaultProvider)
	}
	return p, nil
}

// Update atomically replaces all providers, default models, and the default name.
func (r *ProviderRegistry) Update(providers map[string]provider.Provider, defaultModels map[string]string, defaultName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = providers
	if defaultModels != nil {
		r.defaultModels = defaultModels
	} else {
		r.defaultModels = make(map[string]string)
	}
	r.defaultProvider = defaultName
}

// DefaultName returns the name of the default provider, or empty string if not set.
func (r *ProviderRegistry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultProvider
}

// Names returns a sorted list of all registered provider names.
func (r *ProviderRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
