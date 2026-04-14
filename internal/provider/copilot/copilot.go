package copilot

import (
	"context"
	"fmt"
	"sync"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	openaiProvider "github.com/marad/fenec/internal/provider/openai"
)

const (
	baseURL      = "https://models.github.ai/inference"
	catalogURL   = "https://models.github.ai/v1/models"
	defaultModel = "openai/gpt-4o-mini"
)

// Compile-time check: Provider satisfies provider.Provider.
var _ provider.Provider = (*Provider)(nil)

// Provider wraps openai.Provider with GitHub Models base URL and automatic token resolution.
type Provider struct {
	inner   *openaiProvider.Provider
	token   string
	mu      sync.RWMutex
	catalog []ghModel
}

// New creates a Provider using GitHub authentication from the environment or gh CLI.
// Token resolution order: GH_TOKEN env var > GITHUB_TOKEN env var > gh auth token.
func New() (*Provider, error) {
	token, err := resolveToken()
	if err != nil {
		return nil, fmt.Errorf("copilot: %w", err)
	}
	inner, err := openaiProvider.New(baseURL, token)
	if err != nil {
		return nil, fmt.Errorf("copilot: creating openai client: %w", err)
	}
	return &Provider{inner: inner, token: token}, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "copilot"
}

// DefaultModel returns the recommended default model for the copilot provider.
func (p *Provider) DefaultModel() string {
	return defaultModel
}

// StreamChat delegates to the inner openai.Provider.
func (p *Provider) StreamChat(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
	return p.inner.StreamChat(ctx, req, onToken, onThinking)
}

// ListModels returns available models from the GitHub Models catalog.
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	models, err := p.fetchCatalog(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	return ids, nil
}

// Ping verifies the provider is reachable and authenticated by fetching the model catalog.
func (p *Provider) Ping(ctx context.Context) error {
	models, err := p.fetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("cannot connect to GitHub Models: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("copilot: GitHub Models catalog returned no models")
	}
	return nil
}

// GetContextLength returns the context window size from the GitHub Models catalog.
// Returns 0, nil for unknown models — consistent with the openai provider behavior.
func (p *Provider) GetContextLength(ctx context.Context, modelName string) (int, error) {
	models, err := p.fetchCatalog(ctx)
	if err != nil {
		return 0, err
	}
	for _, m := range models {
		if m.ID == modelName {
			return m.Limits.MaxInputTokens, nil
		}
	}
	return 0, nil
}
