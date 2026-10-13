# Create kkjayakumar/syntropix-console on GitHub and push main (override with GITHUB_OWNER).
# Requires: gh auth login   OR   $env:GH_TOKEN
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

$Owner = if ($env:GITHUB_OWNER) { $env:GITHUB_OWNER } else { "kkjayakumar" }

if (-not (gh auth status 2>$null)) {
  if (-not $env:GH_TOKEN) {
    Write-Host "Run: gh auth login" -ForegroundColor Yellow
    exit 1
  }
}

$Repo = "$Owner/syntropix-console"
$exists = gh repo view $Repo 2>$null
if (-not $exists) {
  gh repo create $Repo `
    --public `
    --description "Syntropix Console Community Edition (self-host closed-loop DevOps)" `
    --source . `
    --remote origin `
    --push
} else {
  git remote remove origin 2>$null
  git remote add origin "https://github.com/$Repo.git"
  git push -u origin main
}

Write-Host "Done: https://github.com/$Repo" -ForegroundColor Green
