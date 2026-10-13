param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string]$GitHubSecret = "",
  [string]$Repo = "kkjayakumar/AegIsOps"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root "deploy\staging\.env"

function Get-EnvValue([string]$Key) {
  if (-not (Test-Path $envFile)) { return "" }
  $line = Get-Content $envFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

function Sign-GitHubBody([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return "sha256=" + ([BitConverter]::ToString($hash).Replace("-", "").ToLower())
}

Write-Host "=== Phase 6: Sentinel on staging (happy path) ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"

$verifyUrl = Get-EnvValue "DEPLOY_VERIFY_URL"
if (-not $verifyUrl) { $verifyUrl = "$BaseUrl/healthz" }
try {
  $hr = Invoke-WebRequest -Uri $verifyUrl -UseBasicParsing -TimeoutSec 20
  if ($hr.StatusCode -eq 200) {
    Write-Host "PASS DEPLOY_VERIFY_URL returns 200 ($verifyUrl)" -ForegroundColor Green
  } else {
    throw "HTTP $($hr.StatusCode)"
  }
} catch {
  throw "DEPLOY_VERIFY_URL unreachable: $verifyUrl — $($_.Exception.Message)"
}

if (-not $GitHubSecret) { $GitHubSecret = Get-EnvValue "GITHUB_WEBHOOK_SECRET" }
if (-not $GitHubSecret) {
  throw "Pass -GitHubSecret or set GITHUB_WEBHOOK_SECRET in deploy/staging/.env"
}

$runId = "9" + (Get-Random -Maximum 999999999).ToString().PadLeft(9, "0")
$sha = -join ((1..40) | ForEach-Object { "{0:x}" -f (Get-Random -Maximum 16) })
$body = @{
  action = "completed"
  repository = @{ full_name = $Repo }
  workflow_run = @{
    id = $runId
    name = "deploy"
    conclusion = "success"
    status = "completed"
    head_sha = $sha
    head_branch = "main"
  }
} | ConvertTo-Json -Compress -Depth 8

$sig = Sign-GitHubBody $body $GitHubSecret
$headers = @{
  "Content-Type" = "application/json"
  "X-GitHub-Event" = "workflow_run"
  "X-GitHub-Delivery" = [guid]::NewGuid().ToString()
  "X-Hub-Signature-256" = $sig
}
Invoke-RestMethod -Uri "$BaseUrl/api/v1/github/webhook" -Method POST -Headers $headers -Body $body | Out-Null
Write-Host "Sent deploy workflow_run success (run $runId)" -ForegroundColor Green
Start-Sleep -Seconds 3

$deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments" -TimeoutSec 20
$dep = $deps.deployments | Where-Object { $_.runId -eq $runId } | Select-Object -First 1
if (-not $dep) {
  throw "Deployment not recorded for run $runId (check production synthetic filter / webhook secret)"
}
Write-Host "PASS deployment recorded (id=$($dep.id))" -ForegroundColor Green

if ($dep.verifyStatus -eq "passed") {
  Write-Host "PASS sentinel verify_status=passed ($($dep.verifyMessage))" -ForegroundColor Green
} elseif ($dep.verifyStatus) {
  throw "Sentinel verify_status=$($dep.verifyStatus) message=$($dep.verifyMessage)"
} else {
  Write-Host "WARN verify_status empty — deploy API may need redeploy with latest image" -ForegroundColor Yellow
  Write-Host "On EC2: docker logs staging-aegisops-1 2>&1 | grep -i 'deploy health\\|Post-deploy'" -ForegroundColor Cyan
}

Write-Host "`nRollback path (optional): run test-phase6-sentinel.ps1 locally with bad DEPLOY_VERIFY_URL" -ForegroundColor DarkGray
Write-Host "Phase 6 staging sentinel check complete." -ForegroundColor Green
