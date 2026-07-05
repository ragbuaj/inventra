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
	"github.com/ragbuaj/inventra/internal/depreciation"
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

// seedAssetNoCost inserts a capitalized asset.assets row (status=available)
// with NO purchase_cost at all (NULL) and returns its id — used to exercise
// BookValueAsOf's "neither entries nor cost" -> "0" fallback.
func seedAssetNoCost(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', 'available')
		 RETURNING id`,
		tag, name, categoryID, officeID).Scan(&id))
	return id
}

// seedCommercialEntry inserts a depreciation.depreciation_entries row directly
// (basis=commercial) for the given asset/period/closing value, bypassing the
// full ComputePeriod engine — sufficient to exercise BookValueAsOf's
// "has entries" path from the disposal side.
func seedCommercialEntry(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID, period time.Time, opening, amount, closing string) {
	t.Helper()
	q := sqlc.New(pool)
	require.NoError(t, q.InsertDepreciationEntry(context.Background(), sqlc.InsertDepreciationEntryParams{
		AssetID:            assetID,
		Basis:              sqlc.SharedDepreciationBasisCommercial,
		Period:             pgtype.Date{Time: period, Valid: true},
		OpeningValue:       opening,
		DepreciationAmount: amount,
		ClosingValue:       closing,
		Method:             sqlc.SharedDepreciationMethodStraightLine,
	}))
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
	deprSvc      *depreciation.Service
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
	deprSvc := depreciation.NewService(q, pool)
	dsvc := disposal.NewService(q, pool, apprSvc, deprSvc)
	assetSvc := asset.NewService(q, pool, minioStore, 5<<20, "")
	apprSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, dsvc.Executor())

	officeRoleID := lookupRole(t, pool, "Kepala Unit")

	return &harness{
		pool:         pool,
		q:            q,
		apprSvc:      apprSvc,
		dsvc:         dsvc,
		assetSvc:     assetSvc,
		deprSvc:      deprSvc,
		office:       office,
		otherOffice:  otherOffice,
		officeRoleID: officeRoleID,
		catID:        catID,
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestDisposal_HappyPath_GainLoss drives submit → approve (single office-tier
// step, book value under the 5M band) and asserts the executor created the
// disposal row with the exact gain_loss string and flipped the asset to
// disposed. The asset has no depreciation entries, so book_value_at_disposal
// (and the approval amount) are server-computed as the purchase_cost
// fallback (spec 2026-07-05 decision #3) — not whatever the caller passes.
func TestDisposal_HappyPath_GainLoss(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00001", "Mesin Fotokopi", h.catID, h.office, "3000000.00")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.happy@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.happy@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "sale",
		DisposalDate: "2026-07-01",
		Proceeds:     strptr("3500000.00"),
		Reason:       strptr("dijual"),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	require.NotNil(t, req.Amount)
	assert.Equal(t, "3000000.00", *req.Amount, "approval amount must be the server-computed book value (purchase_cost fallback)")

	// No disposal row yet — executor only fires on final approval.
	_, err = h.q.GetDisposalByAsset(ctx, assetID)
	require.ErrorIs(t, err, pgx.ErrNoRows)

	finalReq := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, finalReq.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedDisposalMethodSale, row.Method)
	require.NotNil(t, row.Proceeds)
	assert.Equal(t, "3500000.00", *row.Proceeds)
	require.NotNil(t, row.BookValueAtDisposal)
	assert.Equal(t, "3000000.00", *row.BookValueAtDisposal, "book_value_at_disposal must be server-computed from purchase_cost, not caller-supplied")
	require.NotNil(t, row.GainLoss, "gain_loss must be computed when both proceeds and book_value are set")
	assert.Equal(t, "500000.00", *row.GainLoss)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusDisposed, a.Status)
}

// TestDisposal_GainLoss_NullWhenProceedsNil verifies gain_loss stays null
// when proceeds is not supplied (null-propagating numeric subtraction) —
// book_value_at_disposal is no longer the nilable side of that computation
// since Submit now always fills it (falling back to "0" when the asset has
// neither depreciation entries nor a purchase_cost).
func TestDisposal_GainLoss_NullWhenProceedsNil(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00002", "Kursi Kantor", h.catID, h.office, "1000000.00")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.nullgl@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.nullgl@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "write_off",
		DisposalDate: "2026-07-01",
		// Proceeds intentionally nil.
	})
	require.NoError(t, err)

	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, row.BookValueAtDisposal, "book_value_at_disposal is always server-computed, never nil")
	assert.Equal(t, "1000000.00", *row.BookValueAtDisposal)
	assert.Nil(t, row.GainLoss, "gain_loss must be null when proceeds is nil")
}

// TestDisposal_BookValue_NoCostNoEntries_FallsBackToZero verifies that an
// asset with neither depreciation entries nor a purchase_cost gets
// book_value_at_disposal "0" (BookValueAsOf's last-resort fallback), and that
// gain_loss then equals proceeds outright (proceeds - 0).
func TestDisposal_BookValue_NoCostNoEntries_FallsBackToZero(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetNoCost(t, h.pool, "DIS-2026-00014", "Aset Tanpa Biaya", h.catID, h.office)
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.nocost@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.nocost@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "write_off",
		DisposalDate: "2026-07-01",
		Proceeds:     strptr("500000.00"),
		Reason:       strptr("tidak ada biaya perolehan"),
	})
	require.NoError(t, err)
	require.NotNil(t, req.Amount)
	assert.Equal(t, "0", *req.Amount, "approval amount falls back to \"0\" when the asset has no entries and no purchase_cost")

	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, row.BookValueAtDisposal)
	assert.Equal(t, "0", *row.BookValueAtDisposal, "book_value_at_disposal carries BookValueAsOf's literal \"0\" fallback")
	require.NotNil(t, row.GainLoss)
	assert.Equal(t, "500000.00", *row.GainLoss, "gain_loss equals proceeds outright when book value falls back to zero")
}

// TestDisposal_BookValue_ComputedFromEntries verifies that when the asset has
// a commercial depreciation entry at or before the disposal month, both the
// approval amount and the eventual disposal row's book_value_at_disposal are
// that entry's closing_value — not the asset's purchase_cost.
func TestDisposal_BookValue_ComputedFromEntries(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00012", "Aset Dengan Entri Depresiasi", h.catID, h.office, "5000000.00")
	seedCommercialEntry(t, h.pool, assetID, mustParseDate(t, "2026-07-01"), "4600000.00", "400000.00", "4200000.00")

	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.entries@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.entries@test.local")
	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "sale",
		DisposalDate: "2026-07-15",
		Proceeds:     strptr("4500000.00"),
		Reason:       strptr("dijual dengan entri depresiasi"),
	})
	require.NoError(t, err)
	require.NotNil(t, req.Amount)
	assert.Equal(t, "4200000.00", *req.Amount, "approval amount must be the last commercial closing, not purchase_cost")

	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, row.BookValueAtDisposal)
	assert.Equal(t, "4200000.00", *row.BookValueAtDisposal, "book_value_at_disposal must be the last commercial closing as of the disposal month")
	require.NotNil(t, row.GainLoss)
	assert.Equal(t, "300000.00", *row.GainLoss)
}

// TestDisposal_BookValue_MakerCannotInject verifies that a caller-supplied
// book value never reaches the approval amount or the disposal row: even
// constructing SubmitInput.BookValue directly (a stronger attempt than the
// HTTP path, which no longer has the field at all — see dto.go) is
// unconditionally overwritten by Submit's server-side computation.
func TestDisposal_BookValue_MakerCannotInject(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00013", "Aset Anti Injeksi", h.catID, h.office, "1200000.00")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.inject@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.inject@test.local")
	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	injected := "999999999.00"
	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID:      assetID,
		Method:       "sale",
		DisposalDate: "2026-07-01",
		Proceeds:     strptr("1300000.00"),
		BookValue:    strptr(injected),
		Reason:       strptr("percobaan injeksi nilai buku"),
	})
	require.NoError(t, err)
	require.NotNil(t, req.Amount)
	assert.NotEqual(t, injected, *req.Amount, "the caller-supplied book value must never reach the approval amount")
	assert.Equal(t, "1200000.00", *req.Amount, "the approval amount must be server-computed from the purchase_cost fallback")

	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, row.BookValueAtDisposal)
	assert.NotEqual(t, injected, *row.BookValueAtDisposal)
	assert.Equal(t, "1200000.00", *row.BookValueAtDisposal, "book_value_at_disposal must be server-computed, not the caller-supplied value")
}

// TestDisposal_Submit_MalformedDisposalDate verifies Submit rejects a
// disposal_date that does not parse as "2006-01-02" with ErrInvalidRef —
// BEFORE any approval request is opened (the book-value computation needs a
// valid as-of date, so the parse guard runs ahead of appr.Submit).
func TestDisposal_Submit_MalformedDisposalDate(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00015", "Aset Tanggal Rusak", h.catID, h.office, "1000000.00")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.baddate@test.local")
	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})

	for _, tc := range []struct {
		name string
		date string
	}{
		{"Garbage", "not-a-date"},
		{"WrongFormat_DDMMYYYY", "01/07/2026"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
				AssetID:      assetID,
				Method:       "write_off",
				DisposalDate: tc.date,
				Reason:       strptr("tanggal tidak valid"),
			})
			require.ErrorIs(t, err, disposal.ErrInvalidRef)

			// No approval request may have been opened for the asset — count
			// ALL requests (any status) directly, stronger than the
			// pending-only sqlc guard query.
			var reqCount int64
			require.NoError(t, h.pool.QueryRow(ctx,
				`SELECT count(*) FROM approval.requests WHERE target_id = $1 AND deleted_at IS NULL`,
				assetID).Scan(&reqCount))
			assert.EqualValues(t, 0, reqCount, "a malformed disposal_date must not open an approval request")
		})
	}

	// And of course no disposal row either.
	_, err := h.q.GetDisposalByAsset(ctx, assetID)
	require.ErrorIs(t, err, pgx.ErrNoRows)
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

// TestDisposal_Submit_Guard_DisposalExists_LiveRowDirect verifies the
// GetDisposalByAsset guard in Submit fires ErrDisposalExists when a live
// disposal row already exists for an asset that is (artificially, for test
// purposes) still `available`. In normal flow this state is unreachable
// because creating a disposal row always flips the asset to `disposed`,
// which would trip ErrAlreadyDisposed first; here we seed the row directly
// via q.CreateDisposal without touching asset status, to exercise the guard
// on its own.
func TestDisposal_Submit_Guard_DisposalExists_LiveRowDirect(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00010", "Inkonsisten Live Row", h.catID, h.office, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.liverow@test.local")
	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})

	// Seed a live disposal row directly, leaving the asset status untouched
	// (still `available`) — the intentionally inconsistent state the guard
	// defends against.
	disposalDate := pgtype.Date{Time: mustParseDate(t, "2026-06-01"), Valid: true}
	_, err := h.q.CreateDisposal(ctx, sqlc.CreateDisposalParams{
		AssetID:      assetID,
		Method:       sqlc.SharedDisposalMethodWriteOff,
		DisposalDate: disposalDate,
	})
	require.NoError(t, err)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedAssetStatusAvailable, a.Status, "precondition: asset must still be available")

	_, err = h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID: assetID, Method: "sale", DisposalDate: "2026-07-01",
	})
	require.ErrorIs(t, err, disposal.ErrDisposalExists)
}

// TestDisposal_Executor_Guard_ErrConflict_PreexistingRow verifies the
// executor's own GetDisposalByAsset guard (defense-in-depth against a live
// disposal row it did not expect) fires approval.ErrConflict on final
// approval, and that the transaction rolls back cleanly: no second disposal
// row is created and the asset status is left unchanged. As with the
// service-level guard above, this state (a live disposal row on an
// `available` asset) is only reachable by seeding it directly — in normal
// flow the asset would already be `disposed` and Submit's own
// ErrAlreadyDisposed/ErrDisposalExists guards would reject the second
// request before an approval chain could even be opened.
func TestDisposal_Executor_Guard_ErrConflict_PreexistingRow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DIS-2026-00011", "Inkonsisten Executor", h.catID, h.office, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.execconflict@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.execconflict@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	// Submit a legitimate request FIRST, while the asset has no disposal row
	// yet — Submit's own guards pass normally.
	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID: assetID, Method: "write_off", DisposalDate: "2026-07-01", Reason: strptr("conflict test"),
	})
	require.NoError(t, err)

	// Now seed a live disposal row directly, out from under the pending
	// request, without touching asset status — reproducing the inconsistent
	// state the executor's own guard defends against.
	disposalDate := pgtype.Date{Time: mustParseDate(t, "2026-06-01"), Valid: true}
	_, err = h.q.CreateDisposal(ctx, sqlc.CreateDisposalParams{
		AssetID:      assetID,
		Method:       sqlc.SharedDisposalMethodWriteOff,
		DisposalDate: disposalDate,
	})
	require.NoError(t, err)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedAssetStatusAvailable, a.Status, "precondition: asset must still be available")

	// Drive the (single, office-tier) decision step directly so we can
	// assert on the error Decide returns — approveThroughChain calls
	// require.NoError on every Decide and would fail the test here instead.
	_, err = h.apprSvc.Decide(ctx, req.ID, checkerCaller, true, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, approval.ErrConflict)

	// The executor's tx must have rolled back: still exactly one disposal
	// row (the one we seeded directly) and the asset status unchanged.
	rows, err := h.q.ListDisposalsByAssetEnriched(ctx, sqlc.ListDisposalsByAssetEnrichedParams{
		AssetID: assetID, AllScope: true, OfficeIds: []uuid.UUID{},
	})
	require.NoError(t, err)
	assert.Len(t, rows, 1, "executor rollback must not leave a second disposal row")

	a, err = h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, a.Status, "executor rollback must leave asset status unchanged")
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
	assert.Equal(t, row.ID, got.DisposalDisposal.ID)

	// Get: out-of-scope caller gets not found.
	_, err = h.dsvc.Get(ctx, row.ID, false, []uuid.UUID{h.otherOffice})
	require.ErrorIs(t, err, disposal.ErrNotFound)

	// List: in-scope caller sees it.
	rows, total, err := h.dsvc.List(ctx, false, []uuid.UUID{h.office}, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	assert.Equal(t, row.ID, rows[0].DisposalDisposal.ID)

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

// TestDisposal_EnrichedReads verifies List/Get/ListByAsset return resolved
// asset/office/actor display names alongside the raw disposal columns.
// asset_name/asset_tag come from the INNER-joined asset row (never nil, per
// the disposal-scope join), while office_name/created_by_name are LEFT-joined
// and exposed as *string.
func TestDisposal_EnrichedReads(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetWithCost(t, h.pool, "DSP-ENR-1", "Printer HP Lama", h.catID, h.office, "2000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.office, "maker.dspenr@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.office, "checker.dspenr@test.local")
	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.office})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.office})

	req, err := h.dsvc.Submit(ctx, makerCaller, disposal.SubmitInput{
		AssetID: assetID, Method: "sale", DisposalDate: "2026-07-05",
	})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	// List: enriched names resolved for the caller's scope.
	rows, total, err := h.dsvc.List(ctx, true, nil, 20, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, rows)
	assert.Equal(t, "Printer HP Lama", rows[0].AssetName)
	assert.Equal(t, "DSP-ENR-1", rows[0].AssetTag)
	require.NotNil(t, rows[0].OfficeName)
	assert.Equal(t, "Disposal Office", *rows[0].OfficeName)
	require.NotNil(t, rows[0].CreatedByName)
	assert.Equal(t, "maker.dspenr@test.local", *rows[0].CreatedByName)

	// Get: same enrichment for a single row.
	got, err := h.dsvc.Get(ctx, rows[0].DisposalDisposal.ID, true, nil)
	require.NoError(t, err)
	assert.Equal(t, "Printer HP Lama", got.AssetName)
	assert.Equal(t, "DSP-ENR-1", got.AssetTag)
	require.NotNil(t, got.OfficeName)
	assert.Equal(t, "Disposal Office", *got.OfficeName)
	require.NotNil(t, got.CreatedByName)
	assert.Equal(t, "maker.dspenr@test.local", *got.CreatedByName)

	// ListByAsset: same enrichment scoped to the asset's disposal history.
	history, err := h.dsvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "Printer HP Lama", history[0].AssetName)
	require.NotNil(t, history[0].CreatedByName)
	assert.Equal(t, "maker.dspenr@test.local", *history[0].CreatedByName)
}
