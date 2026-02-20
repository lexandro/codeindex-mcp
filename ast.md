# AI Agent Prompt: AST Semantic Indexing Module for codeindex-mcp

> This prompt is self-contained. Read it completely before writing any code. All architectural decisions are pre-made — implement exactly as specified.

---

## What You Are Building

Add an optional `ast/` package to the existing `codeindex-mcp` Go MCP server. The module parses source files using `go/parser` (Go) and Tree-sitter (TypeScript, Python, JavaScript), extracts code symbols into a `SymbolTable`, and registers 5 new MCP tools with the `codeindex_ast_` prefix.

**Why this beats full-text search for AI agents:**
- `codeindex_ast_search_symbols`: finds WHERE a symbol is DEFINED — not where it appears in comments, strings, or variable names. Zero noise.
- `codeindex_ast_file_symbols`: gives a file's complete structure (what's defined) without loading full content — critical for large files.
- `codeindex_ast_get_imports`: reveals a file's dependency graph without reading it.
- Definition vs. usage distinction: reduces token consumption, increases signal quality.

The module is disabled by default (`--ast` flag required). When disabled, zero overhead: no tools registered, no parsing, no memory used.

---

## Codebase Context — Read Before Touching Any File

### main.go (lines 41–191)
All configuration is via CLI flags — `flag.StringVar`, `flag.BoolVar`, etc. **There is no config file.** Add 3 new flags here.

Current flags: `--root`, `--exclude` (repeatable), `--force-include` (repeatable), `--max-file-size`, `--max-results`, `--log-level`, `--log-file`, `--log-enabled`, `--sync-interval`.

At line 130: `performIndexing(rootDir, fileIndex, contentIndex, ignoreMatcher, logger)`
At line 144: `go handleWatcherEvents(fileWatcher, rootDir, fileIndex, contentIndex, ignoreMatcher, logger)`
At line 184: `server.Setup(searchHandler, filesHandler, statusHandler, reindexHandler, readHandler)`

### indexing.go
- `performIndexing()` (line 21): 8-worker pool, calls `indexSingleFile()` per file.
- `indexSingleFile()` (line 92): reads file bytes into `content []byte`, detects language, adds to `fileIndex` and `contentIndex`. The `content` variable is already in scope — pass it to AST module without re-reading.
- `handleWatcherEvents()` (line 151): calls `indexSingleFile()` on write/create; calls `fileIndex.RemoveFile()` + `contentIndex.RemoveFile()` on remove/rename.

**AST integration hooks**: call `astModule.OnFileChanged(relativePath, content)` inside `indexSingleFile()` after the content index update. Call `astModule.OnFileRemoved(relativePath)` in the remove/rename branch of `handleWatcherEvents()`.

### sync.go (lines 62–151)
`performSyncVerification()` also calls `indexSingleFile()` directly (line 111 and 139). Add `astModule *ast.Module` parameter here too, and pass it through.

### server/server.go (lines 8–79)
`Setup()` takes individual handler pointers and calls `mcp.AddTool()` for each. Adding 5 more parameters would make the signature unwieldy. Instead: add a single `*ast.Module` parameter. If non-nil, call `astModule.RegisterTools(mcpServer)` at the end of Setup.

### ignore/matcher.go
`ignore.Matcher` handles all file exclusion: `.gitignore`, `.claudeignore`, custom patterns (`--exclude`), file size limits. **The AST module must reuse this — do not create a parallel exclude/pattern system.**

---

## Technology Decision: go/parser + Tree-sitter Hybrid

### Go files — use `go/parser` (standard library, NO CGo)

```go
import (
    goast "go/ast"
    "go/parser"
    "go/token"
)

fset := token.NewFileSet()
f, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
```

