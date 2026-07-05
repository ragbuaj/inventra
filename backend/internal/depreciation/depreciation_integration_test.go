//go:build integration

package depreciation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/depreciation"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// resetAll truncates the mutable schemas touched by depreciation tests. Each
// test gets its own throwaway container (testsupport.NewPostgres), so this
// mostly guards against any shared-pool scenarios while leaving
// migration-seeded identity rows (roles, scope policies, permissions) intact.
func resetAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`TRUNCATE depreciation.depreciation_entries, depreciation.depreciation_periods,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)
}

// firstOfMonthUTC normalizes t to the first day of its month at UTC midnight.
func firstOfMonthUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
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

// seedCategory inserts a masterdata.categories row (intangible, so tests don't
// need to seed rooms) with the given SL-method depreciation defaults
// (life in months, salvage rate as a ratio, fiscal group) and returns its id.
func seedCategory(t *testing.T, pool *pgxpool.Pool, code string, lifeMonths int32, salvageRate string, fiscalGroup sqlc.SharedFiscalAssetGroup) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.categories
		   (name, code, asset_class, default_depreciation_method, default_useful_life_months,
		    default_salvage_rate, default_fiscal_group)
		 VALUES ($1, $2, 'intangible', 'straight_line', $3, $4, $5)
		 RETURNING id`,
		code, code, lifeMonths, salvageRate, fiscalGroup).Scan(&id))
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

// seedAsset inserts a capitalized asset.assets row (status=available) with the
// given purchase_date + purchase_cost and returns its id.
func seedAsset(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, cost string, purchaseDate time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status,
		    purchase_date, purchase_cost)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', 'available', $5, $6)
		 RETURNING id`,
		tag, name, categoryID, officeID, purchaseDate, cost).Scan(&id))
	return id
}

// seedAssetMonthsAgo is seedAsset with purchase_date computed as N whole
// calendar months before the current month.
func seedAssetMonthsAgo(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, cost string, monthsAgo int) uuid.UUID {
	t.Helper()
	purchaseDate := firstOfMonthUTC(time.Now()).AddDate(0, -monthsAgo, 0)
	return seedAsset(t, pool, tag, name, categoryID, officeID, cost, purchaseDate)
}

// seedAssetNoCost inserts a capitalized asset with NO purchase_cost/purchase_date
// (the "no_cost" skip reason) and returns its id.
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

// seedAssetNotCapitalized inserts a non-capitalized asset (capitalized=false,
// the "not_capitalized" skip reason) with a valid cost/purchase_date and
// returns its id.
func seedAssetNotCapitalized(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, cost string, monthsAgo int) uuid.UUID {
	t.Helper()
	purchaseDate := firstOfMonthUTC(time.Now()).AddDate(0, -monthsAgo, 0)
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status,
		    purchase_date, purchase_cost)
		 VALUES ($1, $2, $3, $4, 'intangible', false, '{}', 'available', $5, $6)
		 RETURNING id`,
		tag, name, categoryID, officeID, purchaseDate, cost).Scan(&id))
	return id
}

// harness bundles everything a depreciation test needs: pool, sqlc queries,
// the service, an office, a category (SL 48m, 10% salvage, kelompok_1 fiscal),
// and a seeded actor user (for computed_by/closed_by FKs).
type harness struct {
	pool    *pgxpool.Pool
	q       *sqlc.Queries
	svc     *depreciation.Service
	office  uuid.UUID
	catID   uuid.UUID
	actorID uuid.UUID
	roleID  uuid.UUID
}

// newHarness boots a throwaway Postgres, resets mutable tables, and wires the
// depreciation service plus a single office/category/actor.
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	resetAll(t, pool)

	office := seedOfficeWithType(t, pool, "DeprType-"+uuid.New().String()[:8], "DPR"+uuid.New().String()[:4])
	catID := seedCategory(t, pool, "DPR"+uuid.New().String()[:4], 48, "0.10", sqlc.SharedFiscalAssetGroupKelompok1)

	q := sqlc.New(pool)
	roleID := lookupRole(t, pool, "Superadmin")
	actorID := seedUser(t, pool, roleID, office, "actor."+uuid.New().String()[:8]+"@test.local")

	svc := depreciation.NewService(q, pool)

	return &harness{pool: pool, q: q, svc: svc, office: office, catID: catID, actorID: actorID, roleID: roleID}
}

