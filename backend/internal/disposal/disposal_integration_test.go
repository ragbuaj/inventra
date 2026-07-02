//go:build integration

package disposal_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/disposal"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func strptr(s string) *string { return &s }

// mustParseDate parses a "2006-01-02" date string, failing the test on error.
func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return d
}

// resetAll truncates the mutable schemas touched by disposal tests. Each test
// gets its own throwaway container (testsupport.NewPostgres), so this mostly
// guards against any shared-pool scenarios while leaving migration-seeded
// identity rows (roles, scope policies, thresholds) intact.
func resetAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 disposal.disposals, asset.asset_documents,
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
// request row. The seeded asset_disposal band under 5M gives exactly one
// office-tier step, so this loop should resolve on the first Decide.
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

// harness bundles everything a disposal test needs: pool, sqlc queries, the
// approval + disposal + asset services (with the asset_disposal executor
// registered), and an office pair (asset office + an unrelated office) for
// scope assertions.
type harness struct {
	pool         *pgxpool.Pool
	q            *sqlc.Queries
	apprSvc      *approval.Service
	dsvc         *disposal.Service
	assetSvc     *asset.Service
	office       uuid.UUID
	otherOffice  uuid.UUID
	officeRoleID uuid.UUID
	catID        uuid.UUID
}

// newHarness boots a throwaway Postgres + Redis + MinIO, resets mutable tables,
// and wires the approval/disposal/asset services with the asset_disposal
// executor registered. Seeds a single office (in its own subtree) plus a
// second, unrelated office for out-of-scope assertions.
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	minioStore := testsupport.NewMinIO(t)
	resetAll(t, pool)

	parent := seedOfficeWithType(t, pool, "DisposalParentType-"+uuid.New().String()[:8], "DPX")
	office := seedOfficeChild(t, pool, parent, "Disposal Office", "DIS"+uuid.New().String()[:4])
	otherOffice := seedOfficeWithType(t, pool, "OtherType-"+uuid.New().String()[:8], "OTH"+uuid.New().String()[:4])

	catID := seedCategory(t, pool, "DSP"+uuid.New().String()[:4])

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	apprSvc := approval.NewService(q, pool, scopeSvc, rdb)
	dsvc := disposal.NewService(q, pool, apprSvc)
	assetSvc := asset.NewService(q, pool, minioStore, 5<<20, "")
	apprSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, dsvc.Executor())

	officeRoleID := lookupRole(t, pool, "Kepala Unit")

	return &harness{
		pool:         pool,
		q:            q,
		apprSvc:      apprSvc,
		dsvc:         dsvc,
		assetSvc:     assetSvc,
		office:       office,
		otherOffice:  otherOffice,
		officeRoleID: officeRoleID,
		catID:        catID,
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestDisposal_HappyPath_GainLoss drives submit → approve (single office-tier
// step, purchase_cost under the 5M band) and asserts the executor created the
// disposal row with the exact gain_loss string and flipped the asset to
// disposed.
func TestDisposal_HappyPath_GainLoss(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00001", "Mesin Fotokopi", h.catID, h.office, "2000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.happy@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.happy@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "sale",
		DisposalDate: "2026-07-01",
		Proceeds:     strptr("120000000.00"),
		BookValue:    strptr("100000000.00"),
		Reason:       strptr("dijual"),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)

	// No disposal row yet — executor only fires on final approval.
	_, err = h.q.GetDisposalByAsset(ctx, assetID)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	finalReq := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, finalReq.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedDisposalMethodSale, row.Method)
	require.NotNil(t, row.Proceeds)
	assert.Equal(t, "120000000.00", *row.Proceeds)
	require.NotNil(t, row.BookValueAtDisposal)
	assert.Equal(t, "100000000.00", *row.BookValueAtDisposal)
	require.NotNil(t, row.GainLoss, "gain_loss must be computed when both proceeds and book_value are set")
	assert.Equal(t, "20000000.00", *row.GainLoss)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusDisposed, a.Status)
}

