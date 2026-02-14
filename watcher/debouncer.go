package watcher

import (
	"sync"
	"time"
)

// DebouncedEvent represents a batched file system event.
type DebouncedEvent struct {
	Path string
	Op   EventOp
}

// EventOp represents the type of file system operation.
type EventOp int

const (
	OpCreate EventOp = iota
	OpWrite
	OpRemove
	OpRename
)

// Debouncer collects file system events and emits batched events after a quiet period.
// Multiple events for the same path within the debounce window are collapsed into one.
type Debouncer struct {
	interval time.Duration
	events   map[string]DebouncedEvent
	mu       sync.Mutex
	timer    *time.Timer
	output   chan []DebouncedEvent
}

// NewDebouncer creates a debouncer with the specified quiet interval.
func NewDebouncer(interval time.Duration) *Debouncer {
	return &Debouncer{
		interval: interval,
		events:   make(map[string]DebouncedEvent),
		output:   make(chan []DebouncedEvent, 16),
	}
}

// Output returns the channel that receives batched events.
func (d *Debouncer) Output() <-chan []DebouncedEvent {
	return d.output
}

// Add adds an event to the debounce window. If an event for the same path
// already exists, it is replaced with the latest operation.
func (d *Debouncer) Add(path string, op EventOp) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.events[path] = DebouncedEvent{Path: path, Op: op}

	// Reset the timer each time a new event arrives
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.interval, d.flush)
}

// flush sends the accumulated events to the output channel and resets the buffer.
func (d *Debouncer) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.events) == 0 {
		return
	}

	batch := make([]DebouncedEvent, 0, len(d.events))
	for _, event := range d.events {
		batch = append(batch, event)
	}

	d.events = make(map[string]DebouncedEvent)
	d.output <- batch
}
