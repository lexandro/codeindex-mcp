package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// GoExtractor extracts symbols from Go source files using go/parser.
// No CGo required — uses the Go standard library only.
type GoExtractor struct{}

func (e *GoExtractor) Extensions() []string { return []string{".go"} }
func (e *GoExtractor) Language() string     { return "go" }

// ExtractSymbols parses a Go source file and returns all top-level symbols.
func (e *GoExtractor) ExtractSymbols(filePath string, source []byte) ([]*Symbol, []string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
	if err != nil {
		// Partial parse errors are ok — parser returns what it can
		if file == nil {
			return nil, nil, fmt.Errorf("parsing go file %s: %w", filePath, err)
		}
	}

	var symbols []*Symbol
	var imports []string

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := &Symbol{
				Name:       d.Name.Name,
				Kind:       SymbolFunction,
				File:       filePath,
				Line:       fset.Position(d.Pos()).Line,
				EndLine:    fset.Position(d.End()).Line,
				Column:     fset.Position(d.Pos()).Column - 1,
				Language:   "go",
				Visibility: goVisibility(d.Name.Name),
				Signature:  goFuncSignature(fset, d),
			}
			if d.Doc != nil {
				sym.DocComment = strings.TrimSpace(d.Doc.Text())
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				sym.Kind = SymbolMethod
				sym.Parent = goReceiverType(d.Recv)
			}
			symbols = append(symbols, sym)

		case *ast.GenDecl:
			docText := ""
			if d.Doc != nil {
				docText = strings.TrimSpace(d.Doc.Text())
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					kind := SymbolTypeAlias
					switch s.Type.(type) {
					case *ast.StructType:
						kind = SymbolClass
					case *ast.InterfaceType:
						kind = SymbolInterface
					}
					sym := &Symbol{
						Name:       s.Name.Name,
						Kind:       kind,
						File:       filePath,
						Line:       fset.Position(s.Pos()).Line,
						EndLine:    fset.Position(s.End()).Line,
						Language:   "go",
						Visibility: goVisibility(s.Name.Name),
						DocComment: docText,
					}
					symbols = append(symbols, sym)

				case *ast.ValueSpec:
					kind := SymbolVariable
					if d.Tok == token.CONST {
						kind = SymbolConstant
					}
					for _, name := range s.Names {
						sym := &Symbol{
							Name:       name.Name,
							Kind:       kind,
							File:       filePath,
							Line:       fset.Position(name.Pos()).Line,
							Language:   "go",
							Visibility: goVisibility(name.Name),
							DocComment: docText,
						}
						symbols = append(symbols, sym)
					}
				}
			}
		}
	}

	return symbols, imports, nil
}

// goVisibility returns "public" if the name starts with uppercase, otherwise "private".
func goVisibility(name string) string {
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return "public"
	}
	return "private"
}

// goReceiverType extracts the type name from a method receiver field list.
func goReceiverType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	// Unwrap pointer receiver: *Foo → Foo
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// goFuncSignature builds a concise signature string for a function declaration.
func goFuncSignature(fset *token.FileSet, d *ast.FuncDecl) string {
	var parts []string

	if d.Recv != nil && len(d.Recv.List) > 0 {
		recvType := goReceiverType(d.Recv)
		if recvType != "" {
			parts = append(parts, fmt.Sprintf("(%s)", recvType))
		}
	}

	parts = append(parts, d.Name.Name)

	// Count parameters
	paramParts := []string{}
	if d.Type.Params != nil {
		for _, field := range d.Type.Params.List {
			typeName := goTypeExprString(field.Type)
			if len(field.Names) == 0 {
				paramParts = append(paramParts, typeName)
			} else {
				names := make([]string, len(field.Names))
				for i, n := range field.Names {
					names[i] = n.Name
				}
				paramParts = append(paramParts, strings.Join(names, ", ")+" "+typeName)
			}
		}
	}
	sig := "func " + strings.Join(parts, " ") + "(" + strings.Join(paramParts, ", ") + ")"

	// Results
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		resultParts := []string{}
		for _, field := range d.Type.Results.List {
			resultParts = append(resultParts, goTypeExprString(field.Type))
		}
		if len(resultParts) == 1 {
			sig += " " + resultParts[0]
		} else {
			sig += " (" + strings.Join(resultParts, ", ") + ")"
		}
	}

	return sig
}

// goTypeExprString returns a simple string representation of a type expression.
func goTypeExprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + goTypeExprString(t.X)
	case *ast.ArrayType:
		return "[]" + goTypeExprString(t.Elt)
	case *ast.MapType:
		return "map[" + goTypeExprString(t.Key) + "]" + goTypeExprString(t.Value)
	case *ast.SelectorExpr:
		return goTypeExprString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + goTypeExprString(t.Value)
	case *ast.FuncType:
		return "func(...)"
	default:
		return "..."
	}
}
