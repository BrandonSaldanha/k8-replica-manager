package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/api"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("load config: %v", err)
		return 1
	}

	km, err := kube.NewManager(cfg.Namespace)
	if err != nil {
		log.Printf("failed to init kubernetes manager: %v", err)
		return 1
	}
	defer km.Shutdown()

	s := api.New(cfg, km)

	// Run server in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Wait for signal or server exit.
	select {
	case sig := <-sigCh:
		log.Printf("received signal %s, shutting down", sig)

	case err := <-errCh:
		// Start() returned before we even got a signal.
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped")
			return 0
		}
		log.Printf("server error: %v", err)
		return 1
	}

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
		return 1
	}

	// Optional: wait for Start() goroutine to exit so logs look clean.
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error during shutdown: %v", err)
			return 1
		}
	case <-time.After(2 * time.Second):
		// Not fatal; shutdown already requested.
	}

	return 0
}