// ─── tests ──────────────────────────────────────────────────────────────────

// TestDepreciation_Compute_HappyPath seeds one asset (cost 18,500,000,
// purchased 3 months ago; category SL 48m/10% salvage/kelompok_1 fiscal) and
// computes the current month. Both bases must produce entries from the
// purchase month through the target month (4 months inclusive); the asset's
// commercial accumulated_depreciation/book_value must reflect the sum/last
// closing of the commercial entries; and the period row must land as
// `computed` with asset_count=1 and total_amount equal to the sum of the
// target month's commercial amounts (here, just this one asset's amount,
// since straight-line divides evenly: (18,500,000-1,850,000)/48 = 346,875.00
// with zero rounding drift across the first 4 months).
func TestDepreciation_Compute_HappyPath(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00001", "Lisensi ERP", h.catID, h.office, "18500000.00", 3)
	target := firstOfMonthUTC(time.Now())

	summary, err := h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)

	assert.Equal(t, 1, summary.AssetCount)
	assert.Equal(t, "346875.00", summary.TotalAmount)
	assert.Equal(t, 0, summary.SkippedCount)
	assert.Empty(t, summary.Skipped)

	entries, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)

	var commercial, fiscal []sqlc.DepreciationDepreciationEntry
	for _, e := range entries {
		switch e.Basis {
		case sqlc.SharedDepreciationBasisCommercial:
			commercial = append(commercial, e)
		case sqlc.SharedDepreciationBasisFiscal:
			fiscal = append(fiscal, e)
		}
	}
	require.Len(t, commercial, 4, "commercial entries: purchase month through target, inclusive")
	require.Len(t, fiscal, 4, "fiscal entries: purchase month through target, inclusive")

	for _, e := range commercial {
		assert.Equal(t, "346875.00", e.DepreciationAmount)
	}
	last := commercial[len(commercial)-1]
	assert.True(t, last.Period.Time.Equal(target))
	assert.Equal(t, "17112500.00", last.ClosingValue, "18,500,000 - 4*346,875")

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, "1387500.00", a.AccumulatedDepreciation)
	require.NotNil(t, a.BookValue)
	assert.Equal(t, "17112500.00", *a.BookValue)

	period, err := h.q.GetDepreciationPeriod(ctx, pgDate(target))
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedDepreciationPeriodStatusComputed, period.Status)
	assert.EqualValues(t, 1, period.AssetCount)
	assert.Equal(t, "346875.00", period.TotalAmount)
	assert.EqualValues(t, 0, period.SkippedCount)
}

// entryValues is the business-meaningful projection of a depreciation entry —
// everything EXCEPT its surrogate id/created_at/updated_at, which legitimately
// change across a delete+reinsert regeneration even when the schedule itself
// is unchanged (idempotency is a value-level guarantee, not row identity).
type entryValues struct {
	Basis   sqlc.SharedDepreciationBasis
	Period  time.Time
	Opening string
	Amount  string
	Closing string
	Method  sqlc.SharedDepreciationMethod
}

func valuesOf(entries []sqlc.DepreciationDepreciationEntry) []entryValues {
	out := make([]entryValues, len(entries))
	for i, e := range entries {
		out[i] = entryValues{
			Basis: e.Basis, Period: e.Period.Time, Opening: e.OpeningValue,
			Amount: e.DepreciationAmount, Closing: e.ClosingValue, Method: e.Method,
		}
	}
	return out
}

// TestDepreciation_Compute_Idempotent verifies running ComputePeriod twice for
// the same (open) period produces an identical entry set — same count, and
// the same values (surrogate ids differ across the delete+reinsert
// regeneration, but the schedule itself does not) — with no
// unique-constraint violation.
func TestDepreciation_Compute_Idempotent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00002", "Software Akuntansi", h.catID, h.office, "12000000.00", 5)
	target := firstOfMonthUTC(time.Now())

	_, err := h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)
	first, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)

	_, err = h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)
	second, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)

	require.Len(t, second, len(first))
	require.NotEmpty(t, first)
	assert.Equal(t, valuesOf(first), valuesOf(second))
}

