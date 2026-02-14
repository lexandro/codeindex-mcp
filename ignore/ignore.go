package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	gitignore "github.com/denormal/go-gitignore"
)

// Matcher determines whether a file path should be ignored during indexing.
// It combines default patterns, .gitignore rules, .claudeignore rules, and custom CLI patterns.
// Thread-safe: Reload() acquires a write lock, ShouldIgnore()/ShouldIgnoreDir() acquire a read lock.
type Matcher struct {
	mu               sync.RWMutex
	rootDir          string
	gitIgnore        gitignore.GitIgnore
	claudeIgnore     gitignore.GitIgnore
	customPatterns   []string
	maxFileSizeBytes int64
}

// MatcherOptions configures the ignore matcher.
type MatcherOptions struct {
	RootDir          string
	CustomPatterns   []string
	MaxFileSizeBytes int64
}

// NewMatcher creates an ignore matcher that checks default patterns, .gitignore, .claudeignore, and custom patterns.
func NewMatcher(options MatcherOptions) *Matcher {
	matcher := &Matcher{
		rootDir:          options.RootDir,
		customPatterns:   options.CustomPatterns,
		maxFileSizeBytes: options.MaxFileSizeBytes,
	}

	if matcher.maxFileSizeBytes <= 0 {
		matcher.maxFileSizeBytes = 1024 * 1024 // 1MB default
	}

	// Load .gitignore from project root
	matcher.gitIgnore = loadIgnoreFile(filepath.Join(options.RootDir, ".gitignore"), options.RootDir)

	// Load .claudeignore from project root
	matcher.claudeIgnore = loadIgnoreFile(filepath.Join(options.RootDir, ".claudeignore"), options.RootDir)

	return matcher
}

// ShouldIgnore returns true if the given path should be excluded from indexing.
// The path should be an absolute path or relative to the root directory.
func (m *Matcher) ShouldIgnore(absolutePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get path relative to root for pattern matching
	relativePath, err := filepath.Rel(m.rootDir, absolutePath)
	if err != nil {
		relativePath = absolutePath
	}
	// Normalize to forward slashes for consistent matching
	relativePath = filepath.ToSlash(relativePath)

	// Check default patterns
	if m.matchesDefaultPatterns(relativePath, absolutePath) {
		return true
	}

	// Determine if path is a directory (for gitignore matching)
	isDir := false
	if info, err := os.Stat(absolutePath); err == nil {
		isDir = info.IsDir()
	}

	// Check .gitignore using Relative() which doesn't require the file to exist on disk
	if m.gitIgnore != nil {
		match := m.gitIgnore.Relative(relativePath, isDir)
		if match != nil && match.Ignore() {
			return true
		}
	}

	// Check .claudeignore using Relative()
	if m.claudeIgnore != nil {
		match := m.claudeIgnore.Relative(relativePath, isDir)
		if match != nil && match.Ignore() {
			return true
		}
	}

	// Check custom CLI patterns
	if m.matchesCustomPatterns(relativePath) {
		return true
	}

	return false
}

// ShouldIgnoreDir returns true if a directory should be skipped entirely during traversal.
func (m *Matcher) ShouldIgnoreDir(absolutePath string) bool {
	dirName := filepath.Base(absolutePath)

	// Fast check: common directories that should always be skipped (no lock needed)
	switch dirName {
	case ".git", ".svn", ".hg", "node_modules", "__pycache__",
		".idea", ".vscode", ".vs", ".next", ".nuxt",
		".cache", ".parcel-cache", "coverage", ".nyc_output", "htmlcov",
		".venv", "venv", ".env":
		return true
	}

	// Full ignore check (includes .gitignore, .claudeignore, custom patterns)
	// ShouldIgnore acquires the read lock internally
	return m.ShouldIgnore(absolutePath)
}

// IsFileTooLarge returns true if the file exceeds the max file size limit.
func (m *Matcher) IsFileTooLarge(fileSize int64) bool {
	return fileSize > m.maxFileSizeBytes
}

// MaxFileSizeBytes returns the configured maximum file size.
func (m *Matcher) MaxFileSizeBytes() int64 {
	return m.maxFileSizeBytes
}

// matchesDefaultPatterns checks if the path matches any hardcoded default ignore pattern.
func (m *Matcher) matchesDefaultPatterns(relativePath string, absolutePath string) bool {
	baseName := filepath.Base(absolutePath)
	baseNameLower := strings.ToLower(baseName)

	for _, pattern := range DefaultIgnorePatterns {
		// Pattern is a directory/file name (no glob) - check path components
		if !strings.ContainsAny(pattern, "*?[") {
			// Exact basename match
			if baseNameLower == strings.ToLower(pattern) {
				return true
			}
			// Check if any path component matches
			parts := strings.Split(relativePath, "/")
			for _, part := range parts {
				if strings.ToLower(part) == strings.ToLower(pattern) {
					return true
				}
			}
			continue
		}

		// Glob pattern - match against basename
		matched, err := filepath.Match(strings.ToLower(pattern), baseNameLower)
		if err == nil && matched {
			return true
		}

		// Also try matching against the full relative path
		matched, err = filepath.Match(strings.ToLower(pattern), strings.ToLower(relativePath))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// matchesCustomPatterns checks if the path matches any user-provided CLI exclude pattern.
func (m *Matcher) matchesCustomPatterns(relativePath string) bool {
	for _, pattern := range m.customPatterns {
		// Try matching against relative path
		matched, err := filepath.Match(pattern, relativePath)
		if err == nil && matched {
			return true
		}

		// Try matching against basename
		baseName := filepath.Base(relativePath)
		matched, err = filepath.Match(pattern, baseName)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// Reload re-reads .gitignore and .claudeignore files from disk.
// Used when the watcher detects changes to these files.
func (m *Matcher) Reload() {
	newGitIgnore := loadIgnoreFile(filepath.Join(m.rootDir, ".gitignore"), m.rootDir)
	newClaudeIgnore := loadIgnoreFile(filepath.Join(m.rootDir, ".claudeignore"), m.rootDir)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.gitIgnore = newGitIgnore
	m.claudeIgnore = newClaudeIgnore
}

// loadIgnoreFile reads an ignore file and creates a GitIgnore matcher from it.
// Uses io.Reader approach to ensure the file handle is properly closed on Windows.
func loadIgnoreFile(filePath string, baseDir string) gitignore.GitIgnore {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	gi := gitignore.New(f, baseDir, nil)
	return gi
}
