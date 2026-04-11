package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSession(t *testing.T) {
	before := time.Now()
	sess := NewSession("gemma4:latest")
	after := time.Now()

	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, "gemma4:latest", sess.Model)
	assert.Empty(t, sess.Messages)
	assert.Equal(t, 0, sess.TokenCount)

	// CreatedAt and UpdatedAt should be within the time window.
	assert.False(t, sess.CreatedAt.Before(before), "CreatedAt should be >= before")
	assert.False(t, sess.CreatedAt.After(after), "CreatedAt should be <= after")
	assert.Equal(t, sess.CreatedAt, sess.UpdatedAt, "CreatedAt and UpdatedAt should match initially")

	// ID should be in the expected timestamp format.
	_, err := time.Parse("2006-01-02T15-04-05", sess.ID)
	assert.NoError(t, err, "ID should be parseable as timestamp format 2006-01-02T15-04-05")
}

func TestSessionJSONRoundTrip(t *testing.T) {
	sess := &Session{
		ID:        "2026-04-11T10-30-00",
		Model:     "gemma4:latest",
		CreatedAt: time.Date(2026, 4, 11, 10, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 11, 10, 35, 0, 0, time.UTC),
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
		TokenCount: 42,
	}

	data, err := json.Marshal(sess)
	require.NoError(t, err)

	var restored Session
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, sess.ID, restored.ID)
	assert.Equal(t, sess.Model, restored.Model)
	assert.True(t, sess.CreatedAt.Equal(restored.CreatedAt), "CreatedAt should round-trip")
	assert.True(t, sess.UpdatedAt.Equal(restored.UpdatedAt), "UpdatedAt should round-trip")
	assert.Equal(t, sess.TokenCount, restored.TokenCount)
	require.Len(t, restored.Messages, 3)
	assert.Equal(t, "system", restored.Messages[0].Role)
	assert.Equal(t, "You are helpful.", restored.Messages[0].Content)
	assert.Equal(t, "user", restored.Messages[1].Role)
	assert.Equal(t, "Hello", restored.Messages[1].Content)
	assert.Equal(t, "assistant", restored.Messages[2].Role)
	assert.Equal(t, "Hi there!", restored.Messages[2].Content)
}

func TestSessionJSONKeys(t *testing.T) {
	sess := NewSession("test-model")

	data, err := json.Marshal(sess)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	expectedKeys := []string{"id", "model", "created_at", "updated_at", "messages", "token_count"}
	for _, key := range expectedKeys {
		_, exists := raw[key]
		assert.True(t, exists, "JSON should contain key %q", key)
	}
}

func TestHasContentEmpty(t *testing.T) {
	sess := NewSession("model")
	assert.False(t, sess.HasContent(), "Empty session should not have content")
}

func TestHasContentSystemOnly(t *testing.T) {
	sess := NewSession("model")
	sess.Messages = []api.Message{
		{Role: "system", Content: "You are helpful."},
	}
	assert.False(t, sess.HasContent(), "Session with only system message should not have content")
}

func TestHasContentWithUserMessage(t *testing.T) {
	sess := NewSession("model")
	sess.Messages = []api.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
	}
	assert.True(t, sess.HasContent(), "Session with user message should have content")
}
