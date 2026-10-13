#!/usr/bin/env bash
# Run on EC2 via SSM (or manually). Fetches .env from Secrets Manager and redeploys AegisOps from GHCR.

set -euo pipefail

IMAGE_TAG="${1:-latest}"
REPO_DIR="${REPO_DIR:-/opt/aegisops}"
ENV_DIR="${REPO_DIR}/deploy/staging"
SECRET_ID="${AEGISOPS_SECRET_ID:-dev/syntropix}"
AWS_REGION="${AWS_REGION:-ap-south-1}"
COMPOSE="docker compose -f docker-compose.yml -f docker-compose.external-db.yml -f docker-compose.ghcr.yml"

log() {
  echo "[deploy] $*"
}

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI required on EC2"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq required on EC2 (apt install -y jq)"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker required on EC2"
  exit 1
fi

log "deployment started"
log "image tag: ${IMAGE_TAG}"
log "secret id: ${SECRET_ID}"
log "region: ${AWS_REGION}"

# Fetch environment variables from Secrets Manager
log "fetch Secrets Manager secret: ${SECRET_ID}"

mkdir -p "${ENV_DIR}"

SECRET_JSON="$(aws secretsmanager get-secret-value \
  --secret-id "${SECRET_ID}" \
  --region "${AWS_REGION}" \
  --query SecretString \
  --output text)"

# Production requires a strong workspace encryption key (AES-256-GCM).
if ! echo "${SECRET_JSON}" | jq -e '.WORKSPACE_SECRETS_KEY | type=="string" and (length >= 32)' >/dev/null 2>&1; then
  GEN_KEY="$(openssl rand -hex 32)"
  log "WORKSPACE_SECRETS_KEY missing — generating and persisting to Secrets Manager"
  SECRET_JSON="$(echo "${SECRET_JSON}" | jq --arg k "${GEN_KEY}" '.WORKSPACE_SECRETS_KEY = $k')"
  if aws secretsmanager put-secret-value \
    --secret-id "${SECRET_ID}" \
    --region "${AWS_REGION}" \
    --secret-string "${SECRET_JSON}" >/dev/null; then
    log "persisted WORKSPACE_SECRETS_KEY to ${SECRET_ID}"
  else
    log "warning: could not persist WORKSPACE_SECRETS_KEY to Secrets Manager (IAM?); using generated key for this host .env only"
  fi
fi

# Production HTTP API bearer token (dashboard Settings → paste same value).
if ! echo "${SECRET_JSON}" | jq -e '.AEGISOPS_API_TOKEN | type=="string" and (length >= 16)' >/dev/null 2>&1; then
  GEN_TOKEN="$(openssl rand -hex 32)"
  log "AEGISOPS_API_TOKEN missing — generating and persisting to Secrets Manager"
  SECRET_JSON="$(echo "${SECRET_JSON}" | jq --arg k "${GEN_TOKEN}" '.AEGISOPS_API_TOKEN = $k')"
  if aws secretsmanager put-secret-value \
    --secret-id "${SECRET_ID}" \
    --region "${AWS_REGION}" \
    --secret-string "${SECRET_JSON}" >/dev/null; then
    log "persisted AEGISOPS_API_TOKEN to ${SECRET_ID}"
  else
    log "warning: could not persist AEGISOPS_API_TOKEN to Secrets Manager (IAM?); using generated token for this host .env only"
  fi
fi

echo "${SECRET_JSON}" \
  | jq -r 'to_entries | .[] | select(.value != null and .value != "") | "\(.key)=\(.value)"' \
  > "${ENV_DIR}/.env"

chmod 600 "${ENV_DIR}/.env"

# Preserve GHCR creds injected by CI/SSM when Secrets Manager omits them.
PRESERVED_GHCR_TOKEN="${GHCR_TOKEN:-}"
PRESERVED_GHCR_USER="${GHCR_USER:-}"

# Load environment variables
set -a
# shellcheck disable=SC1091
source "${ENV_DIR}/.env"
set +a

if [[ -z "${GHCR_TOKEN:-}" && -n "${PRESERVED_GHCR_TOKEN}" ]]; then
  GHCR_TOKEN="${PRESERVED_GHCR_TOKEN}"
  GHCR_USER="${PRESERVED_GHCR_USER:-kkjayakumar}"
fi

# GHCR Login
if [[ -n "${GHCR_TOKEN:-}" ]]; then
  GHCR_USER="${GHCR_USER:-kkjayakumar}"

  log "docker login ghcr.io as ${GHCR_USER}"

  echo "${GHCR_TOKEN}" | docker login ghcr.io \
    -u "${GHCR_USER}" \
    --password-stdin
else
  log "warning: GHCR_TOKEN unset — private image pulls will fail"
fi

export AEGISOPS_IMAGE_TAG="${IMAGE_TAG}"
export DOCKER_DEFAULT_PLATFORM="${DOCKER_DEFAULT_PLATFORM:-linux/arm64}"
ARCH="$(uname -m)"
log "host arch=${ARCH} docker platform=${DOCKER_DEFAULT_PLATFORM}"

cd "${ENV_DIR}"

log "pull image ghcr.io/kkjayakumar/aegisops:${AEGISOPS_IMAGE_TAG}"
$COMPOSE pull aegisops

log "restart container"

$COMPOSE up -d --no-build aegisops

log "running containers"

docker ps

VERIFY_URL="${DEPLOY_VERIFY_URL:-https://aegisops.syntropix.io/healthz}"

log "health check ${VERIFY_URL}"

for i in $(seq 1 12); do
  if curl -sf "${VERIFY_URL}" | jq -e '.status == "ok"' >/dev/null 2>&1; then
    log "health check passed"
    exit 0
  fi

  echo "attempt ${i}/12"
  sleep 5
done

log "health check failed"

docker ps -a

docker logs staging-aegisops-1 --tail 100 2>/dev/null || true

exit 1
