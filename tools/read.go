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
}

// ReadHandler holds the dependencies for the read tool.
type ReadHandler struct {
	ContentIndex *index.ContentIndex
	Logger       *slog.Logger
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
	if !ok {
		h.Logger.Info("codeindex_read file not found", "filePath", args.FilePath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("File not found in index: %s", args.FilePath)}},
			IsError: true,
		}, nil, nil
	}

	elapsed := time.Since(start)
	h.Logger.Info("codeindex_read", "filePath", args.FilePath, "elapsed", elapsed)

	output := FormatFileContent(content)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil, nil
}
