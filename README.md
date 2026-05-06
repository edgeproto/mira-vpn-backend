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

## Step 10 real WireGuard mode (remote POPs)

Step 10 now runs the backend stack only (`postgres` + `api`) and validates
reachability + provisioning against remote `mira-vpn-wgmgr` POPs listed in
`config/location-profiles.json`.

1) Create a real env file from template:

```bash
cp .env.real.example .env.real
```

2) Edit `.env.real` and set at least:
- `JWT_SECRET` (strong random value)
- `WGMGR_LOCATION_PROFILES_FILE` (default is `/etc/mira-config/location-profiles.json`)
- `WGMGR_ADMIN_TOKEN_DEFAULT` and/or `WGMGR_ADMIN_TOKEN_<LOCATION>`

3) Ensure each POP in `config/location-profiles.json` has:
- `endpoint` set to `<public-ip-or-hostname>:51820`
- `serverPublicKey` set to that POP's WireGuard server public key
- `wgmgrBaseUrl` set to `http://<pop-ip-or-hostname>:9090`

4) Run real stack + multi-location smoke validation:

```bash
./scripts/step10_real.sh
```

It will:
- check authenticated `GET /health` on every configured `wgmgrBaseUrl`
- start `postgres`, `migrations`, and `api` using:
  - `docker-compose.yml`
  - `docker-compose.real.yml`
- run auth + config provisioning smoke checks for every location

Stop and clean real stack:

```bash
docker compose -p mira_vpn_real -f docker-compose.yml -f docker-compose.real.yml --env-file .env.real down -v
```

## Phased rollout checklist

1) **Refactor + test locally**
- Run `go test ./...` in `mira-vpn-backend` and `mira-vpn-wgmgr`
- Run `./scripts/step9.sh` for mock end-to-end validation

2) **Backend redeploy (dedicated server)**
- Deploy backend stack without local `wgmgr`
- Mount `./config` and set `WGMGR_LOCATION_PROFILES_FILE`
- Set `WGMGR_ADMIN_TOKEN_DEFAULT` or per-location token env vars

3) **Mock POP smoke**
- Point one location at a mock `mira-vpn-wgmgr` instance
- Run `./scripts/step10_real.sh` and confirm all location checks pass

4) **First real VPS**
- Follow `mira-vpn-wgmgr/docs/vps-deploy.md` on the first POP
- Update `config/location-profiles.json` + matching token env var
- Re-run `./scripts/step10_real.sh`

5) **Remaining VPSes**
- Repeat VPS onboarding per location
- Verify each addition by running `./scripts/step10_real.sh`
