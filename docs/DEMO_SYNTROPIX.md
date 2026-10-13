# Syntropix demo readiness — AegisOps closed loop

Embedded Syntropix console ships inside the AegisOps binary (`web/` → `embed/out`). One deploy serves API + UI on `:5001`.

## Closed loop (demo narrative)

```text
Create → Develop (Chat/MCP) → Commit (aegis/fix-*|feat-*) → PR
  → CI/CD monitor → Deploy (GHCR → EC2/SSM) → Monitor (Spotting/Copilot/healthz)
```

| Step | Where in UI | Backend |
|------|-------------|---------|
| Create | `/issues/new` or Chat | `POST /api/v1/issues` |
| Develop | Chat + MCP `start_fix_from_issue` | LLM API keys per env + MCP |
| Commit | Agent pushes branch | Convention `aegis/fix-{id}` |
| PR | Issues → Open PR / push webhook | `open-pr`, auto-PR |
| CI/CD | `/cicd` | `workflow_run` webhook |
| Deploy | GitHub Actions → SSM | `deploy-ec2.yml`, `deploy/ec2/deploy.sh` |
| Monitor | Spotting embed, Copilot, `/healthz` | Webhooks + dogfood |

## Industry encryption (secrets)

| Control | Implementation |
|---------|----------------|
| Algorithm | **AES-256-GCM** random nonce |
| Key derivation | 32-byte hex/base64 **or HKDF-SHA256** |
| Production | `WORKSPACE_SECRETS_KEY` required (no default) |
| List API | **Never decrypts** — fixed `••••••••` |
| Write API | Admin/maintainer only + audit (no values in audit) |
| LLM use | Decrypt in-process only; Google key via `x-goog-api-key` header |

Generate: `openssl rand -hex 32` → set `WORKSPACE_SECRETS_KEY` in `.env` and AWS Secrets Manager `dev/syntropix`.

## Preflight checklist

- [ ] `WORKSPACE_SECRETS_KEY` set (prod/staging)
- [ ] GitHub webhook events: `push`, `pull_request`, `workflow_run`
- [ ] `AEGISOPS_ALLOW_DOGFOOD=true` for live error → issue demo
- [ ] Spotting + Sentry DSN reachable from container
- [ ] Rebuild image so embedded UI matches (`docker compose up -d --build`)
- [ ] Project RBAC: demo user has `admin` or `developer` for chat
- [ ] Env secrets: `OPENAI_API_KEY` (or Claude/Google/Cursor) for Chat models

## Local demo (5 minutes)

1. Open http://localhost:5001 — closed-loop banner on Chat
2. Workspace → Secrets → add LLM key (name/value or JSON)
3. Chat → pick project `aegis` / env `dev` / model → ask
4. Issues → New feature → Start feature / Start fix
5. Push `aegis/fix-<id>` → watch `/cicd` after Actions run
6. Dogfood panel → trigger backend 500 → correlated issue

## Staging (Syntropix)

- URL: `https://aegisops.syntropix.io`
- Secrets Manager: `dev/syntropix` (include `WORKSPACE_SECRETS_KEY`)
- Deploy: merge `main` → CI → `deploy-ec2.yml` → EC2 SSM
