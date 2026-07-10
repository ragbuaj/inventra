//go:build integration

package maintenance_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/maintenance"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// seedOfficeWithType inserts a single office_type + one office and returns the
// office ID.
func seedOfficeWithType(t *testing.T, pool *pgxpool.Pool, typeCode, officeCode string) uuid.UUID {
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
		typeID, officeCode, officeCode).Scan(&officeID))

	return officeID
}

// seedCategory inserts a masterdata.categories row (intangible) and returns its id.
func seedCategory(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.categories (name, code, asset_class)
		 VALUES ($1, $2, 'intangible') RETURNING id`,
		code, code).Scan(&id))
	return id
}

// seedProblemCategory inserts a masterdata.problem_categories row and returns its id.
func seedProblemCategory(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.problem_categories (name) VALUES ($1) RETURNING id`,
		name).Scan(&id))
	return id
}

// seedAsset inserts an asset.assets row with the given status and returns its id.
// Uses asset_class=intangible to avoid the room FK constraint.
func seedAsset(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, status sqlc.SharedAssetStatus) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', $5)
		 RETURNING id`,
		tag, name, categoryID, officeID, string(status)).Scan(&id))
	return id
}

// seedUser inserts an identity.users row (placed in officeID) and returns its id.
func seedUser(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		email, email, roleID, officeID).Scan(&id))
	return id
}

// lookupRole queries identity.roles by name and returns its id.
func lookupRole(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM identity.roles WHERE name = $1 AND deleted_at IS NULL LIMIT 1`,
		name).Scan(&id))
	return id
}

// buildCaller returns an approval.Caller with the given parameters.
func buildCaller(userID, roleID uuid.UUID, allScope bool, officeIDs []uuid.UUID) approval.Caller {
	return approval.Caller{UserID: userID, RoleID: roleID, AllScope: allScope, OfficeIDs: officeIDs}
}

func strptr(s string) *string                                            { return &s }
func int32ptr(v int32) *int32                                            { return &v }
func msPtr(s sqlc.SharedMaintenanceStatus) *sqlc.SharedMaintenanceStatus { return &s }

// harness bundles everything a maintenance test needs: pool, sqlc queries, the
// approval + maintenance services (with the maintenance executor registered),
// an office O plus an unrelated sibling office, the Manager/Staf roles, a
// shared asset category and a shared problem category.
type harness struct {
	pool      *pgxpool.Pool
	q         *sqlc.Queries
	apprSvc   *approval.Service
	msvc      *maintenance.Service
	office    uuid.UUID
	sibling   uuid.UUID
	managerRl uuid.UUID
	stafRl    uuid.UUID
	catID     uuid.UUID
	problemID uuid.UUID
}

// newHarness boots a throwaway Postgres + Redis, and wires the approval +
// maintenance services with the maintenance executor registered. Seeds office
// O and an unrelated sibling office (for out-of-scope assertions). The asset
// service dependency is left nil — no test here exercises the optional photo
// upload path of SubmitReport, so it is never dereferenced.
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	office := seedOfficeWithType(t, pool, "MaintType-"+uuid.New().String()[:8], "OFC"+uuid.New().String()[:4])
	sibling := seedOfficeWithType(t, pool, "MaintSibling-"+uuid.New().String()[:8], "SIB"+uuid.New().String()[:4])
	catID := seedCategory(t, pool, "MNT"+uuid.New().String()[:4])
	problemID := seedProblemCategory(t, pool, "Kerusakan-"+uuid.New().String()[:8])

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	apprSvc := approval.NewService(q, pool, scopeSvc, rdb)
	msvc := maintenance.NewService(q, pool, apprSvc, nil)
	apprSvc.RegisterExecutor(sqlc.SharedRequestTypeMaintenance, msvc.Executor())

	managerRl := lookupRole(t, pool, "Manager")
	stafRl := lookupRole(t, pool, "Staf")

	return &harness{
		pool:      pool,
		q:         q,
		apprSvc:   apprSvc,
		msvc:      msvc,
		office:    office,
		sibling:   sibling,
		managerRl: managerRl,
		stafRl:    stafRl,
		catID:     catID,
		problemID: problemID,
	}
}