// TestDisposal_GainLoss_NullWhenBookValueNil verifies gain_loss is null when
// book_value_at_disposal is not supplied (null-propagating numeric subtraction).
func TestDisposal_GainLoss_NullWhenBookValueNil(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00002", "Kursi Kantor", h.catID, h.office, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.nullgl@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.nullgl@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "write_off",
		DisposalDate: "2026-07-01",
		Proceeds:     strptr("0.00"),
		// BookValue intentionally nil.
	})
	require.NoError(t, err)

	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Nil(t, row.GainLoss, "gain_loss must be null when book_value_at_disposal is nil")
}

// TestDisposal_Reject_NoDisposalRow verifies that rejecting the final step
// finalises the request as rejected, creates NO disposal row, and leaves the
// asset's status unchanged.
func TestDisposal_Reject_NoDisposalRow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00003", "Printer Rusak", h.catID, h.office, "1500000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.reject@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.reject@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "write_off",
		DisposalDate: "2026-07-01",
		Reason:       strptr("rusak berat"),
	})
	require.NoError(t, err)

	final := rejectFinalStep(t, h.apprSvc, req.ID, checkerCaller)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, final.Status)

	_, err = h.q.GetDisposalByAsset(ctx, assetID)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, a.Status, "asset status must be unchanged after rejection")
}

// TestDisposal_Submit_Guards covers the submit-time validation guards: an
// already-disposed asset, an existing disposal, and an out-of-scope caller.
func TestDisposal_Submit_Guards(t *testing.T) {
	t.Run("AlreadyDisposed", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00004", "Sudah Dihapus", h.catID, h.office, "1000000")
		_, err := h.pool.Exec(ctx, `UPDATE asset.assets SET status = 'disposed' WHERE id = $1`, assetID)
		require.NoError(t, err)

		maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.alreadydisposed@test.local")
		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})

		_, err = h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
			AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01",
		})
		require.ErrorIs(t, err, disposal.ErrAlreadyDisposed)
	})

	t.Run("DisposalExists_ApprovedRow", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00005", "Sudah Ada Disposal", h.catID, h.office, "1000000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.exists@test.local")
		checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.exists@test.local")

		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
		checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

		// First submit + approve → a live disposal row now exists.
		req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
			AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01", Reason: strptr("first"),
		})
		require.NoError(t, err)
		final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
		require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

		// Second submit for the same (now-disposed) asset must fail. The asset is
		// already disposed, so ErrAlreadyDisposed fires before the disposal-exists check.
		_, err = h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
			AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01", Reason: strptr("second"),
		})
		require.ErrorIs(t, err, disposal.ErrAlreadyDisposed)
	})

	t.Run("DisposalExists_PendingRequest", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00006", "Pending Disposal", h.catID, h.office, "1000000")
		maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.pending@test.local")
		makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})

		// First submit leaves a pending request (not yet approved) — asset status
		// is still available, so a second submit must be rejected by the
		// pending-request guard rather than ErrAlreadyDisposed.
		_, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
			AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01", Reason: strptr("first"),
		})
		require.NoError(t, err)

		_, err = h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
			AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01", Reason: strptr("second"),
		})
		require.ErrorIs(t, err, disposal.ErrDisposalExists)
	})

	t.Run("OutOfScope_WhenCallerLacksAssetOffice", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()

		assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00007", "Meja Kantor", h.catID, h.office, "1000000")
		outsideUser := seedUser(t, h.pool, h.officeRoleID, h.otherOffice, "outside.submit@test.local")
		outsideCaller := buildCaller(outsideUser, h.officeRoleID, false, []uuid.UUID{h.otherOffice})

		_, err := h.dsvc.Submit(ctx, outsideCaller, disposal.SubmitInput{
			AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01", Reason: strptr("oos"),
		})
		require.ErrorIs(t, err, disposal.ErrOutOfScope)
	})
}

