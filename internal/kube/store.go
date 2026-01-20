package kube

import "context"

// Store provides cached reads and write operations against Kubernetes Deployments.
// Reads should be served from cache (informer), not direct API calls.
type Store interface {
	// Ready reports whether the cache has synced at least once and the store is usable.
	Ready() bool

	// ListDeployments returns cached deployment names.
	ListDeployments(ctx context.Context) ([]string, error)

	// GetReplicas returns cached desired replicas for the given deployment name.
	GetReplicas(ctx context.Context, name string) (int32, bool, error)

	// SetReplicas updates desired replicas in Kubernetes (cache updates asynchronously via informer).
	SetReplicas(ctx context.Context, name string, replicas int32) error
}