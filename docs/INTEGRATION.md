# AegisOps Integration Guide

Connect your product apps to the three-service ecosystem.

## Architecture

| Service | Port | Role |
|---------|------|------|
| DevOps Copilot | 5000 | Backend Sentry ingest, Docker logs, uptime |
| Spotting | 3000 | Frontend capture (console, network, session) |
| AegisOps | 5001 | Orchestrator — receives webhooks, correlates, creates issues |

## Backend (Node.js / Express)

Install Sentry SDK and point DSN to DevOps Copilot:

```bash
npm install @sentry/node
```

```javascript
import * as Sentry from "@sentry/node";

Sentry.init({
  dsn: process.env.SENTRY_DSN, // http://<key>@localhost:5000/<projectId>
  tracesSampleRate: 1.0,
});

// Tag errors with request id for correlation
Sentry.setTag("request_id", req.headers["x-request-id"]);
```

Get DSN from DevOps Copilot dashboard → Observability → Connect a Service.

### DevOps Copilot → AegisOps webhook

Set in DevOps Copilot `.env`:

```env
AEGISOPS_WEBHOOK_URL=http://localhost:5001/api/v1/webhooks/copilot
AEGISOPS_WEBHOOK_SECRET=dev-webhook-secret
```

## Frontend (Spotting embed)

```html
<script src="https://your-spotting-host/sdk.js"></script>
<script>
  Spotting.init({
    publicKey: "spk_live_...",
    apiBaseUrl: "http://localhost:3000",
  });
</script>
```

Ensure all API calls include `X-Request-Id` header — Spotting captures it as `correlationId`.

### Spotting → AegisOps webhook

Set in Spotting API `.env`:

```env
AEGISOPS_WEBHOOK_URL=http://localhost:5001/api/v1/webhooks/spotting
AEGISOPS_WEBHOOK_SECRET=dev-webhook-secret
```

## AegisOps

```env
PORT=5001
DATABASE_URL=postgres://aegisops:aegisops@localhost:5433/aegisops?sslmode=disable
AEGIS_WEBHOOK_SECRET=dev-webhook-secret
COPILOT_API_URL=http://localhost:5000
SPOTTING_API_URL=http://localhost:3000
```

## Webhook signature

All webhooks use HMAC-SHA256:

```
X-Aegis-Signature: sha256=<hex>
```

## Test webhooks

```powershell
# From AegisOps repo
./scripts/test-webhooks.ps1
```