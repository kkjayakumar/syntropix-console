param(
  [string]$AegisUrl = "http://localhost:5001",
  [string]$CopilotUrl = "http://localhost:5000",
  [string]$SpottingUrl = "http://localhost:3000",
  [int]$TimeoutSec = 120
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root ".env"
if (-not (Test-Path $envFile)) { throw "Missing $envFile" }

function Get-EnvValue([string]$Key) {
  $line = Get-Content $envFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

function Wait-HttpOk([string]$Url, [string]$Label) {
  $deadline = (Get-Date).AddSeconds($TimeoutSec)
  while ((Get-Date) -lt $deadline) {
    try {
      $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
      if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 300) {
        Write-Host "$Label ready: $Url" -ForegroundColor Green
        return
      }
    } catch {}
    Start-Sleep -Seconds 3
  }
  throw "$Label not ready at $Url after ${TimeoutSec}s"
}

function Sign-Body([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return ([BitConverter]::ToString($hash).Replace("-", "").ToLower())
}

$secret = Get-EnvValue "AEGIS_WEBHOOK_SECRET"
if (-not $secret) { throw "AEGIS_WEBHOOK_SECRET missing in .env" }

Write-Host "=== AegisOps self-test: wait for stack ===" -ForegroundColor Cyan
Wait-HttpOk "$AegisUrl/healthz" "AegisOps"
Wait-HttpOk "$CopilotUrl/api/ingest/config" "DevOps Copilot"
Wait-HttpOk "$SpottingUrl/healthz" "Spotting API"

Write-Host "=== Backend: Sentry envelope -> Copilot -> AegisOps ===" -ForegroundColor Cyan
$ingest = Invoke-RestMethod -Uri "$CopilotUrl/api/ingest/config"
$projectId = $ingest.projectId
$cid = "req_aegis_" + [guid]::NewGuid().ToString().Substring(0, 8)
$event = @{
  event_id = ([guid]::NewGuid().ToString().Replace("-", ""))
  timestamp  = (Get-Date).ToUniversalTime().ToString("o")
  level      = "error"
  message    = "AegisOps self-test backend error"
  tags       = @{ request_id = $cid; service = "aegisops" }
  exception  = @{ values = @(@{ type = "AegisSelfTest"; value = "dogfood crash from aegisops stack" }) }
} | ConvertTo-Json -Compress -Depth 6
$envelope = "{}" + [Environment]::NewLine + '{"type":"event"}' + [Environment]::NewLine + $event
Invoke-RestMethod -Uri "$CopilotUrl/api/$projectId/envelope" -Method POST -Body $envelope -ContentType "application/x-sentry-envelope" | Out-Null
Write-Host "Posted Sentry envelope (correlationId=$cid)" -ForegroundColor Green
Start-Sleep -Seconds 2

Write-Host "=== Frontend: Spotting-style webhook -> AegisOps ===" -ForegroundColor Cyan
$spotBody = @{
  event         = "spotting.report.created"
  correlationId = $cid
  reportId      = "rpt_aegis_selftest_" + [guid]::NewGuid().ToString().Substring(0, 8)
  severity      = "error"
  pageUrl       = "$AegisUrl/"
  networkFailures = @(@{ method = "GET"; url = "/api/v1/stats"; status = 500 })
  consoleErrors = 1
} | ConvertTo-Json -Compress -Depth 6
$sig = Sign-Body $spotBody $secret
$headers = @{
  "Content-Type"       = "application/json"
  "X-Aegis-Signature"  = "sha256=$sig"
}
Invoke-RestMethod -Uri "$AegisUrl/api/v1/webhooks/spotting" -Method POST -Headers $headers -Body $spotBody | Out-Null
Write-Host "Posted Spotting webhook (same correlationId)" -ForegroundColor Green
Start-Sleep -Seconds 4

Write-Host "=== Verify correlated issue in AegisOps ===" -ForegroundColor Cyan
$list = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues"
$issue = $list.issues | Where-Object { $_.correlationId -eq $cid } | Select-Object -First 1
if (-not $issue) { throw "No correlated issue found for $cid" }
Write-Host "Issue #$($issue.id) status=$($issue.status) title=$($issue.title)" -ForegroundColor Green
if (-not $issue.telemetryBundle) { throw "telemetry bundle missing" }
if ($issue.githubIssueNumber) {
  Write-Host "GitHub issue #$($issue.githubIssueNumber) on $($issue.githubRepo)" -ForegroundColor Green
} else {
  Write-Host "GitHub issue not linked yet (check GITHUB_TOKEN)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Self-test passed. Open dashboard: $AegisUrl" -ForegroundColor Green
