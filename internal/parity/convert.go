package parity

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	exactEnvRef  = regexp.MustCompile(`^(?:\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*))$`)
	bearerEnvRef = regexp.MustCompile(`^Bearer\s+(?:\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*))$`)
)

func ConvertClaudeServer(server ClaudeServer) Conversion {
	result := Conversion{
		Name:   server.Name,
		Config: map[string]any{},
	}

	config := server.Config
	serverType, _ := asString(config["type"])
	serverType = strings.ToLower(serverType)

	if url, ok := asString(config["url"]); ok && url != "" {
		result.Transport = "http"
		result.Config["url"] = url
		if serverType == "sse" {
			result.Warnings = append(result.Warnings, "Claude server uses SSE transport; Codex supports streamable HTTP URLs, so this may need manual validation")
		}
	} else if command, ok := asString(config["command"]); ok && command != "" {
		result.Transport = "stdio"
		result.Config["command"] = command
	} else {
		result.Error = "server has neither url nor command"
		return result
	}

	copyString(config, result.Config, "cwd")
	copyBool(config, result.Config, "enabled")
	if disabled, ok := asBool(config["disabled"]); ok && disabled {
		result.Config["enabled"] = false
	}

	if args, ok := stringSlice(config["args"]); ok && len(args) > 0 {
		result.Config["args"] = args
	} else if _, exists := config["args"]; exists {
		result.Warnings = append(result.Warnings, "args is not a string array and was not copied")
	}

	if env, ok := mapStringString(config["env"]); ok && len(env) > 0 {
		result.Config["env"] = env
	} else if _, exists := config["env"]; exists {
		result.Warnings = append(result.Warnings, "env is not a string map and was not copied")
	}

	if headers, ok := mapStringString(config["headers"]); ok && len(headers) > 0 {
		staticHeaders, envHeaders, bearerEnv, warnings := convertHeaders(headers)
		for _, warning := range warnings {
			result.Warnings = append(result.Warnings, warning)
		}
		if len(staticHeaders) > 0 {
			result.Config["http_headers"] = staticHeaders
		}
		if len(envHeaders) > 0 {
			result.Config["env_http_headers"] = envHeaders
		}
		if bearerEnv != "" {
			result.Config["bearer_token_env_var"] = bearerEnv
		}
	} else if _, exists := config["headers"]; exists {
		result.Warnings = append(result.Warnings, "headers is not a string map and was not copied")
	}

	if oauth, ok := asMap(config["oauth"]); ok {
		if scopes, ok := asString(oauth["scopes"]); ok && strings.TrimSpace(scopes) != "" {
			result.Config["scopes"] = strings.Fields(scopes)
		} else if scopes, ok := stringSlice(oauth["scopes"]); ok && len(scopes) > 0 {
			result.Config["scopes"] = scopes
		}
		if _, exists := oauth["authServerMetadataUrl"]; exists {
			result.Warnings = append(result.Warnings, "oauth.authServerMetadataUrl has no direct Codex config equivalent and was not copied")
		}
	}

	if _, exists := config["headersHelper"]; exists {
		result.Warnings = append(result.Warnings, "headersHelper executes dynamically in Claude; Codex has no direct equivalent and it was not copied")
	}
	if _, exists := config["autoStart"]; exists {
		result.Warnings = append(result.Warnings, "autoStart has no direct Codex config equivalent and was not copied")
	}

	for _, key := range unsupportedKeys(config) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("unsupported Claude field %q was not copied", key))
	}

	sort.Strings(result.Warnings)
	return result
}

