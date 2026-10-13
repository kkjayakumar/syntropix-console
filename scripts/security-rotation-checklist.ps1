# Security rotation checklist — secrets exposed in chat or dev defaults.
# Does NOT rotate automatically; prints ordered steps for AWS Secrets Manager + Supabase.

$ErrorActionPreference = "Continue"

Write-Host "=== AegisOps security rotation checklist ===" -ForegroundColor Cyan
Write-Host @"

1. Supabase — rotate database password
   - Dashboard -> Project Settings -> Database -> Reset password
   - Update dev/syntropix in AWS Secrets Manager: DATABASE_URL

2. GitHub webhooks
   - Regenerate GITHUB_WEBHOOK_SECRET on repo webhook + Secrets Manager
   - Regenerate AEGIS_WEBHOOK_SECRET; update Copilot + Spotting .env on EC2

3. GitHub App — only if private key was exposed (usually not)

4. Spotting — rotate NEXT_PUBLIC_SPOTTING_PUBLIC_KEY if exposed

5. Teams / Power Automate — regenerate webhook URLs if pasted in chat (optional)

6. Redeploy staging after Secrets Manager update:
   bash /opt/aegisops/deploy/ec2/deploy.sh latest

7. Verify:
   curl -s https://aegisops.syntropix.io/api/v1/version | jq .github
   .\scripts\verify-github-webhook.ps1

EC2 inbox SQL fallback (before close API is deployed):
  source /opt/aegisops/deploy/staging/.env
  docker run --rm postgres:16-alpine psql "`$DATABASE_URL" -c "
    UPDATE aegis_issues SET status='closed', updated_at=now()
    WHERE status IN ('in_progress','correlated') AND issue_type='fix';"

"@ -ForegroundColor White

Write-Host "Mark complete in your runbook when done." -ForegroundColor Yellow
