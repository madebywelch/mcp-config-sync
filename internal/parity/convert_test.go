package parity

import "testing"

func TestConvertHTTPHeaders(t *testing.T) {
	server := ClaudeServer{
		Name: "secure",
		Config: map[string]any{
			"type": "http",
			"url":  "https://example.com/mcp",
			"headers": map[string]any{
				"Authorization": "Bearer ${API_TOKEN}",
				"X-Region":      "us-east-1",
				"X-Env":         "${DYNAMIC_HEADER}",
			},
		},
	}

	converted := ConvertClaudeServer(server)
	if converted.Error != "" {
		t.Fatalf("unexpected error: %s", converted.Error)
	}
	if got := converted.Config["bearer_token_env_var"]; got != "API_TOKEN" {
		t.Fatalf("bearer token env = %v", got)
	}
	envHeaders := converted.Config["env_http_headers"].(map[string]string)
	if got := envHeaders["X-Env"]; got != "DYNAMIC_HEADER" {
		t.Fatalf("env header = %v", got)
	}
	staticHeaders := converted.Config["http_headers"].(map[string]string)
	if got := staticHeaders["X-Region"]; got != "us-east-1" {
		t.Fatalf("static header = %v", got)
	}
}

func TestConvertStdio(t *testing.T) {
	server := ClaudeServer{
		Name: "local",
		Config: map[string]any{
			"command": "npx",
			"args":    []any{"-y", "example-mcp"},
			"env": map[string]any{
				"API_KEY": "secret",
			},
		},
	}

	converted := ConvertClaudeServer(server)
	if converted.Transport != "stdio" {
		t.Fatalf("transport = %q", converted.Transport)
	}
	if got := converted.Config["command"]; got != "npx" {
		t.Fatalf("command = %v", got)
	}
	args := converted.Config["args"].([]string)
	if len(args) != 2 || args[1] != "example-mcp" {
		t.Fatalf("args = %#v", args)
	}
}

func TestConvertRejectsInvalid(t *testing.T) {
	converted := ConvertClaudeServer(ClaudeServer{Name: "bad", Config: map[string]any{}})
	if converted.Error == "" {
		t.Fatal("expected conversion error")
	}
}
