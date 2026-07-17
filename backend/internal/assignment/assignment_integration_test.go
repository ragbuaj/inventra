//go:build integration

package assignment_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/assignment"
	"github.com/ragbuaj/inventra/internal/authz"
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

// seedAsset inserts an asset.assets row directly with the given status and
// returns its id. Uses asset_class=intangible to avoid the room FK constraint.
func seedAsset(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', $5::shared.asset_status)
		 RETURNING id`,
		tag, name, categoryID, officeID, status).Scan(&id))
	return id
}

// seedUser inserts an identity.users row (placed in officeID, optionally linked
// to an employee) and returns its id.
func seedUser(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, employeeID *uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, employee_id, status)
		 VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
		email, email, roleID, officeID, employeeID).Scan(&id))
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

// harness bundles everything an assignment test needs: pool, sqlc queries, the
// approval + assignment services (with the assignment executor registered),
// an office O plus a sibling office, the Manager/Kepala-Unit and Staf roles,
// and a shared category.
type harness struct {
	pool      *pgxpool.Pool
	rdb       *redis.Client
	q         *sqlc.Queries
	apprSvc   *approval.Service
	asvc      *assignment.Service
	office    uuid.UUID
	sibling   uuid.UUID
	managerRl uuid.UUID
	stafRl    uuid.UUID
	catID     uuid.UUID
}

// newHarness boots a throwaway Postgres + Redis, and wires the approval +
// assignment services with the assignment executor registered. Seeds office O
// and an unrelated sibling office (for out-of-scope assertions).
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	office := seedOfficeWithType(t, pool, "AssignType-"+uuid.New().String()[:8], "OFC"+uuid.New().String()[:4])
	sibling := seedOfficeWithType(t, pool, "SiblingType-"+uuid.New().String()[:8], "SIB"+uuid.New().String()[:4])
	catID := seedCategory(t, pool, "ASG"+uuid.New().String()[:4])

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	apprSvc := approval.NewService(q, pool, scopeSvc, rdb)
	asvc := assignment.NewService(q, pool, apprSvc)
	apprSvc.RegisterExecutor(sqlc.SharedRequestTypeAssignment, asvc.Executor())

	managerRl := lookupRole(t, pool, "Manager")
	stafRl := lookupRole(t, pool, "Staf")

	return &harness{
		pool:      pool,
		rdb:       rdb,
		q:         q,
		apprSvc:   apprSvc,
		asvc:      asvc,
		office:    office,
		sibling:   sibling,
		managerRl: managerRl,
		stafRl:    stafRl,
		catID:     catID,
	}
}

// seedManager creates a Manager-role user scoped (by the caller) to the given
// office and returns its id.
func (h *harness) seedManager(t *testing.T, officeID uuid.UUID, email string) uuid.UUID {
	return seedUser(t, h.pool, h.managerRl, officeID, nil, email)
}

// seedStaf creates a Staf-role user linked to a fresh employee in officeID and
// returns (userID, employeeID).
func (h *harness) seedStaf(t *testing.T, officeID uuid.UUID, email, employeeCode string) (uuid.UUID, uuid.UUID) {
	empID := testsupport.SeedEmployee(t, h.pool, officeID, employeeCode)
	userID := seedUser(t, h.pool, h.stafRl, officeID, &empID, email)
	return userID, empID
}

// getAssetStatus fetches the current status of an asset.
func (h *harness) getAssetStatus(t *testing.T, assetID uuid.UUID) sqlc.SharedAssetStatus {
	t.Helper()
	a, err := h.q.GetAsset(context.Background(), assetID)
	require.NoError(t, err)
	return a.Status
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestAssignment_Checkout_flips_asset_to_assigned(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00001", "Laptop Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.checkout@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.checkout@test.local", "EMP-CO-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID:      assetID,
		EmployeeID:   empID,
		CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssignmentStatusActive, a.Status)
	assert.Equal(t, assetID, a.AssetID)
	assert.Equal(t, empID, a.EmployeeID)
	assert.Equal(t, manager, a.AssignedByID)

	assert.Equal(t, sqlc.SharedAssetStatusAssigned, h.getAssetStatus(t, assetID))
}

func TestAssignment_Checkout_rejects_unavailable_asset(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00002", "Printer Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.unavail@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.unavail@test.local", "EMP-UN-1")

	_, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusAssigned, h.getAssetStatus(t, assetID))

	// Second checkout of the same (now-assigned) asset must fail.
	_, empID2 := h.seedStaf(t, h.office, "staf.unavail2@test.local", "EMP-UN-2")
	_, err = h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID2, CheckoutDate: "2026-07-06",
	})
	require.ErrorIs(t, err, assignment.ErrAssetNotAvailable)
}

func TestAssignment_Checkout_unique_active_per_asset(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00003", "Kamera Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.unique@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.unique@test.local", "EMP-UQ-1")

	_, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	// The availability guard fires before any unique-index path, but the net
	// effect must be identical: only one active row for this asset.
	_, empID2 := h.seedStaf(t, h.office, "staf.unique2@test.local", "EMP-UQ-2")
	_, err = h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID2, CheckoutDate: "2026-07-06",
	})
	require.ErrorIs(t, err, assignment.ErrAssetNotAvailable)

	activeStatus := "active"
	rows, total, err := h.asvc.List(ctx, true, nil, activeStatus, nil, "", 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, assetID, rows[0].AssignmentAssignment.AssetID)
}

func TestAssignment_Checkout_out_of_scope(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00004", "Scanner Dinas", h.catID, h.office, "available")
	// Manager scoped to the sibling office only — excludes h.office.
	manager := h.seedManager(t, h.sibling, "manager.oos@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.oos@test.local", "EMP-OS-1")

	_, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.sibling}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.ErrorIs(t, err, assignment.ErrOutOfScope)
	// Asset must be unaffected.
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID))
}

func TestAssignment_Checkin_returns_and_frees_asset(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00005", "Router Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.checkin@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.checkin@test.local", "EMP-CI-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	before, after, err := h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, manager, assignment.CheckinInput{})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssignmentStatusActive, before.Status)
	assert.Equal(t, sqlc.SharedAssignmentStatusReturned, after.Status)
	require.True(t, after.CheckinDate.Valid)

	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID))
}

func TestAssignment_Checkin_needs_maintenance_sets_under_maintenance(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00006", "AC Split Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.maint@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.maint@test.local", "EMP-MT-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, after, err := h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, manager, assignment.CheckinInput{NeedsMaintenance: true})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssignmentStatusReturned, after.Status)

	assert.Equal(t, sqlc.SharedAssetStatusUnderMaintenance, h.getAssetStatus(t, assetID))
}

func TestAssignment_Checkin_rejects_non_active(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00007", "Meja Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.nonactive@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.nonactive@test.local", "EMP-NA-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, manager, assignment.CheckinInput{})
	require.NoError(t, err)

	// Check-in an already-returned assignment must fail.
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, manager, assignment.CheckinInput{})
	require.ErrorIs(t, err, assignment.ErrNotActive)
}

func TestAssignment_Borrow_then_approve_creates_assignment(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00008", "Proyektor Dinas", h.catID, h.office, "available")
	stafID, empID := h.seedStaf(t, h.office, "staf.borrow@test.local", "EMP-BW-1")
	// Approver eligible for the office-level assignment step: office-scoped,
	// distinct from the requester, with request.decide (Manager holds it).
	approverID := h.seedManager(t, h.office, "approver.borrow@test.local")

	callerS := buildCaller(stafID, h.stafRl, false, []uuid.UUID{h.office})
	req, err := h.asvc.SubmitBorrow(ctx, callerS, assignment.BorrowInput{
		AssetID: assetID,
		DueDate: strptr("2026-07-20"),
		Notes:   strptr("perlu buat presentasi"),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, sqlc.SharedRequestTypeAssignment, req.Type)

	callerP := buildCaller(approverID, h.managerRl, false, []uuid.UUID{h.office})
	final, err := h.apprSvc.Decide(ctx, req.ID, callerP, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	rows, err := h.asvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, sqlc.SharedAssignmentStatusActive, rows[0].AssignmentAssignment.Status)
	assert.Equal(t, empID, rows[0].AssignmentAssignment.EmployeeID)
	assert.Equal(t, approverID, rows[0].AssignmentAssignment.AssignedByID)

	assert.Equal(t, sqlc.SharedAssetStatusAssigned, h.getAssetStatus(t, assetID))
}

func TestAssignment_Borrow_reject_leaves_no_assignment(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00009", "Tablet Dinas", h.catID, h.office, "available")
	stafID, _ := h.seedStaf(t, h.office, "staf.rejborrow@test.local", "EMP-RB-1")
	approverID := h.seedManager(t, h.office, "approver.rejborrow@test.local")

	callerS := buildCaller(stafID, h.stafRl, false, []uuid.UUID{h.office})
	req, err := h.asvc.SubmitBorrow(ctx, callerS, assignment.BorrowInput{AssetID: assetID})
	require.NoError(t, err)

	callerP := buildCaller(approverID, h.managerRl, false, []uuid.UUID{h.office})
	note := "tidak diperlukan"
	final, err := h.apprSvc.Decide(ctx, req.ID, callerP, false, &note)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, final.Status)

	rows, err := h.asvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	assert.Empty(t, rows, "no assignment row should exist for a rejected borrow")

	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID))
}

func TestAssignment_Borrow_executor_rejects_unavailable_at_approval(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00010", "Mic Wireless Dinas", h.catID, h.office, "available")
	stafID, _ := h.seedStaf(t, h.office, "staf.race@test.local", "EMP-RC-1")
	_, directEmpID := h.seedStaf(t, h.office, "staf.race.direct@test.local", "EMP-RC-2")
	approverID := h.seedManager(t, h.office, "approver.race@test.local")

	callerS := buildCaller(stafID, h.stafRl, false, []uuid.UUID{h.office})
	req, err := h.asvc.SubmitBorrow(ctx, callerS, assignment.BorrowInput{AssetID: assetID})
	require.NoError(t, err)

	// Before approving, check out the asset directly to someone else.
	direct, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, approverID, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: directEmpID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusAssigned, h.getAssetStatus(t, assetID))

	callerP := buildCaller(approverID, h.managerRl, false, []uuid.UUID{h.office})
	_, err = h.apprSvc.Decide(ctx, req.ID, callerP, true, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, approval.ErrConflict)

	// The request must not have been approved.
	reloadedReq, err := h.q.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, reloadedReq.Status, "request should still be pending after executor rollback")

	// The asset must stay with the direct assignee — only one active row, for directEmpID.
	rows, err := h.asvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, direct.ID, rows[0].AssignmentAssignment.ID)
	assert.Equal(t, directEmpID, rows[0].AssignmentAssignment.EmployeeID)
	assert.Equal(t, sqlc.SharedAssignmentStatusActive, rows[0].AssignmentAssignment.Status)
	assert.Equal(t, sqlc.SharedAssetStatusAssigned, h.getAssetStatus(t, assetID))
}

func TestAssignment_List_scoped(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00011", "Speaker Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.scoped@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.scoped@test.local", "EMP-SC-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	// Manager scoped to O sees it.
	rows, total, err := h.asvc.List(ctx, false, []uuid.UUID{h.office}, "", nil, "", 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, a.ID, rows[0].AssignmentAssignment.ID)

	// Manager scoped to a sibling office does not see it.
	rows, total, err = h.asvc.List(ctx, false, []uuid.UUID{h.sibling}, "", nil, "", 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows)
}

func TestAssignment_ListByAsset_history(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00012", "Monitor Dinas", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.history@test.local")
	_, emp1 := h.seedStaf(t, h.office, "staf.history1@test.local", "EMP-HS-1")
	_, emp2 := h.seedStaf(t, h.office, "staf.history2@test.local", "EMP-HS-2")

	// Cycle 1: checkout to emp1, then check in.
	a1, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: emp1, CheckoutDate: "2026-07-01",
	})
	require.NoError(t, err)
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a1.ID, manager, assignment.CheckinInput{})
	require.NoError(t, err)

	// Cycle 2: checkout to emp2, then check in.
	a2, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: emp2, CheckoutDate: "2026-07-05",
	})
	require.NoError(t, err)
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a2.ID, manager, assignment.CheckinInput{})
	require.NoError(t, err)

	rows, err := h.asvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// Newest-first: the second cycle (later checkout_date) must come first.
	assert.Equal(t, a2.ID, rows[0].AssignmentAssignment.ID)
	assert.Equal(t, emp2, rows[0].AssignmentAssignment.EmployeeID)
	assert.Equal(t, a1.ID, rows[1].AssignmentAssignment.ID)
	assert.Equal(t, emp1, rows[1].AssignmentAssignment.EmployeeID)
	assert.Equal(t, sqlc.SharedAssignmentStatusReturned, rows[0].AssignmentAssignment.Status)
	assert.Equal(t, sqlc.SharedAssignmentStatusReturned, rows[1].AssignmentAssignment.Status)
}

func TestAssignment_Borrow_out_of_scope(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Asset lives in the sibling office; the Staf's scope only covers h.office.
	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00013", "Laptop Sibling", h.catID, h.sibling, "available")
	stafID, _ := h.seedStaf(t, h.office, "staf.borrowoos@test.local", "EMP-BO-1")

	callerS := buildCaller(stafID, h.stafRl, false, []uuid.UUID{h.office})
	_, err := h.asvc.SubmitBorrow(ctx, callerS, assignment.BorrowInput{
		AssetID: assetID,
		DueDate: strptr("2026-07-20"),
		Notes:   strptr("perlu buat presentasi"),
	})
	require.ErrorIs(t, err, assignment.ErrOutOfScope)

	// No approval request must have been created for the out-of-scope asset.
	rows, err := h.asvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	assert.Empty(t, rows, "no assignment row should exist for an out-of-scope borrow attempt")
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, h.getAssetStatus(t, assetID))

	var reqCount int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM approval.requests WHERE target_entity = 'asset' AND target_id = $1`,
		assetID).Scan(&reqCount))
	assert.Equal(t, 0, reqCount, "no approval request row should exist for an out-of-scope borrow attempt")
}

