param(
  [string]$BaseUrl = "https://aegisops.syntropix.io",
  [string]$EnvFile = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not $EnvFile) { $EnvFile = Join-Path $root "deploy\staging\.env" }

function Get-EnvValue([string]$Key) {
  if (-not (Test-Path $EnvFile)) { return "" }
  $line = Get-Content $EnvFile | Where-Object { $_ -match "^$Key=" } | Select-Object -First 1
  if (-not $line) { return "" }
  return ($line -replace "^$Key=", "").Trim()
}

$pass = 0; $warn = 0; $fail = 0
function Pass([string]$Msg) { Write-Host "PASS $Msg" -ForegroundColor Green; $script:pass++ }
function Warn([string]$Msg) { Write-Host "WARN $Msg" -ForegroundColor Yellow; $script:warn++ }
function Fail([string]$Msg) { Write-Host "FAIL $Msg" -ForegroundColor Red; $script:fail++ }

Write-Host "=== Phase 8: Production hardening (remote checks) ===" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"

Write-Host "`n--- Health & sentinel ---" -ForegroundColor Cyan
try {
  $h = Invoke-RestMethod -Uri "$BaseUrl/healthz" -TimeoutSec 20
  if ($h.status -eq "ok") { Pass "healthz ok" } else { Fail "healthz not ok" }
} catch { Fail "healthz unreachable: $($_.Exception.Message)" }

$verify = Get-EnvValue "DEPLOY_VERIFY_URL"
if ($verify -and $verify -notmatch "example\.com|your-app") {
  Pass "DEPLOY_VERIFY_URL configured locally: $verify"
  try {
    $vr = Invoke-WebRequest -Uri $verify -UseBasicParsing -TimeoutSec 20
    if ($vr.StatusCode -eq 200) { Pass "DEPLOY_VERIFY_URL returns 200" } else { Warn "DEPLOY_VERIFY_URL HTTP $($vr.StatusCode)" }
  } catch { Warn "DEPLOY_VERIFY_URL unreachable from this machine" }
} else {
  Warn "Set DEPLOY_VERIFY_URL in deploy/staging/.env on EC2"
}

Write-Host "`n--- Subdomains ---" -ForegroundColor Cyan
foreach ($url in @(
  "https://api.spotting.syntropix.io/healthz",
  "https://spotting.syntropix.io",
  "https://monitoring.syntropix.io"
)) {
  try {
    $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 20 -MaximumRedirection 0 -ErrorAction SilentlyContinue
    $code = [int]$r.StatusCode
  } catch {
    $code = [int]$_.Exception.Response.StatusCode.value__
    if (-not $code) { $code = 0 }
  }
  if ($code -in 200,301,302,307,308) { Pass "$url -> $code" } else { Warn "$url -> HTTP $code" }
}

Write-Host "`n--- Staging API ---" -ForegroundColor Cyan
try {
  $ver = Invoke-RestMethod -Uri "$BaseUrl/api/v1/version" -TimeoutSec 20
  if ($ver.github.runtimeEnabled) { Pass "GitHub App runtimeEnabled on staging" } else { Warn "github.runtimeEnabled=false" }
} catch { Warn "version: $($_.Exception.Message)" }
try {
  $deps = Invoke-RestMethod -Uri "$BaseUrl/api/v1/deployments" -TimeoutSec 20
  if ($deps.total -gt 0) { Pass "deployments total=$($deps.total)" } else { Warn "deployments empty" }
} catch { Warn "deployments: $($_.Exception.Message)" }

Write-Host "`n--- Local .env hints (optional) ---" -ForegroundColor Cyan
if (Test-Path $EnvFile) {
  Pass "deploy/staging/.env exists locally"
  $aegis = Get-EnvValue "AEGIS_WEBHOOK_SECRET"
  $gh = Get-EnvValue "GITHUB_WEBHOOK_SECRET"
  if ($aegis -and $aegis -ne "dev-webhook-secret") { Pass "AEGIS_WEBHOOK_SECRET non-default" } else { Warn "AEGIS_WEBHOOK_SECRET default/missing in local copy" }
  if ($gh) { Pass "GITHUB_WEBHOOK_SECRET set locally" } else { Warn "GITHUB_WEBHOOK_SECRET missing in local copy" }
  if ((Get-EnvValue "GITHUB_APP_ID") -and (Get-EnvValue "GITHUB_APP_INSTALLATION_ID")) {
    Pass "GitHub App configured in local .env"
  } else {
    Warn 'GitHub App not in local .env - still on PAT?'
  }
} else {
  Warn 'No local deploy/staging/.env - run full check on EC2 with ./scripts/setup-phase8-staging.sh'
}

Write-Host "`n--- Summary ---" -ForegroundColor Cyan
Write-Host "PASS=$pass WARN=$warn FAIL=$fail"
if ($fail -gt 0) {
  Write-Host "Fix FAIL items on EC2, then: ./scripts/setup-phase8-staging.sh $BaseUrl" -ForegroundColor Red
  exit 1
}
Write-Host "Run on EC2 for full Phase 8 audit: ./scripts/setup-phase8-staging.sh $BaseUrl" -ForegroundColor White
Write-Host "Then: GitHub App + branch protection + docs/RUNBOOK.md" -ForegroundColor White