// TestDepreciation_Compute_SkippedReporting seeds two problem assets (one with
// no purchase_cost/date, one not capitalized) and asserts RunSummary reports
// exactly one SkippedAsset per problem asset (not one per basis).
func TestDepreciation_Compute_SkippedReporting(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	noCostID := seedAssetNoCost(t, h.pool, "DPR-2026-00003", "Tanpa Biaya", h.catID, h.office)
	notCapID := seedAssetNotCapitalized(t, h.pool, "DPR-2026-00004", "Tak Dikapitalisasi", h.catID, h.office, "5000000.00", 2)
	target := firstOfMonthUTC(time.Now())

	summary, err := h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)

	assert.Equal(t, 2, summary.SkippedCount)
	require.Len(t, summary.Skipped, 2)

	reasons := map[uuid.UUID]string{}
	for _, s := range summary.Skipped {
		reasons[s.AssetID] = s.Reason
	}
	assert.Equal(t, "no_cost", reasons[noCostID])
	assert.Equal(t, "not_capitalized", reasons[notCapID])
}

// TestDepreciation_StateMachine drives the period status machine directly.
func TestDepreciation_StateMachine(t *testing.T) {
	t.Run("SequentialLifecycle", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()
		seedAssetMonthsAgo(t, h.pool, "DPR-2026-00005", "Aset SM1", h.catID, h.office, "10000000.00", 4)

		month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
		month2 := firstOfMonthUTC(time.Now())

		// Close before compute → ErrPeriodNotComputed (no row at all yet).
		err := h.svc.ClosePeriod(ctx, month1, h.actorID)
		require.ErrorIs(t, err, depreciation.ErrPeriodNotComputed)

		// Compute → Close OK.
		_, err = h.svc.ComputePeriod(ctx, month1, h.actorID)
		require.NoError(t, err)
		err = h.svc.ClosePeriod(ctx, month1, h.actorID)
		require.NoError(t, err)

		// Compute again on a closed period → ErrPeriodClosed.
		_, err = h.svc.ComputePeriod(ctx, month1, h.actorID)
		require.ErrorIs(t, err, depreciation.ErrPeriodClosed)

		// Close(month2) without computing month2 first → ErrPeriodNotComputed.
		err = h.svc.ClosePeriod(ctx, month2, h.actorID)
		require.ErrorIs(t, err, depreciation.ErrPeriodNotComputed)
	})

	t.Run("PriorPeriodOpenGuard", func(t *testing.T) {
		h := newHarness(t)
		ctx := context.Background()
		seedAssetMonthsAgo(t, h.pool, "DPR-2026-00006", "Aset SM2", h.catID, h.office, "10000000.00", 4)

		month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
		month2 := firstOfMonthUTC(time.Now())

		// month1 computed but left OPEN (not closed)...
		_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
		require.NoError(t, err)

		// ...month2 computed fine (compute itself doesn't require sequencing)...
		_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
		require.NoError(t, err)

		// ...but closing month2 must fail: month1 has a row and isn't closed.
		err = h.svc.ClosePeriod(ctx, month2, h.actorID)
		require.ErrorIs(t, err, depreciation.ErrPriorPeriodOpen)
	})
}

