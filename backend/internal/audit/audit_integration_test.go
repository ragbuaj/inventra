//go:build integration

package audit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestAuditService(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := audit.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("List is office-scoped", func(t *testing.T) {
		testsupport.Reset(t, pool)
		officeA, officeB := uuid.New(), uuid.New()
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &officeA}))
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionUpdate, OfficeID: &officeA}))
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &officeB}))

		rows, total, err := svc.List(ctx, audit.ListFilter{AllScope: false, OfficeIDs: []uuid.UUID{officeA}, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, rows, 2)
		for _, r := range rows {
			require.NotNil(t, r.OfficeID)
			assert.Equal(t, officeA, *r.OfficeID)
		}

		all, totalAll, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(3), totalAll)
		assert.Len(t, all, 3)
	})

	t.Run("filter by action and entity_type", func(t *testing.T) {
		testsupport.Reset(t, pool)
		office := uuid.New()
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &office}))
		require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "employee", EntityID: uuid.New(), Action: audit.ActionUpdate, OfficeID: &office}))

		actionCreate := audit.ActionCreate
		byAction, totalA, err := svc.List(ctx, audit.ListFilter{AllScope: true, Action: &actionCreate, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalA)
		require.Len(t, byAction, 1)
		assert.Equal(t, audit.ActionCreate, byAction[0].Action)
		assert.Equal(t, "office", byAction[0].EntityType)

		entity := "employee"
		byType, totalT, err := svc.List(ctx, audit.ListFilter{AllScope: true, EntityType: &entity, Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, int64(1), totalT)
		require.Len(t, byType, 1)
		assert.Equal(t, "employee", byType[0].EntityType)
	})

	t.Run("List is newest-first", func(t *testing.T) {
		testsupport.Reset(t, pool)
		office := uuid.New()
		for i := 0; i < 3; i++ {
			require.NoError(t, svc.Log(ctx, audit.LogInput{EntityType: "office", EntityID: uuid.New(), Action: audit.ActionCreate, OfficeID: &office}))
		}
		rows, _, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
		require.NoError(t, err)
		require.Len(t, rows, 3)
		for i := 0; i < len(rows)-1; i++ {
			assert.False(t, rows[i].CreatedAt.Time.Before(rows[i+1].CreatedAt.Time), "rows must be ordered newest-first")
		}
	})

	t.Run("Diff round-trips through Changes JSON", func(t *testing.T) {
		testsupport.Reset(t, pool)
		office := uuid.New()

		before := map[string]any{"name": "A", "code": "X"}
		after := map[string]any{"name": "B", "code": "X"}
		diff := audit.Diff(before, after)
		// Minimal direct sanity (full Diff logic is unit-tested elsewhere): only the
		// changed field is present, with before/after recorded.
		require.Len(t, diff, 1)
		assert.Equal(t, map[string]any{"before": "A", "after": "B"}, diff["name"])

		require.NoError(t, svc.Log(ctx, audit.LogInput{
			EntityType: "office", EntityID: uuid.New(), Action: audit.ActionUpdate,
			Changes: diff, OfficeID: &office,
		}))

		rows, _, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
		require.NoError(t, err)
		require.Len(t, rows, 1)

		var got map[string]map[string]any
		require.NoError(t, json.Unmarshal(rows[0].Changes, &got))
		assert.Equal(t, diff, got)
	})
}
