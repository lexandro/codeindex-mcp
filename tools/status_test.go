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

// --- formatDuration ---

func Test_FormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"Seconds_zero", 0, "0s"},
		{"Seconds_30", 30 * time.Second, "30s"},
		{"Seconds_59", 59 * time.Second, "59s"},
		{"Minutes_1m0s", 60 * time.Second, "1m0s"},
		{"Minutes_5m30s", 5*time.Minute + 30*time.Second, "5m30s"},
		{"Hours_1h30m", 90 * time.Minute, "1h30m"},
		{"Hours_2h0m", 2 * time.Hour, "2h0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}

// --- StatusHandler ---

func newTestStatusHandler(t *testing.T) *StatusHandler {
	t.Helper()
	ci, err := index.NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	t.Cleanup(func() { ci.Close() })

	return &StatusHandler{
		FileIndex:    index.NewFileIndex(),
		ContentIndex: ci,
		StartTime:    time.Now(),
		RootDir:      "/test/project",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func Test_StatusHandler_Handle(t *testing.T) {
	h := newTestStatusHandler(t)

	// Add some test data
	h.FileIndex.AddFile(&index.IndexedFile{
		Path:         "/test/project/main.go",
		RelativePath: "main.go",
		Language:     "Go",
		SizeBytes:    1024,
		LineCount:    30,
	})
	h.ContentIndex.IndexFile("main.go", "package main\n\nfunc main() {}\n", "Go")

	result, _, err := h.Handle(context.Background(), nil, StatusArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success, got error result")
	}

	text := result.Content[0].(*mcp.TextContent).Text

	checks := []string{
		"codeindex-mcp Status",
		"/test/project",
		"Indexed files: 1",
		"Content-indexed documents: 1",
		"Go",
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Errorf("expected output to contain %q, got:\n%s", check, text)
		}
	}
}
