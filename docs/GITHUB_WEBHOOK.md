# GitHub Webhook Setup (Phase 3)

Connect **AegIsOps** on GitHub to AegisOps so CI events, PR lifecycle, and issues flow automatically.

## Webhook URL

| Environment | Payload URL |
|-------------|-------------|
| Local (ngrok) | `https://<subdomain>.ngrok-free.app/api/v1/github/webhook` |
| Staging / prod | `https://<your-host>/api/v1/github/webhook` |

## GitHub repo settings

1. Open https://github.com/kkjayakumar/AegIsOps/settings/hooks
2. **Add webhook**
3. **Payload URL:** URL from table above
4. **Content type:** `application/json`
5. **Secret:** same value as `GITHUB_WEBHOOK_SECRET` in AegisOps `.env`
6. **Events:** select individual events:
   - Issues
   - Pull requests
   - **Pushes**
   - Workflow runs
7. **Active:** checked → **Add webhook**

Recent deliveries should show **200** responses.

## Local dev with ngrok

```powershell
# Terminal 1 — ensure AegisOps is up
cd c:\Users\kkjay\Documents\personal\AegisOps
docker compose up -d

# Terminal 2 — tunnel (install: winget install ngrok)
ngrok http 5001
```

Copy the `https://....ngrok-free.app` URL into GitHub webhook settings.

## Verify locally (no ngrok)

Signed test script mimics GitHub delivery:

```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\test-phase3-github.ps1
```

## What each event does

| Event | AegisOps action |
|-------|-----------------|
| `pull_request` opened | Issue → `pr_created` (branch `aegis/fix-<id>`) |
| `pull_request` closed + merged | Issue → `fixed` |
| `push` to `aegis/fix-*` or `aegis/feat-*` | Auto-opens PR if none exists → `pr_created` |
| `workflow_run` (any conclusion) | Recorded on **CI/CD** page + issue inspector |
| `workflow_run` failure | CI issue + deployment row |
| `workflow_run` success (deploy) | Sentinel health check + optional rollback |

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `401 invalid signature` | `GITHUB_WEBHOOK_SECRET` must match GitHub webhook secret exactly |
| No events received | Webhook URL must be public; localhost only works via ngrok |
| PR merge did not update issue | Branch must be `aegis/fix-<id>` or PR body `Fixes #<id>` |
| Issue #10 vs GitHub #10 | AegisOps issue ID ≠ GitHub issue number; use branch `aegis/fix-10` |

## Environment

Ensure Docker loads secrets:

```env
GITHUB_WEBHOOK_SECRET=<same-as-github-hook>
GITHUB_DEFAULT_REPO=kkjayakumar/AegIsOps
AEGISOPS_PUBLIC_URL=http://localhost:5001
DEPLOY_VERIFY_URL=http://localhost:5001/healthz
ROLLBACK_WORKFLOW=rollback.yml
```

Restart after changes: `docker compose up -d --force-recreate aegisops`
