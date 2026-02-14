package watcher

import (
	"sort"
	"testing"
	"time"
)

const testInterval = 50 * time.Millisecond

func receiveBatch(t *testing.T, d *Debouncer, timeout time.Duration) []DebouncedEvent {
	t.Helper()
	select {
	case batch := <-d.Output():
		return batch
	case <-time.After(timeout):
		t.Fatal("timed out waiting for debouncer batch")
		return nil
	}
}

func Test_Debouncer_SingleEvent(t *testing.T) {
	d := NewDebouncer(testInterval)

	d.Add("main.go", OpWrite)

	batch := receiveBatch(t, d, 500*time.Millisecond)

	if len(batch) != 1 {
		t.Fatalf("expected 1 event, got %d", len(batch))
	}
	if batch[0].Path != "main.go" {
		t.Errorf("expected path 'main.go', got '%s'", batch[0].Path)
	}
	if batch[0].Op != OpWrite {
		t.Errorf("expected OpWrite, got %d", batch[0].Op)
	}
}

func Test_Debouncer_EventCollapsing(t *testing.T) {
	d := NewDebouncer(testInterval)

	// Add the same path twice — should collapse to one event with the latest op
	d.Add("main.go", OpCreate)
	d.Add("main.go", OpWrite)

	batch := receiveBatch(t, d, 500*time.Millisecond)

	if len(batch) != 1 {
		t.Fatalf("expected 1 event (collapsed), got %d", len(batch))
	}
	if batch[0].Op != OpWrite {
		t.Errorf("expected latest op OpWrite, got %d", batch[0].Op)
	}
}

func Test_Debouncer_MultiplePaths(t *testing.T) {
	d := NewDebouncer(testInterval)

	d.Add("main.go", OpWrite)
	d.Add("util.go", OpCreate)
	d.Add("README.md", OpRemove)

	batch := receiveBatch(t, d, 500*time.Millisecond)

	if len(batch) != 3 {
		t.Fatalf("expected 3 events, got %d", len(batch))
	}

	// Sort by path for deterministic checks
	sort.Slice(batch, func(i, j int) bool {
		return batch[i].Path < batch[j].Path
	})

	expectedPaths := []string{"README.md", "main.go", "util.go"}
	for i, expected := range expectedPaths {
		if batch[i].Path != expected {
			t.Errorf("event[%d]: expected path '%s', got '%s'", i, expected, batch[i].Path)
		}
	}
}

func Test_Debouncer_TimerReset(t *testing.T) {
	d := NewDebouncer(testInterval)

	// Add first event
	d.Add("main.go", OpWrite)

	// Wait less than the interval, then add another event — should reset timer
	time.Sleep(testInterval / 2)
	d.Add("util.go", OpWrite)

	// Both events should arrive in a single batch
	batch := receiveBatch(t, d, 500*time.Millisecond)

	if len(batch) != 2 {
		t.Fatalf("expected 2 events in single batch, got %d", len(batch))
	}

	paths := make(map[string]bool)
	for _, e := range batch {
		paths[e.Path] = true
	}
	if !paths["main.go"] || !paths["util.go"] {
		t.Errorf("expected both main.go and util.go in batch, got: %v", batch)
	}
}
