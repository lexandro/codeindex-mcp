package index

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// IndexedFile represents a file that has been indexed.
// Used by both the file path index and the content index.
type IndexedFile struct {
	Path         string    // Absolute file path
	RelativePath string    // Path relative to project root (forward slashes)
	Language     string    // Detected programming language
	SizeBytes    int64     // File size in bytes
	ModTime      time.Time // Last modification time
	LineCount    int       // Number of lines in the file
}

// FileIndex maintains an in-memory index of file paths for fast glob-based searching.
// It uses a map for O(1) path lookups and a sorted slice for glob iteration.
type FileIndex struct {
	mu          sync.RWMutex
	files       map[string]*IndexedFile // key: relative path (forward slashes)
	sortedPaths []string                // sorted for consistent iteration
}

// NewFileIndex creates a new empty file path index.
func NewFileIndex() *FileIndex {
	return &FileIndex{
		files:       make(map[string]*IndexedFile),
		sortedPaths: make([]string, 0),
	}
}

// AddFile adds or updates a file in the index.
func (fi *FileIndex) AddFile(file *IndexedFile) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	_, exists := fi.files[file.RelativePath]
	fi.files[file.RelativePath] = file

	if !exists {
		fi.sortedPaths = append(fi.sortedPaths, file.RelativePath)
		sort.Strings(fi.sortedPaths)
	}
}

// RemoveFile removes a file from the index by its relative path.
func (fi *FileIndex) RemoveFile(relativePath string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if _, exists := fi.files[relativePath]; !exists {
		return
	}

	delete(fi.files, relativePath)

	// Remove from sorted slice
	idx := sort.SearchStrings(fi.sortedPaths, relativePath)
	if idx < len(fi.sortedPaths) && fi.sortedPaths[idx] == relativePath {
		fi.sortedPaths = append(fi.sortedPaths[:idx], fi.sortedPaths[idx+1:]...)
	}
}

// GetFile returns the IndexedFile for a given relative path, or nil if not found.
func (fi *FileIndex) GetFile(relativePath string) *IndexedFile {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.files[relativePath]
}

// FileCount returns the number of indexed files.
func (fi *FileIndex) FileCount() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return len(fi.files)
}

// TotalSizeBytes returns the total size of all indexed files.
func (fi *FileIndex) TotalSizeBytes() int64 {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	var totalSize int64
	for _, file := range fi.files {
		totalSize += file.SizeBytes
	}
	return totalSize
}

// LanguageCounts returns a map of language -> file count for all indexed files.
func (fi *FileIndex) LanguageCounts() map[string]int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	counts := make(map[string]int)
	for _, file := range fi.files {
		counts[file.Language]++
	}
	return counts
}

// SearchResult holds a file match from a glob search.
type FileSearchResult struct {
	File *IndexedFile
}

// SearchByGlob returns files matching a doublestar glob pattern.
// The pattern is matched against relative paths (forward slashes).
func (fi *FileIndex) SearchByGlob(pattern string, maxResults int) ([]FileSearchResult, error) {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 50
	}

	// Normalize pattern to forward slashes
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	// Validate the pattern
	if !doublestar.ValidatePattern(pattern) {
		return nil, fmt.Errorf("invalid glob pattern: %s", pattern)
	}

	var results []FileSearchResult
	for _, path := range fi.sortedPaths {
		if len(results) >= maxResults {
			break
		}

		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			continue
		}
		if matched {
			if file, ok := fi.files[path]; ok {
				results = append(results, FileSearchResult{File: file})
			}
		}
	}

	return results, nil
}

// AllFiles returns all indexed files in sorted order. Use with caution on large indexes.
func (fi *FileIndex) AllFiles() []*IndexedFile {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	result := make([]*IndexedFile, 0, len(fi.sortedPaths))
	for _, path := range fi.sortedPaths {
		if file, ok := fi.files[path]; ok {
			result = append(result, file)
		}
	}
	return result
}

// Clear removes all files from the index.
func (fi *FileIndex) Clear() {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.files = make(map[string]*IndexedFile)
	fi.sortedPaths = make([]string, 0)
}
