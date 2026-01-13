# Design Document — Kubernetes Replica Management Service (Level 4)

## 1. Overview

This service exposes an HTTP API for retrieving and updating the replica count of Kubernetes Deployments.

The service maintains an in-memory cache of Deployment replica counts using Kubernetes watches and enforces mutual TLS (mTLS) for all inbound API requests.

Primary goals of the service are to:
- Provide a simple and secure interface for interacting with Deployment replica counts
- Minimize Kubernetes API load by serving read requests from a cached view of cluster state
- Support a smooth developer workflow with automated builds, testing, and deployment via make
- Offer a configurable Helm chart that supports safe, zero-downtime upgrades

## 2. Scope

In Scope:
- HTTP API server implemented in Go
- Listing, retrieval, and modification of Kubernetes Deployment replica counts
- Informer-based caching of replica counts
- Health check endpoint verifying Kubernetes connectivity
- Mutual TLS authentication between clients and the API server
- Helm chart including Deployment, ServiceAccount, and Service resources
- Build, test, and deployment automation via Makefile
- Integration tests executed against a local Kubernetes cluster (KIND)

Out of Scope:
- Horizontal Pod Autoscaling logic
- Multi-namespace management beyond an explicitly configured namespace
- Persistent data storage


## 3. Architecture & Design Approach
### 3.1 Components

| Component         | Responsibility |
|------------------|----------------|
| API Server       | Exposes HTTP endpoints, validates requests, enforces mTLS authentication, and serves data from the cache. |
| Kubernetes Client| Uses client-go to watch Deployments and perform replica updates. |
| Replica Cache    | Maintains cached Deployment replica counts, updated via a shared informer. |
| mTLS Layer       | Configures mutual TLS using Go’s crypto/tls for secure client-server communication. |
| Helm Chart       | Deploys the service with configurable TLS secrets, RBAC, and networking resources. |


### 3.2 Data Flow

The server initializes a shared Deployment informer on startup, which lists and watches Deployments.
Informer events keep the replica cache up to date.
Read requests are served from cache only.
Write requests patch Deployments and rely on informer updates for cache consistency.
All API access is secured with mTLS.


## 4. Proposed API

Base path: /api/v1

GET /deployments

Returns the list of Kubernetes Deployments visible to the service.

Response (200):

```json
{
  "deployments": ["frontend", "orders", "worker"]
}
```

GET /deployments/{name}/replicas

Returns the cached replica count for the specified Deployment.

Response (200):

```json
{
  "name": "frontend",
  "replicas": 3
}
```

POST /deployments/{name}/replicas

Updates the desired replica count for the Deployment.

Request:

```json
{ 
  "replicas": 5
}
```

Response (200):

```json
{ 
  "status": "updated"
}
```

GET /healthz

Checks Kubernetes client connectivity by performing a lightweight API call

## 5. Replica Cache & Pod Lifecycle Considerations
### 5.1 Informer-based Caching

The service uses a shared client-go Deployment informer to maintain an in-memory cache of replica counts.

The cache tracks, per Deployment:
- Name
- Namespace
- Desired replica count (spec.replicas)
- Observed generation (optional, for debugging and visibility)

### 5.2 Cache Consistency Guarantees

Write requests update the desired replica count by issuing a patch or update to the Kubernetes Deployment.

The Deployment informer observes the change and asynchronously updates the in-memory cache.

Read requests are always served from the cache and never trigger direct Kubernetes API calls.

The system is intentionally eventually consistent, which is acceptable for this use case.

### 5.3 Pod Lifecycle

Updating spec.replicas triggers the Kubernetes Deployment controller to reconcile the desired state by managing underlying ReplicaSets.

The service observes these changes via Deployment informer events.

The service does not directly interact with ReplicaSets or Pods.

## 6. TLS & Security
### 6.1 mTLS Requirements

All API endpoints are secured using mutual TLS (mTLS).

The server presents a certificate signed by an internal Certificate Authority (CA), and clients must present a certificate issued by the same CA. Certificate verification is enforced at the TLS connection layer.

Client certificate identity may optionally be used for authorization decisions (e.g., validating the certificate subject), though fine-grained RBAC is out of scope for the initial implementation.

### 6.2 Secret Management

TLS materials are provided to the service via Kubernetes Secrets, including:
- Server certificate and private key
- CA certificate bundle used to validate client certificates

For local development, a Makefile target generates a local CA along with server and client certificates using standard tooling.

### 6.3 Threat Model

| Risk                        | Mitigation |
|-----------------------------|------------|
| Unauthorized API access     | Mandatory mTLS authentication |
| Man-in-the-middle attacks  | TLS 1.2+, strict CA verification |
| Excessive cluster requests | Informer-based caching and watches |
| Unsafe replica updates     | Input validation and controlled patch operations |

## 7. Developer Workflow
### 7.1 Dependencies

The project requires the following tools:
- Go 1.22 or newer
- Docker
- KIND
- kubectl
- make

### 7.2 Local Development Flow

Local development and testing are driven through Makefile targets:

make kind-up          # Create a local Kubernetes cluster
make build            # Build the Go binary
make docker-build     # Build the container image
make deploy           # Deploy the service via Helm
make test             # Run unit tests
make integration-test # Run integration tests against the local cluster

### 7.3 Fresh Clone Experience

From a fresh clone, a developer should be able to:

- Clone the repository
- Run make setup to provision a local KIND cluster, generate TLS certificates, and load the container image
- Run make deploy to install the service
- Interact with the API via port-forwarding or a local client

## 8. Build & Release Automation (Level 4)
### 8.1 Makefile Targets

Build, packaging, deployment, and testing are automated via Makefile targets:

make build              Builds the local Go binary
make docker-build       Builds the container image
make docker-push        Pushes the image to a registry (local KIND or GHCR)
make deploy             Installs or upgrades the service using Helm
make integration-test   Executes integration tests against the local cluster

### 8.2 CI Pipeline (Optional)

A lightweight CI pipeline may include:
- Linting using golangci-lint
- Unit tests
- Container image build
- Integration tests executed against an ephemeral KIND cluster

## 9. Helm Chart
The service is packaged and deployed using a configurable Helm chart.

Included Kubernetes resources:
- Deployment
- ClusterIP Service
- ServiceAccount
- Role and RoleBinding granting read/write access to Deployments within the target namespace
- Secret containing TLS materials
- ConfigMap for server configuration

### Upgrade Strategy

The Deployment is configured to use a rolling update strategy with maxUnavailable set to 0 and maxSurge set to 1 to avoid service disruption during upgrades.

Helm upgrades are expected to complete without API downtime.

### Configuration

The Helm chart exposes values for:
- TLS secret configuration (including optional external secret references)
- Target namespace
- Resource requests and limits
- Logging verbosity

## 10. Testing Strategy

### 10.1 Unit Tests

Unit tests cover core logic and error handling, including:
- Replica cache initialization and informer-driven updates
- Validation of replica update requests
- mTLS authentication failures (unhappy path)
- API error scenarios such as unknown Deployments or invalid replica counts

### 10.2 Integration Tests

Integration tests are executed via make integration-test against a local KIND cluster.

The test flow includes:
- Deploying the service into the cluster
- Creating one or more sample Deployments
- Verifying API behavior, including:
  - Listing available Deployments
  - Reading replica counts from the cache
  - Updating replica counts and observing informer-synchronized state
  - Validating the health check endpoint

## 11. Conclusion

This design describes a secure, cache-driven Kubernetes service that meets Teleport’s Level 4 challenge requirements. It emphasizes correctness, performance, and operational simplicity while providing a clear foundation for future enhancements without adding unnecessary complexity.