# AegisOps Master Plan: Autonomous AI-Ops Lifecycle Orchestration
> **Version:** 1.0 (Production Blueprint)  
> **Target Environment:** AWS EC2 (Docker + PM2)  
> **Core Architecture:** Closed-Loop AI-Native Software Engineering Operations  

AegisOps is an in-house, fully autonomous software maintenance, troubleshooting, and self-healing engine. By combining custom frontend/backend telemetry with an Agentic IDE workspace via Model Context Protocol (MCP), AegisOps intercepts production anomalies, performs automated Root Cause Analysis (RCA), writes hotfixes, routes approvals, and validates deployments.

---

## 🏗️ The 7 Pillars Architecture

### 1. Smart IDE Workspace (Cursor / Antigravity) `[🟢 COMPLETED]`
- **Role:** Autonomous agent workspace utilizing generative AI to implement hotfixes.
- **Integration:** Plugs directly into the **Synapse MCP Server** to parse runtime execution contexts on-demand.

### 2. Version Control & CI/CD Pipeline (GitHub Actions) `[🟢 COMPLETED]`
- **Role:** Automated branch isolation (`aegis/fix-*`), Pull Request generation, and continuous deployment straight to target AWS EC2 instances utilizing `docker-compose --build` and PM2 process reloads.

### 3. Frontend Telemetry Engine (Spotting) `[🟢 COMPLETED - LIVE IN PROD]`
- **Licensing:** Fully Licensed AGPL 3.0.
- **Core Features:** 
  - **Fidelity Screen Recording:** High-precision video capture via native browser `MediaRecorder` & `getDisplayMedia` API (Superior to RRWeb).
  - **Interception Engines:** Full `Fetch`/`XHR` network call capturing (Requests, Responses, and 400/500 Status Codes) along with complete browser console stack traces.
  - **Telemetry Metadata:** Auto-tagging of OS, Browser User-Agent, Window Resolution, and precise timestamps.
  - **Jam-Style Export:** Seamless S3/MinIO bucket report uploading with synchronized playback layouts.

### 4. Backend & DB Telemetry (DevOps Copilot Core) `[🟡 IN PROGRESS]`
- **Tech Stack:** Vite + React (Frontend), Express Server (Backend - Port 5000), Local MemStorage Fallback (Runs flawlessly without requiring an external database).
- **Core Features:** 
  - Real-time tracking of container statuses, logs, and live system resource constraints.
  - In-house replacement for GlitchTip/Sentry built straight into the Express core to trap Node.js exceptions and Prisma ORM/DB constraints.
  - Integration with `/var/run/docker.sock` to dynamically isolate **2,000 lines of live logs** and **10 minutes of active container metrics** at the exact millisecond of failure.

### 5. Contextual Issue Broker & Synapse MCP Server `[🔴 PENDING]`
- **Role:** The orchestration gateway that merges `Spotting` frontend metrics with `DevOps Copilot` backend states.
- **Deduplication:** Hash-based log deduplication using MD5 checksums of stack traces to avoid AI context exhaustion.
- **MCP Protocols:** An internal server exposing Python-based MCP SDK tools (`fetch_incident_context`, `check_host_infra_status`) directly to Cursor.

### 6. Post-Deployment Verification Suite `[🔴 PENDING]`
- **Role:** Automated validation gateway using headless testing (Playwright or programmatic `curl` scripts) triggered instantly post-deployment to ensure system readiness or trigger automated image rollbacks.

### 7. Human-in-the-Loop Governance Gateway `[🔴 PENDING]`
- **Role:** ChatOps security compliance layer. Routes interactive `[Approve Hotfix]` / `[Reject]` webhook cards via Slack/Zulip before code touches the production server.

---

## 📅 Phased Implementation Timeline (Est. 120 - 145 Total Engineering Hours)

### Phase 1: In-House Backend Telemetry Core (`DevOps Copilot` Upgrade)
*Estimated Duration: 25 - 30 Hours*
- [ ] Implement global Express exception handlers and Prisma Error Parsers in the core backend.
- [ ] Build the **Log Merger Module**: Write an asynchronous routine to fetch 2k lines from `docker.sock` and grab 10-minute snapshot trends of container RAM/CPU whenever an exception triggers.
- [ ] Write the **MD5 Deduplicator**: Map incoming errors against an internal `MemStorage` schema to throttle redundant log events.

### Phase 2: Synapse MCP Layer & GitHub Broker Integration
*Estimated Duration: 35 - 40 Hours*
- [ ] **GitHub Issue Automated Webhook:** Write a module inside `DevOps Copilot` to construct a markdown report bundle and push it to GitHub via the REST API, tagging it as `aegisops-pending`.
- [ ] **Synapse MCP Server Deployment:** Build the Python/Node-based MCP endpoint structure to pipe isolated error snapshots directly into Cursor upon request.

### Phase 3: Hotfix Delivery, ChatOps, and Validation Hooks
*Estimated Duration: 45 - 55 Hours*
- [ ] **Slack/Zulip ChatOps Pipeline:** Develop an interactive endpoint to dispatch interactive webhook actions to the team communications server when a patch PR is proposed by the agent.
- [ ] **Post-Deployment Verification Hook:** Code a lightweight `smoke-test.sh` tool inside GitHub Actions to hit endpoints like `/api/healthz` and verify core functionality.
- [ ] **Auto-Close & Resiliency Rollback Engine:** Configure the automated workflow to drop the GitHub issue upon test success or fire up a quick image version rollback upon test failure.

### Phase 4: Chaos Testing & Optimization
*Estimated Duration: 15 - 20 Hours*
- [ ] Simulate broken DB schemas and fake 500 exceptions inside the Staging EC2 environment.
- [ ] Verify that the complete loop executes from detection to automated fix without code breaking down or dropping logs.

---

## 📊 Target Master Error Payload Schema (The Unified Context)

When an error happens, AegisOps generates this single standard object to feed into the AI Agentic IDE:

```json
{
  "incidentId": "aegis_err_1717753386",
  "environment": "production",
  "component": "node-backend-service",
  "errorSummary": {
    "type": "PrismaClientKnownRequestError",
    "code": "P2002",
    "message": "Unique constraint failed on the fields: (`email`)"
  },
  "telemetryLinks": {
    "frontendSpottingLink": "[https://spotting.sheshi.ai/s/unq_session_8891](https://spotting.sheshi.ai/s/unq_session_8891)",
    "hostNodeExporterUrl": "http://localhost:9100/metrics"
  },
  "contextSnapshot": {
    "stackTrace": "at PrismaClient.Users.create... src/controllers/auth.ts:42:18",
    "containerMetrics10Min": {
      "averageCpuUsage": "44.2%",
      "peakMemoryUsageMb": "512MB",
      "networkThrottled": false
    },
    "hostInfraState": {
      "systemCpu": "12%",
      "availableMemory": "4.2GB",
      "diskStorageRemaining": "14.8GB"
    },
    "rawDockerLogs2000Lines": "[Standard Output Logs Stream Dump here...]"
  }
}