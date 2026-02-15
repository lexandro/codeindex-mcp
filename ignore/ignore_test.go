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

func Test_Matcher_ForceInclude_OverridesDefaultExclude(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.log"},
	})

	// *.log is in DefaultIgnorePatterns, but force-include should override
	logPath := filepath.Join(tmpDir, "app.log")
	if matcher.ShouldIgnore(logPath) {
		t.Error("expected *.log to NOT be ignored when force-included")
	}

	// *.exe is still default-excluded (not force-included)
	exePath := filepath.Join(tmpDir, "app.exe")
	if !matcher.ShouldIgnore(exePath) {
		t.Error("expected *.exe to still be ignored (not force-included)")
	}
}

func Test_Matcher_ForceInclude_OverridesGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.generated.go\n"), 0644)

	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.generated.go"},
	})

	generatedPath := filepath.Join(tmpDir, "models.generated.go")
	if matcher.ShouldIgnore(generatedPath) {
		t.Error("expected *.generated.go to NOT be ignored when force-included")
	}
}

func Test_Matcher_ForceInclude_OverridesCustomExclude(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		CustomPatterns:       []string{"*.custom"},
		ForceIncludePatterns: []string{"*.custom"},
	})

	customPath := filepath.Join(tmpDir, "data.custom")
	if matcher.ShouldIgnore(customPath) {
		t.Error("expected *.custom to NOT be ignored when both excluded and force-included")
	}
}

func Test_Matcher_ForceInclude_MultiplePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.log", "*.db"},
	})

	// Both patterns should work (additive)
	logPath := filepath.Join(tmpDir, "app.log")
	if matcher.ShouldIgnore(logPath) {
		t.Error("expected *.log to NOT be ignored when force-included")
	}

	dbPath := filepath.Join(tmpDir, "data.db")
	if matcher.ShouldIgnore(dbPath) {
		t.Error("expected *.db to NOT be ignored when force-included")
	}
}

func Test_Matcher_ForceInclude_NonMatchingFilesStillExcluded(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.log"},
	})

	// Force-including *.log should not affect other default excludes
	exePath := filepath.Join(tmpDir, "app.exe")
	if !matcher.ShouldIgnore(exePath) {
		t.Error("expected *.exe to still be ignored when only *.log is force-included")
	}

	pycPath := filepath.Join(tmpDir, "module.pyc")
	if !matcher.ShouldIgnore(pycPath) {
		t.Error("expected *.pyc to still be ignored when only *.log is force-included")
	}
}

func Test_Matcher_ForceInclude_DirectoryNotPruned(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.log"},
	})

	// When *.log is force-included, vendor/ should NOT be pruned
	// (because it might contain .log files)
	vendorPath := filepath.Join(tmpDir, "vendor")
	if matcher.ShouldIgnoreDir(vendorPath) {
		t.Error("expected vendor/ to NOT be pruned when force-include has wildcard pattern")
	}

	// node_modules should also not be pruned (wildcard pattern could match anywhere)
	nodeModulesPath := filepath.Join(tmpDir, "node_modules")
	if matcher.ShouldIgnoreDir(nodeModulesPath) {
		t.Error("expected node_modules/ to NOT be pruned when force-include has wildcard pattern")
	}
}

func Test_Matcher_ForceInclude_OverridesClaudeignore(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".claudeignore"), []byte("*.draft.md\n"), 0644)

	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.draft.md"},
	})

	draftPath := filepath.Join(tmpDir, "notes.draft.md")
	if matcher.ShouldIgnore(draftPath) {
		t.Error("expected *.draft.md to NOT be ignored when force-included over .claudeignore")
	}
}

func Test_Matcher_ForceInclude_DirectoryPrefixPattern(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"vendor/*.go"},
	})

	// vendor/utils.go should match the force-include pattern
	vendorGoPath := filepath.Join(tmpDir, "vendor", "utils.go")
	if matcher.ShouldIgnore(vendorGoPath) {
		t.Error("expected vendor/*.go to NOT be ignored when force-included")
	}

	// vendor/ directory should NOT be pruned (it's a prefix of the pattern)
	vendorDir := filepath.Join(tmpDir, "vendor")
	if matcher.ShouldIgnoreDir(vendorDir) {
		t.Error("expected vendor/ to NOT be pruned when force-include has vendor/*.go pattern")
	}

	// node_modules/ SHOULD be pruned (unrelated to vendor/*.go pattern)
	nodeModulesDir := filepath.Join(tmpDir, "node_modules")
	if !matcher.ShouldIgnoreDir(nodeModulesDir) {
		t.Error("expected node_modules/ to be pruned when force-include only has vendor/*.go pattern")
	}
}

func Test_Matcher_ForceInclude_GitAlwaysPruned(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := NewMatcher(MatcherOptions{
		RootDir:              tmpDir,
		ForceIncludePatterns: []string{"*.log"},
	})

	// .git should ALWAYS be pruned, even with force-include patterns
	gitPath := filepath.Join(tmpDir, ".git")
	if !matcher.ShouldIgnoreDir(gitPath) {
		t.Error("expected .git/ to ALWAYS be pruned regardless of force-include")
	}
}
