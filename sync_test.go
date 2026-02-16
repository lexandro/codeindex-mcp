package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lexandro/codeindex-mcp/ignore"
	"github.com/lexandro/codeindex-mcp/index"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testIgnoreMatcher(rootDir string) *ignore.Matcher {
	return ignore.NewMatcher(ignore.MatcherOptions{
		RootDir:          rootDir,
		MaxFileSizeBytes: 1024 * 1024,
	})
}

func Test_performSyncVerification_DetectsMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Create a file on disk but don't index it
	filePath := filepath.Join(tmpDir, "missing.go")
	os.WriteFile(filePath, []byte("package main\n"), 0644)

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.MissingFiles != 1 {
		t.Errorf("expected 1 missing file, got %d", result.MissingFiles)
	}
	if result.StaleFiles != 0 {
		t.Errorf("expected 0 stale files, got %d", result.StaleFiles)
	}
	if result.ModifiedFiles != 0 {
		t.Errorf("expected 0 modified files, got %d", result.ModifiedFiles)
	}

	// Verify the file was actually indexed
	if fileIndex.GetFile("missing.go") == nil {
		t.Error("expected missing.go to be indexed after sync")
	}
}

func Test_performSyncVerification_DetectsStaleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Add a file to the index that doesn't exist on disk
	fileIndex.AddFile(&index.IndexedFile{
		Path:         filepath.Join(tmpDir, "deleted.go"),
		RelativePath: "deleted.go",
		Language:     "Go",
		SizeBytes:    100,
		ModTime:      time.Now(),
		LineCount:    5,
	})
	contentIndex.IndexFile("deleted.go", "package main\n", "Go")

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.StaleFiles != 1 {
		t.Errorf("expected 1 stale file, got %d", result.StaleFiles)
	}
	if result.MissingFiles != 0 {
		t.Errorf("expected 0 missing files, got %d", result.MissingFiles)
	}

	// Verify the file was removed from index
	if fileIndex.GetFile("deleted.go") != nil {
		t.Error("expected deleted.go to be removed from index after sync")
	}
}

func Test_performSyncVerification_DetectsModifiedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Create and index a file
	filePath := filepath.Join(tmpDir, "modified.go")
	os.WriteFile(filePath, []byte("package main\n"), 0644)

	info, _ := os.Stat(filePath)
	fileIndex.AddFile(&index.IndexedFile{
		Path:         filePath,
		RelativePath: "modified.go",
		Language:     "Go",
		SizeBytes:    info.Size(),
		ModTime:      info.ModTime().Add(-1 * time.Hour), // old ModTime
		LineCount:    1,
	})
	contentIndex.IndexFile("modified.go", "package main\n", "Go")

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.ModifiedFiles != 1 {
		t.Errorf("expected 1 modified file, got %d", result.ModifiedFiles)
	}
	if result.MissingFiles != 0 {
		t.Errorf("expected 0 missing files, got %d", result.MissingFiles)
	}
	if result.StaleFiles != 0 {
		t.Errorf("expected 0 stale files, got %d", result.StaleFiles)
	}
}

func Test_performSyncVerification_InSyncReturnsZeros(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Create and properly index a file
	filePath := filepath.Join(tmpDir, "synced.go")
	os.WriteFile(filePath, []byte("package main\n"), 0644)

	info, _ := os.Stat(filePath)
	fileIndex.AddFile(&index.IndexedFile{
		Path:         filePath,
		RelativePath: "synced.go",
		Language:     "Go",
		SizeBytes:    info.Size(),
		ModTime:      info.ModTime(),
		LineCount:    1,
	})
	contentIndex.IndexFile("synced.go", "package main\n", "Go")

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.MissingFiles != 0 {
		t.Errorf("expected 0 missing files, got %d", result.MissingFiles)
	}
	if result.StaleFiles != 0 {
		t.Errorf("expected 0 stale files, got %d", result.StaleFiles)
	}
	if result.ModifiedFiles != 0 {
		t.Errorf("expected 0 modified files, got %d", result.ModifiedFiles)
	}
}

func Test_performSyncVerification_SkipsBinaryFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Create a binary file (contains null bytes)
	binaryPath := filepath.Join(tmpDir, "image.dat")
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x0A, 0x1A, 0x0A}
	os.WriteFile(binaryPath, binaryData, 0644)

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	// Binary file should not count as missing (it's skipped by indexSingleFile)
	if result.MissingFiles != 0 {
		t.Errorf("expected 0 missing files (binary skipped), got %d", result.MissingFiles)
	}
	if fileIndex.GetFile("image.dat") != nil {
		t.Error("expected binary file to NOT be indexed")
	}
}

func Test_performSyncVerification_SkipsIgnoredDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Create node_modules directory with a file (default ignored)
	nodeModulesDir := filepath.Join(tmpDir, "node_modules")
	os.Mkdir(nodeModulesDir, 0755)
	os.WriteFile(filepath.Join(nodeModulesDir, "index.js"), []byte("module.exports = {};\n"), 0644)

	// Create a normal file
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.MissingFiles != 1 {
		t.Errorf("expected 1 missing file (main.go only), got %d", result.MissingFiles)
	}
	if fileIndex.GetFile("node_modules/index.js") != nil {
		t.Error("expected files in node_modules to be ignored")
	}
	if fileIndex.GetFile("main.go") == nil {
		t.Error("expected main.go to be indexed")
	}
}

func Test_performSyncVerification_SkipsTooLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()

	// Matcher with 100 byte size limit
	matcher := ignore.NewMatcher(ignore.MatcherOptions{
		RootDir:          tmpDir,
		MaxFileSizeBytes: 100,
	})

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	// Create a small file (under limit)
	os.WriteFile(filepath.Join(tmpDir, "small.go"), []byte("package main\n"), 0644)

	// Create a large file (over limit)
	largeContent := make([]byte, 200)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	os.WriteFile(filepath.Join(tmpDir, "large.go"), largeContent, 0644)

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.MissingFiles != 1 {
		t.Errorf("expected 1 missing file (small.go only), got %d", result.MissingFiles)
	}
	if fileIndex.GetFile("large.go") != nil {
		t.Error("expected large.go to be skipped (too large)")
	}
}

func Test_performSyncVerification_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	result := performSyncVerification(tmpDir, fileIndex, contentIndex, matcher, logger)

	if result.MissingFiles != 0 {
		t.Errorf("expected 0 missing files, got %d", result.MissingFiles)
	}
	if result.StaleFiles != 0 {
		t.Errorf("expected 0 stale files, got %d", result.StaleFiles)
	}
	if result.ModifiedFiles != 0 {
		t.Errorf("expected 0 modified files, got %d", result.ModifiedFiles)
	}
	if result.Duration == 0 {
		t.Error("expected Duration to be set even for empty directory")
	}
}

func Test_runPeriodicSync_StopsOnChannelClose(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	matcher := testIgnoreMatcher(tmpDir)

	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		t.Fatal(err)
	}
	defer contentIndex.Close()

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		runPeriodicSync(1, tmpDir, fileIndex, contentIndex, matcher, logger, stop)
		close(done)
	}()

	// Close stop channel to signal shutdown
	close(stop)

	// Wait for goroutine to finish with timeout
	select {
	case <-done:
		// OK - goroutine stopped cleanly
	case <-time.After(3 * time.Second):
		t.Fatal("runPeriodicSync did not stop within 3 seconds after closing stop channel")
	}
}