// getAssetStatus fetches the current status of an asset.
func (h *harness) getAssetStatus(t *testing.T, assetID uuid.UUID) sqlc.SharedAssetStatus {
	t.Helper()
	a, err := h.q.GetAsset(context.Background(), assetID)
	require.NoError(t, err)
	return a.Status
}

func containsAttentionAsset(rows []sqlc.ListMaintAttentionAssetsRow, id uuid.UUID) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// ─── 1. schedule CRUD ───────────────────────────────────────────────────────

func TestScheduleCRUD_HappyPath(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00001", "AC Split Lantai 2", h.catID, h.office, sqlc.SharedAssetStatusAvailable)

	sch, err := h.msvc.CreateSchedule(ctx, false, []uuid.UUID{h.office}, maintenance.ScheduleInput{
		AssetID:        assetID,
		IntervalMonths: 6,
		StartDate:      "2026-07-01",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(6), sch.IntervalMonths)
	assert.True(t, sch.IsActive)
	require.Equal(t, "2026-07-01", *common.DateStr(sch.NextDueDate))
	assert.False(t, sch.LastDoneDate.Valid, "a fresh schedule has no last_done_date")

	rows, total, err := h.msvc.ListSchedules(ctx, false, []uuid.UUID{h.office}, nil, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, sch.ID, rows[0].MaintenanceMaintenanceSchedule.ID)
	assert.Equal(t, "AC Split Lantai 2", rows[0].AssetName)
	assert.Equal(t, "OFC-MNT-2026-00001", rows[0].AssetTag)
	require.NotNil(t, rows[0].OfficeName)

	// Simulate a first completion having set last_done_date, then update the
	// interval — next_due_date must be recomputed off last_done_date.
	_, err = h.pool.Exec(ctx,
		`UPDATE maintenance.maintenance_schedules SET last_done_date = '2026-01-15' WHERE id = $1`, sch.ID)
	require.NoError(t, err)

	updated, err := h.msvc.UpdateSchedule(ctx, false, []uuid.UUID{h.office}, sch.ID, maintenance.ScheduleUpdateInput{
		IntervalMonths: int32ptr(3),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), updated.IntervalMonths)
	require.Equal(t, "2026-01-15", *common.DateStr(updated.LastDoneDate))
	require.Equal(t, "2026-04-15", *common.DateStr(updated.NextDueDate), "next_due = last_done + new interval")

	require.NoError(t, h.msvc.DeleteSchedule(ctx, false, []uuid.UUID{h.office}, sch.ID))

	rows, total, err = h.msvc.ListSchedules(ctx, false, []uuid.UUID{h.office}, nil, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows)
}

// ─── 2. schedule create guards ──────────────────────────────────────────────

func TestScheduleCreate_Guards(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	t.Run("out of scope asset rejected", func(t *testing.T) {
		siblingAsset := seedAsset(t, h.pool, "OFC-MNT-2026-00002", "Printer Sibling", h.catID, h.sibling, sqlc.SharedAssetStatusAvailable)
		_, err := h.msvc.CreateSchedule(ctx, false, []uuid.UUID{h.office}, maintenance.ScheduleInput{
			AssetID: siblingAsset, IntervalMonths: 6, StartDate: "2026-07-01",
		})
		require.ErrorIs(t, err, maintenance.ErrOutOfScope)
	})

	t.Run("disposed asset rejected", func(t *testing.T) {
		disposedAsset := seedAsset(t, h.pool, "OFC-MNT-2026-00003", "Laptop Disposed", h.catID, h.office, sqlc.SharedAssetStatusDisposed)
		_, err := h.msvc.CreateSchedule(ctx, false, []uuid.UUID{h.office}, maintenance.ScheduleInput{
			AssetID: disposedAsset, IntervalMonths: 6, StartDate: "2026-07-01",
		})
		require.ErrorIs(t, err, maintenance.ErrAssetNotMaintainable)
	})

	t.Run("interval 0 rejected", func(t *testing.T) {
		okAsset := seedAsset(t, h.pool, "OFC-MNT-2026-00004", "Genset", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
		_, err := h.msvc.CreateSchedule(ctx, false, []uuid.UUID{h.office}, maintenance.ScheduleInput{
			AssetID: okAsset, IntervalMonths: 0, StartDate: "2026-07-01",
		})
		require.ErrorIs(t, err, maintenance.ErrInvalidInterval)
	})
}

// ─── 3. record create status effects ────────────────────────────────────────

func TestRecordCreate_StatusEffects(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	actor := seedUser(t, h.pool, h.managerRl, h.office, "actor.create@test.local")

	assetInProgress := seedAsset(t, h.pool, "OFC-MNT-2026-00005", "Lift Barang", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
	rec, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID:     assetInProgress,
		Type:        sqlc.SharedMaintenanceTypePreventive,
		Status:      sqlc.SharedMaintenanceStatusInProgress,
		Description: "servis lift rutin",
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedMaintenanceStatusInProgress, rec.Status)
	assert.Equal(t, sqlc.SharedAssetStatusUnderMaintenance, h.getAssetStatus(t, assetInProgress))

	assetScheduled := seedAsset(t, h.pool, "OFC-MNT-2026-00006", "Genset Cadangan", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
	rec2, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID:     assetScheduled,
		Type:        sqlc.SharedMaintenanceTypePreventive,
		Description: "jadwal servis bulan depan",
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedMaintenanceStatusScheduled, rec2.Status, "an empty Status input defaults to scheduled")
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetScheduled), "a merely-scheduled record must not touch the asset")
}

