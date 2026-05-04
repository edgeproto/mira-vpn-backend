# Multi-location WireGuard registry (optional)

**New VPS?** Install WireGuard on the host first: **[docs/vps-wireguard-setup.md](../docs/vps-wireguard-setup.md)**.

When `WGMGR_LOCATION_PROFILES_FILE` is set (see `.env.real.example`), the API and
`wgmgr` load location rows from a JSON **file** mounted at `/etc/mira-config/` in
Docker real mode.

1. Copy the example and edit endpoints / keys for your servers:

   ```bash
   cp config/location-profiles.example.json config/location-profiles.json
   ```

2. In `.env.real`, set:

   ```bash
   WGMGR_LOCATION_PROFILES_FILE=/etc/mira-config/location-profiles.json
   ```

3. Restart the real stack (`./scripts/step10_real.sh` or your compose command).

See **[docs/wireguard-locations.md](../docs/wireguard-locations.md)** for the full
field list and operational notes.

**Single WireGuard host, two logical regions:** you may point two entries at the
same `endpoint` and `serverPublicKey` (different `name`) so the app shows two
servers while one `wg0` accepts peers for both. When you add a **second
physical** host, put its public endpoint and server public key on the second
row and ensure that host runs WireGuard (and optionally its own `wgmgr` if you
split provisioning).
