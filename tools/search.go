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
	Query             string `json:"query" jsonschema:"Search query. Plain text = literal substring (case-insensitive); quoted = exact literal (case-sensitive); /regex/ = RE2 regular expression"`
	FilePath          string `json:"filePath,omitempty" jsonschema:"Exact relative file path to search in (overrides fileGlob). Use this to search within a single specific file"`
	FileGlob          string `json:"fileGlob,omitempty" jsonschema:"Optional glob pattern to filter files (e.g. **/*.go)"`
	MaxResults        int    `json:"maxResults,omitempty" jsonschema:"Maximum number of file results to return (default 50)"`
	ContextLines      *int   `json:"contextLines,omitempty" jsonschema:"Context lines before and after each match (default 2, 0 = matching lines only)"`
	MaxMatchesPerFile int    `json:"maxMatchesPerFile,omitempty" jsonschema:"Maximum matches shown per file (default 10)"`
	OutputMode        string `json:"outputMode,omitempty" jsonschema:"content (default) = matching lines with context; files = file paths only; count = file paths with match counts"`
	CaseSensitive     bool   `json:"caseSensitive,omitempty" jsonschema:"Case-sensitive matching for plain text and regex queries (default false)"`
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

	outputMode := args.OutputMode
	switch outputMode {
	case "":
		outputMode = "content"
	case "content", "files", "count":
	default:
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: unknown outputMode %q: use content, files, or count", args.OutputMode)}},
			IsError: true,
		}, nil, nil
	}

	contextLines := 2
	if args.ContextLines != nil && *args.ContextLines >= 0 {
		contextLines = *args.ContextLines
	}
	if outputMode != "content" {
		// files/count modes never render lines; skip context collection entirely
		contextLines = 0
	}

	results, totalMatches, truncated, err := h.ContentIndex.Search(index.SearchOptions{
		Query:             args.Query,
		FilePath:          args.FilePath,
		FileGlob:          args.FileGlob,
		MaxResults:        args.MaxResults,
		ContextLines:      contextLines,
		MaxMatchesPerFile: args.MaxMatchesPerFile,
		CaseSensitive:     args.CaseSensitive,
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
		"outputMode", outputMode,
		"files", len(results),
		"matches", totalMatches,
		"elapsed", elapsed,
	)

	output := FormatSearchResults(results, totalMatches, truncated, outputMode)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil, nil
}
