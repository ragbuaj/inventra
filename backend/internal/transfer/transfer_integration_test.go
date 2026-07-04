//go:build integration

package transfer_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
	"github.com/ragbuaj/inventra/internal/transfer"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func strptr(s string) *string { return &s }

// resetAll truncates the mutable schemas touched by transfer tests. Each test
// gets its own throwaway container (testsupport.NewPostgres), so this mostly
// guards against any shared-pool scenarios while leaving migration-seeded
// identity rows (roles, scope policies, thresholds) intact.
func resetAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 transfer.asset_transfers, asset.asset_documents,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)
}

// seedOfficeWithType inserts a single-office setup (one type, one office) and
// returns the office ID.
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

// seedOfficeChild inserts an office under the given parent, sharing the parent's
// office_type_id, and returns the new office ID.
func seedOfficeChild(t *testing.T, pool *pgxpool.Pool, parentID uuid.UUID, name, code string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT office_type_id FROM masterdata.offices WHERE id = $1`, parentID).Scan(&typeID))

	var officeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		parentID, typeID, name, code).Scan(&officeID))
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

// seedAssetWithCost inserts an asset.assets row directly (status=available) with
// the given purchase_cost (or NULL when empty) and returns its id.
func seedAssetWithCost(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, purchaseCost string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status, purchase_cost)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', 'available', $5)
		 RETURNING id`,
		tag, name, categoryID, officeID, purchaseCost).Scan(&id))
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

// approveThroughChain drives Decide(approve=true) for every pending step of the
// request using the same caller (sufficient scope + tier), returning the final
// request row. It mirrors the single-caller single-step happy path used across
// these tests (the seed thresholds under 50M give asset_transfer exactly one
// office-tier step).
func approveThroughChain(t *testing.T, apprSvc *approval.Service, reqID uuid.UUID, caller approval.Caller) sqlc.ApprovalRequest {
	t.Helper()
	ctx := context.Background()
	var out sqlc.ApprovalRequest
	var err error
	for i := 0; i < 10; i++ { // hard cap to avoid infinite loop on a bug
		out, err = apprSvc.Decide(ctx, reqID, caller, true, nil)
		require.NoError(t, err)
		if out.Status != sqlc.SharedRequestStatusPending {
			return out
		}
	}
	t.Fatalf("approveThroughChain: request %s still pending after 10 decisions", reqID)
	return out
}

// rejectFinalStep rejects the current (assumed final, or any) pending step.
func rejectFinalStep(t *testing.T, apprSvc *approval.Service, reqID uuid.UUID, caller approval.Caller) sqlc.ApprovalRequest {
	t.Helper()
	ctx := context.Background()
	note := "ditolak"
	out, err := apprSvc.Decide(ctx, reqID, caller, false, &note)
	require.NoError(t, err)
	return out
}

// harness bundles everything a transfer test needs: pool, redis, sqlc queries,
// the approval + transfer + asset services (with the asset_transfer executor
// registered), and a tiered from/to office pair in the same subtree.
type harness struct {
	pool         *pgxpool.Pool
	q            *sqlc.Queries
	apprSvc      *approval.Service
	tsvc         *transfer.Service
	assetSvc     *asset.Service
	fromOffice   uuid.UUID
	toOffice     uuid.UUID
	otherOffice  uuid.UUID
	officeRoleID uuid.UUID
	catID        uuid.UUID
}

