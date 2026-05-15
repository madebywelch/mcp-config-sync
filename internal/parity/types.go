package parity

type Diagnostic struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Source struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Project string `json:"project,omitempty"`
}

func (s Source) Label() string {
	switch s.Kind {
	case "local":
		return "Claude local:" + s.Project
	case "project":
		return "Claude project:" + s.Path
	case "user":
		return "Claude user:" + s.Path
	case "codex-user":
		return "Codex user:" + s.Path
	case "codex-project":
		return "Codex project:" + s.Path
	default:
		if s.Path != "" {
			return s.Kind + ":" + s.Path
		}
		return s.Kind
	}
}

type ClaudeServer struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"-"`
	Source Source         `json:"source"`
}

type CodexConfig struct {
	Path    string                    `json:"path"`
	Servers map[string]map[string]any `json:"-"`
}

type CodexServer struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"-"`
	Source Source         `json:"source"`
}

type Conversion struct {
	Name      string         `json:"name"`
	Config    map[string]any `json:"-"`
	Transport string         `json:"transport"`
	Warnings  []string       `json:"warnings,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type PlannedAdd struct {
	Name          string         `json:"name"`
	Source        Source         `json:"source"`
	Target        string         `json:"target"`
	TargetKind    string         `json:"target_kind,omitempty"`
	TargetProject string         `json:"target_project,omitempty"`
	Transport     string         `json:"transport"`
	Config        map[string]any `json:"-"`
	Warnings      []string       `json:"warnings,omitempty"`
}

type SkippedServer struct {
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	Reason string `json:"reason"`
}

type Plan struct {
	Adds        []PlannedAdd    `json:"adds"`
	Skipped     []SkippedServer `json:"skipped"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
}

type ApplyResult struct {
	Path       string `json:"path"`
	BackupPath string `json:"backup_path,omitempty"`
	Added      int    `json:"added"`
}

type PrintablePlan struct {
	Adds        []PrintableAdd  `json:"adds"`
	Skipped     []SkippedServer `json:"skipped"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
}

type PrintableAdd struct {
	Name      string   `json:"name"`
	Source    string   `json:"source"`
	Target    string   `json:"target"`
	Transport string   `json:"transport"`
	Warnings  []string `json:"warnings,omitempty"`
}

type PrintableApplyResult struct {
	Added   int           `json:"added"`
	Results []ApplyResult `json:"results"`
	Plan    PrintablePlan `json:"plan"`
}

type PrintableClaudeServer struct {
	Name      string   `json:"name"`
	Source    string   `json:"source"`
	Transport string   `json:"transport"`
	Warnings  []string `json:"warnings,omitempty"`
}

type ParityConfig struct {
	Path        string   `json:"path"`
	DenyServers []string `json:"deny_servers"`
}
