package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/lexandro/codeindex-mcp/ignore"
	"github.com/lexandro/codeindex-mcp/index"
	"github.com/lexandro/codeindex-mcp/language"
	"github.com/lexandro/codeindex-mcp/watcher"
)

// performIndexing walks the root directory and indexes all eligible files.
// Returns the number of files indexed and total bytes processed.
func performIndexing(
	rootDir string,
	fileIndex *index.FileIndex,
	contentIndex *index.ContentIndex,
	ignoreMatcher *ignore.Matcher,
	logger *slog.Logger,
) (int, int64) {
	var indexedCount int
	var totalSize int64
	var mu sync.Mutex

	// Use a bounded worker pool for parallel file reading
	const workerCount = 8
	type indexJob struct {
		path    string
		relPath string
		info    os.FileInfo
	}
	jobs := make(chan indexJob, 100)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := indexSingleFile(job.path, job.relPath, job.info, rootDir, fileIndex, contentIndex, ignoreMatcher); err != nil {
					logger.Debug("skipped file", "path", job.relPath, "error", err)
					continue
				}
				mu.Lock()
				indexedCount++
				totalSize += job.info.Size()
				mu.Unlock()
			}
		}()
	}

	// Walk directory tree
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
		jobs <- indexJob{path: path, relPath: relPath, info: info}
		return nil
	})

	close(jobs)
	wg.Wait()
	return indexedCount, totalSize
}

// indexSingleFile reads and indexes one file into both indexes.
func indexSingleFile(
	absolutePath string,
	relativePath string,
	info os.FileInfo,
	rootDir string,
	fileIndex *index.FileIndex,
	contentIndex *index.ContentIndex,
	ignoreMatcher *ignore.Matcher,
) error {
	// Read file content with retry for Windows file locking
	content, err := readFileWithRetry(absolutePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Skip binary files
	if language.IsBinaryContent(content) {
		return fmt.Errorf("binary file")
	}

	contentStr := string(content)
	lineCount := strings.Count(contentStr, "\n") + 1
	lang := language.DetectLanguage(absolutePath)

	// Add to file index
	indexedFile := &index.IndexedFile{
		Path:         absolutePath,
		RelativePath: relativePath,
		Language:     lang,
		SizeBytes:    info.Size(),
		ModTime:      info.ModTime(),
		LineCount:    lineCount,
	}
	fileIndex.AddFile(indexedFile)

	// Add to content index
	if err := contentIndex.IndexFile(relativePath, contentStr, lang); err != nil {
		return fmt.Errorf("indexing content: %w", err)
	}

	return nil
}

// readFileWithRetry attempts to read a file, retrying once after a short delay
// if the file is locked (common on Windows when editors are saving).
func readFileWithRetry(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Retry after 50ms for Windows file locking
		time.Sleep(50 * time.Millisecond)
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

// handleWatcherEvents processes debounced file system events and updates the indexes.
func handleWatcherEvents(
	fileWatcher *watcher.Watcher,
	rootDir string,
	fileIndex *index.FileIndex,
	contentIndex *index.ContentIndex,
	ignoreMatcher *ignore.Matcher,
	logger *slog.Logger,
) {
	for events := range fileWatcher.Events() {
		for _, event := range events {
			relPath, _ := filepath.Rel(rootDir, event.Path)
			relPath = filepath.ToSlash(relPath)

			switch event.Op {
			case watcher.OpRemove, watcher.OpRename:
				fileIndex.RemoveFile(relPath)
				contentIndex.RemoveFile(relPath)
				logger.Debug("removed from index", "path", relPath)

			case watcher.OpCreate, watcher.OpWrite:
				// Check if this is a .gitignore or .claudeignore change
				baseName := filepath.Base(event.Path)
				if baseName == ".gitignore" || baseName == ".claudeignore" {
					ignoreMatcher.Reload()
					logger.Info("reloaded ignore rules", "trigger", baseName)
					continue
				}

				if ignoreMatcher.ShouldIgnore(event.Path) {
					continue
				}

				info, err := os.Stat(event.Path)
				if err != nil {
					continue
				}
				if info.IsDir() {
					continue
				}
				if ignoreMatcher.IsFileTooLarge(info.Size()) {
					continue
				}

				err = indexSingleFile(event.Path, relPath, info, rootDir, fileIndex, contentIndex, ignoreMatcher)
				if err != nil {
					logger.Debug("skipped file update", "path", relPath, "error", err)
					continue
				}
				logger.Debug("updated index", "path", relPath)
			}
		}
	}
}
