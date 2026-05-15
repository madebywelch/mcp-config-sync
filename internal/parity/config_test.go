package parity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParityConfigAndFilterDeniedServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`deny_servers = ["private-only"]`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, diagnostics, err := LoadParityConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if len(config.DenyServers) != 1 || config.DenyServers[0] != "private-only" {
		t.Fatalf("deny servers = %#v", config.DenyServers)
	}

	servers := []ClaudeServer{
		{Name: "private-only", Config: map[string]any{"command": "npx"}},
		{Name: "sync-me", Config: map[string]any{"command": "npx"}},
	}
	filtered, filterDiagnostics := FilterDeniedServers(servers, config)
	if len(filtered) != 1 || filtered[0].Name != "sync-me" {
		t.Fatalf("filtered = %#v", filtered)
	}
	if len(filterDiagnostics) != 1 {
		t.Fatalf("filter diagnostics = %#v", filterDiagnostics)
	}
}
