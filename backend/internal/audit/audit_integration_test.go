//go:build integration

package audit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ─── seeding helpers (modeled on internal/search/search_integration_test.go) ──

// seedRole inserts an identity.roles row and returns its id.
func seedRole(t *testing.T, pool *pgxpool.Pool, code, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.roles (code, name) VALUES ($1, $2) RETURNING id`,
		code, name).Scan(&id))
	return id
}

// seedAuditOffice inserts a fresh office_type + one office (distinct name/code)
// and returns the office ID.
func seedAuditOffice(t *testing.T, pool *pgxpool.Pool, typeCode, name, code string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		typeCode).Scan(&typeID))

	var officeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, name, code).Scan(&officeID))

	return officeID
}

// seedAuditUser inserts an identity.users row with the given role/office and
// returns its id.
func seedAuditUser(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, name, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		name, email, roleID, officeID).Scan(&id))
	return id
}

// TestListAudit_IncludesRoleAndOfficeName exercises the ListAuditLogs JOINs
// against identity.roles (the actor's CURRENT role, not snapshotted — an
// accepted limitation) and masterdata.offices (the audit row's office_id).
func TestListAudit_IncludesRoleAndOfficeName(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := audit.NewService(sqlc.New(pool))
	ctx := context.Background()

	roleID := seedRole(t, pool, "auditor-role", "auditor-role")
	officeID := seedAuditOffice(t, pool, "KP", "KP Test", "KP-TEST")
	userID := seedAuditUser(t, pool, roleID, officeID, "Auditor Satu", "auditor.satu@test.local")

	require.NoError(t, svc.Log(ctx, audit.LogInput{
		ActorID: &userID, EntityType: "office", EntityID: uuid.New(),
		Action: audit.ActionCreate, OfficeID: &officeID,
	}))

	rows, total, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)

	require.NotNil(t, rows[0].ActorRole)
	assert.Equal(t, "auditor-role", *rows[0].ActorRole)
	require.NotNil(t, rows[0].OfficeName)
	assert.Equal(t, "KP Test", *rows[0].OfficeName)
}

// TestListAudit_OfficeNameNullWhenOfficeMissing covers the NULL-safety
// requirement: office_id has no FK and is nullable, so a row whose office_id
// points at a non-existent (or soft-deleted) office must still list with
// office_name = null rather than erroring or dropping the row.
func TestListAudit_OfficeNameNullWhenOfficeMissing(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := audit.NewService(sqlc.New(pool))
	ctx := context.Background()

	orphanOffice := uuid.New() // deliberately not present in masterdata.offices
	require.NoError(t, svc.Log(ctx, audit.LogInput{
		EntityType: "orphan", EntityID: uuid.New(),
		Action: audit.ActionCreate, OfficeID: &orphanOffice,
	}))

	rows, _, err := svc.List(ctx, audit.ListFilter{AllScope: true, Limit: 100})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].OfficeName)
	assert.Nil(t, rows[0].ActorRole) // no actor at all on this row
}
