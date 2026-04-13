package config

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcherCallsOnChange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// Create the config file so the watcher has something to watch.
	require.NoError(t, os.WriteFile(configPath, []byte("initial"), 0644))

	var called atomic.Int32
	w, err := NewConfigWatcher(configPath, func() {
		called.Add(1)
	})
	require.NoError(t, err)
	defer w.Stop()

	// Write to the file — should trigger onChange.
	require.NoError(t, os.WriteFile(configPath, []byte("updated"), 0644))

	// Wait for debounce + processing (100ms debounce + margin).
	assert.Eventually(t, func() bool {
		return called.Load() >= 1
	}, 500*time.Millisecond, 10*time.Millisecond, "onChange should be called after file write")
}

func TestWatcherDebounce(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	require.NoError(t, os.WriteFile(configPath, []byte("initial"), 0644))

	var counter atomic.Int32
	w, err := NewConfigWatcher(configPath, func() {
		counter.Add(1)
	})
	require.NoError(t, err)
	defer w.Stop()

	// Write 5 times rapidly — debounce should collapse to a single call.
	for i := 0; i < 5; i++ {
		require.NoError(t, os.WriteFile(configPath, []byte("update"), 0644))
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to settle (100ms debounce + margin).
	time.Sleep(300 * time.Millisecond)

	count := counter.Load()
	assert.Equal(t, int32(1), count, "onChange should be called exactly once after rapid writes, got %d", count)
}

func TestWatcherIgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	otherPath := filepath.Join(dir, "other.txt")

	require.NoError(t, os.WriteFile(configPath, []byte("initial"), 0644))

	var counter atomic.Int32
	w, err := NewConfigWatcher(configPath, func() {
		counter.Add(1)
	})
	require.NoError(t, err)
	defer w.Stop()

	// Write to a different file in the same directory.
	require.NoError(t, os.WriteFile(otherPath, []byte("other content"), 0644))

	// Wait long enough — onChange should NOT be called.
	time.Sleep(500 * time.Millisecond)

	assert.Equal(t, int32(0), counter.Load(), "onChange should not be called for other files")
}

func TestWatcherStop(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	require.NoError(t, os.WriteFile(configPath, []byte("initial"), 0644))

	w, err := NewConfigWatcher(configPath, func() {})
	require.NoError(t, err)

	// Stop should not panic and should return nil error.
	err = w.Stop()
	assert.NoError(t, err, "Stop should not return an error")

	// After stop, writing to the file should not cause issues.
	require.NoError(t, os.WriteFile(configPath, []byte("after stop"), 0644))
	time.Sleep(200 * time.Millisecond)
	// No panic or goroutine leak expected.
}
