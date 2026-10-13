# Gateway & notifications (Phase 5)

Human-in-the-loop approval before an IDE fix starts. When AegisOps creates a correlated fix issue, it posts **Approve Fix** / **Reject** links to Teams and/or Slack.

## Flow

```
New incident → AegisOps issue (detected/correlated)
            → Gateway tokens (approve + reject, 72h TTL)
            → Teams MessageCard + Slack blocks
            → Human clicks Approve → issue status in_progress
            → Human clicks Reject  → issue status closed
```

**Dashboard Start Fix bypass:** By default `POST /api/v1/issues/{id}/start-fix` can move an issue to `in_progress` without gateway approval. Set `AEGISOPS_REQUIRE_GATEWAY=true` to reject start-fix until the issue is already `in_progress` (or later) via gateway approve.

Gateway URLs (HTML confirmation page):

- `GET /api/v1/gateway/approve?token=…&format=html`
- `GET /api/v1/gateway/reject?token=…&format=html`

JSON response when `Accept: application/json` or no `format=html`:

```json
{"issueId": 13, "action": "approve", "status": "ok"}
```

## Which URL to use

| URL type | Example | Works as `TEAMS_WEBHOOK_URL`? |
|----------|---------|--------------------------------|
| Teams channel/team link | `https://teams.microsoft.com/l/team/...` | **No** — browser link only |
| Incoming Webhook | `https://….webhook.office.com/webhookb2/…` | **Yes** — MessageCard POST |
| Power Automate HTTP trigger | `https://….powerplatform.com/.../workflows/.../triggers/manual/...` | **Yes** — JSON POST (202) |

## Power Automate (recommended if Incoming Webhook is disabled)

Use the **HTTP request** trigger URL from a flow that posts into your Teams channel.

```env
TEAMS_WEBHOOK_URL=https://….powerplatform.com/.../triggers/manual/paths/invoke?...&sig=...
```

AegisOps POSTs JSON (not MessageCard) for Power Automate URLs:

**Alert** (`kind: alert`):

```json
{ "kind": "alert", "title": "...", "message": "...", "severity": "warning" }
```

**Approval** (`kind: approval`):

```json
{
  "kind": "approval",
  "title": "AegisOps: approve fix?",
  "message": "...",
  "severity": "warning",
  "approveUrl": "http://localhost:5001/api/v1/gateway/approve?token=...&format=html",
  "rejectUrl": "http://localhost:5001/api/v1/gateway/reject?token=...&format=html"
}
```

`message` already includes markdown **Approve Fix** / **Reject** links. If your flow only shows plain text, use the raw URLs at the bottom of `message`, or map `actions[]` to Adaptive Card buttons:

| Field | Use in flow |
|-------|-------------|
| `triggerBody()?['title']` | Card title |
| `triggerBody()?['message']` | Card body (includes links) |
| `triggerBody()?['approveUrl']` | Open URL action "Approve Fix" |
| `triggerBody()?['rejectUrl']` | Open URL action "Reject" |
| `triggerBody()?['actions']` | Loop for button row (`title` + `url`) |

Test:

```powershell
.\scripts\setup-phase5-gateway.ps1 -SendTest
```

HTTP **202** means the flow accepted the request; confirm the message appears in Teams.

## Microsoft Teams Incoming Webhook

**Do not** use a channel URL like `https://teams.microsoft.com/l/channel/...` — that opens Teams in the browser and cannot receive POSTs.

1. Open the target channel in Teams.
2. Click **⋯** next to the channel name → **Connectors** (or **Workflows** on newer tenants).
3. Search **Incoming Webhook** → **Configure** → name it (e.g. `AegisOps`).
4. Copy the URL (`https://….webhook.office.com/webhookb2/…`).
5. Set in AegisOps `.env`:

```env
TEAMS_WEBHOOK_URL=https://outlook.webhook.office.com/webhookb2/...
AEGISOPS_PUBLIC_URL=http://localhost:5001
```

Restart AegisOps. On startup, a misconfigured channel link logs:

```
level=WARN msg="TEAMS_WEBHOOK_URL misconfigured" hint="this is a Teams channel link..."
```

## Slack Incoming Webhook

1. https://api.slack.com/apps → Create app → **Incoming Webhooks** → On.
2. **Add New Webhook to Workspace** → pick channel.
3. Copy URL (`https://hooks.slack.com/services/...`).

```env
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/T…/B…/…
```

## Public URL for link buttons

Approve/Reject links use `AEGISOPS_PUBLIC_URL`. For local dev, `http://localhost:5001` works on your machine. For the team to click links from Teams on other devices, use a tunnel or staging host (Phase 7):

```env
AEGISOPS_PUBLIC_URL=https://aegisops.staging.example.com
```

## Verify locally (no Teams click required)

```powershell
cd c:\Users\kkjay\Documents\personal\AegisOps
.\scripts\setup-phase5-gateway.ps1
.\scripts\test-phase5-gateway.ps1
```

Optional: send a real test card to Teams/Slack:

```powershell
.\scripts\setup-phase5-gateway.ps1 -SendTest
```

Then create an incident (`.\scripts\self-test-aegis.ps1`) and click **Approve Fix** in the channel.