**Rationale**: better accuracy for Go than tree-sitter (understands Go's type system, correct struct/interface/method distinction), zero CGo build complexity for the primary language of this tool.

### TypeScript, Python, JavaScript — use Tree-sitter (CGo required)

```
go get github.com/tree-sitter/go-tree-sitter@latest
go get github.com/tree-sitter/tree-sitter-typescript@latest
go get github.com/tree-sitter/tree-sitter-python@latest
go get github.com/tree-sitter/tree-sitter-javascript@latest
```

Key API:
```go
import tree_sitter "github.com/tree-sitter/go-tree-sitter"
import tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

parser := tree_sitter.NewParser()
defer parser.Close()  // REQUIRED — CGo memory, not GC-managed

err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()))

tree := parser.Parse(source, nil)  // nil = no incremental (always full parse in v1)
defer tree.Close()                 // REQUIRED

root := tree.RootNode()
// Node access:
// node.Type()                      — string, e.g. "class_declaration"
// node.ChildByFieldName("name")    — named field child
// node.Child(i), node.ChildCount() — positional children
// node.StartPoint().Row, .Column   — 0-indexed
// source[node.StartByte():node.EndByte()] — node content
```

**Incremental parsing (v1)**: NOT implemented. Always pass `nil` as the previous tree. Storing previous trees requires lifecycle management (which tree maps to which file, when to free). Out of scope for v1.

**Dart (v1)**: NOT included. `github.com/UserNobody14/tree-sitter-dart` is an unofficial grammar with no Go binding guarantee and maintenance risk. Defer to v2.

### Language Priority (v1 scope)
1. Go — `go/parser`, no CGo
2. TypeScript (`.ts`, `.tsx`) — tree-sitter
3. Python (`.py`) — tree-sitter
4. JavaScript (`.js`, `.jsx`) — tree-sitter

Java, Rust, C#, Dart, etc. are explicitly out of v1 scope.

### CGo and Cross-compilation

The current `.goreleaser.yml` has `CGO_ENABLED=0`. Adding tree-sitter requires `CGO_ENABLED=1` and a CGo-capable cross-compiler for each target.

**Required change to `.goreleaser.yml`**: enable CGo with `zig cc` as the cross-compiler.

```yaml
builds:
  - env:
      - CGO_ENABLED=1
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w
    overrides:
      - goos: linux
        goarch: amd64
        env:
          - CGO_ENABLED=1
          - CC=zig cc -target x86_64-linux-musl
          - CXX=zig c++ -target x86_64-linux-musl
      - goos: linux
        goarch: arm64
        env:
          - CGO_ENABLED=1
          - CC=zig cc -target aarch64-linux-musl
          - CXX=zig c++ -target aarch64-linux-musl
      - goos: darwin
        goarch: amd64
        env:
          - CGO_ENABLED=1
          - CC=zig cc -target x86_64-macos
          - CXX=zig c++ -target x86_64-macos
      - goos: darwin
        goarch: arm64
        env:
          - CGO_ENABLED=1
          - CC=zig cc -target aarch64-macos
          - CXX=zig c++ -target aarch64-macos
      - goos: windows
        goarch: amd64
        env:
          - CGO_ENABLED=1
          - CC=zig cc -target x86_64-windows
          - CXX=zig c++ -target x86_64-windows
```

**Required change to `.github/workflows/release.yml`**: install `zig` before running GoReleaser.

```yaml
- name: Install zig
  run: |
    wget -q https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz
    tar xf zig-linux-x86_64-0.13.0.tar.xz
    echo "$PWD/zig-linux-x86_64-0.13.0" >> $GITHUB_PATH

- uses: goreleaser/goreleaser-action@v6
  with:
    version: "~> v2"
    args: release --clean
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Binary size impact**: each tree-sitter grammar adds approximately 0.5–2 MB to the binary. Adding 3 grammars (TypeScript, Python, JavaScript) adds ~3–5 MB total. Acceptable for v1. If this becomes an issue in future, grammars can be made opt-in via Go build tags — but do NOT add build tags in v1.

---

## CLI Flags to Add in main.go

Add these 3 flags in `main.go` after line 67 (`flag.IntVar(&syncInterval, ...)`):

```go
var astEnabled bool
var astLanguages string
var astMaxFileSizeKB int

flag.BoolVar(&astEnabled, "ast", false, "Enable AST symbol indexing (requires CGo-built binary)")
flag.StringVar(&astLanguages, "ast-languages", "go,typescript,python,javascript", "Comma-separated languages for AST indexing")
flag.IntVar(&astMaxFileSizeKB, "ast-max-file-size-kb", 500, "Max file size in KB for AST parsing (default: 500)")
```

After flag parsing (after line 84 `rootDir, _ = filepath.Abs(rootDir)`), create the AST module:

```go
// Create AST module (nil if disabled)
var astModule *ast.Module
if astEnabled {
    langs := strings.Split(astLanguages, ",")
    for i, lang := range langs {
        langs[i] = strings.TrimSpace(strings.ToLower(lang))
    }
    astModule = ast.NewModule(ast.ModuleConfig{
        Languages:        langs,
        MaxFileSizeBytes: int64(astMaxFileSizeKB) * 1024,
    }, logger)
    logger.Info("AST module enabled", "languages", langs, "maxFileSizeKB", astMaxFileSizeKB)
}
```

Add import: `"github.com/lexandro/codeindex-mcp/ast"`

Update the 3 call sites:
```go
// line 130
indexedCount, totalSize := performIndexing(rootDir, fileIndex, contentIndex, ignoreMatcher, astModule, logger)

// line 144
go handleWatcherEvents(fileWatcher, rootDir, fileIndex, contentIndex, ignoreMatcher, astModule, logger)

// line 184
mcpServer := server.Setup(searchHandler, filesHandler, statusHandler, reindexHandler, readHandler, astModule)
```

---

## Directory Structure

```
ast/
├── module.go               — AstModule: NewModule, RegisterTools, OnFileChanged, OnFileRemoved
├── symbols.go              — Symbol, SymbolKind, SymbolTable, AstStats
├── extractor.go            — LanguageExtractor interface
├── extractor_go.go         — GoExtractor: go/parser-based, no CGo
├── extractor_typescript.go — TypeScriptExtractor: tree-sitter
├── extractor_python.go     — PythonExtractor: tree-sitter
├── extractor_js.go         — JavaScriptExtractor: tree-sitter
├── languages.go            — buildExtractorRegistry(): extension → extractor map
├── tools.go                — all 5 MCP tool handlers as Module methods
├── symbols_test.go
├── extractor_go_test.go
└── testdata/
    ├── sample.go
    ├── sample.ts
    └── sample.py
```

Each file target: under 300 lines (per CLAUDE.md §5).

---

## Data Structures (symbols.go)

```go
package ast

import "sync"

type SymbolKind int

const (
    SymbolClass     SymbolKind = iota
    SymbolInterface
    SymbolEnum
    SymbolFunction
    SymbolMethod
    SymbolField
    SymbolVariable
    SymbolConstant
    SymbolImport
    SymbolTypeAlias
)

// kindName returns the human-readable string for a SymbolKind.
func kindName(k SymbolKind) string {
    switch k {
    case SymbolClass:     return "class"
    case SymbolInterface: return "interface"
    case SymbolEnum:      return "enum"
    case SymbolFunction:  return "function"
    case SymbolMethod:    return "method"
    case SymbolField:     return "field"
    case SymbolVariable:  return "variable"
    case SymbolConstant:  return "constant"
    case SymbolImport:    return "import"
    case SymbolTypeAlias: return "type_alias"
    default:              return "unknown"
    }
}

type Symbol struct {
    Name       string
    Kind       SymbolKind
    File       string     // relative path, forward slashes
    Line       int        // 1-indexed start line
    EndLine    int        // 1-indexed end line
    Column     int        // 0-indexed start column
    Language   string     // "go", "typescript", "python", "javascript"
    Parent     string     // containing class/struct name (empty if top-level)
    Signature  string     // e.g. "func Foo(x int) error"
    Visibility string     // "public" or "private"
    DocComment string     // doc comment above the symbol
}

// SymbolTable is the in-memory AST index. Thread-safe.
// No allSymbols flat list — iterate byFile values when a full scan is needed.
type SymbolTable struct {
    mu      sync.RWMutex
    byName  map[string][]*Symbol     // lowercase name → symbols (same name in multiple files)
    byFile  map[string][]*Symbol     // relative path → symbols
    byKind  map[SymbolKind][]*Symbol // kind → symbols
    imports map[string][]string      // relative path → import paths
}

func NewSymbolTable() *SymbolTable {
    return &SymbolTable{
        byName:  make(map[string][]*Symbol),
        byFile:  make(map[string][]*Symbol),
        byKind:  make(map[SymbolKind][]*Symbol),
        imports: make(map[string][]string),
    }
}

// UpdateFile atomically replaces all symbols for a file.
// Acquires write lock, removes old entries, inserts new ones.
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
    t.imports[path] = fileImports
    for _, sym := range symbols {
        lowerName := strings.ToLower(sym.Name)
        t.byName[lowerName] = append(t.byName[lowerName], sym)
        t.byKind[sym.Kind] = append(t.byKind[sym.Kind], sym)
    }
}

