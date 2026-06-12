package ignore

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	gitignore "github.com/denormal/go-gitignore"
)

// Matcher determines whether a file path should be ignored during indexing.
// It combines default patterns, .gitignore rules, .claudeignore rules, and custom CLI patterns.
// Ignore files are matched hierarchically: nested .gitignore/.claudeignore files in
// subdirectories are respected, with deeper files taking precedence (standard git behavior).
// Thread-safe: Reload() acquires a write lock, ShouldIgnore()/ShouldIgnoreDir() acquire a read lock.
type Matcher struct {
	mu                   sync.RWMutex
	rootDir              string
	gitIgnores           []scopedIgnore
	claudeIgnores        []scopedIgnore
	customPatterns       []string
	forceIncludePatterns []string
	maxFileSizeBytes     int64
}

// scopedIgnore is one ignore file's matcher bound to the directory that contains it.
type scopedIgnore struct {
	dirRelPath string // directory containing the ignore file, relative to root ("" = root, forward slashes)
	matcher    gitignore.GitIgnore
}

// MatcherOptions configures the ignore matcher.
type MatcherOptions struct {
	RootDir              string
	CustomPatterns       []string
	ForceIncludePatterns []string
	MaxFileSizeBytes     int64
}

// NewMatcher creates an ignore matcher that checks default patterns, .gitignore, .claudeignore, and custom patterns.
func NewMatcher(options MatcherOptions) *Matcher {
	matcher := &Matcher{
		rootDir:              options.RootDir,
		customPatterns:       options.CustomPatterns,
		forceIncludePatterns: options.ForceIncludePatterns,
		maxFileSizeBytes:     options.MaxFileSizeBytes,
	}

	if matcher.maxFileSizeBytes <= 0 {
		matcher.maxFileSizeBytes = 1024 * 1024 // 1MB default
	}

	matcher.gitIgnores = loadIgnoreHierarchy(options.RootDir, ".gitignore")
	matcher.claudeIgnores = loadIgnoreHierarchy(options.RootDir, ".claudeignore")

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

	// Force-include overrides ALL exclude rules
	if m.matchesForceIncludePatterns(relativePath) {
		return false
	}

	// Check default patterns
	if m.matchesDefaultPatterns(relativePath, absolutePath) {
		return true
	}

	// Determine if path is a directory (for gitignore matching)
	isDir := false
	if info, err := os.Stat(absolutePath); err == nil {
		isDir = info.IsDir()
	}

	// Check .gitignore hierarchy (deepest ignore file wins, like git)
	if ignored, matched := matchScopedIgnores(m.gitIgnores, relativePath, isDir); matched {
		if ignored {
			return true
		}
		// An explicit negation (re-include) in .gitignore still falls through
		// to .claudeignore and custom patterns, matching previous behavior.
	}

	// Check .claudeignore hierarchy
	if ignored, matched := matchScopedIgnores(m.claudeIgnores, relativePath, isDir); matched && ignored {
		return true
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

	// .git is ALWAYS pruned — no force-include can override this
	if dirName == ".git" {
		return true
	}

	// If force-include patterns exist, check if this directory could contain force-included files.
	// If it could, don't prune it even if it would normally be ignored.
	m.mu.RLock()
	hasForceIncludes := len(m.forceIncludePatterns) > 0
	m.mu.RUnlock()

	if hasForceIncludes {
		relativePath, err := filepath.Rel(m.rootDir, absolutePath)
		if err != nil {
			relativePath = absolutePath
		}
		relativePath = filepath.ToSlash(relativePath)

		if m.couldContainForceIncluded(relativePath) {
			return false
		}
	}

	// Fast check: common directories that should always be skipped
	if isAlwaysSkippedDirName(dirName) {
		return true
	}

	// Full ignore check (includes .gitignore, .claudeignore, custom patterns)
	// ShouldIgnore acquires the read lock internally
	return m.ShouldIgnore(absolutePath)
}

// isAlwaysSkippedDirName reports whether a directory name is so common and so
// useless for code search that it is pruned without consulting ignore files.
func isAlwaysSkippedDirName(dirName string) bool {
	switch dirName {
	case ".svn", ".hg", "node_modules", "__pycache__",
		".idea", ".vscode", ".vs", ".next", ".nuxt",
		".cache", ".parcel-cache", "coverage", ".nyc_output", "htmlcov",
		".venv", "venv", ".env":
		return true
	}
	return false
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

// matchesForceIncludePatterns checks if the path matches any force-include pattern.
// Force-include patterns override ALL exclude rules (default, .gitignore, .claudeignore, custom).
func (m *Matcher) matchesForceIncludePatterns(relativePath string) bool {
	for _, pattern := range m.forceIncludePatterns {
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

// couldContainForceIncluded returns true if the directory might contain files matching force-include patterns.
// This prevents premature directory pruning when force-include patterns are active.
func (m *Matcher) couldContainForceIncluded(relativeDirPath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pattern := range m.forceIncludePatterns {
		// Pattern without directory component (e.g. "*.log") — could match files in ANY directory
		if !strings.Contains(pattern, "/") {
			return true
		}

		// Pattern has directory prefix (e.g. "vendor/*.go") — check if this dir is a prefix of the pattern's dir
		patternDir := pattern[:strings.LastIndex(pattern, "/")]
		if strings.HasPrefix(patternDir, relativeDirPath) || strings.HasPrefix(relativeDirPath, patternDir) {
			return true
		}
	}
	return false
}

// Reload re-reads all .gitignore and .claudeignore files from disk (at every depth).
// Used when the watcher detects changes to these files.
func (m *Matcher) Reload() {
	newGitIgnores := loadIgnoreHierarchy(m.rootDir, ".gitignore")
	newClaudeIgnores := loadIgnoreHierarchy(m.rootDir, ".claudeignore")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.gitIgnores = newGitIgnores
	m.claudeIgnores = newClaudeIgnores
}

// loadIgnoreHierarchy walks the tree under rootDir and loads every ignore file named
// fileName into a scoped matcher. The result is sorted deepest-first, so iterating in
// order gives git's precedence rule: the ignore file closest to the path wins.
func loadIgnoreHierarchy(rootDir string, fileName string) []scopedIgnore {
	var scoped []scopedIgnore

	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != rootDir && (d.Name() == ".git" || isAlwaysSkippedDirName(d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != fileName {
			return nil
		}

		containingDir := filepath.Dir(path)
		matcher := loadIgnoreFile(path, containingDir)
		if matcher == nil {
			return nil
		}

		dirRelPath, relErr := filepath.Rel(rootDir, containingDir)
		if relErr != nil {
			return nil
		}
		dirRelPath = filepath.ToSlash(dirRelPath)
		if dirRelPath == "." {
			dirRelPath = ""
		}
		scoped = append(scoped, scopedIgnore{dirRelPath: dirRelPath, matcher: matcher})
		return nil
	})

	sort.Slice(scoped, func(i, j int) bool {
		return ignoreDirDepth(scoped[i].dirRelPath) > ignoreDirDepth(scoped[j].dirRelPath)
	})
	return scoped
}

// ignoreDirDepth returns the directory depth of a root-relative path ("" = 0, "a" = 1, "a/b" = 2).
func ignoreDirDepth(dirRelPath string) int {
	if dirRelPath == "" {
		return 0
	}
	return strings.Count(dirRelPath, "/") + 1
}

// matchScopedIgnores matches a root-relative path against a deepest-first list of
// scoped ignore matchers. It returns (ignored, matched): matched is true when any
// ignore file had a pattern matching the path, and ignored is that pattern's verdict.
// The first (deepest) matching ignore file decides, like git.
func matchScopedIgnores(scoped []scopedIgnore, relativePath string, isDir bool) (bool, bool) {
	for _, s := range scoped {
		subPath := relativePath
		if s.dirRelPath != "" {
			prefix := s.dirRelPath + "/"
			if !strings.HasPrefix(relativePath, prefix) {
				continue
			}
			subPath = relativePath[len(prefix):]
		}

		match := s.matcher.Relative(subPath, isDir)
		if match != nil {
			return match.Ignore(), true
		}
	}
	return false, false
}

// loadIgnoreFile reads an ignore file and creates a GitIgnore matcher from it.
// Uses the io.Reader approach so the file handle is explicitly closed — the
// library's file-based constructors leak the handle, which locks files on Windows.
func loadIgnoreFile(filePath string, baseDir string) gitignore.GitIgnore {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	gi := gitignore.New(f, baseDir, nil)
	return gi
}
