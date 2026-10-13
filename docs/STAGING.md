# Staging deployment (EC2)

Deploy the full AegisOps closed loop on a single EC2 instance with Docker Compose.

**Live staging:** https://aegisops.syntropix.io

## Stack

| Service | Port | Notes |
|---------|------|-------|
| AegisOps | 5001 | Orchestrator + embedded dashboard |
| DevOps Copilot | 5000 | Clone `devops-copilot-v2` on host |
| Spotting API | 3000 | Clone `spotting` on host |
| PostgreSQL | internal | AegisOps state (Docker volume) |
| Caddy | 443 | TLS reverse proxy → `:5001` |

## Phase 7 checklist

| Step | Task | Status |
|------|------|--------|
| 7.1 | EC2 + Docker | Done |
| 7.2 | Clone AegIsOps on host | Done |
| 7.3 | `deploy/staging/.env` with production secrets | Verify on EC2 |
| 7.4 | `bootstrap.sh` | Done |
| 7.5 | TLS (Caddy) | Done — `aegisops.syntropix.io` |
| 7.6 | Copilot + Spotting → staging webhooks | **Next** |
| 7.7 | GitHub webhook → staging URL | **Next** |
| 7.8 | `staging-smoke.ps1` from workstation | **Next** |

## Quick deploy

```bash
# On EC2 (Ubuntu 22.04+)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

git clone https://github.com/kkjayakumar/AegIsOps.git /opt/aegisops
cd /opt/aegisops
cp deploy/staging/.env.example deploy/staging/.env
# Edit .env: secrets, AEGISOPS_PUBLIC_URL, DEPLOY_VERIFY_URL

./deploy/staging/bootstrap.sh
```

## TLS (Caddy)

See `deploy/staging/Caddyfile.example`. Caddy must proxy **all** paths (`/`, `/api/*`, `/_next/*`) to `localhost:5001`.

```bash
sudo apt install -y caddy
sudo cp deploy/staging/Caddyfile.example /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## After AegisOps is up — wire the closed loop

### 1. GitHub webhook

| Field | Value |
|-------|-------|
| Payload URL | `https://aegisops.syntropix.io/api/v1/github/webhook` |
| Secret | `GITHUB_WEBHOOK_SECRET` from `deploy/staging/.env` |
| Events | Issues, Pull requests, Workflow runs |

https://github.com/kkjayakumar/AegIsOps/settings/hooks

### 2. Copilot on EC2

**One-command install** (reads `AEGIS_WEBHOOK_SECRET` from AegisOps staging `.env`):

```bash
cd /opt/aegisops
chmod +x deploy/staging/install-copilot-spotting.sh
sudo ./deploy/staging/install-copilot-spotting.sh
```

Or manual:

```bash
git clone https://github.com/kkjayakumar/devops-copilot-v2.git /opt/copilot
cd /opt/copilot
```

`.env`:

```env
AEGISOPS_WEBHOOK_URL=https://aegisops.syntropix.io/api/v1/webhooks/copilot
AEGISOPS_WEBHOOK_SECRET=<same as AEGIS_WEBHOOK_SECRET in deploy/staging/.env>
```

```bash
docker compose up -d --build
curl -s http://127.0.0.1:5000/api/ingest/config   # copy dsn → SENTRY_DSN in AegisOps .env
```

### 3. Spotting on EC2

Installed by `install-copilot-spotting.sh`, or manual:

```bash
git clone https://github.com/kkjayakumar/spotting.git /opt/spotting
cd /opt/spotting
```

`.env`:

```env
AEGISOPS_WEBHOOK_URL=https://aegisops.syntropix.io/api/v1/webhooks/spotting
AEGISOPS_WEBHOOK_SECRET=<same as AEGIS_WEBHOOK_SECRET>
CORS_ORIGINS=https://aegisops.syntropix.io
DOCKER_BIND_HOST=127.0.0.1
```

```bash
docker compose up -d --build
```

AegisOps already uses `host.docker.internal:5000` and `:3000` — works when Copilot/Spotting run on the same host.

**Browser capture** (dashboard Spotting SDK): expose Spotting API publicly, e.g. Caddy `spotting.syntropix.io` → `127.0.0.1:3000`, then set in AegisOps `.env`:

```env
NEXT_PUBLIC_SPOTTING_API_URL=https://spotting.syntropix.io
NEXT_PUBLIC_SPOTTING_PUBLIC_KEY=spk_live_...
```

Rebuild AegisOps after changing those build args.

### 4. Validate

**On EC2 (bash):**

```bash
cd /opt/aegisops
chmod +x scripts/setup-phase7-staging.sh
./scripts/setup-phase7-staging.sh https://aegisops.syntropix.io
```

**On Windows (PowerShell):**

```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\setup-phase7-staging.ps1 -BaseUrl https://aegisops.syntropix.io -SendTestIncident
.\scripts\staging-smoke.ps1 -BaseUrl https://aegisops.syntropix.io -Secret <AEGIS_WEBHOOK_SECRET>
```

## External database (Supabase)

If `DATABASE_URL` points to Supabase, use the external-db compose override:

```bash
cd /opt/aegisops/deploy/staging
docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops
```

Use session pooler port **5432** (not `6543` transaction mode) for goose migrations.

## `.env` secrets (common mistakes)

| Variable | Purpose | Wrong value |
|----------|---------|-------------|
| `GITHUB_WEBHOOK_SECRET` | GitHub webhook HMAC secret | `github_pat_...` (that belongs in `GITHUB_TOKEN`) |
| `GITHUB_TOKEN` | GitHub API PAT | Webhook secret |
| `AEGIS_WEBHOOK_SECRET` | Copilot/Spotting → AegisOps | — |

Generate: `openssl rand -hex 32` — use the same string in GitHub webhook **Secret** field.

Remove duplicate lines in `.env` (e.g. two `DEPLOY_VERIFY_URL` entries).

## Phase 8 — production hardening

After Phase 7 smoke passes:

```bash
cd /opt/aegisops && git pull
chmod +x scripts/setup-phase8-staging.sh deploy/staging/install-health-cron.sh
./scripts/setup-phase8-staging.sh https://aegisops.syntropix.io
```

From Windows (remote checks only):

```powershell
.\scripts\setup-phase8-staging.ps1 -BaseUrl https://aegisops.syntropix.io
```

**Priority tasks:** GitHub App (below), health cron (`sudo bash deploy/staging/install-health-cron.sh`), branch protection on `main`.

Full operations guide: `docs/RUNBOOK.md`

## Rollback

Set `ROLLBACK_WORKFLOW=rollback.yml` and `DEPLOY_VERIFY_URL=https://aegisops.syntropix.io/healthz` in staging `.env`.
Failed health checks dispatch the rollback workflow automatically.

## GitHub App (recommended for prod)

1. Create a GitHub App with Issues + Actions read/write
2. Install on your org/repo
3. Set `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY_PATH`
4. Remove `GITHUB_TOKEN` PAT when App auth is verified
