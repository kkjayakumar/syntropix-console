param(
  [string]$BaseUrl = "http://localhost:5001",
  [int]$IssueId = 10,
  [string]$Repo = "kkjayakumar/AegIsOps"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root ".env"

function Get-EnvValue([string]$Key) {
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
  $secret = Get-EnvValue "GITHUB_WEBHOOK_SECRET"
  if (-not $secret) { throw "GITHUB_WEBHOOK_SECRET missing in .env" }
  $body = $Payload | ConvertTo-Json -Compress -Depth 12
  $sig = Sign-GitHubBody $body $secret
  $headers = @{
    "Content-Type"       = "application/json"
    "X-GitHub-Event"     = $Event
    "X-GitHub-Delivery"  = [guid]::NewGuid().ToString()
    "X-Hub-Signature-256" = $sig
  }
  Invoke-RestMethod -Uri "$BaseUrl/api/v1/github/webhook" -Method POST -Headers $headers -Body $body
}

Write-Host "=== Phase 3: health ===" -ForegroundColor Cyan
$h = Invoke-RestMethod -Uri "$BaseUrl/healthz"
if ($h.status -ne "ok") { throw "health check failed" }

Write-Host "=== Phase 3: PR opened (aegis/fix-$IssueId) ===" -ForegroundColor Cyan
Send-GitHubWebhook "pull_request" @{
  action = "opened"
  pull_request = @{
    merged = $false
    body   = "Fixes #$IssueId"
    head   = @{ ref = "aegis/fix-$IssueId" }
    base   = @{ repo = @{ full_name = $Repo } }
  }
} | Out-Null
Start-Sleep -Seconds 1
$afterOpen = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$IssueId"
Write-Host "Issue #$IssueId status=$($afterOpen.issue.status)" -ForegroundColor Green
if ($afterOpen.issue.status -ne "pr_created") {
  throw "expected pr_created after PR opened, got $($afterOpen.issue.status)"
}

Write-Host "=== Phase 3: PR merged ===" -ForegroundColor Cyan
Send-GitHubWebhook "pull_request" @{
  action = "closed"
  pull_request = @{
    merged = $true
    body   = "Fixes #$IssueId"
    head   = @{ ref = "aegis/fix-$IssueId" }
    base   = @{ repo = @{ full_name = $Repo } }
  }
} | Out-Null
Start-Sleep -Seconds 1
$afterMerge = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$IssueId"
Write-Host "Issue #$IssueId status=$($afterMerge.issue.status)" -ForegroundColor Green
if ($afterMerge.issue.status -ne "fixed") {
  throw "expected fixed after PR merge, got $($afterMerge.issue.status)"
}

Write-Host "=== Phase 3: workflow_run failure ===" -ForegroundColor Cyan
$runId = [int][double]::Parse(("9" + (Get-Random -Maximum 999999)))
Send-GitHubWebhook "workflow_run" @{
  action = "completed"
  repository = @{ full_name = $Repo }
  workflow_run = @{
    id           = $runId
    name         = "CI"
    conclusion   = "failure"
    status       = "completed"
    head_sha     = "deadbeef1234567890"
    head_branch  = "main"
  }
} | Out-Null
$deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments"
Write-Host "Deployments total: $($deps.total)" -ForegroundColor Green

Write-Host "=== Phase 3: workflow_run success (deploy path) ===" -ForegroundColor Cyan
$runId2 = [int][double]::Parse(("8" + (Get-Random -Maximum 999999)))
Send-GitHubWebhook "workflow_run" @{
  action = "completed"
  repository = @{ full_name = $Repo }
  workflow_run = @{
    id           = $runId2
    name         = "deploy"
    conclusion   = "success"
    status       = "completed"
    head_sha     = "abc123def456"
    head_branch  = "main"
  }
} | Out-Null
Write-Host "Deploy workflow_run accepted" -ForegroundColor Green

Write-Host ""
Write-Host "Phase 3 local webhook tests passed." -ForegroundColor Green
Write-Host "Next: add live GitHub webhook per docs/GITHUB_WEBHOOK.md (ngrok for local)." -ForegroundColor Cyan
