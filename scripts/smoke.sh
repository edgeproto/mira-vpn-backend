#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"
EMAIL="${SMOKE_EMAIL:-smoke-$(date +%s)@example.com}"
PASSWORD="${SMOKE_PASSWORD:-changeme123}"

echo "==> register user: ${EMAIL}"
register_payload=$(cat <<EOF
{"email":"${EMAIL}","password":"${PASSWORD}"}
EOF
)
register_resp="$(curl -sS -X POST "${API_BASE_URL}/auth/register" -H "Content-Type: application/json" -d "${register_payload}")"
echo "${register_resp}"

echo "==> login user"
login_payload=$(cat <<EOF
{"email":"${EMAIL}","password":"${PASSWORD}"}
EOF
)
login_resp="$(curl -sS -X POST "${API_BASE_URL}/auth/login" -H "Content-Type: application/json" -d "${login_payload}")"
token="$(printf '%s' "${login_resp}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("accessToken","") or d.get("token",""))')"
if [[ -z "${token}" ]]; then
  echo "failed to read accessToken from login response: ${login_resp}" >&2
  exit 1
fi
echo "token acquired"

echo "==> fetch available locations"
locations_resp="$(curl -sS "${API_BASE_URL}/wireguard/locations" -H "Authorization: Bearer ${token}")"
locations="$(
  printf '%s' "${locations_resp}" | python3 -c '
import json
import sys
payload = json.load(sys.stdin)
if isinstance(payload, dict):
    payload = payload.get("locations", [])
if not isinstance(payload, list):
    raise SystemExit(1)
names = [item.get("name", "") for item in payload if isinstance(item, dict)]
names = [name for name in names if name]
if not names:
    raise SystemExit(2)
print("\n".join(names))
'
)" || {
  echo "failed to parse locations response: ${locations_resp}" >&2
  exit 1
}

while IFS= read -r location; do
  [[ -z "${location}" ]] && continue
  echo "==> request wireguard config (${location})"
  wg_payload=$(cat <<EOF
{"location":"${location}"}
EOF
)
  wg_resp="$(curl -sS -X POST "${API_BASE_URL}/wireguard/config" -H "Authorization: Bearer ${token}" -H "Content-Type: application/json" -d "${wg_payload}")"
  peer_id="$(printf '%s' "${wg_resp}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("peerId",""))')"
  config_present="$(printf '%s' "${wg_resp}" | python3 -c 'import json,sys; print("yes" if json.load(sys.stdin).get("config","") else "no")')"
  if [[ -z "${peer_id}" || "${config_present}" != "yes" ]]; then
    echo "wireguard config request failed for ${location}: ${wg_resp}" >&2
    exit 1
  fi
  echo "location ${location}: ok (peer ${peer_id})"
done <<< "${locations}"

echo "smoke test passed"
