//go:build integration

package department_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/department"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func strptr(s string) *string { return &s }

func deptIDs(rows []sqlc.MasterdataDepartment) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

// TestDepartmentDataScope exercises the scope enforcement that motivated promoting
// departments off the generic (scope-less) reference engine: reads are filtered to
// the caller's office subtree plus shared NULL-office departments, and a scoped
// caller can neither create/edit/delete outside their scope nor touch a global one.
func TestDepartmentDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := department.NewService(sqlc.New(pool))
	ctx := context.Background()

	// seedGlobal creates a NULL-office (shared) department via a global-scope call.
	seedGlobal := func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		d, err := svc.Create(ctx, true, nil, department.CreateInput{Name: name, IsActive: true})
		require.NoError(t, err)
		return d.ID
	}

	t.Run("scoped List returns in-scope + global, excludes other office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		inScope, err := svc.Create(ctx, true, nil, department.CreateInput{Name: "Cabang Dept", Code: strptr("CBG-D"), OfficeID: &tree.Cabang, IsActive: true})
		require.NoError(t, err)
		other, err := svc.Create(ctx, true, nil, department.CreateInput{Name: "W2 Dept", Code: strptr("W2-D"), OfficeID: &tree.Wilayah2, IsActive: true})
		require.NoError(t, err)
		global := seedGlobal(t, "Global Dept")

		rows, total, err := svc.List(ctx, false, []uuid.UUID{tree.Wilayah, tree.Cabang}, "", 100, 0)
		require.NoError(t, err)
		got := deptIDs(rows)
		assert.True(t, got[inScope.ID], "in-subtree department visible")
		assert.True(t, got[global], "global NULL-office department visible to everyone")
		assert.False(t, got[other.ID], "other office's department hidden")
		assert.Equal(t, int64(2), total)
	})

	t.Run("global List returns all", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		_, err := svc.Create(ctx, true, nil, department.CreateInput{Name: "A", OfficeID: &tree.Cabang, IsActive: true})
		require.NoError(t, err)
		_, err = svc.Create(ctx, true, nil, department.CreateInput{Name: "B", OfficeID: &tree.Wilayah2, IsActive: true})
		require.NoError(t, err)
		seedGlobal(t, "C")

		_, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
	})

	t.Run("scoped Create within scope succeeds, out-of-scope + global rejected", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, department.CreateInput{Name: "OK", OfficeID: &tree.Cabang, IsActive: true})
		require.NoError(t, err)

		_, err = svc.Create(ctx, false, ids, department.CreateInput{Name: "Bad", OfficeID: &tree.Wilayah2, IsActive: true})
		assert.ErrorIs(t, err, department.ErrOfficeOutOfScope, "cannot create in another office")

		_, err = svc.Create(ctx, false, ids, department.CreateInput{Name: "Global", IsActive: true})
		assert.ErrorIs(t, err, department.ErrOfficeOutOfScope, "scoped caller cannot create a global (NULL-office) department")
	})

	t.Run("scoped caller cannot edit or delete a global department", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}
		global := seedGlobal(t, "Global Dept")

		// visible on read...
		_, err := svc.Get(ctx, global, false, ids)
		require.NoError(t, err, "global department is readable by a scoped caller")

		// ...but not writable.
		_, _, err = svc.Update(ctx, global, false, ids, department.UpdateInput{CreateInput: department.CreateInput{Name: "Hijack", OfficeID: &tree.Cabang, IsActive: true}})
		assert.ErrorIs(t, err, department.ErrOfficeOutOfScope)

		_, err = svc.Delete(ctx, global, false, ids)
		assert.ErrorIs(t, err, department.ErrOfficeOutOfScope)
	})

	t.Run("scoped Update cannot move a department into another office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}
		d, err := svc.Create(ctx, false, ids, department.CreateInput{Name: "Mine", OfficeID: &tree.Cabang, IsActive: true})
		require.NoError(t, err)

		_, _, err = svc.Update(ctx, d.ID, false, ids, department.UpdateInput{CreateInput: department.CreateInput{Name: "Mine", OfficeID: &tree.Wilayah2, IsActive: true}})
		assert.ErrorIs(t, err, department.ErrOfficeOutOfScope)
	})

	t.Run("scoped Update cannot globalize its own department (office_id -> NULL)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}
		d, err := svc.Create(ctx, false, ids, department.CreateInput{Name: "Mine", OfficeID: &tree.Cabang, IsActive: true})
		require.NoError(t, err)

		// Setting office_id to nil would turn it into a shared global department;
		// the new-office guard must reject that for a scoped caller.
		_, _, err = svc.Update(ctx, d.ID, false, ids, department.UpdateInput{CreateInput: department.CreateInput{Name: "Mine", OfficeID: nil, IsActive: true}})
		assert.ErrorIs(t, err, department.ErrOfficeOutOfScope)
	})

	t.Run("scoped Get / Delete of another office's department is not found / rejected", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		other, err := svc.Create(ctx, true, nil, department.CreateInput{Name: "Other", OfficeID: &tree.Wilayah2, IsActive: true})
		require.NoError(t, err)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err = svc.Get(ctx, other.ID, false, ids)
		assert.True(t, errors.Is(err, common.ErrNotFound), "other office's department is not visible")

		_, err = svc.Delete(ctx, other.ID, false, ids)
		assert.True(t, errors.Is(err, common.ErrNotFound) || errors.Is(err, department.ErrOfficeOutOfScope))
	})
}