// ─── 4. record complete releases + touches schedule ────────────────────────

func TestRecordComplete_ReleasesAndTouchesSchedule(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	actor := seedUser(t, h.pool, h.managerRl, h.office, "actor.complete@test.local")

	assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00007", "AC Ruang Server", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
	sch, err := h.msvc.CreateSchedule(ctx, false, []uuid.UUID{h.office}, maintenance.ScheduleInput{
		AssetID: assetID, IntervalMonths: 6, StartDate: "2026-01-01",
	})
	require.NoError(t, err)

	rec, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID:     assetID,
		ScheduleID:  &sch.ID,
		Type:        sqlc.SharedMaintenanceTypePreventive,
		Status:      sqlc.SharedMaintenanceStatusInProgress,
		Description: "servis rutin AC",
	})
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedAssetStatusUnderMaintenance, h.getAssetStatus(t, assetID))

	completedDate := "2026-07-05"
	updated, err := h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec.ID, maintenance.RecordUpdateInput{
		Status:        msPtr(sqlc.SharedMaintenanceStatusCompleted),
		CompletedDate: &completedDate,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedMaintenanceStatusCompleted, updated.Status)
	require.Equal(t, completedDate, *common.DateStr(updated.CompletedDate))

	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID), "asset must be released once its only active record completes")

	schedAfter, err := h.q.GetMaintScheduleScoped(ctx, sqlc.GetMaintScheduleScopedParams{ID: sch.ID, AllScope: true, OfficeIds: []uuid.UUID{}})
	require.NoError(t, err)
	require.Equal(t, "2026-07-05", *common.DateStr(schedAfter.LastDoneDate))
	require.Equal(t, "2027-01-05", *common.DateStr(schedAfter.NextDueDate), "next_due = completed_date + interval_months")
}

// ─── 5. record complete keeps asset when another active record remains ────

