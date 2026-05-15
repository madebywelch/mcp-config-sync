package parity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyClaudePlanAddsUserAndLocalServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	project := filepath.Join(dir, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	original := `{
  "mcpServers": {
    "existing-user": {
      "command": "npx"
    }
  },
  "projects": {
    "` + jsonPath(project) + `": {
      "mcpServers": {
        "existing-local": {
          "command": "npx"
        }
      }
    }
  }
}
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	adds := []PlannedAdd{
		{
			Name:       "new-user",
			Target:     path,
			TargetKind: TargetClaudeUser,
			Config:     map[string]any{"type": "http", "url": "https://example.com/mcp"},
		},
		{
			Name:          "new-local",
			Target:        path,
			TargetKind:    TargetClaudeLocal,
			TargetProject: project,
			Config:        map[string]any{"type": "stdio", "command": "npx"},
		},
	}

	result, err := ApplyClaudePlan(path, adds, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 2 {
		t.Fatalf("added = %d", result.Added)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{"existing-user", "existing-local", "new-user", "new-local"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestApplyClaudePlanAddsProjectFileServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")
	adds := []PlannedAdd{{
		Name:       "project-server",
		Target:     path,
		TargetKind: TargetClaudeProject,
		Config:     map[string]any{"type": "stdio", "command": "npx"},
	}}

	result, err := ApplyClaudePlan(path, adds, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Fatalf("added = %d", result.Added)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"project-server"`) {
		t.Fatalf("missing project server:\n%s", string(content))
	}
}
