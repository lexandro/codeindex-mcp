package index

import (
	"fmt"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/bmatcuk/doublestar/v4"
)

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
			normalizedGlob := strings.ReplaceAll(options.FileGlob, "\\", "/")
			matched, matchErr := doublestar.Match(normalizedGlob, relativePath)
			if matchErr != nil || !matched {
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
