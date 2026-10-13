param(
  [string]$AegisUrl = "http://localhost:5001",
  [string]$CopilotUrl = "http://localhost:5000",
  [string]$SpottingUrl = "http://localhost:3000",
  [string]$Origin = "http://localhost:5001"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root ".env"

function Get-EnvValue([string]$Key) {
  if (-not (Test-Path $envFile)) { return "" }
  $line = Get-Content $envFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

function Set-EnvValue([string]$Key, [string]$Value) {
  $lines = @()
  $found = $false
  if (Test-Path $envFile) {
    foreach ($line in Get-Content $envFile) {
      if ($line -match "^$Key=") {
        $lines += "$Key=$Value"
        $found = $true
      } else {
        $lines += $line
      }
    }
  }
  if (-not $found) { $lines += "$Key=$Value" }
  Set-Content -Path $envFile -Value $lines -Encoding UTF8
}

Write-Host "=== Phase 4 setup: Copilot DSN ===" -ForegroundColor Cyan
$ingest = Invoke-RestMethod -Uri "$CopilotUrl/api/ingest/config" -TimeoutSec 10
$dsn = $ingest.dsn -replace "localhost", "host.docker.internal"
Write-Host "Copilot DSN (docker-friendly): $dsn" -ForegroundColor Green
Set-EnvValue "SENTRY_DSN" $dsn

Write-Host "`n=== Phase 4 setup: Spotting public key ===" -ForegroundColor Cyan
$existing = Get-EnvValue "NEXT_PUBLIC_SPOTTING_PUBLIC_KEY"
if ($existing) {
  Write-Host "NEXT_PUBLIC_SPOTTING_PUBLIC_KEY already set in .env" -ForegroundColor Yellow
} else {
  Write-Host @"
Create a capture key in Spotting:
  1. Open http://127.0.0.1:3001/dashboard/settings/keys
  2. Create key with allowed origin: $Origin
  3. Copy spk_live_… token into .env as NEXT_PUBLIC_SPOTTING_PUBLIC_KEY
  4. Rebuild dashboard: make web-build && docker compose build aegisops
"@ -ForegroundColor Yellow

  $pg = docker ps --format "{{.Names}}" | Where-Object { $_ -match "spotting.*postgres" } | Select-Object -First 1
  if ($pg) {
    $sql = "SELECT token FROM capture_public_keys WHERE revoked_at IS NULL ORDER BY created_at DESC LIMIT 1;"
    $key = (docker exec $pg psql -U postgres -d spotting -t -A -c $sql 2>$null).Trim()
    if ($key -match "^spk_") {
      Write-Host "Found existing key in Spotting DB: $key" -ForegroundColor Green
      Set-EnvValue "NEXT_PUBLIC_SPOTTING_PUBLIC_KEY" $key
      $updateSql = @"
UPDATE capture_public_keys
SET "allowedOrigins" = '["$Origin"]'::jsonb
WHERE token = '$key';
"@
      $updateSql | docker exec -i $pg psql -U postgres -d spotting | Out-Null
      Write-Host "Updated allowedOrigins to [$Origin]" -ForegroundColor Green
    }
  }
}

Set-EnvValue "NEXT_PUBLIC_SPOTTING_API_URL" $SpottingUrl
Set-EnvValue "NEXT_PUBLIC_SPOTTING_DASHBOARD_URL" "http://127.0.0.1:3001"

Write-Host "`n=== Next steps ===" -ForegroundColor Cyan
Write-Host "1. make web-build   (or docker compose build aegisops)"
Write-Host "2. docker compose up -d aegisops   (or go run ./cmd/aegisops)"
Write-Host "3. Open $AegisUrl → Phase 4 panel → Trigger backend 500"
Write-Host "4. Run .\scripts\test-phase4-dogfood.ps1"