// TestAssignment_Mine_returns_only_caller_own_rows guards the fix for the
// office-wide read leak (see docs/PROGRESS.md item 38): two Staf in the same
// office each hold an active assignment; Mine(ctx, employeeID, ...) must
// return ONLY the rows for the given employee id, never the coworker's, even
// though the service call itself sets AllScope=true internally (that is only
// safe because the employee id here is meant to always be resolved
// server-side from the caller's own JWT — see the Mine doc comment).
func TestAssignment_Mine_returns_only_caller_own_rows(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetA := seedAsset(t, h.pool, "OFC-ASG-2026-00014", "Laptop Staf A", h.catID, h.office, "available")
	assetB := seedAsset(t, h.pool, "OFC-ASG-2026-00015", "Laptop Staf B", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.mine@test.local")
	_, empA := h.seedStaf(t, h.office, "staf.mineA@test.local", "EMP-MI-A")
	_, empB := h.seedStaf(t, h.office, "staf.mineB@test.local", "EMP-MI-B")

	aA, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetA, EmployeeID: empA, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)
	_, err = h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetB, EmployeeID: empB, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	rows, err := h.asvc.Mine(ctx, empA, "")
	require.NoError(t, err)
	require.Len(t, rows, 1, "Staf A must see exactly their own assignment, never their coworker's")
	assert.Equal(t, aA.ID, rows[0].AssignmentAssignment.ID)
	assert.Equal(t, empA, rows[0].AssignmentAssignment.EmployeeID)

	// Status filter still applies within the caller's own rows.
	rows, err = h.asvc.Mine(ctx, empA, "returned")
	require.NoError(t, err)
	assert.Empty(t, rows, "no returned assignment exists yet for Staf A")
}

// TestAssignment_Staf_role_lacks_assignment_view is the seed-level guard for
// the reverted office-wide grant: migration 000028 (which would have granted
// Staf `assignment.view`) was deleted rather than applied, precisely because
// `assignment.view` + the office-level data scope would let any Staf list
// every coworker's assignment via `GET /assignments` (the general, non-"mine"
// list endpoint) simply by omitting the client-supplied `employee_id` filter.
// The picker instead uses the dedicated, server-scoped `GET /assignments/mine`
// (gated by `request.create`, already seeded in 000005) — no new permission
// grant. This asserts the Staf role still has no `assignment.view` row after
// all migrations have run, i.e. the general list route stays 403 for Staf.
func TestAssignment_Staf_role_lacks_assignment_view(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	var count int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*)
		   FROM identity.role_permissions rp
		   JOIN identity.roles r ON r.id = rp.role_id
		  WHERE r.name = 'Staf' AND r.deleted_at IS NULL AND rp.permission_key = 'assignment.view'`,
	).Scan(&count))
	assert.Equal(t, 0, count, "Staf must NOT hold assignment.view — the picker leak fix uses GET /assignments/mine (request.create) instead")
}

func strptr(s string) *string { return &s }
