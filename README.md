# Kubernetes Replica Management Service

A Go service that exposes an HTTP API for listing Kubernetes Deployments and getting or setting their replica counts.

The service maintains an in-memory cache of Deployment replica counts using Kubernetes informers and requires Kubernetes connectivity at startup.

## Requirements

- Go 1.25+
- make
- Docker
- kind
- kubectl
- Helm (for Kubernetes deployment)

Linux users may need a C toolchain installed (e.g. `sudo apt install -y build-essential`).

---

## Quickstart (Fresh Clone)

```bash
make setup
```

This will:

- create a KIND cluster
- build and load the Docker image into KIND
- generate local TLS certificates
- deploy the service to Kubernetes via Helm with mTLS enabled

---

## Local Development (KIND)

### 1. Create a local cluster

```bash
make kind-up
```

Verify the cluster is reachable:

```bash
kubectl get nodes
```

---

### 2. Build and run the service locally

```bash
make run
```

On startup, the service will:

- initialize a Kubernetes client
- start a Deployment informer
- wait for the informer cache to sync
- begin serving HTTP requests

By default:

- API server listens on `:8080`
- Probe server listens on `:8081`

Addresses can be overridden using environment variables:

```bash
LISTEN_ADDR=:8080 PROBE_LISTEN_ADDR=:8081 make run
```

---

### 3. Health and readiness checks

API (HTTP by default):

```bash
curl http://localhost:8080/api/v1/deployments
```

Probe endpoints (always HTTP):

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

---

## Manual Verification (Optional)

### 1. Create a sample Deployment

```bash
kubectl create deployment demo --image=nginx --replicas=2
```

Verify via Kubernetes:

```bash
kubectl get deploy demo -o jsonpath='{.spec.replicas}{"\n"}'
```

---

### 2. List Deployments via the API

```bash
curl http://localhost:8080/api/v1/deployments
```

---

### 3. Get replica count from the cache

```bash
curl http://localhost:8080/api/v1/deployments/demo/replicas
```

---

### 4. Update replica count via the API

```bash
curl -X POST http://localhost:8080/api/v1/deployments/demo/replicas \
  -H "Content-Type: application/json" \
  -d '{"replicas": 5}'
```

Verify via Kubernetes:

```bash
kubectl get deploy demo -o jsonpath='{.spec.replicas}{"\n"}'
```

Verify via cached API read:

```bash
curl http://localhost:8080/api/v1/deployments/demo/replicas
```

---

## TLS / mTLS (Local)

When TLS is enabled, the API server:

- runs over HTTPS
- requires a valid client certificate (mTLS)
- probe endpoints remain HTTP and unauthenticated

### 1. Generate local certificates

```bash
make certs
```

Certificates are written to `./certs/`.

---

### 2. Run the service with mTLS enabled

```bash
LISTEN_ADDR=:8443 \
PROBE_LISTEN_ADDR=:8081 \
TLS_ENABLED=true \
TLS_CERT_FILE=certs/server.crt \
TLS_KEY_FILE=certs/server.key \
TLS_CLIENT_CA_FILE=certs/ca.crt \
make run
```

---

### 3. Verify behavior

Probe endpoints still work without TLS:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

API without client cert fails:

```bash
curl --cacert certs/ca.crt https://localhost:8443/api/v1/deployments
```

API with valid client cert succeeds:

```bash
curl \
  --cacert certs/ca.crt \
  --cert certs/client.crt \
  --key certs/client.key \
  https://localhost:8443/api/v1/deployments
```

---

## Kubernetes Deployment (Helm)

### Deploy via Makefile (recommended)

```bash
make certs
make deploy
```

This deploys the service into the `replica-manager` namespace with mTLS enabled.

---

### Verify deployment

```bash
kubectl get pods -n replica-manager
kubectl logs -n replica-manager -l app.kubernetes.io/name=k8-replica-manager
```

---

### Access the service

Port-forward the API service:

```bash
kubectl port-forward -n replica-manager svc/replica-manager-k8-replica-manager 8443:8080
```

Then test with mTLS:

```bash
curl \
  --cacert certs/ca.crt \
  --cert certs/client.crt \
  --key certs/client.key \
  https://localhost:8443/api/v1/deployments
```

Probe endpoints:

```bash
kubectl port-forward -n replica-manager pod/<pod-name> 8081:8081
curl http://localhost:8081/healthz
```

---

## Tests

### Unit tests

```bash
go test ./...
```

### Integration test (Helm + KIND + mTLS)

```bash
make integration-test
```

This test:

- ensures a KIND cluster exists
- builds and loads the Docker image
- deploys the service via Helm with mTLS enabled
- waits for readiness
- verifies:
  - probes work unauthenticated
  - API fails without a client certificate
  - API succeeds with a valid client certificate
  - replica list/get/set works end-to-end
- cleans up the Helm release and namespace

---

## Cleanup

```bash
make undeploy
make kind-down
```