// RemoveFile removes all symbols for a file. Acquires write lock.
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
func (t *SymbolTable) SearchByName(query string, kind *SymbolKind, language string, limit int) []*Symbol { ... }

// GetByFile returns all symbols in a file. Acquires read lock.
func (t *SymbolTable) GetByFile(path string) []*Symbol { ... }

// GetImports returns the import paths of a file. Acquires read lock.
func (t *SymbolTable) GetImports(path string) []string { ... }

// Stats returns index statistics. Acquires read lock.
func (t *SymbolTable) Stats() AstStats { ... }

// removeSymbol removes the first occurrence of sym from a slice (pointer equality).
func removeSymbol(slice []*Symbol, sym *Symbol) []*Symbol {
    for i, s := range slice {
        if s == sym {
            return append(slice[:i], slice[i+1:]...)
        }
    }
    return slice
}

type AstStats struct {
    FilesIndexed int
    TotalSymbols int
    ByLanguage   map[string]int
    ByKind       map[string]int
}
```

---

## LanguageExtractor Interface (extractor.go)

```go
package ast

// LanguageExtractor parses source files and extracts symbols.
// Two implementations use go/parser (Go, no CGo) and tree-sitter (all others).
type LanguageExtractor interface {
    Extensions() []string // file extensions this extractor handles, e.g. [".go"]
    Language() string     // canonical name: "go", "typescript", "python", "javascript"
    // ExtractSymbols parses source and returns symbols and import paths.
    ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error)
}
```

---

## Go Extractor (extractor_go.go) — CGo-Free

```go
package ast

