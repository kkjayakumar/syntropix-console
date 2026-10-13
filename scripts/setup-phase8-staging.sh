#!/usr/bin/env bash
# Phase 8 production hardening checks — run on EC2 or against staging URL.
set -euo pipefail

BASE_URL="${1:-https://aegisops.syntropix.io}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${ENV_FILE:-$(dirname "$SCRIPT_DIR")/deploy/staging/.env}"
SECRETS_DIR="${GITHUB_APP_SECRETS_DIR:-/opt/aegisops/secrets}"
PASS=0
WARN=0
FAIL=0

pass() { echo "PASS $*"; PASS=$((PASS + 1)); }
warn() { echo "WARN $*"; WARN=$((WARN + 1)); }
fail() { echo "FAIL $*"; FAIL=$((FAIL + 1)); }

env_val() {
  local key="$1"
  grep -E "^${key}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '\r' || true
}

echo "=== Phase 8: Production hardening check ==="
echo "Base URL: $BASE_URL"
echo "Env file: $ENV_FILE"
echo ""

if [[ ! -f "$ENV_FILE" ]]; then
  fail "missing $ENV_FILE"
else
  pass "staging .env exists"
fi

echo ""
echo "--- 8.1 Production mode ---"
aegis_env="$(env_val AEGISOPS_ENV)"
if [[ "$aegis_env" == "production" || "$aegis_env" == "prod" ]]; then
  pass "AEGISOPS_ENV=$aegis_env"
else
  warn "AEGISOPS_ENV should be production (got: ${aegis_env:-unset})"
fi

echo ""
echo "--- 8.2 Secrets ---"
aegis_secret="$(env_val AEGIS_WEBHOOK_SECRET)"
gh_secret="$(env_val GITHUB_WEBHOOK_SECRET)"
if [[ -n "$aegis_secret" && "$aegis_secret" != "dev-webhook-secret" && "$aegis_secret" != "CHANGE_ME" ]]; then
  pass "AEGIS_WEBHOOK_SECRET set (non-default)"
else
  fail "AEGIS_WEBHOOK_SECRET missing or default"
fi
if [[ -n "$gh_secret" && "$gh_secret" != "CHANGE_ME" ]]; then
  pass "GITHUB_WEBHOOK_SECRET set"
else
  fail "GITHUB_WEBHOOK_SECRET missing"
fi
if [[ "$gh_secret" == github_pat_* ]]; then
  fail "GITHUB_WEBHOOK_SECRET looks like a PAT — use openssl rand -hex 32"
fi
db_url="$(env_val DATABASE_URL)"
if [[ -n "$db_url" && "$db_url" != *"aegisops:aegisops@localhost"* ]]; then
  pass "DATABASE_URL uses external/managed Postgres"
else
  warn "DATABASE_URL still points at local default — use Supabase or managed DB"
fi

echo ""
echo "--- 8.3 GitHub auth (App preferred) ---"
app_id="$(env_val GITHUB_APP_ID)"
install_id="$(env_val GITHUB_APP_INSTALLATION_ID)"
key_path="$(env_val GITHUB_APP_PRIVATE_KEY_PATH)"
pat="$(env_val GITHUB_TOKEN)"
if [[ -n "$app_id" && -n "$install_id" ]]; then
  pass "GitHub App IDs configured"
  host_key="${key_path:-/secrets/github-app.pem}"
  if [[ -f "${SECRETS_DIR}/github-app.pem" || -f "$host_key" ]]; then
    pass "GitHub App private key present on host"
  else
    fail "GitHub App key missing — install to ${SECRETS_DIR}/github-app.pem"
  fi
  if [[ -n "$pat" ]]; then
    warn "GITHUB_TOKEN still set — remove after App auth verified"
  fi
elif [[ -n "$pat" ]]; then
  warn "Using GITHUB_TOKEN PAT only — create GitHub App (Phase 8.3)"
else
  fail "No GitHub auth — set App or GITHUB_TOKEN"
fi

echo ""
echo "--- 8.4 Sentinel / deploy verify ---"
verify_url="$(env_val DEPLOY_VERIFY_URL)"
if [[ -n "$verify_url" && "$verify_url" != *"example.com"* && "$verify_url" != *"your-app"* ]]; then
  pass "DEPLOY_VERIFY_URL=$verify_url"
  code=$(curl -s -o /dev/null -w "%{http_code}" "$verify_url" || echo "000")
  if [[ "$code" == "200" ]]; then
    pass "DEPLOY_VERIFY_URL returns 200"
  else
    warn "DEPLOY_VERIFY_URL returned HTTP $code"
  fi
else
  fail "DEPLOY_VERIFY_URL not set or still placeholder"
fi
rollback_wf="$(env_val ROLLBACK_WORKFLOW)"
if [[ -n "$rollback_wf" ]]; then
  pass "ROLLBACK_WORKFLOW=$rollback_wf"
else
  warn "ROLLBACK_WORKFLOW unset — sentinel rollback disabled"
fi

echo ""
echo "--- 8.5 Public endpoints ---"
for url in \
  "$BASE_URL/healthz" \
  "https://api.spotting.syntropix.io/healthz" \
  "https://spotting.syntropix.io" \
  "https://monitoring.syntropix.io"; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "$url" || echo "000")
  if [[ "$code" =~ ^(200|301|302|307|308)$ ]]; then
    pass "$url -> $code"
  else
    warn "$url -> HTTP $code"
  fi
done

echo ""
echo "--- 8.7 Copilot / Spotting webhook alignment ---"
for f in /opt/devops-copilot/.env /opt/copilot/.env /opt/spotting/.env; do
  [[ -f "$f" ]] || continue
  if grep -q "AEGISOPS_WEBHOOK_URL=${BASE_URL}/api/v1/webhooks" "$f" 2>/dev/null; then
    pass "$(basename "$(dirname "$f")") webhook URL points at staging"
  else
    warn "$(basename "$(dirname "$f")") webhook URL may be wrong — check $f"
  fi
  if [[ -n "$aegis_secret" ]] && grep -q "AEGISOPS_WEBHOOK_SECRET=${aegis_secret}" "$f" 2>/dev/null; then
    pass "$(basename "$(dirname "$f")") secret matches AegisOps"
  elif [[ -n "$aegis_secret" ]]; then
    warn "$(basename "$(dirname "$f")") AEGISOPS_WEBHOOK_SECRET mismatch"
  fi
done

echo ""
echo "--- Summary ---"
echo "PASS=$PASS WARN=$WARN FAIL=$FAIL"
echo ""
if [[ $FAIL -gt 0 ]]; then
  echo "Phase 8 incomplete — fix FAIL items above."
  echo "See docs/RUNBOOK.md and deploy/staging/.env.example"
  exit 1
fi
if [[ $WARN -gt 0 ]]; then
  echo "Phase 8 partial — address WARN items (GitHub App, monitoring, branch protection)."
  exit 0
fi
echo "Phase 8 checks passed."
exit 0
