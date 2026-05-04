# Adding a second WireGuard POP (e.g. Colorado)

Your **Flutter app** and **Mira API** are fine when `location-profiles.json` points at a working server. If **`sudo wg show wg0` on Colorado shows no peers**, the phone can still get a valid-looking **client config**, but **no WireGuard peer exists on Colorado’s kernel** — so handshakes or traffic will fail.

Today **`cmd/api` uses one `WGMGR_BASE_URL`** (see `main.go`): every `CreatePeer` call goes to **one** `wgmgr` process. That process runs **`wg set`** only on **its host’s** `wg0`. It does **not** configure other VPSes.

Use one of the paths below.

---

## 1. Bring up WireGuard on Colorado (always required)

On the Colorado VPS, do the full server setup:

- Keys, **`/etc/wireguard/wg0.conf`**, **NAT**, **`ip_forward`**, **firewall**, **`wg-quick@wg0`**
- **`ListenPort`** and **`serverPublicKey`** must match what you put in `location-profiles.json` for USA-Colorado

Follow **[vps-wireguard-setup.md](./vps-wireguard-setup.md)** end-to-end, then confirm:

```bash
sudo wg show wg0
sudo ss -ulnp | grep wg0
```

---

## 2. Choose how Colorado gets **client peers**

### Option A — Same exit as Finland (no extra work)

Point USA-Colorado at **Finland’s** `endpoint` + `serverPublicKey` in JSON. One `wg0`, one `wgmgr`, two UI names. Good until you have a real second POP.

---

### Option B — Second `wgmgr` on Colorado (recommended when you want a real second POP)

Run the **same** `wgmgr` image/binary on Colorado (real mode, its own `WGMGR_REAL_*`, same `location-profiles.json` or a one-location file).

**Catch:** the **API** must call **that** `wgmgr` when `location` is USA-Colorado. Right now the API only knows **`WGMGR_BASE_URL`**. Implementing **per-location `wgmgr` base URL** (or a small routing table in the API) is the clean fix; until then, use **Option C** or keep **Option A**.

If you add per-location routing in code, each `wgmgr` should:

- Use its **own** `WGMGR_REAL_OUTPUT_DIR` (local disk)
- Share the **same** `10.200.0.0/24` plan is OK **per server** (each host tracks its own next free IP in its own output dir)

#### Option B — implementation spec (per-location `wgmgr` URL)

**Goal:** the API process still runs on one host (e.g. Finland with Postgres), but `POST /v1/peers` is sent to the `wgmgr` instance that owns the WireGuard interface for that **logical location**.

**Current behavior (to change):** `cmd/api/main.go` builds a single `wgmgrclient.Client` with `WGMGR_BASE_URL` and passes it into `handlers.NewWireGuardHandler`. The handler calls `CreatePeer` with the resolved canonical location name:

```131:134:internal/handlers/wireguard.go
	mgmtResp, err := h.provSvc.CreatePeer(r.Context(), wgmgrclient.CreatePeerRequest{
		UserID:   userID,
		Location: location,
	})
```

**1. API code shape**

- Keep `wgmgrclient.Client` as the low-level HTTP client (`POST …/v1/peers`).
- Add a small **router** type that implements the same interface the handler already uses (`CreatePeer(ctx, req) …`), for example:
  - **Default base URL:** `WGMGR_BASE_URL` (today’s behavior — e.g. `http://127.0.0.1:9090` on the Finland host with `network_mode: host`).
  - **Overrides:** a map **canonical location name → base URL**, loaded from env (e.g. `WGMGR_BASE_URL_MAP_JSON`) or from a file path (e.g. `WGMGR_BASE_URL_MAP_FILE`) so you do not stuff huge JSON in one line.
- **Lookup key** must match the registry’s **`name`** exactly (same string the handler already uses after `ProfileForLocation`, e.g. `USA-Colorado`). Case is whatever you stored in JSON; the code uses `profile.Name` from the registry.
- If a location has **no** override, use the default client (Finland’s local `wgmgr`).
- **Errors:** if a location is valid in `location-profiles.json` but missing from the override map and the default `wgmgr` is not the right host, you get the old bug (peer on wrong box). Optional startup check: for every loaded profile name, require an explicit URL when `WGMGR_STRICT_LOCATION_ROUTING=true`, or log a warning when multiple distinct `endpoint` values exist but only one `WGMGR_BASE_URL`.

**2. Networking (API → remote `wgmgr`)**

- `wgmgr` HTTP today has **no authentication** on `POST /v1/peers` / `DELETE /v1/peers/{peerID}` — treat it as an **internal control plane**. Do **not** expose `:9090` on the public internet without adding auth or IP allowlists.
- Typical patterns:
  - **Private link:** Tailscale / WireGuard site-to-site / cloud VPC peering so the Finland API can `http://100.x.y.z:9090` Colorado’s `wgmgr`.
  - **SSH tunnel** for bootstrap only (API connects to `127.0.0.1` forwarded to Colorado) — workable, brittle for production.
  - **TLS reverse proxy + mTLS or source IP allowlist** if you must cross the public internet.

