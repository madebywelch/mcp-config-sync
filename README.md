# codex-mcp-parity

`codex-mcp-parity` is a small Go CLI that additively syncs MCP server configuration from Claude Code into Codex.

It is intentionally one-way: **Claude Code -> Codex**. Codex is read only to detect what already exists; it is never used as a source to write back into Claude.

## What It Does

- Reads Claude Code user MCP servers from `~/.claude.json`.
- Reads Claude Code project/local MCP servers from `~/.claude.json` project entries and `<project>/.mcp.json`.
- Adds missing MCP servers to Codex without removing or rewriting existing entries.
- Preserves scope by default:
  - Claude user scope -> `~/.codex/config.toml`
  - Claude project/local scope -> `<project>/.codex/config.toml`
- Supports a deny list for MCP servers that should remain Claude-only.

Codex project-scoped config is supported through `.codex/config.toml`, and Codex loads it only for trusted projects.

## Install

```sh
go build -o bin/codex-mcp-parity ./cmd/codex-mcp-parity
```

## Usage

```sh
codex-mcp-parity diff
codex-mcp-parity sync --dry-run
codex-mcp-parity sync
codex-mcp-parity verify
codex-mcp-parity verify --probe
```

Common options:

```sh
--project PATH                Project used for Claude/Codex project-scope config
--target-scope preserve       preserve, user, or project
--parity-config PATH          Path to codex-mcp-parity config
--codex-config PATH           Codex user config path
--codex-project-config PATH   Codex project config path
--json                        JSON output without secret values
```

## Deny List

Create a config file:

```sh
codex-mcp-parity init-config
```

Then add server names that should never be copied into Codex:

```toml
deny_servers = [
  "private-company-mcp",
  "claude-only-server",
]
```

An example is included in [`codex-mcp-parity.example.toml`](codex-mcp-parity.example.toml).

## Notes

- OAuth-backed MCP servers may still need `codex mcp login <server-name>` after sync.
- `verify --probe` checks HTTP reachability and local command availability, but does not start stdio MCP servers.
- Secret values are not printed in normal or JSON output.

## License

MIT. Use it however you want; no warranty is provided.

