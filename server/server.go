package server

import (
	"github.com/lexandro/codeindex-mcp/ast"
	"github.com/lexandro/codeindex-mcp/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the codeindex-mcp release version, used in the MCP handshake and status output.
const Version = "0.6.0"

// readOnly marks a tool as non-mutating so MCP clients can skip permission prompts.
var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)}

func boolPtr(b bool) *bool { return &b }

// Setup creates and configures the MCP server with all tool registrations.
func Setup(
	searchHandler *tools.SearchHandler,
	filesHandler *tools.FilesHandler,
	statusHandler *tools.StatusHandler,
	reindexHandler *tools.ReindexHandler,
	readHandler *tools.ReadHandler,
	astModule *ast.Module,
) *mcp.Server {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "codeindex-mcp",
			Version: Version,
		},
		&mcp.ServerOptions{
			Instructions: `This server provides in-memory indexed code search with exact grep semantics. Its tools are faster than built-in Grep, Search, Glob, Read, and find because all file contents are held in memory instead of being scanned from disk on every call.

ALWAYS prefer these tools over built-in alternatives:
- Use codeindex_search instead of Grep or Search for content search (substring, exact, and regex queries)
- Use codeindex_search with filePath to search within a specific file (instead of Read + manual search)
- Use codeindex_search with outputMode "files" or "count" when you only need file locations - saves tokens
- Use codeindex_read instead of Read to read file contents (zero disk I/O, served from memory)
- Use codeindex_files instead of Glob or find for file search
- The index updates automatically when files change (via filesystem watcher)`,
		},
	)

	// Register codeindex_search tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name: "codeindex_search",
		Description: `Search file contents from the in-memory index with exact grep semantics (full substring recall, e.g. "mutex" finds sync.RWMutex).

Query formats:
  - Plain text: literal substring match, case-insensitive (e.g., "handleRequest")
  - "quoted text": exact literal match, case-sensitive (e.g., "\"func main\"")
  - /regex/: RE2 regular expression per line (e.g., "/func\s+\w+Handler/")

Filtering: filePath = exact relative path (overrides fileGlob); fileGlob = glob pattern (e.g. "**/*.go").
Output: outputMode "content" (default, hunks with line numbers), "files" (paths only), "count" (paths with match counts). Use files/count to save tokens when locations suffice.`,
		Annotations: readOnly,
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
		Annotations: readOnly,
	}, filesHandler.Handle)

	// Register codeindex_read tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_read",
		Description: `Read a file's contents from the in-memory index. Zero disk I/O — faster than the built-in Read tool. Returns numbered lines (format: "N: content"). Use this instead of Read for any indexed file. By default reads up to 2000 lines. Optionally specify a line offset and limit (especially handy for long files).`,
		Annotations: readOnly,
	}, readHandler.Handle)

	// Register codeindex_status tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_status",
		Description: "Show index status: file count, size, languages, memory usage, watcher health, and uptime.",
		Annotations: readOnly,
	}, statusHandler.Handle)

	// Register codeindex_reindex tool
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_reindex",
		Description: "Force a full re-index of the project. Clears existing index and rebuilds from scratch.",
		Annotations: &mcp.ToolAnnotations{
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
	}, reindexHandler.Handle)

	// Register AST tools if module is enabled
	if astModule != nil {
		astModule.RegisterTools(mcpServer)
	}

	return mcpServer
}
