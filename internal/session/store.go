package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const autoSaveFilename = "_autosave.json"

// Store manages session persistence to the filesystem.
type Store struct {
	dir string // Sessions directory path
}

// NewStore creates a Store for the given directory.
// The directory must already exist.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save persists a session to disk as {id}.json using atomic writes.
func (s *Store) Save(sess *Session) error {
	path := filepath.Join(s.dir, sess.ID+".json")
	return atomicWriteJSON(path, sess)
}

// Load reads a session from disk by ID.
func (s *Store) Load(id string) (*Session, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", id)
		}
		return nil, fmt.Errorf("reading session %q: %w", id, err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parsing session %q: %w", id, err)
	}
	return &sess, nil
}

// Delete removes a session file from disk.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session %q not found", id)
		}
		return fmt.Errorf("deleting session %q: %w", id, err)
	}
	return nil
}

// List returns summaries of all saved sessions, sorted by UpdatedAt descending (newest first).
// Excludes the auto-save file.
func (s *Store) List() ([]SessionInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	var infos []SessionInfo
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || name == autoSaveFilename {
			continue
		}

		path := filepath.Join(s.dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue // Skip corrupt files
		}

		infos = append(infos, SessionInfo{
			ID:           sess.ID,
			Model:        sess.Model,
			UpdatedAt:    sess.UpdatedAt,
			MessageCount: len(sess.Messages),
		})
	}

	// Sort newest first.
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})

	return infos, nil
}

// AutoSave saves the session to the auto-save file.
// Skips if session has no user content (system prompt only).
// This is idempotent -- subsequent calls overwrite the previous auto-save.
func (s *Store) AutoSave(sess *Session) error {
	if !sess.HasContent() {
		return nil // Nothing to save
	}
	path := filepath.Join(s.dir, autoSaveFilename)
	return atomicWriteJSON(path, sess)
}

// LoadAutoSave loads the auto-saved session.
// Returns an error if no auto-save exists.
func (s *Store) LoadAutoSave() (*Session, error) {
	path := filepath.Join(s.dir, autoSaveFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no auto-save found")
		}
		return nil, fmt.Errorf("reading auto-save: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parsing auto-save: %w", err)
	}
	return &sess, nil
}

// atomicWriteJSON writes a value as indented JSON to a file atomically.
// Uses temp file + rename to prevent corruption on crash.
func atomicWriteJSON(path string, v any) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".fenec-session-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()

	// Cleanup on failure.
	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		return fmt.Errorf("encoding JSON: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("syncing file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	tmpPath = "" // Prevent deferred cleanup
	return nil
}
