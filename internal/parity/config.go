package parity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

const defaultParityConfig = `# codex-mcp-parity configuration
#
# Sync direction is selected per command. This file controls what is allowed
# to be copied in either direction.
#
# Names listed here are never copied into the target config.
deny_servers = []
`

func LoadParityConfig(path string) (ParityConfig, []Diagnostic, error) {
	if path == "" {
		var err error
		path, err = DefaultParityConfigPath()
		if err != nil {
			return ParityConfig{}, nil, err
		}
	}

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ParityConfig{Path: path}, nil, nil
	}
	if err != nil {
		return ParityConfig{}, nil, err
	}

	var raw struct {
		DenyServers []string `toml:"deny_servers"`
	}
	if err := toml.Unmarshal(content, &raw); err != nil {
		return ParityConfig{}, nil, fmt.Errorf("decode %s: %w", path, err)
	}
	sort.Strings(raw.DenyServers)
	return ParityConfig{Path: path, DenyServers: raw.DenyServers}, nil, nil
}

func WriteDefaultParityConfig(path string, overwrite bool) (string, error) {
	if path == "" {
		var err error
		path, err = DefaultParityConfigPath()
		if err != nil {
			return "", err
		}
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("config already exists: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(defaultParityConfig), 0o600)
}

func FilterDeniedServers(servers []ClaudeServer, config ParityConfig) ([]ClaudeServer, []Diagnostic) {
	if len(config.DenyServers) == 0 {
		return servers, nil
	}
	denied := map[string]bool{}
	for _, name := range config.DenyServers {
		denied[name] = true
	}

	filtered := make([]ClaudeServer, 0, len(servers))
	var diagnostics []Diagnostic
	for _, server := range servers {
		if denied[server.Name] {
			diagnostics = append(diagnostics, Diagnostic{
				Level:   "info",
				Message: fmt.Sprintf("skipping denied MCP server %q from %s", server.Name, server.Source.Label()),
			})
			continue
		}
		filtered = append(filtered, server)
	}
	return filtered, diagnostics
}

func FilterDeniedCodexServers(servers []CodexServer, config ParityConfig) ([]CodexServer, []Diagnostic) {
	if len(config.DenyServers) == 0 {
		return servers, nil
	}
	denied := map[string]bool{}
	for _, name := range config.DenyServers {
		denied[name] = true
	}

	filtered := make([]CodexServer, 0, len(servers))
	var diagnostics []Diagnostic
	for _, server := range servers {
		if denied[server.Name] {
			diagnostics = append(diagnostics, Diagnostic{
				Level:   "info",
				Message: fmt.Sprintf("skipping denied MCP server %q from %s", server.Name, server.Source.Label()),
			})
			continue
		}
		filtered = append(filtered, server)
	}
	return filtered, diagnostics
}