func TestRecordComplete_KeepsAssetWhenAnotherActive(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	actor := seedUser(t, h.pool, h.managerRl, h.office, "actor.keep@test.local")

	assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00008", "Server Rack A", h.catID, h.office, sqlc.SharedAssetStatusAvailable)

	rec1, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID: assetID, Type: sqlc.SharedMaintenanceTypeCorrective, Status: sqlc.SharedMaintenanceStatusInProgress, Description: "perbaikan 1",
	})
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedAssetStatusUnderMaintenance, h.getAssetStatus(t, assetID))

	rec2, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID: assetID, Type: sqlc.SharedMaintenanceTypeCorrective, Description: "perbaikan 2 (menunggu giliran)",
	})
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedMaintenanceStatusScheduled, rec2.Status)

	_, err = h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec1.ID, maintenance.RecordUpdateInput{
		Status: msPtr(sqlc.SharedMaintenanceStatusCompleted),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusUnderMaintenance, h.getAssetStatus(t, assetID), "asset must stay under_maintenance while rec2 is still active")

	_, err = h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec2.ID, maintenance.RecordUpdateInput{
		Status: msPtr(sqlc.SharedMaintenanceStatusCancelled),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID), "asset must be released once the last active record is cancelled")
}

// ─── 6. invalid record transitions ─────────────────────────────────────────

func TestRecordTransition_Invalid(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	actor := seedUser(t, h.pool, h.managerRl, h.office, "actor.invalid@test.local")

	t.Run("in_progress backward to scheduled rejected", func(t *testing.T) {
		assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00009", "Aset Transisi 1", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
		rec, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
			AssetID: assetID, Type: sqlc.SharedMaintenanceTypeCorrective, Status: sqlc.SharedMaintenanceStatusInProgress, Description: "in progress",
		})
		require.NoError(t, err)
		_, err = h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec.ID, maintenance.RecordUpdateInput{
			Status: msPtr(sqlc.SharedMaintenanceStatusScheduled),
		})
		require.ErrorIs(t, err, maintenance.ErrInvalidTransition)
	})

	// Note: a completed/cancelled record is terminal — UpdateRecord's terminal
	// guard fires before the transition table is even consulted, so *any*
	// further update (including an attempted "completed -> in_progress" move)
	// is rejected with ErrTerminal, never ErrInvalidTransition. This matches
	// service.go's UpdateRecord (terminal check precedes validTransition).
	t.Run("any update on a completed record rejected as terminal", func(t *testing.T) {
		assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00010", "Aset Transisi 2", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
		rec, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
			AssetID: assetID, Type: sqlc.SharedMaintenanceTypeCorrective, Status: sqlc.SharedMaintenanceStatusCompleted, Description: "sudah selesai",
		})
		require.NoError(t, err)
		require.Equal(t, sqlc.SharedMaintenanceStatusCompleted, rec.Status)

		_, err = h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec.ID, maintenance.RecordUpdateInput{
			Status: msPtr(sqlc.SharedMaintenanceStatusInProgress),
		})
		require.ErrorIs(t, err, maintenance.ErrTerminal)

		_, err = h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec.ID, maintenance.RecordUpdateInput{
			Description: strptr("edit deskripsi"),
		})
		require.ErrorIs(t, err, maintenance.ErrTerminal)
	})

	t.Run("in_progress on an in_transfer asset rejected as busy", func(t *testing.T) {
		busyAsset := seedAsset(t, h.pool, "OFC-MNT-2026-00011", "Aset Transfer", h.catID, h.office, sqlc.SharedAssetStatusInTransfer)
		_, err := h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
			AssetID: busyAsset, Type: sqlc.SharedMaintenanceTypeCorrective, Status: sqlc.SharedMaintenanceStatusInProgress, Description: "coba mulai",
		})
		require.ErrorIs(t, err, maintenance.ErrAssetBusy)

		rows, err := h.msvc.ListByAsset(ctx, busyAsset, true, nil)
		require.NoError(t, err)
		assert.Empty(t, rows, "the failed create must be rolled back entirely")
		assert.Equal(t, sqlc.SharedAssetStatusInTransfer, h.getAssetStatus(t, busyAsset), "asset status must be untouched")
	})

	t.Run("schedule belonging to another asset rejected", func(t *testing.T) {
		assetX := seedAsset(t, h.pool, "OFC-MNT-2026-00012", "Aset X", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
		schedX, err := h.msvc.CreateSchedule(ctx, false, []uuid.UUID{h.office}, maintenance.ScheduleInput{
			AssetID: assetX, IntervalMonths: 6, StartDate: "2026-07-01",
		})
		require.NoError(t, err)

		assetY := seedAsset(t, h.pool, "OFC-MNT-2026-00013", "Aset Y", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
		_, err = h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
			AssetID: assetY, ScheduleID: &schedX.ID, Type: sqlc.SharedMaintenanceTypePreventive, Description: "salah pasang jadwal",
		})
		require.ErrorIs(t, err, maintenance.ErrScheduleMismatch)
	})
}

