package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lexandro/codeindex-mcp/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchArgs defines the input parameters for the codeindex_search tool.
type SearchArgs struct {
	Query        string `json:"query" jsonschema:"Search query. Plain text for word match, quoted for exact phrase, /regex/ for regular expression"`
	FilePath     string `json:"filePath,omitempty" jsonschema:"Exact relative file path to search in (overrides fileGlob). Use this to search within a single specific file"`
	FileGlob     string `json:"fileGlob,omitempty" jsonschema:"Optional glob pattern to filter files (e.g. **/*.go)"`
	MaxResults   int    `json:"maxResults,omitempty" jsonschema:"Maximum number of file results to return (default 50)"`
	ContextLines int    `json:"contextLines,omitempty" jsonschema:"Number of context lines before and after each match (default 2)"`
}

// SearchHandler holds the dependencies for the search tool.
type SearchHandler struct {
	ContentIndex *index.ContentIndex
	Logger       *slog.Logger
}

// Handle processes a codeindex_search request.
func (h *SearchHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, any, error) {
	start := time.Now()

	if args.Query == "" {
		h.Logger.Warn("codeindex_search called with empty query")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Error: query parameter is required"}},
			IsError: true,
		}, nil, nil
	}

	contextLines := args.ContextLines
	if contextLines == 0 {
		contextLines = 2
	}

	results, totalMatches, err := h.ContentIndex.Search(index.SearchOptions{
		Query:        args.Query,
		FilePath:     args.FilePath,
		FileGlob:     args.FileGlob,
		MaxResults:   args.MaxResults,
		ContextLines: contextLines,
	})
	if err != nil {
		h.Logger.Error("codeindex_search failed", "query", args.Query, "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Search error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	elapsed := time.Since(start)
	h.Logger.Info("codeindex_search",
		"query", args.Query,
		"filePath", args.FilePath,
		"fileGlob", args.FileGlob,
		"files", len(results),
		"matches", totalMatches,
		"elapsed", elapsed,
	)

	output := FormatSearchResults(results, totalMatches)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil, nil
}