import (
    "fmt"
    "strings"
    goast "go/ast"
    "go/parser"
    "go/token"
)

type GoExtractor struct{}

func (e *GoExtractor) Extensions() []string { return []string{".go"} }
func (e *GoExtractor) Language() string     { return "go" }

func (e *GoExtractor) ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error) {
    fset := token.NewFileSet()
    file, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
    if err != nil {
        return nil, nil, fmt.Errorf("parsing go file %s: %w", filePath, err)
    }

    var symbols []*Symbol
    var imports []string

    for _, imp := range file.Imports {
        path := strings.Trim(imp.Path.Value, `"`)
        imports = append(imports, path)
    }

    for _, decl := range file.Decls {
        switch d := decl.(type) {
        case *goast.FuncDecl:
            sym := &Symbol{
                Name:       d.Name.Name,
                Kind:       SymbolFunction,
                File:       filePath,
                Line:       fset.Position(d.Pos()).Line,
                EndLine:    fset.Position(d.End()).Line,
                Column:     fset.Position(d.Pos()).Column - 1,
                Language:   "go",
                Visibility: goVisibility(d.Name.Name),
                Signature:  extractGoFuncSignature(fset, d),
                DocComment: d.Doc.Text(),
            }
            if d.Recv != nil && len(d.Recv.List) > 0 {
                sym.Kind = SymbolMethod
                sym.Parent = extractGoReceiverType(d.Recv)
            }
            symbols = append(symbols, sym)

        case *goast.GenDecl:
            for _, spec := range d.Specs {
                switch s := spec.(type) {
                case *goast.TypeSpec:
                    kind := SymbolTypeAlias
                    switch s.Type.(type) {
                    case *goast.StructType:
                        kind = SymbolClass
                    case *goast.InterfaceType:
                        kind = SymbolInterface
                    }
                    symbols = append(symbols, &Symbol{
                        Name:       s.Name.Name,
                        Kind:       kind,
                        File:       filePath,
                        Line:       fset.Position(s.Pos()).Line,
                        EndLine:    fset.Position(s.End()).Line,
                        Language:   "go",
                        Visibility: goVisibility(s.Name.Name),
                        DocComment: d.Doc.Text(),
                    })
                case *goast.ValueSpec:
                    kind := SymbolVariable
                    if d.Tok == token.CONST {
                        kind = SymbolConstant
                    }
                    for _, name := range s.Names {
                        symbols = append(symbols, &Symbol{
                            Name:       name.Name,
                            Kind:       kind,
                            File:       filePath,
                            Line:       fset.Position(name.Pos()).Line,
                            Language:   "go",
                            Visibility: goVisibility(name.Name),
                        })
                    }
                }
            }
        }
    }
    return symbols, imports, nil
}

// goVisibility returns "public" if the name starts with an uppercase letter.
func goVisibility(name string) string {
    if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
        return "public"
    }
    return "private"
}

// extractGoReceiverType returns the type name from a method receiver.
func extractGoReceiverType(recv *goast.FieldList) string {
    if len(recv.List) == 0 {
        return ""
    }
    expr := recv.List[0].Type
    // Unwrap pointer: *Foo → Foo
    if star, ok := expr.(*goast.StarExpr); ok {
        expr = star.X
    }
    if ident, ok := expr.(*goast.Ident); ok {
        return ident.Name
    }
    return ""
}

// extractGoFuncSignature builds a human-readable signature string.
func extractGoFuncSignature(fset *token.FileSet, d *goast.FuncDecl) string {
    // Simple approach: use the source position range to extract from source is not
    // available here; build from AST instead.
    // Returns: "func (r ReceiverType) FuncName(params) results"
    // For brevity, just return the function name with param count hint.
    paramCount := 0
    if d.Type.Params != nil {
        for _, field := range d.Type.Params.List {
            if len(field.Names) == 0 {
                paramCount++
            } else {
                paramCount += len(field.Names)
            }
        }
    }
    return fmt.Sprintf("func %s(...%d params)", d.Name.Name, paramCount)
}
```

---

## Tree-sitter Extractor Pattern (extractor_typescript.go)

All tree-sitter extractors follow this exact pattern. Adapt node type names per language.

```go
package ast

