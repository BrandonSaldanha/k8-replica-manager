package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

type fakeStore struct {
	ready       bool
	deployments []string
	replicas    map[string]int32
	setErr      error
}

func (f *fakeStore) Ready() bool { return f.ready }

func (f *fakeStore) Ping(ctx context.Context) error { return nil }

func (f *fakeStore) ListDeployments(ctx context.Context) ([]string, error) {
	return append([]string(nil), f.deployments...), nil
}

func (f *fakeStore) GetReplicas(ctx context.Context, name string) (int32, bool, error) {
	v, ok := f.replicas[name]
	return v, ok, nil
}

func (f *fakeStore) SetReplicas(ctx context.Context, name string, replicas int32) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.replicas == nil {
		f.replicas = map[string]int32{}
	}
	// simulate immediate cache update for unit test simplicity
	f.replicas[name] = replicas
	return nil
}

var _ kube.Store = (*fakeStore)(nil)
var _ kube.Pinger = (*fakeStore)(nil)

func TestHealthzOK(t *testing.T) {
	s := New(config.Config{ListenAddr: ":0", ProbeListenAddr: ":0"}, &fakeStore{ready: true})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	s.handleHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status"`) {
		t.Fatalf("expected json body, got %q", rr.Body.String())
	}
}

func TestReadyzOK(t *testing.T) {
	s := New(config.Config{ListenAddr: ":0", ProbeListenAddr: ":0"}, &fakeStore{ready: true})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	s.handleReadyz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestSetReplicasRejectsNegative(t *testing.T) {
	s := New(config.Config{ListenAddr: ":0", ProbeListenAddr: ":0"}, &fakeStore{ready: true, replicas: map[string]int32{"frontend": 1}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/frontend/replicas", strings.NewReader(`{"replicas":-1}`))
	rr := httptest.NewRecorder()

	s.routeAPIv1(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}
