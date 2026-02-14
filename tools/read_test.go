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

func newTestReadHandler(t *testing.T) *ReadHandler {
	t.Helper()
	ci, err := index.NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	t.Cleanup(func() { ci.Close() })

	return &ReadHandler{
		ContentIndex: ci,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func Test_ReadHandler_EmptyFilePath(t *testing.T) {
	h := newTestReadHandler(t)

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for empty filePath")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "filePath parameter is required") {
		t.Errorf("expected error message about empty filePath, got: %s", text)
	}
}

func Test_ReadHandler_FileNotFound(t *testing.T) {
	h := newTestReadHandler(t)

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "nonexistent.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing file")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "File not found") {
		t.Errorf("expected 'File not found' message, got: %s", text)
	}
}

func Test_ReadHandler_Success(t *testing.T) {
	h := newTestReadHandler(t)

	fileContent := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	h.ContentIndex.IndexFile("main.go", fileContent, "Go")

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "main.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success, got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text

	// Should contain the header
	if !strings.Contains(text, "── main.go") {
		t.Errorf("expected file path header, got:\n%s", text)
	}
	// Should contain line-numbered content
	if !strings.Contains(text, "1│ package main") {
		t.Errorf("expected line-numbered content, got:\n%s", text)
	}
	if !strings.Contains(text, "hello") {
		t.Errorf("expected content with 'hello', got:\n%s", text)
	}
}
