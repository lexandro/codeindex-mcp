package index

import (
	"strings"
	"sync"
)

// ContentIndex provides exact, grep-style search over file contents held in memory.
// All content is stored in RAM and queries scan the stored lines directly, so
// results have full substring and regex recall (no tokenizer false negatives).
type ContentIndex struct {
	mu    sync.RWMutex
	files map[string]*indexedContent // key: relative path (forward slashes)
}

// indexedContent holds one file's raw content and its pre-split lines.
// Lines share the backing array of content, so the memory cost is one copy.
type indexedContent struct {
	language string
	content  string
	lines    []string
}

// NewContentIndex creates a new empty in-memory content index.
// The error return value is kept for API stability; it is always nil.
func NewContentIndex() (*ContentIndex, error) {
	return &ContentIndex{
		files: make(map[string]*indexedContent),
	}, nil
}

// ContentSearchResult holds all matches within one file.
type ContentSearchResult struct {
	RelativePath string
	MatchCount   int    // total matching lines in the file (before the per-file display cap)
	Hunks        []Hunk // context-merged display hunks, capped at MaxMatchesPerFile matches
}

// Hunk is a contiguous block of lines containing one or more matches plus context.
// Overlapping or adjacent match contexts are merged into a single hunk.
type Hunk struct {
	StartLine int // 1-based line number of Lines[0]
	Lines     []string
	IsMatch   []bool // parallel to Lines; true for lines that match the query
}

// SearchOptions configures a content search.
type SearchOptions struct {
	Query             string
	FilePath          string // Exact relative path to restrict search to a single file (overrides FileGlob)
	FileGlob          string
	MaxResults        int  // max number of files returned (default 50)
	ContextLines      int  // context lines before and after each match
	MaxMatchesPerFile int  // max matches rendered per file (default 10)
	CaseSensitive     bool // case-sensitive matching for plain text and regex queries
}

// IndexFile adds or updates a file's content in the index.
// The error return value is kept for API stability; it is always nil.
func (ci *ContentIndex) IndexFile(relativePath string, content string, language string) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	ci.files[relativePath] = &indexedContent{
		language: language,
		content:  content,
		lines:    strings.Split(content, "\n"),
	}
	return nil
}

// RemoveFile removes a file from the index.
func (ci *ContentIndex) RemoveFile(relativePath string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	delete(ci.files, relativePath)
}

// DocumentCount returns the number of files in the content index.
func (ci *ContentIndex) DocumentCount() uint64 {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return uint64(len(ci.files))
}

// Close releases resources. The in-memory index has nothing to release;
// kept for API stability.
func (ci *ContentIndex) Close() error {
	return nil
}

// GetFileContent returns the raw content of an indexed file.
// Returns the content and true if found, or empty string and false if not indexed.
func (ci *ContentIndex) GetFileContent(relativePath string) (string, bool) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	normalizedPath := strings.ReplaceAll(relativePath, "\\", "/")
	entry, ok := ci.files[normalizedPath]
	if !ok {
		return "", false
	}
	return entry.content, true
}

// Clear removes all files from the index.
// The error return value is kept for API stability; it is always nil.
func (ci *ContentIndex) Clear() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.files = make(map[string]*indexedContent)
	return nil
}
