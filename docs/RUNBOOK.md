# AegisOps Staging Runbook (EC2)

Operations guide for **https://aegisops.syntropix.io** and the closed loop on a single host.

## Stack layout

| Service | Path | Port | Public URL |
|---------|------|------|------------|
| AegisOps | `/opt/aegisops` | 5001 | https://aegisops.syntropix.io |
| DevOps Copilot | `/opt/devops-copilot` | 5000 | https://monitoring.syntropix.io |
| Spotting | `/opt/spotting` | 3000 / 3001 | https://api.spotting.syntropix.io / https://spotting.syntropix.io |

Database: Supabase PostgreSQL (`DATABASE_URL` in `deploy/staging/.env`).

## Restart services

```bash
# AegisOps (Supabase)
cd /opt/aegisops/deploy/staging
docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d aegisops

# Copilot
cd /opt/devops-copilot && docker compose up -d

# Spotting
cd /opt/spotting
docker compose -f docker-compose.yml \
  -f /opt/aegisops/deploy/staging/spotting-compose.staging.yml up -d
```

Full rebuild after code or embed env changes:

```bash
cd /opt/aegisops && git pull
cd deploy/staging
docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops
```

## Health checks

```bash
curl -s https://aegisops.syntropix.io/healthz
curl -s https://api.spotting.syntropix.io/healthz
curl -s http://127.0.0.1:5000/api/ingest/config
./scripts/setup-phase8-staging.sh https://aegisops.syntropix.io
```

Install hourly cron (from repo on EC2):

```bash
sudo bash /opt/aegisops/deploy/staging/install-health-cron.sh
```

## Secret rotation

1. Generate: `openssl rand -hex 32`
2. Update **same value** in:
   - `/opt/aegisops/deploy/staging/.env` → `AEGIS_WEBHOOK_SECRET`
   - `/opt/devops-copilot/.env` → `AEGISOPS_WEBHOOK_SECRET`
   - `/opt/spotting/.env` → `AEGISOPS_WEBHOOK_SECRET`
3. For GitHub webhooks: update `GITHUB_WEBHOOK_SECRET` in AegisOps `.env` **and** [GitHub hook settings](https://github.com/kkjayakumar/AegIsOps/settings/hooks)
4. Restart all three stacks (see above)
5. Verify from workstation:

```powershell
.\scripts\staging-smoke.ps1 -BaseUrl https://aegisops.syntropix.io `
  -Secret <AEGIS_WEBHOOK_SECRET> -GitHubSecret <GITHUB_WEBHOOK_SECRET>
```

## GitHub App (replace PAT)

1. Create app: https://github.com/settings/apps/new  
   Permissions: Issues R/W, Actions R/W, Pull requests R/W, Contents R  
   Events: Issues, Pull requests, Workflow runs  

2. Install on `kkjayakumar/AegIsOps`

3. On EC2:

```bash
mkdir -p /opt/aegisops/secrets && chmod 755 /opt/aegisops/secrets
nano /opt/aegisops/secrets/github-app.pem   # paste private key
chmod 644 /opt/aegisops/secrets/github-app.pem   # container runs as USER nobody — 600 blocks read
```

**Installation ID:** use the numeric id from `https://github.com/settings/installations/<id>` (e.g. `140713656`). **Not** the OAuth Client ID (`Iv23…`) shown on the App settings page.

4. In `/opt/aegisops/deploy/staging/.env`:

```env
GITHUB_APP_ID=<app id>
GITHUB_APP_INSTALLATION_ID=<installation id>
GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem
GITHUB_APP_SECRETS_DIR=/opt/aegisops/secrets
# remove GITHUB_TOKEN after verified
```

5. Redeploy (volume mount is in `docker-compose.external-db.yml`):

```bash
cd /opt/aegisops/deploy/staging
docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops
```

6. Trigger smoke incident; confirm GitHub issue created. Remove `GITHUB_TOKEN`.

## Sentinel / rollback

Requires in `.env`:

```env
DEPLOY_VERIFY_URL=https://aegisops.syntropix.io/healthz
ROLLBACK_WORKFLOW=rollback.yml
GITHUB_TOKEN=<pat or app>   # dispatches workflow
```

Failed post-deploy health check dispatches `rollback.yml` on GitHub.

## Nginx / TLS

Config: `/etc/nginx/sites-enabled/syntropix-multi.conf`  
Renew: `sudo certbot renew`

If HTTPS returns 502, ensure **443** server blocks include `proxy_pass` (not just HTTP).

## Logs

```bash
docker logs staging-aegisops-1 --tail 100
docker logs devops-copilot-app-1 --tail 50
docker logs spotting-api-1 --tail 50
docker logs spotting-web-1 --tail 50
sudo journalctl -u nginx -n 50
```

## Common issues

| Symptom | Fix |
|---------|-----|
| 502 on spotting.syntropix.io | Check `curl http://127.0.0.1:3001`; fix nginx 443 `proxy_pass` |
| Spotting login calls 127.0.0.1:3000 | Rebuild web with `NEXT_PUBLIC_SERVER_URL=https://api.spotting.syntropix.io` |
| GitHub webhook invalid signature | `GITHUB_WEBHOOK_SECRET` must match GitHub hook settings |
| Open PR / auto-PR disabled | `GITHUB_APP_INSTALLATION_ID` must be numeric installation id; PEM `chmod 644` for docker `nobody` user |
| Smoke test GitHub step fails | Pass `-GitHubSecret` (not Aegis secret) to `staging-smoke.ps1` |
| Duplicate incidents | Expected — fingerprint dedup merges into open issue; run `close-stale-inbox.ps1` |

## Phase 8 exit

```bash
cd /opt/aegisops
./scripts/setup-phase8-staging.sh https://aegisops.syntropix.io
```

All **FAIL** items resolved; **WARN** items (GitHub App, branch protection) addressed.

## Phase 9 — autonomous loop (staging)

Enable dogfood while keeping `AEGISOPS_ENV=production`:

```env
AEGISOPS_ALLOW_DOGFOOD=true
COPILOT_INGEST_PUBLIC_URL=https://monitoring.syntropix.io
SENTRY_DSN=http://...@monitoring.syntropix.io/...
```

Copilot’s Observability UI may still show `127.0.0.1:5000` in the DSN string — that is the bind address on the host. SDK clients (AegisOps Sentry SDK) must use **`monitoring.syntropix.io`**. On EC2:

```bash
bash /opt/aegisops/scripts/fix-staging-sentry-dsn.sh
cd /opt/aegisops/deploy/staging
docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops
```

Rebuild (bakes dashboard dogfood panel):

```bash
cd /opt/aegisops && git pull
chmod +x scripts/setup-phase9-staging.sh
./scripts/setup-phase9-staging.sh https://aegisops.syntropix.io
cd deploy/staging
docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops
```

From Windows:

```powershell
.\scripts\test-phase9-staging-detect.ps1 -BaseUrl https://aegisops.syntropix.io
```

Then in Cursor: `start_fix_from_issue` → push `aegis/fix-<id>` → **PR opens automatically** → merge → verify `fixed`.

**GitHub webhook** must include `workflow_run`, `pull_request`, and **`push`** events (see `docs/GITHUB_WEBHOOK.md`). Without `workflow_run`, the CI/CD page stays empty. Without `push`, PRs must be opened via dashboard **Open PR now** or MCP `open_pr_from_issue`.

**What “real detect” means:** `POST /api/v1/debug/trigger-error` sends Sentry → Copilot → AegisOps webhook (not `staging-smoke.ps1` HMAC simulation).

