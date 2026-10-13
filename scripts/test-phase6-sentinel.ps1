param(
  [string]$BaseUrl = "http://localhost:5001",
  [string]$Repo = "kkjayakumar/AegIsOps",
  [string]$BadVerifyUrl = "http://127.0.0.1:59999/healthz-bad",
  [switch]$SkipRestore
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root ".env"
Set-Location $root

function Get-EnvValue([string]$Key) {
  $line = Get-Content $envFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

function Set-EnvValue([string]$Key, [string]$Value) {
  $lines = @()
  $found = $false
  foreach ($line in Get-Content $envFile) {
    if ($line -match "^$Key=") {
      $lines += "$Key=$Value"
      $found = $true
    } else {
      $lines += $line
    }
  }
  if (-not $found) { $lines += "$Key=$Value" }
  Set-Content -Path $envFile -Value $lines -Encoding UTF8
}

function Sign-GitHubBody([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return "sha256=" + ([BitConverter]::ToString($hash).Replace("-", "").ToLower())
}

Write-Host "=== Phase 6: Sentinel rollback path ===" -ForegroundColor Cyan
$secret = Get-EnvValue "GITHUB_WEBHOOK_SECRET"
if (-not $secret) { throw "GITHUB_WEBHOOK_SECRET missing" }

$prevVerify = Get-EnvValue "DEPLOY_VERIFY_URL"
if (-not $prevVerify) { $prevVerify = "http://127.0.0.1:5001/healthz" }

Write-Host "Temporarily setting DEPLOY_VERIFY_URL=$BadVerifyUrl" -ForegroundColor Yellow
Set-EnvValue "DEPLOY_VERIFY_URL" $BadVerifyUrl
docker compose up -d --force-recreate aegisops | Out-Null
Start-Sleep -Seconds 5

$runId = [int][double]::Parse(("7" + (Get-Random -Maximum 999999)))
$failedSha = "phase6sentinel" + (Get-Random -Maximum 99999)
$body = @{
  action = "completed"
  repository = @{ full_name = $Repo }
  workflow_run = @{
    id           = $runId
    name         = "deploy"
    conclusion   = "success"
    status       = "completed"
    head_sha     = $failedSha
    head_branch  = "main"
  }
} | ConvertTo-Json -Compress -Depth 8

$sig = Sign-GitHubBody $body $secret
$headers = @{
  "Content-Type"        = "application/json"
  "X-GitHub-Event"      = "workflow_run"
  "X-GitHub-Delivery"   = [guid]::NewGuid().ToString()
  "X-Hub-Signature-256" = $sig
}
Invoke-RestMethod -Uri "$BaseUrl/api/v1/github/webhook" -Method POST -Headers $headers -Body $body | Out-Null
Write-Host "Sent deploy workflow_run success (run $runId); health check should fail." -ForegroundColor Green
Start-Sleep -Seconds 3

$logText = docker logs aegisops-aegisops-1 2>&1 | Out-String
if ($logText -notmatch "deploy health check failed") {
  throw "Expected deploy health check failed in AegisOps logs (check DEPLOY_VERIFY_URL is not overridden in docker-compose)"
}
Write-Host "PASS: deploy health check failed as expected" -ForegroundColor Green

$rollbackDispatched = $logText -match "rollback workflow dispatched"
$rollbackFailed = $logText -match "github rollback dispatch failed"
if ($rollbackDispatched) {
  Write-Host "PASS: rollback workflow dispatched" -ForegroundColor Green
} elseif ($rollbackFailed) {
  Write-Host "WARN: rollback dispatch failed (PAT needs Actions write, rollback.yml on main)" -ForegroundColor Yellow
  if ($logText -match "github rollback dispatch failed[^\n]*error[^\n]*") {
    Write-Host ($Matches[0]) -ForegroundColor Yellow
  }
} else {
  Write-Host "WARN: no rollback dispatch log yet (GITHUB_TOKEN or ROLLBACK_WORKFLOW missing?)" -ForegroundColor Yellow
}

$ghToken = Get-EnvValue "GITHUB_TOKEN"
if ($ghToken) {
  try {
    $runs = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/actions/workflows/rollback.yml/runs?per_page=5" -Headers @{
      Authorization = "Bearer $ghToken"
      Accept        = "application/vnd.github+json"
    }
    $recent = $runs.workflow_runs | Where-Object { $_.event -eq "workflow_dispatch" } | Select-Object -First 1
    if ($recent) {
      Write-Host "Latest rollback workflow run: $($recent.html_url) (status=$($recent.status))" -ForegroundColor Cyan
    } else {
      Write-Host "No workflow_dispatch runs found for rollback.yml on GitHub yet." -ForegroundColor Yellow
      Write-Host "Push .github/workflows/rollback.yml to main, then re-run this script." -ForegroundColor Yellow
    }
  } catch {
    Write-Host "Could not query GitHub Actions: $($_.Exception.Message)" -ForegroundColor Yellow
  }
}

if (-not $SkipRestore) {
  Set-EnvValue "DEPLOY_VERIFY_URL" $prevVerify
  docker compose up -d --force-recreate aegisops | Out-Null
  Write-Host "Restored DEPLOY_VERIFY_URL=$prevVerify" -ForegroundColor Green
}

Write-Host ""
Write-Host "Phase 6 sentinel test complete." -ForegroundColor Green
