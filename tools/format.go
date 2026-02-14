package tools

import (
	"fmt"
	"strings"

	"github.com/lexandro/codeindex-mcp/index"
)

// FormatSearchResults formats content search results as human-readable text.
// Groups matches by file with line numbers and optional context.
func FormatSearchResults(results []index.ContentSearchResult, totalMatches int) string {
	if len(results) == 0 {
		return "No matches found."
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Found %d matches in %d files:\n\n", totalMatches, len(results)))

	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(fmt.Sprintf("── %s ──\n", result.RelativePath))

		for _, match := range result.Matches {
			// Context before
			for _, ctxLine := range match.ContextBefore {
				builder.WriteString(fmt.Sprintf("  %s\n", ctxLine))
			}

			// The matching line with line number
			builder.WriteString(fmt.Sprintf("  %d: %s\n", match.LineNumber, match.LineText))

			// Context after
			for _, ctxLine := range match.ContextAfter {
				builder.WriteString(fmt.Sprintf("  %s\n", ctxLine))
			}
		}
	}

	return builder.String()
}

// FormatFileResults formats file search results as human-readable text.
func FormatFileResults(results []index.FileSearchResult, nameOnly bool) string {
	if len(results) == 0 {
		return "No files matched."
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Found %d files:\n\n", len(results)))

	for _, result := range results {
		if nameOnly {
			builder.WriteString(result.File.RelativePath)
			builder.WriteString("\n")
		} else {
			builder.WriteString(fmt.Sprintf("  %s  (%s, %s, %d lines)\n",
				result.File.RelativePath,
				result.File.Language,
				formatFileSize(result.File.SizeBytes),
				result.File.LineCount,
			))
		}
	}

	return builder.String()
}

// FormatFileContent formats a file's content with line numbers, similar to the built-in Read tool.
// Output format: header line with path and line count, followed by numbered lines.
func FormatFileContent(filePath string, content string) string {
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("── %s (%d lines) ──\n", filePath, lineCount))

	// Calculate width needed for line numbers
	width := len(fmt.Sprintf("%d", lineCount))

	for i, line := range lines {
		builder.WriteString(fmt.Sprintf("%*d│ %s\n", width, i+1, line))
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
