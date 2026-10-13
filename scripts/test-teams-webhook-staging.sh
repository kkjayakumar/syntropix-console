#!/usr/bin/env bash
# Test TEAMS_WEBHOOK_URL from EC2 staging .env (Incoming Webhook or Power Automate).
set -euo pipefail
ENV_FILE="${1:-/opt/aegisops/deploy/staging/.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "FAIL: missing $ENV_FILE"
  exit 1
fi
set -a
# shellcheck source=/dev/null
source "$ENV_FILE"
set +a
if [[ -z "${TEAMS_WEBHOOK_URL:-}" ]]; then
  echo "FAIL: TEAMS_WEBHOOK_URL not set in $ENV_FILE"
  exit 1
fi
BASE="${AEGISOPS_PUBLIC_URL:-https://aegisops.syntropix.io}"
BASE="${BASE%/}"
APPROVE="${BASE}/api/v1/gateway/approve?token=test&format=html"
REJECT="${BASE}/api/v1/gateway/reject?token=test&format=html"

if [[ "$TEAMS_WEBHOOK_URL" == *powerplatform* ]] || [[ "$TEAMS_WEBHOOK_URL" == *powerautomate* ]]; then
  BODY=$(jq -nc \
    --arg t "AegisOps gateway test" \
    --arg m "Power Automate trigger test from staging." \
    --arg a "$APPROVE" \
    --arg r "$REJECT" \
    '{kind:"approval",title:$t,message:$m,severity:"warning",approveUrl:$a,rejectUrl:$r}')
else
  BODY=$(jq -nc \
    --arg t "AegisOps gateway test" \
    --arg a "$APPROVE" \
    --arg r "$REJECT" \
    '{
      "@type":"MessageCard",
      "@context":"http://schema.org/extensions",
      "themeColor":"0078D7",
      "summary":$t,
      "sections":[{"activityTitle":$t,"text":"If you see this, Incoming Webhook works."}],
      "potentialAction":[
        {"@type":"OpenUri","name":"Approve Fix (sample)","targets":[{"os":"default","uri":$a}]},
        {"@type":"OpenUri","name":"Reject (sample)","targets":[{"os":"default","uri":$r}]}
      ]
    }')
fi

CODE=$(curl -s -o /tmp/aegis-teams-test.out -w "%{http_code}" -X POST "$TEAMS_WEBHOOK_URL" \
  -H "Content-Type: application/json" -d "$BODY")
echo "HTTP $CODE"
if [[ "$CODE" =~ ^(200|202|204|1)$ ]]; then
  echo "PASS: check your Teams channel for the test card"
  exit 0
fi
echo "FAIL response:"
cat /tmp/aegis-teams-test.out
exit 1
