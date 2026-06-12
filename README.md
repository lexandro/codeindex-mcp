# codeindex-mcp

[![CI](https://github.com/lexandro/codeindex-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/lexandro/codeindex-mcp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lexandro/codeindex-mcp)](https://goreportcard.com/report/github.com/lexandro/codeindex-mcp)
[![Go Reference](https://pkg.go.dev/badge/github.com/lexandro/codeindex-mcp.svg)](https://pkg.go.dev/github.com/lexandro/codeindex-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-compatible-blue)](https://modelcontextprotocol.io/)
[![Claude Code](https://img.shields.io/badge/Claude_Code-Extension-blueviolet)](https://docs.anthropic.com/en/docs/claude-code)

In-memory [MCP](https://modelcontextprotocol.io/) server for source code indexing. A fast, indexed replacement for `grep` and `find`, designed for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) and any MCP-compatible client.

## Why?

- **Orders of magnitude faster** than `grep`/`find` on large codebases — all file contents are served from an in-memory index
- **Exact grep semantics** — literal substring, exact phrase, and RE2 regex queries with full recall (`mutex` finds `sync.RWMutex`); no tokenizer false negatives
- **Token-efficient output** — merged context hunks, per-file match caps, and `files`/`count` output modes designed for AI agents
- **Glob-based file search** with `**` doublestar support
- **Auto-updating** — a background file watcher plus periodic sync verification keep the index consistent with disk
- **Configurable filtering** — respects `.gitignore` and `.claudeignore` at every directory level, plus custom exclude patterns
- **Single binary** — no runtime dependencies; lightweight build has zero CGo, full AST build includes tree-sitter grammars

## Quick start

```bash
# Register for a project (creates .mcp.json)
./codeindex-mcp register project /path/to/your/project

# Or register globally for all projects (updates ~/.claude.json)
./codeindex-mcp register user
```

That's it — Claude Code will automatically discover and use the indexed search tools.

## Installation

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- **GCC C compiler** — only required for the full AST build (`make build-ast`); the lightweight build has no CGo dependency

The setup scripts handle everything automatically.

### Build from source

```bash
git clone https://github.com/lexandro/codeindex-mcp.git
cd codeindex-mcp
```

**First-time setup** (installs Go, GCC, make if missing):

```bash
# Windows
powershell -ExecutionPolicy Bypass -File scripts/setup_build.ps1

# Linux / macOS
bash scripts/setup_build.sh
```

The script detects your package manager (apt, dnf, pacman, brew, winget), installs any missing dependencies, and prints the exact build command at the end.

**Two build variants:**

| | Lightweight | Full AST |
|---|---|---|
| Binary size | ~18 MB | ~31 MB |
| CGo / GCC | not required | required |
| Go AST indexing | yes | yes |
| TypeScript / Python / JS AST | no | yes |
| Build command | `make build` | `make build-ast` |

```bash
# Lightweight — no GCC needed, works everywhere
make build
make test

# Full AST — requires GCC (run setup_build first)
make build-ast
make test-ast
```

Without `make`:
```bash
go build -o codeindex-mcp .                               # lightweight
CGO_ENABLED=1 go build -tags ast -o codeindex-mcp .      # full AST
```

### Register in Claude Code

After building, register the server so Claude Code can find it:

```bash
# Project-specific (writes .mcp.json in the target directory)
./codeindex-mcp register project /path/to/your/project

# Global (writes ~/.claude.json — available in all projects)
./codeindex-mcp register user

# With extra server flags
./codeindex-mcp register project . -- --max-file-size 5242880 --exclude "vendor/"
```

The `register` command auto-detects the binary path and creates the correct config entry, including the `cmd /C` wrapper on Windows.

### Run tests

```bash
make test        # lightweight (no CGo)
make test-ast    # full AST build (requires GCC)

# manually:
go test ./...                              # lightweight
CGO_ENABLED=1 go test -tags ast ./...     # full AST
```

## Usage

### Standalone (for testing)

```bash
./codeindex-mcp --root /path/to/project
```

The server communicates over stdio (stdin/stdout) using the MCP protocol, so it is not interactive on its own — use it from an MCP client.

### Claude Code integration

The easiest way to register the server is the built-in `register` subcommand:

```bash
# Register for a specific project (writes .mcp.json in the project directory)
./codeindex-mcp register project /path/to/project

# Register globally for all projects (writes ~/.claude.json)
./codeindex-mcp register user

# Forward extra flags to the server
./codeindex-mcp register project . -- --max-file-size 5242880 --exclude "vendor/"
```

Alternatively, add to your Claude Code MCP settings manually. For project-specific configuration, create `.mcp.json` in the project root:

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

For global configuration, add to `~/.claude.json`:

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

Claude Code will then automatically use `codeindex_search`, `codeindex_files`, `codeindex_read`, `codeindex_status`, and `codeindex_reindex` tools.

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--root DIR` | current directory | Project root directory to index |
| `--exclude PATTERN` | _(none)_ | Extra ignore pattern, repeatable (e.g. `--exclude "*.generated.go" --exclude "vendor/"`) |
| `--force-include PATTERN` | _(none)_ | Force-include pattern that overrides all excludes, repeatable (e.g. `--force-include "*.log"`) |
| `--max-file-size N` | `1048576` (1 MB) | Maximum file size in bytes; larger files are skipped |
| `--max-results N` | `50` | Default maximum number of search results |
| `--log-enabled` | `false` | Enable logging (no log file is created when disabled) |
| `--log-level LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--log-file PATH` | `<root>/codeindex-mcp.log` | Log file path |
| `--sync-interval N` | `300` | Periodic index sync verification interval in seconds (0 = disabled) |
| `--ast` | `false` | Enable AST symbol indexing (adds 5 `codeindex_ast_*` tools) |
| `--ast-languages LIST` | `go,typescript,python,javascript` | Languages to AST-index. In the lightweight build only `go` is available; `typescript`, `python`, `javascript` require `-tags ast` |
| `--ast-max-file-size-kb N` | `500` | Maximum file size in KB to AST-index |

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

# Enable debug logging to a specific file
./codeindex-mcp --root . --log-enabled --log-level debug --log-file /tmp/codeindex.log

# Disable periodic sync verification (enabled every 300s by default)
./codeindex-mcp --root . --sync-interval 0

# Allow larger files (5 MB)
./codeindex-mcp --root . --max-file-size 5242880

# Enable AST symbol indexing
./codeindex-mcp --root . --ast

# AST indexing for Go only (faster, smaller memory footprint)
./codeindex-mcp --root . --ast --ast-languages go
```

## MCP Tools

The server registers 5 tools:

### 1. `codeindex_search` — Content search

Grep-equivalent search across all indexed file contents, served from memory.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | yes | Search query (see formats below) |
| `filePath` | string | no | Exact relative path to search in a single file (overrides `fileGlob`) |
| `fileGlob` | string | no | Glob pattern to filter files (e.g. `**/*.go`) |
| `maxResults` | int | no | Maximum number of file results (default: 50) |
| `contextLines` | int | no | Context lines before/after each match (default: 2, `0` = matching lines only) |
| `maxMatchesPerFile` | int | no | Maximum matches rendered per file (default: 10); the real total is still reported |
| `outputMode` | string | no | `content` (default) = hunks with line numbers; `files` = paths only; `count` = paths with match counts |
| `caseSensitive` | bool | no | Case-sensitive matching for plain text and regex queries (default: false) |

**Query formats:**

| Format | Example | Behavior |
|--------|---------|----------|
| Plain text | `handleRequest` | Literal substring match, case-insensitive — full recall, `mutex` finds `sync.RWMutex` |
| `"quoted"` | `"func main"` | Exact literal match, case-sensitive |
| `/regex/` | `/func\s+\w+Handler/` | RE2 regular expression, matched per line, case-insensitive by default |

**Example output** (`content` mode — `N:` marks a match, `N-` marks context, `--` separates hunks):

```
3 matches in 2 files:
main.go
  4- import "fmt"
  5-
  6: func main() {
  7-     fmt.Println("hello world")
  --
  21- // entry helper
  22: func mainHelper() {
  23-     setup()
  ... +4 more matches

server/server.go
  13-
  14: func main() {
  15-     startServer()
```

`files` mode returns one path per line; `count` mode returns `path: matchCount` per line — both save tokens when you only need locations.

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
src/main.go (Go, 2.1 KB, 85L)
src/utils/helper.go (Go, 1.3 KB, 42L)
src/server/handler.go (Go, 4.7 KB, 156L)
src/config/config.go (Go, 892 B, 31L)
```

### 3. `codeindex_read` — Read file from index

Read a file's contents directly from the in-memory index. Zero disk I/O — faster than the built-in Read tool.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `filePath` | string | yes | Relative file path to read (e.g. `src/main.go`) |

**Example output:**

```
1: package main
2:
3: import "fmt"
4:
5: func main() {
6:     fmt.Println("hello")
7: }
```

### 4. `codeindex_status` — Index status

Display current index statistics.

**Parameters:** none

**Example output:**

```
root: /home/user/myproject
version: 0.6.0
uptime: 45s
files: 1234 (8.5 MB)
memory: 24.7 MB
watcher: 87 dirs
sync: every 300s
languages: TypeScript:456, Go:312, JavaScript:189, Python:98
```

### 5. `codeindex_reindex` — Force reindex

Clear the index and rebuild from scratch. Also reloads `.gitignore` and `.claudeignore` rules.

**Parameters:** none

**Example output:**

```
reindexed: 1234 files (8.5 MB) in 1.234s
```

## AST Tools (optional, `--ast` flag)

When started with `--ast`, the server registers 5 additional tools for structural code navigation.

| Build | Languages | How to build |
|-------|-----------|-------------|
| Lightweight (`make build`) | Go only — via `go/parser`, no CGo | `go build .` |
| Full AST (`make build-ast`) | Go + TypeScript + Python + JavaScript | `go build -tags ast .` |

Both builds expose the same 5 tools — the difference is which languages get indexed.

### 1. `codeindex_ast_search_symbols` — Search symbols by name

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | yes | Symbol name (case-insensitive substring match) |
| `kind` | string | no | Filter by kind: `class`, `interface`, `enum`, `function`, `method`, `field`, `variable`, `constant`, `type_alias` |
| `language` | string | no | Filter by language: `go`, `typescript`, `python`, `javascript` |
| `limit` | int | no | Max results (default: 20) |

**Example output:**
```
3 symbols matching "handler":
function  HandleRequest        server/handler.go:12  go
method    HandleError          server/handler.go:45  go
function  handleWatcherEvents  indexing.go:98        go
```

### 2. `codeindex_ast_file_symbols` — List symbols in a file

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `file` | string | yes | Relative file path |

**Example output:**
```
6 symbols in server/handler.go:
class     Server          line 8
method    Start           line 15   parent: Server
method    Stop            line 28   parent: Server
function  HandleRequest   line 45
function  handleError     line 67
variable  defaultTimeout  line 5
```

### 3. `codeindex_ast_find_usages` — Find files referencing a symbol

Searches for the symbol name as a text pattern across all indexed files. Returns files that likely reference it (not a full type-aware analysis).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `symbol` | string | yes | Symbol name to search for |
| `kind` | string | no | Optional kind hint (informational only) |

**Example output:**
```
5 files reference "HandleRequest":
server/handler.go
server/handler_test.go
main.go
tools/proxy.go
docs/api.md
```

### 4. `codeindex_ast_get_imports` — List imports for a file

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `file` | string | yes | Relative file path |

**Example output:**
```
4 imports in server/handler.go:
fmt
net/http
github.com/lexandro/codeindex-mcp/index
log/slog
```

### 5. `codeindex_ast_stats` — AST index statistics

**Parameters:** none

**Example output:**
```
AST index stats:
files:    142
symbols:  1847
by kind:  function:623 method:441 class:187 variable:312 constant:98 ...
by lang:  go:1203 typescript:489 python:155
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
| Env / secrets | `.env`, `.env.*`, `*.env` (use `--force-include` to index intentionally) |

### 2. `.gitignore` support

Fully respects `.gitignore` patterns **at every directory level** (nested `.gitignore` files included), with globs, negation (`!important.log`), and git's precedence rule: the ignore file closest to a path wins.

### 3. `.claudeignore` support

`.claudeignore` files use the same syntax and the same hierarchical matching as `.gitignore`. Use them to exclude files from the index that you want in git but are not relevant for AI code search.

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
                                ContentMap FileMap  Watcher
                               (line scan) (path)  (fsnotify)
```

### Dual index design

| Index | Technology | Purpose |
|-------|-----------|---------|
| **Content Index** | Go `map` of pre-split lines | Exact grep-style content search (substring / phrase / RE2 regex), scanned entirely in memory |
| **File Path Index** | Go `map` + sorted slice | File name/path search with glob patterns |

There is no tokenizer or inverted index in the search path: queries are compiled to a single RE2 matcher and scanned over the in-memory lines. This guarantees grep-identical results (no recall gaps for substrings inside identifiers) while staying far faster than disk-based grep.

### File watcher

- Uses **fsnotify** (on Windows: `ReadDirectoryChangesW` API)
- Recursive: watches all non-ignored subdirectories at startup
- **100ms debounce window**: editors generate multiple events on save — these are collapsed into one
- Newly created directory trees are watched **recursively** and files already inside them are indexed (covers copy / unzip / `git checkout` bursts)
- Automatically reloads ignore rules when any `.gitignore` or `.claudeignore` changes, then re-syncs the index against the new rules
- A periodic sync verification (default: every 300s) heals any missed events by diffing the index against disk

### Startup sequence

1. Parse CLI flags
2. Create ignore matcher (built-in + hierarchical .gitignore/.claudeignore + CLI patterns)
3. Initialize in-memory content index and file path index
4. Parallel indexing with 8 worker goroutines
5. Start file watcher and periodic sync verification
6. Start MCP server on stdio transport

## Project structure

```
codeindex-mcp/
├── main.go                  # Entry point, CLI flags, component wiring
├── indexing.go              # Directory walking, parallel indexing, watcher events
├── sync.go                  # Periodic background index sync verification
├── server/
│   └── server.go            # MCP server setup, tool registration
├── index/
│   ├── content.go           # In-memory content index (CRUD operations)
│   ├── content_search.go    # Line-scan search logic, query parsing, hunk building
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
├── register/
│   ├── register.go          # Auto-register subcommand for Claude Code config
│   └── register_test.go
├── tools/
│   ├── search.go            # codeindex_search handler
│   ├── files.go             # codeindex_files handler
│   ├── read.go              # codeindex_read handler
│   ├── status.go            # codeindex_status handler
│   ├── reindex.go           # codeindex_reindex handler
│   └── format.go            # Output formatting
├── ast/                     # AST symbol indexing (--ast flag)
│   ├── symbols.go           # Symbol, SymbolKind, SymbolTable
│   ├── extractor.go         # LanguageExtractor interface
│   ├── extractor_go.go      # Go extractor — go/parser, always compiled
│   ├── extractor_typescript.go  # TypeScript extractor — tree-sitter, build tag: ast
│   ├── extractor_python.go  # Python extractor — tree-sitter, build tag: ast
│   ├── extractor_javascript.go  # JavaScript extractor — tree-sitter, build tag: ast
│   ├── languages_noast.go   # Extractor registry, Go only (build tag: !ast)
│   ├── languages_ast.go     # Extractor registry, all 4 languages (build tag: ast)
│   ├── module.go            # Module wiring: OnFileChanged/Removed, RegisterTools
│   ├── tools.go             # 5 codeindex_ast_* MCP tool handlers
│   └── symbols_test.go
├── scripts/
│   ├── setup_build.sh       # Linux/macOS: full build environment setup
│   ├── setup_build.ps1      # Windows: full build environment setup
│   └── setup-gcc.ps1        # Windows: GCC-only install helper
├── Makefile                 # build / build-ast / test / test-ast / run / run-ast
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
| [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | v1.9.0 | File system watching |
| [bmatcuk/doublestar/v4](https://github.com/bmatcuk/doublestar) | v4.10.0 | `**` glob support |
| [denormal/go-gitignore](https://github.com/denormal/go-gitignore) | latest | .gitignore / .claudeignore parsing |
| [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) | v0.25.0 | CGo tree-sitter bindings (`-tags ast` only) |
| [tree-sitter/tree-sitter-typescript](https://github.com/tree-sitter/tree-sitter-typescript) | v0.23.2 | TypeScript grammar (`-tags ast` only) |
| [tree-sitter/tree-sitter-python](https://github.com/tree-sitter/tree-sitter-python) | v0.25.0 | Python grammar (`-tags ast` only) |
| [tree-sitter/tree-sitter-javascript](https://github.com/tree-sitter/tree-sitter-javascript) | v0.25.0 | JavaScript grammar (`-tags ast` only) |

## Performance

| Metric | ~5k files | ~10k files |
|--------|-----------|------------|
| Initial indexing | ~1s | ~1-2s |
| Memory usage | ≈ 2× total indexed file size | ≈ 2× total indexed file size |
| Text / regex search | <30ms | <60ms |
| Glob search | <2ms | <5ms |
| Incremental update | <5ms/file | <5ms/file |

Search is a linear in-memory scan compiled to a single RE2 matcher — no inverted index to build or keep consistent, so indexing is fast and memory stays close to the raw corpus size.

## Supported languages

Language detection recognizes 70+ file extensions, including:

Go, TypeScript, JavaScript, Python, Rust, Java, Kotlin, C, C++, C#, Swift, Dart, Ruby, PHP, Shell, PowerShell, HTML, CSS, SCSS, Sass, Less, JSON, YAML, TOML, XML, SQL, GraphQL, Protobuf, Terraform, Lua, R, Scala, Elixir, Erlang, Haskell, Zig, Vue, Svelte, Markdown, Dockerfile, Makefile, CMake, Batch, and more.

## License

[MIT](LICENSE)
