# AegisOps Production Gap Checklist

Honest status for **multi-tenant SaaS + 7 pillars**, separate from the historical dogfood phases in [`PRODUCTION_READY.md`](./PRODUCTION_READY.md).

**Legend:** `[x]` done · `[~]` partial · `[ ]` missing · **P0** blocks trustworthy prod · **P1** needed soon · **P2** later

**Staging:** https://aegisops.syntropix.io · **Deploy:** push/`merge` to `main` → CI → `deploy-ec2.yml`

---

## Executive verdict

| Area | Reality |
|------|---------|
| Dogfood loop (detect → issue → MCP → PR → deploy → sentinel) | Works on staging with **global** env config |
| Workspace connectors UI | Seeded **disabled + empty** (honest). Apply-defaults + orchestrator prefer connectors for git/verify/telemetry URLs |
| Multi-project SaaS | Schema + UI exist; issues/CI/deploy still **global**, not project-scoped |
| Auth / RBAC | **API token gate** (`AEGISOPS_API_TOKEN`) in production; dashboard Settings pastes token. SSO/OIDC still future |
| Secrets | Encrypted at rest; write-only list — good foundation |

---

## 7 pillars (production bar)

| Pillar | Status | P | Gap |
|--------|--------|---|-----|
| **1 Smart IDE + MCP** | `[~]` | P0 | MCP tools work; `start-fix` mainly flips status — must respect `issue_agent` / MCP connector and return a real next step (or invoke API route) |
| **2 CI/CD** | `[~]` | P1 | Global GitHub webhooks/workflows OK for single-tenant dogfood; not project-scoped; live hooks must stay documented per env |
| **3 Telemetry** | `[~]` | P0 | Orchestrator prefers `telemetry.copilotApiUrl` / `spottingApiUrl` when connector ready; tokens stay global (`COPILOT_*` / `SPOTTING_*`) |
| **4 Router & dedup** | `[x]` | — | Correlation + fingerprint solid for current single-tenant scope |
| **5 Issue broker** | `[~]` | P0 | Creates issues with `GITHUB_DEFAULT_REPO`; should prefer workspace `git.repository` when connected |
| **6 Sentinel** | `[~]` | P0 | Verify URL from global env only; should prefer `deployment.verifyUrl` / `telemetry.healthUrl` from connectors |
| **7 Gateway (HITL)** | `[~]` | P0 | Tokens work; dashboard **Start Fix** can bypass approve. Need enforce flag + docs |

---

## SaaS workspace

| Item | Status | P | Notes |
|------|--------|---|-------|
| Tenant → Project → Environment | `[x]` | — | Bootstrap + CRUD |
| Connector templates (7 stages) | `[x]` | — | Disabled empty seed — **do not** re-enable fake-green migration |
| Secrets vault (AES-GCM) | `[x]` | — | No plaintext on list |
| **Apply platform defaults + enable** | `[~]`→`[x]` this session | P0 | Non-secret defaults from config; user-explicit connect |
| Orchestrator reads connectors | `[~]`→`[x]` | P0 | git + verify + telemetry URLs + **cicd rollback workflow** |
| Issues / deployments scoped to project | `[ ]` | P1 | Needs `project_id` / `environment_id` on issues |
| Context graph isolated per env (tenant→project→env) | `[x]` | P0 | Env-scoped tables + handler project check; no cross-env merge |
| Real auth (SSO / OIDC) | `[~]` | P0 | **API token middleware shipped**; SSO/OIDC later |
| RBAC beyond default-admin | `[~]` | P1 | Roles exist; token maps to `AEGISOPS_AUTH_USER_ID` (default admin) |
| Per-tenant encryption keys (KMS) | `[ ]` | P2 | |
| Billing / quotas | `[ ]` | P2 | Chat usage counter only |

---

## Orchestrator ↔ connectors (remaining)

| Runtime path | Prefer connector | Fallback |
|--------------|------------------|----------|
| GitHub issue create / PR | `git.repository`, `git.defaultBranch` | `GITHUB_DEFAULT_REPO` / branch |
| Deploy verify / sentinel | `deployment.verifyUrl` then `telemetry.healthUrl` | `DEPLOY_VERIFY_URL` |
| Copilot / Spotting clients | `telemetry.copilotApiUrl` / `spottingApiUrl` (URLs only; tokens global) | `COPILOT_*` / `SPOTTING_*` |
| Rollback workflow | `cicd.rollbackWorkflow` or workflows[] name containing `rollback` | `ROLLBACK_WORKFLOW` |
| start-fix agent route | `issue_agent.route` + provider | MCP-only message |
| Gateway required? | platform flag / future connector | optional today |

---

## P0 checklist (ship before calling it “prod SaaS”)

- [x] Honest connector seed (no fake Connected)
- [x] Non-secret platform defaults API + UI apply/connect
- [x] Orchestrator prefers workspace git + verify URLs + telemetry Copilot/Spotting API URLs (tokens remain global env)
- [x] start-fix returns connector-aware next steps + optional `AEGISOPS_REQUIRE_GATEWAY`
- [x] Real authentication (API token gate; no spoofable admin when enforced)
- [ ] `AEGISOPS_ENV=production` validation + secret rotation completed on host
- [ ] Staging host env includes `EC2_INSTANCE_ID` so deploy stage can Connect with defaults
- [ ] Issues cannot silently skip gateway when HITL is required for the tenant (flag exists; default off)

## P1

- [ ] Project-scoped issues / CI / deployments
- [x] Copilot/Spotting clients from telemetry connector (URLs; tokens global — residual: per-env secret vault tokens)
- [ ] GitHub App as primary auth (PAT optional)
- [ ] Branch protection + webhook delivery SLOs
- [ ] Managed Postgres / backups
- [x] Rollback workflow name from `cicd` connector

## P2

- [ ] Ledger / migration auditor
- [ ] Chaos + load tests
- [ ] Multi-region / KMS per tenant
- [ ] Customer onboarding UX

---

## Verify after deploy

```powershell
Invoke-RestMethod https://aegisops.syntropix.io/healthz
Invoke-RestMethod https://aegisops.syntropix.io/api/v1/workspace/connectors/defaults
# Apply defaults on an env (admin/maintainer):
# POST /api/v1/workspace/environments/{id}/connectors/apply-defaults
#   {"enable":true}
```

---

*Last updated: 2026-08-02*
