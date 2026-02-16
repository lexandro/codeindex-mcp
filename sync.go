package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/lexandro/codeindex-mcp/ignore"
	"github.com/lexandro/codeindex-mcp/index"
)

// SyncResult holds the outcome of a single sync verification run.
type SyncResult struct {
	MissingFiles  int // files on disk but not in index
	StaleFiles    int // files in index but not on disk
	ModifiedFiles int // files where ModTime differs
	Duration      time.Duration
}

// runPeriodicSync starts a background loop that verifies index consistency at the given interval.
// It runs until the provided stop channel is closed.
func runPeriodicSync(
	intervalSeconds int,
	rootDir string,
	fileIndex *index.FileIndex,
	contentIndex *index.ContentIndex,
	ignoreMatcher *ignore.Matcher,
	logger *slog.Logger,
	stop <-chan struct{},
) {
	interval := time.Duration(intervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("periodic sync started", "intervalSeconds", intervalSeconds)

	for {
		select {
		case <-stop:
			logger.Info("periodic sync stopped")
			return
		case <-ticker.C:
			result := performSyncVerification(rootDir, fileIndex, contentIndex, ignoreMatcher, logger)
			totalDiscrepancies := result.MissingFiles + result.StaleFiles + result.ModifiedFiles
			if totalDiscrepancies > 0 {
				logger.Info("sync verification complete",
					"missing", result.MissingFiles,
					"stale", result.StaleFiles,
					"modified", result.ModifiedFiles,
					"duration", result.Duration,
				)
			} else {
				logger.Debug("sync verification complete, index is in sync", "duration", result.Duration)
			}
		}
	}
}

// performSyncVerification compares the filesystem with the current index state
// and re-indexes any out-of-sync files.
func performSyncVerification(
	rootDir string,
	fileIndex *index.FileIndex,
	contentIndex *index.ContentIndex,
	ignoreMatcher *ignore.Matcher,
	logger *slog.Logger,
) SyncResult {
	start := time.Now()
	var result SyncResult

	// Step 1: Build a set of all files currently on disk
	diskFiles := make(map[string]os.FileInfo) // key: relative path (forward slashes)
	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != rootDir && ignoreMatcher.ShouldIgnoreDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoreMatcher.ShouldIgnore(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if ignoreMatcher.IsFileTooLarge(info.Size()) {
			return nil
		}
		relPath, _ := filepath.Rel(rootDir, path)
		relPath = filepath.ToSlash(relPath)
		diskFiles[relPath] = info
		return nil
	})

	// Step 2: Get all currently indexed files
	indexedFiles := fileIndex.AllFiles()
	indexedSet := make(map[string]*index.IndexedFile, len(indexedFiles))
	for _, f := range indexedFiles {
		indexedSet[f.RelativePath] = f
	}

	// Step 3: Find missing files (on disk but not in index)
	for relPath, info := range diskFiles {
		if _, exists := indexedSet[relPath]; !exists {
			absPath := filepath.Join(rootDir, filepath.FromSlash(relPath))
			err := indexSingleFile(absPath, relPath, info, rootDir, fileIndex, contentIndex, ignoreMatcher)
			if err != nil {
				logger.Debug("sync: skipped missing file", "path", relPath, "error", err)
				continue
			}
			logger.Info("sync: indexed missing file", "path", relPath)
			result.MissingFiles++
		}
	}

	// Step 4: Find stale files (in index but not on disk)
	for relPath := range indexedSet {
		if _, exists := diskFiles[relPath]; !exists {
			fileIndex.RemoveFile(relPath)
			contentIndex.RemoveFile(relPath)
			logger.Info("sync: removed stale file", "path", relPath)
			result.StaleFiles++
		}
	}

	// Step 5: Find modified files (ModTime differs)
	for relPath, info := range diskFiles {
		indexed, exists := indexedSet[relPath]
		if !exists {
			continue // already handled as missing
		}
		if !info.ModTime().Equal(indexed.ModTime) {
			absPath := filepath.Join(rootDir, filepath.FromSlash(relPath))
			err := indexSingleFile(absPath, relPath, info, rootDir, fileIndex, contentIndex, ignoreMatcher)
			if err != nil {
				logger.Debug("sync: skipped modified file", "path", relPath, "error", err)
				continue
			}
			logger.Info("sync: re-indexed modified file", "path", relPath)
			result.ModifiedFiles++
		}
	}

	result.Duration = time.Since(start)
	return result
}
