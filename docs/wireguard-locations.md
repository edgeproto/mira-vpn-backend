# VPN location servers (multi-region registry)

The **API** process (`cmd/api`) loads a **location registry**: one entry per logical region (e.g. Finland, Germany). Each entry describes how clients should reach **that region’s WireGuard server** (endpoint, server public key, DNS, routing). The mobile app discovers regions via `GET /wireguard/locations` and sends the chosen **canonical `name`** when requesting a profile (`POST /wireguard/config` or `/wireguard/config/guest`).

The **WireGuard manager** (`cmd/wgmgr`) receives `location` on peer creation and uses the **same** registry (loaded in the API; keep API and wgmgr configs aligned in production) to fill `Endpoint`, `PublicKey`, `AllowedIPs`, etc. in generated client configs.

This document is the reference for **what to configure** and **how it behaves**. It does not replace host-level WireGuard setup (keys, `wg0`, NAT, firewall).

---

## Where configuration lives

| Variable | Purpose |
|----------|---------|
| `WGMGR_LOCATION_PROFILES_FILE` | Path to a JSON **file** (array of location objects). **Takes precedence** over the inline variable below when set. |
| `WGMGR_LOCATION_PROFILES_JSON` | Inline JSON **string** (same array shape). Used when `WGMGR_LOCATION_PROFILES_FILE` is unset or empty. |
| *(neither set)* | API falls back to a **single built-in** profile named `Finland` with empty endpoint/key in the registry; local **mock/real wgmgr** merges those from `WGMGR_MOCK_*` / `WGMGR_REAL_*` instead. |

Set **either** a file **or** inline JSON in production whenever you have **more than one region** or want explicit per-region endpoints and keys in the API process.

Implementation: `internal/wgmgr/template.go` (`LoadLocationProfilesFromEnv`, `ParseLocationProfilesJSON`).

### Docker real stack (`docker-compose.real.yml`)

Compose bind-mounts **`./config` → `/etc/mira-config`** (read-only) into **api** and **wgmgr**. Set in `.env.real`:

`WGMGR_LOCATION_PROFILES_FILE=/etc/mira-config/location-profiles.json`

and keep your registry in **`config/location-profiles.json`** on the host (see `config/README.md` and `config/location-profiles.example.json`). Leave the variable unset to use the built-in single-region default (no file required for the mount; the directory should still exist so the bind mount succeeds — keep `config/README.md` or any file under `config/`).

---

## JSON shape (one object per location)

Array of objects. **Required** for every object loaded from JSON or file:

| Field | Type | Meaning |
|-------|------|---------|
| `name` | string | Canonical region id (e.g. `Finland`). Case-insensitive lookup; stored with the casing you provide. Must be **unique** across the array. |
| `endpoint` | string | Client `[Peer] Endpoint`: `host:port` (UDP). Must be reachable from the public internet (or your target users). |
| `serverPublicKey` | string | WireGuard **server** public key (base64) for `[Peer] PublicKey`. |

**Optional** (WireGuard / routing):

| Field | Type | Default / behavior |
|-------|------|---------------------|
| `dns` | string | `[Interface] DNS` when set. |
| `allowedIPs` | string | Client `AllowedIPs`; default `0.0.0.0/0` if omitted. |
| `keepalive` | number | `PersistentKeepalive` seconds; default `25` if omitted or ≤ 0. |

**Optional** (client/UI only — safe to expose in `GET /wireguard/locations`; **no secrets**):

| Field | Type | Meaning |
|-------|------|---------|
| `displayName` | string | Shown in the app; defaults to `name` if empty. |
| `country` | string | Short hint (e.g. ISO country). |
| `latencyHint` | string | Free-form hint for UI. |
| `flagCode` | string | Optional flag / region code for UI. |

**Never** put private keys or internal-only hostnames in fields returned to the app if you later extend the API — today only the fields above are exposed.

---

## Minimal example (two regions)

```json
[
  {
    "name": "Finland",
    "endpoint": "fi.vpn.example.com:443",
    "serverPublicKey": "BASE64_SERVER_PUBLIC_KEY_FI",
    "dns": "1.1.1.1",
    "displayName": "Helsinki",
    "country": "FI"
  },
  {
    "name": "Germany",
    "endpoint": "de.vpn.example.com:443",
    "serverPublicKey": "BASE64_SERVER_PUBLIC_KEY_DE",
    "dns": "1.1.1.1",
    "displayName": "Frankfurt",
    "country": "DE"
  }
]
```

---

## API behavior (for operators and client authors)

- **`GET /wireguard/locations`** — Returns `{ "locations": [ ... ] }` with **client-safe** fields only (`name`, `displayName`, optional `country`, `latencyHint`, `flagCode`). Sorted by `name`.
- **`POST /wireguard/config`** — Body `{ "location": "<canonical name>" }`. Empty `location` uses the **first** name in the sorted registry (`DefaultLocationName()`).
- **`POST /wireguard/config/guest`** — Same `location` semantics; also requires `deviceId`.

