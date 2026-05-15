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
	claudeConfig        string
	codexConfig         string
	codexProjectConfig  string
	parityConfig        string
	direction           string
	targetScope         string
	claudeProjectTarget string
	project             string
	includeUser         bool
	includeLocal        bool
	includeProjectFile  bool
	json                bool
}

type claudeGroup struct {
	servers     []parity.ClaudeServer
	diagnostics []parity.Diagnostic
	target      string
}

type codexGroup struct {
	servers       []parity.CodexServer
	diagnostics   []parity.Diagnostic
	target        string
	targetKind    string
	targetProject string
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

	results, err := applyPlan(common, plan, !*noBackup)
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
		fmt.Println("No target changes needed.")
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
	if err := normalizeCommon(common); err != nil {
		return err
	}

	if common.direction == "codex-to-claude" {
		return runCodexSources(common)
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
		claudeConfig:        claudeConfig,
		codexConfig:         codexConfig,
		parityConfig:        parityConfig,
		direction:           "claude-to-codex",
		targetScope:         "preserve",
		claudeProjectTarget: "project-file",
		project:             project,
		includeUser:         true,
		includeLocal:        true,
		includeProjectFile:  true,
	}
	fs.StringVar(&common.claudeConfig, "claude-config", common.claudeConfig, "path to Claude Code JSON config")
	fs.StringVar(&common.codexConfig, "codex-config", common.codexConfig, "path to Codex user/global TOML config")
	fs.StringVar(&common.codexProjectConfig, "codex-project-config", common.codexProjectConfig, "path to Codex project TOML config (default: <project>/.codex/config.toml)")
	fs.StringVar(&common.parityConfig, "parity-config", common.parityConfig, "path to codex-mcp-parity TOML config")
	fs.StringVar(&common.direction, "direction", common.direction, "sync direction: claude-to-codex or codex-to-claude")
	fs.StringVar(&common.targetScope, "target-scope", common.targetScope, "target scope strategy: preserve, user, or project")
	fs.StringVar(&common.claudeProjectTarget, "claude-project-target", common.claudeProjectTarget, "Claude project target for codex-to-claude: project-file or local")
	fs.StringVar(&common.project, "project", common.project, "project directory for project-scoped MCP config")
	fs.BoolVar(&common.includeUser, "include-user", common.includeUser, "include user-scope MCP servers")
	fs.BoolVar(&common.includeLocal, "include-local", common.includeLocal, "include local project-scope MCP servers")
	fs.BoolVar(&common.includeProjectFile, "include-project-file", common.includeProjectFile, "include project-file MCP servers")
	fs.BoolVar(&common.json, "json", false, "emit JSON output without secret values")
	return common
}

func buildPlan(common *commonFlags) (parity.Plan, error) {
	if err := normalizeCommon(common); err != nil {
		return parity.Plan{}, err
	}
	if common.direction == "codex-to-claude" {
		return buildCodexToClaudePlan(common)
	}
	return buildClaudeToCodexPlan(common)
}

