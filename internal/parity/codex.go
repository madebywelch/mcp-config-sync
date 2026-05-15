package parity

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

func LoadCodexConfig(path string) (CodexConfig, error) {
	if path == "" {
		var err error
		path, err = DefaultCodexConfigPath()
		if err != nil {
			return CodexConfig{}, err
		}
	}

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CodexConfig{Path: path, Servers: map[string]map[string]any{}}, nil
	}
	if err != nil {
		return CodexConfig{}, err
	}

	root := map[string]any{}
	if len(content) > 0 {
		if err := toml.Unmarshal(content, &root); err != nil {
			return CodexConfig{}, fmt.Errorf("decode %s: %w", path, err)
		}
	}

	servers := map[string]map[string]any{}
	rawServers, ok := root["mcp_servers"].(map[string]any)
	if !ok {
		return CodexConfig{Path: path, Servers: servers}, nil
	}
	for name, rawConfig := range rawServers {
		if config, ok := rawConfig.(map[string]any); ok {
			servers[name] = config
		}
	}

	return CodexConfig{Path: path, Servers: servers}, nil
}
