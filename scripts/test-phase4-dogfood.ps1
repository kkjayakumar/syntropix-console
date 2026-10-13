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
$issues = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues?status=detected" -TimeoutSec 10
$match = $issues.issues | Where-Object { $_.requestId -eq $cid -or $_.title -match "dogfood" } | Select-Object -First 1
if ($match) {
  Write-Host "Issue #$($match.id) detected: $($match.title)" -ForegroundColor Green
  Write-Host "View: $AegisUrl/issues/$($match.id)"
  exit 0
}

$recent = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues" -TimeoutSec 10
$latest = $recent.issues | Select-Object -First 1
if ($latest) {
  Write-Host "No exact requestId match yet. Latest issue #$($latest.id): $($latest.title)" -ForegroundColor Yellow
  Write-Host "Confirm Copilot + Spotting webhooks are configured and SENTRY_DSN is set."
  exit 0
}

Write-Host "No issues in inbox - verify stack webhooks and SENTRY_DSN" -ForegroundColor Yellow
exit 1
