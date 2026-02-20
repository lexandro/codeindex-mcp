package ast

import (
	"strings"
	"sync"
)

// SymbolKind classifies a code symbol.
type SymbolKind int

const (
	SymbolClass     SymbolKind = iota
	SymbolInterface SymbolKind = iota
	SymbolEnum      SymbolKind = iota
	SymbolFunction  SymbolKind = iota
	SymbolMethod    SymbolKind = iota
	SymbolField     SymbolKind = iota
	SymbolVariable  SymbolKind = iota
	SymbolConstant  SymbolKind = iota
	SymbolImport    SymbolKind = iota
	SymbolTypeAlias SymbolKind = iota
)

// KindName returns the human-readable string for a SymbolKind.
func KindName(k SymbolKind) string {
	switch k {
	case SymbolClass:
		return "class"
	case SymbolInterface:
		return "interface"
	case SymbolEnum:
		return "enum"
	case SymbolFunction:
		return "function"
	case SymbolMethod:
		return "method"
	case SymbolField:
		return "field"
	case SymbolVariable:
		return "variable"
	case SymbolConstant:
		return "constant"
	case SymbolImport:
		return "import"
	case SymbolTypeAlias:
		return "type_alias"
	default:
		return "unknown"
	}
}

// KindFromString parses a kind name string into a SymbolKind.
// Returns (kind, true) on success, (0, false) if unknown.
func KindFromString(s string) (SymbolKind, bool) {
	switch s {
	case "class":
		return SymbolClass, true
	case "interface":
		return SymbolInterface, true
	case "enum":
		return SymbolEnum, true
	case "function":
		return SymbolFunction, true
	case "method":
		return SymbolMethod, true
	case "field":
		return SymbolField, true
	case "variable":
		return SymbolVariable, true
	case "constant":
		return SymbolConstant, true
	case "import":
		return SymbolImport, true
	case "type_alias":
		return SymbolTypeAlias, true
	default:
		return 0, false
	}
}

// Symbol represents a code symbol extracted from a source file.
type Symbol struct {
	Name       string
	Kind       SymbolKind
	File       string // relative path with forward slashes
	Line       int    // 1-indexed start line
	EndLine    int    // 1-indexed end line
	Column     int    // 0-indexed start column
	Language   string // "go", "typescript", "python", "javascript"
	Parent     string // containing class/struct name, empty if top-level
	Signature  string // e.g. "func Foo(x int) error"
	Visibility string // "public" or "private"
	DocComment string // doc comment above the symbol
}

// SymbolTable is the in-memory AST index. Thread-safe via sync.RWMutex.
// No allSymbols flat list — iterate byFile values for full scans.
type SymbolTable struct {
	mu      sync.RWMutex
	byName  map[string][]*Symbol     // lowercase name → symbols
	byFile  map[string][]*Symbol     // relative path → symbols
	byKind  map[SymbolKind][]*Symbol // kind → symbols
	imports map[string][]string      // relative path → import paths
}

// NewSymbolTable creates an empty SymbolTable.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		byName:  make(map[string][]*Symbol),
		byFile:  make(map[string][]*Symbol),
		byKind:  make(map[SymbolKind][]*Symbol),
		imports: make(map[string][]string),
	}
}

// UpdateFile atomically replaces all symbols for a file.
// Removes old entries, inserts new ones. Acquires write lock.
func (t *SymbolTable) UpdateFile(path string, symbols []*Symbol, fileImports []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Remove old symbols for this file from all indexes
	for _, old := range t.byFile[path] {
		lowerName := strings.ToLower(old.Name)
		t.byName[lowerName] = removeSymbol(t.byName[lowerName], old)
		t.byKind[old.Kind] = removeSymbol(t.byKind[old.Kind], old)
	}

	// Insert new symbols
	t.byFile[path] = symbols
	if fileImports != nil {
		t.imports[path] = fileImports
	} else {
		delete(t.imports, path)
	}
	for _, sym := range symbols {
		lowerName := strings.ToLower(sym.Name)
		t.byName[lowerName] = append(t.byName[lowerName], sym)
		t.byKind[sym.Kind] = append(t.byKind[sym.Kind], sym)
	}
}

// RemoveFile removes all symbols for a deleted or renamed file. Acquires write lock.
func (t *SymbolTable) RemoveFile(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, sym := range t.byFile[path] {
		lowerName := strings.ToLower(sym.Name)
		t.byName[lowerName] = removeSymbol(t.byName[lowerName], sym)
		t.byKind[sym.Kind] = removeSymbol(t.byKind[sym.Kind], sym)
	}
	delete(t.byFile, path)
	delete(t.imports, path)
}

// SearchByName returns symbols whose name contains query (case-insensitive substring).
// Optional kind and language filters. Acquires read lock.
func (t *SymbolTable) SearchByName(query string, kind *SymbolKind, language string, limit int) []*Symbol {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	lowerQuery := strings.ToLower(query)
	var results []*Symbol

	for name, symbols := range t.byName {
		if !strings.Contains(name, lowerQuery) {
			continue
		}
		for _, sym := range symbols {
			if kind != nil && sym.Kind != *kind {
				continue
			}
			if language != "" && sym.Language != language {
				continue
			}
			results = append(results, sym)
			if len(results) >= limit {
				return results
			}
		}
	}
	return results
}

// GetByFile returns all symbols in a file. Acquires read lock.
func (t *SymbolTable) GetByFile(path string) []*Symbol {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.byFile[path]
}

// GetImports returns the import paths of a file. Acquires read lock.
func (t *SymbolTable) GetImports(path string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.imports[path]
}

// AstStats holds index statistics.
type AstStats struct {
	FilesIndexed int
	TotalSymbols int
	ByLanguage   map[string]int
	ByKind       map[string]int
}

// Stats returns index statistics. Acquires read lock.
func (t *SymbolTable) Stats() AstStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := AstStats{
		FilesIndexed: len(t.byFile),
		ByLanguage:   make(map[string]int),
		ByKind:       make(map[string]int),
	}
	for _, symbols := range t.byFile {
		for _, sym := range symbols {
			stats.TotalSymbols++
			stats.ByLanguage[sym.Language]++
			stats.ByKind[KindName(sym.Kind)]++
		}
	}
	return stats
}

// removeSymbol removes the first occurrence of sym from a slice (pointer equality).
func removeSymbol(slice []*Symbol, sym *Symbol) []*Symbol {
	for i, s := range slice {
		if s == sym {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
