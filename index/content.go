package index

import (
	"fmt"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// ContentIndex provides full-text search over file contents using Bleve in-memory index.
type ContentIndex struct {
	mu    sync.RWMutex
	index bleve.Index
	// fileContents stores raw content for line-level result extraction
	fileContents map[string]string // key: relative path, value: file content
}

// NewContentIndex creates a new in-memory Bleve content index.
func NewContentIndex() (*ContentIndex, error) {
	indexMapping := buildIndexMapping()
	bleveIndex, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return nil, fmt.Errorf("creating bleve index: %w", err)
	}

	return &ContentIndex{
		index:        bleveIndex,
		fileContents: make(map[string]string),
	}, nil
}

// bleveDocument is the document structure stored in Bleve.
type bleveDocument struct {
	Content  string `json:"content"`
	Path     string `json:"path"`
	Language string `json:"language"`
}

// buildIndexMapping creates the Bleve index mapping for code content.
func buildIndexMapping() *mapping.IndexMappingImpl {
	indexMapping := bleve.NewIndexMapping()

	// Use a simple mapping - let Bleve handle the tokenization
	docMapping := bleve.NewDocumentMapping()

	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Store = false // Don't store content in Bleve; we keep it in fileContents
	contentFieldMapping.IncludeInAll = true
	docMapping.AddFieldMappingsAt("content", contentFieldMapping)

	pathFieldMapping := bleve.NewTextFieldMapping()
	pathFieldMapping.Store = true
	pathFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("path", pathFieldMapping)

	langFieldMapping := bleve.NewKeywordFieldMapping()
	langFieldMapping.Store = true
	langFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("language", langFieldMapping)

	indexMapping.DefaultMapping = docMapping
	return indexMapping
}

// ContentSearchResult holds a search match within a file.
type ContentSearchResult struct {
	RelativePath string
	Matches      []LineMatch
}

// LineMatch represents a single line match within a file.
type LineMatch struct {
	LineNumber int
	LineText   string
	// Context lines before and after the match
	ContextBefore []string
	ContextAfter  []string
}

// SearchOptions configures a content search.
type SearchOptions struct {
	Query        string
	FilePath     string // Exact relative path to restrict search to a single file (overrides FileGlob)
	FileGlob     string
	MaxResults   int
	ContextLines int
}

// IndexFile adds or updates a file's content in the search index.
func (ci *ContentIndex) IndexFile(relativePath string, content string, language string) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	doc := bleveDocument{
		Content:  content,
		Path:     relativePath,
		Language: language,
	}

	ci.fileContents[relativePath] = content

	if err := ci.index.Index(relativePath, doc); err != nil {
		return fmt.Errorf("indexing file %s: %w", relativePath, err)
	}
	return nil
}

// RemoveFile removes a file from the search index.
func (ci *ContentIndex) RemoveFile(relativePath string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	delete(ci.fileContents, relativePath)
	ci.index.Delete(relativePath)
}

// DocumentCount returns the number of documents in the Bleve index.
func (ci *ContentIndex) DocumentCount() uint64 {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	count, _ := ci.index.DocCount()
	return count
}

// Close closes the Bleve index.
func (ci *ContentIndex) Close() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	return ci.index.Close()
}

// GetFileContent returns the raw content of an indexed file.
// Returns the content and true if found, or empty string and false if not indexed.
func (ci *ContentIndex) GetFileContent(relativePath string) (string, bool) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	normalizedPath := strings.ReplaceAll(relativePath, "\\", "/")
	content, ok := ci.fileContents[normalizedPath]
	return content, ok
}

// Clear removes all documents and recreates the index.
func (ci *ContentIndex) Clear() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if err := ci.index.Close(); err != nil {
		return fmt.Errorf("closing old index: %w", err)
	}

	indexMapping := buildIndexMapping()
	newIndex, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return fmt.Errorf("creating new index: %w", err)
	}

	ci.index = newIndex
	ci.fileContents = make(map[string]string)
	return nil
}
