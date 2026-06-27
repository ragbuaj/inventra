//go:build integration

package testsupport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestResetTruncatesAppTables(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO masterdata.office_types (name) VALUES ('Kantor')`)
	require.NoError(t, err)

	testsupport.Reset(t, pool)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM masterdata.office_types`).Scan(&count))
	assert.Equal(t, 0, count, "Reset should truncate masterdata tables")
}
