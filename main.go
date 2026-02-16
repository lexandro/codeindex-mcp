package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lexandro/codeindex-mcp/ignore"
	"github.com/lexandro/codeindex-mcp/index"
	"github.com/lexandro/codeindex-mcp/server"
	"github.com/lexandro/codeindex-mcp/tools"
	"github.com/lexandro/codeindex-mcp/watcher"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// excludePatterns is a repeatable CLI flag for custom ignore patterns.
type excludePatterns []string

func (e *excludePatterns) String() string { return strings.Join(*e, ", ") }
func (e *excludePatterns) Set(value string) error {
	*e = append(*e, value)
	return nil
}

// forceIncludePatterns is a repeatable CLI flag for force-include patterns that override all excludes.
type forceIncludePatterns []string

func (f *forceIncludePatterns) String() string { return strings.Join(*f, ", ") }
func (f *forceIncludePatterns) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	// Parse CLI flags
	var rootDir string
	var maxFileSizeBytes int64
	var maxResults int
	var logLevel string
	var logFile string
	var logEnabled bool
	var syncInterval int
	var excludes excludePatterns
	var forceIncludes forceIncludePatterns

	flag.StringVar(&rootDir, "root", "", "Project root directory (default: current working directory)")
	flag.Var(&excludes, "exclude", "Extra ignore pattern (repeatable)")
	flag.Var(&forceIncludes, "force-include", "Force-include pattern that overrides all excludes (repeatable)")
	flag.Int64Var(&maxFileSizeBytes, "max-file-size", 1024*1024, "Maximum file size in bytes (default: 1MB)")
	flag.IntVar(&maxResults, "max-results", 50, "Default max search results (default: 50)")
	flag.StringVar(&logLevel, "log-level", "info", "Log level: debug|info|warn|error")
	flag.StringVar(&logFile, "log-file", "", "Log file path (default: codeindex-mcp.log in root dir)")
	flag.BoolVar(&logEnabled, "log-enabled", true, "Enable logging (default: true, set to false to disable all logging)")
	flag.IntVar(&syncInterval, "sync-interval", 0, "Periodic sync interval in seconds (0 = disabled)")
	flag.Parse()

	if syncInterval < 0 {
		fmt.Fprintf(os.Stderr, "Error: --sync-interval must be >= 0\n")
		os.Exit(1)
	}

	// Resolve root directory
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
			os.Exit(1)
		}
	}
	rootDir, _ = filepath.Abs(rootDir)

	// Setup logger (always to file or stderr, never to stdout - stdout is for MCP stdio)
	var logger *slog.Logger
	if !logEnabled {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	} else {
		if logFile == "" {
			logFile = filepath.Join(rootDir, "codeindex-mcp.log")
		}
		var logFileHandle *os.File
		logger, logFileHandle = setupLogger(logLevel, logFile)
		if logFileHandle != nil {
			defer logFileHandle.Close()
		}
	}

	logger.Info("starting codeindex-mcp",
		"root", rootDir,
		"maxFileSize", maxFileSizeBytes,
		"maxResults", maxResults,
		"forceIncludes", []string(forceIncludes),
	)

	startTime := time.Now()

	// Create ignore matcher
	ignoreMatcher := ignore.NewMatcher(ignore.MatcherOptions{
		RootDir:              rootDir,
		CustomPatterns:       excludes,
		ForceIncludePatterns: forceIncludes,
		MaxFileSizeBytes:     maxFileSizeBytes,
	})

	// Create indexes
	fileIndex := index.NewFileIndex()
	contentIndex, err := index.NewContentIndex()
	if err != nil {
		logger.Error("failed to create content index", "error", err)
		os.Exit(1)
	}
	defer contentIndex.Close()

	// Perform initial indexing
	indexedCount, totalSize := performIndexing(rootDir, fileIndex, contentIndex, ignoreMatcher, logger)
	indexDuration := time.Since(startTime)
	logger.Info("initial indexing complete",
		"files", indexedCount,
		"totalSize", totalSize,
		"duration", indexDuration,
	)

	// Start file watcher
	fileWatcher, err := watcher.NewWatcher(rootDir, ignoreMatcher, logger)
	if err != nil {
		logger.Warn("failed to start file watcher, continuing without live updates", "error", err)
	} else {
		go fileWatcher.Start()
		go handleWatcherEvents(fileWatcher, rootDir, fileIndex, contentIndex, ignoreMatcher, logger)
		defer fileWatcher.Close()
	}

	// Start periodic sync if configured
	var syncStop chan struct{}
	if syncInterval > 0 {
		syncStop = make(chan struct{})
		go runPeriodicSync(syncInterval, rootDir, fileIndex, contentIndex, ignoreMatcher, logger, syncStop)
		defer close(syncStop)
	}

	// Create tool handlers
	searchHandler := &tools.SearchHandler{ContentIndex: contentIndex, Logger: logger}
	filesHandler := &tools.FilesHandler{FileIndex: fileIndex, Logger: logger}
	statusHandler := &tools.StatusHandler{
		FileIndex:    fileIndex,
		ContentIndex: contentIndex,
		StartTime:    startTime,
		RootDir:      rootDir,
		Logger:       logger,
	}
	readHandler := &tools.ReadHandler{ContentIndex: contentIndex, Logger: logger}
	reindexHandler := &tools.ReindexHandler{
		Logger: logger,
		DoReindex: func() (int, int64, string, error) {
			start := time.Now()
			fileIndex.Clear()
			if err := contentIndex.Clear(); err != nil {
				return 0, 0, "", fmt.Errorf("clearing content index: %w", err)
			}
			// Reload ignore rules in case .gitignore or .claudeignore changed
			ignoreMatcher.Reload()
			count, size := performIndexing(rootDir, fileIndex, contentIndex, ignoreMatcher, logger)
			elapsed := time.Since(start).Round(time.Millisecond).String()
			return count, size, elapsed, nil
		},
	}

	// Setup and run MCP server on stdio
	mcpServer := server.Setup(searchHandler, filesHandler, statusHandler, reindexHandler, readHandler)

	logger.Info("MCP server starting on stdio")
	if err := mcpServer.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Error("MCP server error", "error", err)
		os.Exit(1)
	}
}

// setupLogger creates an slog.Logger writing to stderr or a file.
// Returns the logger and the opened file (nil if using stderr), so the caller can defer Close().
func setupLogger(level string, logFile string) (*slog.Logger, *os.File) {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var writer *os.File
	var openedFile *os.File
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot open log file %s: %v, falling back to stderr\n", logFile, err)
			writer = os.Stderr
		} else {
			writer = f
			openedFile = f
		}
	} else {
		writer = os.Stderr
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: logLevel})
	return slog.New(handler), openedFile
}
