param(
  [string]$AegisUrl = "http://localhost:5001",
  [int]$WaitSec = 12
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

function Get-GatewayTokens([long]$IssueId) {
  $pg = docker ps --format "{{.Names}}" | Where-Object { $_ -match "aegisops-postgres" } | Select-Object -First 1
  if (-not $pg) { throw "aegisops-postgres container not running" }
  $sql = "SELECT action, token FROM aegis_gateway_tokens WHERE issue_id = $IssueId AND used = false ORDER BY action;"
  $lines = docker exec $pg psql -U aegisops -d aegisops -t -A -c $sql 2>$null
  $tokens = @{}
  foreach ($line in ($lines -split "`n")) {
    $line = $line.Trim()
    if (-not $line) { continue }
    $parts = $line -split "\|"
    if ($parts.Count -ge 2) {
      $tokens[$parts[0].Trim()] = $parts[1].Trim()
    }
  }
  return $tokens
}

function New-TestIssue() {
  Write-Host "Creating incident via self-test..." -ForegroundColor Cyan
  & (Join-Path $root "scripts\self-test-aegis.ps1") -AegisUrl $AegisUrl | Out-Null
  Start-Sleep -Seconds $WaitSec
  $list = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues?type=fix" -TimeoutSec 15
  if ($list.total -lt 1) { throw "No issues after self-test" }
  return $list.issues[0]
}

function Invoke-Gateway([string]$Action, [string]$Token) {
  $uri = "$AegisUrl/api/v1/gateway/$Action`?token=$Token"
  return Invoke-RestMethod -Uri $uri -TimeoutSec 15
}

Write-Host "=== Phase 5: gateway approve/reject ===" -ForegroundColor Cyan
Invoke-RestMethod -Uri "$AegisUrl/healthz" -TimeoutSec 5 | Out-Null

# --- Approve ---
$issue = New-TestIssue
Write-Host "Issue #$($issue.id) status=$($issue.status)" -ForegroundColor Green

$tokens = Get-GatewayTokens -IssueId $issue.id
if (-not $tokens["approve"]) { throw "No approve token for issue #$($issue.id)" }

Write-Host "Calling gateway approve..." -ForegroundColor Cyan
$approveResult = Invoke-Gateway -Action "approve" -Token $tokens["approve"]
if ($approveResult.action -ne "approve") { throw "Unexpected approve response" }

$afterApprove = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues/$($issue.id)" -TimeoutSec 15
if ($afterApprove.issue.status -ne "in_progress") {
  throw "Expected in_progress after approve, got $($afterApprove.issue.status)"
}
Write-Host "Approve OK -> in_progress" -ForegroundColor Green

# --- Reject (fresh issue) ---
$issue2 = New-TestIssue
Write-Host "Issue #$($issue2.id) for reject test" -ForegroundColor Green
$tokens2 = Get-GatewayTokens -IssueId $issue2.id
if (-not $tokens2["reject"]) { throw "No reject token for issue #$($issue2.id)" }

Write-Host "Calling gateway reject..." -ForegroundColor Cyan
$rejectResult = Invoke-Gateway -Action "reject" -Token $tokens2["reject"]
if ($rejectResult.action -ne "reject") { throw "Unexpected reject response" }

$afterReject = Invoke-RestMethod -Uri "$AegisUrl/api/v1/issues/$($issue2.id)" -TimeoutSec 15
if ($afterReject.issue.status -ne "closed") {
  throw "Expected closed after reject, got $($afterReject.issue.status)"
}
Write-Host "Reject OK -> closed" -ForegroundColor Green

Write-Host "`nPhase 5 gateway flow passed." -ForegroundColor Green
Write-Host "For live Teams/Slack: fix TEAMS_WEBHOOK_URL then .\scripts\setup-phase5-gateway.ps1 -SendTest"
