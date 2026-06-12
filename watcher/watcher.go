package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// IgnoreChecker is used by the watcher to check if a path should be ignored.
type IgnoreChecker interface {
	ShouldIgnoreDir(absolutePath string) bool
	ShouldIgnore(absolutePath string) bool
}

// Watcher provides recursive file system watching with debouncing.
type Watcher struct {
	fsWatcher     *fsnotify.Watcher
	debouncer     *Debouncer
	ignoreChecker IgnoreChecker
	rootDir       string
	logger        *slog.Logger
}

// NewWatcher creates a recursive file watcher on the given root directory.
// It registers all non-ignored subdirectories for watching.
func NewWatcher(rootDir string, ignoreChecker IgnoreChecker, logger *slog.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher:     fsWatcher,
		debouncer:     NewDebouncer(100 * time.Millisecond),
		ignoreChecker: ignoreChecker,
		rootDir:       rootDir,
		logger:        logger,
	}

	// Walk directory tree and add all non-ignored directories to the watcher
	err = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip entries that can't be read
		}
		if !d.IsDir() {
			return nil
		}
		if path != rootDir && ignoreChecker.ShouldIgnoreDir(path) {
			return filepath.SkipDir
		}
		if watchErr := fsWatcher.Add(path); watchErr != nil {
			w.logger.Warn("failed to watch directory", "path", path, "error", watchErr)
		}
		return nil
	})
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}

	return w, nil
}

// Events returns the channel that receives debounced file system events.
func (w *Watcher) Events() <-chan []DebouncedEvent {
	return w.debouncer.Output()
}

// Start begins listening for file system events. Call this in a goroutine.
// It runs until the watcher is closed.
func (w *Watcher) Start() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn("watcher error", "error", err)
		}
	}
}

// handleEvent processes a single fsnotify event, converting it to a debounced event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// If a new directory was created, watch it recursively and index any files
	// already inside it (a copied/extracted tree arrives faster than watches attach).
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			if !w.ignoreChecker.ShouldIgnoreDir(path) {
				w.watchRecursive(path)
			}
			return // Don't emit events for directory creation itself
		}
	}

	// Skip ignored files
	if w.ignoreChecker.ShouldIgnore(path) {
		return
	}

	var op EventOp
	switch {
	case event.Has(fsnotify.Create):
		op = OpCreate
	case event.Has(fsnotify.Write):
		op = OpWrite
	case event.Has(fsnotify.Remove):
		op = OpRemove
	case event.Has(fsnotify.Rename):
		op = OpRename
	default:
		return
	}

	w.debouncer.Add(path, op)
}

// watchRecursive adds watches for root and all non-ignored subdirectories,
// and emits create events for files already present so they get indexed.
func (w *Watcher) watchRecursive(root string) {
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && w.ignoreChecker.ShouldIgnoreDir(path) {
				return filepath.SkipDir
			}
			if addErr := w.fsWatcher.Add(path); addErr != nil {
				w.logger.Warn("failed to watch new directory", "path", path, "error", addErr)
			}
			return nil
		}
		if !w.ignoreChecker.ShouldIgnore(path) {
			w.debouncer.Add(path, OpCreate)
		}
		return nil
	})
}

// WatchedDirCount returns the number of directories currently being watched.
func (w *Watcher) WatchedDirCount() int {
	return len(w.fsWatcher.WatchList())
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	return w.fsWatcher.Close()
}