Unknown `location` → **400** `unsupported location`.

---

## Per-region WireGuard host (operational checklist)

For **each** JSON entry you need a real server (or VM) that:

1. Runs WireGuard (`wg0` or similar), UDP port open to the internet (same port as in `endpoint`).
2. Has a **stable** public hostname or IP in `endpoint`.
3. Publishes the **server public key** you put in `serverPublicKey` (must match the interface that peers connect to).
4. Has **forwarding + NAT** so tunnel traffic can exit to the internet; firewall allows forwarded traffic and UDP input on the listen port.
5. Uses DNS / `allowedIPs` / MTU consistent with your policy (IPv6 only if you actually NAT or route IPv6).

The backend does **not** health-check those hosts; drift between JSON and live `wg` config is an operational risk.

---

## One `wgmgr` process vs several physical servers (important)

In the **Docker real** layout, **`wgmgr` runs on a single host** (host network mode) and runs:

```text
wg set wg0 peer <client_public_key> allowed-ips <client_tunnel_ip>/32
```

only on **that** machine’s **`wg0`**.

- For a location whose **`endpoint`** is that same host (e.g. Finland → `95.217.206.233:443`), the peer is created where the tunnel actually terminates, and **NAT / forwarding** you configured on that host apply. **Internet works** if that host’s `wg0.conf` `PostUp` rules are correct.

- For a location whose **`endpoint`** is **another** IP (e.g. USA-Colorado → `5.180.24.40:443`), the **client profile** tells the phone to talk to **that** server. The peer created by Mira’s `wgmgr` is still only on the **Finland** box unless you **also** provision that peer on the USA box (second `wgmgr`, automation, or manual `wg set`). If the USA server **does** accept the handshake but has **no MASQUERADE / `ip_forward` / FORWARD** rules for the client subnet, you get the classic symptom: **VPN “connected” but no internet**.

**Practical options:**

1. **Separate VPS per region** — On **each** VPS: full WireGuard + NAT per [vps-wireguard-setup.md](./vps-wireguard-setup.md), and a **provisioning path** that adds each new peer on **that** host’s `wg` (today that means **one `wgmgr` (or equivalent) per server**, or your own sync from Finland).
2. **Single VPS, multiple UI locations** — Point every row at the **same** `endpoint` and `serverPublicKey` with different `name` / `displayName` until you have a second host fully wired.
3. **Hybrid** — Keep Finland on this stack; for USA, either replicate `wgmgr`+API there or temporarily set USA’s `endpoint` back to Finland’s until USA is ready.

---

## Troubleshooting: one location works, another has “no internet”

| Situation | What to check |
|-----------|----------------|
| Second location uses a **different `endpoint` IP** | On **that** server: `sudo wg show` — is the client **peer** present after you connect from the app? If not, traffic never lands correctly; fix provisioning on that host. |
| Peer **is** on the second server | Same checklist as Finland: **`net.ipv4.ip_forward=1`**, **`POSTROUTING` MASQUERADE** for your tunnel subnet (e.g. `10.200.0.0/24`) out the **WAN** interface, **FORWARD** allow `wg0`, **UDP** port open in cloud + OS firewall. See [vps-wireguard-setup.md](./vps-wireguard-setup.md) §3–§5. |
| Finland OK, USA different IP | Almost always **NAT/routing on the USA VPS** or **peer not applied on USA** — not an Android app bug. |

---

## Alignment with `WGMGR_REAL_*` (single-host dev / legacy)

When the location registry is **not** set, the built-in Finland profile has no endpoint/key in the registry; **wgmgr** fills them from `WGMGR_REAL_ENDPOINT`, `WGMGR_REAL_SERVER_PUBLIC_KEY`, etc. See `.env.real.example` and `internal/wgmgr/config.go`.

When you use **multi-location JSON**, each row must carry its own `endpoint` and `serverPublicKey`; that is independent of a single global `WGMGR_REAL_*` pair (you may still use one wgmgr instance per host or one process managing multiple interfaces — your deployment choice).

---

## App (Flutter)

The app calls `GET /wireguard/locations`, lets the user pick a server, persists the choice, and sends that **`name`** as `location` when creating a config. If the list request fails, it falls back to a minimal local list so the UI still works offline.

---

## Related files

- **Second POP (e.g. Colorado):** [second-wireguard-pop.md](./second-wireguard-pop.md) — WireGuard on the new VPS, and how peers get there (same endpoint vs second `wgmgr` vs SSH sync).
- **Fresh VPS WireGuard install (keys, wg0, NAT, firewall):** [vps-wireguard-setup.md](./vps-wireguard-setup.md)
- Registry load / validation: `internal/wgmgr/template.go`
- HTTP handlers: `internal/handlers/wireguard.go`
- API startup: `cmd/api/main.go` (`LoadLocationProfilesFromEnv`)
- Example env comments: `.env.real.example`
