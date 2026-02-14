package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lexandro/codeindex-mcp/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FilesArgs defines the input parameters for the codeindex_files tool.
type FilesArgs struct {
	Pattern    string `json:"pattern" jsonschema:"Glob pattern to match files (e.g. **/*.ts or src/**/*.go)"`
	NameOnly   bool   `json:"nameOnly,omitempty" jsonschema:"If true return only file paths without metadata"`
	MaxResults int    `json:"maxResults,omitempty" jsonschema:"Maximum number of results to return (default 50)"`
}

// FilesHandler holds the dependencies for the files tool.
type FilesHandler struct {
	FileIndex *index.FileIndex
	Logger    *slog.Logger
}

// Handle processes a codeindex_files request.
func (h *FilesHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args FilesArgs) (*mcp.CallToolResult, any, error) {
	start := time.Now()

	if args.Pattern == "" {
		h.Logger.Warn("codeindex_files called with empty pattern")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Error: pattern parameter is required"}},
			IsError: true,
		}, nil, nil
	}

	results, err := h.FileIndex.SearchByGlob(args.Pattern, args.MaxResults)
	if err != nil {
		h.Logger.Error("codeindex_files failed", "pattern", args.Pattern, "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Search error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	elapsed := time.Since(start)
	h.Logger.Info("codeindex_files",
		"pattern", args.Pattern,
		"results", len(results),
		"elapsed", elapsed,
	)

	output := FormatFileResults(results, args.NameOnly)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil, nil
}
