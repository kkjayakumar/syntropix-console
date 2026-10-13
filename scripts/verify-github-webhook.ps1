param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string]$GitHubSecret = "",
  [int]$IssueId = 0
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

function Send-GitHubWebhook([string]$Event, [object]$Payload) {
  if (-not $GitHubSecret) {
    $GitHubSecret = Get-EnvValue "GITHUB_WEBHOOK_SECRET"
  }
  if (-not $GitHubSecret) { throw "Pass -GitHubSecret or set GITHUB_WEBHOOK_SECRET in deploy/staging/.env" }
  $body = $Payload | ConvertTo-Json -Compress -Depth 12
  $sig = Sign-GitHubBody $body $GitHubSecret
  $headers = @{
    "Content-Type"        = "application/json"
    "X-GitHub-Event"      = $Event
    "X-GitHub-Delivery"   = [guid]::NewGuid().ToString()
    "X-Hub-Signature-256" = $sig
  }
  return Invoke-WebRequest -Uri "$BaseUrl/api/v1/github/webhook" -Method POST -Headers $headers -Body $body -UseBasicParsing
}

Write-Host "=== Verify GitHub webhook -> AegisOps ===" -ForegroundColor Cyan
Write-Host "URL: $BaseUrl/api/v1/github/webhook"

Write-Host "`n--- workflow_run (CI/CD page) ---" -ForegroundColor Cyan
Write-Host "NOTE: production ignores synthetic SHAs (abc1234) — only real GitHub Actions runs are stored." -ForegroundColor Yellow
$r = Send-GitHubWebhook "workflow_run" @{
  action = "completed"
  repository = @{ full_name = "kkjayakumar/AegIsOps" }
  workflow_run = @{
    id = "999001"
    name = "CI"
    head_sha = "abc1234"
    head_branch = "main"
    conclusion = "success"
    status = "completed"
  }
}
Write-Host "PASS workflow_run -> $($r.StatusCode)" -ForegroundColor Green

Write-Host "`n--- push (auto PR; optional issue id) ---" -ForegroundColor Cyan
$id = $IssueId
if ($id -le 0) { $id = 1 }
$r2 = Send-GitHubWebhook "push" @{
  ref = "refs/heads/aegis/fix-$id"
  deleted = $false
  repository = @{ full_name = "kkjayakumar/AegIsOps" }
}
Write-Host "PASS push aegis/fix-$id -> $($r2.StatusCode)" -ForegroundColor Green
Write-Host "NOTE: auto-PR requires branch on GitHub + GITHUB_APP or GITHUB_TOKEN on server" -ForegroundColor Yellow

Write-Host "`n--- deployments API ---" -ForegroundColor Cyan
$deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments" -TimeoutSec 20
$count = @($deps.deployments).Count
Write-Host "Deployments visible: $count" -ForegroundColor $(if ($count -gt 0) { "Green" } else { "Yellow" })
if ($count -eq 0) {
  Write-Host "Enable workflow_run on GitHub webhook if CI/CD page is empty." -ForegroundColor Yellow
}

Write-Host "`nGitHub webhook verification complete." -ForegroundColor Green
