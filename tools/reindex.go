package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReindexArgs defines the input parameters for the codeindex_reindex tool.
type ReindexArgs struct{}

// ReindexFunc is the function signature for the reindex operation.
// It is provided by main.go to avoid circular dependencies.
type ReindexFunc func() (indexedCount int, totalSize int64, elapsed string, err error)

// ReindexHandler holds the dependencies for the reindex tool.
type ReindexHandler struct {
	DoReindex ReindexFunc
	Logger    *slog.Logger
}

// Handle processes a codeindex_reindex request.
func (h *ReindexHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args ReindexArgs) (*mcp.CallToolResult, any, error) {
	h.Logger.Info("codeindex_reindex started")

	indexedCount, totalSize, elapsed, err := h.DoReindex()
	if err != nil {
		h.Logger.Error("codeindex_reindex failed", "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Reindex error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	h.Logger.Info("codeindex_reindex complete",
		"files", indexedCount,
		"totalSize", totalSize,
		"elapsed", elapsed,
	)

	output := fmt.Sprintf("reindexed: %d files (%s) in %s",
		indexedCount, formatFileSize(totalSize), elapsed)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil, nil
}
