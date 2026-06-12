package watcher

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lexandro/codeindex-mcp/ignore"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Test_Watcher_NewDirectoryTree_EmitsCreateEvents verifies that when a directory
// tree appears (copy, unzip, git checkout), files already inside it are reported
// as create events even though they were written before the watch was attached.
func Test_Watcher_NewDirectoryTree_EmitsCreateEvents(t *testing.T) {
	tmpDir := t.TempDir()
	matcher := ignore.NewMatcher(ignore.MatcherOptions{RootDir: tmpDir})

	w, err := NewWatcher(tmpDir, matcher, testLogger())
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Close()
	go w.Start()

	// Build the tree in one burst, simulating a copied directory
	nestedDir := filepath.Join(tmpDir, "newdir", "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}
	os.WriteFile(filepath.Join(tmpDir, "newdir", "a.go"), []byte("package a\n"), 0644)
	os.WriteFile(filepath.Join(nestedDir, "b.go"), []byte("package b\n"), 0644)

	want := map[string]bool{
		filepath.Join(tmpDir, "newdir", "a.go"): false,
		filepath.Join(nestedDir, "b.go"):        false,
	}

	deadline := time.After(5 * time.Second)
	for {
		remaining := 0
		for _, seen := range want {
			if !seen {
				remaining++
			}
		}
		if remaining == 0 {
			break
		}

		select {
		case events := <-w.Events():
			for _, event := range events {
				if _, ok := want[event.Path]; ok && (event.Op == OpCreate || event.Op == OpWrite) {
					want[event.Path] = true
				}
			}
		case <-deadline:
			for path, seen := range want {
				if !seen {
					t.Errorf("no create event received for %s", path)
				}
			}
			return
		}
	}
}

func Test_Watcher_WatchedDirCount(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755)
	matcher := ignore.NewMatcher(ignore.MatcherOptions{RootDir: tmpDir})

	w, err := NewWatcher(tmpDir, matcher, testLogger())
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Close()

	if got := w.WatchedDirCount(); got != 2 {
		t.Errorf("expected 2 watched dirs (root + sub), got %d", got)
	}
}
