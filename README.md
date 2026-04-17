# mira-vpn-backend

## Step 9 local stack + smoke test

Run this single command from `mira-vpn-backend`:

```bash
./scripts/step9.sh
```

It will:
- start `postgres`, `migrations`, `wgmgr` (mock mode), and `api` via Docker Compose
- wait for API health
- run a smoke flow:
  - register
  - login
  - request WireGuard config

Stop and clean the step9 stack:

```bash
docker compose -p mira_vpn_step9 down -v
```

### Optional overrides

- `COMPOSE_PROJECT_NAME` (default: `mira_vpn_step9`)
- `API_HOST_PORT` (default: `18080`)
- `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB`
