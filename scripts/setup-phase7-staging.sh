#!/usr/bin/env bash
# Phase 7 staging checks — run on EC2 (bash), not PowerShell.
set -euo pipefail

BASE_URL="${1:-https://aegisops.syntropix.io}"
AEGIS_SECRET="${AEGIS_WEBHOOK_SECRET:-}"
GITHUB_SECRET="${GITHUB_WEBHOOK_SECRET:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${ENV_FILE:-$(dirname "$SCRIPT_DIR")/deploy/staging/.env}"

if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  set -a
  source <(grep -E '^(AEGIS_WEBHOOK_SECRET|GITHUB_WEBHOOK_SECRET)=' "$ENV_FILE" | sed 's/\r$//')
  set +a
  [[ -z "$AEGIS_SECRET" ]] && AEGIS_SECRET="${AEGIS_WEBHOOK_SECRET:-}"
  [[ -z "$GITHUB_SECRET" ]] && GITHUB_SECRET="${GITHUB_WEBHOOK_SECRET:-}"
fi

echo "=== Phase 7: Staging setup check ==="
echo "Base URL: $BASE_URL"

echo ""
echo "--- API health ---"
health="$(curl -sf "$BASE_URL/healthz")"
echo "$health"
echo "$health" | grep -q '"status":"ok"' || { echo "FAIL healthz"; exit 1; }
echo "PASS healthz"

stats="$(curl -sf "$BASE_URL/api/v1/stats")"
echo "Stats: $stats"

sign_hmac() {
  local body="$1" secret="$2"
  printf '%s' "$body" | openssl dgst -sha256 -hmac "$secret" | awk '{print $2}'
}

if [[ -n "$AEGIS_SECRET" ]]; then
  echo ""
  echo "--- Test incident (Copilot + Spotting) ---"
  cid="req_staging_$(openssl rand -hex 5)"
  copilot_body=$(printf '{"event":"sentry.issue.created","correlationId":"%s","issue":{"id":9001,"title":"Staging smoke test","level":"error","count":1}}' "$cid")
  sig=$(sign_hmac "$copilot_body" "$AEGIS_SECRET")
  curl -sf -X POST "$BASE_URL/api/v1/webhooks/copilot" \
    -H "Content-Type: application/json" \
    -H "X-Aegis-Signature: sha256=$sig" \
    -d "$copilot_body" >/dev/null

  sleep 1
  spot_body=$(printf '{"event":"spotting.report.created","correlationId":"%s","reportId":"rpt_staging","severity":"error","pageUrl":"%s/","networkFailures":[{"method":"GET","url":"/smoke","status":500}]}' "$cid" "$BASE_URL")
  sig2=$(sign_hmac "$spot_body" "$AEGIS_SECRET")
  curl -sf -X POST "$BASE_URL/api/v1/webhooks/spotting" \
    -H "Content-Type: application/json" \
    -H "X-Aegis-Signature: sha256=$sig2" \
    -d "$spot_body" >/dev/null

  sleep 3
  issues="$(curl -sf "$BASE_URL/api/v1/issues?type=fix")"
  if echo "$issues" | grep -q "$cid"; then
    echo "PASS correlated issue created for $cid"
  else
    echo "WARN no issue for $cid — check AEGIS_WEBHOOK_SECRET and logs"
  fi
else
  echo ""
  echo "Skip webhook test (set AEGIS_WEBHOOK_SECRET in $ENV_FILE)"
fi

if [[ -n "$GITHUB_SECRET" ]]; then
  echo ""
  echo "--- GitHub webhook simulation ---"
  if [[ "$GITHUB_SECRET" == github_pat_* ]]; then
    echo "ERROR: GITHUB_WEBHOOK_SECRET looks like a PAT — use a random secret, not GITHUB_TOKEN"
    echo "  Fix: openssl rand -hex 32  →  put in .env AND GitHub webhook Secret field"
  else
    run_id="7$(shuf -i 100000-999999 -n 1)"
    gh_body=$(printf '{"action":"completed","repository":{"full_name":"kkjayakumar/AegIsOps"},"workflow_run":{"id":%s,"name":"CI","conclusion":"success","status":"completed","head_sha":"staging_smoke","head_branch":"main"}}' "$run_id")
    gh_sig=$(sign_hmac "$gh_body" "$GITHUB_SECRET")
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/github/webhook" \
      -H "Content-Type: application/json" \
      -H "X-GitHub-Event: workflow_run" \
      -H "X-GitHub-Delivery: $(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)" \
      -H "X-Hub-Signature-256: sha256=$gh_sig" \
      -d "$gh_body")
    if [[ "$code" == "202" || "$code" == "200" ]]; then
      echo "PASS GitHub webhook ($code)"
    else
      echo "WARN GitHub webhook returned HTTP $code"
    fi
  fi
fi

echo ""
echo "=== Next: GitHub repo webhook ==="
echo "  URL:    $BASE_URL/api/v1/github/webhook"
echo "  Secret: GITHUB_WEBHOOK_SECRET from deploy/staging/.env (random hex, NOT your PAT)"
echo "  https://github.com/kkjayakumar/AegIsOps/settings/hooks"
echo ""
echo "Phase 7 EC2 check complete."
