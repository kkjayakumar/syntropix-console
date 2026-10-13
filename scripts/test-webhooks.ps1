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
  $body = $Payload | ConvertTo-Json -Compress -Depth 5
  $sig = Sign-Body $body $Secret
  $headers = @{ "Content-Type" = "application/json"; "X-Aegis-Signature" = "sha256=$sig" }
  $res = Invoke-RestMethod -Uri "$BaseUrl$Path" -Method POST -Headers $headers -Body $body
  $res | ConvertTo-Json
}

Write-Host "=== Copilot webhook ===" -ForegroundColor Cyan
Send-Webhook "/api/v1/webhooks/copilot" @{
  event = "sentry.issue.created"
  correlationId = "req_test_001"
  issue = @{ id = 1; projectId = "test"; title = "Test Error"; fingerprint = "abc"; count = 1 }
}

Write-Host "=== Spotting webhook ===" -ForegroundColor Cyan
Send-Webhook "/api/v1/webhooks/spotting" @{
  event = "spotting.report.created"
  correlationId = "req_test_001"
  reportId = "rpt_test_001"
  severity = "error"
  networkFailures = @(@{ method = "POST"; url = "/api/signup"; status = 500 })
  consoleErrors = 1
}

Write-Host "=== List events ===" -ForegroundColor Cyan
Invoke-RestMethod -Uri "$BaseUrl/api/v1/webhooks/events" | ConvertTo-Json -Depth 5