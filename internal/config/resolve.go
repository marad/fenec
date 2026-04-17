package config

import (
	"fmt"
	"strings"

	"github.com/marad/fenec/internal/provider"
)

// ModelResolution is the outcome of merging a --model flag against the registry.
type ModelResolution struct {
	Provider     provider.Provider
	ProviderName string
	ModelName    string // "" means "caller should ListModels and pick first".
}

// UnknownProviderError is returned when a named provider is not in the registry.
type UnknownProviderError struct {
	Name      string
	Available []string
}

func (e *UnknownProviderError) Error() string {
	return fmt.Sprintf("provider %q not found (available: %s)",
		e.Name, strings.Join(e.Available, ", "))
}

// NoDefaultModelError is returned when "provider/" is passed but the provider
// has no default_model configured.
type NoDefaultModelError struct {
	ProviderName string
}

func (e *NoDefaultModelError) Error() string {
	return fmt.Sprintf("provider %q has no default_model configured", e.ProviderName)
}

// ResolveModel applies --model/config-default precedence to select a provider
// and model name. The caller supplies the current default-provider selection
// (provider + name) so resolution can leave it untouched when no override applies.
//
// Rules:
//   - modelArg "provider/model": switch to provider, use model.
//   - modelArg "provider/":      switch to provider, use its per-provider default_model
//     (NoDefaultModelError if none).
//   - modelArg "model" (no slash): keep current provider, use model as-is.
//   - modelArg "" with modelExplicit=true: keep current provider and empty model
//     (caller falls through to ListModels).
//   - modelArg "" with modelExplicit=false: fall back to current provider's
//     per-provider default_model, then to cfgDefaultModel.
//
// An unknown provider in "provider/..." returns UnknownProviderError.
func ResolveModel(
	reg *ProviderRegistry,
	modelArg string,
	modelExplicit bool,
	cfgDefaultModel string,
	current provider.Provider,
	currentName string,
) (ModelResolution, error) {
	res := ModelResolution{
		Provider:     current,
		ProviderName: currentName,
		ModelName:    modelArg,
	}

	if modelExplicit {
		idx := strings.Index(modelArg, "/")
		if idx == -1 {
			return res, nil
		}
		parts := strings.SplitN(modelArg, "/", 2)
		providerName, modelPart := parts[0], parts[1]
		p, ok := reg.Get(providerName)
		if !ok {
			return res, &UnknownProviderError{Name: providerName, Available: reg.Names()}
		}
		res.Provider = p
		res.ProviderName = providerName
		if modelPart != "" {
			res.ModelName = modelPart
			return res, nil
		}
		provDefault := reg.DefaultModelFor(providerName)
		if provDefault == "" {
			return res, &NoDefaultModelError{ProviderName: providerName}
		}
		res.ModelName = provDefault
		return res, nil
	}

	if modelArg != "" {
		return res, nil
	}

	if provDefault := reg.DefaultModelFor(currentName); provDefault != "" {
		res.ModelName = provDefault
	} else if cfgDefaultModel != "" {
		res.ModelName = cfgDefaultModel
	}
	return res, nil
}
