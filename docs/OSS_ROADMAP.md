# Syntropix Console — open-source roadmap (3 phases)

Community Edition = **one Syntropix tenant**, **full closed loop**, observability integrations added in phases.

Cloud (multi-tenant) ships at **console.syntropix.io**; marketing at **syntropix.io**.

## Phase 1 — Initial (v0.1)

Single-tenant self-host, existing workspace connectors, **Sentry** + **DevOps Copilot** (+ optional Spotting).

**Exit tests:** Install → incident → issue → gateway → MCP → PR.

## Phase 2 — Middle (v0.2)

Adapter layer + **one alternative** per class (e.g. Loki or Elasticsearch for logs; Grafana or Alertmanager for alerts).

## Phase 3 — Final (v1.0)

Multiple connectors (Datadog, New Relic, PostHog, Prometheus/Grafana/Loki/EFK, etc.).

## Editions

| | Community (this repo) | Cloud |
|--|----------------------|--------|
| Host | Your infra | console.syntropix.io |
| Tenants | 1 | Many |
| Auth | API token + RBAC | + SSO, billing |

Public branding: **Syntropix Console** only — not AegisOps ([NAMING.md](NAMING.md)).
