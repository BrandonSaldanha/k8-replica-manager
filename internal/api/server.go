package api

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
)

type Server struct {
	cfg config.Config
	srv *http.Server
}

func New(cfg config.Config) *Server {
	mux := http.NewServeMux()

	// PR1: only /healthz. API routes land in later PRs.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
		})
	})

	s := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{
		cfg: cfg,
		srv: s,
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}

	log.Printf("listening on %s (tls=%v namespace=%s)", s.cfg.ListenAddr, s.cfg.TLSEnabled, s.cfg.Namespace)

	// TLS/mTLS to be implemented later. For now always serve HTTP.
	return s.srv.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
