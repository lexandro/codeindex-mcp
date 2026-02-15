# codeindex-mcp

In-memory [MCP](https://modelcontextprotocol.io/) server for source code indexing. A fast, indexed replacement for `grep` and `find`, designed for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) and any MCP-compatible client.

## Why?

- **Orders of magnitude faster** than `grep`/`find` on large codebases — uses a pre-built in-memory index
- **Full-text search** powered by Bleve (word, exact phrase, and regex queries)
- **Glob-based file search** with `**` doublestar support
- **Auto-updating** — a background file watcher keeps the index in sync with disk
- **Configurable filtering** — respects `.gitignore`, `.claudeignore`, and custom exclude patterns
- **Zero runtime dependencies** — single static Go binary (~17 MB)

## Installation

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)

### Build from source

```bash
git clone https://github.com/lexandro/codeindex-mcp.git
cd codeindex-mcp
go build -o codeindex-mcp .
```

On Windows this produces `codeindex-mcp.exe`.

### Run tests

```bash
go test ./...
```

## Usage

### Standalone (for testing)

```bash
./codeindex-mcp --root /path/to/project
```

The server communicates over stdio (stdin/stdout) using the MCP protocol, so it is not interactive on its own — use it from an MCP client.

### Claude Code integration

Add to your Claude Code MCP settings (`.claude/settings.json` or global settings):

```json
{
  "mcpServers": {
    "codeindex": {
      "command": "/path/to/codeindex-mcp",
      "args": ["--root", "/path/to/project"]
    }
  }
}
```

For project-specific configuration, create `.mcp.json` in the project root:

```json
{
  "mcpServers": {
    "codeindex": {
      "command": "/path/to/codeindex-mcp",
      "args": ["--root", "."]
    }
  }
}
```

Claude Code will then automatically use `codeindex_search`, `codeindex_files`, `codeindex_read`, `codeindex_status`, and `codeindex_reindex` tools.

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--root DIR` | current directory | Project root directory to index |
| `--exclude PATTERN` | _(none)_ | Extra ignore pattern, repeatable (e.g. `--exclude "*.generated.go" --exclude "vendor/"`) |
| `--force-include PATTERN` | _(none)_ | Force-include pattern that overrides all excludes, repeatable (e.g. `--force-include "*.log"`) |
| `--max-file-size N` | `1048576` (1 MB) | Maximum file size in bytes; larger files are skipped |
| `--max-results N` | `50` | Default maximum number of search results |
| `--log-level LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--log-file PATH` | `<root>/codeindex-mcp.log` | Log file path |

### Examples

```bash
# Index the current directory
./codeindex-mcp

# Specify project root with extra exclusions
./codeindex-mcp --root ~/myproject \
  --exclude "*.generated.go" \
  --exclude "testdata/"

# Force-include log files (overrides the default *.log exclusion)
./codeindex-mcp --root . --force-include "*.log"

# Multiple force-include patterns (additive)
./codeindex-mcp --root . --force-include "*.log" --force-include "vendor/*.go"

# Combine exclude and force-include
./codeindex-mcp --root ~/myproject \
  --exclude "*.generated.go" \
  --force-include "*.log"

# Debug logging to a specific file
./codeindex-mcp --root . --log-level debug --log-file /tmp/codeindex.log

# Allow larger files (5 MB)
./codeindex-mcp --root . --max-file-size 5242880
```

## MCP Tools

The server registers 5 tools:

### 1. `codeindex_search` — Content search

Full-text search across all indexed file contents.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | yes | Search query (see formats below) |
| `filePath` | string | no | Exact relative path to search in a single file (overrides `fileGlob`) |
| `fileGlob` | string | no | Glob pattern to filter files (e.g. `**/*.go`) |
| `maxResults` | int | no | Maximum number of file results (default: 50) |
| `contextLines` | int | no | Context lines before/after each match (default: 2) |

**Query formats:**

| Format | Example | Behavior |
|--------|---------|----------|
| Plain text | `handleRequest` | Word-level matching (Bleve MatchQuery) |
| `"quoted"` | `"func main"` | Exact phrase matching (PhraseQuery) |
| `/regex/` | `/func\s+\w+Handler/` | Regular expression (RegexpQuery) |

**Example output:**

```
Found 3 matches in 2 files:

── main.go ──
  4: import "fmt"
  5:
  6: func main() {
  7:     fmt.Println("hello world")
  8: }

── server/server.go ──
  14: func main() {
  15:     startServer()
  16: }
```

### 2. `codeindex_files` — File search

Glob-based file search across the index.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `pattern` | string | yes | Glob pattern (e.g. `**/*.ts`, `src/**/*.go`) |
| `nameOnly` | bool | no | If `true`, return only file paths without metadata |
| `maxResults` | int | no | Maximum number of results (default: 50) |

**Example output:**

```
Found 4 files:

  src/main.go  (Go, 2.1 KB, 85 lines)
  src/utils/helper.go  (Go, 1.3 KB, 42 lines)
  src/server/handler.go  (Go, 4.7 KB, 156 lines)
  src/config/config.go  (Go, 892 B, 31 lines)
