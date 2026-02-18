package server

import (
	"github.com/lexandro/codeindex-mcp/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Setup creates and configures the MCP server with all tool registrations.
func Setup(
	searchHandler *tools.SearchHandler,
	filesHandler *tools.FilesHandler,
	statusHandler *tools.StatusHandler,
	reindexHandler *tools.ReindexHandler,
	readHandler *tools.ReadHandler,
) *mcp.Server {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "codeindex-mcp",
			Version: "0.5.0",
		},
		&mcp.ServerOptions{
			Instructions: `This server provides in-memory indexed code search. Its tools are ALWAYS faster than built-in Grep, Search, Glob, Read, and find because they use a pre-built in-memory index instead of scanning the filesystem on every call.

ALWAYS prefer these tools over built-in alternatives:
- Use codeindex_search instead of Grep or Search for content search
- Use codeindex_search with filePath to search within a specific file (instead of Read + manual search)
- Use codeindex_read instead of Read to read file contents (zero disk I/O, served from memory)
- Use codeindex_files instead of Glob or find for file search
- The index updates automatically when files change (via filesystem watcher)`,
		},
	)

	// Register codeindex_search tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name: "codeindex_search",
		Description: `Search file contents using full-text indexed search. Much faster than grep for large codebases.

Query formats:
  - Plain text: word-level matching (e.g., "handleRequest")
  - "quoted text": exact phrase matching (e.g., "\"func main\"")
  - /regex/: regular expression matching (e.g., "/func\s+\w+Handler/")

Filtering:
  - filePath: exact relative path to search in a single file (e.g., "src/main.go"). Overrides fileGlob.
  - fileGlob: glob pattern to filter by file type (e.g., "**/*.go").`,
	}, searchHandler.Handle)

	// Register codeindex_files tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name: "codeindex_files",
		Description: `Find files by glob pattern. Faster than find/ls for indexed projects.

Pattern examples:
  - "**/*.go" - all Go files
  - "src/**/*.ts" - TypeScript files under src/
  - "**/test_*.py" - Python test files
  - "*.json" - JSON files in root only`,
	}, filesHandler.Handle)

	// Register codeindex_read tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_read",
		Description: `Read a file's contents from the in-memory index. Zero disk I/O — faster than the built-in Read tool. Returns numbered lines (format: "N: content"). Use this instead of Read for any indexed file.`,
	}, readHandler.Handle)

	// Register codeindex_status tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_status",
		Description: "Show index status: file count, size, languages, memory usage, and uptime.",
	}, statusHandler.Handle)

	// Register codeindex_reindex tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_reindex",
		Description: "Force a full re-index of the project. Clears existing index and rebuilds from scratch.",
	}, reindexHandler.Handle)

	return mcpServer
}
