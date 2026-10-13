# Syntropix Console — Community Edition

Self-hosted closed-loop DevOps with human approval: production signals → correlated context → approve → AI fix via MCP → pull request → deploy verify.

| | |
|--|--|
| **Marketing & product details** | [syntropix.io](https://syntropix.io) |
| **Managed cloud** | [console.syntropix.io](https://console.syntropix.io) |
| **This repo** | Self-host, single tenant, [AGPL-3.0](LICENSE) |

> Public name is **Syntropix Console**. Legacy code paths may still say `aegisops`; see [docs/NAMING.md](docs/NAMING.md).

## Why Syntropix

Monitoring tools detect. IDEs fix code. CI/CD deploys. **Syntropix Console connects the loop** with gateway approval before anything touches production.

## Quick start (5 steps)

1. **Clone and configure**

   ```bash
   git clone https://github.com/kkjayakumar/syntropix-console.git
   cd syntropix-console
   cp .env.example .env
   # AEGISOPS_API_TOKEN (16+ chars) and WORKSPACE_SECRETS_KEY: openssl rand -hex 32
   ```

2. **Start stack**

   ```bash
   docker compose -f docker-compose.oss.yml up -d --build
   ```

3. **Open console:** [http://localhost:5001](http://localhost:5001) → **Settings** → API token.

4. **Workspace:** enable connectors (Source, CI/CD, Telemetry, IDE/MCP). See [docs/QUICKSTART.md](docs/QUICKSTART.md).

5. **Webhooks:** GitHub + Sentry or DevOps Copilot → your console base URL (see [docs/INTEGRATION.md](docs/INTEGRATION.md)).

Docs: [QUICKSTART](docs/QUICKSTART.md) · [Roadmap](docs/OSS_ROADMAP.md) · [Naming](docs/NAMING.md)

## Architecture

Go API + embedded Next.js UI + PostgreSQL. IDE bridge: [MCP](docs/MCP.md).

```
Errors (Sentry / Copilot) ──┐
Spotting (optional) ────────┼──► Correlate → Gateway → MCP fix → GitHub PR → Sentinel
GitHub (CI/CD) ─────────────┘
```

## Development

```bash
docker compose up -d postgres
make build
./bin/aegisops
cd web && npm install && npm run dev
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

[docs/SECURITY.md](docs/SECURITY.md) — webhooks use HMAC; never commit `.env`.

## License

[GNU AGPL-3.0](LICENSE) — Copyright (c) Syntropix
