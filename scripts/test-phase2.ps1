param(
  [string]$BaseUrl = "http://localhost:5001",
  [string]$Secret = "dev-webhook-secret"
)

function Sign-Body([string]$Body, [string]$Secret) {
  $hmac = New-Object System.Security.Cryptography.HMACSHA256
  $hmac.Key = [Text.Encoding]::UTF8.GetBytes($Secret)
  $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($Body))
  return ([BitConverter]::ToString($hash).Replace("-","").ToLower())
}

function Send-Webhook([string]$Path, [object]$Payload) {
  $body = $Payload | ConvertTo-Json -Compress -Depth 6
  $sig = Sign-Body $body $Secret
  $headers = @{ "Content-Type" = "application/json"; "X-Aegis-Signature" = "sha256=$sig" }
  Invoke-RestMethod -Uri "$BaseUrl$Path" -Method POST -Headers $headers -Body $body
}

Write-Host "=== Phase 2: correlated incident ===" -ForegroundColor Cyan
$cid = "req_phase2_" + [guid]::NewGuid().ToString().Substring(0,8)

Send-Webhook "/api/v1/webhooks/copilot" @{
  event = "sentry.issue.created"
  correlationId = $cid
  issue = @{ id = 99; projectId = "test"; title = "Prisma P2002 on signup"; fingerprint = "fp1"; count = 1; culprit = "auth.ts:42"; level = "error" }
} | Out-Null

Start-Sleep -Seconds 2

Send-Webhook "/api/v1/webhooks/spotting" @{
  event = "spotting.report.created"
  correlationId = $cid
  reportId = "rpt_phase2_001"
  severity = "error"
  pageUrl = "https://app.example/signup"
  networkFailures = @(@{ method = "POST"; url = "/api/signup"; status = 500 })
  consoleErrors = 1
} | Out-Null

Start-Sleep -Seconds 3

Write-Host "=== Issues API ===" -ForegroundColor Cyan
$issues = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues"
$issues | ConvertTo-Json -Depth 6

if ($issues.total -gt 0) {
  $id = $issues.issues[0].id
  Write-Host "=== Issue detail #$id ===" -ForegroundColor Cyan
  Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id" | ConvertTo-Json -Depth 6
  Write-Host "=== Bundle preview ===" -ForegroundColor Cyan
  Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id/bundle"
}

Write-Host "Phase 2 test complete." -ForegroundColor Green