**3. Colorado host: run `wgmgr` like Finland**

- Same binary/Docker image as `docker-compose.real.yml` **`wgmgr`** service: `WGMGR_MODE=real`, `network_mode: host`, `cap_add: NET_ADMIN`, mount **the same** `location-profiles.json` as Finland (`WGMGR_LOCATION_PROFILES_FILE=/etc/mira-config/location-profiles.json`).
- Set **`WGMGR_REAL_INTERFACE`**, **`WGMGR_REAL_OUTPUT_DIR`** (local volume on Colorado), and **`WGMGR_REAL_*`** defaults for DNS/AllowedIPs as on Finland. Client configs use **per-row** `endpoint` / `serverPublicKey` from the JSON profile when present; host defaults are only fallbacks (see `internal/wgmgr/mock.go` `BuildClientConfig` / `firstNonEmpty(profile.Endpoint, m.endpoint)`).
- Complete **kernel WireGuard + NAT** on Colorado per [vps-wireguard-setup.md](./vps-wireguard-setup.md).

**4. IP allocation (`10.200.0.0/24`)**

- Each `wgmgr` scans **only its own** `WGMGR_REAL_OUTPUT_DIR` for `*.json` to pick the next `10.200.0.x/32`. Finland and Colorado therefore maintain **independent** pools on two servers — no coordination file is required. The same numeric address on two different `wg0` interfaces is fine.

**5. Postgres and peer rows**

- The API still **upserts** into Postgres after a successful `CreatePeer` (`internal/handlers/wireguard.go`). No change required for multi-POP as long as **`(userId, location)`** uniqueness matches how each `wgmgr` treats “existing peer” (each host’s `findExisting` only sees **its** disk). If a user switches location, they get a second row / second tunnel — align with your product rules.

**6. Deletes and future endpoints**

- `wgmgr` exposes `DELETE /v1/peers/{peerID}`; the Go API **does not** call it yet. If you add account deletion or “revoke config”, the router must send `DELETE` to the **same** base URL that created that peer (store `peerId` + `location` or resolve location from DB).

**7. Compose / env summary**

| Process | Host | Role |
|--------|------|------|
| `api` | Finland (example) | DB + JWT; `WGMGR_BASE_URL` → local Finland `wgmgr`; map entry `USA-Colorado` → reachable Colorado `wgmgr` URL |
| `wgmgr` | Finland | `wg set` on Finland `wg0`; output dir on Finland |
| `wgmgr` | Colorado | `wg set` on Colorado `wg0`; output dir on Colorado |
| `postgres` | Usually Finland | Single source of truth |

---

### Option C — **No API code change**: sync `wg set` from Finland to Colorado

Finland’s `wgmgr` writes **`{peerId}.json`** next to each client config under **`WGMGR_HOST_OUTPUT_DIR`** (host path mapped into Docker). Each file includes:

- `publicKey` — **client** WireGuard public key  
- `address` — e.g. `10.200.0.7/32`  
- `location` — e.g. `USA-Colorado`

After a new file appears for **`USA-Colorado`**, Colorado must run the same effect as `wgmgr` would locally:

```bash
sudo wg set wg0 peer <CLIENT_PUBLIC_KEY> allowed-ips <ADDRESS_CIDR>
```

**Sketch:**

1. **SSH** from Finland → Colorado (key-based, no password).
2. On Finland, a **small script** (cron, `inotifywait`, or a hook after deploy) watches the peer output directory for new/changed `*.json`, filters `location == "USA-Colorado"`, and runs **remote** `wg set` over SSH.
3. On **peer delete**, Finland’s `wgmgr` removes the peer locally; your script must also run **`wg set wg0 peer <CLIENT_PUBLIC_KEY> remove`** on Colorado (parse deletes or watch file removal).

**Important:** Finland’s `wgmgr` **still** runs `wg set` on **Finland’s** `wg0` for Colorado locations — those peers are **unused** for traffic (clients talk to Colorado) but harmless if you ignore them, or you could later add a “remote-only” mode in code to skip local `applyPeer` for certain locations.

---

## 3. Verify from Colorado

While the Android app is connected to **USA-Colorado**:

```bash
sudo wg show wg0
```

You want a **peer**, **latest handshake**, and **transfer** increasing. Then test browsing. If peer exists but no web, fix **NAT / FORWARD** (same as Finland).

---

## 4. Summary

| Goal | Action |
|------|--------|
| UI only, one server | Option **A** — same endpoint/key as Finland in JSON. |
| Real Colorado, no code yet | Option **C** — WireGuard on Colorado + **SSH sync** of `wg set` from Finland peer JSON. |
| Real Colorado, maintainable | Option **B** — second **`wgmgr` on Colorado** + **API routes `CreatePeer` by location** (code change). |

---

## Related docs

- [vps-wireguard-setup.md](./vps-wireguard-setup.md) — install WireGuard + NAT on a VPS  
- [wireguard-locations.md](./wireguard-locations.md) — JSON registry and “one `wgmgr` vs many servers”
