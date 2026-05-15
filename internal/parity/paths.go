package parity

import (
	"os"
	"path/filepath"
)

func DefaultClaudeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude.json"), nil
}

func DefaultCodexConfigPath() (string, error) {
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return filepath.Join(codexHome, "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func DefaultCodexProjectConfigPath(project string) (string, error) {
	project, err := CleanProjectPath(project)
	if err != nil {
		return "", err
	}
	return filepath.Join(project, ".codex", "config.toml"), nil
}

func DefaultClaudeProjectConfigPath(project string) (string, error) {
	project, err := CleanProjectPath(project)
	if err != nil {
		return "", err
	}
	return filepath.Join(project, ".mcp.json"), nil
}

func CleanProjectPath(project string) (string, error) {
	if project == "" {
		var err error
		project, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	absolute, err := filepath.Abs(project)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func DefaultParityConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "codex-mcp-parity", "config.toml"), nil
}
