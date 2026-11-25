param(
  [string]$AegisUrl = "http://localhost:5001",
  [int]$WaitSec = 15
)

$ErrorActionPreference = "Stop"

Write-Host "=== Phase 4: debug trigger endpoint ===" -ForegroundColor Cyan
$cid = "req_phase4_" + [guid]::NewGuid().ToString().Substring(0, 8)
$headers = @{ "X-Request-Id" = $cid }
try {
  Invoke-WebRequest -Uri "$AegisUrl/api/v1/debug/trigger-error" -Method POST -Headers $headers -UseBasicParsing | Out-Null
  throw "Expected HTTP 500"
} catch {
  $resp = $_.Exception.Response
  if (-not $resp -or [int]$resp.StatusCode -ne 500) {
    throw "debug/trigger-error failed: $($_.Exception.Message)"
  }
  Write-Host "Backend 500 OK (requestId=$cid)" -ForegroundColor Green
}

Write-Host "Waiting ${WaitSec}s for Copilot/Spotting webhooks..." -ForegroundColor Cyan
Start-Sleep -Seconds $WaitSec

Write-Host "=== Phase 4: check correlated issue ===" -ForegroundColor Cyan
$issues = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues" -TimeoutSec 10
$match = $issues.issues | Where-Object { $_.correlationId -eq $cid } | Select-Object -First 1
if ($match) {
  if (-not $match.telemetryBundle) { throw "Issue #$($match.id) has no telemetry bundle for $cid" }
  Write-Host "Issue #$($match.id) detected: $($match.title)" -ForegroundColor Green
  Write-Host "View: $AegisUrl/issues/$($match.id)"
  exit 0
}

throw "No incident matched $cid. Verify telemetry ingest, webhook delivery, and SENTRY_DSN."
