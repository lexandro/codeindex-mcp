package register

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func Test_DeriveServerName(t *testing.T) {
	tests := []struct {
		name       string
		binaryPath string
		want       string
	}{
		{"strip -mcp suffix", "rest-api-mcp", "rest-api"},
		{"strip .exe and -mcp", "rest-api-mcp.exe", "rest-api"},
		{"no -mcp suffix passthrough", "myserver", "myserver"},
		{"only .exe suffix", "myserver.exe", "myserver"},
		{"codeindex-mcp", "codeindex-mcp", "codeindex"},
		{"codeindex-mcp.exe", "codeindex-mcp.exe", "codeindex"},
		{"full path stripped to base", "/usr/local/bin/rest-api-mcp", "rest-api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveServerName(tt.binaryPath)
			if got != tt.want {
				t.Errorf("DeriveServerName(%q) = %q, want %q", tt.binaryPath, got, tt.want)
			}
		})
	}
}

func Test_parseProjectArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDir  string
		wantArgs []string
	}{
		{"no args", nil, ".", nil},
		{"directory only", []string{"mydir"}, "mydir", nil},
		{"directory and server args", []string{"mydir", "--", "--root", "/tmp"}, "mydir", []string{"--root", "/tmp"}},
		{"just separator and args", []string{"--", "--root", "/tmp"}, ".", []string{"--root", "/tmp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotArgs := parseProjectArgs(tt.args)
			if gotDir != tt.wantDir {
				t.Errorf("parseProjectArgs() dir = %q, want %q", gotDir, tt.wantDir)
			}
			if !sliceEqual(gotArgs, tt.wantArgs) {
				t.Errorf("parseProjectArgs() args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func Test_parseUserArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantArgs []string
	}{
		{"no args", nil, nil},
		{"with separator and args", []string{"--", "--timeout", "60s"}, []string{"--timeout", "60s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs := parseUserArgs(tt.args)
			if !sliceEqual(gotArgs, tt.wantArgs) {
				t.Errorf("parseUserArgs() = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func Test_writeConfig_CreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".mcp.json")

	entry := mcpServerEntry{Command: "/usr/bin/myserver", Args: []string{"--root", "/tmp"}}
	if err := writeConfig(configPath, "myserver", entry); err != nil {
		t.Fatalf("writeConfig() error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	servers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers not found or not an object")
	}

	serverEntry, ok := servers["myserver"].(map[string]interface{})
	if !ok {
		t.Fatal("myserver entry not found or not an object")
	}

	if serverEntry["command"] != "/usr/bin/myserver" {
		t.Errorf("command = %v, want /usr/bin/myserver", serverEntry["command"])
	}
}

func Test_writeConfig_UpdatesExistingEntry(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".mcp.json")

	// Write initial config with two entries
	initial := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"command": "/usr/bin/other",
			},
			"myserver": map[string]interface{}{
				"command": "/old/path",
			},
		},
	}
	initialData, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(configPath, initialData, 0644)

	// Update myserver entry
	entry := mcpServerEntry{Command: "/new/path", Args: []string{"--flag"}}
	if err := writeConfig(configPath, "myserver", entry); err != nil {
		t.Fatalf("writeConfig() error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	servers := config["mcpServers"].(map[string]interface{})

	// Other entry preserved
	otherEntry := servers["other-server"].(map[string]interface{})
	if otherEntry["command"] != "/usr/bin/other" {
		t.Errorf("other-server command changed unexpectedly: %v", otherEntry["command"])
	}

	// Updated entry
	myEntry := servers["myserver"].(map[string]interface{})
	if myEntry["command"] != "/new/path" {
		t.Errorf("myserver command = %v, want /new/path", myEntry["command"])
	}
}

func Test_writeConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".mcp.json")

	os.WriteFile(configPath, []byte("not valid json{{{"), 0644)

	entry := mcpServerEntry{Command: "/usr/bin/myserver"}
	err := writeConfig(configPath, "myserver", entry)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func Test_buildEntry(t *testing.T) {
	binaryPath := "/usr/local/bin/codeindex-mcp"
	serverArgs := []string{"--root", "/projects"}

	entry := buildEntry(binaryPath, serverArgs)

	if runtime.GOOS == "windows" {
		if entry.Command != "cmd" {
			t.Errorf("command = %q, want \"cmd\"", entry.Command)
		}
		if len(entry.Args) < 2 || entry.Args[0] != "/C" || entry.Args[1] != binaryPath {
			t.Errorf("args = %v, want [/C %s --root /projects]", entry.Args, binaryPath)
		}
	} else {
		if entry.Command != binaryPath {
			t.Errorf("command = %q, want %q", entry.Command, binaryPath)
		}
		if !sliceEqual(entry.Args, serverArgs) {
			t.Errorf("args = %v, want %v", entry.Args, serverArgs)
		}
	}
}

func Test_buildEntry_NoArgs(t *testing.T) {
	binaryPath := "/usr/local/bin/codeindex-mcp"

	entry := buildEntry(binaryPath, nil)

	if runtime.GOOS == "windows" {
		if entry.Command != "cmd" {
			t.Errorf("command = %q, want \"cmd\"", entry.Command)
		}
		if len(entry.Args) != 2 || entry.Args[0] != "/C" || entry.Args[1] != binaryPath {
			t.Errorf("args = %v, want [/C %s]", entry.Args, binaryPath)
		}
	} else {
		if entry.Command != binaryPath {
			t.Errorf("command = %q, want %q", entry.Command, binaryPath)
		}
		if entry.Args != nil {
			t.Errorf("args = %v, want nil", entry.Args)
		}
	}
}

func Test_resolveConfigPath_Project(t *testing.T) {
	got, err := resolveConfigPath("project", ".")
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	absDir, _ := filepath.Abs(".")
	want := filepath.Join(absDir, ".mcp.json")
	if got != want {
		t.Errorf("resolveConfigPath(project, .) = %q, want %q", got, want)
	}
}

func Test_resolveConfigPath_User(t *testing.T) {
	got, err := resolveConfigPath("user", "")
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	homeDir, _ := os.UserHomeDir()
	want := filepath.Join(homeDir, ".claude.json")
	if got != want {
		t.Errorf("resolveConfigPath(user, ) = %q, want %q", got, want)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
