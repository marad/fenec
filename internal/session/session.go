package session

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/marad/fenec/internal/model"
)

// Session represents a saved conversation.
type Session struct {
	ID         string          `json:"id"`
	Model      string          `json:"model"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Messages   []model.Message `json:"messages"`
	TokenCount int             `json:"token_count"`
}

// SessionInfo is a lightweight summary for listing sessions without loading full message history.
type SessionInfo struct {
	ID           string    `json:"id"`
	Model        string    `json:"model"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

// NewSession creates a new session with the given model.
// ID includes milliseconds and a random suffix to avoid collisions
// when sessions are created in rapid succession (e.g., /clear).
func NewSession(model string) *Session {
	now := time.Now()
	var suffix [2]byte
	rand.Read(suffix[:])
	return &Session{
		ID:        fmt.Sprintf("%s-%x", now.Format("2006-01-02T15-04-05.000"), suffix),
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// HasContent returns true if the session has user-generated content
// (more than just the system message).
func (s *Session) HasContent() bool {
	return len(s.Messages) > 1
}
