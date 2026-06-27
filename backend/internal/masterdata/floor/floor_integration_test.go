//go:build integration

package floor_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/floor"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func floorIDs(rows []sqlc.MasterdataFloor) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestFloorDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := floor.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("scoped List returns the office's floors; out-of-scope office rejected", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW := testsupport.SeedFloor(t, pool, tree.Wilayah, "Lantai 1")
		testsupport.SeedFloor(t, pool, tree.Wilayah2, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, tree.Wilayah, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.True(t, floorIDs(rows)[fW])

		_, _, err = svc.List(ctx, false, ids, tree.Wilayah2, "", 100, 0)
		assert.ErrorIs(t, err, floor.ErrOfficeOutOfScope)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW := testsupport.SeedFloor(t, pool, tree.Wilayah, "Lantai 1")
		fW2 := testsupport.SeedFloor(t, pool, tree.Wilayah2, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, fW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, fW, false, ids)
		require.NoError(t, err)
		assert.Equal(t, fW, got.ID)
	})

	t.Run("Create rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, floor.CreateInput{OfficeID: tree.Pusat, Name: "Bad"})
		assert.ErrorIs(t, err, floor.ErrOfficeOutOfScope)

		created, err := svc.Create(ctx, false, ids, floor.CreateInput{OfficeID: tree.Wilayah, Name: "Lantai OK"})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, created.OfficeID)
	})

	t.Run("Update rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW := testsupport.SeedFloor(t, pool, tree.Wilayah, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, fW, false, ids, floor.UpdateInput{
			CreateInput: floor.CreateInput{OfficeID: tree.Pusat, Name: "Lantai 1"},
		})
		assert.ErrorIs(t, err, floor.ErrOfficeOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fW2 := testsupport.SeedFloor(t, pool, tree.Wilayah2, "Lantai 1")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, fW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted (office,name) can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, floor.CreateInput{OfficeID: tree.Wilayah, Name: "Reuse"})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, floor.CreateInput{OfficeID: tree.Wilayah, Name: "Reuse"})
		assert.NoError(t, err, "(office_id, name) reusable after soft delete")
	})
}
