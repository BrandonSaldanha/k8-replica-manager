# Kubernetes Replica Management Service

A Go service that exposes an HTTP API for listing Kubernetes Deployments and getting or setting their replica counts.

The service maintains an in-memory cache of Deployment replica counts using Kubernetes informers and requires Kubernetes connectivity at startup.

## Requirements

- Go 1.25+
- make
- Docker
- kind
- kubectl

Optional (for later PRs):
- Helm

Linux users may need a C toolchain installed (e.g. sudo apt install -y build-essential).

## Local Development (KIND)

The service requires access to a Kubernetes cluster. For local development, a KIND cluster is used.

### 1. Create a local cluster

```bash
make kind-up
```

Verify the cluster is reachable:
```bash
kubectl get nodes
```

### 2. Build and run the service
```bash
make run
```
On startup, the service will:

- Initialize a Kubernetes client
- Start a Deployment informer
- Wait for the informer cache to sync
- Begin serving HTTP requests

The server listens on `:8080` by default. This can be overridden using the `LISTEN_ADDR` environment variable:

```bash
LISTEN_ADDR=:8081 make run
```

### 3. Health and readiness checks

Health check (process is running):
```bash
curl -s http://localhost:8080/healthz
```

Readiness check (cache synced + Kubernetes reachable):
```bash
curl http://localhost:8080/readyz
```

## Manual Verification (Optional)

The following steps demonstrate the core API functionality against a local KIND cluster.

### 1. Create a sample Deployment

```bash
kubectl create deployment demo --image=nginx --replicas=2
```

Verify via Kubernetes:
```bash
kubectl get deploy demo -o jsonpath='{.spec.replicas}{"\n"}'
```

### 2. List Deployments via the API

```bash
curl -s http://localhost:8080/api/v1/deployments
```

### 3. Get replica count from the cache

```bash
curl -s http://localhost:8080/api/v1/deployments/demo/replicas
```

### 4. Update replica count via the API

```bash
curl -s -X POST http://localhost:8080/api/v1/deployments/demo/replicas \
  -H "Content-Type: application/json" \
  -d '{"replicas": 5}'
```

Verify via Kubernetes:
```bash
kubectl get deploy demo -o jsonpath='{.spec.replicas}{"\n"}'
```
Verify via cached API read:
```bash
curl -s http://localhost:8080/api/v1/deployments/demo/replicas
```

### 5. Cleanup

```bash
kubectl delete deployment demo
make kind-down
```