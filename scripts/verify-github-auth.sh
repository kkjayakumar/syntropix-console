#!/usr/bin/env bash
# Verify GitHub API auth from a running AegisOps instance (pillar 5).
set -euo pipefail

BASE_URL="${1:-https://aegisops.syntropix.io}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${ENV_FILE:-$(dirname "$SCRIPT_DIR")/deploy/staging/.env}"

echo "=== GitHub auth check ==="
echo "Base URL: $BASE_URL"
echo ""

version="$(curl -sf "${BASE_URL}/api/v1/version" || true)"
if [[ -z "$version" ]]; then
  echo "FAIL cannot reach ${BASE_URL}/api/v1/version"
  exit 1
fi

echo "$version" | jq .

enabled="$(echo "$version" | jq -r '.github.enabled // false')"
mode="$(echo "$version" | jq -r '.github.mode // "none"')"
key_readable="$(echo "$version" | jq -r '.github.keyReadable // false')"

if [[ "$enabled" == "true" ]]; then
  echo ""
  echo "PASS GitHub API auth enabled (mode=$mode)"
  exit 0
fi

echo ""
echo "FAIL GitHub API auth not enabled (mode=$mode, keyReadable=$key_readable)"
echo ""
echo "Fix on EC2:"
echo "  1. Ensure Secrets Manager secret includes GITHUB_APP_ID, GITHUB_APP_INSTALLATION_ID"
echo "  2. Install PEM: /opt/aegisops/secrets/github-app.pem (chmod 600)"
echo "  3. GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem in .env"
echo "  4. Compose must include docker-compose.external-db.yml (mounts /secrets)"
echo "  5. Or set GITHUB_TOKEN (PAT with repo Issues write) in Secrets Manager"
echo ""
if [[ -f "$ENV_FILE" ]]; then
  echo "Local .env snapshot:"
  grep -E '^GITHUB_(APP_ID|APP_INSTALLATION_ID|APP_PRIVATE_KEY_PATH|TOKEN)=' "$ENV_FILE" 2>/dev/null | sed 's/=.*/=***/' || true
fi
exit 1
