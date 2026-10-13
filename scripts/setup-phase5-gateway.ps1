param(
  [string]$AegisUrl = "http://localhost:5001",
  [string]$EnvFile = "",
  [switch]$SendTest
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not $EnvFile) { $EnvFile = Join-Path $root ".env" }

function Get-EnvValue([string]$Key) {
  if (-not (Test-Path $EnvFile)) { return "" }
  $line = Get-Content $EnvFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

function Test-TeamsWebhookUrl([string]$Url) {
  if (-not $Url) {
    return @{ valid = $false; hint = "TEAMS_WEBHOOK_URL is empty" }
  }
  if ($Url -match "^https://([a-z0-9.-]*\.)?webhook\.office\.com/") {
    return @{ valid = $true; hint = "" }
  }
  if ($Url -match "teams\.microsoft\.com/l/(channel|team)/") {
    return @{
      valid = $false
      hint  = "Teams browse link - cannot POST. Use Power Automate HTTP trigger or Incoming Webhook"
    }
  }
  if ($Url -match "powerplatform\.com|logic\.azure\.com|powerautomate") {
    return @{ valid = $true; kind = "powerautomate"; hint = "" }
  }
  if ($Url -match "^https://") {
    return @{ valid = $true; hint = "Unrecognized host; if POST fails, recreate webhook" }
  }
  return @{ valid = $false; hint = "Must be https Incoming Webhook URL" }
}

function Test-SlackWebhookUrl([string]$Url) {
  if (-not $Url) { return @{ valid = $false; hint = "SLACK_WEBHOOK_URL is empty (optional)" } }
  if ($Url -match "^https://hooks\.slack\.com/services/") {
    return @{ valid = $true; hint = "" }
  }
  return @{ valid = $false; hint = "Expected https://hooks.slack.com/services/..." }
}

Write-Host "=== Phase 5 setup: notification URLs ===" -ForegroundColor Cyan

$teamsUrl = Get-EnvValue "TEAMS_WEBHOOK_URL"
$slackUrl = Get-EnvValue "SLACK_WEBHOOK_URL"
$publicUrl = Get-EnvValue "AEGISOPS_PUBLIC_URL"

$teamsCheck = Test-TeamsWebhookUrl $teamsUrl
$slackCheck = Test-SlackWebhookUrl $slackUrl

if ($teamsCheck.valid) {
  $kind = if ($teamsCheck.kind -eq "powerautomate") { " (Power Automate HTTP trigger)" } else { "" }
  Write-Host "TEAMS_WEBHOOK_URL: OK$kind" -ForegroundColor Green
} else {
  Write-Host "TEAMS_WEBHOOK_URL: INVALID - $($teamsCheck.hint)" -ForegroundColor Red
  Write-Host @"

Fix Teams (one-time):
  1. Teams channel -> ... -> Connectors -> Incoming Webhook -> Configure
  2. Paste URL into .env as TEAMS_WEBHOOK_URL=
  3. docker compose up -d aegisops

See docs/GATEWAY.md
"@ -ForegroundColor Yellow
}

if ($slackCheck.valid) {
  Write-Host "SLACK_WEBHOOK_URL: OK" -ForegroundColor Green
} elseif ($slackUrl) {
  Write-Host "SLACK_WEBHOOK_URL: INVALID - $($slackCheck.hint)" -ForegroundColor Yellow
} else {
  Write-Host "SLACK_WEBHOOK_URL: not set (optional)" -ForegroundColor Yellow
}

if ($publicUrl -match "localhost|127\.0\.0\.1") {
  Write-Host "AEGISOPS_PUBLIC_URL=$publicUrl (local only - use tunnel/staging for remote team)" -ForegroundColor Yellow
} elseif ($publicUrl) {
  Write-Host "AEGISOPS_PUBLIC_URL=$publicUrl" -ForegroundColor Green
} else {
  Write-Host "AEGISOPS_PUBLIC_URL not set" -ForegroundColor Yellow
}

if ($SendTest) {
  Write-Host "`n=== Send test approval card ===" -ForegroundColor Cyan
  $base = if ($publicUrl) { $publicUrl.TrimEnd("/") } else { $AegisUrl.TrimEnd("/") }
  $approve = '{0}/api/v1/gateway/approve?token=test&format=html' -f $base
  $reject = '{0}/api/v1/gateway/reject?token=test&format=html' -f $base

  if ($teamsCheck.valid) {
    if ($teamsCheck.kind -eq "powerautomate") {
      $card = @{
        kind       = "approval"
        title      = "AegisOps gateway test"
        message    = "Power Automate trigger works. Approve/Reject links are in message and actions[] fields."
        severity   = "warning"
        approveUrl = $approve
        rejectUrl  = $reject
      } | ConvertTo-Json -Compress
    } else {
      $card = @{
        "@type"      = "MessageCard"
        "@context"   = "http://schema.org/extensions"
        "themeColor" = "0078D7"
        "summary"    = "AegisOps gateway test"
        "sections"   = @(@{ "activityTitle" = "AegisOps gateway test"; "text" = "If you see this, Incoming Webhook works." })
        "potentialAction" = @(
          @{ "@type" = "OpenUri"; "name" = "Approve Fix (sample)"; "targets" = @(@{ "os" = "default"; "uri" = $approve }) },
          @{ "@type" = "OpenUri"; "name" = "Reject (sample)"; "targets" = @(@{ "os" = "default"; "uri" = $reject }) }
        )
      } | ConvertTo-Json -Depth 6 -Compress
    }
    try {
      Invoke-RestMethod -Uri $teamsUrl -Method POST -Body $card -ContentType "application/json" | Out-Null
      Write-Host "Teams test card sent" -ForegroundColor Green
    } catch {
      Write-Host "Teams POST failed: $($_.Exception.Message)" -ForegroundColor Red
    }
  }

  if ($slackCheck.valid) {
    $slackBody = @{
      text = "*AegisOps gateway test*`nIncoming Webhook works. Sample links only."
      blocks = @(
        @{ type = "section"; text = @{ type = "mrkdwn"; text = "*AegisOps gateway test*`nIf you see this, Slack webhook works." } },
        @{
          type = "actions"
          elements = @(
            @{ type = "button"; text = @{ type = "plain_text"; text = "Approve (sample)" }; url = $approve },
            @{ type = "button"; text = @{ type = "plain_text"; text = "Reject (sample)" }; url = $reject; style = "danger" }
          )
        }
      )
    } | ConvertTo-Json -Depth 8 -Compress
    try {
      Invoke-RestMethod -Uri $slackUrl -Method POST -Body $slackBody -ContentType "application/json" | Out-Null
      Write-Host "Slack test message sent" -ForegroundColor Green
    } catch {
      Write-Host "Slack POST failed: $($_.Exception.Message)" -ForegroundColor Red
    }
  }
}

Write-Host "`nNext: .\scripts\test-phase5-gateway.ps1" -ForegroundColor Cyan