// newHarness boots a throwaway Postgres + Redis + MinIO, resets mutable tables,
// and wires the approval/transfer/asset services with the asset_transfer
// executor registered. Seeds a two-office (fromOffice, toOffice) pair sharing
// the same parent (so a single office-tier approver covers both), plus a
// third, unrelated office for out-of-scope assertions.
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	minioStore := testsupport.NewMinIO(t)
	resetAll(t, pool)

	parent := seedOfficeWithType(t, pool, "TransferParentType-"+uuid.New().String()[:8], "TPX")
	fromOffice := seedOfficeChild(t, pool, parent, "From Office", "FROM"+uuid.New().String()[:4])
	toOffice := seedOfficeChild(t, pool, parent, "To Office", "TO"+uuid.New().String()[:4])
	otherOffice := seedOfficeWithType(t, pool, "OtherType-"+uuid.New().String()[:8], "OTH"+uuid.New().String()[:4])

	catID := seedCategory(t, pool, "TRF"+uuid.New().String()[:4])

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	apprSvc := approval.NewService(q, pool, scopeSvc, rdb)
	tsvc := transfer.NewService(q, pool, apprSvc)
	assetSvc := asset.NewService(q, pool, minioStore, 5<<20, "")
	apprSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetTransfer, tsvc.Executor())

	officeRoleID := lookupRole(t, pool, "Kepala Unit")

	return &harness{
		pool:         pool,
		q:            q,
		apprSvc:      apprSvc,
		tsvc:         tsvc,
		assetSvc:     assetSvc,
		fromOffice:   fromOffice,
		toOffice:     toOffice,
		otherOffice:  otherOffice,
		officeRoleID: officeRoleID,
		catID:        catID,
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestTransfer_HappyPath_SubmitApproveShipReceive drives the full lifecycle:
// submit (no transfer row yet) → approve through the chain (executor creates
// the row, status=approved) → ship (in_transit, shipped_date set) → receive
// (received; asset relocated to destination office/room).
func TestTransfer_HappyPath_SubmitApproveShipReceive(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00001", "Laptop Mutasi", h.catID, h.fromOffice, "10000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.happy@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.happy@test.local")
	receiverID := seedUser(t, h.pool, h.officeRoleID, h.toOffice, "receiver.happy@test.local")

	// Destination room, so receive can also relocate the asset's room_id.
	destFloor := testsupport.SeedFloor(t, h.pool, h.toOffice, "Lantai 1")
	destRoomID := testsupport.SeedRoom(t, h.pool, destFloor, "Ruang IT")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	// submit
	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{
		AssetID:    assetID,
		ToOfficeID: h.toOffice,
		Reason:     strptr("relok"),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)

	// no transfer row yet — executor only fires on final approval
	_, err = h.q.GetOpenTransferForAsset(ctx, assetID)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	// approve through the chain (10M < 50M → single office-tier step)
	finalReq := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, finalReq.Status)

	// executor created the transfer row, status=approved
	row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedTransferStatusApproved, row.Status)
	assert.Equal(t, h.fromOffice, row.FromOfficeID)
	assert.Equal(t, h.toOffice, row.ToOfficeID)

	// ship
	shipped, err := h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedTransferStatusInTransit, shipped.Status)
	require.True(t, shipped.ShippedDate.Valid)

	// receive → asset moved (office + room)
	before, after, err := h.tsvc.Receive(ctx, true, nil, receiverID, row.ID, transfer.ReceiveInput{BastNo: strptr("BAST-001"), ToRoomID: &destRoomID})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedTransferStatusInTransit, before.Status)
	assert.Equal(t, sqlc.SharedTransferStatusReceived, after.Status)
	require.True(t, after.ReceivedDate.Valid)
	require.NotNil(t, after.ReceivedByID)
	assert.Equal(t, receiverID, *after.ReceivedByID)
	require.NotNil(t, after.BastNo)
	assert.Equal(t, "BAST-001", *after.BastNo)

	movedAsset, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, h.toOffice, movedAsset.OfficeID)
	require.NotNil(t, movedAsset.RoomID, "asset room_id must be relocated on receive")
	assert.Equal(t, destRoomID, *movedAsset.RoomID)
}

// TestTransfer_Reject_NoTransferRow verifies that rejecting the final step
// finalises the request as rejected, creates NO transfer row, and leaves the
// asset's office unchanged.
func TestTransfer_Reject_NoTransferRow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00002", "Printer Mutasi", h.catID, h.fromOffice, "5000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.reject@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.reject@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{
		AssetID:    assetID,
		ToOfficeID: h.toOffice,
		Reason:     strptr("relok"),
	})
	require.NoError(t, err)

	final := rejectFinalStep(t, h.apprSvc, req.ID, checkerCaller)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, final.Status)

	_, err = h.q.GetOpenTransferForAsset(ctx, assetID)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, h.fromOffice, a.OfficeID, "asset office must be unchanged after rejection")
}

// TestTransfer_Submit_Guards covers the submit-time validation guards:
// asset already has an open transfer, destination == origin, and an
// out-of-scope caller.
func TestTransfer_Submit_Guards(t *testing.T) {
	t.Run("AssetInTransit_WhenOpenTransferExists", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00003", "Server Mutasi", h.catID, h.fromOffice, "5000000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.intransit@test.local")
		checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.intransit@test.local")

		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
		checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

		// First submit + approve → open transfer row (status=approved) now exists.
		req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("first")})
		require.NoError(t, err)
		final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
		require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

		// Second submit for the same asset must fail — an open transfer already exists.
		_, err = h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("second")})
		require.ErrorIs(t, err, transfer.ErrAssetInTransit)
	})

	t.Run("SameOffice_WhenToEqualsFrom", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00004", "Kursi Mutasi", h.catID, h.fromOffice, "1000000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.sameoffice@test.local")
		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})

		_, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.fromOffice, Reason: strptr("noop")})
		require.ErrorIs(t, err, transfer.ErrSameOffice)
	})

	t.Run("OutOfScope_WhenCallerLacksFromOffice", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00005", "Meja Mutasi", h.catID, h.fromOffice, "1000000")
		outsideUser := seedUser(t, h.pool, h.officeRoleID, h.otherOffice, "outside.submit@test.local")
		outsideCaller := buildCaller(outsideUser, h.officeRoleID, false, []uuid.UUID{h.otherOffice})

		_, err := h.tsvc.Submit(ctx, outsideCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("oos")})
		require.ErrorIs(t, err, transfer.ErrOutOfScope)
	})
}

