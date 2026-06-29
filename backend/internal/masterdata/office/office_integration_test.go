//go:build integration

package office_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/office"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func rowIDs(rows []sqlc.MasterdataOffice) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func f64(v float64) *float64 { return &v }

func TestOfficeDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("GetOfficeSubtree returns self + descendants only", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		sub, err := q.GetOfficeSubtree(ctx, tree.Wilayah)
		require.NoError(t, err)
		got := idSet(sub)
		assert.Len(t, sub, 2)
		assert.True(t, got[tree.Wilayah], "subtree includes itself")
		assert.True(t, got[tree.Cabang], "subtree includes descendant")
		assert.False(t, got[tree.Pusat], "subtree excludes ancestor")
		assert.False(t, got[tree.Wilayah2], "subtree excludes sibling")

		full, err := q.GetOfficeSubtree(ctx, tree.Pusat)
		require.NoError(t, err)
		assert.Len(t, full, 5, "root subtree spans the whole tree")
	})

	t.Run("scoped List returns only in-scope offices", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		got := rowIDs(rows)
		assert.True(t, got[tree.Wilayah] && got[tree.Cabang])
		assert.False(t, got[tree.Pusat] || got[tree.Wilayah2] || got[tree.Cabang2])
	})

	t.Run("global List returns all offices", func(t *testing.T) {
		testsupport.Reset(t, pool)
		testsupport.SeedOfficeTree(t, pool)

		rows, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, rows, 5)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, tree.Pusat, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, tree.Cabang, false, ids)
		require.NoError(t, err)
		assert.Equal(t, tree.Cabang, got.ID)
	})

	t.Run("Create rejects out-of-scope parent", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Bad", Code: "BAD", IsActive: true,
		})
		assert.ErrorIs(t, err, office.ErrParentOutOfScope)

		created, err := svc.Create(ctx, false, ids, office.CreateInput{
			ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
			Name: "Outlet", Code: "O1", IsActive: true,
		})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, *created.ParentID)
	})

	t.Run("Update rejects reparent outside scope", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, tree.Cabang, false, ids, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1", Code: "C1", IsActive: true,
			},
		})
		assert.ErrorIs(t, err, office.ErrReparentOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, tree.Wilayah2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted code can be reused (partial-unique)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Reuse", Code: "REUSE", IsActive: true,
		})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Reuse Again", Code: "REUSE", IsActive: true,
		})
		assert.NoError(t, err, "code reusable after soft delete")
	})

	t.Run("update advances updated_at (set_updated_at trigger)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		before, after, err := svc.Update(ctx, tree.Cabang, true, nil, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1 Renamed", Code: "C1", IsActive: true,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Cabang 1 Renamed", after.Name)
		assert.False(t, after.UpdatedAt.Time.Before(before.UpdatedAt.Time), "updated_at must not regress")
	})
}

func TestOfficeCoordinates(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("create stores and returns coordinates", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		created, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Coord Office", Code: "COORD", IsActive: true,
			Latitude: f64(-6.1754), Longitude: f64(106.8272),
		})
		require.NoError(t, err)
		require.NotNil(t, created.Latitude)
		require.NotNil(t, created.Longitude)
		assert.InDelta(t, -6.1754, *created.Latitude, 1e-9)
		assert.InDelta(t, 106.8272, *created.Longitude, 1e-9)
	})

	t.Run("update changes coordinates", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		_, after, err := svc.Update(ctx, tree.Cabang, true, nil, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1", Code: "C1", IsActive: true,
				Latitude: f64(-6.29), Longitude: f64(106.80),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, after.Latitude)
		assert.InDelta(t, -6.29, *after.Latitude, 1e-9)
	})
}
