package ast

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchSymbolsArgs defines the input parameters for codeindex_ast_search_symbols.
type SearchSymbolsArgs struct {
	Query    string `json:"query" jsonschema:"Symbol name or partial name (case-insensitive substring match)"`
	Kind     string `json:"kind,omitempty" jsonschema:"Optional: filter by symbol kind (class, interface, enum, function, method, field, variable, constant, import, type_alias)"`
	Language string `json:"language,omitempty" jsonschema:"Optional: filter by language (go, typescript, python, javascript)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default: 20, max: 100)"`
}

// FileSymbolsArgs defines the input parameters for codeindex_ast_file_symbols.
type FileSymbolsArgs struct {
	File string `json:"file" jsonschema:"File path relative to project root (e.g. src/auth/auth.service.ts)"`
}

// FindUsagesArgs defines the input parameters for codeindex_ast_find_usages.
type FindUsagesArgs struct {
	Symbol string `json:"symbol" jsonschema:"Exact symbol name to search for"`
	Kind   string `json:"kind,omitempty" jsonschema:"Optional: kind of the symbol being searched (class, interface, enum, function, method)"`
}

// GetImportsArgs defines the input parameters for codeindex_ast_get_imports.
type GetImportsArgs struct {
	File string `json:"file" jsonschema:"File path relative to project root"`
}

// StatsArgs defines the input parameters for codeindex_ast_stats (none required).
type StatsArgs struct{}

// handleSearchSymbols handles codeindex_ast_search_symbols requests.
func (m *Module) handleSearchSymbols(ctx context.Context, req *mcp.CallToolRequest, args SearchSymbolsArgs) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return errorResult("query parameter is required"), nil, nil
	}

	var kindFilter *SymbolKind
	if args.Kind != "" {
		k, ok := KindFromString(args.Kind)
		if !ok {
			return errorResult(fmt.Sprintf("unknown kind %q: use one of class, interface, enum, function, method, field, variable, constant, import, type_alias", args.Kind)), nil, nil
		}
		kindFilter = &k
	}

	results := m.table.SearchByName(args.Query, kindFilter, args.Language, args.Limit)

	if len(results) == 0 {
		return textResult("No symbols found."), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d symbol(s) found:\n\n", len(results)))

	// Sort by file then line for stable output
	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		return results[i].Line < results[j].Line
	})

	for _, sym := range results {
		parent := ""
		if sym.Parent != "" {
			parent = fmt.Sprintf("  parent:%s", sym.Parent)
		}
		vis := ""
		if sym.Visibility != "" {
			vis = fmt.Sprintf("  %s", sym.Visibility)
		}
		sb.WriteString(fmt.Sprintf("%-30s  %-12s  %s:%d%s%s\n",
			sym.Name, KindName(sym.Kind), sym.File, sym.Line, vis, parent))
	}

	return textResult(sb.String()), nil, nil
}

// handleFileSymbols handles codeindex_ast_file_symbols requests.
func (m *Module) handleFileSymbols(ctx context.Context, req *mcp.CallToolRequest, args FileSymbolsArgs) (*mcp.CallToolResult, any, error) {
	if args.File == "" {
		return errorResult("file parameter is required"), nil, nil
	}

	// Normalize path separator
	filePath := strings.ReplaceAll(args.File, "\\", "/")

	symbols := m.table.GetByFile(filePath)
	if symbols == nil {
		return textResult(fmt.Sprintf("File not found in AST index: %s\n(Is the file type supported? Run codeindex_ast_stats to check.)", filePath)), nil, nil
	}
	if len(symbols) == 0 {
		return textResult(fmt.Sprintf("File: %s\n\n(no symbols found)", filePath)), nil, nil
	}

	// Sort by line number
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Line < symbols[j].Line
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s (%d symbols)\n\n", filePath, len(symbols)))

	for _, sym := range symbols {
		parent := ""
		if sym.Parent != "" {
			parent = fmt.Sprintf("  parent: %s", sym.Parent)
		}
		vis := ""
		if sym.Visibility != "" {
			vis = fmt.Sprintf("  %s", sym.Visibility)
		}
		sig := ""
		if sym.Signature != "" {
			sig = fmt.Sprintf("  — %s", sym.Signature)
		}
		sb.WriteString(fmt.Sprintf("line %-6d  %-12s  %-30s%s%s%s\n",
			sym.Line, KindName(sym.Kind), sym.Name, vis, parent, sig))
	}

	return textResult(sb.String()), nil, nil
}

