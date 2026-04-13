package config

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches a config file's parent directory for changes and
// invokes the onChange callback when the config file is written or created.
// It debounces rapid events to a single callback invocation.
type ConfigWatcher struct {
	watcher    *fsnotify.Watcher
	configPath string
	onChange   func()
	done       chan struct{}
}

// NewConfigWatcher creates a ConfigWatcher that watches the parent directory
// of configPath for changes. When the config file is written or created, the
// onChange callback is invoked after a 100ms debounce period.
// The watcher starts immediately.
func NewConfigWatcher(configPath string, onChange func()) (*ConfigWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cw := &ConfigWatcher{
		watcher:    w,
		configPath: configPath,
		onChange:   onChange,
		done:       make(chan struct{}),
	}

	if err := cw.start(); err != nil {
		w.Close()
		return nil, err
	}

	return cw, nil
}

// start begins watching the config file's parent directory.
func (cw *ConfigWatcher) start() error {
	dir := filepath.Dir(cw.configPath)
	if err := cw.watcher.Add(dir); err != nil {
		return err
	}

	go func() {
		var debounceTimer *time.Timer
		for {
			select {
			case event, ok := <-cw.watcher.Events:
				if !ok {
					return
				}
				// Filter: only react when the event is for our config file.
				if filepath.Clean(event.Name) != filepath.Clean(cw.configPath) {
					continue
				}
				// Filter: only react to Write or Create events.
				// Create handles editor atomic saves (write temp + rename).
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
					continue
				}
				// Debounce: reset timer on each matching event.
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(100*time.Millisecond, cw.onChange)

			case err, ok := <-cw.watcher.Errors:
				if !ok {
					return
				}
				slog.Error("config watcher error", "error", err)

			case <-cw.done:
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
		}
	}()

	return nil
}

// Stop stops the config watcher and releases resources.
func (cw *ConfigWatcher) Stop() error {
	close(cw.done)
	return cw.watcher.Close()
}
