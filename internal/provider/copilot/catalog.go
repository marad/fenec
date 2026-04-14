package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ghModel represents a single model entry from the GitHub Models catalog.
type ghModel struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Limits       struct {
		MaxInputTokens  int `json:"max_input_tokens"`
		MaxOutputTokens int `json:"max_output_tokens"`
	} `json:"limits"`
	RateLimitTier string `json:"rate_limit_tier"`
}

// modelsResponse is the top-level catalog response wrapper.
type modelsResponse struct {
	Data []ghModel `json:"data"`
}

// fetchCatalog fetches and caches the GitHub Models catalog.
// Thread-safe: uses double-checked locking. Fetched once per session.
func (p *Provider) fetchCatalog(ctx context.Context) ([]ghModel, error) {
	return p.fetchCatalogFrom(ctx, catalogURL)
}

// fetchCatalogFrom fetches the catalog from the given URL. Separated for testability.
func (p *Provider) fetchCatalogFrom(ctx context.Context, url string) ([]ghModel, error) {
	// Fast path: already cached
	p.mu.RLock()
	if p.catalog != nil {
		models := p.catalog
		p.mu.RUnlock()
		return models, nil
	}
	p.mu.RUnlock()

	// Slow path: fetch and cache
	p.mu.Lock()
	defer p.mu.Unlock()
	// Double-check after acquiring write lock
	if p.catalog != nil {
		return p.catalog, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("copilot: building catalog request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot: fetching model catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("copilot: GitHub token is invalid or expired. Run: gh auth login")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot: model catalog returned %s", resp.Status)
	}

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("copilot: decoding model catalog: %w", err)
	}

	p.catalog = result.Data
	return p.catalog, nil
}