```

### 3. `codeindex_read` — Read file from index

Read a file's contents directly from the in-memory index. Zero disk I/O — faster than the built-in Read tool.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `filePath` | string | yes | Relative file path to read (e.g. `src/main.go`) |

**Example output:**

```
── src/main.go (12 lines) ──
 1│ package main
 2│
 3│ import "fmt"
 4│
 5│ func main() {
 6│     fmt.Println("hello")
 7│ }
```

### 4. `codeindex_status` — Index status

Display current index statistics.

**Parameters:** none

**Example output:**

```
=== codeindex-mcp Status ===

Root directory: /home/user/myproject
Uptime: 45s
Indexed files: 1234
Content-indexed documents: 1234
Total indexed size: 8.5 MB
Memory usage: 95.2 MB (heap: 82.1 MB)

Languages:
  TypeScript           456 files
  Go                   312 files
  JavaScript           189 files
  Python               98 files
```

### 5. `codeindex_reindex` — Force reindex

Clear the index and rebuild from scratch. Also reloads `.gitignore` and `.claudeignore` rules.

**Parameters:** none

**Example output:**

```
Reindex complete.
  Files indexed: 1234
  Total size: 8.5 MB
  Duration: 1.234s
```

## Ignore system

The server uses a multi-layered filtering system to determine which files to index:

### 1. Built-in default patterns

Automatically skipped without any configuration:

| Category | Patterns |
|----------|----------|
| Version control | `.git`, `.svn`, `.hg` |
| Dependencies | `node_modules`, `vendor`, `bower_components`, `.yarn` |
| Build output | `dist`, `build`, `out`, `target`, `bin`, `obj` |
| IDE files | `.idea`, `.vscode`, `.vs` |
| Binaries | `*.exe`, `*.dll`, `*.so`, `*.dylib`, `*.class`, `*.jar` |
| Images | `*.png`, `*.jpg`, `*.gif`, `*.webp`, `*.ico` |
| Fonts | `*.woff`, `*.woff2`, `*.ttf`, `*.eot` |
| Media | `*.mp3`, `*.mp4`, `*.avi`, `*.mov` |
| Documents | `*.pdf`, `*.doc`, `*.xlsx`, `*.pptx` |
| Lock files | `package-lock.json`, `yarn.lock`, `go.sum`, `Cargo.lock` |
| Archives | `*.zip`, `*.tar`, `*.tar.gz`, `*.rar`, `*.7z` |
| Minified | `*.min.js`, `*.min.css` |
| Source maps | `*.map` |
| Cache | `.cache`, `.next`, `.nuxt`, `.parcel-cache` |
| Logs | `*.log` |
| Database | `*.sqlite`, `*.sqlite3`, `*.db` |

### 2. `.gitignore` support

Fully respects `.gitignore` patterns in the project root, including globs, negation (`!important.log`), and directory-specific patterns.

### 3. `.claudeignore` support

A `.claudeignore` file in the project root uses the same syntax as `.gitignore`. Use it to exclude files from the index that you want in git but are not relevant for AI code search.

Example `.claudeignore`:
```
# Generated files
*.generated.go
*.pb.go

# Large test fixtures
testdata/large/

# Archived migrations
migrations/archive/
```

### 4. CLI `--exclude` patterns

Runtime exclusions via the `--exclude` flag:

```bash
./codeindex-mcp --exclude "*.generated.go" --exclude "vendor/"
```

### 5. CLI `--force-include` patterns

Force-include patterns override **all** exclude rules (built-in defaults, `.gitignore`, `.claudeignore`, and `--exclude`). Multiple `--force-include` flags are additive. Binary detection and file size limits still apply.

```bash
# Index *.log files even though they are excluded by default
./codeindex-mcp --force-include "*.log"

