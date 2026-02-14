package tools

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lexandro/codeindex-mcp/index"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestFilesHandler(t *testing.T) *FilesHandler {
	t.Helper()
	return &FilesHandler{
		FileIndex: index.NewFileIndex(),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func Test_FilesHandler_EmptyPattern(t *testing.T) {
	h := newTestFilesHandler(t)

	result, _, err := h.Handle(context.Background(), nil, FilesArgs{Pattern: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for empty pattern")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "pattern parameter is required") {
		t.Errorf("expected error message about empty pattern, got: %s", text)
	}
}

func Test_FilesHandler_GlobSearch(t *testing.T) {
	h := newTestFilesHandler(t)

	h.FileIndex.AddFile(&index.IndexedFile{
		Path:         "/project/src/main.go",
		RelativePath: "src/main.go",
		Language:     "Go",
		SizeBytes:    512,
		LineCount:    20,
		ModTime:      time.Now(),
	})
	h.FileIndex.AddFile(&index.IndexedFile{
		Path:         "/project/README.md",
		RelativePath: "README.md",
		Language:     "Markdown",
		SizeBytes:    256,
		LineCount:    10,
		ModTime:      time.Now(),
	})

	result, _, err := h.Handle(context.Background(), nil, FilesArgs{Pattern: "**/*.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success, got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "src/main.go") {
		t.Errorf("expected result to contain src/main.go, got:\n%s", text)
	}
	if strings.Contains(text, "README.md") {
		t.Errorf("expected result to NOT contain README.md, got:\n%s", text)
	}
}

func Test_FilesHandler_NoResults(t *testing.T) {
	h := newTestFilesHandler(t)

	h.FileIndex.AddFile(&index.IndexedFile{
		Path:         "/project/main.go",
		RelativePath: "main.go",
		Language:     "Go",
		SizeBytes:    512,
		LineCount:    20,
		ModTime:      time.Now(),
	})

	result, _, err := h.Handle(context.Background(), nil, FilesArgs{Pattern: "**/*.rs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success (no error), got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "No files matched") {
		t.Errorf("expected 'No files matched', got:\n%s", text)
	}
}
