package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/madebywelch/codex-mcp-parity/internal/parity"
)

type commonFlags struct {
	claudeConfig       string
	codexConfig        string
	codexProjectConfig string
	parityConfig       string
	targetScope        string
	project            string
	includeUser        bool
	includeLocal       bool
	includeProjectFile bool
	json               bool
}

type claudeGroup struct {
	servers     []parity.ClaudeServer
	diagnostics []parity.Diagnostic
	target      string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "diff":
		return runDiff(args[1:])
	case "sync":
		return runSync(args[1:])
	case "verify":
		return runVerify(args[1:])
	case "sources":
		return runSources(args[1:])
	case "init-config":
		return runInitConfig(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	common := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	plan, err := buildPlan(common)
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(parity.PrintablePlanFrom(plan))
	}
	printPlan("Diff", plan)
	return nil
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	common := addCommonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "print the planned additions without writing Codex config")
	noBackup := fs.Bool("no-backup", false, "do not create a timestamped backup before writing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	plan, err := buildPlan(common)
	if err != nil {
		return err
	}
	if *dryRun {
		if common.json {
			return writeJSON(parity.PrintablePlanFrom(plan))
		}
		printPlan("Dry Run", plan)
		return nil
	}

	results, err := parity.ApplyTargetedPlan(plan, !*noBackup)
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(parity.PrintableApplyResultsFrom(results, plan))
	}
	printPlan("Sync", plan)
	added := 0
	for _, result := range results {
		added += result.Added
	}
	if added == 0 {
		fmt.Println("No Codex changes needed.")
		return nil
	}
	fmt.Printf("Added %d server(s).\n", added)
	for _, result := range results {
		fmt.Printf("- %d to %s\n", result.Added, result.Path)
		if result.BackupPath != "" {
			fmt.Printf("  backup: %s\n", result.BackupPath)
		}
	}
	return nil
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	common := addCommonFlags(fs)
	probe := fs.Bool("probe", false, "check HTTP reachability and local command availability without starting stdio servers")
	timeout := fs.Duration("timeout", 5*time.Second, "probe timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	report, err := buildVerifyReport(common, parity.VerifyOptions{
		Probe:   *probe,
		Timeout: *timeout,
	})
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(report)
	}
	printVerify(report)
	if !report.OK {
		return errors.New("verification failed")
	}
	return nil
}

func runSources(args []string) error {
	fs := flag.NewFlagSet("sources", flag.ContinueOnError)
	common := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	groups, err := loadClaudeGroups(common)
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(map[string]any{
			"servers": printableSourcesFrom(groups),
		})
	}
	count := 0
	for _, group := range groups {
		for _, diagnostic := range group.diagnostics {
			fmt.Printf("%s: %s\n", strings.ToUpper(diagnostic.Level), diagnostic.Message)
		}
	}
	for _, group := range groups {
		for _, server := range group.servers {
			count++
			converted := parity.ConvertClaudeServer(server)
			fmt.Printf("%s\t%s\t%s\t%s\n", server.Name, server.Source.Label(), converted.Transport, group.target)
		}
	}
	if count == 0 {
		fmt.Println("No Claude MCP servers found for the selected scopes.")
	}
	return nil
}

func runInitConfig(args []string) error {
	fs := flag.NewFlagSet("init-config", flag.ContinueOnError)
	defaultPath, _ := parity.DefaultParityConfigPath()
	path := fs.String("parity-config", defaultPath, "path to write codex-mcp-parity config")
	force := fs.Bool("force", false, "overwrite an existing config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	written, err := parity.WriteDefaultParityConfig(*path, *force)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote config: %s\n", written)
	return nil
}

