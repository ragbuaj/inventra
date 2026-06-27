//go:build integration

package testsupport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestNewPostgresAppliesMigrations(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	ctx := context.Background()

	// A table from a late migration proves the full migration set ran.
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'masterdata' AND table_name = 'offices'
		)`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "masterdata.offices should exist after migrations")
}
