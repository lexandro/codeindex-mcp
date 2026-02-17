# codeindex-mcp

In-memory MCP server for source code indexing. Replaces grep/find with fast indexed search.

## Tech Stack
- Language: Go 1.22+
- MCP SDK: github.com/modelcontextprotocol/go-sdk
- Search: github.com/blevesearch/bleve/v2 (NewMemOnly)
- File watching: github.com/fsnotify/fsnotify
- Glob: github.com/bmatcuk/doublestar/v4
- Gitignore: github.com/denormal/go-gitignore

## Build & Test
- Build: `go build -o codeindex-mcp.exe .`
- Test all: `go test ./...`
- Test one package: `go test ./index/...`
- Run: `./codeindex-mcp.exe --root <project-dir>`

## Architecture
- `main.go` - Entry point, CLI flag parsing, component wiring
- `indexing.go` - Directory walking, parallel file indexing, watcher event handling
- `server/` - MCP server setup, tool registration (stdio transport)
- `index/` - Dual index: Bleve content index + file path index
- `watcher/` - Recursive fsnotify wrapper with debouncing
- `ignore/` - .gitignore + .claudeignore + default + custom ignore patterns + force-include overrides
- `tools/` - MCP tool handlers (search, files, status, reindex, read)
- `register/` - `register` subcommand: auto-registers server in Claude Code config files
- `language/` - File extension to language mapping, binary detection

## AI-Optimized Coding Principles

These principles override traditional SOLID/Clean Code/DRY when they conflict.
The goal: any AI agent can read, understand, and correctly modify any file in isolation.

### 1. Explicit Over Implicit (overrides DRY)
- NO init() functions - all initialization in explicit function calls from main.go
- NO interface-based dependency injection unless there are 2+ real implementations
- NO reflection, struct tags only for JSON/schema serialization
- Duplicate 3-5 lines rather than create a shared helper that obscures the logic flow
- Every function signature tells its full story - no hidden state, no package-level vars mutated as side effects

### 2. Flat Over Deep (overrides SOLID's abstraction layers)
- Maximum 1 level of function call depth for business logic (handler -> index operation)
- No "manager calls service calls repository calls adapter" chains
- Each tool handler in tools/ directly calls index/ methods - no intermediate layers
- Prefer switch statements over polymorphism when there are <5 cases
- No abstract factory, strategy pattern, or visitor pattern - use plain functions

### 3. Co-located Over Separated (overrides separation of concerns when it hurts readability)
- Each file is self-contained: reading one file gives the full picture of one feature
- Types used only in one file are defined in that file, not in a separate types.go
- Error types specific to a package are defined in the file that returns them
- Test files mirror source files 1:1 (content.go -> content_test.go)

### 4. Predictable Patterns (the most important principle)
- Every MCP tool handler follows the exact same structure:
  1. Parse input struct
  2. Validate parameters
  3. Call index method
  4. Format text output
  5. Return MCP result
- Every index method follows: acquire RLock -> query -> release RLock -> return
- Every write operation follows: acquire Lock -> mutate -> release Lock -> return
- Consistent error handling: wrap with context, return early, never panic

### 5. Small Context Window Files (overrides "one class per file" dogma)
- Target: each .go file under 300 lines
- If a file grows beyond 300 lines, split by FUNCTIONALITY not by type
- Good split: search_text.go, search_regex.go, search_phrase.go
- Bad split: search_types.go, search_interfaces.go, search_impl.go

### 6. Self-Documenting Names (overrides brevity)
- Functions: verb + noun, describe what they do: `IndexFileContent()`, `SearchByRegex()`, `RebuildPathList()`
- Variables: full words, no abbreviations: `fileContent` not `fc`, `matchCount` not `mc`
- Constants: SCREAMING_SNAKE for true constants, descriptive: `MaxFileSizeBytes`, `DebounceIntervalMs`
- Package names: single word, lowercase, obvious: `index`, `watcher`, `ignore`, `tools`

### 7. Error Handling (explicit, not clever)
- Always return errors, never log-and-continue silently
- Wrap errors with context: `fmt.Errorf("indexing file %s: %w", path, err)`
- No custom error types unless the caller needs to match on error type
- Log at the boundary (main.go, tool handlers), not deep in library code

### 8. Concurrency (explicit, simple patterns only)
- sync.RWMutex for index access (readers don't block each other)
- Goroutines only in: startup indexing (bounded worker pool), file watcher loop
- Channel usage limited to: watcher events, debouncer output
- No complex channel orchestration - use sync.WaitGroup for fan-out/fan-in

### 9. Testing (pragmatic, not dogmatic)
- Test public API of each package, not internal functions
- Table-driven tests for input/output variations
- Test files use testdata/ subdirectory for fixture files
- No mocks unless testing against external I/O (filesystem) - use real Bleve in-memory index in tests
- Test names: Test_FunctionName_Scenario (e.g., Test_SearchContent_RegexQuery)

### 10. When Traditional Principles DO Apply
- DRY: Apply for true business logic duplication (same algorithm in 2+ places)
- KISS: Always - never add complexity without immediate need
- Single Responsibility: Each PACKAGE has one responsibility; within a package, files can do related things
- Interface Segregation: MCP tool input/output types should be minimal
- Open/Closed: Index engine can support new query types without modifying existing ones
