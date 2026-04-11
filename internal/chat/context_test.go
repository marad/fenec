package chat

import (
	"testing"

	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextTrackerNewCreatesWithParams(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	assert.Equal(t, 8192, ct.Available())
	assert.Equal(t, 0.85, ct.Threshold())
}

func TestContextTrackerUpdateSetsTokenCounts(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	ct.Update(500, 100)
	assert.Equal(t, 600, ct.TokenUsage())
}

func TestContextTrackerShouldTruncateFalseWhenBelowThreshold(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	// Threshold = 8192 * 0.85 = 6963.2
	ct.Update(500, 100) // Total = 600, well below 6963
	assert.False(t, ct.ShouldTruncate())
}

func TestContextTrackerShouldTruncateTrueWhenAboveThreshold(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	// Threshold = 8192 * 0.85 = 6963.2
	ct.Update(7000, 500) // Total = 7500, above 6963
	assert.True(t, ct.ShouldTruncate())
}

func TestContextTrackerTokenUsageReturnsSum(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	ct.Update(300, 200)
	assert.Equal(t, 500, ct.TokenUsage())
}

func TestContextTrackerAvailableReturnsMaxTokens(t *testing.T) {
	ct := NewContextTracker(16384, 0.90)
	assert.Equal(t, 16384, ct.Available())
}

func TestContextTrackerThresholdReturnsConfiguredValue(t *testing.T) {
	ct := NewContextTracker(8192, 0.75)
	assert.Equal(t, 0.75, ct.Threshold())
}

func TestContextTrackerTruncateOldestPreservesSystemMessage(t *testing.T) {
	ct := NewContextTracker(100, 0.50)
	// Set token usage above threshold (100 * 0.50 = 50)
	ct.Update(60, 20) // Total = 80, above 50

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
			{Role: "assistant", Content: "I am fine!"},
		},
	}

	removed := ct.TruncateOldest(conv)
	assert.Greater(t, removed, 0)

	// System message must still be first
	require.NotEmpty(t, conv.Messages)
	assert.Equal(t, "system", conv.Messages[0].Role)
	assert.Equal(t, "You are helpful.", conv.Messages[0].Content)
}

func TestContextTrackerTruncateOldestRemovesPairs(t *testing.T) {
	ct := NewContextTracker(100, 0.50)
	// Set token usage above threshold
	ct.Update(60, 20) // Total = 80, above 50

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "First question"},
			{Role: "assistant", Content: "First answer"},
			{Role: "user", Content: "Second question"},
			{Role: "assistant", Content: "Second answer"},
		},
	}

	removed := ct.TruncateOldest(conv)
	// Should remove at least one pair (2 messages)
	assert.True(t, removed >= 2)
	// System message should remain
	assert.Equal(t, "system", conv.Messages[0].Role)
}

func TestContextTrackerTruncateOldestReturnsRemovedCount(t *testing.T) {
	ct := NewContextTracker(100, 0.50)
	ct.Update(60, 20) // Total = 80, above 50

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
	}

	originalLen := len(conv.Messages)
	removed := ct.TruncateOldest(conv)
	assert.Equal(t, originalLen-len(conv.Messages), removed)
}

func TestContextTrackerTruncateOldestOnlySystemReturnsZero(t *testing.T) {
	ct := NewContextTracker(100, 0.50)
	ct.Update(60, 20) // Total = 80, above 50

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
		},
	}

	removed := ct.TruncateOldest(conv)
	assert.Equal(t, 0, removed)
	assert.Len(t, conv.Messages, 1)
}

func TestContextTrackerTruncateOldestSingleUserMessage(t *testing.T) {
	ct := NewContextTracker(100, 0.50)
	ct.Update(60, 20) // Total = 80, above 50

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}

	removed := ct.TruncateOldest(conv)
	assert.Equal(t, 1, removed)
	assert.Len(t, conv.Messages, 1)
	assert.Equal(t, "system", conv.Messages[0].Role)
}

func TestContextTrackerTruncateOldestNoTruncationNeeded(t *testing.T) {
	ct := NewContextTracker(8192, 0.85)
	ct.Update(100, 50) // Total = 150, well below 6963

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
	}

	removed := ct.TruncateOldest(conv)
	assert.Equal(t, 0, removed)
	assert.Len(t, conv.Messages, 3)
}

func TestContextTrackerTruncateOldestWithoutSystemMessage(t *testing.T) {
	ct := NewContextTracker(100, 0.50)
	ct.Update(60, 20) // Total = 80, above 50

	conv := &Conversation{
		Messages: []api.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
			{Role: "user", Content: "Question"},
			{Role: "assistant", Content: "Answer"},
		},
	}

	removed := ct.TruncateOldest(conv)
	assert.Greater(t, removed, 0)
	// Messages should be shorter now
	assert.Less(t, len(conv.Messages), 4)
}
