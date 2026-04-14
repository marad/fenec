package copilot

import (
	"context"
	"fmt"
	"net/http"

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
	inner *openaiProvider.Provider
	token string
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

// ListModels returns available models. Stub for Phase 12 — replaced in Phase 13 with catalog HTTP call.
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	return p.inner.ListModels(ctx)
}

// Ping verifies the provider is reachable by fetching the GitHub Models catalog.
func (p *Provider) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return fmt.Errorf("copilot: creating ping request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("copilot: cannot reach GitHub Models API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("copilot: GitHub Models API returned %s", resp.Status)
	}
	return nil
}

// GetContextLength returns the context window size. Stub for Phase 12 — replaced in Phase 13 with catalog data.
func (p *Provider) GetContextLength(ctx context.Context, modelName string) (int, error) {
	return p.inner.GetContextLength(ctx, modelName)
}
