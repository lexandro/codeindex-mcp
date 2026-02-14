package tools

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/lexandro/codeindex-mcp/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestSearchHandler(t *testing.T) *SearchHandler {
	t.Helper()
	ci, err := index.NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	t.Cleanup(func() { ci.Close() })

	return &SearchHandler{
		ContentIndex: ci,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func Test_SearchHandler_EmptyQuery(t *testing.T) {
	h := newTestSearchHandler(t)

	result, _, err := h.Handle(context.Background(), nil, SearchArgs{Query: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for empty query")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "query parameter is required") {
		t.Errorf("expected error message about empty query, got: %s", text)
	}
}

func Test_SearchHandler_BasicSearch(t *testing.T) {
	h := newTestSearchHandler(t)

	h.ContentIndex.IndexFile("main.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n", "Go")
	h.ContentIndex.IndexFile("util.go", "package main\n\nfunc helper() int {\n\treturn 42\n}\n", "Go")

	result, _, err := h.Handle(context.Background(), nil, SearchArgs{Query: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success, got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected result to contain main.go, got:\n%s", text)
	}
	if !strings.Contains(text, "hello") {
		t.Errorf("expected result to contain 'hello', got:\n%s", text)
	}
}

func Test_SearchHandler_NoResults(t *testing.T) {
	h := newTestSearchHandler(t)

	h.ContentIndex.IndexFile("main.go", "package main\n\nfunc main() {}\n", "Go")

	result, _, err := h.Handle(context.Background(), nil, SearchArgs{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success (no error), got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "No matches found") {
		t.Errorf("expected 'No matches found', got:\n%s", text)
	}
}
