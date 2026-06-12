package index

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Search scans all indexed file contents line by line and returns grep-equivalent results.
// Query format:
//   - Plain text: literal substring match, case-insensitive by default
//   - "quoted text": exact literal match, always case-sensitive
//   - /regex/: Go (RE2) regular expression matched per line, case-insensitive by default
//
// The boolean return value reports whether the file limit (MaxResults) was reached
// while more matching files remained.
func (ci *ContentIndex) Search(options SearchOptions) ([]ContentSearchResult, int, bool, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if options.MaxResults <= 0 {
		options.MaxResults = 50
	}
	if options.ContextLines < 0 {
		options.ContextLines = 0
	}
	if options.MaxMatchesPerFile <= 0 {
		options.MaxMatchesPerFile = 10
	}

	matcher, err := buildLineMatcher(options.Query, options.CaseSensitive)
	if err != nil {
		return nil, 0, false, fmt.Errorf("invalid query: %w", err)
	}

	paths, err := ci.candidatePaths(options)
	if err != nil {
		return nil, 0, false, err
	}

	var results []ContentSearchResult
	totalMatches := 0
	truncated := false

	for _, path := range paths {
		entry := ci.files[path]
		matchLines := matchingLineIndexes(entry.lines, matcher)
		if len(matchLines) == 0 {
			continue
		}
		if len(results) >= options.MaxResults {
			truncated = true
			break
		}

		totalMatches += len(matchLines)

		displayed := matchLines
		if len(displayed) > options.MaxMatchesPerFile {
			displayed = displayed[:options.MaxMatchesPerFile]
		}

		results = append(results, ContentSearchResult{
			RelativePath: path,
			MatchCount:   len(matchLines),
			Hunks:        buildHunks(entry.lines, displayed, matchLines, options.ContextLines),
		})
	}

	return results, totalMatches, truncated, nil
}

// buildLineMatcher compiles the query string into a line matcher regexp.
// Plain text and /regex/ queries are case-insensitive unless caseSensitive is true.
// "quoted" queries are exact literal matches and always case-sensitive.
func buildLineMatcher(queryString string, caseSensitive bool) (*regexp.Regexp, error) {
	queryString = strings.TrimSpace(queryString)

	var pattern string
	switch {
	case strings.HasPrefix(queryString, "/") && strings.HasSuffix(queryString, "/") && len(queryString) > 2:
		pattern = queryString[1 : len(queryString)-1]
	case strings.HasPrefix(queryString, "\"") && strings.HasSuffix(queryString, "\"") && len(queryString) > 2:
		return regexp.Compile(regexp.QuoteMeta(queryString[1 : len(queryString)-1]))
	default:
		pattern = regexp.QuoteMeta(queryString)
	}

	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

// candidatePaths returns the sorted list of indexed paths that pass the
// FilePath / FileGlob filters. Caller must hold the read lock.
func (ci *ContentIndex) candidatePaths(options SearchOptions) ([]string, error) {
	// Exact file path overrides glob filtering
	if options.FilePath != "" {
		normalizedPath := strings.ReplaceAll(options.FilePath, "\\", "/")
		if _, ok := ci.files[normalizedPath]; ok {
			return []string{normalizedPath}, nil
		}
		return nil, nil
	}

	var normalizedGlob string
	if options.FileGlob != "" {
		normalizedGlob = strings.ReplaceAll(options.FileGlob, "\\", "/")
		if !doublestar.ValidatePattern(normalizedGlob) {
			return nil, fmt.Errorf("invalid glob pattern: %s", options.FileGlob)
		}
	}

	paths := make([]string, 0, len(ci.files))
	for path := range ci.files {
		if normalizedGlob != "" {
			matched, _ := doublestar.Match(normalizedGlob, path)
			if !matched {
				continue
			}
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

// matchingLineIndexes returns the 0-based indexes of all lines matching the query.
func matchingLineIndexes(lines []string, matcher *regexp.Regexp) []int {
	var matches []int
	for lineIdx, line := range lines {
		if matcher.MatchString(line) {
			matches = append(matches, lineIdx)
		}
	}
	return matches
}

// buildHunks groups the displayed matches into contiguous hunks with context lines.
// Overlapping or adjacent context ranges are merged. allMatches is used to mark
// match lines that fall inside a hunk even when they are beyond the display cap.
func buildHunks(lines []string, displayedMatches []int, allMatches []int, contextLines int) []Hunk {
	if len(displayedMatches) == 0 {
		return nil
	}

	matchSet := make(map[int]bool, len(allMatches))
	for _, matchIdx := range allMatches {
		matchSet[matchIdx] = true
	}

	type span struct{ start, end int }
	var spans []span
	for _, matchIdx := range displayedMatches {
		start := matchIdx - contextLines
		if start < 0 {
			start = 0
		}
		end := matchIdx + contextLines
		if end > len(lines)-1 {
			end = len(lines) - 1
		}
		if len(spans) > 0 && start <= spans[len(spans)-1].end+1 {
			spans[len(spans)-1].end = end
		} else {
			spans = append(spans, span{start: start, end: end})
		}
	}

	hunks := make([]Hunk, 0, len(spans))
	for _, s := range spans {
		hunk := Hunk{StartLine: s.start + 1}
		for lineIdx := s.start; lineIdx <= s.end; lineIdx++ {
			hunk.Lines = append(hunk.Lines, lines[lineIdx])
			hunk.IsMatch = append(hunk.IsMatch, matchSet[lineIdx])
		}
		hunks = append(hunks, hunk)
	}
	return hunks
}
