package parity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClaudeServersPrecedence(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	claudeConfig := filepath.Join(dir, ".claude.json")
	writeFile(t, claudeConfig, `{
	  "mcpServers": {
	    "shared": {"command": "user-command"},
	    "user-only": {"url": "https://user.example/mcp"}
	  },
	  "projects": {
	    "`+jsonPath(project)+`": {
	      "mcpServers": {
	        "shared": {"command": "local-command"},
	        "local-only": {"command": "local"}
	      },
	      "disabledMcpjsonServers": ["project-disabled"]
	    }
	  }
	}`)
	writeFile(t, filepath.Join(project, ".mcp.json"), `{
	  "mcpServers": {
	    "shared": {"command": "project-command"},
	    "project-only": {"command": "project"},
	    "project-disabled": {"command": "skip"}
	  }
	}`)

	servers, diagnostics, err := LoadClaudeServers(ClaudeLoadOptions{
		ConfigPath:          claudeConfig,
		ProjectPath:         project,
		IncludeUser:         true,
		IncludeLocal:        true,
		IncludeProjectFile:  true,
		DedupeByPrecedence:  true,
		RespectDisabledSets: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("expected duplicate/disabled diagnostics")
	}
	got := names(servers)
	want := []string{"local-only", "shared", "project-only", "user-only"}
	if !equalStrings(got, want) {
		t.Fatalf("names = %#v, want %#v", got, want)
	}
	if servers[1].Config["command"] != "local-command" {
		t.Fatalf("shared did not use local precedence: %#v", servers[1].Config)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func jsonPath(path string) string {
	bytes := []byte(path)
	out := ""
	for _, b := range bytes {
		if b == '\\' || b == '"' {
			out += "\\"
		}
		out += string(b)
	}
	return out
}

func names(servers []ClaudeServer) []string {
	result := make([]string, 0, len(servers))
	for _, server := range servers {
		result = append(result, server.Name)
	}
	return result
}

func equalStrings(a, b []string) bool {
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
