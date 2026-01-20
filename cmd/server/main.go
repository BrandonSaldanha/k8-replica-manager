package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/api"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

func main() {
	cfg := config.Load()

	km, err := kube.NewManager(cfg.Namespace)
	if err != nil {
		log.Fatalf("failed to init kubernetes manager: %v", err)
	}

	s := api.New(cfg, km)

	// Run server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var exitCode int
	select {
	case sig := <-sigCh:
		log.Printf("received signal %s, shutting down", sig)
	case err := <-errCh:
		// If Start() returns, server stopped. Treat non-nil as fatal.
		if err != nil {
			log.Printf("server error: %v", err)
			exitCode = 1
		} else {
			log.Printf("server stopped")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop accepting new connections; finish in-flight requests.
	if err := s.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
		exitCode = 1
	}

	// Stop informers / background k8s watchers.
	km.Shutdown()

	os.Exit(exitCode)
}
