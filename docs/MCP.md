# AegisOps MCP Server

The `aegisops-mcp` binary exposes AegisOps incident context to Cursor over **stdio**.

## Tools

| Tool | Description |
|------|-------------|
| `get_incident_bundle` | Returns markdown telemetry bundle for an issue ID |
| `start_fix_from_issue` | Mark fix issue `in_progress` and return bundle |
| `open_pr_from_issue` | Open GitHub PR for `aegis/fix-<id>` (also auto on push) |
| `start_feature_from_issue` | Mark feat issue `in_progress` and return bundle |
| `get_recent_issues` | List issues (optional `type`, `status` filters) |
| `get_live_logs` | Proxy Docker container logs from DevOps Copilot |
| `get_sentry_issue` | Proxy Sentry issue detail from DevOps Copilot |

## Cursor configuration

MCP config must live where Cursor reads it:

| Workspace opened | Config file |
|------------------|-------------|
| `AegisOps` repo root | `AegisOps/.cursor/mcp.json` (local dev) |
| Staging (same DB as EC2) | Copy `.cursor/mcp.staging.json.example` → `mcp.json` with Supabase `DATABASE_URL` |
| Global (all projects) | `~/.cursor/mcp.json` |

Use **one** MCP config only — duplicate entries in `Documents/.cursor` and `AegisOps/.cursor` cause confusion.

Local dev uses Postgres on **localhost:5433**. Staging issues on https://aegisops.syntropix.io require the staging `DATABASE_URL` or use the REST API / dashboard instead.

```json
{
  "mcpServers": {
    "aegisops": {
      "command": "c:\\Users\\kkjay\\Documents\\personal\\AegisOps\\bin\\aegisops-mcp.exe",
      "env": {
        "DATABASE_URL": "postgres://aegisops:aegisops@localhost:5433/aegisops?sslmode=disable",
        "COPILOT_API_URL": "http://localhost:5000",
        "SPOTTING_API_URL": "http://localhost:3000"
      }
    }
  }
}
```

### Troubleshooting

| Symptom | Fix |
|---------|-----|
| **No MCP Tools** in Settings | Workspace is `Documents`, not `AegisOps` — use `Documents/.cursor/mcp.json` or open the AegisOps folder |
| MCP server red / error | Ensure Postgres is up: `docker compose up -d` in AegisOps (port **5433**, not 5432) |
| Empty `~/.cursor/mcp.json` | Was `{}` — now populated with aegisops server |

## Closed-loop workflow

1. Copilot + Spotting webhooks create a correlated `fix:` issue.
2. Teams gateway sends **Approve Fix** / **Reject** links (or use dashboard **Start Fix**).
3. Cursor calls `start_fix_from_issue(issue_id=<id>)`.
4. Agent implements fix on branch `aegis/fix-<id>` and **pushes** — AegisOps auto-opens the PR (GitHub `push` webhook), or call `open_pr_from_issue`.
5. Merge PR → issue status becomes `fixed` automatically.
