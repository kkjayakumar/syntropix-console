param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string[]]$KeepIds = @(),
  [switch]$IncludeFeat,
  [switch]$DryRun,
  [string[]]$Statuses = @("in_progress", "correlated")
)

$ErrorActionPreference = "Stop"

function Close-Issue([int]$Id) {
  if ($DryRun) {
    Write-Host "DRY RUN close #$Id" -ForegroundColor Yellow
    return
  }
  try {
    $res = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$Id/close" -Method POST -TimeoutSec 20
    Write-Host "CLOSED #$Id - $($res.issue.title)" -ForegroundColor Green
  } catch {
    $detail = $_.ErrorDetails.Message
    if ($detail -match "404|405|Not Found") {
      Write-Host "FAIL #$Id - close API not deployed yet. Run EC2 SQL fallback (see scripts/security-rotation-checklist.ps1 header or RUNBOOK)." -ForegroundColor Red
    } else {
      Write-Host "FAIL #$Id - $detail" -ForegroundColor Red
    }
  }
}

Write-Host "=== Close stale AegisOps inbox issues ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"
Write-Host "Statuses: $($Statuses -join ', ')"
if ($KeepIds.Count -gt 0) { Write-Host "Keeping IDs: $($KeepIds -join ', ')" }

$all = @()
foreach ($st in $Statuses) {
  $batch = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues?status=$st" -TimeoutSec 20
  $all += $batch.issues
}

$targets = $all | Where-Object {
  if ($KeepIds -contains $_.id) { return $false }
  if (-not $IncludeFeat -and $_.issueType -eq "feat") { return $false }
  return $true
} | Sort-Object { [int]$_.id }

if ($targets.Count -eq 0) {
  Write-Host "Nothing to close." -ForegroundColor Green
  exit 0
}

Write-Host "Closing $($targets.Count) issue(s)..." -ForegroundColor Cyan
foreach ($issue in $targets) {
  Close-Issue -Id $issue.id
}

$stats = Invoke-RestMethod -Uri "$BaseUrl/api/v1/stats" -TimeoutSec 20
Write-Host "`nInbox after: in_progress=$($stats.issuesByStatus.in_progress) correlated=$($stats.issuesByStatus.correlated) fixed=$($stats.issuesByStatus.fixed)" -ForegroundColor Cyan
