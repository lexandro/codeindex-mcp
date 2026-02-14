package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_Matcher_DefaultPatterns_NodeModules(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	nodePath := filepath.Join(tmpDir, "node_modules", "express", "index.js")
	if !matcher.ShouldIgnore(nodePath) {
		t.Error("expected node_modules files to be ignored")
	}
}

func Test_Matcher_DefaultPatterns_GitDir(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	gitPath := filepath.Join(tmpDir, ".git", "config")
	if !matcher.ShouldIgnore(gitPath) {
		t.Error("expected .git files to be ignored")
	}
}

func Test_Matcher_DefaultPatterns_BinaryExtension(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	exePath := filepath.Join(tmpDir, "app.exe")
	if !matcher.ShouldIgnore(exePath) {
		t.Error("expected .exe files to be ignored")
	}
}

func Test_Matcher_DefaultPatterns_AllowsSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	goPath := filepath.Join(tmpDir, "main.go")
	if matcher.ShouldIgnore(goPath) {
		t.Error("expected .go files to NOT be ignored")
	}
}

func Test_Matcher_GitignoreIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .gitignore file
	gitignoreContent := "*.generated.go\nsecret/\n"
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignoreContent), 0644)

	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	generatedPath := filepath.Join(tmpDir, "models.generated.go")
	if !matcher.ShouldIgnore(generatedPath) {
		t.Error("expected .gitignore pattern to ignore *.generated.go")
	}

	normalPath := filepath.Join(tmpDir, "main.go")
	if matcher.ShouldIgnore(normalPath) {
		t.Error("expected normal .go files to NOT be ignored by .gitignore")
	}
}

func Test_Matcher_ClaudeignoreIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .claudeignore file
	claudeignoreContent := "docs/internal/\n*.draft.md\n"
	os.WriteFile(filepath.Join(tmpDir, ".claudeignore"), []byte(claudeignoreContent), 0644)

	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	draftPath := filepath.Join(tmpDir, "notes.draft.md")
	if !matcher.ShouldIgnore(draftPath) {
		t.Error("expected .claudeignore pattern to ignore *.draft.md")
	}
}

func Test_Matcher_CustomPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:        tmpDir,
		CustomPatterns: []string{"*.custom"},
	})

	customPath := filepath.Join(tmpDir, "data.custom")
	if !matcher.ShouldIgnore(customPath) {
		t.Error("expected custom pattern to ignore *.custom files")
	}
}

func Test_Matcher_FileSizeLimit(t *testing.T) {
	matcher := NewMatcher(MatcherOptions{
		RootDir:          t.TempDir(),
		MaxFileSizeBytes: 1024,
	})

	if !matcher.IsFileTooLarge(2048) {
		t.Error("expected 2KB file to exceed 1KB limit")
	}
	if matcher.IsFileTooLarge(512) {
		t.Error("expected 512B file to be within 1KB limit")
	}
}

func Test_Matcher_ShouldIgnoreDir(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{RootDir: tmpDir})

	tests := []struct {
		dirName string
		ignored bool
	}{
		{".git", true},
		{"node_modules", true},
		{"__pycache__", true},
		{".idea", true},
		{"src", false},
		{"lib", false},
	}

	for _, tt := range tests {
		dirPath := filepath.Join(tmpDir, tt.dirName)
		got := matcher.ShouldIgnoreDir(dirPath)
		if got != tt.ignored {
			t.Errorf("ShouldIgnoreDir(%s) = %v, want %v", tt.dirName, got, tt.ignored)
		}
	}
}

func Test_Matcher_DefaultMaxFileSize(t *testing.T) {
	matcher := NewMatcher(MatcherOptions{RootDir: t.TempDir()})
	if matcher.MaxFileSizeBytes() != 1024*1024 {
		t.Errorf("expected default max file size 1MB, got %d", matcher.MaxFileSizeBytes())
	}
}
