package register

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Run executes the register subcommand.
// serverName is the MCP server name (e.g. "codeindex").
// args is os.Args[2:] (everything after "register").
func Run(serverName string, args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	scope := args[0]
	if scope != "project" && scope != "user" {
		fmt.Fprintf(os.Stderr, "Error: unknown scope %q (must be \"project\" or \"user\")\n", scope)
		printUsage()
		os.Exit(1)
	}

	var directory string
	var serverArgs []string

	if scope == "project" {
		directory, serverArgs = parseProjectArgs(args[1:])
	} else {
		serverArgs = parseUserArgs(args[1:])
	}

	binaryPath, err := detectBinaryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting binary path: %v\n", err)
		os.Exit(1)
	}

	configPath, err := resolveConfigPath(scope, directory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving config path: %v\n", err)
		os.Exit(1)
	}

	entry := buildEntry(binaryPath, serverArgs)

	if err := writeConfig(configPath, serverName, entry); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Registered %q in %s\n", serverName, configPath)
}

func printUsage() {
	binaryName := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s register project [directory]  # → <directory>/.mcp.json (default: .)\n", binaryName)
	fmt.Fprintf(os.Stderr, "  %s register user                 # → ~/.claude.json\n", binaryName)
	fmt.Fprintf(os.Stderr, "  %s register project . -- --flag  # forward args to server\n", binaryName)
	fmt.Fprintf(os.Stderr, "  %s register user -- --flag       # forward args to server\n", binaryName)
}

// DeriveServerName extracts a server name from a binary path by stripping .exe and -mcp suffixes.
func DeriveServerName(binaryPath string) string {
	name := filepath.Base(binaryPath)
	name = strings.TrimSuffix(name, ".exe")
	name = strings.TrimSuffix(name, "-mcp")
	return name
}

func parseProjectArgs(args []string) (directory string, serverArgs []string) {
	directory = "."
	for i, arg := range args {
		if arg == "--" {
			serverArgs = args[i+1:]
			return directory, serverArgs
		}
		// First non-separator arg is the directory
		if i == 0 {
			directory = arg
		}
	}
	return directory, nil
}

func parseUserArgs(args []string) (serverArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}

func detectBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("getting executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks for %s: %w", exe, err)
	}
	return resolved, nil
}

func resolveConfigPath(scope string, directory string) (string, error) {
	if scope == "project" {
		absDir, err := filepath.Abs(directory)
		if err != nil {
			return "", fmt.Errorf("resolving directory %s: %w", directory, err)
		}
		return filepath.Join(absDir, ".mcp.json"), nil
	}
	// user scope
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(homeDir, ".claude.json"), nil
}

func buildEntry(binaryPath string, serverArgs []string) mcpServerEntry {
	if runtime.GOOS == "windows" {
		args := []string{"/C", binaryPath}
		args = append(args, serverArgs...)
		return mcpServerEntry{
			Command: "cmd",
			Args:    args,
		}
	}
	return mcpServerEntry{
		Command: binaryPath,
		Args:    serverArgs,
	}
}

func writeConfig(configPath string, serverName string, entry mcpServerEntry) error {
	// Read existing config or start fresh
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{},
	}

	data, err := os.ReadFile(configPath)
	if err == nil {
		// File exists, parse it
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing existing config %s: %w", configPath, err)
		}
	}

	// Ensure mcpServers key exists
	servers, ok := config["mcpServers"]
	if !ok {
		servers = map[string]interface{}{}
		config["mcpServers"] = servers
	}

	serversMap, ok := servers.(map[string]interface{})
	if !ok {
		return fmt.Errorf("mcpServers in %s is not an object", configPath)
	}

	// Add/update the server entry
	serversMap[serverName] = entry

	// Write back
	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	output = append(output, '\n')

	// Atomic write: write to temp file in same directory, then rename
	configDir := filepath.Dir(configPath)
	tmpFile, err := os.CreateTemp(configDir, ".mcp-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", configDir, err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(output); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming %s to %s: %w", tmpPath, configPath, err)
	}

	return nil
}
