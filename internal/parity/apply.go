package parity

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func ApplyTargetedPlan(plan Plan, backup bool) ([]ApplyResult, error) {
	groups := map[string][]PlannedAdd{}
	for _, add := range plan.Adds {
		if add.Target == "" {
			return nil, fmt.Errorf("planned add %q has no Codex target", add.Name)
		}
		groups[add.Target] = append(groups[add.Target], add)
	}

	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	results := make([]ApplyResult, 0, len(paths))
	for _, path := range paths {
		result, err := ApplyPlan(path, Plan{Adds: groups[path]}, backup)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func ApplyPlan(path string, plan Plan, backup bool) (ApplyResult, error) {
	if path == "" {
		var err error
		path, err = DefaultCodexConfigPath()
		if err != nil {
			return ApplyResult{}, err
		}
	}

	result := ApplyResult{Path: path}
	if len(plan.Adds) == 0 {
		return result, nil
	}

	existing, mode, exists, err := readExistingFile(path)
	if err != nil {
		return ApplyResult{}, err
	}
	if mode == 0 {
		mode = 0o600
	}

	if backup && exists {
		backupPath := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backupPath, existing, mode); err != nil {
			return ApplyResult{}, fmt.Errorf("write backup: %w", err)
		}
		result.BackupPath = backupPath
	}

	var output bytes.Buffer
	output.Write(bytes.TrimRight(existing, "\n"))
	if output.Len() > 0 {
		output.WriteString("\n\n")
	}
	output.WriteString("# Added by codex-mcp-parity. Existing Codex MCP entries were left unchanged.\n")

	for _, add := range plan.Adds {
		block, err := marshalCodexServer(add.Name, add.Config)
		if err != nil {
			return ApplyResult{}, fmt.Errorf("encode %s: %w", add.Name, err)
		}
		output.Write(block)
		if len(block) == 0 || block[len(block)-1] != '\n' {
			output.WriteByte('\n')
		}
		output.WriteByte('\n')
		result.Added++
	}

	if err := atomicWriteFile(path, output.Bytes(), mode); err != nil {
		return ApplyResult{}, err
	}
	return result, nil
}

func readExistingFile(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	return content, info.Mode().Perm(), true, nil
}

func marshalCodexServer(name string, config map[string]any) ([]byte, error) {
	doc := map[string]any{
		"mcp_servers": map[string]any{
			name: config,
		},
	}
	content, err := toml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return bytes.Replace(content, []byte("[mcp_servers]\n"), nil, 1), nil
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".codex-mcp-parity-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
