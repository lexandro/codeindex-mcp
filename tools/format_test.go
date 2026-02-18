package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/lexandro/codeindex-mcp/index"
)

// --- formatFileSize ---

func Test_FormatFileSize_Bytes(t *testing.T) {
	got := formatFileSize(500)
	if got != "500 B" {
		t.Errorf("expected '500 B', got '%s'", got)
	}
}

func Test_FormatFileSize_Kilobytes(t *testing.T) {
	got := formatFileSize(2048)
	if got != "2.0 KB" {
		t.Errorf("expected '2.0 KB', got '%s'", got)
	}
}

func Test_FormatFileSize_Megabytes(t *testing.T) {
	got := formatFileSize(3 * 1024 * 1024)
	if got != "3.0 MB" {
		t.Errorf("expected '3.0 MB', got '%s'", got)
	}
}

// --- FormatSearchResults ---

func Test_FormatSearchResults_NoMatches(t *testing.T) {
	got := FormatSearchResults(nil, 0)
	if got != "No matches found." {
		t.Errorf("expected 'No matches found.', got '%s'", got)
	}
}

func Test_FormatSearchResults_WithMatches(t *testing.T) {
	results := []index.ContentSearchResult{
		{
			RelativePath: "main.go",
			Matches: []index.LineMatch{
				{
					LineNumber:    5,
					LineText:      `fmt.Println("hello")`,
					ContextBefore: []string{"func main() {"},
					ContextAfter:  []string{"}"},
				},
			},
		},
	}

	got := FormatSearchResults(results, 1)

	if !strings.Contains(got, "1 matches in 1 files") {
		t.Errorf("expected header with match/file counts, got:\n%s", got)
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected file path, got:\n%s", got)
	}
	if !strings.Contains(got, `5: fmt.Println("hello")`) {
		t.Errorf("expected matching line with line number, got:\n%s", got)
	}
	if !strings.Contains(got, "func main() {") {
		t.Errorf("expected context before, got:\n%s", got)
	}
	if !strings.Contains(got, "}") {
		t.Errorf("expected context after, got:\n%s", got)
	}
}

// --- FormatFileResults ---

func Test_FormatFileResults_Empty(t *testing.T) {
	got := FormatFileResults(nil, false)
	if got != "No files matched." {
		t.Errorf("expected 'No files matched.', got '%s'", got)
	}
}

func Test_FormatFileResults_WithMetadata(t *testing.T) {
	results := []index.FileSearchResult{
		{
			File: &index.IndexedFile{
				RelativePath: "src/app.go",
				Language:     "Go",
				SizeBytes:    2048,
				LineCount:    50,
				ModTime:      time.Now(),
			},
		},
	}

	got := FormatFileResults(results, false)

	if !strings.Contains(got, "src/app.go") {
		t.Errorf("expected file path, got:\n%s", got)
	}
	if !strings.Contains(got, "Go") {
		t.Errorf("expected language, got:\n%s", got)
	}
	if !strings.Contains(got, "2.0 KB") {
		t.Errorf("expected formatted size, got:\n%s", got)
	}
	if !strings.Contains(got, "50L") {
		t.Errorf("expected line count, got:\n%s", got)
	}
}

func Test_FormatFileResults_NameOnly(t *testing.T) {
	results := []index.FileSearchResult{
		{
			File: &index.IndexedFile{
				RelativePath: "src/app.go",
				Language:     "Go",
				SizeBytes:    2048,
				LineCount:    50,
			},
		},
	}

	got := FormatFileResults(results, true)

	if !strings.Contains(got, "src/app.go") {
		t.Errorf("expected file path, got:\n%s", got)
	}
	// nameOnly should NOT include metadata
	if strings.Contains(got, "Go") && strings.Contains(got, "2.0 KB") {
		t.Errorf("nameOnly should not include metadata, got:\n%s", got)
	}
}

// --- FormatFileContent ---

func Test_FormatFileContent_LineNumbers(t *testing.T) {
	content := "line one\nline two\nline three"
	got := FormatFileContent(content)

	if !strings.Contains(got, "1: line one") {
		t.Errorf("expected line 1 with number, got:\n%s", got)
	}
	if !strings.Contains(got, "2: line two") {
		t.Errorf("expected line 2 with number, got:\n%s", got)
	}
	if !strings.Contains(got, "3: line three") {
		t.Errorf("expected line 3 with number, got:\n%s", got)
	}
}
