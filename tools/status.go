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
	Version      string
	// WatchedDirs reports how many directories the file watcher covers; nil when the watcher failed to start.
	WatchedDirs         func() int
	SyncIntervalSeconds int
	Logger              *slog.Logger
}

// Handle processes a codeindex_status request.
func (h *StatusHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args StatusArgs) (*mcp.CallToolResult, any, error) {
	var builder strings.Builder

	fileCount := h.FileIndex.FileCount()
	totalSize := h.FileIndex.TotalSizeBytes()
	langCounts := h.FileIndex.LanguageCounts()
	uptime := time.Since(h.StartTime)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	h.Logger.Info("codeindex_status",
		"files", fileCount,
		"totalSize", totalSize,
		"memory", memStats.Alloc,
		"uptime", uptime,
	)

	builder.WriteString(fmt.Sprintf("root: %s\n", h.RootDir))
	if h.Version != "" {
		builder.WriteString(fmt.Sprintf("version: %s\n", h.Version))
	}
	builder.WriteString(fmt.Sprintf("uptime: %s\n", formatDuration(uptime)))
	builder.WriteString(fmt.Sprintf("files: %d (%s)\n", fileCount, formatFileSize(totalSize)))
	builder.WriteString(fmt.Sprintf("memory: %s\n", formatFileSize(int64(memStats.Alloc))))
	if h.WatchedDirs != nil {
		builder.WriteString(fmt.Sprintf("watcher: %d dirs\n", h.WatchedDirs()))
	} else {
		builder.WriteString("watcher: not running\n")
	}
	if h.SyncIntervalSeconds > 0 {
		builder.WriteString(fmt.Sprintf("sync: every %ds\n", h.SyncIntervalSeconds))
	} else {
		builder.WriteString("sync: disabled\n")
	}

	if len(langCounts) > 0 {
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

		parts := make([]string, 0, len(entries))
		for _, entry := range entries {
			parts = append(parts, fmt.Sprintf("%s:%d", entry.lang, entry.count))
		}
		builder.WriteString("languages: " + strings.Join(parts, ", ") + "\n")
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
