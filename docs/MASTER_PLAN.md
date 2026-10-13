# AegisOps Unified Master Plan (The Parent Ecosystem)

> **Operational checklist:** follow phases one-by-one in [`PRODUCTION_READY.md`](./PRODUCTION_READY.md).

This is the master platform plan for **AegisOps**. **AegisOps** serves as the central orchestrator of the autonomous AI software engineering and DevOps lifecycle. Under this unified model, **Spotting** (Frontend) and **DevOps Copilot** (Backend) operate as modular sub-products, providing real-time telemetry inputs to close the loop.

---

## 🏗️ Core Architecture & The 7 Pillars

AegisOps bridges client-side, server-side, and pipeline environments into a single automated diagnostic and hotfix loop:

```
[Spotting SDK] (Frontend network/console errors) ──┐
                                                    ├──> [AegisOps Core Router] ──(Fingerprint)──> [Create GitHub Issue]
[Sentry SDKs] (Backend Exceptions & APM) ──────────┤        │ (Correlated via Request ID)                  │
                                                    │        ▼                                              ▼
[Docker Socket] (Container logs & metrics) ─────────┘   [MCP Server] <──(Read Context & Fix Code)── [Antigravity/Cursor]
                                                             │
                                                        (Deploy to EC2)
                                                             │
                                                             ▼
                                                        [Slack Gate] <──(Verify & Ask Approval)── [GitHub Action]
```

*   **Pillar 1: Smart IDE (Cursor/Antigravity + MCP):** Ingests live server telemetry dynamically into the AI workspace.
*   **Pillar 2: CI/CD Pipeline (GitHub & GitHub Actions):** Triggers automated branching, testing, and deployment (Docker / PM2).
*   **Pillar 3: Telemetry Engine (Spotting & DevOps Copilot):**
    - **Frontend (Spotting):** Session capture, console traces, client network requests.
    - **Backend (DevOps Copilot):** Sentry-compatible backend exception logging, container logs (2000 lines), and stats.
*   **Pillar 4: Intelligent Router & Deduplicator (The Filter):** Groups identical crashes via SHA-256 fingerprinting.
*   **Pillar 5: Contextual Issue Broker (The Orchestrator):** Auto-creates GitHub Issues loaded with full telemetry profiles (logs, traces, metrics, DB schema).
*   **Pillar 6: Post-Deployment Verification (Sentinel & synthetics):** Active synthetic uptime and health pings post-deploy.
*   **Pillar 7: Human-in-the-Loop Gateway (Gateway Bot):** Slack/Zulip hotfix approval buttons.

---

## Pillar implementation status (AegisOps core)

| Pillar | Scope | Status | Notes |
|--------|--------|--------|-------|
| **1 — Smart IDE + MCP** | Cursor/Antigravity reads telemetry via MCP | **Done (staging)** | 7 MCP tools; staging Supabase in `.cursor/mcp.json`; `runtimeEnabled` on deploy |
| **2 — CI/CD (GitHub Actions)** | Webhooks, PR lifecycle, deploy tracking | **Done (staging)** | `workflow_run` + push + PR webhooks; arm64 GHCR; SSM deploy; auto-PR on `aegis/fix-*` |
| **3 — Telemetry Engine** | Spotting + DevOps Copilot ingest | **Done (staging)** | Dogfood detect; correlated bundles; Sentry + Spotting on staging |
| **4 — Router & Deduplicator** | Correlation + fingerprint dedup | **Done** | Request-ID + SHA-256 fingerprint; close stale inbox to avoid bad dedup |
| **5 — Issue Broker** | GitHub issues + telemetry bundle | **Done (staging)** | Auto GitHub issues; markdown bundle; optional DB schema in bundle still pending |
| **6 — Sentinel** | Post-deploy health + rollback | **Done (staging)** | `verify_status=passed`; temp monitors; `rollback.yml` documents rollback |
| **7 — Gateway** | Approve/reject before fix | **Optional** | API + tokens work; Teams/Slack optional; dashboard Start Fix bypasses gate |

**Not started:** Phase 5 **Ledger** (migration auditor).

---

## 🗓️ Step-by-Step Ecosystem Roadmap

### 📦 Phase 1: Sub-Product Telemetry Alignment — **mostly complete**
- DevOps Copilot v2 Sentry ingest → AegisOps webhook (**done**, envelope path fixed)
- Spotting correlation schema + webhooks (**done**)
- Spotting capture key on AegisOps dashboard (**pending** — create key at Spotting Settings)

### 🧠 Phase 2: Router & Broker — **mostly complete**
- Correlation matcher + GitHub issue broker (**done**)
- Fingerprint deduplicator (**done**)

### 🚨 Phase 3: Sentinel — **partial**
- Post-deploy health check + rollback dispatch (**done** locally)
- EC2 staging + real deploy URL (**pending**)

### 🔒 Phase 4: Gateway — **partial**
- Approve/reject tokens + Teams/Slack cards (**done**)
- Valid Teams Incoming Webhook + live approval test (**pending**)

### 💾 Phase 5: Ledger — **not started**
