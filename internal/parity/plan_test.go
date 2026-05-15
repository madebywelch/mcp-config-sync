package parity

import "testing"

func TestBuildPlanIsAdditiveByName(t *testing.T) {
	claude := []ClaudeServer{
		{Name: "existing", Config: map[string]any{"command": "npx"}},
		{Name: "missing", Config: map[string]any{"url": "https://example.com/mcp"}},
	}
	codex := CodexConfig{Servers: map[string]map[string]any{
		"existing": {"command": "npx"},
	}}

	plan := BuildPlan(claude, codex, nil)
	if len(plan.Adds) != 1 {
		t.Fatalf("adds = %#v", plan.Adds)
	}
	if plan.Adds[0].Name != "missing" {
		t.Fatalf("add name = %q", plan.Adds[0].Name)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Name != "existing" {
		t.Fatalf("skipped = %#v", plan.Skipped)
	}
}

func TestBuildCodexToClaudePlanIsAdditiveByName(t *testing.T) {
	codex := []CodexServer{
		{Name: "existing", Config: map[string]any{"command": "npx"}},
		{Name: "missing", Config: map[string]any{"url": "https://example.com/mcp"}},
	}
	claude := []ClaudeServer{
		{Name: "existing", Config: map[string]any{"command": "npx"}},
	}

	plan := BuildCodexToClaudePlan(codex, claude, "/tmp/.claude.json", TargetClaudeUser, "", nil)
	if len(plan.Adds) != 1 {
		t.Fatalf("adds = %#v", plan.Adds)
	}
	if plan.Adds[0].Name != "missing" {
		t.Fatalf("add name = %q", plan.Adds[0].Name)
	}
	if plan.Adds[0].TargetKind != TargetClaudeUser {
		t.Fatalf("target kind = %q", plan.Adds[0].TargetKind)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Name != "existing" {
		t.Fatalf("skipped = %#v", plan.Skipped)
	}
}
