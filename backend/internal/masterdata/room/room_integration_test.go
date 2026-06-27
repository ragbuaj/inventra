//go:build integration

package room_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/room"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func roomIDs(rows []sqlc.MasterdataRoom) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestRoomDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := room.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("List via in-scope floor returns rooms; out-of-scope floor rejected", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		rIn := testsupport.SeedRoom(t, pool, fIn, "R-in")
		testsupport.SeedRoom(t, pool, fOut, "R-out")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, fIn, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.True(t, roomIDs(rows)[rIn])

		_, _, err = svc.List(ctx, false, ids, fOut, "", 100, 0)
		assert.ErrorIs(t, err, room.ErrFloorOutOfScope)
	})

	t.Run("Get/Delete a room on an out-of-scope floor is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		rIn := testsupport.SeedRoom(t, pool, fIn, "R-in")
		rOut := testsupport.SeedRoom(t, pool, fOut, "R-out")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, rOut, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, rIn, false, ids)
		require.NoError(t, err)
		assert.Equal(t, rIn, got.ID)

		_, err = svc.Delete(ctx, rOut, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("Create rejects out-of-scope floor", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, room.CreateInput{FloorID: fOut, Name: "Bad"})
		assert.ErrorIs(t, err, room.ErrFloorOutOfScope)

		created, err := svc.Create(ctx, false, ids, room.CreateInput{FloorID: fIn, Name: "R-OK"})
		require.NoError(t, err)
		assert.Equal(t, fIn, created.FloorID)
	})

	t.Run("Update rejects move to out-of-scope floor", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")
		fOut := testsupport.SeedFloor(t, pool, tree.Wilayah2, "F-out")
		rIn := testsupport.SeedRoom(t, pool, fIn, "R-in")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, rIn, false, ids, room.UpdateInput{
			CreateInput: room.CreateInput{FloorID: fOut, Name: "R-in"},
		})
		assert.ErrorIs(t, err, room.ErrFloorOutOfScope)
	})

	t.Run("soft-deleted (floor,name) can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		fIn := testsupport.SeedFloor(t, pool, tree.Wilayah, "F-in")

		first, err := svc.Create(ctx, true, nil, room.CreateInput{FloorID: fIn, Name: "Reuse"})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, room.CreateInput{FloorID: fIn, Name: "Reuse"})
		assert.NoError(t, err, "(floor_id, name) reusable after soft delete")
	})
}
