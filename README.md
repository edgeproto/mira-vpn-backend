# mira-vpn-backend

## VPN location servers (multi-region)

Operators and developers: see **[docs/wireguard-locations.md](docs/wireguard-locations.md)** for how the location registry works (`WGMGR_LOCATION_PROFILES_FILE` / `WGMGR_LOCATION_PROFILES_JSON`), the JSON schema, API endpoints, and what each WireGuard host must provide.

**Install/deploy a VPN POP (VPS):** `mira-vpn-wgmgr/docs/vps-deploy.md` (canonical runbook).
Compatibility pointer in this repo: [docs/vps-wireguard-setup.md](docs/vps-wireguard-setup.md).

**Second region / own VPS (e.g. Colorado):**
- Backend location/profile behavior: [docs/wireguard-locations.md](docs/wireguard-locations.md)
- Per-VPS wgmgr deployment runbook: `mira-vpn-wgmgr/docs/vps-deploy.md`

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

## Step 10 real WireGuard mode (dedicated server)

This keeps Step 9 mock mode unchanged and adds an override for real mode.

1) Create a real env file from template:

```bash
cp .env.real.example .env.real
```

2) Edit `.env.real` and set at least:
- `WGMGR_REAL_ENDPOINT` (example: `95.217.206.233:51820`)
- `WGMGR_REAL_SERVER_PUBLIC_KEY` (from `/etc/wireguard/server_public.key` on your VPN host)
- `JWT_SECRET` (strong random value)

3) Run the real stack + smoke validation:

```bash
./scripts/step10_real.sh
```

It will:
- start `postgres`, `migrations`, `wgmgr` (real mode), and `api` using:
  - `docker-compose.yml`
  - `docker-compose.real.yml`
- run auth + guest provisioning smoke checks
- assert returned WireGuard configs contain `Endpoint = $WGMGR_REAL_ENDPOINT`

Stop and clean real stack:

```bash
docker compose -p mira_vpn_real -f docker-compose.yml -f docker-compose.real.yml --env-file .env.real down -v
```
