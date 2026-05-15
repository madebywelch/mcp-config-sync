package parity

func BuildPlan(claude []ClaudeServer, codex CodexConfig, diagnostics []Diagnostic) Plan {
	plan := Plan{Diagnostics: append([]Diagnostic{}, diagnostics...)}

	for _, server := range claude {
		if _, exists := codex.Servers[server.Name]; exists {
			plan.Skipped = append(plan.Skipped, SkippedServer{Name: server.Name, Target: codex.Path, Reason: "same name exists"})
			continue
		}
		converted := ConvertClaudeServer(server)
		if converted.Error != "" {
			plan.Diagnostics = append(plan.Diagnostics, Diagnostic{Level: "warning", Message: "skipping " + server.Name + ": " + converted.Error})
			continue
		}
		plan.Adds = append(plan.Adds, PlannedAdd{
			Name:      server.Name,
			Source:    server.Source,
			Target:    codex.Path,
			Transport: converted.Transport,
			Config:    converted.Config,
			Warnings:  converted.Warnings,
		})
	}

	return plan
}

func MergePlans(plans ...Plan) Plan {
	var merged Plan
	for _, plan := range plans {
		merged.Adds = append(merged.Adds, plan.Adds...)
		merged.Skipped = append(merged.Skipped, plan.Skipped...)
		merged.Diagnostics = append(merged.Diagnostics, plan.Diagnostics...)
	}
	return merged
}

func PrintablePlanFrom(plan Plan) PrintablePlan {
	adds := make([]PrintableAdd, 0, len(plan.Adds))
	for _, add := range plan.Adds {
		adds = append(adds, PrintableAdd{
			Name:      add.Name,
			Source:    add.Source.Label(),
			Target:    add.Target,
			Transport: add.Transport,
			Warnings:  add.Warnings,
		})
	}
	return PrintablePlan{
		Adds:        adds,
		Skipped:     plan.Skipped,
		Diagnostics: plan.Diagnostics,
	}
}

func PrintableApplyResultFrom(result ApplyResult, plan Plan) PrintableApplyResult {
	return PrintableApplyResultsFrom([]ApplyResult{result}, plan)
}

func PrintableApplyResultsFrom(results []ApplyResult, plan Plan) PrintableApplyResult {
	added := 0
	for _, result := range results {
		added += result.Added
	}
	return PrintableApplyResult{
		Added:   added,
		Results: results,
		Plan:    PrintablePlanFrom(plan),
	}
}

func PrintableClaudeServersFrom(servers []ClaudeServer) []PrintableClaudeServer {
	printable := make([]PrintableClaudeServer, 0, len(servers))
	for _, server := range servers {
		converted := ConvertClaudeServer(server)
		printable = append(printable, PrintableClaudeServer{
			Name:      server.Name,
			Source:    server.Source.Label(),
			Transport: converted.Transport,
			Warnings:  converted.Warnings,
		})
	}
	return printable
}
