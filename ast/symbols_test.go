package ast

import (
	"testing"
)

func Test_SymbolTable_UpdateAndSearch(t *testing.T) {
	table := NewSymbolTable()

	symbols := []*Symbol{
		{Name: "Foo", Kind: SymbolClass, File: "a.go", Line: 1, Language: "go"},
		{Name: "Bar", Kind: SymbolFunction, File: "a.go", Line: 10, Language: "go"},
		{Name: "foo", Kind: SymbolVariable, File: "a.go", Line: 20, Language: "go"},
	}
	table.UpdateFile("a.go", symbols, []string{"fmt", "strings"})

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantName  string
	}{
		{name: "case insensitive match", query: "foo", wantCount: 2},
		{name: "exact match", query: "bar", wantCount: 1, wantName: "Bar"},
		{name: "no match", query: "zzz", wantCount: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := table.SearchByName(tc.query, nil, "", 10)
			if len(results) != tc.wantCount {
				t.Errorf("query %q: want %d results, got %d: %v", tc.query, tc.wantCount, len(results), results)
			}
			if tc.wantName != "" && len(results) > 0 {
				found := false
				for _, r := range results {
					if r.Name == tc.wantName {
						found = true
					}
				}
				if !found {
					t.Errorf("query %q: want symbol %q in results %v", tc.query, tc.wantName, results)
				}
			}
		})
	}
}

func Test_SymbolTable_KindFilter(t *testing.T) {
	table := NewSymbolTable()
	symbols := []*Symbol{
		{Name: "MyClass", Kind: SymbolClass, File: "f.go", Line: 1, Language: "go"},
		{Name: "myFunc", Kind: SymbolFunction, File: "f.go", Line: 5, Language: "go"},
	}
	table.UpdateFile("f.go", symbols, nil)

	classKind := SymbolClass
	results := table.SearchByName("my", &classKind, "", 10)
	if len(results) != 1 || results[0].Name != "MyClass" {
		t.Errorf("kind filter: want MyClass only, got %v", results)
	}
}

func Test_SymbolTable_RemoveFile(t *testing.T) {
	table := NewSymbolTable()
	symbols := []*Symbol{
		{Name: "Foo", Kind: SymbolFunction, File: "b.go", Line: 1, Language: "go"},
	}
	table.UpdateFile("b.go", symbols, nil)

	table.RemoveFile("b.go")

	results := table.SearchByName("foo", nil, "", 10)
	if len(results) != 0 {
		t.Errorf("after RemoveFile: expected 0 results, got %d", len(results))
	}
}

func Test_SymbolTable_GetByFile(t *testing.T) {
	table := NewSymbolTable()
	syms := []*Symbol{
		{Name: "Alpha", Kind: SymbolClass, File: "c.go", Line: 1},
		{Name: "Beta", Kind: SymbolFunction, File: "c.go", Line: 10},
	}
	table.UpdateFile("c.go", syms, nil)

	got := table.GetByFile("c.go")
	if len(got) != 2 {
		t.Errorf("GetByFile: want 2 symbols, got %d", len(got))
	}

	got2 := table.GetByFile("nonexistent.go")
	if got2 != nil {
		t.Errorf("GetByFile on missing file: want nil, got %v", got2)
	}
}

func Test_SymbolTable_GetImports(t *testing.T) {
	table := NewSymbolTable()
	table.UpdateFile("x.go", nil, []string{"fmt", "os"})

	imports := table.GetImports("x.go")
	if len(imports) != 2 {
		t.Errorf("GetImports: want 2, got %d: %v", len(imports), imports)
	}
}

func Test_SymbolTable_Stats(t *testing.T) {
	table := NewSymbolTable()
	table.UpdateFile("a.go", []*Symbol{
		{Name: "Foo", Kind: SymbolClass, Language: "go"},
		{Name: "Bar", Kind: SymbolFunction, Language: "go"},
	}, nil)
	table.UpdateFile("b.ts", []*Symbol{
		{Name: "Baz", Kind: SymbolClass, Language: "typescript"},
	}, nil)

	stats := table.Stats()
	if stats.FilesIndexed != 2 {
		t.Errorf("Stats.FilesIndexed: want 2, got %d", stats.FilesIndexed)
	}
	if stats.TotalSymbols != 3 {
		t.Errorf("Stats.TotalSymbols: want 3, got %d", stats.TotalSymbols)
	}
	if stats.ByLanguage["go"] != 2 {
		t.Errorf("Stats.ByLanguage[go]: want 2, got %d", stats.ByLanguage["go"])
	}
}

func Test_KindName(t *testing.T) {
	tests := []struct {
		kind SymbolKind
		want string
	}{
		{SymbolClass, "class"},
		{SymbolFunction, "function"},
		{SymbolMethod, "method"},
		{SymbolTypeAlias, "type_alias"},
	}
	for _, tc := range tests {
		got := KindName(tc.kind)
		if got != tc.want {
			t.Errorf("KindName(%d): want %q, got %q", tc.kind, tc.want, got)
		}
	}
}

func Test_KindFromString(t *testing.T) {
	tests := []struct {
		input   string
		wantOK  bool
		wantKnd SymbolKind
	}{
		{"class", true, SymbolClass},
		{"function", true, SymbolFunction},
		{"unknown_xyz", false, 0},
	}
	for _, tc := range tests {
		k, ok := KindFromString(tc.input)
		if ok != tc.wantOK {
			t.Errorf("KindFromString(%q): wantOK=%v, got=%v", tc.input, tc.wantOK, ok)
		}
		if ok && k != tc.wantKnd {
			t.Errorf("KindFromString(%q): want kind %d, got %d", tc.input, tc.wantKnd, k)
		}
	}
}