import (
    "fmt"
    tree_sitter "github.com/tree-sitter/go-tree-sitter"
    tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type TypeScriptExtractor struct{}

func (e *TypeScriptExtractor) Extensions() []string { return []string{".ts", ".tsx"} }
func (e *TypeScriptExtractor) Language() string     { return "typescript" }

func (e *TypeScriptExtractor) ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error) {
    languagePtr := tree_sitter_typescript.LanguageTypescript()
    parser := tree_sitter.NewParser()
    defer parser.Close()

    if err := parser.SetLanguage(tree_sitter.NewLanguage(languagePtr)); err != nil {
        return nil, nil, fmt.Errorf("setting typescript language: %w", err)
    }

    tree := parser.Parse(source, nil) // nil = no incremental (v1 always does full parse)
    defer tree.Close()

    var symbols []*Symbol
    var imports []string
    walkTypescriptNode(tree.RootNode(), source, filePath, "", &symbols, &imports)
    return symbols, imports, nil
}

func walkTypescriptNode(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
    nodeType := node.Type()

    switch nodeType {
    case "class_declaration":
        nameNode := node.ChildByFieldName("name")
        if nameNode != nil {
            name := string(source[nameNode.StartByte():nameNode.EndByte()])
            *symbols = append(*symbols, &Symbol{
                Name:     name,
                Kind:     SymbolClass,
                File:     filePath,
                Line:     int(node.StartPoint().Row) + 1,
                EndLine:  int(node.EndPoint().Row) + 1,
                Column:   int(node.StartPoint().Column),
                Language: "typescript",
                Parent:   parentName,
            })
            // Walk body with this class as parent
            for i := range int(node.ChildCount()) {
                walkTypescriptNode(node.Child(uint(i)), source, filePath, name, symbols, imports)
            }
            return
        }

    case "interface_declaration":
        nameNode := node.ChildByFieldName("name")
        if nameNode != nil {
            *symbols = append(*symbols, &Symbol{
                Name:     string(source[nameNode.StartByte():nameNode.EndByte()]),
                Kind:     SymbolInterface,
                File:     filePath,
                Line:     int(node.StartPoint().Row) + 1,
                Language: "typescript",
            })
        }

    case "enum_declaration":
        nameNode := node.ChildByFieldName("name")
        if nameNode != nil {
            *symbols = append(*symbols, &Symbol{
                Name:     string(source[nameNode.StartByte():nameNode.EndByte()]),
                Kind:     SymbolEnum,
                File:     filePath,
                Line:     int(node.StartPoint().Row) + 1,
                Language: "typescript",
            })
        }

    case "function_declaration":
        nameNode := node.ChildByFieldName("name")
        if nameNode != nil {
            *symbols = append(*symbols, &Symbol{
                Name:     string(source[nameNode.StartByte():nameNode.EndByte()]),
                Kind:     SymbolFunction,
                File:     filePath,
                Line:     int(node.StartPoint().Row) + 1,
                EndLine:  int(node.EndPoint().Row) + 1,
                Language: "typescript",
                Parent:   parentName,
            })
        }

    case "method_definition":
        nameNode := node.ChildByFieldName("name")
        if nameNode != nil {
            *symbols = append(*symbols, &Symbol{
                Name:     string(source[nameNode.StartByte():nameNode.EndByte()]),
                Kind:     SymbolMethod,
                File:     filePath,
                Line:     int(node.StartPoint().Row) + 1,
                Language: "typescript",
                Parent:   parentName,
            })
        }

    case "type_alias_declaration":
        nameNode := node.ChildByFieldName("name")
        if nameNode != nil {
            *symbols = append(*symbols, &Symbol{
                Name:     string(source[nameNode.StartByte():nameNode.EndByte()]),
                Kind:     SymbolTypeAlias,
                File:     filePath,
                Line:     int(node.StartPoint().Row) + 1,
                Language: "typescript",
            })
        }

    case "import_statement":
        // Extract the import path from the string literal inside the import
        for i := range int(node.ChildCount()) {
            child := node.Child(uint(i))
            if child.Type() == "string" {
                raw := string(source[child.StartByte():child.EndByte()])
                path := strings.Trim(raw, `"'`)
                *imports = append(*imports, path)
            }
        }
    }

    // Default: recurse into children (unless we already did a targeted walk above)
    for i := range int(node.ChildCount()) {
        walkTypescriptNode(node.Child(uint(i)), source, filePath, parentName, symbols, imports)
    }
}
```

**Verify node type names** from the grammar's `node-types.json`. The TypeScript grammar is at `https://github.com/tree-sitter/tree-sitter-typescript`. If a node type doesn't match at runtime (symbols not extracted), dump `node.Type()` in a debug log to find the correct name.