// TestDepreciation_ClosedWatermark_Immutable computes+closes month1, then
// computes month2, and verifies month1's entries are byte-identical
// (untouched) while month2's opening continues from month1's closing.
func TestDepreciation_ClosedWatermark_Immutable(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00007", "Aset Watermark", h.catID, h.office, "9600000.00", 6)

	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	month2 := firstOfMonthUTC(time.Now())

	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, month1, h.actorID))

	before, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)
	require.NotEmpty(t, before)

	_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
	require.NoError(t, err)

	after, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)

	// Every entry at or before month1 must be byte-identical to before compute(month2).
	beforeByPeriod := map[time.Time]sqlc.DepreciationDepreciationEntry{}
	for _, e := range before {
		if !e.Period.Time.After(month1) {
			beforeByPeriod[keyOf(e)] = e
		}
	}
	require.NotEmpty(t, beforeByPeriod)
	var m1Commercial, m2Commercial sqlc.DepreciationDepreciationEntry
	for _, e := range after {
		if !e.Period.Time.After(month1) {
			want, ok := beforeByPeriod[keyOf(e)]
			require.True(t, ok, "entry %+v must have existed before month2 compute too", e)
			assert.Equal(t, want, e, "closed-watermark entries must not be regenerated")
		}
		if e.Basis == sqlc.SharedDepreciationBasisCommercial {
			if e.Period.Time.Equal(month1) {
				m1Commercial = e
			}
			if e.Period.Time.Equal(month2) {
				m2Commercial = e
			}
		}
	}
	require.NotEmpty(t, m1Commercial.ID)
	require.NotEmpty(t, m2Commercial.ID)
	assert.Equal(t, m1Commercial.ClosingValue, m2Commercial.OpeningValue, "m2 opening must continue from m1 closing")
}

// TestDepreciation_UpsertGuard_ClosedPeriodNotReopened exercises the SQL-level
// guard on UpsertPeriodComputed directly: once a period is closed, the upsert's
// DO UPDATE must not match (0 rows → pgx.ErrNoRows) so a racing ComputePeriod
// that lost the close/compute race can never flip a closed period back to
// 'computed'. Also re-asserts the service-level outcome: ComputePeriod on the
// closed period returns ErrPeriodClosed and leaves its entries byte-identical.
func TestDepreciation_UpsertGuard_ClosedPeriodNotReopened(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00012", "Aset Reopen Guard", h.catID, h.office, "6000000.00", 3)

	target := firstOfMonthUTC(time.Now())
	_, err := h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, target, h.actorID))

	before, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)
	require.NotEmpty(t, before)

	// Layer (b): the raw upsert against a closed period must return 0 rows.
	_, err = h.q.UpsertPeriodComputed(ctx, sqlc.UpsertPeriodComputedParams{
		Period:     pgDate(target),
		ComputedBy: &h.actorID,
		AssetCount: 99, TotalAmount: "999999.00", SkippedCount: 9,
	})
	require.ErrorIs(t, err, pgx.ErrNoRows, "DO UPDATE must not match a closed period row")

	row, err := h.q.GetDepreciationPeriod(ctx, pgDate(target))
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedDepreciationPeriodStatusClosed, row.Status, "period must stay closed")
	assert.NotEqualValues(t, 99, row.AssetCount, "closed period summary must be untouched")

	// Service level: recompute is rejected and entries survive unchanged.
	_, err = h.svc.ComputePeriod(ctx, target, h.actorID)
	require.ErrorIs(t, err, depreciation.ErrPeriodClosed)

	after, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, before, after, "entries of a closed period must be byte-identical after a rejected recompute")
}

