package index

import (
	"fmt"
	"strings"
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

// firstMatchLine returns the 1-based line number of the first matching line in the result.
func firstMatchLine(t *testing.T, result ContentSearchResult) int {
	t.Helper()
	for _, hunk := range result.Hunks {
		for lineIdx, isMatch := range hunk.IsMatch {
			if isMatch {
				return hunk.StartLine + lineIdx
			}
		}
	}
	t.Fatal("no matching line found in result")
	return 0
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

	results, totalMatches, _, err := ci.Search(SearchOptions{
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

func Test_ContentIndex_SubstringInsideIdentifier(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	// "mutex" appears only inside sync.RWMutex - grep semantics must still find it
	ci.IndexFile("content.go", "package index\n\ntype ContentIndex struct {\n\tmu sync.RWMutex\n}\n", "Go")

	results, totalMatches, _, err := ci.Search(SearchOptions{Query: "Mutex"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 || totalMatches != 1 {
		t.Fatalf("expected 1 result with 1 match for substring inside identifier, got %d results / %d matches", len(results), totalMatches)
	}
	if got := firstMatchLine(t, results[0]); got != 4 {
		t.Errorf("expected match on line 4, got %d", got)
	}
}

func Test_ContentIndex_RegexQuery_FindsLines(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("indexing.go", "package main\n\nfunc performIndexing() {}\n\nfunc helperThing() {}\n", "Go")

	results, totalMatches, _, err := ci.Search(SearchOptions{Query: `/perform\w+ing/`})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 || totalMatches != 1 {
		t.Fatalf("expected 1 regex match, got %d results / %d matches", len(results), totalMatches)
	}
	if got := firstMatchLine(t, results[0]); got != 3 {
		t.Errorf("expected match on line 3, got %d", got)
	}
}

func Test_ContentIndex_RegexQuery_Invalid(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("main.go", "package main\n", "Go")

	_, _, _, err := ci.Search(SearchOptions{Query: `/[unclosed/`})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func Test_ContentIndex_MultiWordLiteral(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "// the quick brown fox\n// quick fox\n", "Go")

	results, totalMatches, _, err := ci.Search(SearchOptions{Query: "quick brown fox"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 1 || totalMatches != 1 {
		t.Fatalf("expected exactly the literal multi-word line, got %d results / %d matches", len(results), totalMatches)
	}
	if got := firstMatchLine(t, results[0]); got != 1 {
		t.Errorf("expected match on line 1, got %d", got)
	}
}

func Test_ContentIndex_PlainQuery_CaseInsensitiveByDefault(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "HELLO World\n", "Go")

	_, totalMatches, _, err := ci.Search(SearchOptions{Query: "hello"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 1 {
		t.Errorf("expected case-insensitive match, got %d matches", totalMatches)
	}
}

func Test_ContentIndex_PlainQuery_CaseSensitiveOption(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "HELLO World\nhello world\n", "Go")

	_, totalMatches, _, err := ci.Search(SearchOptions{Query: "hello", CaseSensitive: true})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 1 {
		t.Errorf("expected 1 case-sensitive match, got %d", totalMatches)
	}
}

func Test_ContentIndex_PhraseSearch(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("app.go", `package app

func handleRequest(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello world"))
}`, "Go")

	results, _, _, err := ci.Search(SearchOptions{
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

func Test_ContentIndex_PhraseSearch_CaseSensitive(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "Hello World\n", "Go")

	_, totalMatches, _, err := ci.Search(SearchOptions{Query: `"hello world"`})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 0 {
		t.Errorf("quoted query must be case-sensitive, got %d matches", totalMatches)
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

	results, _, _, err := ci.Search(SearchOptions{
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

	hunk := results[0].Hunks[0]
	if hunk.StartLine != 2 {
		t.Errorf("expected hunk to start at line 2 (1 context line before match on line 3), got %d", hunk.StartLine)
	}
	if len(hunk.Lines) != 3 {
		t.Errorf("expected 3 lines (context + match + context), got %d", len(hunk.Lines))
	}
	if !hunk.IsMatch[1] {
		t.Error("expected middle line to be the match")
	}
}

func Test_ContentIndex_HunkMerging_OverlappingContext(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	// Matches on lines 2 and 4 with 2 context lines overlap into one hunk
	ci.IndexFile("a.go", "one\ntarget two\nthree\ntarget four\nfive\nsix\n", "Go")

	results, totalMatches, _, err := ci.Search(SearchOptions{
		Query:        "target",
		ContextLines: 2,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 2 {
		t.Fatalf("expected 2 matches, got %d", totalMatches)
	}
	if len(results[0].Hunks) != 1 {
		t.Fatalf("expected overlapping contexts to merge into 1 hunk, got %d", len(results[0].Hunks))
	}
	hunk := results[0].Hunks[0]
	matchCount := 0
	for _, isMatch := range hunk.IsMatch {
		if isMatch {
			matchCount++
		}
	}
	if matchCount != 2 {
		t.Errorf("expected 2 match lines in merged hunk, got %d", matchCount)
	}
	// No line may appear twice: hunk must be contiguous from StartLine
	if hunk.StartLine != 1 || len(hunk.Lines) != 6 {
		t.Errorf("expected hunk covering lines 1-6, got start %d with %d lines", hunk.StartLine, len(hunk.Lines))
	}
}

func Test_ContentIndex_HunkSplitting_DistantMatches(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	var sb strings.Builder
	for i := 1; i <= 30; i++ {
		if i == 5 || i == 25 {
			fmt.Fprintf(&sb, "line %d target\n", i)
		} else {
			fmt.Fprintf(&sb, "line %d\n", i)
		}
	}
	ci.IndexFile("a.go", sb.String(), "Go")

	results, _, _, err := ci.Search(SearchOptions{Query: "target", ContextLines: 2})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results[0].Hunks) != 2 {
		t.Errorf("expected 2 separate hunks for distant matches, got %d", len(results[0].Hunks))
	}
}

func Test_ContentIndex_MaxMatchesPerFile(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString("target line\nfiller\nfiller\nfiller\nfiller\nfiller\nfiller\n")
	}
	ci.IndexFile("a.go", sb.String(), "Go")

	results, totalMatches, _, err := ci.Search(SearchOptions{
		Query:             "target",
		MaxMatchesPerFile: 5,
		ContextLines:      0,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 30 {
		t.Errorf("expected MatchCount to report all 30 matches, got %d", totalMatches)
	}
	if results[0].MatchCount != 30 {
		t.Errorf("expected per-file MatchCount 30, got %d", results[0].MatchCount)
	}
	displayed := 0
	for _, hunk := range results[0].Hunks {
		for _, isMatch := range hunk.IsMatch {
			if isMatch {
				displayed++
			}
		}
	}
	if displayed != 5 {
		t.Errorf("expected 5 displayed matches, got %d", displayed)
	}
}

func Test_ContentIndex_Truncation_MaxResults(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	for i := 0; i < 5; i++ {
		ci.IndexFile(fmt.Sprintf("file%d.go", i), "target\n", "Go")
	}

	results, _, truncated, err := ci.Search(SearchOptions{Query: "target", MaxResults: 3})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !truncated {
		t.Error("expected truncated=true when more files match than MaxResults")
	}

	_, _, truncated, _ = ci.Search(SearchOptions{Query: "target", MaxResults: 10})
	if truncated {
		t.Error("expected truncated=false when all matching files fit")
	}
}

func Test_ContentIndex_DeterministicOrder(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("b.go", "target\n", "Go")
	ci.IndexFile("a.go", "target\n", "Go")
	ci.IndexFile("c.go", "target\n", "Go")

	results, _, _, err := ci.Search(SearchOptions{Query: "target"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	want := []string{"a.go", "b.go", "c.go"}
	for i, result := range results {
		if result.RelativePath != want[i] {
			t.Errorf("expected sorted path order %v, got %s at index %d", want, result.RelativePath, i)
		}
	}
}

func Test_ContentIndex_SearchWithFileGlob(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("main.go", "hello from Go", "Go")
	ci.IndexFile("app.ts", "hello from TypeScript", "TypeScript")

	results, _, _, err := ci.Search(SearchOptions{
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

	results, _, _, err := ci.Search(SearchOptions{
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
	results, _, _, err := ci.Search(SearchOptions{
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

	results, totalMatches, _, err := ci.Search(SearchOptions{
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

func Test_ContentIndex_SlashOnlyQueries_AreLiteral(t *testing.T) {
	ci := newTestContentIndex(t)
	defer ci.Close()

	ci.IndexFile("a.go", "// a comment line\nplain line\npath/to/file\n", "Go")

	// "//" is too short to be regex syntax (needs /x/) - treated as literal
	_, totalMatches, _, err := ci.Search(SearchOptions{Query: "//"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 1 {
		t.Errorf("expected literal '//' to match the comment line once, got %d", totalMatches)
	}

	// A single "/" is a plain literal too
	_, totalMatches, _, err = ci.Search(SearchOptions{Query: "/"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if totalMatches != 2 {
		t.Errorf("expected literal '/' to match 2 lines, got %d", totalMatches)
	}
}
