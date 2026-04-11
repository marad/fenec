package chat

import (
	"context"
	"fmt"
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPI implements chatAPI for testing.
type mockAPI struct {
	listFunc func(ctx context.Context) (*api.ListResponse, error)
	chatFunc func(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error
}

func (m *mockAPI) List(ctx context.Context) (*api.ListResponse, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	return &api.ListResponse{}, nil
}

func (m *mockAPI) Chat(ctx context.Context, req *api.ChatRequest, fn api.ChatResponseFunc) error {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req, fn)
	}
	return nil
}

func TestNewClientDefaultHost(t *testing.T) {
	client, err := NewClient("")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.api)
}

func TestNewClientCustomHost(t *testing.T) {
	client, err := NewClient("http://myhost:11434")
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClientInvalidHost(t *testing.T) {
	_, err := NewClient("://bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid host URL")
}

func TestListModelsFormatsNames(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return &api.ListResponse{
				Models: []api.ListModelResponse{
					{Name: "gemma4:latest"},
					{Name: "llama3:8b"},
					{Name: "codellama:7b"},
				},
			}, nil
		},
	}

	client := newClientWithAPI(mock)
	names, err := client.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"gemma4:latest", "llama3:8b", "codellama:7b"}, names)
}

func TestListModelsError(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	client := newClientWithAPI(mock)
	_, err := client.ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list models")
}

func TestPingSuccess(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return &api.ListResponse{
				Models: []api.ListModelResponse{
					{Name: "gemma4:latest"},
				},
			}, nil
		},
	}

	client := newClientWithAPI(mock)
	err := client.Ping(context.Background())
	assert.NoError(t, err)
}

func TestPingNoModelsError(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return &api.ListResponse{
				Models: []api.ListModelResponse{},
			}, nil
		},
	}

	client := newClientWithAPI(mock)
	err := client.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no models installed")
	assert.Contains(t, err.Error(), "ollama pull gemma4")
}

func TestPingConnectionError(t *testing.T) {
	mock := &mockAPI{
		listFunc: func(_ context.Context) (*api.ListResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	client := newClientWithAPI(mock)
	err := client.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect to Ollama")
}
