package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BrandonSaldanha/k8s-replica-manager/internal/api"
	"github.com/BrandonSaldanha/k8s-replica-manager/internal/config"
)

func main() {
	cfg := config.Load()

	s := api.New(cfg)

	// Run server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received signal %s, shutting down", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