# Force-include vendor Go files while still excluding the rest of vendor/
./codeindex-mcp --force-include "vendor/*.go"
```

When force-include patterns are active, directories that might contain matching files are not pruned during traversal. The `.git` directory is always skipped regardless of force-include patterns.

### 6. Binary file detection

Scans the first 512 bytes of each file for null bytes. If found, the file is treated as binary and skipped. This works independently of `.gitignore`.

### 7. File size limit

Configurable via `--max-file-size` (default: 1 MB). Files larger than this are skipped.

### Priority

Filters are applied in order:
1. **`--force-include` patterns** (highest priority — if matched, the file is included regardless of rules 2–5)
2. Built-in default patterns
3. `.gitignore` rules
4. `.claudeignore` rules
5. CLI `--exclude` patterns
6. Binary detection (always applies, even for force-included files)
7. File size limit (always applies, even for force-included files)

If a force-include pattern matches, the file bypasses all exclude rules (2–5). Binary detection and file size limits are safety checks that always apply.

## Architecture

```
MCP Client (stdio) <──> MCP Server <──> Index Engine
                                            │
                                    ┌───────┼────────┐
                                    │       │        │
                                  Bleve   FileMap   Watcher
                               (full-text) (path)  (fsnotify)
```

### Dual index design

| Index | Technology | Purpose |
|-------|-----------|---------|
| **Content Index** | Bleve `NewMemOnly()` | Full-text search over file contents (inverted index) |
| **File Path Index** | Go `map` + sorted slice | File name/path search with glob patterns |

### File watcher

- Uses **fsnotify** (on Windows: `ReadDirectoryChangesW` API)
- Recursive: watches all non-ignored subdirectories at startup
- **100ms debounce window**: editors generate multiple events on save — these are collapsed into one
- Automatically watches newly created directories
- Automatically reloads ignore rules when `.gitignore` or `.claudeignore` changes

### Startup sequence

1. Parse CLI flags
2. Create ignore matcher (built-in + .gitignore + .claudeignore + CLI patterns)
3. Initialize Bleve in-memory index and file path index
4. Parallel indexing with 8 worker goroutines
5. Start file watcher
6. Start MCP server on stdio transport

## Project structure

```
codeindex-mcp/
├── main.go                  # Entry point, CLI flags, component wiring
├── indexing.go              # Directory walking, parallel indexing, watcher events
├── server/
│   └── server.go            # MCP server setup, tool registration
├── index/
│   ├── content.go           # Bleve content index (CRUD operations)
│   ├── content_search.go    # Full-text search logic, query parsing
│   ├── content_test.go
│   ├── files.go             # File path index (glob search) + IndexedFile type
│   └── files_test.go
├── watcher/
│   ├── watcher.go           # Recursive fsnotify wrapper
│   └── debouncer.go         # 100ms event collapsing
├── ignore/
│   ├── ignore.go            # .gitignore + .claudeignore + custom patterns
│   ├── ignore_test.go
│   └── defaults.go          # Built-in ignore patterns
├── tools/
│   ├── search.go            # codeindex_search handler
│   ├── files.go             # codeindex_files handler
│   ├── read.go              # codeindex_read handler
│   ├── status.go            # codeindex_status handler
│   ├── reindex.go           # codeindex_reindex handler
│   └── format.go            # Output formatting
└── language/
    ├── detect.go            # Extension → language mapping (70+)
    ├── detect_test.go
    ├── binary.go            # Binary file detection
    └── binary_test.go
```

## Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | v1.3.0 | MCP server (stdio transport) |
| [blevesearch/bleve/v2](https://github.com/blevesearch/bleve) | v2.5.7 | In-memory full-text search |
| [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | v1.9.0 | File system watching |
| [bmatcuk/doublestar/v4](https://github.com/bmatcuk/doublestar) | v4.10.0 | `**` glob support |
| [denormal/go-gitignore](https://github.com/denormal/go-gitignore) | latest | .gitignore / .claudeignore parsing |

## Performance

| Metric | ~5k files | ~10k files |
|--------|-----------|------------|
| Initial indexing | ~1-2s | ~2-3s |
| Memory usage | ~75-100 MB | ~180-230 MB |
| Text search | <5ms | <10ms |
| Regex search | <50ms | <50ms |
| Glob search | <2ms | <5ms |
| Incremental update | <10ms/file | <10ms/file |

## Supported languages

Language detection recognizes 70+ file extensions, including:

Go, TypeScript, JavaScript, Python, Rust, Java, Kotlin, C, C++, C#, Swift, Dart, Ruby, PHP, Shell, PowerShell, HTML, CSS, SCSS, Sass, Less, JSON, YAML, TOML, XML, SQL, GraphQL, Protobuf, Terraform, Lua, R, Scala, Elixir, Erlang, Haskell, Zig, Vue, Svelte, Markdown, Dockerfile, Makefile, CMake, Batch, and more.

## License

[MIT](LICENSE)
