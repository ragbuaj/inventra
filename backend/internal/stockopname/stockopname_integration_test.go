//go:build integration

package stockopname_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/depreciation"
	"github.com/ragbuaj/inventra/internal/disposal"
	"github.com/ragbuaj/inventra/internal/stockopname"
	"github.com/ragbuaj/inventra/internal/testsupport"
	"github.com/ragbuaj/inventra/internal/transfer"
)

// ─── helpers ────────────────────────────────────────────────────────────────

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

// seedAsset inserts an asset.assets row with the given status and returns its id.
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

// harness bundles everything a stockopname test needs.
type harness struct {
	pool         *pgxpool.Pool
	q            *sqlc.Queries
	svc          *stockopname.Service
	officeA      uuid.UUID
	officeB      uuid.UUID
	officeRoleID uuid.UUID
	catID        uuid.UUID
}

// newHarness boots a throwaway Postgres + Redis, wires the stockopname service
// (with disposal/transfer deps for future tasks), and seeds two unrelated
// offices (A, B) plus a shared category.
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	officeA := seedOfficeWithType(t, pool, "OpnameTypeA-"+uuid.New().String()[:8], "OPA"+uuid.New().String()[:4])
	officeB := seedOfficeWithType(t, pool, "OpnameTypeB-"+uuid.New().String()[:8], "OPB"+uuid.New().String()[:4])
	catID := seedCategory(t, pool, "OPN"+uuid.New().String()[:4])

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	apprSvc := approval.NewService(q, pool, scopeSvc, rdb)
	deprSvc := depreciation.NewService(q, pool)
	dispSvc := disposal.NewService(q, pool, apprSvc, deprSvc)
	trSvc := transfer.NewService(q, pool, apprSvc)
	svc := stockopname.NewService(q, pool, dispSvc, trSvc)

	officeRoleID := lookupRole(t, pool, "Kepala Unit")

	return &harness{
		pool:         pool,
		q:            q,
		svc:          svc,
		officeA:      officeA,
		officeB:      officeB,
		officeRoleID: officeRoleID,
		catID:        catID,
	}
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestCreateSessionSnapshotsInScopeAssets verifies that CreateSession opens a
// session and snapshots exactly the non-deleted, non-disposed assets of the
// target office — a disposed asset in the same office is excluded.
func TestCreateSessionSnapshotsInScopeAssets(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	seedAsset(t, h.pool, "OPN-A-001", "Laptop 1", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	seedAsset(t, h.pool, "OPN-A-002", "Laptop 2", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	seedAsset(t, h.pool, "OPN-A-003", "Laptop 3", h.catID, h.officeA, sqlc.SharedAssetStatusAssigned)
	seedAsset(t, h.pool, "OPN-A-004", "Laptop Disposed", h.catID, h.officeA, sqlc.SharedAssetStatusDisposed)

	maker := seedUser(t, h.pool, h.officeRoleID, h.officeA, "maker.snapshot@test.local")
	caller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.officeA})

	period := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	sess, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
		OfficeID: h.officeA,
		Period:   period,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedOpnameSessionStatusOpen, sess.Status)
	assert.Equal(t, h.officeA, sess.OfficeID)

	items, err := h.q.ListOpnameItemsEnriched(ctx, sqlc.ListOpnameItemsEnrichedParams{SessionID: sess.ID})
	require.NoError(t, err)
	require.Len(t, items, 3, "disposed asset must not be snapshotted")
	for _, it := range items {
		assert.Equal(t, sqlc.SharedOpnameItemResultPending, it.StockopnameStockOpnameItem.Result)
		assert.True(t, it.StockopnameStockOpnameItem.Expected)
	}
}

// TestCreateSessionOutOfScopeRejected verifies a caller scoped only to office B
// cannot open a session for office A.
func TestCreateSessionOutOfScopeRejected(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	maker := seedUser(t, h.pool, h.officeRoleID, h.officeB, "maker.oos@test.local")
	caller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.officeB})

	_, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
		OfficeID: h.officeA,
		Period:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	require.ErrorIs(t, err, stockopname.ErrOutOfScope)
}

