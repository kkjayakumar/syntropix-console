#!/usr/bin/env bash
# Phase 9 prerequisites — run on EC2 before the autonomous loop test.
set -euo pipefail

BASE_URL="${1:-https://aegisops.syntropix.io}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${ENV_FILE:-$(dirname "$SCRIPT_DIR")/deploy/staging/.env}"
PASS=0
WARN=0
FAIL=0

pass() { echo "PASS $*"; PASS=$((PASS + 1)); }
warn() { echo "WARN $*"; WARN=$((WARN + 1)); }
fail() { echo "FAIL $*"; FAIL=$((FAIL + 1)); }

env_val() {
  grep -E "^${1}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '\r' || true
}

echo "=== Phase 9: Autonomous loop prerequisites ==="
echo "Base URL: $BASE_URL"
echo ""

echo "--- Stack ---"
for name in staging-aegisops-1 devops-copilot-app-1 spotting-api-1 spotting-web-1; do
  if docker ps --format '{{.Names}}' | grep -qx "$name"; then
    pass "container $name running"
  else
    fail "container $name not running"
  fi
done

echo ""
echo "--- Dogfood (real detect, no staging-smoke.ps1) ---"
if [[ "$(env_val AEGISOPS_ALLOW_DOGFOOD)" =~ ^(1|true|yes|on|TRUE)$ ]]; then
  pass "AEGISOPS_ALLOW_DOGFOOD enabled"
else
  fail "Set AEGISOPS_ALLOW_DOGFOOD=true in $ENV_FILE and rebuild aegisops"
fi
if [[ -n "$(env_val SENTRY_DSN)" ]]; then
  pass "SENTRY_DSN set (Copilot ingest path)"
  if [[ "$(env_val SENTRY_DSN)" == *"127.0.0.1"* || "$(env_val SENTRY_DSN)" == *"localhost"* ]]; then
    if [[ -n "$(env_val COPILOT_INGEST_PUBLIC_URL)" ]]; then
      warn "SENTRY_DSN still uses loopback — set COPILOT_INGEST_PUBLIC_URL (app rewrites at runtime) or run scripts/fix-staging-sentry-dsn.sh"
    else
      fail "SENTRY_DSN uses 127.0.0.1/localhost — add COPILOT_INGEST_PUBLIC_URL=https://monitoring.syntropix.io to $ENV_FILE"
    fi
  fi
else
  fail "SENTRY_DSN missing — Copilot will not receive backend errors"
fi
if [[ -n "$(env_val NEXT_PUBLIC_SPOTTING_PUBLIC_KEY)" ]]; then
  pass "NEXT_PUBLIC_SPOTTING_PUBLIC_KEY set"
else
  warn "Spotting capture key unset — UI capture optional for backend-only dogfood"
fi

echo ""
echo "--- Debug endpoint ---"
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/debug/trigger-error" \
  -H "X-Request-Id: req_phase9_probe" || echo "000")
if [[ "$code" == "500" ]]; then
  pass "POST /api/v1/debug/trigger-error returns 500 (dogfood enabled)"
elif [[ "$code" == "404" ]]; then
  fail "debug endpoint disabled — enable AEGISOPS_ALLOW_DOGFOOD and rebuild"
else
  warn "debug/trigger-error returned HTTP $code"
fi

echo ""
echo "--- Copilot webhook path ---"
if curl -sf http://127.0.0.1:5000/api/ingest/config >/dev/null; then
  pass "Copilot ingest config reachable"
else
  fail "Copilot not reachable on :5000"
fi

echo ""
echo "--- GitHub API auth (issue + PR automation) ---"
if command -v jq >/dev/null 2>&1; then
  gh_json="$(curl -sf "${BASE_URL}/api/v1/version" 2>/dev/null || true)"
  if [[ -n "$gh_json" ]]; then
    gh_enabled="$(echo "$gh_json" | jq -r '.github.enabled // false')"
    gh_mode="$(echo "$gh_json" | jq -r '.github.mode // "none"')"
  else
    gh_enabled="false"
    gh_mode="unknown"
  fi
  if [[ "$gh_enabled" == "true" ]]; then
    pass "GitHub API auth enabled (mode=$gh_mode)"
  else
    fail "GitHub API auth disabled (mode=$gh_mode) — run scripts/verify-github-auth.sh"
  fi
else
  warn "jq not installed — skip live GitHub auth check"
fi

echo ""
echo "--- GitHub webhook (CI/CD + auto PR) ---"
echo "GitHub repo -> Settings -> Webhooks must subscribe to:"
echo "  - workflow_run (CI/CD monitor)"
echo "  - push (auto-open PR for aegis/fix-* branches)"
echo "  - pull_request (merge -> fixed)"
echo "Verify from workstation:"
echo "  .\\scripts\\verify-github-webhook.ps1 -BaseUrl $BASE_URL -GitHubSecret <secret>"

echo ""
echo "--- Summary ---"
echo "PASS=$PASS WARN=$WARN FAIL=$FAIL"
echo ""
if [[ $FAIL -gt 0 ]]; then
  echo "Fix FAIL items, then from Windows run:"
  echo "  .\\scripts\\test-phase9-staging-detect.ps1 -BaseUrl $BASE_URL"
  exit 1
fi
echo "Ready for Phase 9 detect test."
echo ""
echo "Next (Windows):"
echo "  .\\scripts\\test-phase9-staging-detect.ps1 -BaseUrl $BASE_URL"
echo "Then in Cursor: start_fix_from_issue -> PR -> merge -> deploy"
exit 0
