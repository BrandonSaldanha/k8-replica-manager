# Kubernetes Replica Management Service

A small Go service that exposes an HTTP API for listing Kubernetes Deployments and getting or setting their replica counts.

See `DESIGN.md` for details on the proposed architecture and implementation.

## Requirements

- Go 1.22+
- make

Optional (for later PRs):
- Docker
- kind
- kubectl
- Helm

Linux users may need a C toolchain installed (e.g. sudo apt install -y build-essential).

## Quickstart

Build and run the server locally:

```bash
make run
```

The server listens on `:8080` by default. This can be overridden using the `LISTEN_ADDR` environment variable:

```bash
LISTEN_ADDR=:8081 make run
```

Health check:

```bash
curl -s http://localhost:8080/healthz
```
