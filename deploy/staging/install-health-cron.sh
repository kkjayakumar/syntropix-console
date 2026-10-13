#!/usr/bin/env bash
# Install hourly health checks (Phase 8.7). Logs failures via syslog.
set -euo pipefail

TARGET="/etc/cron.hourly/aegisops-health"
AEGIS_URL="${AEGIS_URL:-https://aegisops.syntropix.io/healthz}"
SPOTTING_URL="${SPOTTING_URL:-https://api.spotting.syntropix.io/healthz}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo $0"
  exit 1
fi

cat >"$TARGET" <<EOF
#!/bin/bash
check() {
  local url="\$1"
  local code
  code=\$(curl -sf -o /dev/null -w "%{http_code}" --max-time 15 "\$url" 2>/dev/null || echo "000")
  if [[ "\$code" != "200" ]]; then
    logger -t aegisops-health "FAIL \$url -> HTTP \$code"
  fi
}
check "$AEGIS_URL"
check "$SPOTTING_URL"
EOF
chmod +x "$TARGET"
echo "Installed $TARGET"
echo "Test: $TARGET && logger -t aegisops-health OK"
