# Syntropix product naming

We use **Syntropix** only — not **AegisOps** in public branding, repos, or domains. That avoids confusion with unrelated “AegisOps” uses elsewhere.

## Three surfaces

| Surface | URL / location | Audience |
|---------|----------------|----------|
| **Marketing** | [syntropix.io](https://syntropix.io) | Product story, pricing, docs links |
| **Cloud console** | [console.syntropix.io](https://console.syntropix.io) | Managed multi-tenant (future / paid) |
| **Community (OSS)** | This repo — self-host on your infra | Single tenant, full loop, AGPL-3.0 |

Same product capabilities in Community and Cloud; Cloud adds hosting, SSO, billing, and multi-tenant ops.

## Names to use

| Concept | Public name | Notes |
|---------|-------------|--------|
| Company | **Syntropix** | License, GitHub, support |
| Self-host product | **Syntropix Console (Community Edition)** | README, releases |
| Cloud product | **Syntropix Console (Cloud)** | console.syntropix.io |
| GitHub repo | **`kkjayakumar/syntropix-console`** | Community OSS (override `GITHUB_OWNER` only for forks) |
| Default org in app | Tenant **Syntropix** (slug `syntropix`) | One workspace in Community Edition |

## Internal legacy identifiers (temporary)

The codebase still uses historical names (`aegisops` binary, `AEGISOPS_*` env vars, `aegis_*` tables). They are **implementation details**, not product names. A rename is planned; do not use “AegisOps” in new user-facing copy, domains, or repo titles.

## Domains (do not use for new docs)

| Deprecated | Use instead |
|------------|-------------|
| `aegisops.syntropix.io` | `console.syntropix.io` (cloud) or `localhost:5001` (self-host) |
| Public “AegisOps” repo branding | **Syntropix Console** |

## Community Edition tenant

Migrations seed a single organization: slug **`syntropix`**, display **Syntropix**. Community installs stay on that one tenant (Phase 1 roadmap).
