package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConversationContextLengthDefault(t *testing.T) {
	conv := NewConversation("test-model", "You are helpful.")
	assert.Equal(t, 0, conv.ContextLength)
}

func TestConversationContextLengthAccessible(t *testing.T) {
	conv := NewConversation("test-model", "You are helpful.")
	conv.ContextLength = 8192
	assert.Equal(t, 8192, conv.ContextLength)
}