func buildClaudeToCodexPlan(common *commonFlags) (parity.Plan, error) {
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

func buildCodexToClaudePlan(common *commonFlags) (parity.Plan, error) {
	groups, err := loadCodexGroups(common)
	if err != nil {
		return parity.Plan{}, err
	}
	plans := make([]parity.Plan, 0, len(groups))
	for _, group := range groups {
		claude, diagnostics, err := loadClaudeTarget(common, group.targetKind)
		if err != nil {
			return parity.Plan{}, err
		}
		diagnostics = append(group.diagnostics, diagnostics...)
		plans = append(plans, parity.BuildCodexToClaudePlan(group.servers, claude, group.target, group.targetKind, group.targetProject, diagnostics))
	}
	return parity.MergePlans(plans...), nil
}

func applyPlan(common *commonFlags, plan parity.Plan, backup bool) ([]parity.ApplyResult, error) {
	if err := normalizeCommon(common); err != nil {
		return nil, err
	}
	if common.direction == "codex-to-claude" {
		return parity.ApplyClaudeTargetedPlan(plan, backup)
	}
	return parity.ApplyTargetedPlan(plan, backup)
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

func loadCodexGroups(common *commonFlags) ([]codexGroup, error) {
	config, configDiagnostics, err := parity.LoadParityConfig(common.parityConfig)
	if err != nil {
		return nil, err
	}
	filter := func(servers []parity.CodexServer, diagnostics []parity.Diagnostic) ([]parity.CodexServer, []parity.Diagnostic) {
		filtered, denyDiagnostics := parity.FilterDeniedCodexServers(servers, config)
		diagnostics = append(diagnostics, denyDiagnostics...)
		if len(configDiagnostics) > 0 {
			diagnostics = append(append([]parity.Diagnostic{}, configDiagnostics...), diagnostics...)
		}
		return filtered, diagnostics
	}

	loadUser := func(target string, targetKind string, targetProject string) (codexGroup, error) {
		servers, diagnostics, err := parity.LoadCodexServers(common.codexConfig, parity.Source{Kind: "codex-user", Path: common.codexConfig})
		if err != nil {
			return codexGroup{}, err
		}
		servers, diagnostics = filter(servers, diagnostics)
		return codexGroup{servers: servers, diagnostics: diagnostics, target: target, targetKind: targetKind, targetProject: targetProject}, nil
	}
	loadProject := func(target string, targetKind string, targetProject string) (codexGroup, error) {
		projectConfig, err := codexProjectTarget(common)
		if err != nil {
			return codexGroup{}, err
		}
		servers, diagnostics, err := parity.LoadCodexServers(projectConfig, parity.Source{Kind: "codex-project", Path: projectConfig})
		if err != nil {
			return codexGroup{}, err
		}
		servers, diagnostics = filter(servers, diagnostics)
		return codexGroup{servers: servers, diagnostics: diagnostics, target: target, targetKind: targetKind, targetProject: targetProject}, nil
	}

	switch common.targetScope {
	case "user":
		target, targetKind, targetProject, err := claudeUserTarget(common)
		if err != nil {
			return nil, err
		}
		var groups []codexGroup
		if common.includeUser {
			group, err := loadUser(target, targetKind, targetProject)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
		}
		if includeProjectSources(common) {
			group, err := loadProject(target, targetKind, targetProject)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
		}
		return groups, nil
	case "project":
		target, targetKind, targetProject, err := claudeProjectTarget(common)
		if err != nil {
			return nil, err
		}
		var groups []codexGroup
		if common.includeUser {
			group, err := loadUser(target, targetKind, targetProject)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
		}
		if includeProjectSources(common) {
			group, err := loadProject(target, targetKind, targetProject)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
		}
		return groups, nil
	case "preserve":
		var groups []codexGroup
		if common.includeUser {
			target, targetKind, targetProject, err := claudeUserTarget(common)
			if err != nil {
				return nil, err
			}
			group, err := loadUser(target, targetKind, targetProject)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
		}
		if includeProjectSources(common) {
			target, targetKind, targetProject, err := claudeProjectTarget(common)
			if err != nil {
				return nil, err
			}
			group, err := loadProject(target, targetKind, targetProject)
			if err != nil {
				return nil, err
			}
			groups = append(groups, group)
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
	if err := normalizeCommon(common); err != nil {
		return parity.VerifyReport{}, err
	}
	if common.direction == "codex-to-claude" {
		return buildCodexToClaudeVerifyReport(common, options)
	}
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

func buildCodexToClaudeVerifyReport(common *commonFlags, options parity.VerifyOptions) (parity.VerifyReport, error) {
	groups, err := loadCodexGroups(common)
	if err != nil {
		return parity.VerifyReport{}, err
	}
	reports := make([]parity.VerifyReport, 0, len(groups))
	for _, group := range groups {
		claude, diagnostics, err := loadClaudeTarget(common, group.targetKind)
		if err != nil {
			return parity.VerifyReport{}, err
		}
		diagnostics = append(group.diagnostics, diagnostics...)
		reports = append(reports, parity.VerifyCodexToClaude(group.servers, claude, group.target, diagnostics, options))
	}
	return parity.MergeVerifyReports(reports...), nil
}

func loadClaudeTarget(common *commonFlags, targetKind string) ([]parity.ClaudeServer, []parity.Diagnostic, error) {
	switch targetKind {
	case parity.TargetClaudeUser:
		return parity.LoadClaudeServers(parity.ClaudeLoadOptions{
			ConfigPath:          common.claudeConfig,
			ProjectPath:         common.project,
			IncludeUser:         true,
			IncludeLocal:        false,
			IncludeProjectFile:  false,
			DedupeByPrecedence:  true,
			RespectDisabledSets: true,
		})
	case parity.TargetClaudeLocal:
		return parity.LoadClaudeServers(parity.ClaudeLoadOptions{
			ConfigPath:          common.claudeConfig,
			ProjectPath:         common.project,
			IncludeUser:         false,
			IncludeLocal:        true,
			IncludeProjectFile:  false,
			DedupeByPrecedence:  true,
			RespectDisabledSets: true,
		})
	case parity.TargetClaudeProject:
		return parity.LoadClaudeServers(parity.ClaudeLoadOptions{
			ConfigPath:          common.claudeConfig,
			ProjectPath:         common.project,
			IncludeUser:         false,
			IncludeLocal:        false,
			IncludeProjectFile:  true,
			DedupeByPrecedence:  true,
			RespectDisabledSets: true,
		})
	default:
		return nil, nil, fmt.Errorf("unsupported Claude target kind %q", targetKind)
	}
}

func includeProjectSources(common *commonFlags) bool {
	return common.includeLocal || common.includeProjectFile
}

func claudeUserTarget(common *commonFlags) (string, string, string, error) {
	return common.claudeConfig, parity.TargetClaudeUser, "", nil
}

func claudeProjectTarget(common *commonFlags) (string, string, string, error) {
	project, err := parity.CleanProjectPath(common.project)
	if err != nil {
		return "", "", "", err
	}
	switch common.claudeProjectTarget {
	case "project-file":
		path, err := parity.DefaultClaudeProjectConfigPath(project)
		if err != nil {
			return "", "", "", err
		}
		return path, parity.TargetClaudeProject, project, nil
	case "local":
		return common.claudeConfig, parity.TargetClaudeLocal, project, nil
	default:
		return "", "", "", fmt.Errorf("invalid --claude-project-target %q; expected project-file or local", common.claudeProjectTarget)
	}
}

func normalizeCommon(common *commonFlags) error {
	switch common.direction {
	case "claude-to-codex", "ctc":
		common.direction = "claude-to-codex"
	case "codex-to-claude", "ctc-reverse", "ctcl", "reverse":
		common.direction = "codex-to-claude"
	default:
		return fmt.Errorf("invalid --direction %q; expected claude-to-codex or codex-to-claude", common.direction)
	}
	switch common.targetScope {
	case "preserve", "user", "project":
	default:
		return fmt.Errorf("invalid --target-scope %q; expected preserve, user, or project", common.targetScope)
	}
	switch common.claudeProjectTarget {
	case "project-file", "local":
	default:
		return fmt.Errorf("invalid --claude-project-target %q; expected project-file or local", common.claudeProjectTarget)
	}
	return nil
}

func runCodexSources(common *commonFlags) error {
	groups, err := loadCodexGroups(common)
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(map[string]any{
			"servers": printableCodexSourcesFrom(groups),
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
			converted := parity.ConvertCodexServer(server)
			fmt.Printf("%s\t%s\t%s\t%s\n", server.Name, server.Source.Label(), converted.Transport, group.target)
		}
	}
	if count == 0 {
		fmt.Println("No Codex MCP servers found for the selected scopes.")
	}
	return nil
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

func printableCodexSourcesFrom(groups []codexGroup) []printableSource {
	var sources []printableSource
	for _, group := range groups {
		for _, server := range group.servers {
			converted := parity.ConvertCodexServer(server)
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
		fmt.Println("No missing target MCP servers found.")
	} else {
		fmt.Println("Missing in target:")
		for _, add := range plan.Adds {
			fmt.Printf("- %s (%s, from %s -> %s)\n", add.Name, add.Transport, add.Source.Label(), add.Target)
			for _, warning := range add.Warnings {
				fmt.Printf("  warning: %s\n", warning)
			}
		}
	}
	if len(plan.Skipped) > 0 {
		fmt.Println("Already present in target:")
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
		fmt.Println("No selected MCP servers found.")
		return
	}
	for _, item := range report.Items {
		status := "ok"
		if !item.PresentInTarget {
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
	fmt.Println(`codex-mcp-parity keeps Claude Code and Codex MCP config in additive sync.

Usage:
  codex-mcp-parity diff [--direction claude-to-codex|codex-to-claude] [flags]
  codex-mcp-parity sync [--dry-run] [--direction claude-to-codex|codex-to-claude] [flags]
  codex-mcp-parity verify [--probe] [--direction claude-to-codex|codex-to-claude] [flags]
  codex-mcp-parity sources [--direction claude-to-codex|codex-to-claude] [flags]
  codex-mcp-parity init-config [flags]

Run a command with -h for flags.`)
}
