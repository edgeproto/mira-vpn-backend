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

PROFILES_FILE="$(read_env_line WGMGR_LOCATION_PROFILES_FILE)"
PROFILES_FILE="${PROFILES_FILE:-config/location-profiles.json}"
if [[ "${PROFILES_FILE}" == /etc/mira-config/* ]]; then
  PROFILES_FILE="./config/${PROFILES_FILE##*/}"
fi
if [[ ! -f "${PROFILES_FILE}" ]]; then
  echo "location profiles file not found: ${PROFILES_FILE}" >&2
  exit 1
fi

WGMGR_ADMIN_TOKEN_DEFAULT="$(read_env_line WGMGR_ADMIN_TOKEN_DEFAULT)"

echo "==> checking per-POP wgmgr health from ${PROFILES_FILE}"
profiles="$(
  python3 - "${PROFILES_FILE}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)

if not isinstance(data, list) or not data:
    raise SystemExit("location profiles must be a non-empty list")

for item in data:
    if not isinstance(item, dict):
        continue
    name = (item.get("name") or "").strip()
    base = (item.get("wgmgrBaseUrl") or "").strip()
    if name and base:
        print(f"{name}\t{base}")
PY
)"
if [[ -z "${profiles}" ]]; then
  echo "no profiles with wgmgrBaseUrl found in ${PROFILES_FILE}" >&2
  exit 1
fi

while IFS=$'\t' read -r profile_name wgmgr_base_url; do
  [[ -z "${profile_name}" || -z "${wgmgr_base_url}" ]] && continue
  token_var_suffix="$(printf '%s' "${profile_name}" | tr '[:lower:]' '[:upper:]' | sed -E 's/[^A-Z0-9]+/_/g')"
  token_var="WGMGR_ADMIN_TOKEN_${token_var_suffix}"
  token_override="$(read_env_line "${token_var}")"
  token="${token_override:-${WGMGR_ADMIN_TOKEN_DEFAULT}}"
  auth_headers=()
  if [[ -n "${token}" ]]; then
    auth_headers=(-H "Authorization: Bearer ${token}")
  fi

  echo "   - ${profile_name}: ${wgmgr_base_url}/health"
  curl -fsS "${wgmgr_base_url}/health" "${auth_headers[@]}" >/dev/null
done <<< "${profiles}"
echo "all configured POP health checks passed"

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

echo "==> running multi-location smoke flow (auth + config per location)"
API_BASE_URL="http://127.0.0.1:${API_HOST_PORT}" ./scripts/smoke.sh

echo "==> done"
echo "To stop stack: docker compose -p ${PROJECT_NAME} -f docker-compose.yml -f docker-compose.real.yml --env-file ${ENV_FILE} down -v"
