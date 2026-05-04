#!/usr/bin/env bash
set -euo pipefail

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-mira_vpn_real}"
ENV_FILE="${ENV_FILE:-.env.real}"
API_HOST_PORT="${API_HOST_PORT:-18080}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing ${ENV_FILE}. Create it from .env.real.example first." >&2
  exit 1
fi

echo "==> reading ${ENV_FILE} (compose loads full file; shell only needs a few keys for preflight)"

# Do not `source` the whole env file: multi-line JSON or special characters in
# WGMGR_LOCATION_PROFILES_* would break bash.
read_env_line() {
  local key="$1"
  local line
  line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | head -n1 || true)"
  line="${line#${key}=}"
  line="${line%$'\r'}"
  if [[ "${line}" == \"*\" ]]; then
    line="${line#\"}"
    line="${line%\"}"
  elif [[ "${line}" == \'*\' ]]; then
    line="${line#\'}"
    line="${line%\'}"
  fi
  printf '%s' "${line}"
}

WGMGR_REAL_ENDPOINT="$(read_env_line WGMGR_REAL_ENDPOINT)"
WGMGR_REAL_SERVER_PUBLIC_KEY="$(read_env_line WGMGR_REAL_SERVER_PUBLIC_KEY)"
export WGMGR_REAL_ENDPOINT WGMGR_REAL_SERVER_PUBLIC_KEY

if [[ -z "${WGMGR_REAL_ENDPOINT}" ]]; then
  echo "WGMGR_REAL_ENDPOINT must be set in ${ENV_FILE}" >&2
  exit 1
fi
if [[ -z "${WGMGR_REAL_SERVER_PUBLIC_KEY}" ]]; then
  echo "WGMGR_REAL_SERVER_PUBLIC_KEY must be set in ${ENV_FILE}" >&2
  exit 1
fi

WG_INTERFACE="$(read_env_line WGMGR_REAL_INTERFACE)"
WG_INTERFACE="${WG_INTERFACE:-wg0}"
if ! command -v wg >/dev/null 2>&1; then
  echo "wg binary not found on host; install wireguard-tools before running real mode." >&2
  exit 1
fi

echo "==> validating server public key against host interface (${WG_INTERFACE})"
HOST_SERVER_PUBLIC_KEY="$(wg show "${WG_INTERFACE}" public-key 2>/dev/null || true)"
if [[ -z "${HOST_SERVER_PUBLIC_KEY}" ]]; then
  echo "failed to read public key for interface ${WG_INTERFACE}. Ensure interface exists and is up." >&2
  exit 1
fi
if [[ "${HOST_SERVER_PUBLIC_KEY}" != "${WGMGR_REAL_SERVER_PUBLIC_KEY}" ]]; then
  echo "server public key mismatch for ${WG_INTERFACE}" >&2
  echo "host key: ${HOST_SERVER_PUBLIC_KEY}" >&2
  echo "env  key: ${WGMGR_REAL_SERVER_PUBLIC_KEY}" >&2
  echo "update WGMGR_REAL_SERVER_PUBLIC_KEY in ${ENV_FILE} to match host key, then retry." >&2
  exit 1
fi
echo "server public key matches host interface"

export API_HOST_PORT
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-mira_vpn}"
export POSTGRES_HOST_PORT="${POSTGRES_HOST_PORT:-15432}"

echo "==> resetting compose project (${PROJECT_NAME})"
docker compose -p "${PROJECT_NAME}" -f docker-compose.yml -f docker-compose.real.yml --env-file "${ENV_FILE}" down -v --remove-orphans >/dev/null 2>&1 || true

echo "==> starting real stack (postgres + api; wgmgr must already be listening on 127.0.0.1:9090, e.g. from mira-vpn-wgmgr)"
docker compose -p "${PROJECT_NAME}" -f docker-compose.yml -f docker-compose.real.yml --env-file "${ENV_FILE}" up -d --build postgres migrations api

echo "==> waiting for api health"
for _ in $(seq 1 45); do
  if curl -fsS "http://127.0.0.1:${API_HOST_PORT}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${API_HOST_PORT}/health" >/dev/null
echo "api is healthy"

echo "==> running real-mode smoke flow (auth + guest)"
API_BASE_URL="http://127.0.0.1:${API_HOST_PORT}" EXPECTED_WG_ENDPOINT="${WGMGR_REAL_ENDPOINT}" ./scripts/smoke_real.sh

echo "==> done"
echo "To inspect peers on host: sudo wg show ${WGMGR_REAL_INTERFACE:-wg0}"
echo "To stop stack: docker compose -p ${PROJECT_NAME} -f docker-compose.yml -f docker-compose.real.yml --env-file ${ENV_FILE} down -v"