func addCommonFlags(fs *flag.FlagSet) *commonFlags {
	claudeConfig, _ := parity.DefaultClaudeConfigPath()
	codexConfig, _ := parity.DefaultCodexConfigPath()
	project, _ := os.Getwd()
	parityConfig, _ := parity.DefaultParityConfigPath()

	common := &commonFlags{
		claudeConfig:       claudeConfig,
		codexConfig:        codexConfig,
		parityConfig:       parityConfig,
		targetScope:        "preserve",
		project:            project,
		includeUser:        true,
		includeLocal:       true,
		includeProjectFile: true,
	}
	fs.StringVar(&common.claudeConfig, "claude-config", common.claudeConfig, "path to Claude Code JSON config")
	fs.StringVar(&common.codexConfig, "codex-config", common.codexConfig, "path to Codex user/global TOML config")
	fs.StringVar(&common.codexProjectConfig, "codex-project-config", common.codexProjectConfig, "path to Codex project TOML config (default: <project>/.codex/config.toml)")
	fs.StringVar(&common.parityConfig, "parity-config", common.parityConfig, "path to codex-mcp-parity TOML config")
	fs.StringVar(&common.targetScope, "target-scope", common.targetScope, "Codex target scope: preserve, user, or project")
	fs.StringVar(&common.project, "project", common.project, "project directory for Claude local and .mcp.json scopes")
	fs.BoolVar(&common.includeUser, "include-user", common.includeUser, "include Claude user-scope MCP servers")
	fs.BoolVar(&common.includeLocal, "include-local", common.includeLocal, "include Claude local project MCP servers")
	fs.BoolVar(&common.includeProjectFile, "include-project-file", common.includeProjectFile, "include project .mcp.json MCP servers")
	fs.BoolVar(&common.json, "json", false, "emit JSON output without secret values")
	return common
}

func buildPlan(common *commonFlags) (parity.Plan, error) {
	groups, err := loadClaudeGroups(common)
	if err != nil {
		return parity.Plan{}, err
	}
	plans := make([]parity.Plan, 0, len(groups))
	for _, group := range groups {
		codex, err := parity.LoadCodexConfig(group.target)
		if err != nil {
			return parity.Plan{}, err
		}
		plans = append(plans, parity.BuildPlan(group.servers, codex, group.diagnostics))
	}
	return parity.MergePlans(plans...), nil
}

func loadClaude(common *commonFlags) ([]parity.ClaudeServer, []parity.Diagnostic, error) {
	return parity.LoadClaudeServers(parity.ClaudeLoadOptions{
		ConfigPath:          common.claudeConfig,
		ProjectPath:         common.project,
		IncludeUser:         common.includeUser,
		IncludeLocal:        common.includeLocal,
		IncludeProjectFile:  common.includeProjectFile,
		DedupeByPrecedence:  true,
		RespectDisabledSets: true,
	})
}

func loadClaudeGroups(common *commonFlags) ([]claudeGroup, error) {
	config, configDiagnostics, err := parity.LoadParityConfig(common.parityConfig)
	if err != nil {
		return nil, err
	}
	addConfigDiagnostics := func(diagnostics []parity.Diagnostic) []parity.Diagnostic {
		if len(configDiagnostics) == 0 {
			return diagnostics
		}
		return append(append([]parity.Diagnostic{}, configDiagnostics...), diagnostics...)
	}
	filter := func(servers []parity.ClaudeServer, diagnostics []parity.Diagnostic) ([]parity.ClaudeServer, []parity.Diagnostic) {
		filtered, denyDiagnostics := parity.FilterDeniedServers(servers, config)
		diagnostics = append(diagnostics, denyDiagnostics...)
		return filtered, addConfigDiagnostics(diagnostics)
	}

	switch common.targetScope {
	case "user":
		servers, diagnostics, err := loadClaude(common)
		if err != nil {
			return nil, err
		}
		servers, diagnostics = filter(servers, diagnostics)
		return []claudeGroup{{servers: servers, diagnostics: diagnostics, target: common.codexConfig}}, nil
	case "project":
		projectTarget, err := codexProjectTarget(common)
		if err != nil {
			return nil, err
		}
		servers, diagnostics, err := loadClaude(common)
		if err != nil {
			return nil, err
		}
		servers, diagnostics = filter(servers, diagnostics)
		return []claudeGroup{{servers: servers, diagnostics: diagnostics, target: projectTarget}}, nil
	case "preserve":
		var groups []claudeGroup
		if common.includeUser {
			userServers, diagnostics, err := parity.LoadClaudeServers(parity.ClaudeLoadOptions{
				ConfigPath:          common.claudeConfig,
				ProjectPath:         common.project,
				IncludeUser:         true,
				IncludeLocal:        false,
				IncludeProjectFile:  false,
				DedupeByPrecedence:  true,
				RespectDisabledSets: true,
			})
			if err != nil {
				return nil, err
			}
			userServers, diagnostics = filter(userServers, diagnostics)
			groups = append(groups, claudeGroup{servers: userServers, diagnostics: diagnostics, target: common.codexConfig})
		}
		if common.includeLocal || common.includeProjectFile {
			projectTarget, err := codexProjectTarget(common)
			if err != nil {
				return nil, err
			}
			projectServers, diagnostics, err := parity.LoadClaudeServers(parity.ClaudeLoadOptions{
				ConfigPath:          common.claudeConfig,
				ProjectPath:         common.project,
				IncludeUser:         false,
				IncludeLocal:        common.includeLocal,
				IncludeProjectFile:  common.includeProjectFile,
				DedupeByPrecedence:  true,
				RespectDisabledSets: true,
			})
			if err != nil {
				return nil, err
			}
			projectServers, diagnostics = filter(projectServers, diagnostics)
			groups = append(groups, claudeGroup{servers: projectServers, diagnostics: diagnostics, target: projectTarget})
		}
		return groups, nil
	default:
		return nil, fmt.Errorf("invalid --target-scope %q; expected preserve, user, or project", common.targetScope)
	}
}

