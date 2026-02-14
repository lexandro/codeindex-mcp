package index

import (
	"testing"
	"time"
)

func newTestFile(relPath string, lang string, size int64) *IndexedFile {
	return &IndexedFile{
		Path:         "/project/" + relPath,
		RelativePath: relPath,
		Language:     lang,
		SizeBytes:    size,
		ModTime:      time.Now(),
		LineCount:    100,
	}
}

func Test_FileIndex_AddAndGetFile(t *testing.T) {
	fi := NewFileIndex()
	file := newTestFile("src/main.go", "Go", 1024)
	fi.AddFile(file)

	got := fi.GetFile("src/main.go")
	if got == nil {
		t.Fatal("expected to find file, got nil")
	}
	if got.Language != "Go" {
		t.Errorf("expected Go, got %s", got.Language)
	}
}

func Test_FileIndex_RemoveFile(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("src/main.go", "Go", 1024))
	fi.RemoveFile("src/main.go")

	if fi.FileCount() != 0 {
		t.Errorf("expected 0 files, got %d", fi.FileCount())
	}
	if fi.GetFile("src/main.go") != nil {
		t.Error("expected nil after removal")
	}
}

func Test_FileIndex_SearchByGlob_DoubleStarExtension(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("src/main.go", "Go", 1024))
	fi.AddFile(newTestFile("src/utils/helper.go", "Go", 512))
	fi.AddFile(newTestFile("src/app.ts", "TypeScript", 2048))
	fi.AddFile(newTestFile("README.md", "Markdown", 256))

	results, err := fi.SearchByGlob("**/*.go", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 Go files, got %d", len(results))
	}
}

func Test_FileIndex_SearchByGlob_SpecificDirectory(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("src/main.go", "Go", 1024))
	fi.AddFile(newTestFile("test/main_test.go", "Go", 512))

	results, err := fi.SearchByGlob("src/**/*.go", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 file in src/, got %d", len(results))
	}
}

func Test_FileIndex_SearchByGlob_InvalidPattern(t *testing.T) {
	fi := NewFileIndex()
	_, err := fi.SearchByGlob("[invalid", 50)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func Test_FileIndex_FileCount(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("a.go", "Go", 100))
	fi.AddFile(newTestFile("b.go", "Go", 200))
	fi.AddFile(newTestFile("c.ts", "TypeScript", 300))

	if fi.FileCount() != 3 {
		t.Errorf("expected 3 files, got %d", fi.FileCount())
	}
}

func Test_FileIndex_TotalSizeBytes(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("a.go", "Go", 100))
	fi.AddFile(newTestFile("b.go", "Go", 200))

	if fi.TotalSizeBytes() != 300 {
		t.Errorf("expected 300 bytes, got %d", fi.TotalSizeBytes())
	}
}

func Test_FileIndex_LanguageCounts(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("a.go", "Go", 100))
	fi.AddFile(newTestFile("b.go", "Go", 200))
	fi.AddFile(newTestFile("c.ts", "TypeScript", 300))

	counts := fi.LanguageCounts()
	if counts["Go"] != 2 {
		t.Errorf("expected 2 Go files, got %d", counts["Go"])
	}
	if counts["TypeScript"] != 1 {
		t.Errorf("expected 1 TypeScript file, got %d", counts["TypeScript"])
	}
}

func Test_FileIndex_Clear(t *testing.T) {
	fi := NewFileIndex()
	fi.AddFile(newTestFile("a.go", "Go", 100))
	fi.Clear()

	if fi.FileCount() != 0 {
		t.Errorf("expected 0 after clear, got %d", fi.FileCount())
	}
}

func Test_FileIndex_MaxResults(t *testing.T) {
	fi := NewFileIndex()
	for i := 0; i < 100; i++ {
		fi.AddFile(newTestFile("file"+string(rune('a'+i%26))+".go", "Go", 100))
	}

	results, err := fi.SearchByGlob("**/*.go", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}
