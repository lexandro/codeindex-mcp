//go:build ast

package ast

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// PythonExtractor extracts symbols from Python source files using tree-sitter.
type PythonExtractor struct{}

func (e *PythonExtractor) Extensions() []string { return []string{".py"} }
func (e *PythonExtractor) Language() string     { return "python" }

// ExtractSymbols parses a Python source file and returns all symbols.
func (e *PythonExtractor) ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error) {
	p := tree_sitter.NewParser()
	defer p.Close()

	lang := tree_sitter.NewLanguage(tree_sitter_python.Language())
	if err := p.SetLanguage(lang); err != nil {
		return nil, nil, fmt.Errorf("setting python language: %w", err)
	}

	tree := p.Parse(source, nil)
	defer tree.Close()

	var symbols []*Symbol
	var imports []string
	walkPyNode(tree.RootNode(), source, filePath, "", &symbols, &imports)
	return symbols, imports, nil
}

// walkPyNode recursively traverses a Python AST node.
// parentName tracks the enclosing class name for method detection.
func walkPyNode(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
	nodeType := node.Kind()

	switch nodeType {
	case "class_definition":
		if name, ok := nodeFieldText(node, "name", source); ok {
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     SymbolClass,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Language: "python",
			})
			// Walk body with class as parent
			walkPyChildren(node, source, filePath, name, symbols, imports)
			return
		}

	case "function_definition":
		if name, ok := nodeFieldText(node, "name", source); ok {
			kind := SymbolFunction
			if parentName != "" {
				kind = SymbolMethod
			}
			*symbols = append(*symbols, &Symbol{
				Name:     name,
				Kind:     kind,
				File:     filePath,
				Line:     int(node.StartPosition().Row) + 1,
				EndLine:  int(node.EndPosition().Row) + 1,
				Language: "python",
				Parent:   parentName,
			})
		}

	case "import_statement":
		// import foo, import foo as bar
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(uint(i))
			if child.Kind() == "dotted_name" || child.Kind() == "aliased_import" {
				// Get first named child (the module name)
				modNode := child.NamedChild(0)
				if modNode == nil {
					modNode = child
				}
				modName := string(source[modNode.StartByte():modNode.EndByte()])
				if modName != "" {
					*imports = append(*imports, modName)
				}
			}
		}

	case "import_from_statement":
		// from foo import bar → record "foo"
		moduleNode := node.ChildByFieldName("module_name")
		if moduleNode != nil {
			modName := string(source[moduleNode.StartByte():moduleNode.EndByte()])
			modName = strings.TrimLeft(modName, ".")
			if modName != "" {
				*imports = append(*imports, modName)
			}
		}
	}

	walkPyChildren(node, source, filePath, parentName, symbols, imports)
}

// walkPyChildren recurses into all children of a node.
func walkPyChildren(node *tree_sitter.Node, source []byte, filePath string, parentName string, symbols *[]*Symbol, imports *[]string) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		walkPyNode(child, source, filePath, parentName, symbols, imports)
	}
}
