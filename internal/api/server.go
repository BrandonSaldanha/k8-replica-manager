package api

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

// Server wraps an HTTP server and exposes lifecycle helpers for starting and shutting down.
type Server struct {
	cfg config.Config
	srv *http.Server
	store kube.Store
}

// New constructs a Server with routes registered.
func New(cfg config.Config, store kube.Store) *Server {
	mux := http.NewServeMux()

	s := &Server{
		cfg:   cfg,
		store: store,
	}

	// Health endpoints.
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)

	// Core API routes.
	mux.HandleFunc("/api/v1/", s.routeAPIv1)

	s.srv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

// Start begins serving HTTP requests and blocks until the server stops.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}

	log.Printf("listening on %s (tls=%v namespace=%s)", s.cfg.ListenAddr, s.cfg.TLSEnabled, s.cfg.Namespace)

	// TLS/mTLS to be implemented later. For now always serve HTTP.
	return s.srv.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
