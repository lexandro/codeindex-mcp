# Development Prompt: `register` subcommand for MCP servers

Reusable prompt for adding a `register` subcommand to any Go MCP server project. This command auto-registers the server in Claude Code's config files, replacing manual JSON editing.

**To use in another project:** Copy this file and change `SERVER_NAME` (see "Customization" section at the bottom).

## What it does

The `register` subcommand writes the MCP server entry into Claude Code's JSON config files:
- **Project scope:** `<directory>/.mcp.json` — for project-specific MCP servers
- **User scope:** `~/.claude.json` — for globally available MCP servers

## Command syntax

```
<binary> register project [directory]                          # → <directory>/.mcp.json (default: .)
<binary> register user                                         # → ~/.claude.json
<binary> register project . -- --base-url http://localhost:8080 # forward args to server
<binary> register user -- --timeout 60s                        # forward args to server
```

## Config file format

Both `.mcp.json` and `~/.claude.json` use the same structure:

```json
{
  "mcpServers": {
    "server-name": {
      "command": "/path/to/binary",
      "args": ["--flag1", "value1"]
    }
  }
}
```

On Windows, the command wraps through `cmd /C`:
```json
{
  "mcpServers": {
    "server-name": {
      "command": "cmd",
      "args": ["/C", "C:\\path\\to\\binary.exe", "--flag1", "value1"]
    }
  }
}
```

## Architecture

```
register/register.go       Register subcommand implementation (~120 lines)
register/register_test.go  Tests (~100 lines)
main.go                    +5 lines subcommand dispatch
```

## Implementation: register/register.go

### Package and exports

```go
package register

// ServerInfo holds the identity of the MCP server being registered.
type ServerInfo struct {
    Name string // e.g. "rest-api" (derived from binary name, without -mcp suffix)
}

// Run executes the register subcommand.
// args is os.Args[2:] (everything after "register").
func Run(info ServerInfo, args []string)
```

### Run() logic

1. Parse scope from `args[0]` — must be `"project"` or `"user"`
2. Parse remaining args:
   - **project scope:** optional directory (default `.`), then `--` separator, then server args
   - **user scope:** `--` separator, then server args
3. Detect binary path via `os.Executable()` + `filepath.EvalSymlinks()`
4. Resolve config file path:
   - project → `filepath.Abs(directory)` + `/.mcp.json`
   - user → `os.UserHomeDir()` + `/.claude.json`
5. Build server entry: `{command, args}` (on Windows: wrap with `cmd /C`)
6. Read existing config JSON or start with `{"mcpServers": {}}`
7. Add/update `mcpServers[name]` with the new entry
8. Write back with `json.MarshalIndent` (2-space indent)
9. Print: `Registered "server-name" in /path/to/config.json`

### Server name derivation

```go
func DeriveServerName(binaryPath string) string {
    name := filepath.Base(binaryPath)
    name = strings.TrimSuffix(name, ".exe")
    name = strings.TrimSuffix(name, "-mcp")
    return name
}
```

Examples:
- `rest-api-mcp` → `rest-api`
- `rest-api-mcp.exe` → `rest-api`
- `codeindex-mcp` → `codeindex`

### Edge cases

| Scenario | Behavior |
|----------|----------|
| Config file doesn't exist | Create new file with `{"mcpServers": {}}` |
| Server entry already exists | Overwrite with new command/args |
| Other entries in config | Preserved (read-modify-write) |
| Invalid JSON in existing file | Return error, don't overwrite |
| No `--` separator | Empty server args |
| Missing scope argument | Print usage and exit |

### Internal functions

```go
func parseProjectArgs(args []string) (directory string, serverArgs []string)
func parseUserArgs(args []string) (serverArgs []string)
func detectBinaryPath() (string, error)
func resolveConfigPath(scope string, directory string) (string, error)
func buildEntry(binaryPath string, serverArgs []string) mcpServerEntry
func writeConfig(configPath string, serverName string, entry mcpServerEntry) error
```

## Implementation: main.go dispatch

Add before `flag.Parse()`:

```go
import "github.com/yourmodule/register"

func main() {
    if len(os.Args) > 1 && os.Args[1] == "register" {
        register.Run(register.ServerInfo{Name: "SERVER_NAME"}, os.Args[2:])
        return
    }
    // ... existing flag parsing and server startup
}
```

## Implementation: register/register_test.go

Table-driven tests for:

### Test_DeriveServerName
- Strip `-mcp` suffix
- Strip `.exe` and `-mcp`
- No `-mcp` suffix (passthrough)
- Only `.exe` suffix

### Test_parseProjectArgs
- No args → dir=`.`, args=nil
- Directory only → dir=`<dir>`, args=nil
- Directory + server args → dir + args after `--`
- Just `--` + args → dir=`.`, args after `--`

### Test_parseUserArgs
- No args → nil
- With `--` + args → args after `--`

### Test_writeConfig_CreatesNewFile
- Write to non-existent path → creates valid JSON with mcpServers entry

### Test_writeConfig_UpdatesExistingEntry
- Existing config with other entries → updated entry, others preserved

### Test_writeConfig_InvalidJSON
- Existing file with invalid JSON → returns error

### Test_buildEntry
- Verify command/args structure
- On Windows: command=`cmd`, args starts with `/C`

### Test_resolveConfigPath_Project
- Returns `<absDir>/.mcp.json`

### Test_resolveConfigPath_User
- Returns `~/.claude.json`

## Verification checklist

```bash
go build -o <binary>.exe .
go vet ./...
go test ./...

# Manual tests
./<binary>.exe register project .
# → check .mcp.json was created/updated

./<binary>.exe register user
# → check ~/.claude.json was created/updated

./<binary>.exe register project . -- --base-url http://localhost:8080
# → check .mcp.json has args

# Verify MCP server still works
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./<binary>.exe
```

## Customization

To use this prompt in another MCP server project:

1. Copy `register/register.go` and `register/register_test.go` to your project
2. Update the module import path in `main.go`
3. Change the server name in `main.go`:
   ```go
   register.Run(register.ServerInfo{Name: "YOUR_SERVER_NAME"}, os.Args[2:])
   ```
   Replace `YOUR_SERVER_NAME` with the desired name (e.g., `"codeindex"`, `"db-query"`)
4. The name should match what users expect to see in their Claude Code config