// TestDisposal_Scope_Reads verifies Get/List respect caller office scope: a
// caller scoped to the asset's office sees the row; a caller scoped to an
// unrelated office does not.
func TestDisposal_Scope_Reads(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00008", "Laptop Bekas", h.catID, h.office, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.scope@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.scope@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID: assetID, Method: "sale", DisposalDate: "2026-07-01",
		Proceeds: strptr("500000.00"), Reason: strptr("scope test"),
	})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)

	// Get: in-scope caller sees the row.
	got, err := h.dsvc.Get(ctx, row.ID, false, []uuid.UUID{h.office})
	require.NoError(t, err)
	assert.Equal(t, row.ID, got.ID)

	// Get: out-of-scope caller gets not found.
	_, err = h.dsvc.Get(ctx, row.ID, false, []uuid.UUID{h.otherOffice})
	require.ErrorIs(t, err, disposal.ErrNotFound)

	// List: in-scope caller sees it.
	rows, total, err := h.dsvc.List(ctx, false, []uuid.UUID{h.office}, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	assert.Equal(t, row.ID, rows[0].ID)

	// List: out-of-scope caller sees nothing.
	rows, total, err = h.dsvc.List(ctx, false, []uuid.UUID{h.otherOffice}, 20, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
	assert.Empty(t, rows)

	// Global scope caller also sees it.
	rows, total, err = h.dsvc.List(ctx, true, nil, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
}

// TestDisposal_BAST_DocumentAndBastNo verifies that, after a disposal exists,
// invoking asset.Service.CreateDocument the same way the handler's
// attachDocument does produces an asset_documents row with
// doc_type=bast_disposal and related_disposal_id set; attaching a file sets
// object_key; and SetDisposalBastNo persists bast_no on the disposal row.
func TestDisposal_BAST_DocumentAndBastNo(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00009", "Asset BAST", h.catID, h.office, "2000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.bast@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.bast@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID: assetID, Method: "sale", DisposalDate: "2026-07-01",
		Proceeds: strptr("1000000.00"), BookValue: strptr("800000.00"), Reason: strptr("bast"),
	})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)

	// Mirror the handler's attachDocument: create the asset_documents(bast_disposal) row.
	disposalID := row.ID
	docDate := pgtype.Date{Time: mustParseDate(t, "2026-07-02"), Valid: true}
	doc, err := h.assetSvc.CreateDocument(ctx, asset.DocumentInput{
		AssetID:           assetID,
		DocType:           sqlc.SharedAssetDocumentTypeBastDisposal,
		DocNo:             strptr("BAST-DISP-001"),
		DocDate:           docDate,
		RelatedRequestID:  row.RequestID,
		RelatedDisposalID: &disposalID,
		CreatedBy:         maker,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetDocumentTypeBastDisposal, doc.DocType)

	// Attach a small file, same as the handler's best-effort file upload.
	fileData := []byte("%PDF-1.4 fake bast content")
	updated, err := h.assetSvc.AttachFile(ctx, doc, asset.DocumentFileInput{
		ContentType: "application/pdf", Data: fileData,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.ObjectKey)

	docs, err := h.q.ListAssetDocuments(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, sqlc.SharedAssetDocumentTypeBastDisposal, docs[0].DocType)
	require.NotNil(t, docs[0].RelatedDisposalID)
	assert.Equal(t, disposalID, *docs[0].RelatedDisposalID)
	require.NotNil(t, docs[0].ObjectKey, "object_key must be set after file attach")
	assert.NotEmpty(t, *docs[0].ObjectKey)

	// SetDisposalBastNo persists bast_no on the disposal.
	updatedDisposal, err := h.q.SetDisposalBastNo(ctx, sqlc.SetDisposalBastNoParams{
		ID: disposalID, BastNo: strptr("BAST-DISP-001"),
	})
	require.NoError(t, err)
	require.NotNil(t, updatedDisposal.BastNo)
	assert.Equal(t, "BAST-DISP-001", *updatedDisposal.BastNo)

	// Re-fetch to confirm persistence beyond the returned row.
	refetched, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, refetched.BastNo)
	assert.Equal(t, "BAST-DISP-001", *refetched.BastNo)
}
