package parity

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type VerifyOptions struct {
	Probe   bool
	Timeout time.Duration
}

type VerifyReport struct {
	OK          bool         `json:"ok"`
	Items       []VerifyItem `json:"items"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type VerifyItem struct {
	Name           string   `json:"name"`
	Source         string   `json:"source"`
	Target         string   `json:"target"`
	Transport      string   `json:"transport"`
	PresentInCodex bool     `json:"present_in_codex"`
	ProbeStatus    string   `json:"probe_status,omitempty"`
	ProbeDetail    string   `json:"probe_detail,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func Verify(claude []ClaudeServer, codex CodexConfig, diagnostics []Diagnostic, options VerifyOptions) VerifyReport {
	report := VerifyReport{OK: true, Diagnostics: append([]Diagnostic{}, diagnostics...)}
	if options.Timeout <= 0 {
		options.Timeout = 5 * time.Second
	}

	for _, server := range claude {
		converted := ConvertClaudeServer(server)
		item := VerifyItem{
			Name:           server.Name,
			Source:         server.Source.Label(),
			Target:         codex.Path,
			Transport:      converted.Transport,
			PresentInCodex: codex.Servers[server.Name] != nil,
			Warnings:       converted.Warnings,
		}
		if converted.Error != "" {
			item.ProbeStatus = "invalid"
			item.ProbeDetail = converted.Error
			report.OK = false
		}
		if !item.PresentInCodex {
			report.OK = false
		}
		if options.Probe && converted.Error == "" {
			item.ProbeStatus, item.ProbeDetail = probe(converted, options.Timeout)
			if item.ProbeStatus == "unavailable" || item.ProbeStatus == "invalid" {
				report.OK = false
			}
		}
		report.Items = append(report.Items, item)
	}

	return report
}

func MergeVerifyReports(reports ...VerifyReport) VerifyReport {
	merged := VerifyReport{OK: true}
	for _, report := range reports {
		if !report.OK {
			merged.OK = false
		}
		merged.Items = append(merged.Items, report.Items...)
		merged.Diagnostics = append(merged.Diagnostics, report.Diagnostics...)
	}
	return merged
}

func probe(converted Conversion, timeout time.Duration) (string, string) {
	switch converted.Transport {
	case "http":
		url, _ := converted.Config["url"].(string)
		return probeHTTP(url, timeout)
	case "stdio":
		command, _ := converted.Config["command"].(string)
		return probeCommand(command)
	default:
		return "invalid", "unknown transport"
	}
}

func probeHTTP(url string, timeout time.Duration) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: timeout}
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		request, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return "invalid", err.Error()
		}
		response, err := client.Do(request)
		if err != nil {
			continue
		}
		response.Body.Close()
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return "auth_required", response.Status
		}
		if response.StatusCode == http.StatusMethodNotAllowed {
			return "reachable", response.Status
		}
		if response.StatusCode >= 200 && response.StatusCode < 500 {
			return "reachable", response.Status
		}
		return "unavailable", response.Status
	}
	return "unavailable", "request failed"
}

func probeCommand(command string) (string, string) {
	if command == "" {
		return "invalid", "empty command"
	}
	if hasPathSeparator(command) {
		info, err := os.Stat(command)
		if err != nil {
			return "unavailable", err.Error()
		}
		if info.IsDir() {
			return "unavailable", "command path is a directory"
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			return "unavailable", "command is not executable"
		}
		return "available", command
	}
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "unavailable", err.Error()
	}
	return "available", resolved
}

func hasPathSeparator(command string) bool {
	return strings.ContainsRune(command, filepath.Separator) || strings.Contains(command, "/") || strings.Contains(command, "\\")
}
