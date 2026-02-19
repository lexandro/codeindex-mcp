package tools

import (
	"fmt"
	"strings"

	"github.com/lexandro/codeindex-mcp/index"
)

// FormatSearchResults formats content search results for AI consumption.
func FormatSearchResults(results []index.ContentSearchResult, totalMatches int) string {
	if len(results) == 0 {
		return "No matches found."
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%d matches in %d files:\n", totalMatches, len(results)))

	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("%s\n", result.RelativePath))

		for _, match := range result.Matches {
			for _, ctxLine := range match.ContextBefore {
				builder.WriteString(fmt.Sprintf("  %s\n", ctxLine))
			}
			builder.WriteString(fmt.Sprintf("  %d: %s\n", match.LineNumber, match.LineText))
			for _, ctxLine := range match.ContextAfter {
				builder.WriteString(fmt.Sprintf("  %s\n", ctxLine))
			}
		}
	}

	return builder.String()
}

// FormatFileResults formats file search results for AI consumption.
func FormatFileResults(results []index.FileSearchResult, nameOnly bool) string {
	if len(results) == 0 {
		return "No files matched."
	}

	var builder strings.Builder
	for _, result := range results {
		if nameOnly {
			builder.WriteString(result.File.RelativePath)
			builder.WriteString("\n")
		} else {
			builder.WriteString(fmt.Sprintf("%s (%s, %s, %dL)\n",
				result.File.RelativePath,
				result.File.Language,
				formatFileSize(result.File.SizeBytes),
				result.File.LineCount,
			))
		}
	}

	return builder.String()
}

// FormatFileContent formats a file's content with line numbers for AI consumption.
// offset: 1-based starting line (0 = from beginning). limit: max lines (0 = all).
// Line numbers in the output reflect actual file positions, not local indices.
func FormatFileContent(content string, offset, limit int) string {
	lines := strings.Split(content, "\n")

	startIdx := 0
	if offset > 1 {
		startIdx = offset - 1
	}
	if startIdx >= len(lines) {
		return "Offset exceeds file length.\n"
	}
	lines = lines[startIdx:]

	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	firstLineNum := startIdx + 1
	lastLineNum := firstLineNum + len(lines) - 1
	width := len(fmt.Sprintf("%d", lastLineNum))

	var builder strings.Builder
	for i, line := range lines {
		builder.WriteString(fmt.Sprintf("%*d: %s\n", width, firstLineNum+i, line))
	}
	return builder.String()
}

// formatFileSize converts bytes to a human-readable string.
func formatFileSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