// ─── 7. record scope (read + write) ─────────────────────────────────────────

func TestRecordScope_ReadAndWrite(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	actor := seedUser(t, h.pool, h.managerRl, h.office, "actor.scope@test.local")

	siblingAsset := seedAsset(t, h.pool, "OFC-MNT-2026-00014", "Aset Sibling", h.catID, h.sibling, sqlc.SharedAssetStatusAvailable)

	// Seed the record bypassing scope (all=true) so it exists to be read/written
	// against below.
	rec, err := h.msvc.CreateRecord(ctx, true, nil, actor, maintenance.RecordInput{
		AssetID: siblingAsset, Type: sqlc.SharedMaintenanceTypePreventive, Description: "punya sibling office",
	})
	require.NoError(t, err)

	rows, total, err := h.msvc.ListRecords(ctx, false, []uuid.UUID{h.office}, "", "", "", 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows, "a caller scoped only to h.office must not see the sibling office's record")

	_, err = h.msvc.GetRecord(ctx, rec.ID, false, []uuid.UUID{h.office})
	require.ErrorIs(t, err, maintenance.ErrNotFound)

	_, err = h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID: siblingAsset, Type: sqlc.SharedMaintenanceTypePreventive, Description: "seharusnya gagal",
	})
	require.ErrorIs(t, err, maintenance.ErrOutOfScope)

	_, err = h.msvc.UpdateRecord(ctx, false, []uuid.UUID{h.office}, rec.ID, maintenance.RecordUpdateInput{
		Description: strptr("seharusnya gagal juga"),
	})
	require.ErrorIs(t, err, maintenance.ErrNotFound, "GetMaintRecordScoped filters the row out of scope before any update can apply")
}

// ─── 8/9. Staf damage report -> approval -> corrective record ──────────────

