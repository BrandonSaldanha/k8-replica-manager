package kube

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Manager implements Store using a client-go shared informer and an in-memory cache.
type Manager struct {
	namespace string
	client    kubernetes.Interface

	// informer lifecycle
	factory informers.SharedInformerFactory
	synced  cache.InformerSynced
	stopCh  chan struct{}

	// cache
	mu       sync.RWMutex
	replicas map[string]int32

	// readiness
	readyMu sync.RWMutex
	ready   bool
}

// NewManager constructs a Manager and starts the Deployment informer in the background.
// Call Shutdown() to stop the informer.
func NewManager(namespace string) (*Manager, error) {
	if namespace == "" {
		namespace = "default"
	}

	cfg, err := buildRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	// Shared informer scoped to namespace.
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		30*time.Second,
		informers.WithNamespace(namespace),
	)

	deployInformer := factory.Apps().V1().Deployments().Informer()

	m := &Manager{
		namespace: namespace,
		client:    client,
		factory:   factory,
		synced:    deployInformer.HasSynced,
		stopCh:    make(chan struct{}),
		replicas:  make(map[string]int32),
	}

	// Register event handlers to keep cache updated.
	_, err = deployInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.onAddOrUpdate,
		UpdateFunc: func(_, newObj any) { m.onAddOrUpdate(newObj) },
		DeleteFunc: m.onDelete,
	})
	if err != nil {
		return nil, fmt.Errorf("add informer handler: %w", err)
	}

	// Start informers
	factory.Start(m.stopCh)

	// Wait for initial sync in background; mark ready when synced.
	go func() {
		if ok := cache.WaitForCacheSync(m.stopCh, m.synced); !ok {
			log.Printf("kube cache sync did not complete")
			return
		}
		m.readyMu.Lock()
		m.ready = true
		m.readyMu.Unlock()
		log.Printf("kube cache synced for namespace=%s", m.namespace)
	}()

	return m, nil
}

// Shutdown stops informers.
func (m *Manager) Shutdown() {
	close(m.stopCh)
}

// Ready returns true once the informer cache has synced at least once.
func (m *Manager) Ready() bool {
	m.readyMu.RLock()
	defer m.readyMu.RUnlock()
	return m.ready
}

func (m *Manager) ListDeployments(ctx context.Context) ([]string, error) {
	// No cluster calls; return keys from cache.
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, len(m.replicas))
	for name := range m.replicas {
		out = append(out, name)
	}
	return out, nil
}

func (m *Manager) GetReplicas(ctx context.Context, name string) (int32, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.replicas[name]
	return v, ok, nil
}

func (m *Manager) SetReplicas(ctx context.Context, name string, replicas int32) error {
	if replicas < 0 {
		return fmt.Errorf("replicas must be >= 0")
	}

	// Patch spec.replicas only.
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)

	_, err := m.client.AppsV1().Deployments(m.namespace).Patch(
		ctx,
		name,
		types.MergePatchType,
		[]byte(patch),
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patch deployment replicas: %w", err)
	}
	return nil
}

// Ping is used for readiness checks to verify Kubernetes API connectivity.
func (m *Manager) Ping(ctx context.Context) error {
	// Lightweight call: list deployments with limit 1.
	_, err := m.client.AppsV1().Deployments(m.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("kubernetes connectivity check failed: %w", err)
	}
	return nil
}

func (m *Manager) onAddOrUpdate(obj any) {
	d, ok := obj.(*appsv1.Deployment)
	if !ok || d == nil {
		return
	}

	var rep int32 = 0
	if d.Spec.Replicas != nil {
		rep = *d.Spec.Replicas
	}

	m.mu.Lock()
	m.replicas[d.Name] = rep
	m.mu.Unlock()
}

func (m *Manager) onDelete(obj any) {
	// Delete events can come as Deployment or as tombstone.
	d, ok := obj.(*appsv1.Deployment)
	if !ok {
		t, ok2 := obj.(cache.DeletedFinalStateUnknown)
		if ok2 {
			d, _ = t.Obj.(*appsv1.Deployment)
		}
	}
	if d == nil {
		return
	}

	m.mu.Lock()
	delete(m.replicas, d.Name)
	m.mu.Unlock()
}

// buildRESTConfig tries in-cluster config first, then falls back to local kubeconfig.
func buildRESTConfig() (*rest.Config, error) {
	// In-cluster (when running in Kubernetes)
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	// Local kubeconfig for dev
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig (%s): %w", kubeconfig, err)
	}
	return cfg, nil
}
