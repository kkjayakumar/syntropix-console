param(
  [string]$BaseUrl = "http://localhost:5001",
  [string]$Secret = "dev-webhook-secret",
  [string]$GitHubSecret = ""
)

function Sign-Body([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return ([BitConverter]::ToString($hash).Replace("-","").ToLower())
}

function Send-AegisWebhook([string]$Path, [object]$Payload) {
  $body = $Payload | ConvertTo-Json -Compress -Depth 8
  $sig = Sign-Body $body $Secret
  $headers = @{ "Content-Type" = "application/json"; "X-Aegis-Signature" = "sha256=$sig" }
  Invoke-RestMethod -Uri "$BaseUrl$Path" -Method POST -Headers $headers -Body $body
}

function Send-GitHubWebhook([string]$Event, [object]$Payload) {
  $ghSecret = if ($GitHubSecret) { $GitHubSecret } else { $Secret }
  $body = $Payload | ConvertTo-Json -Compress -Depth 10
  $headers = @{
    "Content-Type"      = "application/json"
    "X-GitHub-Event"    = $Event
    "X-GitHub-Delivery" = [guid]::NewGuid().ToString()
  }
  if ($ghSecret) {
    $headers["X-Hub-Signature-256"] = "sha256=$(Sign-Body $body $ghSecret)"
  }
  Invoke-RestMethod -Uri "$BaseUrl/api/v1/github/webhook" -Method POST -Headers $headers -Body $body
}

Write-Host "=== E2E smoke: health ===" -ForegroundColor Cyan
$health = Invoke-RestMethod -Uri "$BaseUrl/healthz"
if ($health.status -ne "ok") { throw "health check failed" }

Write-Host "=== E2E smoke: correlated incident (phase 2 flow) ===" -ForegroundColor Cyan
$cid = "req_e2e_" + [guid]::NewGuid().ToString().Substring(0,8)
Send-AegisWebhook "/api/v1/webhooks/copilot" @{
  event = "sentry.issue.created"
  correlationId = $cid
  issue = @{ id = 1001; title = "E2E smoke backend error"; level = "error" }
} | Out-Null
Start-Sleep -Seconds 1
Send-AegisWebhook "/api/v1/webhooks/spotting" @{
  event = "spotting.report.created"
  correlationId = $cid
  reportId = "rpt_e2e_001"
  severity = "error"
  pageUrl = "https://app.example/e2e"
  networkFailures = @(@{ method = "GET"; url = "/api/e2e"; status = 500 })
} | Out-Null
Start-Sleep -Seconds 3

$list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues"
if ($list.total -lt 1) { throw "expected at least one issue after webhooks" }
$issue = $list.issues | Where-Object { $_.correlationId -eq $cid } | Select-Object -First 1
if (-not $issue) { $issue = $list.issues[0] }
$id = $issue.id
Write-Host "Issue #$id status=$($issue.status)" -ForegroundColor Green

if (-not $issue.telemetryBundle) {
  $detail = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id"
  if (-not $detail.issue.telemetryBundle) { throw "telemetry bundle missing" }
}
Write-Host "Bundle present" -ForegroundColor Green

Write-Host "=== E2E smoke: PR lifecycle ===" -ForegroundColor Cyan
Send-GitHubWebhook "pull_request" @{
  action = "opened"
  pull_request = @{
    merged = $false
    body = "Automated e2e PR"
    head = @{ ref = "aegis/fix-$id" }
    base = @{ repo = @{ full_name = "owner/repo" } }
  }
} | Out-Null
Start-Sleep -Seconds 1
$afterOpen = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id"
if ($afterOpen.issue.status -ne "pr_created") {
  throw "expected pr_created after PR opened, got $($afterOpen.issue.status)"
}
Write-Host "PR opened -> pr_created" -ForegroundColor Green

Send-GitHubWebhook "pull_request" @{
  action = "closed"
  pull_request = @{
    merged = $true
    body = "Fixes #$id"
    head = @{ ref = "aegis/fix-$id" }
    base = @{ repo = @{ full_name = "owner/repo" } }
  }
} | Out-Null
Start-Sleep -Seconds 1
$afterMerge = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id"
if ($afterMerge.issue.status -ne "fixed") {
  throw "expected fixed after PR merge, got $($afterMerge.issue.status)"
}
Write-Host "PR merged -> fixed" -ForegroundColor Green

Write-Host "=== E2E smoke: deploy success webhook ===" -ForegroundColor Cyan
Send-GitHubWebhook "workflow_run" @{
  action = "completed"
  repository = @{ full_name = "owner/repo" }
  workflow_run = @{
    id = [int][double]::Parse(("1" + (Get-Random -Maximum 999999)))
    name = "deploy"
    conclusion = "success"
    status = "completed"
    head_sha = "abc123def456"
    head_branch = "main"
  }
} | Out-Null
Write-Host "workflow_run success accepted" -ForegroundColor Green

$deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments"
if ($deps.total -lt 1) { throw "expected deployment row" }
Write-Host "Deployments total: $($deps.total)" -ForegroundColor Green

Write-Host "E2E smoke test passed." -ForegroundColor Green
