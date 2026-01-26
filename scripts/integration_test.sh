#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CLUSTER_NAME="${CLUSTER_NAME:-replica-manager}"
NAMESPACE="${NAMESPACE:-replica-manager}"
RELEASE="${RELEASE:-replica-manager}"
CHART_DIR="${CHART_DIR:-./charts/k8-replica-manager}"

IMAGE_REPO="${IMAGE_REPO:-k8-replica-manager}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

API_LOCAL_PORT="${API_LOCAL_PORT:-8443}"     # local port for API port-forward
PROBE_LOCAL_PORT="${PROBE_LOCAL_PORT:-8081}" # local port for probe port-forward

DELETE_CLUSTER="${DELETE_CLUSTER:-false}"     # set true to delete kind cluster at end

log() { printf "\n==> %s\n" "$*"; }

cleanup() {
  set +e
  log "cleanup: stop port-forwards"
  [[ -n "${PF_API_PID:-}" ]] && kill "${PF_API_PID}" >/dev/null 2>&1 || true
  [[ -n "${PF_PROBE_PID:-}" ]] && kill "${PF_PROBE_PID}" >/dev/null 2>&1 || true

  log "cleanup: uninstall helm release + namespace"
  helm uninstall "${RELEASE}" -n "${NAMESPACE}" >/dev/null 2>&1 || true
  kubectl delete namespace "${NAMESPACE}" >/dev/null 2>&1 || true

  if [[ "${DELETE_CLUSTER}" == "true" ]]; then
    log "cleanup: delete kind cluster ${CLUSTER_NAME}"
    kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1"; exit 1; }
}

log "check deps"
require kind
require kubectl
require helm
require docker
require curl
require openssl

log "ensure kind cluster exists"
if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  kind create cluster --name "${CLUSTER_NAME}"
fi
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

log "build docker image"
docker build -t "${IMAGE_REPO}:${IMAGE_TAG}" .

log "load image into kind"
kind load docker-image "${IMAGE_REPO}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

log "generate certs if missing"
if [[ ! -f certs/ca.crt || ! -f certs/server.crt || ! -f certs/server.key || ! -f certs/client.crt || ! -f certs/client.key ]]; then
  make certs >/dev/null
fi

log "install helm chart (mTLS enabled)"
helm upgrade --install "${RELEASE}" "${CHART_DIR}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set image.repository="${IMAGE_REPO}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set tls.enabled=true \
  --set tls.createSecret=true \
  --set-file tls.serverCrt=certs/server.crt \
  --set-file tls.serverKey=certs/server.key \
  --set-file tls.clientCA=certs/ca.crt

APP_LABEL="app.kubernetes.io/instance=${RELEASE}"
NAME_LABEL="app.kubernetes.io/name=k8-replica-manager"

log "find deployment + wait for ready"
DEPLOY="$(kubectl get deploy -n "${NAMESPACE}" -l "${APP_LABEL},${NAME_LABEL}" -o jsonpath='{.items[0].metadata.name}')"
echo "deployment=${DEPLOY}"
kubectl rollout status "deployment/${DEPLOY}" -n "${NAMESPACE}" --timeout=120s

log "find service"
SVC="$(kubectl get svc -n "${NAMESPACE}" -l "${APP_LABEL},${NAME_LABEL}" -o jsonpath='{.items[0].metadata.name}')"
echo "service=${SVC}"

log "find pod"
POD="$(kubectl get pod -n "${NAMESPACE}" -l "${APP_LABEL},${NAME_LABEL}" -o jsonpath='{.items[0].metadata.name}')"
echo "pod=${POD}"

log "port-forward API service to localhost:${API_LOCAL_PORT}"
kubectl port-forward -n "${NAMESPACE}" "svc/${SVC}" "${API_LOCAL_PORT}:8080" >/tmp/pf-api.log 2>&1 &
PF_API_PID=$!

log "port-forward probe port (pod) to localhost:${PROBE_LOCAL_PORT}"
kubectl port-forward -n "${NAMESPACE}" "pod/${POD}" "${PROBE_LOCAL_PORT}:8081" >/tmp/pf-probe.log 2>&1 &
PF_PROBE_PID=$!

log "wait for probe health"
for i in {1..60}; do
  if curl -fsS "http://localhost:${PROBE_LOCAL_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
curl -fsS "http://localhost:${PROBE_LOCAL_PORT}/healthz" >/dev/null

log "wait for readiness (cache sync + kube reachable)"
for i in {1..120}; do
  if curl -fsS "http://localhost:${PROBE_LOCAL_PORT}/readyz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
curl -fsS "http://localhost:${PROBE_LOCAL_PORT}/readyz" >/dev/null

log "API without client cert should fail"
if curl -fsS --cacert certs/ca.crt "https://localhost:${API_LOCAL_PORT}/api/v1/deployments" >/dev/null 2>&1; then
  echo "ERROR: request without client cert unexpectedly succeeded"
  exit 1
fi

log "API with client cert should succeed"
curl -fsS \
  --cacert certs/ca.crt \
  --cert certs/client.crt \
  --key certs/client.key \
  "https://localhost:${API_LOCAL_PORT}/api/v1/deployments" >/dev/null

log "create demo deployment"
kubectl -n "${NAMESPACE}" create deployment demo --image=nginx --replicas=2 >/dev/null
kubectl -n "${NAMESPACE}" rollout status deployment/demo --timeout=120s >/dev/null

log "API list includes demo (eventually consistent)"
for i in {1..60}; do
  out="$(curl -fsS \
    --cacert certs/ca.crt \
    --cert certs/client.crt \
    --key certs/client.key \
    "https://localhost:${API_LOCAL_PORT}/api/v1/deployments")"
  if echo "$out" | grep -q '"demo"'; then
    break
  fi
  sleep 0.5
done
echo "$out" | grep -q '"demo"'

log "API get replicas for demo should be 2 (eventually consistent)"
for i in {1..60}; do
  rep="$(curl -fsS \
    --cacert certs/ca.crt \
    --cert certs/client.crt \
    --key certs/client.key \
    "https://localhost:${API_LOCAL_PORT}/api/v1/deployments/demo/replicas" | sed -n 's/.*"replicas":[ ]*\([0-9]\+\).*/\1/p')"
  if [[ "${rep:-}" == "2" ]]; then
    break
  fi
  sleep 0.5
done
[[ "${rep:-}" == "2" ]]

log "API set replicas to 3"
curl -fsS -X POST \
  --cacert certs/ca.crt \
  --cert certs/client.crt \
  --key certs/client.key \
  -H "Content-Type: application/json" \
  -d '{"replicas":3}' \
  "https://localhost:${API_LOCAL_PORT}/api/v1/deployments/demo/replicas" >/dev/null

log "verify Kubernetes spec.replicas is 3"
for i in {1..60}; do
  krep="$(kubectl -n "${NAMESPACE}" get deploy demo -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
  if [[ "${krep:-}" == "3" ]]; then
    break
  fi
  sleep 0.5
done
[[ "${krep:-}" == "3" ]]

log "OK: integration test passed"
