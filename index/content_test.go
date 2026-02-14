package index

import (
	"testing"
)

func newTestContentIndex(t *testing.T) *ContentIndex {
	t.Helper()
	ci, err := NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	return ci
}

func Test_ContentIndex_IndexAndSearch(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	err := ci.IndexFile("main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello world")
}`, "Go")
	if err != nil {
		t.Fatalf("failed to index file: %v", err)
	}

	results, totalMatches, err := ci.Search(SearchOptions{
		Query:      "hello",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if totalMatches == 0 {
		t.Fatal("expected at least one match")
	}
	if results[0].RelativePath != "main.go" {
		t.Errorf("expected main.go, got %s", results[0].RelativePath)
	}
}

func Test_ContentIndex_PhraseSearch(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("app.go", `package app

func handleRequest(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello world"))
}`, "Go")

	results, _, err := ci.Search(SearchOptions{
		Query:      `"hello world"`,
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected phrase match")
	}
}

func Test_ContentIndex_SearchWithContextLines(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("example.go", `line1
line2
line3 target
line4
line5`, "Go")

	results, _, err := ci.Search(SearchOptions{
		Query:        "target",
		MaxResults:   10,
		ContextLines: 1,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}

	match := results[0].Matches[0]
	if match.LineNumber != 3 {
		t.Errorf("expected line 3, got %d", match.LineNumber)
	}
	if len(match.ContextBefore) != 1 {
		t.Errorf("expected 1 context before line, got %d", len(match.ContextBefore))
	}
	if len(match.ContextAfter) != 1 {
		t.Errorf("expected 1 context after line, got %d", len(match.ContextAfter))
	}
}

func Test_ContentIndex_SearchWithFileGlob(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("main.go", "hello from Go", "Go")
	ci.IndexFile("app.ts", "hello from TypeScript", "TypeScript")

	results, _, err := ci.Search(SearchOptions{
		Query:      "hello",
		FileGlob:   "*.go",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (Go only), got %d", len(results))
	}
	if len(results) > 0 && results[0].RelativePath != "main.go" {
		t.Errorf("expected main.go, got %s", results[0].RelativePath)
	}
}

func Test_ContentIndex_RemoveFile(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("temp.go", "temporary content", "Go")
	ci.RemoveFile("temp.go")

	if ci.DocumentCount() != 0 {
		t.Errorf("expected 0 docs after removal, got %d", ci.DocumentCount())
	}
}

func Test_ContentIndex_Clear(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "content a", "Go")
	ci.IndexFile("b.go", "content b", "Go")

	err := ci.Clear()
	if err != nil {
		t.Fatalf("clear error: %v", err)
	}

	if ci.DocumentCount() != 0 {
		t.Errorf("expected 0 docs after clear, got %d", ci.DocumentCount())
	}
}

func Test_ContentIndex_SearchWithFilePath(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("main.go", "hello from main", "Go")
	ci.IndexFile("app.go", "hello from app", "Go")
	ci.IndexFile("lib/util.go", "hello from util", "Go")

	results, _, err := ci.Search(SearchOptions{
		Query:    "hello",
		FilePath: "app.go",
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RelativePath != "app.go" {
		t.Errorf("expected app.go, got %s", results[0].RelativePath)
	}
}

func Test_ContentIndex_SearchWithFilePath_PrecedenceOverFileGlob(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("main.go", "hello from main", "Go")
	ci.IndexFile("app.ts", "hello from app", "TypeScript")

	// FilePath should override FileGlob — search app.ts even though glob says *.go
	results, _, err := ci.Search(SearchOptions{
		Query:    "hello",
		FilePath: "app.ts",
		FileGlob: "*.go",
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (FilePath overrides FileGlob), got %d", len(results))
	}
	if results[0].RelativePath != "app.ts" {
		t.Errorf("expected app.ts, got %s", results[0].RelativePath)
	}
}

func Test_ContentIndex_SearchWithFilePath_NotFound(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("main.go", "hello from main", "Go")

	results, totalMatches, err := ci.Search(SearchOptions{
		Query:    "hello",
		FilePath: "nonexistent.go",
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent file, got %d", len(results))
	}
	if totalMatches != 0 {
		t.Errorf("expected 0 matches, got %d", totalMatches)
	}
}

func Test_ContentIndex_GetFileContent(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	expectedContent := "package main\n\nfunc main() {}\n"
	ci.IndexFile("main.go", expectedContent, "Go")

	content, ok := ci.GetFileContent("main.go")
	if !ok {
		t.Fatal("expected file to be found")
	}
	if content != expectedContent {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", content, expectedContent)
	}
}

func Test_ContentIndex_GetFileContent_NotFound(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	_, ok := ci.GetFileContent("nonexistent.go")
	if ok {
		t.Error("expected file not to be found")
	}
}

func Test_ContentIndex_DocumentCount(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "aaa", "Go")
	ci.IndexFile("b.go", "bbb", "Go")

	if ci.DocumentCount() != 2 {
		t.Errorf("expected 2 documents, got %d", ci.DocumentCount())
	}
}
