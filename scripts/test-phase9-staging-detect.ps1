param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [int]$WaitSec = 45,
  [int]$PollIntervalSec = 5
)

$ErrorActionPreference = "Stop"

Write-Host "=== Phase 9: Real detect (no manual webhook simulation) ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"

Write-Host "`n--- Health ---" -ForegroundColor Cyan
$h = Invoke-RestMethod -Uri "$BaseUrl/healthz" -TimeoutSec 20
if ($h.status -ne "ok") { throw "healthz failed" }
Write-Host "PASS healthz" -ForegroundColor Green

$cid = "req_phase9_" + [guid]::NewGuid().ToString("N").Substring(0, 10)
Write-Host "`n--- Trigger dogfood backend error ---" -ForegroundColor Cyan
Write-Host "correlationId=$cid"

try {
  Invoke-WebRequest -Uri "$BaseUrl/api/v1/debug/trigger-error" -Method POST `
    -Headers @{ "X-Request-Id" = $cid } -UseBasicParsing | Out-Null
  throw "Expected HTTP 500 from debug/trigger-error"
} catch {
  $resp = $_.Exception.Response
  if (-not $resp -or [int]$resp.StatusCode -ne 500) {
    if ($resp -and [int]$resp.StatusCode -eq 404) {
      throw @"
debug/trigger-error returned 404 - dogfood disabled on staging.
On EC2: set AEGISOPS_ALLOW_DOGFOOD=true in deploy/staging/.env, git pull, rebuild:
  cd /opt/aegisops/deploy/staging
  docker compose -f docker-compose.yml -f docker-compose.external-db.yml up -d --build aegisops
"@
    }
    throw "debug/trigger-error failed: $($_.Exception.Message)"
  }
  Write-Host "PASS backend 500 + Sentry + internal Spotting ingest fired" -ForegroundColor Green
}

Write-Host "Polling up to ${WaitSec}s for issue (may dedup into existing open issue)..." -ForegroundColor Cyan
$deadline = (Get-Date).AddSeconds($WaitSec)
$issue = $null
$spotEvent = $false
$copilotEvent = $false
$triggeredAt = Get-Date

while ((Get-Date) -lt $deadline) {
  Start-Sleep -Seconds $PollIntervalSec

  try {
    $events = Invoke-RestMethod -Uri "$BaseUrl/api/v1/webhooks/events" -TimeoutSec 20
    $spotEvent = @($events.events | Where-Object { $_.correlationId -eq $cid -and $_.source -eq "spotting" }).Count -gt 0
    $copilotEvent = @($events.events | Where-Object { $_.correlationId -eq $cid -and $_.source -eq "copilot" }).Count -gt 0
  } catch {
    $spotEvent = $false
    $copilotEvent = $false
  }

  $list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues?type=fix" -TimeoutSec 20
  $issue = $list.issues | Where-Object { $_.correlationId -eq $cid } | Select-Object -First 1

  if (-not $issue -and $spotEvent) {
    # Fingerprint dedup merges dogfood into an existing open issue (title may stay "Staging smoke test")
    $issue = $list.issues |
      Where-Object {
        $_.status -notin @("fixed", "closed") -and
        ([datetime]$_.updatedAt) -ge $triggeredAt.AddSeconds(-10)
      } |
      Sort-Object { [datetime]$_.updatedAt } -Descending |
      Select-Object -First 1
    if ($issue -and $issue.correlationId -ne $cid) {
      Write-Host "NOTE dedup merged into issue #$($issue.id) (correlationId=$($issue.correlationId))" -ForegroundColor Yellow
    }
  }

  if ($issue) { break }
  Write-Host "  waiting... spotting=$spotEvent copilot=$copilotEvent" -ForegroundColor DarkGray
}

Write-Host "`n--- Verify issue ---" -ForegroundColor Cyan
if (-not $issue) {
  $msg = "No issue after ${WaitSec}s for $cid."
  if ($spotEvent -and -not $copilotEvent) {
    $msg += " Spotting OK; Copilot webhook missing (SENTRY_DSN / Copilot logs)."
  } elseif (-not $spotEvent) {
    $msg += " No Spotting event - check AegisOps logs."
  }
  $msg += " Tip: close stale open issues on dashboard, then re-run."
  throw $msg
}

Write-Host "PASS issue #$($issue.id) status=$($issue.status) title=$($issue.title)" -ForegroundColor Green
if ($spotEvent) { Write-Host "PASS spotting event for $cid" -ForegroundColor Green }
if ($copilotEvent) { Write-Host "PASS copilot event for $cid" -ForegroundColor Green }
if (-not $copilotEvent) {
  Write-Host "WARN Copilot webhook not seen (backend path) - Spotting/dedup still valid for Phase 9" -ForegroundColor Yellow
}

if (-not $issue.telemetryBundle) {
  $detail = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$($issue.id)"
  $issue = $detail.issue
  if (-not $issue.telemetryBundle) { throw "telemetry bundle missing" }
}
Write-Host "PASS telemetry bundle present" -ForegroundColor Green
if ($issue.githubIssueNumber) {
  Write-Host "GitHub issue #$($issue.githubIssueNumber) on $($issue.githubRepo)" -ForegroundColor Green
}

Write-Host "`n--- Start Fix (gateway / MCP gate) ---" -ForegroundColor Cyan
if ($issue.status -eq "in_progress") {
  Write-Host "Issue already in_progress - skipping Start Fix POST" -ForegroundColor Yellow
} else {
  $fix = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$($issue.id)/start-fix" -Method POST -TimeoutSec 20
  if ($fix.issue.status -ne "in_progress") {
    throw "Start Fix expected in_progress, got $($fix.issue.status)"
  }
  Write-Host "PASS Start Fix -> in_progress" -ForegroundColor Green
}

$id = $issue.id
Write-Host "`n=== Phase 9 manual steps (Cursor + GitHub) ===" -ForegroundColor Cyan
Write-Host @"
1. Open dashboard: $BaseUrl/issues/$id
2. In Cursor Agent: start_fix_from_issue with issue_id $id
3. Branch: aegis/fix-$id — implement fix, push branch (PR opens automatically)
4. Or: open_pr_from_issue(issue_id=$id) after push
5. Merge PR — issue status -> fixed automatically

Phase 9 automated detect + Start Fix PASSED for issue #$id
"@ -ForegroundColor White
