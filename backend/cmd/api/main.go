// Command api is the entry point for the Inventra backend service.
//
// Inventra follows a modular monolith with clean architecture (see docs/PRD.md §7).
// This entry point wires configuration, infrastructure (PostgreSQL, Redis), and the
// HTTP server; feature modules are registered through the router as they are implemented.
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

	"github.com/ragbuaj/inventra/internal/cache"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
	"github.com/ragbuaj/inventra/internal/server"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	// PostgreSQL (authoritative store).
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()
	if err := db.Ping(ctx, pool); err != nil {
		log.Printf("WARNING: PostgreSQL not reachable at startup: %v", err)
	} else {
		log.Println("PostgreSQL connected")
	}

	// Redis (cache/state).
	rdb := cache.NewClient(cfg)
	defer func() { _ = rdb.Close() }()
	if err := cache.Ping(ctx, rdb); err != nil {
		log.Printf("WARNING: Redis not reachable at startup: %v", err)
	} else {
		log.Println("Redis connected")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           server.NewRouter(server.Deps{Cfg: cfg, Pool: pool, Redis: rdb}),
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}
