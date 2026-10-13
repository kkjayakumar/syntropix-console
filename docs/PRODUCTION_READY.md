# AegisOps Production Readiness Plan

> **Current SaaS / pillar gaps:** see [`PRODUCTION_GAP.md`](./PRODUCTION_GAP.md) (honest P0–P2 checklist). This file tracks the historical dogfood phase ladder.

Follow phases **in order**. Do not skip ahead until the **Exit criteria** for the current phase passes.

**Legend:** `[x]` done · `[~]` partial · `[ ]` not started

**Quick links:** [Dashboard](http://localhost:5001) · [Copilot](http://localhost:5000) · [Spotting](http://127.0.0.1:3001) · [Staging](https://aegisops.syntropix.io)

---

## Progress overview

| Phase | Name | Status | Notes |
|-------|------|--------|-------|
| **0** | Local stack & secrets | `[x]` | Rotate PAT still recommended |
| **1** | Webhook closed loop | `[x]` | `self-test-aegis.ps1` passes |
| **2** | IDE + MCP fix flow | `[x]` | PR #4 merged; issue #10 → `fixed` |
| **3** | GitHub CI/CD integration | `[x]` | Local webhooks + GHCR CI; live hook → Phase 7 |
| **4** | Dogfood telemetry on AegisOps | `[x]` | Dogfood panel + Sentry + Spotting embed |
| **5** | Gateway & notifications | `[x]` | UI + gateway tokens; Power Automate optional |
| **6** | Sentinel & rollback | `[x]` | `test-phase6-sentinel.ps1` passes |
| **7** | Staging (EC2) | `[x]` | Live — smoke tests pass |
| **8** | Production hardening | `[x]` | setup-phase8-staging.sh passes |
| **9** | Ledger & chaos validation | `[~]` | Dogfood detect on staging |

---

## Phase 0 — Local stack & secrets

**Goal:** All three services run locally with aligned secrets.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 0.1 | AegisOps `.env` complete (`GITHUB_TOKEN`, `GITHUB_DEFAULT_REPO`, webhook secrets) | `[x]` | |
| 0.2 | Docker Compose loads `.env` for AegisOps | `[x]` | |
| 0.3 | Copilot `docker-compose` passes `AEGISOPS_*` via `env_file` | `[x]` | |
| 0.4 | Spotting `.env` has `AEGISOPS_WEBHOOK_URL` + secret | `[x]` | |
| 0.5 | All stacks running (`:5001`, `:5000`, `:3000`) | `[x]` | |
| 0.6 | Rotate exposed secrets (GitHub PAT, old git history) | `[ ]` | Security |
| 0.7 | Commit uncommitted repo changes (docker-compose, migrations, docs) | `[~]` | Phase 3–4 files pending commit |

**Verify**
```powershell
docker ps --format "table {{.Names}}\t{{.Ports}}"
Invoke-RestMethod http://localhost:5001/healthz
Invoke-RestMethod http://localhost:5000/api/ingest/config
Invoke-RestMethod http://localhost:3000/healthz
```

**Exit criteria:** All three health checks return OK.

---

## Phase 1 — Webhook closed loop

**Goal:** Copilot + Spotting → AegisOps → correlated issue → GitHub issue.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 1.1 | Copilot Sentry envelope notifies AegisOps | `[x]` | Fixed in `routes.ts` |
| 1.2 | Shared `AEGIS_WEBHOOK_SECRET` across 3 services | `[x]` | |
| 1.3 | Request-ID correlation (Copilot ↔ Spotting) | `[x]` | |
| 1.4 | SHA-256 fingerprint dedup | `[x]` | Migration `003_fingerprint` |
| 1.5 | Auto GitHub issue on incident | `[x]` | Issues + Actions PAT perms |
| 1.6 | Telemetry markdown bundle | `[x]` | Copilot logs + Spotting network |
| 1.7 | Run self-test end-to-end | `[x]` | `scripts/self-test-aegis.ps1` |
| 1.8 | Run full e2e smoke (PR lifecycle) | `[x]` | `test-phase3-github.ps1` covers PR path |

**Verify**
```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\self-test-aegis.ps1
.\scripts\test-phase3-github.ps1
```

**Exit criteria:** Self-test + phase-3 webhook tests pass; issue visible at http://localhost:5001/issues and on GitHub.

---

## Phase 2 — IDE + MCP fix flow

**Goal:** Developer approves fix in dashboard; Cursor agent reads bundle via MCP and implements.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 2.1 | Build MCP binary | `[x]` | `make build` → `bin/aegisops-mcp.exe` |
| 2.2 | `.cursor/mcp.json` configured | `[x]` | Project-level |
| 2.3 | Enable MCP server in Cursor (reload / Settings → MCP) | `[x]` | aegisops running |
| 2.4 | Dashboard **Start Fix** marks issue `in_progress` | `[x]` | Verified on issue #10 |
| 2.5 | MCP `start_fix_from_issue` returns bundle | `[x]` | Verified on issue #10 |
| 2.6 | MCP `get_live_logs` / `get_sentry_issue` against Copilot | `[x]` | `test-phase2-mcp.ps1` |
| 2.7 | Agent opens PR branch `aegis/fix-<id>` | `[x]` | PR #4 merged |
| 2.8 | PR webhook updates issue → `pr_created` → `fixed` | `[x]` | Issue #10 → `fixed` |

**Verify**
```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\test-phase2-mcp.ps1
```

**Exit criteria:** MCP tools respond; one issue driven from `detected` → `in_progress` → `fixed` via MCP + PR.

---

## Phase 3 — GitHub CI/CD integration

**Goal:** Real GitHub events flow into AegisOps (not just simulated webhooks).

| # | Task | Status | Notes |
|---|------|--------|-------|
| 3.1 | Fine-grained PAT (Issues + Actions on target repo) | `[x]` | |
| 3.2 | Add repo webhook → `/api/v1/github/webhook` | `[~]` | Deferred to Phase 7 staging; use ngrok locally |
| 3.3 | Webhook secret matches `GITHUB_WEBHOOK_SECRET` | `[x]` | In `.env` |
| 3.4 | Events: Issues, Pull requests, Workflow runs | `[~]` | Local simulation via `test-phase3-github.ps1` |
| 3.5 | CI workflow: test + publish Docker image to GHCR | `[x]` | See `docs/CI_CD.md` |
| 3.6 | Failed workflow creates CI issue in AegisOps | `[x]` | `test-phase3-github.ps1` |
| 3.7 | Successful deploy triggers Sentinel path | `[x]` | Local deploy workflow_run test |
| 3.8 | `rollback.yml` workflow_dispatch stub | `[x]` | Replace with real rollback job |

**Verify**
```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\test-phase3-github.ps1
```
Live hook setup: [`docs/GITHUB_WEBHOOK.md`](./GITHUB_WEBHOOK.md) (ngrok for local).

**Exit criteria:** CI config on `main`; local webhook simulation passes. Live GitHub delivery optional until staging (Phase 7).

---

## Phase 4 — Dogfood telemetry on AegisOps

**Goal:** Monitor AegisOps itself with Spotting + Copilot (not just synthetic scripts).

| # | Task | Status | Notes |
|---|------|--------|-------|
| 4.1 | Copilot project/DNS for AegisOps backend | `[x]` | `SENTRY_DSN` from `/api/ingest/config` |
| 4.2 | Dashboard sends `X-Request-Id` on API calls | `[x]` | `CorrelationBridge` |
| 4.3 | Create Spotting public key (origin `http://localhost:5001`) | `[x]` | `setup-phase4-dogfood.ps1` |
| 4.4 | Embed Spotting SDK on AegisOps dashboard | `[x]` | `SpottingCapture` + `web/public/spotting-capture.js` |
| 4.5 | Optional: Sentry Go SDK on `cmd/aegisops` → Copilot DSN | `[x]` | `SENTRY_DSN` + `internal/telemetry` |
| 4.6 | Trigger real UI error → correlated issue (no script) | `[x]` | Dashboard **Dogfood panel** + `debug/trigger-error` |
| 4.7 | Bundle includes live Copilot logs (2000 lines path) | `[~]` | Via Copilot client; expand in broker |

**Verify**
```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\setup-phase4-dogfood.ps1
make web-build
docker compose up -d --build aegisops   # or go run ./cmd/aegisops
.\scripts\test-phase4-dogfood.ps1
```
Then open http://localhost:5001 → **Phase 4 — Dogfood telemetry** → **Trigger backend 500**.

**Exit criteria:** One real (non-script) incident from the AegisOps UI with correlated Spotting + Copilot telemetry.

---

## Phase 5 — Gateway & notifications

**Goal:** Team gets approve/reject cards; human gate before IDE fix.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 5.1 | Gateway approve/reject token URLs | `[x]` | `/api/v1/gateway/approve|reject` |
| 5.2 | Teams MessageCard on new issue | `[x]` | `SendApprovalCard` with OpenUri buttons |
| 5.3 | Replace `TEAMS_WEBHOOK_URL` with Incoming Webhook | `[x]` | Staging Incoming Webhook configured |
| 5.4 | Slack notifier wired | `[x]` | Set `SLACK_WEBHOOK_URL` |
| 5.5 | Test Approve → issue `in_progress` | `[x]` | `test-phase5-gateway.ps1` |
| 5.6 | Test Reject → issue `closed` | `[x]` | `test-phase5-gateway.ps1` |
| 5.7 | `AEGISOPS_PUBLIC_URL` reachable by team (not localhost) | `[~]` | OK locally; staging in Phase 7 |

**Verify**
```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\setup-phase5-gateway.ps1
.\scripts\test-phase5-gateway.ps1
# Optional live Teams card:
.\scripts\setup-phase5-gateway.ps1 -SendTest
.\scripts\self-test-aegis.ps1   # then click Approve in Teams channel
```

**Exit criteria:** Approve link from notification updates issue status without using dashboard.

---

## Phase 6 — Sentinel & rollback

**Goal:** Post-deploy health fails → alert + rollback workflow dispatch.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 6.1 | `DEPLOY_VERIFY_URL` set | `[x]` | Local: `/healthz` |
| 6.2 | `ROLLBACK_WORKFLOW=rollback.yml` | `[x]` | |
| 6.3 | Sentinel worker + temp Copilot monitors | `[x]` | |
| 6.4 | Health fail triggers rollback dispatch | `[x]` | Verified via `test-phase6-sentinel.ps1` |
| 6.5 | Implement real rollback job in `rollback.yml` | `[x]` | GHCR tag resolve + manifest verify |
| 6.6 | Sentinel cleanup after TTL | `[x]` | Worker in `main.go` |

**Verify**
```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\test-phase6-sentinel.ps1
```
Push `rollback.yml` to `main` first so GitHub Actions can receive the dispatch.

**Exit criteria:** Simulated failed health check dispatches `rollback` workflow on GitHub.

---

## Phase 7 — Staging (EC2)

**Goal:** Full stack on a public host; smoke tests from workstation.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 7.1 | EC2 instance (Ubuntu 22.04+, Docker) | `[ ]` | |
| 7.2 | Clone AegIsOps + Copilot + Spotting on host | `[ ]` | See `docs/STAGING.md` |
| 7.3 | `deploy/staging/.env` with production-like secrets | `[ ]` | |
| 7.4 | Run `deploy/staging/bootstrap.sh` | `[ ]` | |
| 7.5 | TLS / reverse proxy (Caddy or nginx) | `[ ]` | |
| 7.6 | Update Copilot/Spotting webhooks to staging AegisOps URL | `[ ]` | |
| 7.7 | GitHub webhook → staging public URL | `[ ]` | Completes Phase 3.2 / 3.4 |
| 7.8 | `scripts/staging-smoke.ps1 -BaseUrl https://...` | `[ ]` | |

**Verify**
```powershell
.\scripts\staging-smoke.ps1 -BaseUrl https://aegisops.staging.example.com
```

**Exit criteria:** Staging smoke passes against public URL.

---

## Phase 8 — Production hardening

**Goal:** Secure, observable, maintainable production deployment.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 8.1 | `AEGISOPS_ENV=production` | `[ ]` | Enforces non-default webhook secrets |
| 8.2 | Rotate all secrets (webhook, PAT, DB, Teams, Slack) | `[ ]` | |
| 8.3 | Replace PAT with GitHub App | `[ ]` | `docs/STAGING.md` |
| 8.4 | Postgres: managed DB or encrypted backups | `[ ]` | Not `aegisops/aegisops` in prod |
| 8.5 | Rate limits + TLS on all public endpoints | `[~]` | Rate limit exists |
| 8.6 | `.env` never in git; use secrets manager on host | `[x]` | `.gitignore` |
| 8.7 | Monitoring: health + disk + container alerts | `[ ]` | Via Copilot or external |
| 8.8 | Document runbook (restart, rollback, rotate) | `[x]` | `docs/RUNBOOK.md` |
| 8.9 | CI green on `main`; branch protection | `[~]` | Enable on GitHub; CI workflow in repo |

**Exit criteria:** Production checklist in `docs/SECURITY.md` satisfied; no default secrets.

---

## Phase 9 — Ledger & chaos validation

**Goal:** Migration auditor + prove full loop under failure.

| # | Task | Status | Notes |
|---|------|--------|-------|
| 9.1 | Ledger schema (migration audit log) | `[ ]` | Phase 5 in MASTER_PLAN |
| 9.2 | Goose migration drift detector | `[ ]` | |
| 9.3 | Chaos: break DB schema in staging | `[ ]` | |
| 9.4 | Chaos: inject 500 errors | `[ ]` | |
| 9.5 | End-to-end: detect → approve → MCP fix → PR → deploy → sentinel | `[x]` | Staging dogfood loop verified (issue #18, `req_phase9_*`) |
| 9.6 | Load test webhook endpoints | `[ ]` | |

**Exit criteria:** Full loop completes on staging without manual webhook simulation.

---

## 7 pillars → phase map

| Pillar | Primary phases |
|--------|----------------|
| 1 Smart IDE + MCP | 2 |
| 2 CI/CD GitHub Actions | 3, 7 |
| 3 Telemetry (Spotting + Copilot) | 1, 4 |
| 4 Router & deduplicator | 1 |
| 5 Issue broker | 1, 4 |
| 6 Sentinel | 6, 7 |
| 7 Gateway (HITL) | 5 |

---

## What to do next (start here)

You are **in Phase 9** — prove the full autonomous loop on staging.

| Step | Action |
|------|--------|
| **Done** | Phase 8 — production hardening checks pass |
| **Done** | Phase 9.5 — full autonomous loop on staging |
| **Done** | Seven pillars aligned on staging (Teams optional) |
| **Now** | Inbox hygiene (`close-stale-inbox.ps1`), secret rotation checklist, GitHub hardening |
| **Later** | Phase 9.1–9.2 ledger; chaos / load tests |

Update this file as you complete items: change `[ ]` → `[x]` and phase status in the overview table.

---

## Script reference

| Script | Purpose |
|--------|---------|
| `scripts/setup-phase4-dogfood.ps1` | Phase 4: DSN + Spotting key in `.env` |
| `scripts/test-phase4-dogfood.ps1` | Phase 4: debug endpoint + inbox check |
| `scripts/setup-phase5-gateway.ps1` | Phase 5: validate Teams/Slack URLs; optional `-SendTest` |
| `scripts/test-phase5-gateway.ps1` | Phase 5: approve → `in_progress`, reject → `closed` |
| `scripts/test-phase6-sentinel.ps1` | Phase 6: failed deploy verify → rollback dispatch |
| `scripts/update-from-ghcr.ps1` | Pull GHCR image and restart app locally |
| `scripts/test-phase2-mcp.ps1` | Phase 2: Start Fix, bundle, Copilot MCP proxies |
| `scripts/test-phase3-github.ps1` | Phase 3: signed PR + workflow_run webhooks |
| `scripts/self-test-aegis.ps1` | Dogfood: Copilot envelope + Spotting → AegisOps |
| `scripts/e2e-smoke.ps1` | Full loop incl. PR lifecycle + deploy webhook |
| `scripts/setup-phase8-staging.sh` | Phase 8: production hardening audit |
| `scripts/setup-phase9-staging.sh` | Phase 9: dogfood + stack prerequisites |
| `scripts/test-phase9-staging-detect.ps1` | Phase 9: real detect + Start Fix (no staging-smoke) |
| `scripts/close-stale-inbox.ps1` | Close stale in_progress/correlated issues (dogfood hygiene) |
| `scripts/security-rotation-checklist.ps1` | Ordered secret rotation steps after exposure |
| `scripts/verify-github-hardening.ps1` | Branch protection + webhook + runtime GitHub checks |
| `scripts/verify-github-webhook.ps1` | Phase 8/9: verify workflow_run + push → CI/CD + auto PR |

---

*Last updated: 2026-06-07 · Repo: `kkjayakumar/AegIsOps`*
