package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lexandro/codeindex-mcp/ignore"
	"github.com/lexandro/codeindex-mcp/index"
)


// Test_Indexing_BatFilesAreIndexed verifies that .bat files in the root
// and subdirectories are picked up by the full indexing pipeline.
func Test_Indexing_BatFilesAreIndexed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .bat files similar to the mote-bo project
	batFiles := map[string]string{
		"start.bat":          "@echo off\r\necho Starting...\r\ndocker-compose up -d\r\n",
		"stop.bat":           "@echo off\r\necho Stopping...\r\ndocker-compose down\r\n",
		"restart.bat":        "call stop.bat\r\ncall start.bat\r\n",
		"logs.bat":           "@echo off\r\ndocker-compose logs -f\r\n",
		"scripts/deploy.bat": "@echo off\r\necho Deploying...\r\n",
	}

	for relPath, content := range batFiles {
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(relPath))
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", relPath, err)
		}
	}

	// Also create a non-bat file to verify normal indexing still works
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)

	matcher := ignore.NewMatcher(ignore.MatcherOptions{
		RootDir:          tmpDir,
		MaxFileSizeBytes: 1024 * 1024,
	})
	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	defer contentIndex.Close()

	performIndexing(tmpDir, fileIndex, contentIndex, matcher, nil, testLogger())

	// Check that all bat files are in the index
	for relPath := range batFiles {
		normalizedPath := filepath.ToSlash(relPath)
		if fileIndex.GetFile(normalizedPath) == nil {
			t.Errorf("expected %s to be indexed, but it was not found", normalizedPath)
		}
	}

	// Verify language detection
	for relPath := range batFiles {
		normalizedPath := filepath.ToSlash(relPath)
		f := fileIndex.GetFile(normalizedPath)
		if f != nil && f.Language != "Batch" {
			t.Errorf("expected %s language to be Batch, got %s", normalizedPath, f.Language)
		}
	}

	// Verify content is searchable
	results, _, _, err := contentIndex.Search(index.SearchOptions{Query: "docker-compose", MaxResults: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected to find 'docker-compose' in indexed bat file content, got 0 results")
	}
}

// Test_Indexing_BatFilesWithMoteboGitignore reproduces the exact mote-bo .gitignore
// to verify that unrelated .bat files are not accidentally excluded.
func Test_Indexing_BatFilesWithMoteboGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Exact .gitignore content from C:\projects\mote\mote-bo
	gitignoreContent := `# Dependencies
node_modules/
**/node_modules/
.pnp
.pnp.js

# Build outputs
dist/
build/
*.tsbuildinfo
packages/backend/public/

# Environment
.env
.env.local
.env.*.local

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Logs
logs/
*.log
npm-debug.log*

# Testing
coverage/
test_output/

# Misc
*.local

# Backups
backups/
*.dump
restore.bat
restore.log

# Bun
bun.lockb

# Claude
.claude/
.mcp.json
clau.bat
`
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignoreContent), 0644)

	batFiles := []string{"start.bat", "stop.bat", "restart.bat", "logs.bat", "visual-test.bat"}
	for _, name := range batFiles {
		os.WriteFile(filepath.Join(tmpDir, name), []byte("@echo off\r\necho "+name+"\r\n"), 0644)
	}
	// These should be excluded
	os.WriteFile(filepath.Join(tmpDir, "restore.bat"), []byte("@echo off\r\necho restore\r\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "clau.bat"), []byte("@echo off\r\necho clau\r\n"), 0644)

	matcher := ignore.NewMatcher(ignore.MatcherOptions{
		RootDir:          tmpDir,
		MaxFileSizeBytes: 1024 * 1024,
	})
	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	defer contentIndex.Close()

	performIndexing(tmpDir, fileIndex, contentIndex, matcher, nil, testLogger())

	for _, name := range batFiles {
		if fileIndex.GetFile(name) == nil {
			t.Errorf("expected %s to be indexed (not in .gitignore), but it was missing", name)
		}
	}
	if fileIndex.GetFile("restore.bat") != nil {
		t.Error("expected restore.bat to NOT be indexed (listed in .gitignore)")
	}
	if fileIndex.GetFile("clau.bat") != nil {
		t.Error("expected clau.bat to NOT be indexed (listed in .gitignore)")
	}
}

// Test_Indexing_BatFilesWithGitignore verifies that only gitignore-listed
// .bat files are excluded, not all bat files.
func Test_Indexing_BatFilesWithGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// .gitignore excludes only restore.bat (like in mote-bo)
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("restore.bat\n"), 0644)

	os.WriteFile(filepath.Join(tmpDir, "start.bat"), []byte("@echo off\r\necho start\r\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "restore.bat"), []byte("@echo off\r\necho restore\r\n"), 0644)

	matcher := ignore.NewMatcher(ignore.MatcherOptions{
		RootDir:          tmpDir,
		MaxFileSizeBytes: 1024 * 1024,
	})
	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatalf("failed to create content index: %v", err)
	}
	defer contentIndex.Close()

	performIndexing(tmpDir, fileIndex, contentIndex, matcher, nil, testLogger())

	if fileIndex.GetFile("start.bat") == nil {
		t.Error("expected start.bat to be indexed (not in .gitignore)")
	}
	if fileIndex.GetFile("restore.bat") != nil {
		t.Error("expected restore.bat to NOT be indexed (listed in .gitignore)")
	}
}

// Test_Ignore_BatFilesNotIgnoredByDefault verifies that the ignore matcher
// does not exclude .bat files out of the box.
func Test_Ignore_BatFilesNotIgnoredByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := ignore.NewMatcher(ignore.MatcherOptions{RootDir: tmpDir})

	batFiles := []string{"start.bat", "stop.bat", "scripts/deploy.bat"}
	for _, rel := range batFiles {
		absPath := filepath.Join(tmpDir, filepath.FromSlash(rel))
		if matcher.ShouldIgnore(absPath) {
			t.Errorf("expected %s to NOT be ignored by default, but it was", rel)
		}
	}
}

// Test_FileIndex_SearchByGlob_BatFilesAtRoot verifies that both *.bat and
// **/*.bat patterns match root-level .bat files.
func Test_FileIndex_SearchByGlob_BatFilesAtRoot(t *testing.T) {
	fi := index.NewFileIndex()
	fi.AddFile(&index.IndexedFile{RelativePath: "start.bat", Language: "Batch"})
	fi.AddFile(&index.IndexedFile{RelativePath: "stop.bat", Language: "Batch"})
	fi.AddFile(&index.IndexedFile{RelativePath: "scripts/deploy.bat", Language: "Batch"})

	tests := []struct {
		pattern       string
		expectedCount int
	}{
		{"*.bat", 2},       // root-level only
		{"**/*.bat", 3},    // all bat files including subdirs
		{"scripts/*.bat", 1},
	}

	for _, tt := range tests {
		results, err := fi.SearchByGlob(tt.pattern, 50)
		if err != nil {
			t.Fatalf("pattern %q: unexpected error: %v", tt.pattern, err)
		}
		if len(results) != tt.expectedCount {
			t.Errorf("pattern %q: expected %d results, got %d", tt.pattern, tt.expectedCount, len(results))
		}
	}
}
