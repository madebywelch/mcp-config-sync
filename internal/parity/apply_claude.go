package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const (
	TargetClaudeUser    = "claude-user"
	TargetClaudeLocal   = "claude-local"
	TargetClaudeProject = "claude-project"
)

func ApplyClaudeTargetedPlan(plan Plan, backup bool) ([]ApplyResult, error) {
	groups := map[string][]PlannedAdd{}
	for _, add := range plan.Adds {
		if add.Target == "" {
			return nil, fmt.Errorf("planned add %q has no Claude target", add.Name)
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
		result, err := ApplyClaudePlan(path, groups[path], backup)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func ApplyClaudePlan(path string, adds []PlannedAdd, backup bool) (ApplyResult, error) {
	result := ApplyResult{Path: path}
	if len(adds) == 0 {
		return result, nil
	}

	existing, mode, exists, err := readExistingFile(path)
	if err != nil {
		return ApplyResult{}, err
	}
	if mode == 0 {
		mode = 0o600
	}

	root := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return ApplyResult{}, fmt.Errorf("decode %s: %w", path, err)
		}
	}

	if backup && exists {
		backupPath := fmt.Sprintf("%s.bak-%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backupPath, existing, mode); err != nil {
			return ApplyResult{}, fmt.Errorf("write backup: %w", err)
		}
		result.BackupPath = backupPath
	}

	for _, add := range adds {
		targetMap, err := claudeTargetMap(root, add)
		if err != nil {
			return ApplyResult{}, err
		}
		if _, exists := targetMap[add.Name]; exists {
			continue
		}
		targetMap[add.Name] = add.Config
		result.Added++
	}

	content, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return ApplyResult{}, err
	}
	content = append(content, '\n')
	if err := atomicWriteFile(path, content, mode); err != nil {
		return ApplyResult{}, err
	}
	return result, nil
}

func claudeTargetMap(root map[string]any, add PlannedAdd) (map[string]any, error) {
	switch add.TargetKind {
	case TargetClaudeUser:
		return ensureObject(root, "mcpServers")
	case TargetClaudeLocal:
		if add.TargetProject == "" {
			return nil, fmt.Errorf("planned add %q targets Claude local config without a project", add.Name)
		}
		projects, err := ensureObject(root, "projects")
		if err != nil {
			return nil, err
		}
		project, err := ensureObject(projects, add.TargetProject)
		if err != nil {
			return nil, err
		}
		return ensureObject(project, "mcpServers")
	case TargetClaudeProject:
		return ensureObject(root, "mcpServers")
	default:
		return nil, fmt.Errorf("planned add %q has unsupported Claude target kind %q", add.Name, add.TargetKind)
	}
}

func ensureObject(parent map[string]any, key string) (map[string]any, error) {
	if raw, exists := parent[key]; exists {
		if value, ok := raw.(map[string]any); ok {
			return value, nil
		}
		return nil, fmt.Errorf("expected %q to be an object", key)
	}
	value := map[string]any{}
	parent[key] = value
	return value, nil
}
