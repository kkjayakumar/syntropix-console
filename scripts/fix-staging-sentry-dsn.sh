#!/usr/bin/env bash
# Fix SENTRY_DSN on EC2 when Copilot ingest config returns 127.0.0.1 (unreachable from AegisOps container).
set -euo pipefail

ENV_FILE="${ENV_FILE:-/opt/aegisops/deploy/staging/.env}"
PUBLIC_INGEST="${COPILOT_INGEST_PUBLIC_URL:-https://monitoring.syntropix.io}"
PUBLIC_HOST="${PUBLIC_INGEST#https://}"
PUBLIC_HOST="${PUBLIC_HOST#http://}"
PUBLIC_HOST="${PUBLIC_HOST%%/*}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE"
  exit 1
fi

INGEST=$(curl -sf http://127.0.0.1:5000/api/ingest/config)
RAW_DSN=$(echo "$INGEST" | grep -o '"dsn":"[^"]*"' | cut -d'"' -f4)
if [[ -z "$RAW_DSN" ]]; then
  echo "Could not read dsn from Copilot ingest config"
  exit 1
fi

FIXED_DSN="${RAW_DSN/@127.0.0.1:5000/@host.docker.internal:5000/}"
FIXED_DSN="${FIXED_DSN/@localhost:5000/@host.docker.internal:5000/}"
FIXED_DSN="${FIXED_DSN/@${PUBLIC_HOST}/@host.docker.internal:5000/}"

set_kv() {
  local key="$1" val="$2"
  if grep -q "^${key}=" "$ENV_FILE"; then
    sed -i "s|^${key}=.*|${key}=${val}|" "$ENV_FILE"
  else
    echo "${key}=${val}" >>"$ENV_FILE"
  fi
}

set_kv "SENTRY_DSN" "$FIXED_DSN"
set_kv "COPILOT_INGEST_PUBLIC_URL" "$PUBLIC_INGEST"

echo "Updated $ENV_FILE:"
echo "  SENTRY_DSN=$FIXED_DSN"
echo "  COPILOT_INGEST_PUBLIC_URL=$PUBLIC_INGEST"
echo ""
echo "Rebuild AegisOps:"
echo "  cd /opt/aegisops/deploy/staging"
echo "  docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops"
echo ""
echo "Note: DSN uses host.docker.internal:5000 so the AegisOps container reaches Copilot directly."
echo "      Copilot UI may still show monitoring.syntropix.io — that is for external SDK clients."