// handleFindUsages handles codeindex_ast_find_usages requests.
func (m *Module) handleFindUsages(ctx context.Context, req *mcp.CallToolRequest, args FindUsagesArgs) (*mcp.CallToolResult, any, error) {
	if args.Symbol == "" {
		return errorResult("symbol parameter is required"), nil, nil
	}

	lowerSymbol := strings.ToLower(args.Symbol)
	matchedFiles := make(map[string][]string) // file → reasons

	m.table.mu.RLock()
	defer m.table.mu.RUnlock()

	// Search imports: files that import a path containing the symbol name
	for filePath, importPaths := range m.table.imports {
		for _, imp := range importPaths {
			if strings.Contains(strings.ToLower(imp), lowerSymbol) {
				matchedFiles[filePath] = append(matchedFiles[filePath], fmt.Sprintf("imports %q", imp))
				break
			}
		}
	}

	// Search symbols: find symbols whose Parent field matches the symbol name
	for _, symbols := range m.table.byFile {
		for _, sym := range symbols {
			if strings.ToLower(sym.Parent) == lowerSymbol {
				reason := fmt.Sprintf("line %d: %s %s has parent %s", sym.Line, KindName(sym.Kind), sym.Name, args.Symbol)
				matchedFiles[sym.File] = append(matchedFiles[sym.File], reason)
			}
		}
	}

	if len(matchedFiles) == 0 {
		return textResult(fmt.Sprintf("No usages found for %q.\n(Note: this is text-based matching, not semantic resolution.)", args.Symbol)), nil, nil
	}

	// Sort files for stable output
	sortedFiles := make([]string, 0, len(matchedFiles))
	for f := range matchedFiles {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Strings(sortedFiles)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Symbol: %q — approximate text-based matches (%d file(s))\n\n", args.Symbol, len(matchedFiles)))
	for _, f := range sortedFiles {
		sb.WriteString(fmt.Sprintf("%s\n", f))
		for _, reason := range matchedFiles[f] {
			sb.WriteString(fmt.Sprintf("  %s\n", reason))
		}
	}
	sb.WriteString("\nNote: results are text-based (symbol name matching), not semantic reference resolution.\n")

	return textResult(sb.String()), nil, nil
}

// handleGetImports handles codeindex_ast_get_imports requests.
func (m *Module) handleGetImports(ctx context.Context, req *mcp.CallToolRequest, args GetImportsArgs) (*mcp.CallToolResult, any, error) {
	if args.File == "" {
		return errorResult("file parameter is required"), nil, nil
	}

	filePath := strings.ReplaceAll(args.File, "\\", "/")

	imports := m.table.GetImports(filePath)
	if imports == nil {
		return textResult(fmt.Sprintf("File not found in AST index: %s", filePath)), nil, nil
	}
	if len(imports) == 0 {
		return textResult(fmt.Sprintf("File: %s\n\n(no imports)", filePath)), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s (%d imports)\n\n", filePath, len(imports)))
	for _, imp := range imports {
		sb.WriteString(fmt.Sprintf("  %s\n", imp))
	}

	return textResult(sb.String()), nil, nil
}

// handleStats handles codeindex_ast_stats requests.
func (m *Module) handleStats(ctx context.Context, req *mcp.CallToolRequest, args StatsArgs) (*mcp.CallToolResult, any, error) {
	stats := m.table.Stats()

	var sb strings.Builder
	sb.WriteString("AST Index Statistics\n\n")
	sb.WriteString(fmt.Sprintf("Files indexed: %d\n", stats.FilesIndexed))
	sb.WriteString(fmt.Sprintf("Total symbols: %d\n", stats.TotalSymbols))

	if len(stats.ByLanguage) > 0 {
		sb.WriteString("\nBy language:\n")
		// Sort by count desc
		type entry struct {
			lang  string
			count int
		}
		langs := make([]entry, 0, len(stats.ByLanguage))
		for k, v := range stats.ByLanguage {
			langs = append(langs, entry{k, v})
		}
		sort.Slice(langs, func(i, j int) bool {
			if langs[i].count != langs[j].count {
				return langs[i].count > langs[j].count
			}
			return langs[i].lang < langs[j].lang
		})
		for _, e := range langs {
			sb.WriteString(fmt.Sprintf("  %-15s %d\n", e.lang+":", e.count))
		}
	}

	if len(stats.ByKind) > 0 {
		sb.WriteString("\nBy kind:\n")
		type entry struct {
			kind  string
			count int
		}
		kinds := make([]entry, 0, len(stats.ByKind))
		for k, v := range stats.ByKind {
			kinds = append(kinds, entry{k, v})
		}
		sort.Slice(kinds, func(i, j int) bool {
			if kinds[i].count != kinds[j].count {
				return kinds[i].count > kinds[j].count
			}
			return kinds[i].kind < kinds[j].kind
		})
		for _, e := range kinds {
			sb.WriteString(fmt.Sprintf("  %-15s %d\n", e.kind+":", e.count))
		}
	}

	return textResult(sb.String()), nil, nil
}

// textResult wraps text in a successful MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// errorResult wraps text in an error MCP tool result.
func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + text}},
		IsError: true,
	}
}
