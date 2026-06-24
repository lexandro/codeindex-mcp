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

func Test_ReadHandler_DiskFallbackRecovered(t *testing.T) {
	h := newTestReadHandler(t)

	syncTriggered := false
	h.TriggerSync = func() { syncTriggered = true }
	h.DiskFallback = func(relativePath string) (string, DiskFallbackStatus) {
		if relativePath != "stray.go" {
			t.Errorf("unexpected path passed to fallback: %s", relativePath)
		}
		return "package stray\n", DiskFallbackIndexed
	}

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "stray.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(*mcp.TextContent).Text)
	}
	if !syncTriggered {
		t.Error("expected a background sync to be triggered after recovering a missing file")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "recovered from disk") {
		t.Errorf("expected a recovery note, got:\n%s", text)
	}
	if !strings.Contains(text, "1: package stray") {
		t.Errorf("expected line-numbered disk content, got:\n%s", text)
	}
}

func Test_ReadHandler_DiskFallbackServedRaw(t *testing.T) {
	h := newTestReadHandler(t)

	syncTriggered := false
	h.TriggerSync = func() { syncTriggered = true }
	h.DiskFallback = func(string) (string, DiskFallbackStatus) {
		return "secret=1\n", DiskFallbackServedRaw
	}

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: ".env"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(*mcp.TextContent).Text)
	}
	if syncTriggered {
		t.Error("excluded files must not trigger a background sync")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "excluded from the index") {
		t.Errorf("expected an exclusion note, got:\n%s", text)
	}
}

func Test_ReadHandler_DiskFallbackBinary(t *testing.T) {
	h := newTestReadHandler(t)
	h.DiskFallback = func(string) (string, DiskFallbackStatus) {
		return "", DiskFallbackBinary
	}

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "logo.png"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for a binary file")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "binary") {
		t.Errorf("expected a binary note, got: %s", text)
	}
}

func Test_ReadHandler_DiskFallbackMissing(t *testing.T) {
	h := newTestReadHandler(t)
	h.DiskFallback = func(string) (string, DiskFallbackStatus) {
		return "", DiskFallbackMissing
	}

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "ghost.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true when the file is nowhere")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "not on disk") {
		t.Errorf("expected a 'not on disk' note, got: %s", text)
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

	if !strings.Contains(text, "1: package main") {
		t.Errorf("expected line-numbered content, got:\n%s", text)
	}
	if !strings.Contains(text, "hello") {
		t.Errorf("expected content with 'hello', got:\n%s", text)
	}
}

func Test_ReadHandler_WithOffset(t *testing.T) {
	h := newTestReadHandler(t)

	fileContent := "line1\nline2\nline3\nline4\nline5"
	h.ContentIndex.IndexFile("test.go", fileContent, "Go")

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "test.go", Offset: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	if strings.Contains(text, "1: line1") || strings.Contains(text, "2: line2") {
		t.Errorf("expected offset to skip first two lines, got:\n%s", text)
	}
	if !strings.Contains(text, "3: line3") {
		t.Errorf("expected line 3 with actual file number, got:\n%s", text)
	}
}

func Test_ReadHandler_WithLimit(t *testing.T) {
	h := newTestReadHandler(t)

	fileContent := "line1\nline2\nline3\nline4\nline5"
	h.ContentIndex.IndexFile("test.go", fileContent, "Go")

	result, _, err := h.Handle(context.Background(), nil, ReadArgs{FilePath: "test.go", Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(text, "1: line1") {
		t.Errorf("expected line 1, got:\n%s", text)
	}
	if !strings.Contains(text, "2: line2") {
		t.Errorf("expected line 2, got:\n%s", text)
	}
	if strings.Contains(text, "line3") {
		t.Errorf("expected limit to stop after 2 lines, got:\n%s", text)
	}
}
