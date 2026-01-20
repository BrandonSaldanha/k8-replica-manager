package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

type listDeploymentsResponse struct {
	Deployments []string `json:"deployments"`
}

type getReplicasResponse struct {
	Name     string `json:"name"`
	Replicas int32  `json:"replicas"`
}

type setReplicasRequest struct {
	Replicas int32 `json:"replicas"`
}

type statusResponse struct {
	Status string `json:"status"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		http.Error(w, "store not configured", http.StatusServiceUnavailable)
		return
	}
	if !s.store.Ready() {
		http.Error(w, "cache not synced", http.StatusServiceUnavailable)
		return
	}

	// Verify k8s connectivity with a short timeout.
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	pinger, ok := s.store.(interface{ Ping(context.Context) error })
	if ok {
		if err := pinger.Ping(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	deps, err := s.store.ListDeployments(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Stable ordering helps tests and makes the API nicer to consume.
	sort.Strings(deps)

	writeJSON(w, http.StatusOK, listDeploymentsResponse{Deployments: deps})
}

func (s *Server) handleGetReplicas(w http.ResponseWriter, r *http.Request, name string) {
	rep, ok, err := s.store.GetReplicas(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "deployment not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, getReplicasResponse{Name: name, Replicas: rep})
}

func (s *Server) handleSetReplicas(w http.ResponseWriter, r *http.Request, name string) {
	var req setReplicasRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.Replicas < 0 {
		http.Error(w, "replicas must be >= 0", http.StatusBadRequest)
		return
	}

	if err := s.store.SetReplicas(r.Context(), name, req.Replicas); err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "updated"})
}

func (s *Server) routeAPIv1(w http.ResponseWriter, r *http.Request) {
	// Expected:
	// /api/v1/deployments
	// /api/v1/deployments/{name}/replicas
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	if path == "/deployments" || path == "/deployments/" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleListDeployments(w, r)
		return
	}

	// /deployments/{name}/replicas
	const prefix = "/deployments/"
	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "replicas" {
		http.NotFound(w, r)
		return
	}
	name := parts[0]
	if name == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetReplicas(w, r, name)
	case http.MethodPost:
		s.handleSetReplicas(w, r, name)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Make sure Server satisfies "uses Store".
var _ kube.Store = (kube.Store)(nil)