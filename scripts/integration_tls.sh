#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

LOG=/tmp/replica-manager.log
: > "$LOG"

echo "[1] build"
make build >/dev/null

echo "[2] generate certs"
make certs >/dev/null

echo "[3] start server with mTLS"
LISTEN_ADDR=:8443 \
PROBE_LISTEN_ADDR=:8081 \
TLS_ENABLED=true \
TLS_CERT_FILE=certs/server.crt \
TLS_KEY_FILE=certs/server.key \
TLS_CLIENT_CA_FILE=certs/ca.crt \
./bin/server >"$LOG" 2>&1 &

PID=$!
trap 'kill $PID >/dev/null 2>&1 || true' EXIT

# If the server dies immediately, show logs and fail fast.
sleep 0.2
if ! kill -0 "$PID" >/dev/null 2>&1; then
  echo "ERROR: server exited immediately. Logs:"
  tail -200 "$LOG"
  exit 1
fi

echo "[4] wait for probe health"
for i in {1..50}; do
  if curl -fsS http://localhost:8081/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

if ! curl -fsS http://localhost:8081/healthz >/dev/null 2>&1; then
  echo "ERROR: probe endpoint never became reachable. Logs:"
  tail -200 "$LOG"
  exit 1
fi

echo "[5] API without client cert should fail"
if curl -fsS --cacert certs/ca.crt https://localhost:8443/api/v1/deployments >/dev/null 2>&1; then
  echo "ERROR: request without client cert unexpectedly succeeded"
  exit 1
fi

echo "[6] API with client cert should succeed (200 or 503 if cache not synced yet)"
code="$(curl -s -o /dev/null -w "%{http_code}" \
  --cacert certs/ca.crt \
  --cert certs/client.crt \
  --key certs/client.key \
  https://localhost:8443/api/v1/deployments || true)"

if [[ "$code" != "200" && "$code" != "503" ]]; then
  echo "ERROR: expected 200 or 503, got $code. Logs:"
  tail -200 "$LOG"
  exit 1
fi

echo "OK"
