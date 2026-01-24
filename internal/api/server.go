package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

// Server wraps an HTTP server and exposes lifecycle helpers for starting and shutting down.
type Server struct {
	cfg      config.Config
	apiSrv   *http.Server
	probeSrv *http.Server
	store    kube.Store
}

// New constructs a Server with routes registered.
func New(cfg config.Config, store kube.Store) *Server {
	s := &Server{
		cfg:   cfg,
		store: store,
	}

	// API mux: only API routes (will be HTTPS+mTLS when enabled)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/", s.routeAPIv1)

	s.apiSrv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           apiMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Probe mux: always unauthenticated
	probeMux := http.NewServeMux()
	probeMux.HandleFunc("/healthz", s.handleHealthz)
	probeMux.HandleFunc("/readyz", s.handleReadyz)

	s.probeSrv = &http.Server{
		Addr:              cfg.ProbeListenAddr,
		Handler:           probeMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return s
}

func (s *Server) buildTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server cert/key: %w", err)
	}

	caPEM, err := os.ReadFile(s.cfg.TLSClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read client CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse client CA: no certs found")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}

// Start begins serving HTTP requests and blocks until the server stops.
func (s *Server) Start() error {
	// Start probe server first.
	probeLn, err := net.Listen("tcp", s.cfg.ProbeListenAddr)
	if err != nil {
		return err
	}
	go func() {
		log.Printf("probe listening on %s", s.cfg.ProbeListenAddr)
		if err := s.probeSrv.Serve(probeLn); err != nil && err != http.ErrServerClosed {
			log.Printf("probe server error: %v", err)
		}
	}()

	// Start API server.
	apiLn, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}

	log.Printf("api listening on %s (tls=%v namespace=%s)", s.cfg.ListenAddr, s.cfg.TLSEnabled, s.cfg.Namespace)

	if !s.cfg.TLSEnabled {
		return s.apiSrv.Serve(apiLn)
	}

	tlsCfg, err := s.buildTLSConfig()
	if err != nil {
		return err
	}
	s.apiSrv.TLSConfig = tlsCfg

	// Certificates are in TLSConfig, so pass empty filenames.
	return s.apiSrv.ServeTLS(apiLn, "", "")
}

// Shutdown gracefully stops both servers.
func (s *Server) Shutdown(ctx context.Context) error {
	err1 := s.apiSrv.Shutdown(ctx)
	err2 := s.probeSrv.Shutdown(ctx)
	if err1 != nil {
		return err1
	}
	return err2
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Can't reliably change the status code here since headers may already be written.
		log.Printf("write json response: %v", err)
	}
}
