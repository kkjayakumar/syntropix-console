#!/usr/bin/env bash
# Install DevOps Copilot + Spotting on the same EC2 host as AegisOps.
# Run as root or a user in the docker group.
set -euo pipefail

AEGIS_PUBLIC_URL="${AEGIS_PUBLIC_URL:-https://aegisops.syntropix.io}"
AEGIS_SECRET="${AEGIS_WEBHOOK_SECRET:-}"
OPT_DIR="${OPT_DIR:-/opt}"
COPILOT_REPO="${COPILOT_REPO:-https://github.com/kkjayakumar/devops-copilot-v2.git}"
SPOTTING_REPO="${SPOTTING_REPO:-https://github.com/kkjayakumar/spotting.git}"

ENV_FILE="${ENV_FILE:-/opt/aegisops/deploy/staging/.env}"
if [[ -z "$AEGIS_SECRET" && -f "$ENV_FILE" ]]; then
  AEGIS_SECRET=$(grep -E '^AEGIS_WEBHOOK_SECRET=' "$ENV_FILE" | head -1 | cut -d= -f2- | tr -d '\r')
fi
if [[ -z "$AEGIS_SECRET" ]]; then
  echo "Set AEGIS_WEBHOOK_SECRET or pass AEGIS_WEBHOOK_SECRET=... (must match AegisOps deploy/staging/.env)"
  exit 1
fi

echo "=== Installing DevOps Copilot ==="
if [[ ! -d "$OPT_DIR/copilot" ]]; then
  git clone "$COPILOT_REPO" "$OPT_DIR/copilot"
fi
cat >"$OPT_DIR/copilot/.env" <<EOF
AEGISOPS_WEBHOOK_URL=${AEGIS_PUBLIC_URL}/api/v1/webhooks/copilot
AEGISOPS_WEBHOOK_SECRET=${AEGIS_SECRET}
EOF
cd "$OPT_DIR/copilot"
docker compose up -d --build
echo "Copilot: http://127.0.0.1:5000"

echo ""
echo "=== Installing Spotting ==="
if [[ ! -d "$OPT_DIR/spotting" ]]; then
  git clone "$SPOTTING_REPO" "$OPT_DIR/spotting"
fi
# API on localhost only; AegisOps reaches it via host.docker.internal:3000
cat >"$OPT_DIR/spotting/.env" <<EOF
AEGISOPS_WEBHOOK_URL=${AEGIS_PUBLIC_URL}/api/v1/webhooks/spotting
AEGISOPS_WEBHOOK_SECRET=${AEGIS_SECRET}
CORS_ORIGINS=${AEGIS_PUBLIC_URL},http://127.0.0.1:3001
DOCKER_BIND_HOST=127.0.0.1
EOF
cd "$OPT_DIR/spotting"
docker compose up -d --build
echo "Spotting API: http://127.0.0.1:3000"
echo "Spotting UI:  http://127.0.0.1:3001"

echo ""
echo "=== Fetch Copilot Sentry DSN for AegisOps ==="
sleep 5
INGEST_JSON=$(curl -sf http://127.0.0.1:5000/api/ingest/config || true)
DSN=$(echo "$INGEST_JSON" | grep -o '"dsn":"[^"]*"' | cut -d'"' -f4 || true)
PUBLIC_INGEST="${COPILOT_INGEST_PUBLIC_URL:-https://monitoring.syntropix.io}"
if [[ -n "$DSN" ]]; then
  # Copilot UI may show 127.0.0.1 — AegisOps needs a reachable host from its container.
  PUBLIC_HOST="${PUBLIC_INGEST#https://}"
  PUBLIC_HOST="${PUBLIC_HOST#http://}"
  PUBLIC_HOST="${PUBLIC_HOST%%/*}"
  DSN="${DSN/@127.0.0.1:5000/@${PUBLIC_HOST}/}"
  DSN="${DSN/@localhost:5000/@${PUBLIC_HOST}/}"
  echo "SENTRY_DSN=$DSN"
  echo "COPILOT_INGEST_PUBLIC_URL=$PUBLIC_INGEST"
  echo "Add both to $ENV_FILE then rebuild AegisOps."
else
  echo "Could not fetch DSN yet — run: curl http://127.0.0.1:5000/api/ingest/config"
fi

echo ""
echo "=== Next steps ==="
cat <<EOF

1. Spotting capture key (for dashboard embed):
   - SSH tunnel or open port 3001, or add Caddy for spotting.syntropix.io
   - Create key at Spotting Settings with origin: ${AEGIS_PUBLIC_URL}
   - Set NEXT_PUBLIC_SPOTTING_PUBLIC_KEY in deploy/staging/.env

2. Rebuild AegisOps with DSN + Spotting key:
   cd /opt/aegisops/deploy/staging
   # add SENTRY_DSN and NEXT_PUBLIC_SPOTTING_* to .env
   docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops

3. Verify closed loop:
   cd /opt/aegisops && ./scripts/setup-phase7-staging.sh ${AEGIS_PUBLIC_URL}

4. Optional public Spotting API (browser capture from dashboard):
   Add Caddy vhost spotting.syntropix.io -> 127.0.0.1:3000
   Set NEXT_PUBLIC_SPOTTING_API_URL=https://spotting.syntropix.io

EOF

echo "Install complete."
