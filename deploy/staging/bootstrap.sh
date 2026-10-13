#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/deploy/staging"

if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env — edit secrets before production use"
fi

docker compose -f docker-compose.yml --env-file .env up -d --build

echo "Waiting for AegisOps health..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:5001/healthz >/dev/null; then
    echo "AegisOps is healthy"
    exit 0
  fi
  sleep 2
done
echo "Health check timed out — check logs: docker compose logs aegisops"
exit 1
