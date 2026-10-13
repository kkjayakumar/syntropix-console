param(
  [string]$Tag = "latest",
  [switch]$Login
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$image = "ghcr.io/kkjayakumar/aegisops:$Tag"
$env:AEGISOPS_IMAGE_TAG = $Tag

Write-Host "=== Pull $image ===" -ForegroundColor Cyan
if ($Login) {
  if (-not $env:GITHUB_TOKEN) {
    throw "Set GITHUB_TOKEN (PAT with read:packages) or run: `$env:GITHUB_TOKEN = 'ghp_...'"
  }
  $env:GITHUB_TOKEN | docker login ghcr.io -u $env:GITHUB_USER --password-stdin 2>&1 | Out-Null
  if (-not $env:GITHUB_USER) {
    Write-Host "Tip: set GITHUB_USER to your GitHub username for docker login" -ForegroundColor Yellow
  }
}

docker compose -f docker-compose.yml -f docker-compose.ghcr.yml pull aegisops
docker compose -f docker-compose.yml -f docker-compose.ghcr.yml up -d --no-build aegisops

Write-Host "=== Health ===" -ForegroundColor Cyan
Start-Sleep -Seconds 3
Invoke-RestMethod -Uri http://localhost:5001/healthz | ConvertTo-Json
Write-Host "Running image: $image" -ForegroundColor Green
