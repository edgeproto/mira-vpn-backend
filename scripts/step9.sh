#!/usr/bin/env bash
set -euo pipefail

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-mira_vpn_step9}"
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-mira_vpn}"
export API_HOST_PORT="${API_HOST_PORT:-18080}"

echo "==> resetting step9 compose project (${PROJECT_NAME})"
docker compose -p "${PROJECT_NAME}" down -v --remove-orphans >/dev/null 2>&1 || true

echo "==> starting postgres + api via docker compose"
echo "    (run mira-vpn-wgmgr in mock mode on the host on :9090 so the API can reach WGMGR_BASE_URL, default http://host.docker.internal:9090)"
docker compose -p "${PROJECT_NAME}" up -d --build postgres migrations api

echo "==> waiting for api health"
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${API_HOST_PORT}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${API_HOST_PORT}/health" >/dev/null
echo "api is healthy"

echo "==> running smoke flow"
API_BASE_URL="http://127.0.0.1:${API_HOST_PORT}" ./scripts/smoke.sh

echo "==> done"
echo "To stop stack: docker compose -p ${PROJECT_NAME} down -v"