// TestSessionStateMachineLegalAndIllegal drives the legal open→counting→
// reconciling→closed chain (closed stamps closed_by/at), and asserts the
// illegal shortcuts (open→closed, closed→counting) are rejected.
func TestSessionStateMachineLegalAndIllegal(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	seedAsset(t, h.pool, "OPN-SM-001", "Aset SM", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	maker := seedUser(t, h.pool, h.officeRoleID, h.officeA, "maker.sm@test.local")
	caller := buildCaller(maker, h.officeRoleID, true, nil)

	sess, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
		OfficeID: h.officeA,
		Period:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedOpnameSessionStatusOpen, sess.Status)

	t.Run("open to counting", func(t *testing.T) {
		out, err := h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusCounting)
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedOpnameSessionStatusCounting, out.Status)
	})

	t.Run("counting to reconciling", func(t *testing.T) {
		out, err := h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusReconciling)
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedOpnameSessionStatusReconciling, out.Status)
	})

	t.Run("reconciling to closed stamps closed_by/at", func(t *testing.T) {
		out, err := h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusClosed)
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedOpnameSessionStatusClosed, out.Status)
		require.NotNil(t, out.ClosedByID)
		assert.Equal(t, maker, *out.ClosedByID)
		assert.True(t, out.ClosedAt.Valid)
	})

	t.Run("illegal open to closed", func(t *testing.T) {
		sess2, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
			OfficeID: h.officeA,
			Period:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		})
		require.NoError(t, err)
		_, err = h.svc.Transition(ctx, caller, sess2.ID, sqlc.SharedOpnameSessionStatusClosed)
		require.ErrorIs(t, err, stockopname.ErrInvalidState)
	})

	t.Run("illegal closed to counting", func(t *testing.T) {
		_, err := h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusCounting)
		require.ErrorIs(t, err, stockopname.ErrInvalidState)
	})
}

// TestKpisCountByResult verifies GetSession's KPI counts (total/found/pending/
// variance) reflect the item results after some are set.
func TestKpisCountByResult(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	seedAsset(t, h.pool, "OPN-KPI-001", "Aset KPI 1", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	seedAsset(t, h.pool, "OPN-KPI-002", "Aset KPI 2", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	seedAsset(t, h.pool, "OPN-KPI-003", "Aset KPI 3", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	seedAsset(t, h.pool, "OPN-KPI-004", "Aset KPI 4", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)

	maker := seedUser(t, h.pool, h.officeRoleID, h.officeA, "maker.kpi@test.local")
	caller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.officeA})

	sess, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
		OfficeID: h.officeA,
		Period:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	items, err := h.q.ListOpnameItemsEnriched(ctx, sqlc.ListOpnameItemsEnrichedParams{SessionID: sess.ID})
	require.NoError(t, err)
	require.Len(t, items, 4)

	// Mark item 1 as found, item 2 as not_found, item 3 as damaged; leave item 4 pending.
	_, err = h.q.SetOpnameItemResult(ctx, sqlc.SetOpnameItemResultParams{
		Result: sqlc.SharedOpnameItemResultFound, CountedByID: &maker,
		ID: items[0].StockopnameStockOpnameItem.ID, SessionID: sess.ID,
	})
	require.NoError(t, err)
	_, err = h.q.SetOpnameItemResult(ctx, sqlc.SetOpnameItemResultParams{
		Result: sqlc.SharedOpnameItemResultNotFound, CountedByID: &maker,
		ID: items[1].StockopnameStockOpnameItem.ID, SessionID: sess.ID,
	})
	require.NoError(t, err)
	_, err = h.q.SetOpnameItemResult(ctx, sqlc.SetOpnameItemResultParams{
		Result: sqlc.SharedOpnameItemResultDamaged, CountedByID: &maker,
		ID: items[2].StockopnameStockOpnameItem.ID, SessionID: sess.ID,
	})
	require.NoError(t, err)

	_, kpi, err := h.svc.GetSession(ctx, caller, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(4), kpi.Total)
	assert.Equal(t, int64(1), kpi.Found)
	assert.Equal(t, int64(1), kpi.Pending)
	assert.Equal(t, int64(2), kpi.Variance)
}

// TestSetItemResultOnlyWhenCounting verifies SetItemResult is rejected unless
// the session is in 'counting' (both before start and after the session has
// moved on to 'reconciling'), and succeeds — stamping counted_by/at — while
// counting.
func TestSetItemResultOnlyWhenCounting(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	seedAsset(t, h.pool, "OPN-SIR-001", "Aset SIR", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	maker := seedUser(t, h.pool, h.officeRoleID, h.officeA, "maker.sir@test.local")
	caller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.officeA})

	sess, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
		OfficeID: h.officeA,
		Period:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	items, err := h.q.ListOpnameItemsEnriched(ctx, sqlc.ListOpnameItemsEnrichedParams{SessionID: sess.ID})
	require.NoError(t, err)
	require.Len(t, items, 1)
	itemID := items[0].StockopnameStockOpnameItem.ID

	t.Run("rejected while open", func(t *testing.T) {
		_, err := h.svc.SetItemResult(ctx, caller, sess.ID, itemID, sqlc.SharedOpnameItemResultFound, nil)
		require.ErrorIs(t, err, stockopname.ErrInvalidState)
	})

	_, err = h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusCounting)
	require.NoError(t, err)

	t.Run("succeeds while counting", func(t *testing.T) {
		row, err := h.svc.SetItemResult(ctx, caller, sess.ID, itemID, sqlc.SharedOpnameItemResultFound, nil)
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedOpnameItemResultFound, row.Result)
		require.NotNil(t, row.CountedByID)
		assert.Equal(t, maker, *row.CountedByID)
		assert.True(t, row.CountedAt.Valid)
	})

	_, err = h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusReconciling)
	require.NoError(t, err)

	t.Run("rejected while reconciling (locked)", func(t *testing.T) {
		_, err := h.svc.SetItemResult(ctx, caller, sess.ID, itemID, sqlc.SharedOpnameItemResultNotFound, nil)
		require.ErrorIs(t, err, stockopname.ErrInvalidState)
	})
}

