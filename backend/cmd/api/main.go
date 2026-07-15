// Command api is the entry point for the Inventra backend service.
//
// Inventra follows a modular monolith with clean architecture (see docs/PRD.md §7).
// This entry point wires configuration, infrastructure (PostgreSQL, Redis), and the
// HTTP server; feature modules are registered through the router as they are implemented.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ragbuaj/inventra/internal/cache"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
	"github.com/ragbuaj/inventra/internal/geoip"
	"github.com/ragbuaj/inventra/internal/logging"
	"github.com/ragbuaj/inventra/internal/ratelimit"
	"github.com/ragbuaj/inventra/internal/server"
	"github.com/ragbuaj/inventra/internal/storage"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg)
	slog.SetDefault(logger)
	ctx := context.Background()

	// Cancellable root context for background work (the import worker) tied to
	// graceful shutdown below — cancelled once the SIGINT/SIGTERM handler
	// starts shutting the HTTP server down.
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()

	// PostgreSQL (authoritative store).
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		slog.Error("db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Ping(ctx, pool); err != nil {
		slog.Warn("PostgreSQL not reachable at startup", "error", err)
	} else {
		slog.Info("PostgreSQL connected")
	}

	// Redis (cache/state).
	rdb := cache.NewClient(cfg)
	defer func() { _ = rdb.Close() }()
	if err := cache.Ping(ctx, rdb); err != nil {
		slog.Warn("Redis not reachable at startup", "error", err)
	} else {
		slog.Info("Redis connected")
	}

	limiter := ratelimit.New(rdb, cfg)

	// MinIO (object storage for asset attachments).
	store, err := storage.NewMinIOStorage(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		slog.Error("minio init failed", "error", err)
		os.Exit(1)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		slog.Error("minio bucket ensure failed", "error", err)
		os.Exit(1)
	}
	slog.Info("MinIO connected", "bucket", cfg.MinIOBucket)

	// GeoIP locator for device-session locations (no-op without a provisioned DB).
	geoLocator := geoip.New(cfg.GeoIPDBPath, logger)
	defer func() { _ = geoLocator.Close() }()

	handler, importWorker := server.NewRouter(server.Deps{Cfg: cfg, Pool: pool, Redis: rdb, Log: logger, Limiter: limiter, Storage: store, GeoIP: geoLocator})
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Inventra API listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	if cfg.ImportWorkerEnabled {
		if err := importWorker.Recover(workerCtx); err != nil {
			slog.Error("import worker recover failed", "error", err)
		}
		go importWorker.Run(workerCtx)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	stopWorker()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