func ConvertCodexServer(server CodexServer) Conversion {
	result := Conversion{
		Name:   server.Name,
		Config: map[string]any{},
	}

	config := server.Config
	if url, ok := asString(config["url"]); ok && url != "" {
		result.Transport = "http"
		result.Config["type"] = "http"
		result.Config["url"] = url
	} else if command, ok := asString(config["command"]); ok && command != "" {
		result.Transport = "stdio"
		result.Config["type"] = "stdio"
		result.Config["command"] = command
	} else {
		result.Error = "server has neither url nor command"
		return result
	}

	copyString(config, result.Config, "cwd")

	if enabled, ok := asBool(config["enabled"]); ok && !enabled {
		result.Config["disabled"] = true
	}

	if args, ok := stringSliceFromAny(config["args"]); ok && len(args) > 0 {
		result.Config["args"] = args
	} else if _, exists := config["args"]; exists {
		result.Warnings = append(result.Warnings, "args is not a string array and was not copied")
	}

	if env, ok := mapStringStringFromAny(config["env"]); ok && len(env) > 0 {
		result.Config["env"] = env
	} else if _, exists := config["env"]; exists {
		result.Warnings = append(result.Warnings, "env is not a string map and was not copied")
	}

	headers := map[string]string{}
	if httpHeaders, ok := mapStringStringFromAny(config["http_headers"]); ok {
		for key, value := range httpHeaders {
			headers[key] = value
		}
	} else if _, exists := config["http_headers"]; exists {
		result.Warnings = append(result.Warnings, "http_headers is not a string map and was not copied")
	}
	if envHeaders, ok := mapStringStringFromAny(config["env_http_headers"]); ok {
		for key, value := range envHeaders {
			if _, exists := headers[key]; exists {
				result.Warnings = append(result.Warnings, fmt.Sprintf("header %q exists in both http_headers and env_http_headers; static value kept", key))
				continue
			}
			headers[key] = "${" + value + "}"
		}
	} else if _, exists := config["env_http_headers"]; exists {
		result.Warnings = append(result.Warnings, "env_http_headers is not a string map and was not copied")
	}
	if bearerEnv, ok := asString(config["bearer_token_env_var"]); ok && bearerEnv != "" {
		if _, exists := headers["Authorization"]; exists {
			result.Warnings = append(result.Warnings, "bearer_token_env_var was not copied because Authorization header already exists")
		} else {
			headers["Authorization"] = "Bearer ${" + bearerEnv + "}"
		}
	}
	if len(headers) > 0 {
		result.Config["headers"] = headers
	}

	if scopes, ok := stringSliceFromAny(config["scopes"]); ok && len(scopes) > 0 {
		result.Config["oauth"] = map[string]any{"scopes": scopes}
	} else if _, exists := config["scopes"]; exists {
		result.Warnings = append(result.Warnings, "scopes is not a string array and was not copied")
	}

	for _, key := range unsupportedCodexKeys(config) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("unsupported Codex field %q was not copied", key))
	}

	sort.Strings(result.Warnings)
	return result
}

func copyString(source map[string]any, target map[string]any, key string) {
	if value, ok := asString(source[key]); ok && value != "" {
		target[key] = value
	}
}

func copyBool(source map[string]any, target map[string]any, key string) {
	if value, ok := asBool(source[key]); ok {
		target[key] = value
	}
}

func convertHeaders(headers map[string]string) (map[string]string, map[string]string, string, []string) {
	staticHeaders := map[string]string{}
	envHeaders := map[string]string{}
	var bearerEnv string
	var warnings []string

	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := headers[name]
		if strings.EqualFold(name, "Authorization") {
			if matches := bearerEnvRef.FindStringSubmatch(value); len(matches) == 3 {
				envName := firstNonEmpty(matches[1], matches[2])
				if bearerEnv != "" && bearerEnv != envName {
					warnings = append(warnings, "multiple bearer token headers were found; only the first was converted to bearer_token_env_var")
					continue
				}
				bearerEnv = envName
				continue
			}
		}
		if matches := exactEnvRef.FindStringSubmatch(value); len(matches) == 3 {
			envHeaders[name] = firstNonEmpty(matches[1], matches[2])
			continue
		}
		if strings.Contains(value, "${") || strings.Contains(value, "$") {
			warnings = append(warnings, fmt.Sprintf("header %q uses dynamic expansion that Codex cannot represent directly; it was not copied", name))
			continue
		}
		staticHeaders[name] = value
	}

	return staticHeaders, envHeaders, bearerEnv, warnings
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func unsupportedKeys(config map[string]any) []string {
	supported := map[string]bool{
		"args":          true,
		"autoStart":     true,
		"command":       true,
		"cwd":           true,
		"disabled":      true,
		"enabled":       true,
		"env":           true,
		"headers":       true,
		"headersHelper": true,
		"oauth":         true,
		"type":          true,
		"url":           true,
	}
	var keys []string
	for key := range config {
		if !supported[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func unsupportedCodexKeys(config map[string]any) []string {
	supported := map[string]bool{
		"args":                 true,
		"bearer_token_env_var": true,
		"command":              true,
		"cwd":                  true,
		"enabled":              true,
		"env":                  true,
		"env_http_headers":     true,
		"http_headers":         true,
		"scopes":               true,
		"type":                 true,
		"url":                  true,
	}
	var keys []string
	for key := range config {
		if !supported[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
