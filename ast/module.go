package ast

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ModuleConfig holds configuration for the AST module.
type ModuleConfig struct {
	Languages        []string // enabled language names: ["go", "typescript", "python", "javascript"]
	MaxFileSizeBytes int64    // skip files larger than this
}

// Module is the AST indexing module. It parses source files, maintains a SymbolTable,
// and registers 5 MCP tools with the codeindex_ast_ prefix.
// Module is nil-safe — all methods on a nil *Module are no-ops.
type Module struct {
	config     ModuleConfig
	table      *SymbolTable
	extractors map[string]LanguageExtractor // file extension (with dot) → extractor
	logger     *slog.Logger
}

// NewModule creates a new AST module with the given configuration.
func NewModule(config ModuleConfig, logger *slog.Logger) *Module {
	return &Module{
		config:     config,
		table:      NewSymbolTable(),
		extractors: buildExtractorRegistry(config.Languages),
		logger:     logger,
	}
}

// OnFileChanged parses the file and updates the symbol table.
// content must be the already-read file bytes (no re-read inside this call).
// Safe to call on a nil *Module.
func (m *Module) OnFileChanged(relativePath string, content []byte) {
	if m == nil {
		return
	}
	ext := strings.ToLower(filepath.Ext(relativePath))
	extractor, ok := m.extractors[ext]
	if !ok {
		return // unsupported extension or language not enabled
	}
	if int64(len(content)) > m.config.MaxFileSizeBytes {
		m.logger.Debug("ast: skipping large file", "path", relativePath)
		return
	}
	symbols, imports, err := extractor.ExtractSymbols(relativePath, content)
	if err != nil {
		m.logger.Debug("ast: extraction failed", "path", relativePath, "error", err)
		return
	}
	m.table.UpdateFile(relativePath, symbols, imports)
}

// OnFileRemoved removes all symbols for a deleted or renamed file.
// Safe to call on a nil *Module.
func (m *Module) OnFileRemoved(relativePath string) {
	if m == nil {
		return
	}
	m.table.RemoveFile(relativePath)
}

// RegisterTools adds AST MCP tools to the server.
// Called from server/server.go:Setup when module is non-nil.
func (m *Module) RegisterTools(mcpServer *mcp.Server) {
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_ast_search_symbols",
		Description: `Search for code symbols (functions, classes, interfaces, methods, enums, variables, constants) by name across the AST-indexed codebase. Returns symbol name, kind, file path, line number, and signature. Unlike codeindex_search, this finds DEFINITIONS only — not comments, strings, or references. Use this to find where something is defined.`,
	}, m.handleSearchSymbols)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_ast_file_symbols",
		Description: `List all symbols defined in a specific file: functions, classes, methods, interfaces, enums. Gives a structural outline of the file without loading its full content. Use this before codeindex_read to understand a file's structure and decide whether to load it.`,
	}, m.handleFileSymbols)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_ast_find_usages",
		Description: `Find files that likely use a given symbol by searching for its name in imports and in the parent/signature fields of other symbols. This is TEXT-BASED name matching against the symbol table — NOT semantic reference resolution. Results are approximate. For precise text occurrences, use codeindex_search instead.`,
	}, m.handleFindUsages)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_ast_get_imports",
		Description: `Get the list of imports/dependencies for a specific file. Shows what modules, packages, or files this file depends on. Faster than reading the file.`,
	}, m.handleGetImports)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "codeindex_ast_stats",
		Description: `Show AST index statistics: files indexed, total symbol count, breakdown by language and symbol kind. Use this to verify the AST module is working.`,
	}, m.handleStats)
}
