# WireGuard on a Linux VPS (operator guide)

Step-by-step setup for a **single** WireGuard server (one `wg0`) on a typical Ubuntu/Debian VPS. This matches what **Mira VPN’s `wgmgr` real mode** expects: interface name **`wg0`** by default (`WGMGR_REAL_INTERFACE`), UDP endpoint `PUBLIC_IP:PORT` or `hostname:PORT`, and the server’s **public key** in `.env.real` / `location-profiles.json`.

Pick a **UDP listen port** clients will use (common choices: `51820`, `443`). Your **Mira** env must use the **same** host:port and public key.

## Table of contents

1. [Before you start](#0-before-you-start)
2. [Install packages](#1-install-packages)
3. [Generate server keys](#2-generate-server-keys)
4. [Server `wg0` config (`wg-quick`)](#3-server-wg0-config-wg-quick)
5. [IP forwarding](#4-ip-forwarding-required-for-nat)
6. [Firewall](#5-firewall-allow-wireguard-udp)
7. [Bring the tunnel up](#6-bring-the-tunnel-up)
8. [What to copy into Mira](#7-what-to-copy-into-mira)
9. [Sanity checks](#8-sanity-checks-from-your-laptop)
10. [Common problems](#9-common-problems)
11. [Optional hardening](#10-optional-hardening)
12. [Related Mira docs](#related-mira-docs)

---

## 0. Before you start

- **VPS** with a public IPv4 (and optionally IPv6).
- **Root or sudo** on the server.
- Decide: **`LISTEN_PORT`** (e.g. `443`), **`WG_INTERFACE`** (usually `wg0`), **`CLIENT_SUBNET`** for tunnel IPs (e.g. `10.200.0.0/24` — must match what your backend assigns; Mira mock/real uses a range like `10.200.x.x` for peers).
- Open that UDP port in the **cloud firewall** (AWS security group, Hetzner firewall, etc.) **and** on the VPS (`ufw` / `nftables`).

---

## 1. Install packages

Ubuntu/Debian:

```bash
sudo apt update
sudo apt install -y wireguard wireguard-tools
```

Verify the module loads:

```bash
sudo modprobe wireguard
```

---

## 2. Generate server keys

WireGuard uses a **private** key on the server and a **public** key you give to clients (and to Mira’s `WGMGR_REAL_SERVER_PUBLIC_KEY` / `serverPublicKey` in JSON).

```bash
sudo umask 077
sudo wg genkey | sudo tee /etc/wireguard/server_private.key >/dev/null
sudo wg pubkey < /etc/wireguard/server_private.key | sudo tee /etc/wireguard/server_public.key
```

**Save the public key line** — base64, one line. That is `WGMGR_REAL_SERVER_PUBLIC_KEY` and what clients put in `[Peer] PublicKey`.

Never leak `server_private.key`.

---

## 3. Server `wg0` config (`wg-quick`)

`wg-quick(8)` reads **`/etc/wireguard/wg0.conf`**, creates interface **`wg0`**, applies keys/addresses, runs **`PostUp`** shell commands after the interface is up, and **`PostDown`** when you stop the interface. You maintain this file; WireGuard does not auto-generate it for you.

Create the file (root-owned, mode **600** recommended):

```bash
sudo install -m 600 /dev/null /etc/wireguard/wg0.conf
sudo nano /etc/wireguard/wg0.conf
```

Below is a **full example** for IPv4 + NAT on a typical VPS, then a **line-by-line** explanation, then how to pick the correct **outbound NIC** instead of `eth0`.

### 3.1 Example `wg0.conf` (IPv4 + NAT)

```ini
[Interface]
Address = 10.200.0.1/24
ListenPort = 443
PrivateKey = PASTE_SERVER_PRIVATE_KEY_HERE_ONE_LINE_BASE64
SaveConfig = false

# WAN_IF must be your real default route interface (see §3.4). Example uses eth0.
PostUp = iptables -t nat -A POSTROUTING -s 10.200.0.0/24 -o eth0 -j MASQUERADE; iptables -A FORWARD -i wg0 -j ACCEPT; iptables -A FORWARD -o wg0 -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -s 10.200.0.0/24 -o eth0 -j MASQUERADE; iptables -D FORWARD -i wg0 -j ACCEPT; iptables -D FORWARD -o wg0 -j ACCEPT
```

Replace **`eth0`** everywhere in `PostUp`/`PostDown` with your **`WAN_IF`** from §3.4. Replace **`443`** if you chose another UDP port. **`PrivateKey`** must be the **raw base64 private key** (one line, no quotes) — see §3.3.

### 3.2 `[Interface]` — what each field does

| Directive | Example | Purpose |
|-----------|---------|---------|
| **`Address`** | `10.200.0.1/24` | The VPN **gateway** address **on the server** inside the tunnel. Mira’s provisioner assigns clients **`10.200.0.2/32` … `10.200.0.254/32`** inside the same `10.200.0.0/24` range, so **`.1` is reserved for the server** and must not be given to a client. Use **`/24`** on the server so Linux knows the whole LAN is reachable via `wg0`. |
| **`ListenPort`** | `443` | UDP port WireGuard listens on. Must match the **port** in Mira’s `endpoint` (e.g. `203.0.113.10:443`). Open this port in **cloud + VPS** firewalls. |
| **`PrivateKey`** | *(44-char base64)* | **Server** private key; must be the pair of the **public** key you put in Mira (`WGMGR_REAL_SERVER_PUBLIC_KEY` / `serverPublicKey`). **Never** commit this file to git or share it. `wg-quick` expects the key **inline** in this file (not a path to another file on stock installs). |
| **`SaveConfig`** | `false` | If `true`, `wg-quick` can rewrite the file when peers change. Mira **real** mode adds peers with **`wg set`** at runtime; keeping **`false`** avoids surprises unless you intentionally persist dynamic peers into this file. |

### 3.3 Putting the private key into the file

**Option A — paste in an editor**

```bash
sudo cat /etc/wireguard/server_private.key   # copy the single line
sudo nano /etc/wireguard/wg0.conf             # set PrivateKey = that line
sudo chmod 600 /etc/wireguard/wg0.conf
```

**Option B — one-shot from the key file** (still ends up inline in `wg0.conf`; this only helps you avoid manual paste errors)

```bash
PRIV=$(sudo tr -d '\n' </etc/wireguard/server_private.key)
sudo sed -i "s|^PrivateKey = .*|PrivateKey = ${PRIV}|" /etc/wireguard/wg0.conf
```

Verify the line has **no spaces**, **no quotes**, and **one** key.

### 3.4 Finding the correct outbound interface (`WAN_IF`)

`PostUp` uses **`-o WAN_IF`**: traffic **from VPN clients** (`10.200.0.0/24`) is **NAT’d** out that interface so replies come back to the VPS public IP.

**Reliable check** (Linux):

```bash
ip route get 1.1.1.1
```

Example output:

```text
1.1.1.1 via 203.0.113.1 dev ens3 src 203.0.113.10 uid 0 ...
```

Here **`ens3`** is **`WAN_IF`** (the `dev …` after `via`). Use that name in **`PostUp`/`PostDown`** instead of `eth0`.

Other useful commands:

```bash
ip -br route show default
ip -br link
```

Typical cloud names: **`ens3`**, **`enp0s3`**, **`eth0`**. **Docker bridges** (`docker0`) are **not** WAN — do not use them for `-o`.

### 3.5 `PostUp` / `PostDown` — what the `iptables` lines do

`PostUp` runs **once** when `wg0` goes up; `PostDown` must **exactly reverse** what `PostUp` added (same table, chain, match, jump).

**1) NAT (MASQUERADE)**

```text
iptables -t nat -A POSTROUTING -s 10.200.0.0/24 -o eth0 -j MASQUERADE
```

- **`-t nat`** — the NAT table.
- **`POSTROUTING`** — after routing: packets **leaving** the server to the internet.
- **`-s 10.200.0.0/24`** — only traffic **sourced** from your VPN client subnet.
- **`-o eth0`** — only when the packet goes out the **WAN** interface (replace with **`WAN_IF`**).
- **`-j MASQUERADE`** — replace the client’s private `10.200.x.x` source IP with the VPS **public** IP so return traffic works. Required on most VPSes where you don’t have a routed public subnet for each client.

**2) FORWARD accept**

```text
iptables -A FORWARD -i wg0 -j ACCEPT
iptables -A FORWARD -o wg0 -j ACCEPT
```

- Allows **forwarded** flows: **into** `wg0` from clients and **back** out to clients. Together with **`net.ipv4.ip_forward=1`** (§4), this lets client traffic **transit** the VPS.

**Semicolons** (`;`) chain multiple shell commands on one line; `wg-quick` runs the whole string as **`sh -c '…'`**.

If **`PostDown`** does not match **`PostUp`** (wrong interface name or typo), stopping `wg-quick` can leave orphan rules or fail to remove them — always edit both lines in sync.

### 3.6 After editing

```bash
sudo systemctl restart wg-quick@wg0
sudo wg show wg0
```

If the service fails, read **`journalctl -u wg-quick@wg0 -e`** and fix `Address`/`PrivateKey`/typos in `PostUp`.

### 3.7 `nftables` instead of `iptables`

If your VPS uses **nftables** only (no `iptables` legacy), the same logic applies: **NAT** for `10.200.0.0/24` → WAN, and **forward** filter rules for `wg0`. Exact commands depend on your table/chain layout; many operators still install **`iptables-nft`** compatibility so the example `PostUp` works unchanged — verify with `iptables -V` if unsure.

---

## 4. IP forwarding (required for NAT)

```bash
echo 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-wireguard-forward.conf
sudo sysctl --system
```

---

## 5. Firewall: allow WireGuard UDP

**UFW** example:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 443/udp
sudo ufw enable
sudo ufw status
```

Use your real **`LISTEN_PORT`**. Ensure the **cloud** panel also allows the same UDP port.

---

## 6. Bring the tunnel up

```bash
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0
sudo systemctl status wg-quick@wg0
```

Check interface and keys:

```bash
sudo wg show wg0
```

Confirm **listening**:

```bash
sudo ss -ulnp | grep wg0
```

---

## 7. What to copy into Mira

From the VPS:

| Value | Where it goes in Mira |
|--------|------------------------|
| `PUBLIC_IP:LISTEN_PORT` or DNS name | `WGMGR_REAL_ENDPOINT`, and each location’s `endpoint` in `config/location-profiles.json` |
| Server public key (`wg show wg0` or `server_public.key`) | `WGMGR_REAL_SERVER_PUBLIC_KEY` and `serverPublicKey` |
| Interface name | `WGMGR_REAL_INTERFACE=wg0` (default) |

**`step10_real.sh`** compares `WGMGR_REAL_SERVER_PUBLIC_KEY` to `wg show wg0 public-key` on the host — they must match exactly.

---

## 8. Sanity checks from your laptop

Replace `YOUR_VPS_IP` and port:

```bash
nc -vzu YOUR_VPS_IP 443
```

(UDP “success” is ambiguous; better: generate a **temporary client** config with `wg` and try `wg-quick up`, or use the Mira app once the API returns a config.)

On the **server**, after a client connects:

```bash
sudo wg show wg0
```

You should see **latest handshake** and **transfer** counters for each peer.

---

## 9. Common problems

| Symptom | Likely cause |
|--------|----------------|
| Client never handshakes | UDP blocked (VPS or cloud firewall), wrong `Endpoint` port, NAT hairpin if testing from same LAN |
| Handshake but no web | Missing `ip_forward` or NAT `PostUp` rules, wrong `-o eth0` interface |
| Wrong interface name | Run `ip -br a` and `ip route get 1.1.1.1` to find outbound iface (`ens3`, `enp0s3`, etc.) |
| Key mismatch | Server `PrivateKey` in `wg0.conf` must match the pair whose **public** key is in Mira env |
| `wg-quick` fails | Syntax error in `.conf`, or `Address` overlaps with existing routes |

---

## 10. Optional hardening

- Restrict `ListenPort` to known client IPs only if you have fixed IPs (usually impractical for a VPN).
- Keep **`SaveConfig = false`** if you manage peers with **`wg set`** / automation (Mira `wgmgr` real mode applies peers dynamically); avoid `wg-quick` overwriting runtime peers on restart unless you persist them yourself.
- **DNS**: pushing `DNS = 1.1.1.1` in **client** configs (Mira does via API profile) is separate from installing `unbound`/`dnsmasq` on the VPS — only needed if you want the **server** to resolve for clients.

---

## Related Mira docs

- [wireguard-locations.md](./wireguard-locations.md) — multi-location JSON and API fields.