func TestSubmitReport_ApproveCreatesRecord(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00015", "Laptop Staf", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
	staf := seedUser(t, h.pool, h.stafRl, h.office, "staf.report.approve@test.local")
	manager := seedUser(t, h.pool, h.managerRl, h.office, "manager.report.approve@test.local")

	callerS := buildCaller(staf, h.stafRl, false, []uuid.UUID{h.office})
	desc := "Layar retak setelah jatuh"
	req, err := h.msvc.SubmitReport(ctx, callerS, maintenance.ReportInput{
		AssetID: assetID, ProblemCategoryID: h.problemID, Description: &desc,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, sqlc.SharedRequestTypeMaintenance, req.Type)
	require.NotNil(t, req.TargetID)
	assert.Equal(t, assetID, *req.TargetID)

	var payload maintenance.MaintenancePayload
	require.NoError(t, json.Unmarshal(req.Payload, &payload))
	assert.Equal(t, assetID.String(), payload.AssetID)
	assert.Equal(t, h.problemID.String(), payload.ProblemCategoryID)
	require.NotNil(t, payload.Description)
	assert.Equal(t, desc, *payload.Description)
	assert.Nil(t, payload.AttachmentID)

	// Duplicate pending report for the same (asset, maker) is rejected.
	_, err = h.msvc.SubmitReport(ctx, callerS, maintenance.ReportInput{
		AssetID: assetID, ProblemCategoryID: h.problemID, Description: &desc,
	})
	require.ErrorIs(t, err, maintenance.ErrDuplicatePending)

	// Manager (a different user, same office) approves.
	callerP := buildCaller(manager, h.managerRl, false, []uuid.UUID{h.office})
	final, err := h.apprSvc.Decide(ctx, req.ID, callerP, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	rows, err := h.msvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rec := rows[0].MaintenanceMaintenanceRecord
	assert.Equal(t, sqlc.SharedMaintenanceStatusScheduled, rec.Status)
	assert.Equal(t, sqlc.SharedMaintenanceTypeCorrective, rec.Type)
	require.NotNil(t, rec.ProblemCategoryID)
	assert.Equal(t, h.problemID, *rec.ProblemCategoryID)
	require.NotNil(t, rec.ReportedByID)
	assert.Equal(t, staf, *rec.ReportedByID, "reported_by must be the maker, not the approver")
	assert.Equal(t, desc, rec.Description)

	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID), "approval alone must not flip the asset — only starting the work does")
}

func TestSubmitReport_RejectLeavesNoRecord(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00016", "Printer Staf", h.catID, h.office, sqlc.SharedAssetStatusAvailable)
	staf := seedUser(t, h.pool, h.stafRl, h.office, "staf.report.reject@test.local")
	manager := seedUser(t, h.pool, h.managerRl, h.office, "manager.report.reject@test.local")

	callerS := buildCaller(staf, h.stafRl, false, []uuid.UUID{h.office})
	req, err := h.msvc.SubmitReport(ctx, callerS, maintenance.ReportInput{
		AssetID: assetID, ProblemCategoryID: h.problemID,
	})
	require.NoError(t, err)

	callerP := buildCaller(manager, h.managerRl, false, []uuid.UUID{h.office})
	note := "tidak sesuai laporan"
	final, err := h.apprSvc.Decide(ctx, req.ID, callerP, false, &note)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, final.Status)

	rows, err := h.msvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	assert.Empty(t, rows, "a rejected report must not create any maintenance record")

	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID))
}

// ─── 10. attention queue ────────────────────────────────────────────────────

func TestAttention_Queue(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	actor := seedUser(t, h.pool, h.managerRl, h.office, "actor.attention@test.local")

	assetID := seedAsset(t, h.pool, "OFC-MNT-2026-00017", "Aset Perlu Tindak Lanjut", h.catID, h.office, sqlc.SharedAssetStatusUnderMaintenance)

	rows, err := h.msvc.Attention(ctx, false, []uuid.UUID{h.office})
	require.NoError(t, err)
	assert.True(t, containsAttentionAsset(rows, assetID), "an under_maintenance asset with no active record must appear")

	_, err = h.msvc.CreateRecord(ctx, false, []uuid.UUID{h.office}, actor, maintenance.RecordInput{
		AssetID: assetID, Type: sqlc.SharedMaintenanceTypeCorrective, Description: "sudah dijadwalkan",
	})
	require.NoError(t, err)

	rows, err = h.msvc.Attention(ctx, false, []uuid.UUID{h.office})
	require.NoError(t, err)
	assert.False(t, containsAttentionAsset(rows, assetID), "once a scheduled/in_progress record exists, the asset must drop off the queue")

	siblingAsset := seedAsset(t, h.pool, "OFC-MNT-2026-00018", "Aset Sibling Perlu Tindak Lanjut", h.catID, h.sibling, sqlc.SharedAssetStatusUnderMaintenance)
	rows, err = h.msvc.Attention(ctx, false, []uuid.UUID{h.office})
	require.NoError(t, err)
	assert.False(t, containsAttentionAsset(rows, siblingAsset), "an out-of-scope asset must never appear")
}
