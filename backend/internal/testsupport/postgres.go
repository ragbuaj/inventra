//go:build integration

// Package testsupport provides container-backed fixtures for integration tests
// (build tag `integration`). It boots throwaway Postgres/Redis, applies the
// production migrations, seeds data, and resets between tests (ADR-0001).
package testsupport

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// NewPostgres starts a throwaway postgres:16-alpine, applies every migration in
// backend/db/migrations, and returns a connected pool. The container and pool are
// terminated via t.Cleanup.
func NewPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("inventra_test"),
		tcpostgres.WithUsername("inventra"),
		tcpostgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	applyMigrations(t, dsn)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func applyMigrations(t *testing.T, dsn string) {
	t.Helper()

	// Locate backend/db/migrations relative to THIS source file, so the CWD does
	// not matter. This file lives at backend/internal/testsupport/postgres.go.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations")

	src, err := iofs.New(os.DirFS(migrationsDir), ".")
	require.NoError(t, err)

	// golang-migrate's pgx/v5 driver registers the "pgx5" scheme.
	dbURL := strings.Replace(dsn, "postgres://", "pgx5://", 1)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Up())
}
