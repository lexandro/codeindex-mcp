package index

import "time"

// IndexedFile represents a file that has been indexed.
// Used by both the file path index and the content index.
type IndexedFile struct {
	Path         string    // Absolute file path
	RelativePath string    // Path relative to project root (forward slashes)
	Language     string    // Detected programming language
	SizeBytes    int64     // File size in bytes
	ModTime      time.Time // Last modification time
	LineCount    int       // Number of lines in the file
}
