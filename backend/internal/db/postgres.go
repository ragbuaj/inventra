// Package db provides the PostgreSQL connection pool (pgx).
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ragbuaj/inventra/internal/config"
)

// searchPath lists module schemas so unqualified identifiers resolve; sqlc
// queries are schema-qualified, but this keeps ad-hoc/migrate access convenient.
const searchPath = "identity,masterdata,asset,assignment,maintenance,depreciation,approval,import,audit,shared,public"

// NewPool builds a pgx connection pool from configuration. It does not connect
// eagerly (pgx connects lazily); use Ping to verify connectivity.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolCfg.ConnConfig.RuntimeParams["search_path"] = searchPath

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}
	return pool, nil
}

// Ping verifies the database is reachable within the given timeout.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return pool.Ping(ctx)
}
