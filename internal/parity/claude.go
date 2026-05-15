package parity

import (
	"fmt"
	"path/filepath"
	"sort"
)

type ClaudeLoadOptions struct {
	ConfigPath          string
	ProjectPath         string
	IncludeUser         bool
	IncludeLocal        bool
	IncludeProjectFile  bool
	DedupeByPrecedence  bool
	RespectDisabledSets bool
}

func LoadClaudeServers(options ClaudeLoadOptions) ([]ClaudeServer, []Diagnostic, error) {
	var diagnostics []Diagnostic
	var servers []ClaudeServer
	seen := map[string]bool{}

	configPath := options.ConfigPath
	if configPath == "" {
		var err error
		configPath, err = DefaultClaudeConfigPath()
		if err != nil {
			return nil, nil, err
		}
	}
	projectPath := options.ProjectPath
	if projectPath != "" {
		absolute, err := filepath.Abs(projectPath)
		if err != nil {
			return nil, nil, err
		}
		projectPath = filepath.Clean(absolute)
	}

	root, exists, err := readJSONFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		diagnostics = append(diagnostics, Diagnostic{Level: "warning", Message: fmt.Sprintf("Claude config not found: %s", configPath)})
		root = map[string]any{}
	}

	projectEntry := map[string]any{}
	if projectPath != "" {
		if projects, ok := asMap(root["projects"]); ok {
			if exact, ok := asMap(projects[projectPath]); ok {
				projectEntry = exact
			}
		}
	}

	addGroup := func(source Source, group map[string]any, disabled map[string]bool) {
		names := make([]string, 0, len(group))
		for name := range group {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if disabled[name] {
				diagnostics = append(diagnostics, Diagnostic{Level: "info", Message: fmt.Sprintf("skipping disabled Claude MCP server %q from %s", name, source.Label())})
				continue
			}
			config, ok := asMap(group[name])
			if !ok {
				diagnostics = append(diagnostics, Diagnostic{Level: "warning", Message: fmt.Sprintf("skipping Claude MCP server %q from %s because it is not an object", name, source.Label())})
				continue
			}
			if options.DedupeByPrecedence && seen[name] {
				diagnostics = append(diagnostics, Diagnostic{Level: "info", Message: fmt.Sprintf("skipping lower-precedence duplicate Claude MCP server %q from %s", name, source.Label())})
				continue
			}
			seen[name] = true
			servers = append(servers, ClaudeServer{Name: name, Config: config, Source: source})
		}
	}

	if options.IncludeLocal && projectPath != "" {
		disabled := map[string]bool{}
		if options.RespectDisabledSets {
			disabled = stringSet(projectEntry["disabledMcpServers"])
		}
		if localServers, ok := asMap(projectEntry["mcpServers"]); ok {
			addGroup(Source{Kind: "local", Path: configPath, Project: projectPath}, localServers, disabled)
		}
	}

	if options.IncludeProjectFile && projectPath != "" {
		projectMCPPath := filepath.Join(projectPath, ".mcp.json")
		projectRoot, exists, err := readJSONFile(projectMCPPath)
		if err != nil {
			return nil, nil, err
		}
		if exists {
			disabled := map[string]bool{}
			if options.RespectDisabledSets {
				disabled = stringSet(projectEntry["disabledMcpjsonServers"])
			}
			if projectServers, ok := asMap(projectRoot["mcpServers"]); ok {
				addGroup(Source{Kind: "project", Path: projectMCPPath, Project: projectPath}, projectServers, disabled)
			} else {
				diagnostics = append(diagnostics, Diagnostic{Level: "warning", Message: fmt.Sprintf("project MCP file has no mcpServers object: %s", projectMCPPath)})
			}
		}
	}

	if options.IncludeUser {
		if userServers, ok := asMap(root["mcpServers"]); ok {
			addGroup(Source{Kind: "user", Path: configPath}, userServers, nil)
		}
	}

	return servers, diagnostics, nil
}
