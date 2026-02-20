//go:build ast

package ast

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// TypeScriptExtractor extracts symbols from TypeScript/TSX source files using tree-sitter.
type TypeScriptExtractor struct{}

func (e *TypeScriptExtractor) Extensions() []string { return []string{".ts", ".tsx"} }
func (e *TypeScriptExtractor) Language() string     { return "typescript" }

// ExtractSymbols parses a TypeScript source file and returns all symbols.
func (e *TypeScriptExtractor) ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error) {
	p := tree_sitter.NewParser()
	defer p.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	if err := p.SetLanguage(lang); err != nil {
		return nil, nil, fmt.Errorf("setting typescript language: %w", err)
	}

	tree := p.Parse(source, nil) // nil = no incremental (v1 always full parse)
	defer tree.Close()

	var symbols []*Symbol
	var imports []string
	walkTSNode(tree.RootNode(), source, filePath, "", &symbols, &imports)
	return symbols, imports, nil
}

// walkTSNode recursively traverses an AST node and extracts symbols and imports.
func walkTSNode(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
	nodeType := node.Kind()

	switch nodeType {
	case "class_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolClass,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Column:   int(node.StartPosition().Column),
				Language: "typescript",
				Parent:   parentName,
			})
			// Walk body with class as parent context
			walkTSChildren(node, source, filePath, name, symbols, imports)
			return
		}

	case "abstract_class_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolClass,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Column:   int(node.StartPosition().Column),
				Language: "typescript",
				Parent:   parentName,
			})
			walkTSChildren(node, source, filePath, name, symbols, imports)
			return
		}

	case "interface_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolInterface,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Language: "typescript",
			})
		}

	case "enum_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolEnum,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				Language: "typescript",
			})
		}

	case "function_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolFunction,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Language: "typescript",
				Parent:   parentName,
			})
		}

	case "method_definition":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolMethod,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Language: "typescript",
				Parent:   parentName,
			})
		}

	case "type_alias_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolTypeAlias,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				Language: "typescript",
			})
		}

	case "import_statement":
		// Look for the string literal in the import source
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(uint(i))
			if child.Kind() == "string" {
				raw := string(source[child.StartByte():child.EndByte()])
				path := strings.Trim(raw, `"'`)
				*imports = append(*imports, path)
			}
		}
	}

	walkTSChildren(node, source, filePath, parentName, symbols, imports)
}

// walkTSChildren recurses into all children of a node.
func walkTSChildren(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		walkTSNode(child, source, filePath, parentName, symbols, imports)
	}
}

// nodeFieldText returns the text of a named field child, if present.
func nodeFieldText(node *tree_sitter.Node, field string, source []byte) (string, bool) {
	child := node.ChildByFieldName(field)
	if child == nil {
		return "", false
	}
	return string(source[child.StartByte():child.EndByte()]), true
}
