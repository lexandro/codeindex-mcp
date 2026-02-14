package tools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Test_ReindexHandler_Success(t *testing.T) {
	h := &ReindexHandler{
		DoReindex: func() (int, int64, string, error) {
			return 42, 1024 * 1024, "1.5s", nil
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	result, _, err := h.Handle(context.Background(), nil, ReindexArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success, got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(text, "Reindex complete") {
		t.Errorf("expected 'Reindex complete', got:\n%s", text)
	}
	if !strings.Contains(text, "42") {
		t.Errorf("expected file count '42', got:\n%s", text)
	}
	if !strings.Contains(text, "1.0 MB") {
		t.Errorf("expected formatted size '1.0 MB', got:\n%s", text)
	}
	if !strings.Contains(text, "1.5s") {
		t.Errorf("expected elapsed '1.5s', got:\n%s", text)
	}
}

func Test_ReindexHandler_Error(t *testing.T) {
	h := &ReindexHandler{
		DoReindex: func() (int, int64, string, error) {
			return 0, 0, "", fmt.Errorf("disk full")
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	result, _, err := h.Handle(context.Background(), nil, ReindexArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for failed reindex")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "disk full") {
		t.Errorf("expected error message 'disk full', got: %s", text)
	}
}
