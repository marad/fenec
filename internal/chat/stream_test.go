package chat

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstTokenNotifierCallsOnce(t *testing.T) {
	var callCount atomic.Int32

	notifier := NewFirstTokenNotifier(func() {
		callCount.Add(1)
	})

	// Call Notify multiple times
	notifier.Notify()
	notifier.Notify()
	notifier.Notify()

	assert.Equal(t, int32(1), callCount.Load())
}

func TestFirstTokenNotifierNilCallback(t *testing.T) {
	notifier := NewFirstTokenNotifier(nil)
	// Should not panic
	assert.NotPanics(t, func() {
		notifier.Notify()
	})
}