### Python Node Types (extractor_python.go)

Key types:
- `class_definition` → SymbolClass (field `name`)
- `function_definition` → SymbolFunction at top-level; SymbolMethod when `parentName != ""`
- `import_statement` / `import_from_statement` → imports list

Walk with `parentName string` to distinguish top-level functions vs methods inside a class body.

### JavaScript Node Types (extractor_js.go)

Use `tree_sitter_javascript.Language()`. Node types are identical to TypeScript (JS grammar is a subset). Reuse the same walk logic; only the language string and Language() call differ.

---

## AstModule (module.go)

```go
package ast

import (
    "log/slog"
    "path/filepath"
    "strings"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type ModuleConfig struct {
    Languages        []string // enabled language names: ["go", "typescript", ...]
    MaxFileSizeBytes int64
}

type Module struct {
    config     ModuleConfig
    table      *SymbolTable
    extractors map[string]LanguageExtractor // file extension (with dot) → extractor
    logger     *slog.Logger
}

func NewModule(config ModuleConfig, logger *slog.Logger) *Module {
    return &Module{
        config:     config,
        table:      NewSymbolTable(),
        extractors: buildExtractorRegistry(config.Languages),
        logger:     logger,
    }
}

// OnFileChanged parses the file and updates the symbol table.
// content is the already-read file bytes from indexSingleFile — no re-read.
func (m *Module) OnFileChanged(relativePath string, content []byte) {
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
func (m *Module) OnFileRemoved(relativePath string) {
    m.table.RemoveFile(relativePath)
}

// RegisterTools adds AST MCP tools to the server. Called from server.Setup.
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
        Description: `Find files that likely use a given symbol by searching for its name in imports and in the parent/signature fields of other symbols. This is TEXT-BASED name matching against the symbol table — NOT semantic reference resolution (not LSP-level). Results are approximate. For precise text occurrences, use codeindex_search instead.`,
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
```

---

## MCP Tool Handlers (tools.go)

All handlers are methods on `*Module`. Each follows the exact same pattern (per CLAUDE.md §4):
1. Parse input struct
2. Validate parameters
3. Call symbol table method
4. Format text output
5. Return MCP result

### Input Structs

```go
type searchSymbolsInput struct {
    Query    string `json:"query"`
    Kind     string `json:"kind"`
    Language string `json:"language"`
    Limit    int    `json:"limit"`
}

type fileSymbolsInput struct {
    File string `json:"file"`
}

type findUsagesInput struct {
    Symbol string `json:"symbol"`
    Kind   string `json:"kind"`
}

type getImportsInput struct {
    File string `json:"file"`
}

type statsInput struct{} // no parameters
```

### Tool Schemas

Register each tool with its input schema via `mcp.AddTool`. Use the same `mcp.Tool` struct pattern as existing tools in `server/server.go`. The schema for `codeindex_ast_search_symbols`:

```json
{
  "type": "object",
  "properties": {
    "query":    { "type": "string", "description": "Symbol name or partial name (case-insensitive substring match)" },
    "kind":     { "type": "string", "enum": ["class","interface","enum","function","method","field","variable","constant","import","type_alias"], "description": "Optional: filter by symbol kind" },
    "language": { "type": "string", "description": "Optional: filter by language (go, typescript, python, javascript)" },
    "limit":    { "type": "integer", "description": "Max results (default: 20, max: 100)" }
  },
  "required": ["query"]
}
```

### Text Output Formats

**codeindex_ast_search_symbols** — one line per symbol:
```
AuthService    class     src/auth/auth.service.ts:12   typescript  public
validateUser   method    src/auth/auth.service.ts:22   typescript  public  parent:AuthService
NewModule      function  ast/module.go:42              go          public
```

**codeindex_ast_file_symbols** — structural outline:
```
File: src/auth/auth.service.ts (3 symbols)

line 12:  class     AuthService          public
line 22:  method    validateUser         public   parent: AuthService
line 35:  method    login                public   parent: AuthService
```

**codeindex_ast_find_usages** — files referencing the symbol:
```
Symbol: AuthService (approximate text-based matches)

src/app.module.ts:3    (import)
src/auth/auth.guard.ts:8   (parent reference)
```

**codeindex_ast_get_imports** — import list:
```
File: src/auth/auth.service.ts

  @nestjs/common
  @nestjs/jwt
  ./user.entity
  ../database/database.service
```

