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
	got := FormatSearchResults(nil, 0, false, "content")
	if got != "No matches found." {
		t.Errorf("expected 'No matches found.', got '%s'", got)
	}
}

func Test_FormatSearchResults_WithMatches(t *testing.T) {
	results := []index.ContentSearchResult{
		{
			RelativePath: "main.go",
			MatchCount:   1,
			Hunks: []index.Hunk{
				{
					StartLine: 4,
					Lines:     []string{"func main() {", `fmt.Println("hello")`, "}"},
					IsMatch:   []bool{false, true, false},
				},
			},
		},
	}

	got := FormatSearchResults(results, 1, false, "content")

	if !strings.Contains(got, "1 matches in 1 files") {
		t.Errorf("expected header with match/file counts, got:\n%s", got)
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected file path, got:\n%s", got)
	}
	if !strings.Contains(got, `5: fmt.Println("hello")`) {
		t.Errorf("expected matching line with line number, got:\n%s", got)
	}
	if !strings.Contains(got, "4- func main() {") {
		t.Errorf("expected numbered context line before, got:\n%s", got)
	}
	if !strings.Contains(got, "6- }") {
		t.Errorf("expected numbered context line after, got:\n%s", got)
	}
}

func Test_FormatSearchResults_FilesMode(t *testing.T) {
	results := []index.ContentSearchResult{
		{RelativePath: "a.go", MatchCount: 3},
		{RelativePath: "b.go", MatchCount: 1},
	}

	got := FormatSearchResults(results, 4, false, "files")

	if got != "a.go\nb.go\n" {
		t.Errorf("files mode should list paths only, got:\n%s", got)
	}
}

func Test_FormatSearchResults_CountMode(t *testing.T) {
	results := []index.ContentSearchResult{
		{RelativePath: "a.go", MatchCount: 3},
		{RelativePath: "b.go", MatchCount: 1},
	}

	got := FormatSearchResults(results, 4, false, "count")

	if got != "a.go: 3\nb.go: 1\n" {
		t.Errorf("count mode should list paths with counts, got:\n%s", got)
	}
}

func Test_FormatSearchResults_TruncationNotices(t *testing.T) {
	results := []index.ContentSearchResult{
		{
			RelativePath: "a.go",
			MatchCount:   25,
			Hunks: []index.Hunk{
				{StartLine: 1, Lines: []string{"target"}, IsMatch: []bool{true}},
			},
		},
	}

	got := FormatSearchResults(results, 25, true, "content")

	if !strings.Contains(got, "+24 more matches") {
		t.Errorf("expected per-file truncation notice, got:\n%s", got)
	}
	if !strings.Contains(got, "file limit reached") {
		t.Errorf("expected file limit notice, got:\n%s", got)
	}
}

func Test_FormatSearchResults_HunkSeparator(t *testing.T) {
	results := []index.ContentSearchResult{
		{
			RelativePath: "a.go",
			MatchCount:   2,
			Hunks: []index.Hunk{
				{StartLine: 1, Lines: []string{"first"}, IsMatch: []bool{true}},
				{StartLine: 50, Lines: []string{"second"}, IsMatch: []bool{true}},
			},
		},
	}

	got := FormatSearchResults(results, 2, false, "content")

	if !strings.Contains(got, "  --\n") {
		t.Errorf("expected hunk separator between distant hunks, got:\n%s", got)
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

func Test_FormatFileContent_NoOffsetNoLimit(t *testing.T) {
	content := "line one\nline two\nline three"
	got := FormatFileContent(content, 0, 0)

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

func Test_FormatFileContent_WithOffset(t *testing.T) {
	content := "line one\nline two\nline three\nline four\nline five"
	got := FormatFileContent(content, 3, 0)

	if strings.Contains(got, "1: ") || strings.Contains(got, "2: ") {
		t.Errorf("expected offset to skip first two lines, got:\n%s", got)
	}
	if !strings.Contains(got, "3: line three") {
		t.Errorf("expected line 3 with actual file line number, got:\n%s", got)
	}
	if !strings.Contains(got, "4: line four") {
		t.Errorf("expected line 4, got:\n%s", got)
	}
	if !strings.Contains(got, "5: line five") {
		t.Errorf("expected line 5, got:\n%s", got)
	}
}

func Test_FormatFileContent_WithLimit(t *testing.T) {
	content := "line one\nline two\nline three\nline four\nline five"
	got := FormatFileContent(content, 0, 2)

	if !strings.Contains(got, "1: line one") {
		t.Errorf("expected line 1, got:\n%s", got)
	}
	if !strings.Contains(got, "2: line two") {
		t.Errorf("expected line 2, got:\n%s", got)
	}
	if strings.Contains(got, "line three") {
		t.Errorf("expected limit to stop after 2 lines, got:\n%s", got)
	}
}

func Test_FormatFileContent_WithOffsetAndLimit(t *testing.T) {
	content := "a\nb\nc\nd\ne\nf\ng"
	got := FormatFileContent(content, 3, 2)

	if strings.Contains(got, "1: ") || strings.Contains(got, "2: ") {
		t.Errorf("expected offset to skip first two lines, got:\n%s", got)
	}
	if !strings.Contains(got, "3: c") {
		t.Errorf("expected line 3: c, got:\n%s", got)
	}
	if !strings.Contains(got, "4: d") {
		t.Errorf("expected line 4: d, got:\n%s", got)
	}
	if strings.Contains(got, "5: ") {
		t.Errorf("expected limit to stop after 2 lines, got:\n%s", got)
	}
}

func Test_FormatFileContent_OffsetBeyondEOF(t *testing.T) {
	content := "line one\nline two"
	got := FormatFileContent(content, 100, 0)

	if !strings.Contains(got, "Offset exceeds file length") {
		t.Errorf("expected error message for offset beyond EOF, got:\n%s", got)
	}
}
