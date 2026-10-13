param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string]$Repo = "kkjayakumar/AegIsOps",
  [string]$AegisSecret = "",
  [string]$GitHubWebhookSecret = "",
  [switch]$SendTestIncident
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root ".env"

function Get-LocalEnv([string]$Key) {
  if (-not (Test-Path $envFile)) { return "" }
  $line = Get-Content $envFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

function Sign-AegisBody([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return "sha256=" + ([BitConverter]::ToString($hash).Replace("-", "").ToLower())
}

function Sign-GitHubBody([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return "sha256=" + ([BitConverter]::ToString($hash).Replace("-", "").ToLower())
}

Write-Host "=== Phase 7: Staging setup check ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl" -ForegroundColor White

Write-Host "`n--- API health ---" -ForegroundColor Cyan
$health = Invoke-RestMethod -Uri "$BaseUrl/healthz" -TimeoutSec 20
if ($health.status -ne "ok") { throw "healthz failed: $($health | ConvertTo-Json -Compress)" }
Write-Host "PASS healthz" -ForegroundColor Green

$stats = Invoke-RestMethod -Uri "$BaseUrl/api/v1/stats" -TimeoutSec 20
Write-Host "Stats: issues=$($stats.issuesTotal) deployments=$($stats.deploymentsTotal) open=$($stats.openIncidents)" -ForegroundColor Green

if (-not $AegisSecret) { $AegisSecret = Get-LocalEnv "AEGIS_WEBHOOK_SECRET" }
if (-not $GitHubWebhookSecret) { $GitHubWebhookSecret = Get-LocalEnv "GITHUB_WEBHOOK_SECRET" }

if ($SendTestIncident) {
  if (-not $AegisSecret) { throw "Set -AegisSecret or AEGIS_WEBHOOK_SECRET in .env for webhook test" }
  Write-Host "`n--- Test incident (Copilot + Spotting) ---" -ForegroundColor Cyan
  $cid = "req_staging_" + [guid]::NewGuid().ToString("N").Substring(0, 10)

  $copilotBody = (@{
    event = "sentry.issue.created"
    correlationId = $cid
    issue = @{ id = 9001; title = "Staging smoke test error"; level = "error"; count = 1 }
  } | ConvertTo-Json -Compress)
  $sig = Sign-AegisBody $copilotBody $AegisSecret
  Invoke-RestMethod -Uri "$BaseUrl/api/v1/webhooks/copilot" -Method POST -Headers @{
    "Content-Type" = "application/json"
    "X-Aegis-Signature" = $sig
  } -Body $copilotBody | Out-Null

  Start-Sleep -Seconds 1
  $spotBody = (@{
    event = "spotting.report.created"
    correlationId = $cid
    reportId = "rpt_staging_smoke"
    severity = "error"
    pageUrl = "$BaseUrl/"
    networkFailures = @(@{ method = "GET"; url = "/api/staging-smoke"; status = 500 })
  } | ConvertTo-Json -Compress)
  $sig2 = Sign-AegisBody $spotBody $AegisSecret
  Invoke-RestMethod -Uri "$BaseUrl/api/v1/webhooks/spotting" -Method POST -Headers @{
    "Content-Type" = "application/json"
    "X-Aegis-Signature" = $sig2
  } -Body $spotBody | Out-Null

  Start-Sleep -Seconds 3
  $issues = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues?type=fix"
  $hit = $issues.issues | Where-Object { $_.correlationId -eq $cid } | Select-Object -First 1
  if ($hit) {
    Write-Host "PASS correlated issue #$($hit.id) status=$($hit.status)" -ForegroundColor Green
  } else {
    Write-Host "WARN no issue for $cid (check AEGIS_WEBHOOK_SECRET matches staging .env)" -ForegroundColor Yellow
  }
}

if ($GitHubWebhookSecret) {
  Write-Host "`n--- GitHub webhook simulation ---" -ForegroundColor Cyan
  $body = (@{
    action = "completed"
    repository = @{ full_name = $Repo }
    workflow_run = @{
      id = [int][double]::Parse(("7" + (Get-Random -Maximum 999999)))
      name = "CI"
      conclusion = "success"
      status = "completed"
      head_sha = "staging_smoke"
      head_branch = "main"
    }
  } | ConvertTo-Json -Compress -Depth 8)
  $ghSig = Sign-GitHubBody $body $GitHubWebhookSecret
  try {
    Invoke-RestMethod -Uri "$BaseUrl/api/v1/github/webhook" -Method POST -Headers @{
      "Content-Type" = "application/json"
      "X-GitHub-Event" = "workflow_run"
      "X-GitHub-Delivery" = [guid]::NewGuid().ToString()
      "X-Hub-Signature-256" = $ghSig
    } -Body $body | Out-Null
    Write-Host "PASS signed GitHub workflow_run accepted" -ForegroundColor Green
  } catch {
    Write-Host "WARN GitHub webhook failed (secret mismatch on staging?): $($_.Exception.Message)" -ForegroundColor Yellow
  }
} else {
  Write-Host "`nSkip GitHub webhook test (no GITHUB_WEBHOOK_SECRET)" -ForegroundColor Yellow
}

Write-Host "`n=== Manual steps (do on GitHub + EC2) ===" -ForegroundColor Cyan
Write-Host @"

1. GitHub repo webhook (completes Phase 7.7):
   URL:     $BaseUrl/api/v1/github/webhook
   Secret:  same as GITHUB_WEBHOOK_SECRET in deploy/staging/.env
   Events:  Issues, Pull requests, Workflow runs
   Settings: https://github.com/$Repo/settings/hooks

2. Staging .env on EC2 (deploy/staging/.env):
   AEGISOPS_PUBLIC_URL=$BaseUrl
   DEPLOY_VERIFY_URL=$BaseUrl/healthz

3. DevOps Copilot (.env on EC2 host):
   AEGISOPS_WEBHOOK_URL=$BaseUrl/api/v1/webhooks/copilot
   AEGISOPS_WEBHOOK_SECRET=<same as AEGIS_WEBHOOK_SECRET>

4. Spotting (.env on EC2 host):
   AEGISOPS_WEBHOOK_URL=$BaseUrl/api/v1/webhooks/spotting
   AEGIS_WEBHOOK_SECRET=<same as AEGIS_WEBHOOK_SECRET>

5. Rebuild dashboard if Spotting key set:
   cd /opt/aegisops/deploy/staging && docker compose up -d --build aegisops

6. Verify from workstation:
   .\scripts\staging-smoke.ps1 -BaseUrl $BaseUrl -Secret <AEGIS_WEBHOOK_SECRET>

"@ -ForegroundColor White

Write-Host "Phase 7 setup check complete." -ForegroundColor Green