**codeindex_ast_stats** — statistics:
```
AST Index Statistics

Files indexed: 142
Total symbols: 1847

By language:
  go:         312
  typescript: 1535

By kind:
  class:     89
  function:  643
  method:    891
  interface: 47
  ...
```

---

## Language Registry (languages.go)

```go
package ast

// buildExtractorRegistry returns the file-extension-to-extractor map
// for the configured set of enabled language names.
func buildExtractorRegistry(enabledLanguages []string) map[string]LanguageExtractor {
    available := []LanguageExtractor{
        &GoExtractor{},
        &TypeScriptExtractor{},
        &PythonExtractor{},
        &JavaScriptExtractor{},
    }

    enabled := make(map[string]bool, len(enabledLanguages))
    for _, lang := range enabledLanguages {
        enabled[lang] = true
    }

    registry := make(map[string]LanguageExtractor)
    for _, extractor := range available {
        if !enabled[extractor.Language()] {
            continue
        }
        for _, ext := range extractor.Extensions() {
            registry[ext] = extractor
        }
    }
    return registry
}
```

Extension mapping:
- `GoExtractor`: `.go`
- `TypeScriptExtractor`: `.ts`, `.tsx`
- `PythonExtractor`: `.py`
- `JavaScriptExtractor`: `.js`, `.jsx`

---

## Integration Changes to Existing Files

### indexing.go — add astModule parameter

Change `performIndexing` signature (add `astModule *ast.Module` before `logger`):
```go
func performIndexing(
    rootDir string,
    fileIndex *index.FileIndex,
    contentIndex *index.ContentIndex,
    ignoreMatcher *ignore.Matcher,
    astModule *ast.Module, // nil when --ast flag not set
    logger *slog.Logger,
) (int, int64)
```

Change `indexSingleFile` signature (add `astModule *ast.Module` at end):
```go
func indexSingleFile(
    absolutePath string,
    relativePath string,
    info os.FileInfo,
    rootDir string,
    fileIndex *index.FileIndex,
    contentIndex *index.ContentIndex,
    ignoreMatcher *ignore.Matcher,
    astModule *ast.Module, // nil when disabled
) error
```

At the end of `indexSingleFile`, after `contentIndex.IndexFile(...)`:
```go
if astModule != nil {
    astModule.OnFileChanged(relativePath, content)
}
```

In `handleWatcherEvents`, change signature (add `astModule *ast.Module`) and update both branches:
```go
// remove/rename branch:
if astModule != nil {
    astModule.OnFileRemoved(relPath)
}

// create/write branch: pass astModule to indexSingleFile
err = indexSingleFile(event.Path, relPath, info, rootDir, fileIndex, contentIndex, ignoreMatcher, astModule)
```

### sync.go — add astModule parameter

Change `performSyncVerification` signature (add `astModule *ast.Module`). Update both `indexSingleFile` call sites (lines 111 and 139) to pass `astModule`. Also update `runPeriodicSync` to accept and forward `astModule`.

### server/server.go — add astModule parameter

```go
func Setup(
    searchHandler *tools.SearchHandler,
    filesHandler *tools.FilesHandler,
    statusHandler *tools.StatusHandler,
    reindexHandler *tools.ReindexHandler,
    readHandler *tools.ReadHandler,
    astModule *ast.Module, // nil when --ast not set
) *mcp.Server {
    // ... existing tool registrations unchanged ...

    if astModule != nil {
        astModule.RegisterTools(mcpServer)
    }
    return mcpServer
}
```

Add import: `"github.com/lexandro/codeindex-mcp/ast"`

---

## Testing (per CLAUDE.md §9)

### symbols_test.go — test SymbolTable directly (no mocks)

```go
func Test_SymbolTable_UpdateAndSearch(t *testing.T) {
    table := NewSymbolTable()
    symbols := []*Symbol{
        {Name: "Foo", Kind: SymbolClass, File: "a.go", Line: 1, Language: "go"},
        {Name: "Bar", Kind: SymbolFunction, File: "a.go", Line: 10, Language: "go"},
    }
    table.UpdateFile("a.go", symbols, nil)

    results := table.SearchByName("foo", nil, "", 10)
    if len(results) != 1 || results[0].Name != "Foo" {
        t.Errorf("expected Foo, got %v", results)
    }

    table.RemoveFile("a.go")
    results = table.SearchByName("foo", nil, "", 10)
    if len(results) != 0 {
        t.Errorf("expected empty after remove, got %v", results)
    }
}
```

### extractor_go_test.go — table-driven tests

