package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lexandro/codeindex-mcp/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReadArgs defines the input parameters for the codeindex_read tool.
type ReadArgs struct {
	FilePath string `json:"filePath" jsonschema:"Relative file path to read from the index (e.g. src/main.go)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"Line number to start reading from (1-based). Only provide if the file is too large to read at once"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Number of lines to read. Only provide if the file is too large to read at once"`
}

// DiskFallbackStatus is the outcome of a last-resort filesystem read for a file
// that is missing from the in-memory index.
type DiskFallbackStatus int

const (
	// DiskFallbackMissing: the file is not on disk either (likely a hallucinated path).
	DiskFallbackMissing DiskFallbackStatus = iota
	// DiskFallbackIndexed: the file was read from disk and added to the index (a genuine index gap).
	DiskFallbackIndexed
	// DiskFallbackServedRaw: the file was read from disk but deliberately not indexed (ignored or too large).
	DiskFallbackServedRaw
	// DiskFallbackBinary: the file exists on disk but is binary and was not served.
	DiskFallbackBinary
)

// DiskFallbackFunc reads a file that is missing from the index directly from
// disk and, when the file is eligible, adds it to the index. It is provided by
// main.go so this package stays free of filesystem and ignore-rule logic.
type DiskFallbackFunc func(relativePath string) (content string, status DiskFallbackStatus)

// ReadHandler holds the dependencies for the read tool.
type ReadHandler struct {
	ContentIndex *index.ContentIndex
	Logger       *slog.Logger
	// DiskFallback is the last-resort filesystem read used when a file is not in
	// the index. Optional; if nil the handler returns a plain "not found" error.
	DiskFallback DiskFallbackFunc
	// TriggerSync kicks off a non-blocking background index reconciliation. It is
	// called after a genuine index gap is recovered, since sibling files may also
	// be missing. Optional.
	TriggerSync func()
}

// Handle processes a codeindex_read request.
func (h *ReadHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args ReadArgs) (*mcp.CallToolResult, any, error) {
	start := time.Now()

	if args.FilePath == "" {
		h.Logger.Warn("codeindex_read called with empty filePath")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Error: filePath parameter is required"}},
			IsError: true,
		}, nil, nil
	}

	content, ok := h.ContentIndex.GetFileContent(args.FilePath)
	if ok {
		h.Logger.Info("codeindex_read", "filePath", args.FilePath, "elapsed", time.Since(start))
		output := FormatFileContent(content, args.Offset, args.Limit)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: output}},
		}, nil, nil
	}

	// Index miss. Last resort: read straight from disk. A file may be missing
	// from the index because of a lost watcher event or an indexing race rather
	// than because it does not exist, so we try the filesystem before giving up.
	if h.DiskFallback == nil {
		h.Logger.Info("codeindex_read file not found", "filePath", args.FilePath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("File not found in index: %s", args.FilePath)}},
			IsError: true,
		}, nil, nil
	}

	diskContent, status := h.DiskFallback(args.FilePath)
	switch status {
	case DiskFallbackIndexed:
		// The file should have been indexed. Serve it now and, since siblings may
		// also have been missed, kick a background sync that does not block this read.
		h.Logger.Warn("codeindex_read recovered missing file from disk", "filePath", args.FilePath, "elapsed", time.Since(start))
		if h.TriggerSync != nil {
			h.TriggerSync()
		}
		return servedFromDisk(diskContent, args, "recovered from disk; index was missing this file"), nil, nil

	case DiskFallbackServedRaw:
		// Excluded from the index (ignored or too large) but the path was requested
		// explicitly, so serve it without polluting the index or triggering a sync.
		h.Logger.Info("codeindex_read served unindexed file from disk", "filePath", args.FilePath, "elapsed", time.Since(start))
		return servedFromDisk(diskContent, args, "served from disk; file is excluded from the index"), nil, nil

	case DiskFallbackBinary:
		h.Logger.Info("codeindex_read binary file not served", "filePath", args.FilePath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("File exists on disk but is binary; not served: %s", args.FilePath)}},
			IsError: true,
		}, nil, nil

	default: // DiskFallbackMissing
		h.Logger.Info("codeindex_read file not found", "filePath", args.FilePath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("File not found (not in index, not on disk): %s", args.FilePath)}},
			IsError: true,
		}, nil, nil
	}
}

// servedFromDisk formats disk-read content with a leading note so the caller
// knows the result bypassed the index.
func servedFromDisk(content string, args ReadArgs, note string) *mcp.CallToolResult {
	output := fmt.Sprintf("// codeindex: %s\n%s", note, FormatFileContent(content, args.Offset, args.Limit))
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}
}
