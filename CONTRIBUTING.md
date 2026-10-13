# Contributing to Syntropix Console (Community Edition)

Thank you for helping improve the open-source edition. Product site: [syntropix.io](https://syntropix.io).

## Setup

1. Fork and clone `https://github.com/kkjayakumar/syntropix-console`
2. Copy `.env.example` → `.env` (never commit `.env`)
3. Start Postgres: `docker compose up -d postgres`
4. Build: `make build` (requires Go 1.22+, Node 20+ for web)
5. Run API: `./bin/aegisops` (or `go run ./cmd/aegisops`)
6. Optional UI dev: `cd web && npm install && npm run dev`

## Tests

```bash
go test ./...
.\scripts\e2e-smoke.ps1
```

## Pull requests

- One logical change per PR
- `go test ./...` green
- No secrets, tokens, or `.env` in commits
- Update docs if you change env vars, webhooks, or connectors

## Roadmap alignment

See [docs/OSS_ROADMAP.md](docs/OSS_ROADMAP.md). Phase 1 focuses on single-tenant + Sentry/Copilot stability before new vendor adapters.

## Code of conduct

Be respectful in issues and discussions. Security issues: see [docs/SECURITY.md](docs/SECURITY.md) (private disclosure preferred).
