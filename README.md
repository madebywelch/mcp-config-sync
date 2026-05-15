# codex-mcp-parity

`codex-mcp-parity` is a small Go CLI that additively syncs MCP server configuration between Claude Code and Codex.

It never performs a hidden two-way merge. You choose one direction per run:

- `claude-to-codex`
- `codex-to-claude`

Existing target entries are left untouched; only missing server names are appended.

## What It Syncs

Claude Code:

- user config: `~/.claude.json`
- local project config: `~/.claude.json` project entries
- project file config: `<project>/.mcp.json`

Codex:

- user config: `~/.codex/config.toml`
- project config: `<project>/.codex/config.toml`

Scope is preserved by default:

- Claude user <-> Codex user
- Claude project/local <-> Codex project

Codex project config is loaded only for trusted projects. Claude project-target writes default to `<project>/.mcp.json`; use `--claude-project-target local` to write reverse-sync project entries into `~/.claude.json` instead.

## Install

```sh
go build -o bin/codex-mcp-parity ./cmd/codex-mcp-parity
```

## Usage

```sh
codex-mcp-parity diff --direction claude-to-codex
codex-mcp-parity sync --direction claude-to-codex --dry-run
codex-mcp-parity sync --direction codex-to-claude --dry-run
codex-mcp-parity verify --direction codex-to-claude
```

Common options:

```sh
--direction claude-to-codex     claude-to-codex or codex-to-claude
--project PATH                  Project used for project-scope config
--target-scope preserve         preserve, user, or project
--claude-project-target MODE    project-file or local
--parity-config PATH            Path to codex-mcp-parity config
--json                          JSON output without secret values
```

## Deny List

Create a config file:

```sh
codex-mcp-parity init-config
```

Then add server names that should never be copied in either direction:

```toml
deny_servers = [
  "private-company-mcp",
  "claude-only-server",
]
```

An example is included in [`codex-mcp-parity.example.toml`](codex-mcp-parity.example.toml).

## Notes

- OAuth-backed MCP servers may still need a fresh login after sync.
- `verify --probe` checks HTTP reachability and local command availability, but does not start stdio MCP servers.
- Secret values are not printed in normal or JSON output.
- In `codex-to-claude`, either project include flag selects Codex's single project config source.

## License

MIT. Use it however you want; no warranty is provided.
