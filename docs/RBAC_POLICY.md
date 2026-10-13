# AegisOps RBAC & SOC2 access policy

Multi-tenant SaaS access is **project-scoped**. Roles: `admin`, `maintainer`, `developer`, `readonly`.

## Role matrix

| Capability | Admin | Maintainer | Developer | Read-only |
|------------|:-----:|:----------:|:---------:|:---------:|
| View project / environments | ✓ | ✓ | ✓ | ✓ |
| Create / edit environments | ✓ | ✓ | ✗ | ✗ |
| Manage connectors | ✓ | ✓ | ✗ | ✗ |
| **Edit closed-loop pipeline** | ✓ | ✗ | ✗ | ✗ |
| View pipeline (read-only) | ✓ | ✓ | ✓ | ✓ |
| Write / rotate secrets (.ENV) | ✓ | ✓ | ✗ | ✗ |
| **Read secret plaintext values** | ✗ | ✗ | ✗ | ✗ |
| View secret names / metadata / masked | ✓ | ✓ | ✓ | ✓ |
| Manage env *variables* (non-secret) | ✓ | ✓ | view | view |
| LLM chat | ✓ | ✓ | ✓ (quota) | ✗ |
| Manage users / groups / RBAC | ✓ | ✗ | ✗ | ✗ |
| Issues / CI actions | ✓ | ✓ | ✓ | view |

## Policy notes (SOC2-oriented)

1. **HTTP API auth (baseline):** Production requires `AEGISOPS_API_TOKEN` (≥16 chars). Clients send `Authorization: Bearer <token>` or `X-AegisOps-Token`. When auth is enforced, `X-User-Id` / `X-User-Email` are **ignored** — actor is `AEGISOPS_AUTH_USER_ID` (default 1). Local/dev without a token keeps header fallback; set `AEGISOPS_ALLOW_INSECURE_AUTH=true` only for emergencies. Dashboard: Settings → Security → paste the same token (browser localStorage).
2. **Secrets never leave ciphertext** after write. APIs return name, provider, updatedAt, and mask only — never plaintext. Applies to every role including admin.
3. **Secret write** is limited to `admin` and `maintainer`. Deletes require the same.
4. **Pipeline** definitions are admin-editable only; other roles get view-only JSON/diagram.
5. **Chat quotas** are enforced per user+project (default developer limit). Admins/maintainers have higher or unlimited quotas.
6. **Groups** inherit the highest role when a user has both direct and group grants (least-privilege default is the *effective* role = max grant).
7. **Audit**: secret write/delete and RBAC changes are recorded in `aegis_audit_events` (who, what, project/env, timestamp). No secret values in logs.
8. **Separation**: each project + environment owns its own `.ENV` / secret namespace (AWS Secrets Manager style name→value + JSON bulk).

## Closed-loop pipeline

Per project environment: detect → correlate → approve → LLM/API or MCP → branch → PR → CI → deploy → verify → sentinel.

Only **admin** may change pipeline steps. Others may observe status for compliance visibility without mutating controls.