// TestTransfer_Scope_And_StateMachine covers ship/receive scope enforcement and
// the state-machine guards (ship a non-approved row, receive a non-in_transit
// row).
func TestTransfer_Scope_And_StateMachine(t *testing.T) {
	// approvedTransfer submits + approves a transfer, returning the open
	// (status=approved) transfer row plus the harness/asset/maker/checker used.
	approvedTransfer := func(t *testing.T) (*harness, sqlc.TransferAssetTransfer, uuid.UUID) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-"+uuid.New().String()[:5], "Asset State", h.catID, h.fromOffice, "1000000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.state."+uuid.New().String()[:8]+"@test.local")
		checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.state."+uuid.New().String()[:8]+"@test.local")

		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
		checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

		req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("state")})
		require.NoError(t, err)
		final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
		require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

		row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
		require.NoError(t, err)
		require.Equal(t, sqlc.SharedTransferStatusApproved, row.Status)
		return h, row, assetID
	}

	t.Run("Ship_OutOfScope", func(t *testing.T) {
		h, row, _ := approvedTransfer(t)
		// Caller scoped to toOffice only — does not cover from_office.
		_, err := h.tsvc.Ship(context.Background(), false, []uuid.UUID{h.toOffice}, row.ID, transfer.ShipInput{})
		require.ErrorIs(t, err, transfer.ErrOutOfScope)
	})

	t.Run("Receive_OutOfScope", func(t *testing.T) {
		h, row, _ := approvedTransfer(t)
		ctx := context.Background()

		// Ship first (in scope) so we reach a valid in_transit row.
		shipped, err := h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
		require.NoError(t, err)
		require.Equal(t, sqlc.SharedTransferStatusInTransit, shipped.Status)

		receiver := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "receiver.oos."+uuid.New().String()[:8]+"@test.local")
		// Caller scoped to fromOffice only — does not cover to_office.
		_, _, err = h.tsvc.Receive(ctx, false, []uuid.UUID{h.fromOffice}, receiver, row.ID, transfer.ReceiveInput{})
		require.ErrorIs(t, err, transfer.ErrOutOfScope)
	})

	t.Run("Ship_InvalidState_AlreadyInTransit", func(t *testing.T) {
		h, row, _ := approvedTransfer(t)
		ctx := context.Background()

		shipped, err := h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
		require.NoError(t, err)
		require.Equal(t, sqlc.SharedTransferStatusInTransit, shipped.Status)

		// Shipping an already in_transit row must fail.
		_, err = h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
		require.ErrorIs(t, err, transfer.ErrInvalidState)
	})

	t.Run("Receive_InvalidState_StillApproved", func(t *testing.T) {
		h, row, _ := approvedTransfer(t)
		ctx := context.Background()
		receiver := seedUser(t, h.pool, h.officeRoleID, h.toOffice, "receiver.invalid."+uuid.New().String()[:8]+"@test.local")

		// Row is still "approved" (not shipped) — receive must fail.
		_, _, err := h.tsvc.Receive(ctx, true, nil, receiver, row.ID, transfer.ReceiveInput{})
		require.ErrorIs(t, err, transfer.ErrInvalidState)
	})
}

