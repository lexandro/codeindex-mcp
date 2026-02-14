package tools

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lexandro/codeindex-mcp/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StatusArgs defines the input parameters for the codeindex_status tool (none required).
type StatusArgs struct{}

// StatusHandler holds the dependencies for the status tool.
type StatusHandler struct {
	FileIndex    *index.FileIndex
	ContentIndex *index.ContentIndex
	StartTime    time.Time
	RootDir      string
	Logger       *slog.Logger
}

// Handle processes a codeindex_status request.
func (h *StatusHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args StatusArgs) (*mcp.CallToolResult, any, error) {
	var builder strings.Builder

	fileCount := h.FileIndex.FileCount()
	totalSize := h.FileIndex.TotalSizeBytes()
	langCounts := h.FileIndex.LanguageCounts()
	docCount := h.ContentIndex.DocumentCount()
	uptime := time.Since(h.StartTime)

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	h.Logger.Info("codeindex_status",
		"files", fileCount,
		"totalSize", totalSize,
		"memory", memStats.Alloc,
		"uptime", uptime,
	)

	builder.WriteString("=== codeindex-mcp Status ===\n\n")
	builder.WriteString(fmt.Sprintf("Root directory: %s\n", h.RootDir))
	builder.WriteString(fmt.Sprintf("Uptime: %s\n", formatDuration(uptime)))
	builder.WriteString(fmt.Sprintf("Indexed files: %d\n", fileCount))
	builder.WriteString(fmt.Sprintf("Content-indexed documents: %d\n", docCount))
	builder.WriteString(fmt.Sprintf("Total indexed size: %s\n", formatFileSize(totalSize)))
	builder.WriteString(fmt.Sprintf("Memory usage: %s (heap: %s)\n",
		formatFileSize(int64(memStats.Alloc)),
		formatFileSize(int64(memStats.HeapAlloc)),
	))

	// Language breakdown
	if len(langCounts) > 0 {
		builder.WriteString("\nLanguages:\n")

		// Sort by count descending
		type langEntry struct {
			lang  string
			count int
		}
		entries := make([]langEntry, 0, len(langCounts))
		for lang, count := range langCounts {
			entries = append(entries, langEntry{lang, count})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].count > entries[j].count
		})

		for _, entry := range entries {
			builder.WriteString(fmt.Sprintf("  %-20s %d files\n", entry.lang, entry.count))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: builder.String()}},
	}, nil, nil
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	}
	totalMinutes := totalSeconds / 60
	remainderSeconds := totalSeconds % 60
	if totalMinutes < 60 {
		return fmt.Sprintf("%dm%ds", totalMinutes, remainderSeconds)
	}
	hours := totalMinutes / 60
	remainderMinutes := totalMinutes % 60
	return fmt.Sprintf("%dh%dm", hours, remainderMinutes)
}