func codexProjectTarget(common *commonFlags) (string, error) {
	if common.codexProjectConfig != "" {
		return common.codexProjectConfig, nil
	}
	return parity.DefaultCodexProjectConfigPath(common.project)
}

func buildVerifyReport(common *commonFlags, options parity.VerifyOptions) (parity.VerifyReport, error) {
	groups, err := loadClaudeGroups(common)
	if err != nil {
		return parity.VerifyReport{}, err
	}
	reports := make([]parity.VerifyReport, 0, len(groups))
	for _, group := range groups {
		codex, err := parity.LoadCodexConfig(group.target)
		if err != nil {
			return parity.VerifyReport{}, err
		}
		reports = append(reports, parity.Verify(group.servers, codex, group.diagnostics, options))
	}
	return parity.MergeVerifyReports(reports...), nil
}

type printableSource struct {
	Name      string   `json:"name"`
	Source    string   `json:"source"`
	Target    string   `json:"target"`
	Transport string   `json:"transport"`
	Warnings  []string `json:"warnings,omitempty"`
}

func printableSourcesFrom(groups []claudeGroup) []printableSource {
	var sources []printableSource
	for _, group := range groups {
		for _, server := range group.servers {
			converted := parity.ConvertClaudeServer(server)
			sources = append(sources, printableSource{
				Name:      server.Name,
				Source:    server.Source.Label(),
				Target:    group.target,
				Transport: converted.Transport,
				Warnings:  converted.Warnings,
			})
		}
	}
	return sources
}

func writeJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func printPlan(title string, plan parity.Plan) {
	fmt.Println(title)
	for _, diagnostic := range plan.Diagnostics {
		fmt.Printf("%s: %s\n", strings.ToUpper(diagnostic.Level), diagnostic.Message)
	}
	if len(plan.Adds) == 0 {
		fmt.Println("No missing Codex MCP servers found.")
	} else {
		fmt.Println("Missing in Codex:")
		for _, add := range plan.Adds {
			fmt.Printf("- %s (%s, from %s -> %s)\n", add.Name, add.Transport, add.Source.Label(), add.Target)
			for _, warning := range add.Warnings {
				fmt.Printf("  warning: %s\n", warning)
			}
		}
	}
	if len(plan.Skipped) > 0 {
		fmt.Println("Already present in Codex:")
		for _, skipped := range plan.Skipped {
			if skipped.Target != "" {
				fmt.Printf("- %s (%s in %s)\n", skipped.Name, skipped.Reason, skipped.Target)
			} else {
				fmt.Printf("- %s (%s)\n", skipped.Name, skipped.Reason)
			}
		}
	}
}

func printVerify(report parity.VerifyReport) {
	for _, diagnostic := range report.Diagnostics {
		fmt.Printf("%s: %s\n", strings.ToUpper(diagnostic.Level), diagnostic.Message)
	}
	if len(report.Items) == 0 {
		fmt.Println("No selected Claude MCP servers found.")
		return
	}
	for _, item := range report.Items {
		status := "ok"
		if !item.PresentInCodex {
			status = "missing"
		}
		if item.ProbeStatus != "" {
			status += "/" + item.ProbeStatus
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", item.Name, item.Transport, status, item.Source, item.Target)
		for _, warning := range item.Warnings {
			fmt.Printf("  warning: %s\n", warning)
		}
	}
	if report.OK {
		fmt.Println("Verification passed.")
	}
}

func printUsage() {
	fmt.Println(`codex-mcp-parity keeps Codex MCP config additive with Claude Code MCP config.

Usage:
  codex-mcp-parity diff [flags]
  codex-mcp-parity sync [--dry-run] [flags]
  codex-mcp-parity verify [--probe] [flags]
  codex-mcp-parity sources [flags]
  codex-mcp-parity init-config [flags]

Run a command with -h for flags.`)
}
