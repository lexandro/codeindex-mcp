package ast

// LanguageExtractor parses source files and extracts code symbols.
//
// Two implementation families:
//   - GoExtractor: uses go/parser from the standard library (no CGo)
//   - TypeScriptExtractor, PythonExtractor, JavaScriptExtractor: use tree-sitter (CGo)
type LanguageExtractor interface {
	// Extensions returns the file extensions this extractor handles (e.g. [".go"]).
	Extensions() []string
	// Language returns the canonical language name (e.g. "go", "typescript").
	Language() string
	// ExtractSymbols parses source bytes and returns all symbols and import paths found.
	ExtractSymbols(filePath string, source []byte) (symbols []*Symbol, imports []string, err error)
}
