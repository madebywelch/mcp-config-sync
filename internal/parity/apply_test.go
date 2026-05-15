package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPlanAppendsAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := "model = \"gpt-5.5\"\n\n[tui]\nstatus_line = [\"model\"]\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Adds: []PlannedAdd{{
		Name:      "new-server",
		Transport: "http",
		Config: map[string]any{
			"url": "https://example.com/mcp",
		},
	}}}

	result, err := ApplyPlan(path, plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Fatalf("added = %d", result.Added)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup path")
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup changed: %q", string(backup))
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if strings.Contains(text, "[mcp_servers]\n") {
		t.Fatalf("unexpected parent mcp_servers table:\n%s", text)
	}
	if !strings.Contains(text, "[mcp_servers.new-server]") {
		t.Fatalf("missing server block:\n%s", text)
	}
	if !strings.HasPrefix(text, original[:len(original)-1]) {
		t.Fatalf("existing content not preserved:\n%s", text)
	}
}

func TestApplyTargetedPlanWritesSeparateTargets(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project", ".codex", "config.toml")
	plan := Plan{Adds: []PlannedAdd{
		{
			Name:      "user-server",
			Target:    userPath,
			Transport: "http",
			Config:    map[string]any{"url": "https://user.example/mcp"},
		},
		{
			Name:      "project-server",
			Target:    projectPath,
			Transport: "stdio",
			Config:    map[string]any{"command": "npx", "args": []string{"project-mcp"}},
		},
	}}

	results, err := ApplyTargetedPlan(plan, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	userContent, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatal(err)
	}
	projectContent, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(userContent), "[mcp_servers.user-server]") {
		t.Fatalf("missing user server:\n%s", string(userContent))
	}
	if !strings.Contains(string(projectContent), "[mcp_servers.project-server]") {
		t.Fatalf("missing project server:\n%s", string(projectContent))
	}
}
