package index

import (
	"fmt"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
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
func (ci *ContentIndex) RemoveFile(relativePath string) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	delete(ci.fileContents, relativePath)
	if err := ci.index.Delete(relativePath); err != nil {
		return fmt.Errorf("removing file %s from index: %w", relativePath, err)
	}
	return nil
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

// Search performs a full-text search across all indexed files.
// Query format:
//   - Plain text: match query (word-level matching)
//   - "quoted text": phrase query (exact phrase match)
//   - /regex/: regexp query
func (ci *ContentIndex) Search(options SearchOptions) ([]ContentSearchResult, int, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if options.MaxResults <= 0 {
		options.MaxResults = 50
	}
	if options.ContextLines < 0 {
		options.ContextLines = 0
	}

	bleveQuery := buildQuery(options.Query)

	searchRequest := bleve.NewSearchRequest(bleveQuery)
	searchRequest.Size = options.MaxResults * 5 // Get more results because we'll filter and group by file
	searchRequest.Fields = []string{"path", "language"}

	searchResults, err := ci.index.Search(searchRequest)
	if err != nil {
		return nil, 0, fmt.Errorf("searching index: %w", err)
	}

	// Group results by file and find matching lines
	resultMap := make(map[string]*ContentSearchResult)
	var orderedPaths []string
	totalMatches := 0

	// Normalize FilePath: backslash to forward slash for cross-platform consistency
	normalizedFilePath := strings.ReplaceAll(options.FilePath, "\\", "/")

	for _, hit := range searchResults.Hits {
		relativePath := hit.ID
		content, ok := ci.fileContents[relativePath]
		if !ok {
			continue
		}

		// Apply file path filter (exact match, overrides FileGlob)
		if normalizedFilePath != "" {
			if relativePath != normalizedFilePath {
				continue
			}
		} else if options.FileGlob != "" {
			// Apply file glob filter if specified
			matched := matchSimpleGlob(relativePath, options.FileGlob)
			if !matched {
				continue
			}
		}

		// Find actual matching lines in the content
		lineMatches := findMatchingLines(content, options.Query, options.ContextLines)
		if len(lineMatches) == 0 {
			continue
		}

		totalMatches += len(lineMatches)

		if _, exists := resultMap[relativePath]; !exists {
			resultMap[relativePath] = &ContentSearchResult{
				RelativePath: relativePath,
			}
			orderedPaths = append(orderedPaths, relativePath)
		}
		resultMap[relativePath].Matches = append(resultMap[relativePath].Matches, lineMatches...)

		if len(orderedPaths) >= options.MaxResults {
			break
		}
	}

	results := make([]ContentSearchResult, 0, len(orderedPaths))
	for _, path := range orderedPaths {
		results = append(results, *resultMap[path])
	}

	return results, totalMatches, nil
}

// buildQuery parses the query string into a Bleve query.
func buildQuery(queryString string) query.Query {
	queryString = strings.TrimSpace(queryString)

	// Regex query: /pattern/
	if strings.HasPrefix(queryString, "/") && strings.HasSuffix(queryString, "/") && len(queryString) > 2 {
		regexPattern := queryString[1 : len(queryString)-1]
		return bleve.NewRegexpQuery(regexPattern)
	}

	// Phrase query: "exact phrase"
	if strings.HasPrefix(queryString, "\"") && strings.HasSuffix(queryString, "\"") && len(queryString) > 2 {
		phrase := queryString[1 : len(queryString)-1]
		return bleve.NewMatchPhraseQuery(phrase)
	}

	// Default: match query (word-level)
	return bleve.NewMatchQuery(queryString)
}

// findMatchingLines searches content line by line for the query terms.
// Returns LineMatch entries with context lines.
func findMatchingLines(content string, queryString string, contextLines int) []LineMatch {
	lines := strings.Split(content, "\n")
	searchTerm := extractSearchTerm(queryString)
	searchTermLower := strings.ToLower(searchTerm)

	var matches []LineMatch

	for lineIdx, line := range lines {
		lineLower := strings.ToLower(line)
		if !strings.Contains(lineLower, searchTermLower) {
			continue
		}

		match := LineMatch{
			LineNumber: lineIdx + 1, // 1-based
			LineText:   line,
		}

		// Gather context lines before
		if contextLines > 0 {
			startCtx := lineIdx - contextLines
			if startCtx < 0 {
				startCtx = 0
			}
			for i := startCtx; i < lineIdx; i++ {
				match.ContextBefore = append(match.ContextBefore, lines[i])
			}
		}

		// Gather context lines after
		if contextLines > 0 {
			endCtx := lineIdx + contextLines + 1
			if endCtx > len(lines) {
				endCtx = len(lines)
			}
			for i := lineIdx + 1; i < endCtx; i++ {
				match.ContextAfter = append(match.ContextAfter, lines[i])
			}
		}

		matches = append(matches, match)
	}

	return matches
}

// extractSearchTerm strips query syntax to get the raw search term for line matching.
func extractSearchTerm(queryString string) string {
	queryString = strings.TrimSpace(queryString)

	// Strip regex delimiters
	if strings.HasPrefix(queryString, "/") && strings.HasSuffix(queryString, "/") && len(queryString) > 2 {
		return queryString[1 : len(queryString)-1]
	}

	// Strip phrase quotes
	if strings.HasPrefix(queryString, "\"") && strings.HasSuffix(queryString, "\"") && len(queryString) > 2 {
		return queryString[1 : len(queryString)-1]
	}

	return queryString
}

// matchSimpleGlob is a basic glob matcher for file filtering within search results.
func matchSimpleGlob(path string, pattern string) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	// Handle **/ prefix
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		if strings.HasSuffix(path, suffix) || strings.Contains(path, "/"+suffix) {
			return true
		}
		// Try matching just the extension part
		if strings.HasPrefix(suffix, "*.") {
			ext := suffix[1:] // e.g., ".go"
			return strings.HasSuffix(path, ext)
		}
	}

	// Handle *.ext pattern
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // e.g., ".go"
		return strings.HasSuffix(path, ext)
	}

	// Direct substring match as fallback
	return strings.Contains(path, pattern)
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
