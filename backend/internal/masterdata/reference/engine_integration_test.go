//go:build integration

package reference

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func officeTypesResource(t *testing.T) resource {
	t.Helper()
	for _, r := range referenceResources {
		if r.Path == "office-types" {
			return r
		}
	}
	t.Fatal("office-types resource not registered")
	return resource{}
}

func TestReferenceEngine(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	e := engine{pool: pool}
	ctx := context.Background()
	res := officeTypesResource(t)
	require.Equal(t, "office_types", res.Table)

	t.Run("write/get/list round trip", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)
		assert.Equal(t, "Tipe A", created["name"])
		id := uuid.MustParse(created["id"].(string))

		got, err := e.get(ctx, res, id)
		require.NoError(t, err)
		assert.Equal(t, "Tipe A", got["name"])

		rows, total, err := e.list(ctx, res, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, rows, 1)
	})

	t.Run("search filters by name", func(t *testing.T) {
		testsupport.Reset(t, pool)
		_, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)

		hit, totalHit, err := e.list(ctx, res, "Tipe A", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalHit)
		assert.Len(t, hit, 1)

		miss, totalMiss, err := e.list(ctx, res, "nope", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), totalMiss)
		assert.Empty(t, miss)
	})

	t.Run("update via write", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)
		id := uuid.MustParse(created["id"].(string))

		updated, err := e.write(ctx, res, &id, map[string]any{"name": "Tipe B", "is_active": false})
		require.NoError(t, err)
		assert.Equal(t, "Tipe B", updated["name"])
		assert.Equal(t, false, updated["is_active"])
	})

	t.Run("soft delete hides the row", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "Tipe A", "is_active": true})
		require.NoError(t, err)
		id := uuid.MustParse(created["id"].(string))

		ok, err := e.del(ctx, res, id)
		require.NoError(t, err)
		assert.True(t, ok)

		_, err = e.get(ctx, res, id)
		assert.ErrorIs(t, err, common.ErrNotFound)

		rows, total, err := e.list(ctx, res, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, rows)
	})

	t.Run("coerce rejects a missing required field", func(t *testing.T) {
		testsupport.Reset(t, pool)
		_, err := e.write(ctx, res, nil, map[string]any{"is_active": true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("soft-deleted name can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		created, err := e.write(ctx, res, nil, map[string]any{"name": "X", "is_active": true})
		require.NoError(t, err)
		id := uuid.MustParse(created["id"].(string))

		_, err = e.del(ctx, res, id)
		require.NoError(t, err)

		_, err = e.write(ctx, res, nil, map[string]any{"name": "X", "is_active": true})
		assert.NoError(t, err, "name reusable after soft delete")
	})
}
