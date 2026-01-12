# k8-replica-manager
This service exposes an HTTP API for retrieving and updating the replica count of Kubernetes Deployments.  The service maintains an in-memory cache of Deployment replica counts using Kubernetes watches and enforces mutual TLS (mTLS) for all inbound API requests.
