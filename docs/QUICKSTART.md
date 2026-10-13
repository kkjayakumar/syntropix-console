# Syntropix Console — Community Edition quick start

Self-host on your machine or cluster. For the managed product, use [console.syntropix.io](https://console.syntropix.io). Overview: [syntropix.io](https://syntropix.io).

**Phase 1:** one **Syntropix** tenant, full closed loop, **Sentry** + **DevOps Copilot** (+ optional Spotting).

## Prerequisites

- Docker Desktop or Docker Engine + Compose v2
- GitHub repo for webhooks
- Optional: DevOps Copilot `:5000`, Spotting `:3000` on the host

## 1. Environment

```bash
cp .env.example .env
```

| Variable | Required | Notes |
|----------|----------|--------|
| `AEGISOPS_API_TOKEN` | Yes (prod) | Settings → API token (legacy env prefix) |
| `WORKSPACE_SECRETS_KEY` | Yes (prod) | `openssl rand -hex 32` |
| `AEGIS_WEBHOOK_SECRET` | Yes | Webhook HMAC shared secret |
| `AEGISOPS_PUBLIC_URL` | Self-host | `http://localhost:5001` (cloud uses `https://console.syntropix.io`) |
| `GITHUB_TOKEN` / GitHub App | For PR loop | See `.env.example` |

Compose sets `AEGISOPS_EDITION=community`.

## 2. Run

```bash
docker compose -f docker-compose.oss.yml up -d --build
curl -s http://localhost:5001/healthz
```

## 3. Console UI

1. http://localhost:5001  
2. **Settings** → API token  
3. **Workspace** → enable Source, CI/CD, Telemetry, IDE/MCP ([MCP.md](MCP.md))

## 4. Webhooks

Base URL = `AEGISOPS_PUBLIC_URL` (local or your reverse proxy).

| Source | Path |
|--------|------|
| DevOps Copilot | `POST /api/v1/webhooks/copilot` |
| Spotting | `POST /api/v1/webhooks/spotting` |
| GitHub | `POST /api/v1/github/webhook` |

```powershell
.\scripts\test-webhooks.ps1
```

## 5. MCP (Cursor)

See [MCP.md](MCP.md). Approve in console → `start_fix_from_issue` in IDE.

Roadmap: [OSS_ROADMAP.md](OSS_ROADMAP.md)
