package copilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	openaiProvider "github.com/marad/fenec/internal/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCatalogJSON = `{"data":[
	{"id":"openai/gpt-4o-mini","name":"GPT-4o mini","capabilities":["chat-completion","tool_call"],"limits":{"max_input_tokens":131072,"max_output_tokens":16384},"rate_limit_tier":"low"},
	{"id":"meta/llama-3.3-70b-instruct","name":"Llama 3.3 70B","capabilities":["chat-completion"],"limits":{"max_input_tokens":131072,"max_output_tokens":4096},"rate_limit_tier":"low"}
]}`

// newTestProviderWithURL builds a Provider that uses the given URL for catalog calls.
func newTestProviderWithURL(t *testing.T, serverURL string) *Provider {
	t.Helper()
	inner, err := openaiProvider.New(baseURL, "test-token")
	require.NoError(t, err)
	return &Provider{inner: inner, token: "test-token"}
}

func TestFetchCatalogReturnsModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testCatalogJSON))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	models, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "openai/gpt-4o-mini", models[0].ID)
	assert.Equal(t, 131072, models[0].Limits.MaxInputTokens)
}

func TestListModelsReturnsIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testCatalogJSON))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	// Seed the cache via fetchCatalogFrom pointed at mock server
	models, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)
	require.Len(t, models, 2)

	// ListModels uses the cached result (no real HTTP call to catalogURL)
	ids, err := p.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"openai/gpt-4o-mini", "meta/llama-3.3-70b-instruct"}, ids)
}

func TestGetContextLengthKnownModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testCatalogJSON))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)

	length, err := p.GetContextLength(context.Background(), "openai/gpt-4o-mini")
	require.NoError(t, err)
	assert.Equal(t, 131072, length)
}

func TestGetContextLengthUnknownModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testCatalogJSON))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)

	length, err := p.GetContextLength(context.Background(), "unknown/model")
	require.NoError(t, err)
	assert.Equal(t, 0, length)
}

func TestCatalogIsCached(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testCatalogJSON))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)
	_, err = p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)

	assert.Equal(t, int32(1), callCount.Load(), "catalog should be fetched only once")
}

func TestFetchCatalog401ReturnsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestFetchCatalogNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // close before calling

	p := newTestProviderWithURL(t, url)
	_, err := p.fetchCatalogFrom(context.Background(), url)
	require.Error(t, err)
}

func TestPingSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testCatalogJSON))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	// Seed cache via fetchCatalogFrom so Ping reads cached result
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)

	err = p.Ping(context.Background())
	assert.NoError(t, err)
}

func TestPingAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	// 401 prevents caching — test the auth error via fetchCatalogFrom
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestPingNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // close before calling

	p := newTestProviderWithURL(t, url)
	_, err := p.fetchCatalogFrom(context.Background(), url)
	require.Error(t, err)
}

func TestPingEmptyCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	p := newTestProviderWithURL(t, srv.URL)
	// Seed cache with empty catalog
	_, err := p.fetchCatalogFrom(context.Background(), srv.URL)
	require.NoError(t, err)

	// Ping checks len(models) == 0 and returns an error
	err = p.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no models")
}
