//go:build integration

package employee_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/employee"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func strptr(s string) *string { return &s }

func empIDs(rows []sqlc.MasterdataEmployee) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestEmployeeDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := employee.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("scoped List returns only in-scope employees", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eW := testsupport.SeedEmployee(t, pool, tree.Wilayah, "E-W")
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		got := empIDs(rows)
		assert.True(t, got[eW] && got[eC])
		assert.False(t, got[eW2])
	})

	t.Run("global List returns all employees", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		testsupport.SeedEmployee(t, pool, tree.Wilayah, "E-W")
		testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")

		rows, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, rows, 3)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, eW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, eC, false, ids)
		require.NoError(t, err)
		assert.Equal(t, eC, got.ID)
	})

	t.Run("Create rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, employee.CreateInput{
			Code: "E-BAD", Name: "Bad", OfficeID: tree.Pusat, Status: sqlc.SharedUserStatusActive,
		})
		assert.ErrorIs(t, err, employee.ErrOfficeOutOfScope)

		created, err := svc.Create(ctx, false, ids, employee.CreateInput{
			Code: "E-OK", Name: "Ok", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, created.OfficeID)
	})

	t.Run("Update rejects move to out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, eC, false, ids, employee.UpdateInput{
			CreateInput: employee.CreateInput{
				Code: "E-C", Name: "Moved", OfficeID: tree.Pusat, Status: sqlc.SharedUserStatusActive,
			},
		})
		assert.ErrorIs(t, err, employee.ErrOfficeOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, eW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted code can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, employee.CreateInput{
			Code: "E-REUSE", Name: "First", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, employee.CreateInput{
			Code: "E-REUSE", Name: "Second", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		assert.NoError(t, err, "code reusable after soft delete")
	})
}

func TestEmployeePhone(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := employee.NewService(q)
	ctx := context.Background()

	testsupport.Reset(t, pool)
	tree := testsupport.SeedOfficeTree(t, pool)

	created, err := svc.Create(ctx, true, nil, employee.CreateInput{
		Code: "EP-1", Name: "Phone Emp", OfficeID: tree.Cabang,
		Status: sqlc.SharedUserStatus("active"), Phone: strptr("0812-1111"),
	})
	require.NoError(t, err)
	require.NotNil(t, created.Phone)
	assert.Equal(t, "0812-1111", *created.Phone)

	_, after, err := svc.Update(ctx, created.ID, true, nil, employee.UpdateInput{CreateInput: employee.CreateInput{
		Code: "EP-1", Name: "Phone Emp", OfficeID: tree.Cabang,
		Status: sqlc.SharedUserStatus("active"), Phone: strptr("0813-2222"),
	}})
	require.NoError(t, err)
	require.NotNil(t, after.Phone)
	assert.Equal(t, "0813-2222", *after.Phone)

	created2, err := svc.Create(ctx, true, nil, employee.CreateInput{
		Code: "EP-2", Name: "No Phone", OfficeID: tree.Cabang, Status: sqlc.SharedUserStatus("active"),
	})
	require.NoError(t, err)
	assert.Nil(t, created2.Phone)
}
