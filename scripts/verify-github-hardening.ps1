param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string]$Repo = "kkjayakumar/AegIsOps"
)

$ErrorActionPreference = "Continue"
$pass = 0; $warn = 0

function Pass([string]$m) { Write-Host "PASS $m" -ForegroundColor Green; $script:pass++ }
function Warn([string]$m) { Write-Host "WARN $m" -ForegroundColor Yellow; $script:warn++ }

Write-Host "=== GitHub hardening checks ===" -ForegroundColor Cyan

try {
  $v = Invoke-RestMethod -Uri "$BaseUrl/api/v1/version" -TimeoutSec 20
  if ($v.github.runtimeEnabled) { Pass "GitHub App runtime enabled on staging" }
  else { Warn "GitHub runtimeEnabled=false - PR automation broken" }
} catch { Warn ("version endpoint - " + $_.Exception.Message) }

try {
  $deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments" -TimeoutSec 20
  if ($deps.total -ge 3) { Pass "CI/CD monitor has $($deps.total) deployments" }
  else { Warn "Few deployments - enable workflow_run on repo webhook" }
} catch { Warn ("deployments API - " + $_.Exception.Message) }

if (Get-Command gh -ErrorAction SilentlyContinue) {
  try {
    $bp = gh api "repos/$Repo/branches/main/protection" 2>$null | ConvertFrom-Json
    if ($bp) { Pass "Branch protection enabled on main" } else { Warn "No branch protection on main" }
  } catch {
    Warn 'Branch protection not configured (or gh lacks admin:scope)'
    Write-Host "  Enable - Require PR, require status checks (CI), block force push" -ForegroundColor DarkGray
  }
  try {
    $hooks = gh api "repos/$Repo/hooks" 2>$null | ConvertFrom-Json
    $aegis = $hooks | Where-Object { $_.config.url -match "aegisops" }
    if ($aegis) {
      $events = ($aegis.events -join ", ")
      Pass ("Webhook -> $($aegis.config.url) events=" + ($aegis.events -join ", "))
      foreach ($need in @("workflow_run", "push", "pull_request")) {
        if ($events -notmatch $need) { Warn ("Webhook missing event - $need") }
      }
    } else {
      Warn "No webhook pointing at aegisops staging URL"
    }
  } catch { Warn ("Could not list webhooks - " + $_.Exception.Message) }
} else {
  Warn "gh CLI not installed - check branch protection and webhook manually on GitHub"
  Write-Host "  Webhook URL - $BaseUrl/api/v1/github/webhook" -ForegroundColor DarkGray
  Write-Host "  Events - workflow_run, push, pull_request" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "PASS=$pass WARN=$warn" -ForegroundColor Cyan
