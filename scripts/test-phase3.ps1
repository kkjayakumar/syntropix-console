param(
  [string]$BaseUrl = "http://localhost:5001"
)

Write-Host "=== Phase 3: embedded console ===" -ForegroundColor Cyan
$homePage = Invoke-WebRequest -Uri "$BaseUrl/" -UseBasicParsing
if ($homePage.StatusCode -ne 200) { throw "Console root failed" }
Write-Host "GET / -> $($homePage.StatusCode)" -ForegroundColor Green

$issuesPage = Invoke-WebRequest -Uri "$BaseUrl/issues" -UseBasicParsing
Write-Host "GET /issues -> $($issuesPage.StatusCode)" -ForegroundColor Green

Write-Host "=== Phase 3: issues API (prerequisite) ===" -ForegroundColor Cyan
$list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues"
Write-Host "Issues total: $($list.total)" -ForegroundColor Green

if ($list.total -gt 0) {
  $id = $list.issues[0].id
  Write-Host "=== Start Fix on issue #$id ===" -ForegroundColor Cyan
  $fix = Invoke-RestMethod -Uri "$BaseUrl/api/v1/issues/$id/start-fix" -Method POST
  Write-Host "Status: $($fix.issue.status) - $($fix.message)" -ForegroundColor Green
}

Write-Host "=== MCP binary ===" -ForegroundColor Cyan
$mcpPath = Join-Path $PSScriptRoot "..\bin\aegisops-mcp.exe"
if (Test-Path $mcpPath) {
  Write-Host "aegisops-mcp.exe built at $mcpPath" -ForegroundColor Green
  Write-Host "Configure Cursor per docs/MCP.md" -ForegroundColor Gray
} else {
  Write-Host "Run 'make build' to compile aegisops-mcp.exe" -ForegroundColor Yellow
}

Write-Host "Phase 3 test complete." -ForegroundColor Green