// TestScanAddsUnexpectedInScopeAsset verifies Scan inserts an expected=false,
// pending item for an in-scope asset not in the session snapshot, returns the
// existing item (no duplicate) on a repeat scan, and rejects an out-of-scope
// asset's tag.
func TestScanAddsUnexpectedInScopeAsset(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Only one asset is present at session-open time; a second asset (moved in
	// after the snapshot) is added to office A later, plus one in office B.
	seedAsset(t, h.pool, "OPN-SCAN-001", "Aset Scan 1", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	maker := seedUser(t, h.pool, h.officeRoleID, h.officeA, "maker.scan@test.local")
	caller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.officeA})

	sess, err := h.svc.CreateSession(ctx, caller, stockopname.CreateInput{
		OfficeID: h.officeA,
		Period:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	unexpectedID := seedAsset(t, h.pool, "OPN-SCAN-002", "Aset Scan Unexpected", h.catID, h.officeA, sqlc.SharedAssetStatusAvailable)
	oosID := seedAsset(t, h.pool, "OPN-SCAN-003", "Aset Scan OOS", h.catID, h.officeB, sqlc.SharedAssetStatusAvailable)
	_ = oosID

	_, err = h.svc.Transition(ctx, caller, sess.ID, sqlc.SharedOpnameSessionStatusCounting)
	require.NoError(t, err)

	t.Run("adds unexpected in-scope asset", func(t *testing.T) {
		row, err := h.svc.Scan(ctx, caller, sess.ID, "OPN-SCAN-002")
		require.NoError(t, err)
		assert.Equal(t, unexpectedID, row.AssetID)
		assert.False(t, row.Expected)
		assert.Equal(t, sqlc.SharedOpnameItemResultPending, row.Result)
	})

	t.Run("repeat scan returns existing item, no duplicate", func(t *testing.T) {
		row, err := h.svc.Scan(ctx, caller, sess.ID, "OPN-SCAN-002")
		require.NoError(t, err)
		assert.Equal(t, unexpectedID, row.AssetID)

		items, err := h.q.ListOpnameItemsEnriched(ctx, sqlc.ListOpnameItemsEnrichedParams{SessionID: sess.ID})
		require.NoError(t, err)
		count := 0
		for _, it := range items {
			if it.StockopnameStockOpnameItem.AssetID == unexpectedID {
				count++
			}
		}
		assert.Equal(t, 1, count, "must not duplicate the item on repeat scan")
	})

	t.Run("out-of-scope asset tag rejected", func(t *testing.T) {
		_, err := h.svc.Scan(ctx, caller, sess.ID, "OPN-SCAN-003")
		require.ErrorIs(t, err, stockopname.ErrOutOfScope)
	})

	t.Run("unknown tag rejected", func(t *testing.T) {
		_, err := h.svc.Scan(ctx, caller, sess.ID, "OPN-SCAN-DOES-NOT-EXIST")
		require.ErrorIs(t, err, stockopname.ErrNoItem)
	})
}