// TestDepreciation_ClosePeriod_BlocksOnComputeLock verifies ClosePeriod
// serializes against ComputePeriod: while another transaction holds the
// depreciation advisory lock (as ComputePeriod does for its whole run),
// ClosePeriod must block, and complete successfully once the lock is released.
func TestDepreciation_ClosePeriod_BlocksOnComputeLock(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	seedAssetMonthsAgo(t, h.pool, "DPR-2026-00013", "Aset Close Lock", h.catID, h.office, "2400000.00", 2)

	target := firstOfMonthUTC(time.Now())
	_, err := h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)

	// Hold the compute advisory lock in a foreign transaction, standing in for
	// an in-flight ComputePeriod.
	lockTx, err := h.pool.Begin(ctx)
	require.NoError(t, err)
	_, err = lockTx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('depreciation.compute'))`)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- h.svc.ClosePeriod(ctx, target, h.actorID) }()

	select {
	case err := <-done:
		require.NoError(t, lockTx.Rollback(ctx))
		t.Fatalf("ClosePeriod completed (err=%v) while the compute advisory lock was held — it must block", err)
	case <-time.After(750 * time.Millisecond):
		// Still blocked — the desired behavior.
	}

	require.NoError(t, lockTx.Rollback(ctx), "release the advisory lock")

	select {
	case err := <-done:
		require.NoError(t, err, "ClosePeriod must succeed once the lock is released")
	case <-time.After(5 * time.Second):
		t.Fatal("ClosePeriod still blocked after the advisory lock was released")
	}

	row, err := h.q.GetDepreciationPeriod(ctx, pgDate(target))
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedDepreciationPeriodStatusClosed, row.Status)
}

// TestDepreciation_ClosePeriod_Twice verifies the close/close path: the second
// ClosePeriod for the same period returns ErrPeriodClosed (never a leaked
// driver error such as pgx.ErrNoRows from the guarded UPDATE).
func TestDepreciation_ClosePeriod_Twice(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	seedAssetMonthsAgo(t, h.pool, "DPR-2026-00014", "Aset Close Twice", h.catID, h.office, "1200000.00", 1)

	target := firstOfMonthUTC(time.Now())
	_, err := h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)

	require.NoError(t, h.svc.ClosePeriod(ctx, target, h.actorID))
	err = h.svc.ClosePeriod(ctx, target, h.actorID)
	require.ErrorIs(t, err, depreciation.ErrPeriodClosed)
}

// TestDepreciation_Compute_BeforeWatermark verifies that computing a period at
// or before the closed watermark — a month that was skipped and never computed
// before later months closed — is rejected with ErrPeriodBeforeWatermark and
// creates NO period row (previously it silently produced a hollow 'computed'
// row with zero entries).
func TestDepreciation_Compute_BeforeWatermark(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	seedAssetMonthsAgo(t, h.pool, "DPR-2026-00015", "Aset Skipped Month", h.catID, h.office, "3600000.00", 6)

	month0 := firstOfMonthUTC(time.Now()).AddDate(0, -2, 0) // skipped, never computed
	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	month2 := firstOfMonthUTC(time.Now())

	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, month1, h.actorID))
	_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, month2, h.actorID))

	_, err = h.svc.ComputePeriod(ctx, month0, h.actorID)
	require.ErrorIs(t, err, depreciation.ErrPeriodBeforeWatermark)

	_, err = h.q.GetDepreciationPeriod(ctx, pgDate(month0))
	require.ErrorIs(t, err, pgx.ErrNoRows, "no period row may be created for a pre-watermark month")
}

// keyOf builds a stable map key (basis+period) for comparing entries across
// two ListAssetEntries calls.
func keyOf(e sqlc.DepreciationDepreciationEntry) time.Time {
	// Distinguish basis by offsetting fiscal entries by a fixed, out-of-range
	// duration so commercial/fiscal periods never collide as map keys.
	if e.Basis == sqlc.SharedDepreciationBasisFiscal {
		return e.Period.Time.AddDate(1000, 0, 0)
	}
	return e.Period.Time
}

// TestDepreciation_AdvisoryLock runs two ComputePeriod calls concurrently for
// the same period and asset set; the advisory lock must serialize them so
// both succeed (no unique-violation) and the final entry set is exactly what
// a single compute would have produced (not duplicated).
func TestDepreciation_AdvisoryLock(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00008", "Aset Lock", h.catID, h.office, "7200000.00", 3)
	target := firstOfMonthUTC(time.Now())

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := h.svc.ComputePeriod(ctx, target, h.actorID)
			errs[i] = err
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	entries, err := h.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)
	assert.Len(t, entries, 8, "4 months x 2 bases, not duplicated")
}

// TestDepreciation_BookValueAsOf covers all three BookValueAsOf paths: an
// asset with entries returns the last commercial closing <= asOf; an asset
// with a cost but no entries falls back to purchase_cost; an asset with
// neither entries nor cost returns "0".
func TestDepreciation_BookValueAsOf(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	t.Run("WithEntries", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00009", "Aset BVAO", h.catID, h.office, "4800000.00", 2)
		target := firstOfMonthUTC(time.Now())
		_, err := h.svc.ComputePeriod(ctx, target, h.actorID)
		require.NoError(t, err)

		bv, err := h.svc.BookValueAsOf(ctx, assetID, target)
		require.NoError(t, err)

		entries, err := h.q.ListAssetEntries(ctx, assetID)
		require.NoError(t, err)
		var wantClosing string
		for _, e := range entries {
			if e.Basis == sqlc.SharedDepreciationBasisCommercial && e.Period.Time.Equal(target) {
				wantClosing = e.ClosingValue
			}
		}
		require.NotEmpty(t, wantClosing)
		assert.Equal(t, wantClosing, bv)
	})

	t.Run("NoEntries_FallbackPurchaseCost", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00010", "Aset Belum Dihitung", h.catID, h.office, "3000000.00", 1)
		bv, err := h.svc.BookValueAsOf(ctx, assetID, firstOfMonthUTC(time.Now()))
		require.NoError(t, err)
		assert.Equal(t, "3000000.00", bv)
	})

	t.Run("NoEntries_NoCost_Zero", func(t *testing.T) {
		assetID := seedAssetNoCost(t, h.pool, "DPR-2026-00011", "Aset Tanpa Apapun", h.catID, h.office)
		bv, err := h.svc.BookValueAsOf(ctx, assetID, firstOfMonthUTC(time.Now()))
		require.NoError(t, err)
		assert.Equal(t, "0", bv)
	})
}

// pgDate wraps a time.Time as a valid pgtype.Date for direct sqlc query calls.
func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// ─── HTTP-wiring tests (Task 4) ─────────────────────────────────────────────
//
// These drive the real depreciation.Handler through depreciation.RegisterRoutes
// + a stub auth middleware + httptest, exactly like approval's
// TestApproval_ThresholdPreview / TestApproval_FieldMasking_HandlerWiring.

// httpHarness extends harness with the HTTP layer: a wired Handler plus a
// doReq helper that drives a fresh gin engine per call (stub auth injecting
// the given user/role, bypassing real JWT).
type httpHarness struct {
	*harness
	t       *testing.T
	handler *depreciation.Handler
	permSvc *authz.PermissionService
}

// newHTTPHarness builds on newHarness, adding the Redis-backed authz services
// (permission/scope/field) and the depreciation HTTP handler.
func newHTTPHarness(t *testing.T) *httpHarness {
	t.Helper()
	h := newHarness(t)
	rdb := testsupport.NewRedis(t)
	scopeSvc := authz.NewScopeService(h.q, rdb)
	permSvc := authz.NewPermissionService(h.q, rdb)
	fieldSvc := authz.NewFieldService(h.q, rdb)
	auditSvc := audit.NewService(h.q)
	scoped := common.ScopedDeps{Q: h.q, Scope: scopeSvc}
	handler := depreciation.NewHandler(h.svc, fieldSvc, scoped, auditSvc)
	return &httpHarness{harness: h, t: t, handler: handler, permSvc: permSvc}
}

// doReq drives one HTTP request (no body) against a fresh gin engine wired
// with depreciation.RegisterRoutes, decoding a JSON object response body.
func (hh *httpHarness) doReq(method, path string, userID, roleID uuid.UUID) (int, map[string]any) {
	gin.SetMode(gin.TestMode)
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	depreciation.RegisterRoutes(v1, hh.handler, stubAuth,
		middleware.RequirePermission(hh.permSvc, "depreciation.manage"),
		middleware.RequirePermission(hh.permSvc, "depreciation.view"),
		middleware.RequirePermission(hh.permSvc, "asset.view"),
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	var body map[string]any
	if w.Body.Len() > 0 {
		require.NoError(hh.t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w.Code, body
}

// TestDepreciation_HTTP_PeriodsComputeClose drives POST compute/close through
// the real routes: 200s for the happy sequential path, 400 on a garbage
// period param, 409 recomputing/reclosing an already-closed period, and 422
// closing a period that was never computed.
func TestDepreciation_HTTP_PeriodsComputeClose(t *testing.T) {
	hh := newHTTPHarness(t)
	seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00016", "Aset HTTP Compute", hh.catID, hh.office, "18500000.00", 3)

	target := firstOfMonthUTC(time.Now())
	period := target.Format("2006-01")

	t.Run("invalid period format -> 400", func(t *testing.T) {
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/not-a-period/compute", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("close before compute -> 422 (ErrPeriodNotComputed)", func(t *testing.T) {
		code, body := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/close", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, body, "error")
	})

	t.Run("compute -> 200 computed", func(t *testing.T) {
		code, body := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/compute", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, period, body["period"])
		assert.Equal(t, "computed", body["status"])
		assert.EqualValues(t, 1, body["asset_count"])
		assert.Equal(t, "346875.00", body["total_amount"])
	})

	t.Run("close -> 200 closed", func(t *testing.T) {
		code, body := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/close", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, period, body["period"])
		assert.Equal(t, "closed", body["status"])
	})

	t.Run("recompute closed period -> 409", func(t *testing.T) {
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/compute", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusConflict, code)
	})

	t.Run("reclose closed period -> 409", func(t *testing.T) {
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/close", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusConflict, code)
	})

	t.Run("list periods includes the closed period", func(t *testing.T) {
		code, body := hh.doReq(http.MethodGet, "/api/v1/depreciation/periods", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		rows, ok := body["data"].([]any)
		require.True(t, ok)
		found := false
		for _, raw := range rows {
			row := raw.(map[string]any)
			if row["period"] == period {
				found = true
				assert.Equal(t, "closed", row["status"])
			}
		}
		assert.True(t, found)
	})
}

// TestDepreciation_HTTP_Schedule drives GET /depreciation/schedule end-to-end:
// after computing a period with one normally-depreciating asset and one
// already-fully-depreciated asset (1-month useful life, purchased 5 months
// ago — so it produced its single entry long before the target period), the
// schedule must return both as rows: the normal asset's real entry, and the
// exhausted asset as a synthetic "fully depreciated" union row (amount
// "0.00", opening==closing==book value 0, fully_depreciated:true). KPIs sum
// across both.
func TestDepreciation_HTTP_Schedule(t *testing.T) {
	hh := newHTTPHarness(t)
	ctx := context.Background()

	normalID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00017", "Aset Berjalan", hh.catID, hh.office, "18500000.00", 3)

	// Fully-depreciated asset: per-asset override life_months=1, salvage=0,
	// purchased 5 months ago — Walk absorbs the entire cost in its single
	// purchase-month entry, then produces nothing for any later period.
	exhaustedPurchase := firstOfMonthUTC(time.Now()).AddDate(0, -5, 0)
	var exhaustedID uuid.UUID
	require.NoError(t, hh.pool.QueryRow(ctx,
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status,
		    purchase_date, purchase_cost, useful_life_months, salvage_value, depreciation_method)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', 'available', $5, $6, 1, '0', 'straight_line')
		 RETURNING id`,
		"DPR-2026-00018", "Aset Sudah Habis", hh.catID, hh.office, exhaustedPurchase, "1000000.00").
		Scan(&exhaustedID))

	target := firstOfMonthUTC(time.Now())
	period := target.Format("2006-01")

	code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/compute", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	// Sanity: the exhausted asset really has no COMMERCIAL entry for the
	// target period (its fiscal life is unaffected by the commercial
	// useful_life_months=1 override — the category's default fiscal group
	// still has a 48-month life, so it keeps generating fiscal entries; only
	// the commercial basis, which the schedule endpoint below queries, is
	// exhausted).
	entries, err := hh.q.ListAssetEntries(ctx, exhaustedID)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Basis == sqlc.SharedDepreciationBasisCommercial {
			assert.False(t, e.Period.Time.Equal(target), "exhausted asset must have no commercial entry in the target period")
		}
	}

	code, body := hh.doReq(http.MethodGet, "/api/v1/depreciation/schedule?period="+period+"&basis=commercial", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	rows, ok := body["rows"].([]any)
	require.True(t, ok)

	var normalRow, exhaustedRow map[string]any
	for _, raw := range rows {
		row := raw.(map[string]any)
		switch row["asset_id"] {
		case normalID.String():
			normalRow = row
		case exhaustedID.String():
			exhaustedRow = row
		}
	}

	require.NotNil(t, normalRow, "normal asset must appear via its real entry")
	assert.Equal(t, "346875.00", normalRow["amount"])
	assert.Equal(t, false, normalRow["fully_depreciated"])

	require.NotNil(t, exhaustedRow, "exhausted asset must appear via the fully-depreciated union")
	assert.Equal(t, "0.00", exhaustedRow["amount"])
	assert.Equal(t, "0.00", exhaustedRow["opening"])
	assert.Equal(t, "0.00", exhaustedRow["closing"])
	assert.Equal(t, true, exhaustedRow["fully_depreciated"])

	kpi, ok := body["kpi"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "19500000.00", kpi["total_cost"], "18,500,000 + 1,000,000")
	assert.Equal(t, "346875.00", kpi["period_expense"], "only the normal asset expenses this period")

	totals, ok := body["totals"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, kpi["period_expense"], totals["amount"])
}

