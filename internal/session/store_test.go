package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marad/fenec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSession(id, model string, msgs []model.Message) *Session {
	return &Session{
		ID:        id,
		Model:     model,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  msgs,
	}
}

func TestStoreSave(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sess := newTestSession("2026-04-11T10-30-00", "gemma4:latest", []model.Message{
		{Role: "user", Content: "Hello"},
	})

	err := store.Save(sess)
	require.NoError(t, err)

	// Verify file exists.
	path := filepath.Join(dir, "2026-04-11T10-30-00.json")
	_, err = os.Stat(path)
	assert.NoError(t, err, "Session file should exist on disk")
}

func TestStoreLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sess := newTestSession("test-roundtrip", "gemma4:latest", []model.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	})
	sess.TokenCount = 100

	err := store.Save(sess)
	require.NoError(t, err)

	loaded, err := store.Load("test-roundtrip")
	require.NoError(t, err)

	assert.Equal(t, sess.ID, loaded.ID)
	assert.Equal(t, sess.Model, loaded.Model)
	assert.Equal(t, sess.TokenCount, loaded.TokenCount)
	require.Len(t, loaded.Messages, 3)
	assert.Equal(t, "system", loaded.Messages[0].Role)
	assert.Equal(t, "user", loaded.Messages[1].Role)
	assert.Equal(t, "assistant", loaded.Messages[2].Role)
}

func TestStoreLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.Load("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestStoreListSortedByUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Create sessions with different UpdatedAt times.
	old := newTestSession("old-session", "model-a", []model.Message{
		{Role: "user", Content: "old"},
	})
	old.UpdatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	middle := newTestSession("mid-session", "model-b", []model.Message{
		{Role: "user", Content: "mid"},
		{Role: "assistant", Content: "reply"},
	})
	middle.UpdatedAt = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	newest := newTestSession("new-session", "model-c", []model.Message{
		{Role: "user", Content: "new"},
	})
	newest.UpdatedAt = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	// Save in non-sorted order.
	require.NoError(t, store.Save(middle))
	require.NoError(t, store.Save(old))
	require.NoError(t, store.Save(newest))

	infos, err := store.List()
	require.NoError(t, err)
	require.Len(t, infos, 3)

	// Should be newest first.
	assert.Equal(t, "new-session", infos[0].ID)
	assert.Equal(t, "model-c", infos[0].Model)
	assert.Equal(t, 1, infos[0].MessageCount)
	assert.Equal(t, "mid-session", infos[1].ID)
	assert.Equal(t, 2, infos[1].MessageCount)
	assert.Equal(t, "old-session", infos[2].ID)
}

func TestStoreListEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	infos, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, infos)
}

func TestStoreAutoSave(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sess := newTestSession("auto-test", "gemma4:latest", []model.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
	})

	err := store.AutoSave(sess)
	require.NoError(t, err)

	// Verify the auto-save file exists.
	path := filepath.Join(dir, "_autosave.json")
	_, err = os.Stat(path)
	assert.NoError(t, err, "Auto-save file should exist")
}

func TestStoreAutoSaveIdempotent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sess := newTestSession("idempotent-test", "gemma4:latest", []model.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "First"},
	})

	require.NoError(t, store.AutoSave(sess))

	// Update session content and auto-save again.
	sess.Messages = append(sess.Messages, model.Message{Role: "assistant", Content: "Reply"})
	require.NoError(t, store.AutoSave(sess))

	// Should still have exactly one auto-save file, not two.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	autoSaveCount := 0
	for _, e := range entries {
		if e.Name() == "_autosave.json" {
			autoSaveCount++
		}
	}
	assert.Equal(t, 1, autoSaveCount, "Should have exactly one auto-save file")

	// Content should reflect the latest save.
	loaded, err := store.LoadAutoSave()
	require.NoError(t, err)
	require.Len(t, loaded.Messages, 3, "Auto-save should contain latest messages")
}

func TestStoreAutoSaveSkipsEmptySessions(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Session with only a system message.
	sess := newTestSession("empty-test", "gemma4:latest", []model.Message{
		{Role: "system", Content: "You are helpful."},
	})

	err := store.AutoSave(sess)
	require.NoError(t, err)

	// Auto-save file should NOT exist.
	path := filepath.Join(dir, "_autosave.json")
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "Auto-save should be skipped for system-only sessions")
}

func TestStoreLoadAutoSave(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sess := newTestSession("load-auto-test", "gemma4:latest", []model.Message{
		{Role: "system", Content: "prompt"},
		{Role: "user", Content: "Hello"},
	})
	sess.TokenCount = 55

	require.NoError(t, store.AutoSave(sess))

	loaded, err := store.LoadAutoSave()
	require.NoError(t, err)
	assert.Equal(t, "load-auto-test", loaded.ID)
	assert.Equal(t, 55, loaded.TokenCount)
	require.Len(t, loaded.Messages, 2)
}

func TestStoreLoadAutoSaveNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.LoadAutoSave()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no auto-save")
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Save initial session.
	sess := newTestSession("overwrite-test", "model-a", []model.Message{
		{Role: "user", Content: "original"},
	})
	require.NoError(t, store.Save(sess))

	// Overwrite with different content.
	sess.Messages = []model.Message{
		{Role: "user", Content: "updated"},
	}
	sess.Model = "model-b"
	require.NoError(t, store.Save(sess))

	// Load and verify new content completely replaced old.
	loaded, err := store.Load("overwrite-test")
	require.NoError(t, err)
	assert.Equal(t, "model-b", loaded.Model)
	require.Len(t, loaded.Messages, 1)
	assert.Equal(t, "updated", loaded.Messages[0].Content)
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sess := newTestSession("delete-me", "gemma4:latest", []model.Message{
		{Role: "user", Content: "bye"},
	})
	require.NoError(t, store.Save(sess))

	// Verify file exists before delete.
	path := filepath.Join(dir, "delete-me.json")
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Delete.
	err = store.Delete("delete-me")
	require.NoError(t, err)

	// Verify file is gone.
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "File should be deleted")

	// Delete again should return error.
	err = store.Delete("delete-me")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStoreListExcludesAutoSave(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Save a regular session.
	sess := newTestSession("regular", "model", []model.Message{
		{Role: "user", Content: "hi"},
	})
	require.NoError(t, store.Save(sess))

	// Auto-save a session.
	autoSess := newTestSession("auto", "model", []model.Message{
		{Role: "system", Content: "prompt"},
		{Role: "user", Content: "hello"},
	})
	require.NoError(t, store.AutoSave(autoSess))

	// List should only return the regular session.
	infos, err := store.List()
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "regular", infos[0].ID)
}
