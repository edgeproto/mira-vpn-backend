#!/usr/bin/env bash
set -euo pipefail

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-mira_vpn_real}"
ENV_FILE="${ENV_FILE:-.env.real}"
API_HOST_PORT="${API_HOST_PORT:-18080}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing ${ENV_FILE}. Create it from .env.real.example first." >&2
  exit 1
fi

echo "==> loading ${ENV_FILE}"
set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

if [[ -z "${WGMGR_REAL_ENDPOINT:-}" ]]; then
  echo "WGMGR_REAL_ENDPOINT must be set in ${ENV_FILE}" >&2
  exit 1
fi
if [[ -z "${WGMGR_REAL_SERVER_PUBLIC_KEY:-}" ]]; then
  echo "WGMGR_REAL_SERVER_PUBLIC_KEY must be set in ${ENV_FILE}" >&2
  exit 1
fi

export API_HOST_PORT
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-mira_vpn}"
export POSTGRES_HOST_PORT="${POSTGRES_HOST_PORT:-15432}"

echo "==> resetting compose project (${PROJECT_NAME})"
docker compose -p "${PROJECT_NAME}" -f docker-compose.yml -f docker-compose.real.yml --env-file "${ENV_FILE}" down -v --remove-orphans >/dev/null 2>&1 || true

echo "==> starting real stack"
docker compose -p "${PROJECT_NAME}" -f docker-compose.yml -f docker-compose.real.yml --env-file "${ENV_FILE}" up -d --build postgres migrations wgmgr api

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
