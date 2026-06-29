//go:build integration

package category_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/category"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func mkInput(name string) category.CreateInput {
	return category.CreateInput{Name: name, AssetClass: sqlc.SharedAssetClass("tangible"), IsActive: true}
}

func TestCategoryTree(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := category.NewService(q)
	ctx := context.Background()

	t.Run("returns non-deleted incl. parent/child, excludes soft-deleted", func(t *testing.T) {
		testsupport.Reset(t, pool)

		parent, err := svc.Create(ctx, mkInput("Perangkat IT"))
		require.NoError(t, err)
		childIn := mkInput("Laptop")
		childIn.ParentID = &parent.ID
		child, err := svc.Create(ctx, childIn)
		require.NoError(t, err)
		gone, err := svc.Create(ctx, mkInput("Dihapus"))
		require.NoError(t, err)
		_, err = svc.Delete(ctx, gone.ID)
		require.NoError(t, err)

		rows, err := svc.Tree(ctx)
		require.NoError(t, err)
		ids := map[string]bool{}
		var childRow sqlc.MasterdataCategory
		for _, r := range rows {
			ids[r.ID.String()] = true
			if r.ID == child.ID {
				childRow = r
			}
		}
		assert.True(t, ids[parent.ID.String()], "parent present")
		assert.True(t, ids[child.ID.String()], "child present")
		assert.False(t, ids[gone.ID.String()], "soft-deleted excluded")
		require.NotNil(t, childRow.ParentID)
		assert.Equal(t, parent.ID, *childRow.ParentID, "child parent_id passthrough")
	})

	t.Run("no pagination cap — returns more than the list's 100-row limit", func(t *testing.T) {
		testsupport.Reset(t, pool)
		for i := 0; i < 101; i++ {
			_, err := svc.Create(ctx, mkInput(fmt.Sprintf("Kategori %03d", i)))
			require.NoError(t, err)
		}
		rows, err := svc.Tree(ctx)
		require.NoError(t, err)
		assert.Equal(t, 101, len(rows), "tree returns all rows, not capped at 100")
	})
}
