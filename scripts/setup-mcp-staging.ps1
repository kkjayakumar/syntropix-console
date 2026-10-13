param([string]$DatabaseUrl = "")
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$example = Join-Path $root ".cursor\mcp.staging.json.example"
$out = Join-Path $root ".cursor\mcp.staging.json"
if (-not $DatabaseUrl) {
  $envFile = Join-Path $root "deploy\staging\.env"
  if (Test-Path $envFile) {
    $line = Get-Content $envFile | Where-Object { $_ -match "^DATABASE_URL=" } | Select-Object -First 1
    if ($line) { $DatabaseUrl = ($line -replace "^DATABASE_URL=", "").Trim() }
  }
}
if (-not $DatabaseUrl) { $DatabaseUrl = Read-Host "DATABASE_URL (Supabase)" }
$content = (Get-Content $example -Raw).Replace("postgres://USER:PASSWORD@HOST:5432/aegisops?sslmode=require", $DatabaseUrl)
Set-Content -Path $out -Value $content -NoNewline
Write-Host "Wrote $out - add to Cursor MCP settings and reload" -ForegroundColor Green
