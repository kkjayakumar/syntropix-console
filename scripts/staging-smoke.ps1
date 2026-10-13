param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string]$Secret = "",
  [string]$GitHubSecret = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

function Get-EnvValue([string]$Path, [string]$Key) {
  if (-not (Test-Path $Path)) { return "" }
  $line = Get-Content $Path | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

$envCandidates = @(
  (Join-Path $root "deploy\staging\.env"),
  (Join-Path $root ".env")
)

if (-not $Secret) {
  foreach ($envFile in $envCandidates) {
    $Secret = Get-EnvValue $envFile "AEGIS_WEBHOOK_SECRET"
    if ($Secret) { break }
  }
}
if (-not $GitHubSecret) {
  foreach ($envFile in $envCandidates) {
    $GitHubSecret = Get-EnvValue $envFile "GITHUB_WEBHOOK_SECRET"
    if ($GitHubSecret) { break }
  }
}
if (-not $Secret) { $Secret = "dev-webhook-secret" }

Write-Host "=== Staging smoke: $BaseUrl ===" -ForegroundColor Cyan
& "$PSScriptRoot\e2e-smoke.ps1" -BaseUrl $BaseUrl -Secret $Secret -GitHubSecret $GitHubSecret

Write-Host "=== Staging: gateway approve flow (dry) ===" -ForegroundColor Cyan
$list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues?type=fix"
if ($list.total -gt 0) {
  Write-Host "Latest issue status: $($list.issues[0].status)" -ForegroundColor Green
}

Write-Host "=== Staging: stats ===" -ForegroundColor Cyan
Invoke-RestMethod -Uri "$BaseUrl/api/v1/stats" | ConvertTo-Json -Depth 4

Write-Host "Staging smoke complete." -ForegroundColor Green
