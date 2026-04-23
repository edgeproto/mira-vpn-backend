#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:18080}"
EXPECTED_WG_ENDPOINT="${EXPECTED_WG_ENDPOINT:-}"
EMAIL="${SMOKE_EMAIL:-real-smoke-$(date +%s)@example.com}"
PASSWORD="${SMOKE_PASSWORD:-changeme123}"
LOCATION="${SMOKE_LOCATION:-Finland}"
DEVICE_ID="${SMOKE_DEVICE_ID:-real-device-$(date +%s)}"

if [[ -z "${EXPECTED_WG_ENDPOINT}" ]]; then
  echo "EXPECTED_WG_ENDPOINT is required, e.g. 95.217.206.233:51820" >&2
  exit 1
fi

echo "==> [auth flow] register user: ${EMAIL}"
register_payload=$(cat <<EOF
{"email":"${EMAIL}","password":"${PASSWORD}"}
EOF
)
curl -sS -X POST "${API_BASE_URL}/auth/register" -H "Content-Type: application/json" -d "${register_payload}" >/dev/null

echo "==> [auth flow] login user"
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

echo "==> [auth flow] request wireguard config (${LOCATION})"
wg_payload=$(cat <<EOF
{"location":"${LOCATION}"}
EOF
)
wg_resp="$(curl -sS -X POST "${API_BASE_URL}/wireguard/config" -H "Authorization: Bearer ${token}" -H "Content-Type: application/json" -d "${wg_payload}")"
auth_peer_id="$(printf '%s' "${wg_resp}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("peerId",""))')"
auth_config="$(printf '%s' "${wg_resp}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("config",""))')"
if [[ -z "${auth_peer_id}" || -z "${auth_config}" ]]; then
  echo "auth wireguard config request failed: ${wg_resp}" >&2
  exit 1
fi
if [[ "${auth_config}" != *"Endpoint = ${EXPECTED_WG_ENDPOINT}"* ]]; then
  echo "auth config endpoint mismatch, expected Endpoint = ${EXPECTED_WG_ENDPOINT}" >&2
  exit 1
fi
echo "auth peer provisioned: ${auth_peer_id}"

echo "==> [guest flow] request wireguard config (${LOCATION})"
guest_payload=$(cat <<EOF
{"deviceId":"${DEVICE_ID}","location":"${LOCATION}"}
EOF
)
guest_resp="$(curl -sS -X POST "${API_BASE_URL}/wireguard/config/guest" -H "Content-Type: application/json" -d "${guest_payload}")"
guest_peer_id="$(printf '%s' "${guest_resp}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("peerId",""))')"
guest_config="$(printf '%s' "${guest_resp}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("config",""))')"
if [[ -z "${guest_peer_id}" || -z "${guest_config}" ]]; then
  echo "guest wireguard config request failed: ${guest_resp}" >&2
  exit 1
fi
if [[ "${guest_config}" != *"Endpoint = ${EXPECTED_WG_ENDPOINT}"* ]]; then
  echo "guest config endpoint mismatch, expected Endpoint = ${EXPECTED_WG_ENDPOINT}" >&2
  exit 1
fi
echo "guest peer provisioned: ${guest_peer_id}"

echo "real smoke test passed"
