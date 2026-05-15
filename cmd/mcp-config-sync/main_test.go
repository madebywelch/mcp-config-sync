package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/madebywelch/mcp-config-sync/internal/parity"
)

func TestStagedClaudeToCodexSync(t *testing.T) {
	stage := newStage(t)
	writeFile(t, stage.claudeConfig, `{
  "mcpServers": {
    "shared-existing": {
      "type": "stdio",
      "command": "npx",
      "args": ["shared-mcp"]
    },
    "user-http": {
      "type": "http",
      "url": "https://user.example/mcp",
      "headers": {
        "Authorization": "Bearer ${USER_TOKEN}",
        "X-Static": "yes"
      }
    },
    "denied-server": {
      "type": "http",
      "url": "https://denied.example/mcp"
    }
  },
  "projects": {
    "`+jsonPath(stage.project)+`": {
      "mcpServers": {
        "local-stdio": {
          "type": "stdio",
          "command": "npx",
          "args": ["local-mcp"]
        }
      },
      "disabledMcpjsonServers": ["disabled-project"]
    }
  }
}
`)
	writeFile(t, filepath.Join(stage.project, ".mcp.json"), `{
  "mcpServers": {
    "project-http": {
      "type": "http",
      "url": "https://project.example/mcp"
    },
    "disabled-project": {
      "type": "stdio",
      "command": "disabled"
    }
  }
}
`)
	writeFile(t, stage.codexConfig, `[mcp_servers.shared-existing]
command = "npx"
args = ["shared-mcp"]
`)
	writeFile(t, stage.codexProjectConfig, `[mcp_servers.existing-project]
url = "https://existing.example/mcp"
`)
	writeFile(t, stage.parityConfig, `deny_servers = ["denied-server"]`)

	runOK(t, "sync",
		"--direction", "claude-to-codex",
		"--project", stage.project,
		"--claude-config", stage.claudeConfig,
		"--codex-config", stage.codexConfig,
		"--codex-project-config", stage.codexProjectConfig,
		"--parity-config", stage.parityConfig,
		"--no-backup",
	)

	userCodex := loadCodex(t, stage.codexConfig)
	projectCodex := loadCodex(t, stage.codexProjectConfig)
	assertCodexHas(t, userCodex, "shared-existing", "user-http")
	assertCodexMissing(t, userCodex, "denied-server")
	if got := userCodex.Servers["user-http"]["bearer_token_env_var"]; got != "USER_TOKEN" {
		t.Fatalf("bearer_token_env_var = %v", got)
	}
	assertCodexHas(t, projectCodex, "existing-project", "local-stdio", "project-http")
	assertCodexMissing(t, projectCodex, "disabled-project")

	runOK(t, "verify",
		"--direction", "claude-to-codex",
		"--project", stage.project,
		"--claude-config", stage.claudeConfig,
		"--codex-config", stage.codexConfig,
		"--codex-project-config", stage.codexProjectConfig,
		"--parity-config", stage.parityConfig,
	)

	beforeUser := readFile(t, stage.codexConfig)
	beforeProject := readFile(t, stage.codexProjectConfig)
	runOK(t, "sync",
		"--direction", "claude-to-codex",
		"--project", stage.project,
		"--claude-config", stage.claudeConfig,
		"--codex-config", stage.codexConfig,
		"--codex-project-config", stage.codexProjectConfig,
		"--parity-config", stage.parityConfig,
		"--no-backup",
	)
	assertEqual(t, readFile(t, stage.codexConfig), beforeUser)
	assertEqual(t, readFile(t, stage.codexProjectConfig), beforeProject)
}

