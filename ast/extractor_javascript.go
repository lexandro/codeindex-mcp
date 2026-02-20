//go:build ast

package ast

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

// JavaScriptExtractor extracts symbols from JavaScript/JSX source files using tree-sitter.
type JavaScriptExtractor struct{}

func (e *JavaScriptExtractor) Extensions() []string { return []string{".js", ".jsx"} }
func (e *JavaScriptExtractor) Language() string     { return "javascript" }

// ExtractSymbols parses a JavaScript source file and returns all symbols.
func (e *JavaScriptExtractor) ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error) {
	p := tree_sitter.NewParser()
	defer p.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_javascript.Language())
	if err := p.SetLanguage(lang); err != nil {
		return nil, nil, fmt.Errorf("setting javascript language: %w", err)
	}

	tree := p.Parse(source, nil)
	defer tree.Close()

	var symbols []*Symbol
	var imports []string
	walkJSNode(tree.RootNode(), source, filePath, "", &symbols, &imports)
	return symbols, imports, nil
}

// walkJSNode recursively traverses a JavaScript AST node.
// Uses the same node type names as TypeScript (JS grammar is a subset of the TS grammar structure).
func walkJSNode(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
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
				Language: "javascript",
				Parent:   parentName,
			})
			walkJSChildren(node, source, filePath, name, symbols, imports)
			return
		}

	case "function_declaration":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolFunction,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Language: "javascript",
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
				Language: "javascript",
				Parent:   parentName,
			})
		}

	case "import_statement":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(uint(i))
			if child.Kind() == "string" {
				raw := string(source[child.StartByte():child.EndByte()])
				path := strings.Trim(raw, `"'`)
				*imports = append(*imports, path)
			}
		}
	}

	walkJSChildren(node, source, filePath, parentName, symbols, imports)
}

// walkJSChildren recurses into all children of a node.
func walkJSChildren(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		walkJSNode(child, source, filePath, parentName, symbols, imports)
	}
}
