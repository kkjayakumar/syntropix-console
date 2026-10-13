# AegisOps Security

## Webhook authentication

| Endpoint | Header | Algorithm |
|----------|--------|-----------|
| `POST /api/v1/webhooks/copilot` | `X-Aegis-Signature: sha256=<hex>` | HMAC-SHA256 of raw body |
| `POST /api/v1/webhooks/spotting` | `X-Aegis-Signature: sha256=<hex>` | HMAC-SHA256 of raw body |
| `POST /api/v1/github/webhook` | `X-Hub-Signature-256: sha256=<hex>` | HMAC-SHA256 of raw body |

Shared secret for Copilot/Spotting: `AEGIS_WEBHOOK_SECRET`  
GitHub App/webhook secret: `GITHUB_WEBHOOK_SECRET`

In production (`AEGISOPS_ENV=production`), startup fails if default or missing webhook secrets.

## HTTP API authentication

| Mode | When | Behavior |
|------|------|----------|
| Enforced | `AEGISOPS_API_TOKEN` set, or production without `AEGISOPS_ALLOW_INSECURE_AUTH` | `/api/*` requires `Authorization: Bearer …` or `X-AegisOps-Token` (except health/version, signed webhooks, gateway approve/reject links) |
| Development | `AEGISOPS_ENV=development` and no token | Default-admin + optional `X-User-*` (local only) |
| Emergency | `AEGISOPS_ALLOW_INSECURE_AUTH=true` | Auth middleware off — do not leave enabled |

Production startup requires `AEGISOPS_API_TOKEN` (≥16 chars) unless insecure allow is set. Deploy generates and persists the token to Secrets Manager when missing. Dashboard Settings stores the client copy in browser localStorage only.

When token auth succeeds, `ActorUserID` uses `AEGISOPS_AUTH_USER_ID` (default `1`) and **ignores** spoofable `X-User-Id` / `X-User-Email`.

## Rate limiting

Webhook endpoints are limited to `WEBHOOK_RATE_LIMIT` requests per minute per client IP (default **120**).

## Service tokens

| Direction | Token env |
|-----------|-----------|
| AegisOps → DevOps Copilot | `COPILOT_SERVICE_TOKEN` |
| AegisOps → Spotting | `SPOTTING_SERVICE_TOKEN` |
| AegisOps → GitHub API | `GITHUB_TOKEN` (PAT) |

Rotate tokens on a quarterly schedule. Never commit `.env` or PATs to git.

## Human approval gate

Telemetry bundles are exported to the IDE only after:

1. Dashboard **Start Fix** (`POST /api/v1/issues/:id/start-fix`), or
2. MCP tool `start_fix_from_issue` or `open_pr_from_issue`

GitHub **push** to `aegis/fix-<id>` auto-opens PRs when `GITHUB_APP_*` or `GITHUB_TOKEN` is configured.

## Context graph isolation (regulated customers)

Finance and healthcare tenants require that customer data never merge across org boundaries.

| Rule | Enforcement |
|------|-------------|
| Graph nodes/edges | Always keyed and filtered by `environment_id` (FK → project → tenant) |
| LLM Complete + `useGraph` | Subgraph retrieved **only** for the path environment; never another env |
| `projectId` on Complete | Server verifies env belongs to that project; otherwise **403** |
| Ingest / seed | Writes only into the target environment; no shared global graph |
| Cross-env edges | Rejected at store layer (`ErrGraphIsolation`) |

See also [`SAAS_ARCHITECTURE.md`](./SAAS_ARCHITECTURE.md) (Context graph section).

## Token rotation checklist

1. Generate new `AEGIS_WEBHOOK_SECRET` and update Copilot + Spotting `.env`
2. Rotate `GITHUB_WEBHOOK_SECRET` in GitHub webhook settings
3. Rotate `GITHUB_TOKEN` PAT with minimal scopes (`repo`, `issues`)
4. Restart AegisOps and sub-products
5. Run `scripts/e2e-smoke.ps1` to verify webhooks still flow
