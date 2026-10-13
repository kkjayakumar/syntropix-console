param(
  [string]$BaseUrl = "https://aegisops.syntropix.io"
)

$ErrorActionPreference = "Stop"
Write-Host "=== GitHub auth check ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"

$v = Invoke-RestMethod -Uri "$BaseUrl/api/v1/version" -TimeoutSec 20
$v | ConvertTo-Json -Depth 4

$enabled = $v.github.enabled
$mode = $v.github.mode
if ($enabled) {
  Write-Host "PASS GitHub API auth enabled (mode=$mode)" -ForegroundColor Green
  exit 0
}

Write-Host "FAIL GitHub API auth disabled (mode=$mode)" -ForegroundColor Red
Write-Host @"
Fix on EC2:
  1. Add GITHUB_APP_* to AWS Secrets Manager (dev/syntropix)
  2. Install /opt/aegisops/secrets/github-app.pem
  3. Redeploy with docker-compose.external-db.yml + docker-compose.ghcr.yml
  4. Or set GITHUB_TOKEN (Issues write) in secrets
"@ -ForegroundColor Yellow
exit 1
