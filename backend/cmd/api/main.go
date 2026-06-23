// Command api is the entry point for the Inventra backend service.
//
// Inventra follows a modular monolith with clean architecture (see docs/PRD.md §7).
// This entry point only wires configuration and the HTTP server; feature modules
// are registered through the router as they are implemented.
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

	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/server"
)

func main() {
	cfg := config.Load()

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           server.NewRouter(cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Inventra API listening on %s (env=%s)", srv.Addr, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}