// TestTransfer_BAST_DocumentCreated verifies that after a successful receive,
// invoking asset.Service.CreateDocument the same way the handler's recordBAST
// does produces an asset_documents row with doc_type=bast_transfer and the
// related_request_id set.
func TestTransfer_BAST_DocumentCreated(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00006", "Asset BAST", h.catID, h.fromOffice, "2000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.bast@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.bast@test.local")
	receiverID := seedUser(t, h.pool, h.officeRoleID, h.toOffice, "receiver.bast@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("bast")})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
	require.NoError(t, err)

	_, err = h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
	require.NoError(t, err)

	_, after, err := h.tsvc.Receive(ctx, true, nil, receiverID, row.ID, transfer.ReceiveInput{BastNo: strptr("BAST-XYZ")})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedTransferStatusReceived, after.Status)

	// Mirror the handler's recordBAST: create the asset_documents(bast_transfer) row.
	doc, err := h.assetSvc.CreateDocument(ctx, asset.DocumentInput{
		AssetID:          after.AssetID,
		DocType:          sqlc.SharedAssetDocumentTypeBastTransfer,
		DocNo:            after.BastNo,
		DocDate:          after.ReceivedDate,
		RelatedRequestID: after.RequestID,
		CreatedBy:        receiverID,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetDocumentTypeBastTransfer, doc.DocType)

	docs, err := h.q.ListAssetDocuments(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, sqlc.SharedAssetDocumentTypeBastTransfer, docs[0].DocType)
	require.NotNil(t, docs[0].RelatedRequestID)
	assert.Equal(t, *after.RequestID, *docs[0].RelatedRequestID)
	require.NotNil(t, docs[0].DocNo)
	assert.Equal(t, "BAST-XYZ", *docs[0].DocNo)
}

// TestTransfer_ConditionAndDate_RoundTrip: submit carries condition_sent+transfer_date
// through the approval payload into the transfer row created by the executor.
func TestTransfer_ConditionAndDate_RoundTrip(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00008", "Proyektor Epson", h.catID, h.fromOffice, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.cond@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.cond@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	cond := "rusak_ringan"
	date := "2026-07-10"
	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{
		AssetID: assetID, ToOfficeID: h.toOffice, ConditionSent: &cond, TransferDate: &date,
	})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, row.ConditionSent)
	assert.Equal(t, sqlc.SharedTransferConditionRusakRingan, *row.ConditionSent)
	require.True(t, row.TransferDate.Valid)
	assert.Equal(t, "2026-07-10", row.TransferDate.Time.Format("2006-01-02"))
}

// TestTransfer_RejectReceive covers the destination office declining an in-transit
// shipment: the row terminates as 'returned' with a note, and the asset never moves.
func TestTransfer_RejectReceive(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	t.Run("happy path: returned, note stored, asset stays at origin", func(t *testing.T) {
		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00009", "Scanner Fujitsu", h.catID, h.fromOffice, "500000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.ret1@test.local")
		checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.ret1@test.local")
		receiver := seedUser(t, h.pool, h.officeRoleID, h.toOffice, "receiver.ret1@test.local")

		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
		checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

		req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
		require.NoError(t, err)
		final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
		require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

		row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
		require.NoError(t, err)
		_, err = h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
		require.NoError(t, err)

		note := "kondisi tidak sesuai"
		out, err := h.tsvc.RejectReceive(ctx, true, nil, receiver, row.ID, &note)
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedTransferStatusReturned, out.Status)
		require.NotNil(t, out.ReturnNote)
		assert.Equal(t, note, *out.ReturnNote)

		// Asset must still live at the origin office.
		a, err := h.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		assert.Equal(t, h.fromOffice, a.OfficeID)
	})

	t.Run("guard: only in_transit can be returned", func(t *testing.T) {
		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00010", "Switch Cisco", h.catID, h.fromOffice, "500000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.ret2@test.local")
		checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.ret2@test.local")

		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
		checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

		req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
		require.NoError(t, err)
		final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
		require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

		row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
		require.NoError(t, err)
		// still status=approved (not shipped)
		_, err = h.tsvc.RejectReceive(ctx, true, nil, maker, row.ID, nil)
		assert.ErrorIs(t, err, transfer.ErrInvalidState)
	})

	t.Run("guard: to-office scope enforced", func(t *testing.T) {
		assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00011", "UPS APC", h.catID, h.fromOffice, "500000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.ret3@test.local")
		checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.ret3@test.local")

		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
		checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

		req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
		require.NoError(t, err)
		final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
		require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

		row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
		require.NoError(t, err)
		_, err = h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
		require.NoError(t, err)

		// caller scoped ONLY to the from-office (not destination) → out of scope
		outsider := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "outsider.ret3@test.local")
		_, err = h.tsvc.RejectReceive(ctx, false, []uuid.UUID{h.fromOffice}, outsider, row.ID, nil)
		assert.ErrorIs(t, err, transfer.ErrOutOfScope)
	})
}

// TestTransfer_ListByAsset_History verifies that ListByAsset returns the
// asset's transfer(s), scoped by the caller's office IDs.
func TestTransfer_ListByAsset_History(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-2026-00007", "Asset History", h.catID, h.fromOffice, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.hist@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.hist@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("history")})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	// Global-scope caller sees the transfer.
	rows, err := h.tsvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 1)
	assert.Equal(t, assetID, rows[0].AssetID)

	// Scoped caller covering from_office also sees it.
	rows, err = h.tsvc.ListByAsset(ctx, assetID, false, []uuid.UUID{h.fromOffice})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 1)

	// Caller scoped to an unrelated office sees nothing.
	rows, err = h.tsvc.ListByAsset(ctx, assetID, false, []uuid.UUID{h.otherOffice})
	require.NoError(t, err)
	assert.Empty(t, rows, "caller scoped to an unrelated office must not see the transfer")
}
