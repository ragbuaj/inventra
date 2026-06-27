//go:build integration

package testsupport

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Reset truncates every base table in the application schemas, restoring the
// database to its post-migration empty state. Use it between tests that share a
// container so each starts from a clean slate.
func Reset(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT format('%I.%I', schemaname, tablename)
		FROM pg_tables
		WHERE schemaname IN ('identity', 'masterdata', 'audit')`)
	require.NoError(t, err)

	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())
	if len(tables) == 0 {
		return
	}

	_, err = pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE")
	require.NoError(t, err)
}
