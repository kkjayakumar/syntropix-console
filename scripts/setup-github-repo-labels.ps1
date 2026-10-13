param([string]$Repo = "kkjayakumar/AegIsOps")
$labels = @("aegisops-detected", "aegisops-selftest", "bug", "ci")
$gh = @("$env:ProgramFiles\GitHub CLI\gh.exe", "$env:LOCALAPPDATA\Programs\GitHub CLI\gh.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $gh) { Write-Host "Create labels manually in GitHub repo Settings -> Labels: $($labels -join ', ')"; exit 0 }
foreach ($name in $labels) { & $gh label create $name --repo $Repo --force 2>$null; Write-Host "label $name" }
