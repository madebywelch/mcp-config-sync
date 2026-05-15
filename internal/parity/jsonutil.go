package parity

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func readJSONFile(path string) (map[string]any, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, true, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, true, fmt.Errorf("decode %s: trailing JSON content", path)
	}
	return root, true, nil
}

func asMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func asString(value any) (string, bool) {
	typed, ok := value.(string)
	return typed, ok
}

func asBool(value any) (bool, bool) {
	typed, ok := value.(bool)
	return typed, ok
}

func stringSet(value any) map[string]bool {
	result := map[string]bool{}
	items, ok := value.([]any)
	if !ok {
		return result
	}
	for _, item := range items {
		if text, ok := item.(string); ok {
			result[text] = true
		}
	}
	return result
}

func mapStringString(value any) (map[string]string, bool) {
	source, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	result := map[string]string{}
	for key, raw := range source {
		text, ok := raw.(string)
		if !ok {
			return nil, false
		}
		result[key] = text
	}
	return result, true
}

func stringSlice(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}
