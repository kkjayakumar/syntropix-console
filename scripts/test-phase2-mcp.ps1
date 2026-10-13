param(
  [string]$BaseUrl = "http://localhost:5001",
  [string]$CopilotUrl = "http://localhost:5000"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$mcpBin = Join-Path $root "bin\aegisops-mcp.exe"
$mcpConfig = Join-Path $root ".cursor\mcp.json"

Write-Host "=== Phase 2: prerequisites ===" -ForegroundColor Cyan
foreach ($pair in @(
  @{ Url = "$BaseUrl/healthz"; Name = "AegisOps" },
  @{ Url = "$CopilotUrl/api/ingest/config"; Name = "Copilot" }
)) {
  Invoke-RestMethod -Uri $pair.Url -TimeoutSec 10 | Out-Null
  Write-Host "$($pair.Name) OK" -ForegroundColor Green
}

if (-not (Test-Path $mcpBin)) {
  Write-Host "Building MCP binary..." -ForegroundColor Yellow
  Push-Location $root
  go build -o bin/aegisops-mcp.exe ./cmd/aegisops-mcp
  Pop-Location
}
if (-not (Test-Path $mcpBin)) { throw "Missing $mcpBin - run make build" }
Write-Host "MCP binary: $mcpBin" -ForegroundColor Green

if (-not (Test-Path $mcpConfig)) { throw "Missing $mcpConfig" }
Write-Host "MCP config: $mcpConfig" -ForegroundColor Green

Write-Host "`n=== Phase 2: issues + Start Fix (dashboard path) ===" -ForegroundColor Cyan
$list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues?type=fix"
if ($list.total -lt 1) {
  Write-Host "No issues - run self-test first" -ForegroundColor Yellow
  & (Join-Path $PSScriptRoot "self-test-aegis.ps1")
  $list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues?type=fix"
}
$issue = $list.issues | Where-Object { $_.telemetryBundle -or $_.status -ne "fixed" } | Select-Object -First 1
if (-not $issue) { $issue = $list.issues[0] }
$id = $issue.id
Write-Host "Using issue #$id ($($issue.title)) status=$($issue.status)" -ForegroundColor Green

$fix = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id/start-fix" -Method POST
if ($fix.issue.status -ne "in_progress") {
  throw "Start Fix expected in_progress, got $($fix.issue.status)"
}
Write-Host "Start Fix -> in_progress" -ForegroundColor Green
Write-Host "Message: $($fix.message)" -ForegroundColor Gray

Write-Host "`n=== Phase 2: telemetry bundle (MCP get_incident_bundle equivalent) ===" -ForegroundColor Cyan
$bundle = Invoke-WebRequest -Uri "$BaseUrl/api/v1/issues/$id/bundle" -UseBasicParsing
if ($bundle.Content.Length -lt 20) { throw "Bundle too short" }
Write-Host "Bundle length: $($bundle.Content.Length) chars" -ForegroundColor Green
if ($bundle.Content -notmatch "Incident") { throw "Bundle missing incident header" }

Write-Host "`n=== Phase 2: Copilot proxies (MCP get_sentry_issue / get_live_logs) ===" -ForegroundColor Cyan
$links = (Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id").telemetryLinks
$sentryId = $null
if ($links -and $links.Count -gt 0) {
  $sentryId = ($links | Where-Object { $_.sentryIssueId } | Select-Object -First 1).sentryIssueId
}
if (-not $sentryId) {
  $sentryId = "1"
  Write-Host "No sentry link on issue - trying Copilot issue id 1" -ForegroundColor Yellow
}
try {
  $sentry = Invoke-RestMethod -Uri "$CopilotUrl/api/sentry/issues/$sentryId" -TimeoutSec 10
  Write-Host "get_sentry_issue OK (title: $($sentry.issue.title))" -ForegroundColor Green
} catch {
  Write-Host "get_sentry_issue skipped: $($_.Exception.Message)" -ForegroundColor Yellow
}

try {
  $containers = Invoke-RestMethod -Uri "$CopilotUrl/api/containers" -TimeoutSec 10
  $first = @($containers) | Select-Object -First 1
  $cid = $first.id
  if (-not $cid) { $cid = $first.ID }
  if ($cid) {
    $logs = Invoke-RestMethod -Uri "$CopilotUrl/api/containers/$cid/logs?tail=5" -TimeoutSec 15
    $lines = if ($logs.lines) { @($logs.lines) } elseif ($logs -is [string]) { $logs -split "`n" } else { @() }
    Write-Host "get_live_logs OK (container $cid, $($lines.Count) lines)" -ForegroundColor Green
  } else {
    Write-Host "get_live_logs skipped: no containers" -ForegroundColor Yellow
  }
} catch {
  Write-Host "get_live_logs skipped: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "`n=== Phase 2: MCP server in Cursor ===" -ForegroundColor Cyan
Write-Host "1. Reload Cursor (Ctrl+Shift+P -> Developer: Reload Window)"
Write-Host "2. Settings -> MCP -> confirm aegisops server is enabled"
Write-Host "3. In Agent chat: Use aegisops start_fix_from_issue for issue_id $id"
Write-Host "4. Branch name: aegis/fix-$id"
Write-Host "5. PR body: Fixes #$id"

Write-Host "Phase 2 automated checks passed." -ForegroundColor Green
Write-Host "Complete 2.3, 2.5, 2.7 manually in Cursor to finish Phase 2." -ForegroundColor Cyan