func TestStagedCodexToClaudeSyncProjectFile(t *testing.T) {
	stage := newStage(t)
	writeFile(t, stage.claudeConfig, `{
  "mcpServers": {
    "shared-existing": {
      "type": "stdio",
      "command": "npx"
    }
  }
}
`)
	writeFile(t, filepath.Join(stage.project, ".mcp.json"), `{
  "mcpServers": {
    "existing-project": {
      "type": "http",
      "url": "https://existing.example/mcp"
    }
  }
}
`)
	writeFile(t, stage.codexConfig, `[mcp_servers.shared-existing]
command = "npx"

[mcp_servers.codex-http]
url = "https://codex.example/mcp"
bearer_token_env_var = "CODEX_TOKEN"

[mcp_servers.denied-server]
url = "https://denied.example/mcp"
`)
	writeFile(t, stage.codexProjectConfig, `[mcp_servers.existing-project]
url = "https://existing.example/mcp"

[mcp_servers.project-stdio]
command = "npx"
args = ["project-mcp"]

[mcp_servers.project-stdio.env]
PROJECT_TOKEN = "secret"
`)
	writeFile(t, stage.parityConfig, `deny_servers = ["denied-server"]`)

	runOK(t, "sync",
		"--direction", "codex-to-claude",
		"--project", stage.project,
		"--claude-config", stage.claudeConfig,
		"--codex-config", stage.codexConfig,
		"--codex-project-config", stage.codexProjectConfig,
		"--parity-config", stage.parityConfig,
		"--no-backup",
	)

	claudeRoot := loadJSON(t, stage.claudeConfig)
	userServers := objectAt(t, claudeRoot, "mcpServers")
	assertJSONHas(t, userServers, "shared-existing", "codex-http")
	assertJSONMissing(t, userServers, "denied-server")
	headers := objectAt(t, objectAt(t, userServers, "codex-http"), "headers")
	if got := headers["Authorization"]; got != "Bearer ${CODEX_TOKEN}" {
		t.Fatalf("Authorization header = %v", got)
	}

	projectRoot := loadJSON(t, filepath.Join(stage.project, ".mcp.json"))
	projectServers := objectAt(t, projectRoot, "mcpServers")
	assertJSONHas(t, projectServers, "existing-project", "project-stdio")
	env := objectAt(t, objectAt(t, projectServers, "project-stdio"), "env")
	if got := env["PROJECT_TOKEN"]; got != "secret" {
		t.Fatalf("PROJECT_TOKEN = %v", got)
	}

	runOK(t, "verify",
		"--direction", "codex-to-claude",
		"--project", stage.project,
		"--claude-config", stage.claudeConfig,
		"--codex-config", stage.codexConfig,
		"--codex-project-config", stage.codexProjectConfig,
		"--parity-config", stage.parityConfig,
	)
}

func TestStagedCodexToClaudeSyncLocalProjectTarget(t *testing.T) {
	stage := newStage(t)
	writeFile(t, stage.claudeConfig, `{
  "projects": {
    "`+jsonPath(stage.project)+`": {
      "mcpServers": {
        "existing-local": {
          "type": "stdio",
          "command": "npx"
        }
      }
    }
  }
}
`)
	writeFile(t, stage.codexConfig, ``)
	writeFile(t, stage.codexProjectConfig, `[mcp_servers.project-stdio]
command = "npx"
args = ["project-mcp"]
`)
	writeFile(t, stage.parityConfig, `deny_servers = []`)

	runOK(t, "sync",
		"--direction", "codex-to-claude",
		"--target-scope", "project",
		"--claude-project-target", "local",
		"--include-user=false",
		"--project", stage.project,
		"--claude-config", stage.claudeConfig,
		"--codex-config", stage.codexConfig,
		"--codex-project-config", stage.codexProjectConfig,
		"--parity-config", stage.parityConfig,
		"--no-backup",
	)

	claudeRoot := loadJSON(t, stage.claudeConfig)
	projectServers := objectAt(t, objectAt(t, objectAt(t, claudeRoot, "projects"), stage.project), "mcpServers")
	assertJSONHas(t, projectServers, "existing-local", "project-stdio")
	if _, err := os.Stat(filepath.Join(stage.project, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("expected project .mcp.json to remain absent, stat err = %v", err)
	}
}

type stagedPaths struct {
	root               string
	project            string
	claudeConfig       string
	codexConfig        string
	codexProjectConfig string
	parityConfig       string
}

func newStage(t *testing.T) stagedPaths {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	return stagedPaths{
		root:               root,
		project:            project,
		claudeConfig:       filepath.Join(root, ".claude.json"),
		codexConfig:        filepath.Join(root, ".codex", "config.toml"),
		codexProjectConfig: filepath.Join(project, ".codex", "config.toml"),
		parityConfig:       filepath.Join(root, "parity.toml"),
	}
}

func runOK(t *testing.T, args ...string) {
	t.Helper()
	if err := run(args); err != nil {
		t.Fatalf("run(%v): %v", args, err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func loadCodex(t *testing.T, path string) parity.CodexConfig {
	t.Helper()
	config, err := parity.LoadCodexConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func loadJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &root); err != nil {
		t.Fatal(err)
	}
	return root
}

func objectAt(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%q is not an object in %#v", key, parent)
	}
	return value
}

func assertCodexHas(t *testing.T, config parity.CodexConfig, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, exists := config.Servers[name]; !exists {
			t.Fatalf("expected Codex server %q in %#v", name, config.Servers)
		}
	}
}

func assertCodexMissing(t *testing.T, config parity.CodexConfig, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, exists := config.Servers[name]; exists {
			t.Fatalf("unexpected Codex server %q in %#v", name, config.Servers)
		}
	}
}

func assertJSONHas(t *testing.T, servers map[string]any, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, exists := servers[name]; !exists {
			t.Fatalf("expected JSON server %q in %#v", name, servers)
		}
	}
}

func assertJSONMissing(t *testing.T, servers map[string]any, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, exists := servers[name]; exists {
			t.Fatalf("unexpected JSON server %q in %#v", name, servers)
		}
	}
}

func assertEqual(t *testing.T, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("content changed\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func jsonPath(path string) string {
	out := ""
	for _, char := range path {
		if char == '\\' || char == '"' {
			out += "\\"
		}
		out += string(char)
	}
	return out
}