```go
func Test_GoExtractor_ExtractSymbols(t *testing.T) {
    tests := []struct {
        name     string
        source   string
        wantName string
        wantKind SymbolKind
    }{
        {
            name:     "exported function",
            source:   `package main; func Foo() {}`,
            wantName: "Foo",
            wantKind: SymbolFunction,
        },
        {
            name:     "struct becomes class",
            source:   `package main; type Bar struct{}`,
            wantName: "Bar",
            wantKind: SymbolClass,
        },
        {
            name:     "interface",
            source:   `package main; type Doer interface { Do() }`,
            wantName: "Doer",
            wantKind: SymbolInterface,
        },
        {
            name:     "method on struct",
            source:   `package main; type S struct{}; func (s S) Hello() {}`,
            wantName: "Hello",
            wantKind: SymbolMethod,
        },
    }
    ext := &GoExtractor{}
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            symbols, _, err := ext.ExtractSymbols("test.go", []byte(tc.source))
            if err != nil {
                t.Fatal(err)
            }
            found := false
            for _, sym := range symbols {
                if sym.Name == tc.wantName && sym.Kind == tc.wantKind {
                    found = true
                }
            }
            if !found {
                t.Errorf("want %s (%s), got %v", tc.wantName, kindName(tc.wantKind), symbols)
            }
        })
    }
}
```

Place larger fixture files in `ast/testdata/`: `sample.go`, `sample.ts`, `sample.py`.

---

## Step-by-Step Execution Order

Execute in this order to keep the project buildable at each step.

1. Add 3 AST flags to `main.go` (no new package yet — just unread variables, compiles fine with `_ = astEnabled`)
2. Create `ast/symbols.go` — Symbol, SymbolKind, SymbolTable, AstStats (no imports needed)
3. Create `ast/extractor.go` — LanguageExtractor interface only
4. Create `ast/extractor_go.go` — GoExtractor using `go/parser` (no CGo, compiles on any platform)
5. Create `ast/languages.go` — buildExtractorRegistry (only GoExtractor for now, stubs for others)
6. Create `ast/module.go` — Module, NewModule, OnFileChanged, OnFileRemoved (RegisterTools as empty stub)
7. Update `indexing.go` — add `astModule *ast.Module` to all three functions, call hooks
8. Update `sync.go` — add `astModule *ast.Module` parameter, pass to indexSingleFile
9. Update `server/server.go` — add `astModule *ast.Module` parameter
10. Wire in `main.go` — create astModule, pass to all call sites
11. `go build -o codeindex-mcp.exe .` must pass at this point (`CGO_ENABLED=1` required)
12. Create `ast/tools.go` — all 5 handler methods on *Module
13. Implement RegisterTools in `ast/module.go`
14. Run `go build` again, verify AST tools register when `--ast` flag is set
15. `go get` tree-sitter packages; create `ast/extractor_typescript.go`, `ast/extractor_python.go`, `ast/extractor_js.go`
16. Wire new extractors into `buildExtractorRegistry` in `languages.go`
17. Write tests: `ast/symbols_test.go`, `ast/extractor_go_test.go`
18. `go test ./...` must pass
19. Update `.goreleaser.yml` and `.github/workflows/release.yml` for CGo cross-compilation with zig

---

## Acceptance Checklist

- [ ] `--ast` flag is required to enable the module; without it, no AST tools are registered and no tree-sitter code runs
- [ ] All 5 tools use the `codeindex_ast_` prefix: `codeindex_ast_search_symbols`, `codeindex_ast_file_symbols`, `codeindex_ast_find_usages`, `codeindex_ast_get_imports`, `codeindex_ast_stats`
- [ ] `.go` files parsed via `go/parser` (no CGo for Go)
- [ ] `.ts`, `.tsx`, `.py`, `.js`, `.jsx` parsed via tree-sitter (CGo, both `defer .Close()` calls present)
- [ ] Dart NOT included in v1
- [ ] Configuration is via CLI flags only (`--ast`, `--ast-languages`, `--ast-max-file-size-kb`) — no config file
- [ ] Existing `ignore.Matcher` reused — no parallel exclude pattern system in the AST module
- [ ] `SymbolTable` has no `allSymbols` flat list; search iterates `byFile` or `byName` only
- [ ] `codeindex_ast_find_usages` description explicitly states text-based matching, not semantic resolution
- [ ] Tree-sitter parsing always passes `nil` as previous tree (no incremental in v1)
- [ ] `sync.go` updated to pass `astModule` to `indexSingleFile` (both call sites)
- [ ] `.goreleaser.yml` updated: `CGO_ENABLED=1`, zig cross-compiler overrides
- [ ] `release.yml` updated: zig installation step added before goreleaser action
- [ ] `go test ./...` passes
- [ ] `CGO_ENABLED=1 go build -o codeindex-mcp.exe .` passes
