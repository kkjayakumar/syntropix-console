param(
  [string]$BaseUrl = "https://aegisops.syntropix.io"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

Write-Host "=== AegisOps fix plan (run in order) ===" -ForegroundColor Cyan

Write-Host "`n[1/5] Issue broker - GitHub auto-issues" -ForegroundColor Yellow
$ver = Invoke-RestMethod -Uri "$BaseUrl/api/v1/version" -TimeoutSec 20
if ($ver.github.enabled) { Write-Host "  PASS github API auth ($($ver.github.mode))" -ForegroundColor Green }
else { Write-Host "  FAIL github API auth" -ForegroundColor Red }
Write-Host "  -> .\scripts\test-phase9-staging-detect.ps1 -BaseUrl $BaseUrl" -ForegroundColor Cyan

Write-Host "`n[2/5] MCP staging" -ForegroundColor Yellow
Write-Host "  -> .\scripts\setup-mcp-staging.ps1" -ForegroundColor Cyan

Write-Host "`n[3/5] GitHub webhook / CI/CD monitor" -ForegroundColor Yellow
$deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments" -TimeoutSec 20
if ($deps.total -gt 0) { Write-Host "  PASS deployments: $($deps.total)" -ForegroundColor Green }
else {
  Write-Host "  PENDING deployments: 0" -ForegroundColor Yellow
  Write-Host "  Enable Workflow runs + Push + Pull requests on repo webhook" -ForegroundColor Cyan
  Write-Host "  Then re-run CI on main" -ForegroundColor Cyan
}

Write-Host "`n[4/5] Sentinel - after step 3 CI success" -ForegroundColor Yellow
Write-Host "  -> .\scripts\test-phase6-sentinel-staging.ps1 -GitHubSecret <secret>" -ForegroundColor Cyan
Write-Host "`n[5/5] Close loop - Start Fix -> MCP -> PR -> merge" -ForegroundColor Yellow