// TestDepreciation_HTTP_Journal drives GET /depreciation/journal end-to-end:
// total_debit must equal total_credit (balanced), and since the harness's
// category has no gl_account_code, its entries must fold into the single
// "-"/"(tanpa akun GL)" debit row (null-GL grouping).
func TestDepreciation_HTTP_Journal(t *testing.T) {
	hh := newHTTPHarness(t)
	seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00019", "Aset Jurnal", hh.catID, hh.office, "12000000.00", 2)

	target := firstOfMonthUTC(time.Now())
	period := target.Format("2006-01")

	code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/compute", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	code, body := hh.doReq(http.MethodGet, "/api/v1/depreciation/journal?period="+period+"&basis=commercial", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	assert.Equal(t, true, body["balanced"])
	assert.Equal(t, body["total_debit"], body["total_credit"])
	assert.NotEqual(t, "0.00", body["total_debit"])

	rows, ok := body["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 2, "one debit row (null-GL group) + one credit row")

	debit := rows[0].(map[string]any)
	assert.Equal(t, "-", debit["account_code"])
	assert.Equal(t, "(tanpa akun GL)", debit["account_name"])
	assert.Equal(t, body["total_debit"], debit["debit"])

	credit := rows[1].(map[string]any)
	assert.Equal(t, "Akumulasi Penyusutan", credit["account_name"])
	assert.Equal(t, body["total_credit"], credit["credit"])
}

// TestDepreciation_HTTP_AssetSchedule drives GET /assets/:id/depreciation:
// Superadmin (no field-permission policy on "assets") sees the full entry
// history + computed book value; a role denied view on "assets".book_value
// (mirroring TestApproval_FieldMasking_Requests's deny-row shape) gets the
// masked shape instead.
func TestDepreciation_HTTP_AssetSchedule(t *testing.T) {
	hh := newHTTPHarness(t)

	assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00020", "Aset Riwayat", hh.catID, hh.office, "6000000.00", 2)
	target := firstOfMonthUTC(time.Now())
	period := target.Format("2006-01")

	code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/compute", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	t.Run("unmasked for Superadmin", func(t *testing.T) {
		code, body := hh.doReq(http.MethodGet, "/api/v1/assets/"+assetID.String()+"/depreciation", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, false, body["masked"])
		assert.NotNil(t, body["computed_book_value"])
		entries, ok := body["entries"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, entries)
	})

	t.Run("masked when book_value view is denied", func(t *testing.T) {
		// Staf is seeded (migration 000016) with can_view=false on
		// assets.book_value — no extra field_permissions row needed (and
		// inserting a duplicate one would violate uq_field_permissions).
		stafRoleID := lookupRole(t, hh.pool, "Staf")
		stafUser := seedUser(t, hh.pool, stafRoleID, hh.office, "staf.assetschedule."+uuid.New().String()[:8]+"@test.local")

		code, body := hh.doReq(http.MethodGet, "/api/v1/assets/"+assetID.String()+"/depreciation", stafUser, stafRoleID)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, true, body["masked"])
		assert.Nil(t, body["computed_book_value"])
		entries, ok := body["entries"].([]any)
		require.True(t, ok)
		assert.Empty(t, entries)
	})

	t.Run("invalid asset id -> 400", func(t *testing.T) {
		code, _ := hh.doReq(http.MethodGet, "/api/v1/assets/not-a-uuid/depreciation", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("unknown asset id -> 404", func(t *testing.T) {
		code, _ := hh.doReq(http.MethodGet, "/api/v1/assets/"+uuid.New().String()+"/depreciation", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusNotFound, code)
	})
}
