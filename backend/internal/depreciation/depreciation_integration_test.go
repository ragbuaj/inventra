//go:build integration

package depreciation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
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

// mustParseRat parses a decimal string into an exact rational for magnitude
// comparisons in the tests below.
func mustParseRat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	require.True(t, ok, "parse decimal %q", s)
	return r
}

// commercialEntryForPeriod returns the single commercial entry for the given
// asset+period (fatal if absent).
func commercialEntryForPeriod(t *testing.T, h *harness, assetID uuid.UUID, period time.Time) sqlc.DepreciationDepreciationEntry {
	t.Helper()
	entries, err := h.q.ListAssetEntries(context.Background(), assetID)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Basis == sqlc.SharedDepreciationBasisCommercial && e.Period.Time.Equal(period) {
			return e
		}
	}
	t.Fatalf("no commercial entry for period %s", period.Format("2006-01"))
	return sqlc.DepreciationDepreciationEntry{}
}

// commercialEntriesOf returns all commercial entries for an asset, ordered as
// ListAssetEntries returns them (basis, period).
func commercialEntriesOf(t *testing.T, h *harness, assetID uuid.UUID) []sqlc.DepreciationDepreciationEntry {
	t.Helper()
	entries, err := h.q.ListAssetEntries(context.Background(), assetID)
	require.NoError(t, err)
	var out []sqlc.DepreciationDepreciationEntry
	for _, e := range entries {
		if e.Basis == sqlc.SharedDepreciationBasisCommercial {
			out = append(out, e)
		}
	}
	return out
}

// TestDepreciation_Recompute_AfterClose_Idempotent is the F1 regression: after
// closing month1, computing the open month2 TWICE must produce a byte-identical
// month2 entry. The buggy override derived "an impairment happened" from
// asset.book_value — which refreshAssetSummary rewrites to the latest computed
// closing on every compute — so the SECOND compute saw a lower book_value than
// month1's (immutable) closing and wrongly resumed from the already-depreciated
// value, DOUBLE-depreciating the fleet. No impairment occurs here, so the two
// runs must be identical ordinary straight-line months.
func TestDepreciation_Recompute_AfterClose_Idempotent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// cost 9,600,000; category salvage 10% -> 960,000; SL 48m; purchased 6mo ago.
	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00032", "Aset Idempoten Setelah Tutup", h.catID, h.office, "9600000.00", 6)

	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	month2 := firstOfMonthUTC(time.Now())

	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, month1, h.actorID))

	// Run 1 of the open month2.
	_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
	require.NoError(t, err)
	run1 := commercialEntryForPeriod(t, h, assetID, month2)
	assert.Equal(t, "8520000.00", run1.OpeningValue, "opens from month1 closing (9,600,000 - 6*180,000)")
	assert.Equal(t, "180000.00", run1.DepreciationAmount, "ordinary SL month: (8,520,000 - 960,000)/42")
	assert.Equal(t, "8340000.00", run1.ClosingValue)

	// Run 2 of the SAME open month2 (idempotent re-run, explicitly supported).
	// Must be byte-identical: no impairment happened, so the compute must NOT
	// ratchet down from the book_value that run1's refreshAssetSummary wrote.
	_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
	require.NoError(t, err)
	run2 := commercialEntryForPeriod(t, h, assetID, month2)

	assert.Equal(t,
		valuesOf([]sqlc.DepreciationDepreciationEntry{run1}),
		valuesOf([]sqlc.DepreciationDepreciationEntry{run2}),
		"F1: second compute of an open period after a close must be idempotent (no double depreciation)")
	assert.Equal(t, "180000.00", run2.DepreciationAmount, "must stay 180,000.00, not the double-depreciated 175,714.29")
	assert.Equal(t, "8340000.00", run2.ClosingValue)
}

// TestDepreciation_Impairment_Prospective_AfterClose verifies that impairment
// resumes from the STABLE floor and stays idempotent across repeated computes:
// close month1, impair well below the current book value, then compute the open
// month2 TWICE — both runs must open from the impaired floor and be identical
// (the buggy version double-depreciated on the second run because book_value
// had already dropped below the floor).
func TestDepreciation_Impairment_Prospective_AfterClose(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00033", "Aset Impairment Idempoten", h.catID, h.office, "9600000.00", 6)

	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	month2 := firstOfMonthUTC(time.Now())

	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, month1, h.actorID))

	// Impair from book 8,520,000 down to recoverable 8,000,000.
	_, err = h.svc.RecordImpairment(ctx, assetID, "8000000.00", "uji penurunan nilai", h.actorID)
	require.NoError(t, err)

	// Recompute the open month2: must resume prospectively from the impaired floor.
	_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
	require.NoError(t, err)
	run1 := commercialEntryForPeriod(t, h, assetID, month2)
	assert.Equal(t, "8000000.00", run1.OpeningValue, "opens from impaired floor, not month1 closing 8,520,000")
	// remaining = 48 - 6 = 42; amount = (8,000,000 - 960,000)/42 = 167,619.047... -> 167,619.05
	assert.Equal(t, "167619.05", run1.DepreciationAmount)
	assert.Equal(t, "7832380.95", run1.ClosingValue)

	// Closed month1 stays byte-identical (impairment is prospective).
	m1 := commercialEntryForPeriod(t, h, assetID, month1)
	assert.Equal(t, "8520000.00", m1.ClosingValue, "closed history must not be rewritten")

	// Second recompute of the open month2 must be byte-identical (idempotent
	// even WITH an impairment in force — the floor is stable, not ratcheted).
	_, err = h.svc.ComputePeriod(ctx, month2, h.actorID)
	require.NoError(t, err)
	run2 := commercialEntryForPeriod(t, h, assetID, month2)
	assert.Equal(t,
		valuesOf([]sqlc.DepreciationDepreciationEntry{run1}),
		valuesOf([]sqlc.DepreciationDepreciationEntry{run2}),
		"impairment recompute must be idempotent (second run must not double-depreciate)")
}

// TestDepreciation_Impairment_BeforeAnyClose_Persists is the F2 regression: an
// impairment recorded when NO period was ever closed (watermark == nil) must
// survive a recompute. The buggy override only ran on the watermark!=nil path,
// so a recompute regenerated entries from cost and refreshAssetSummary raised
// book_value back above the recoverable while impairment_loss stayed positive —
// an inconsistent carrying amount. After the fix the recompute resumes from the
// impaired floor, so book_value stays at/below the recoverable, the loss is
// retained, and repeated computes are idempotent.
func TestDepreciation_Impairment_BeforeAnyClose_Persists(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00034", "Aset Impairment Pra-Tutup", h.catID, h.office, "9600000.00", 6)

	month := firstOfMonthUTC(time.Now())

	// Compute (but never CLOSE) — genesis, no closed watermark exists.
	_, err := h.svc.ComputePeriod(ctx, month, h.actorID)
	require.NoError(t, err)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, a.BookValue)
	// 7 entries (purchase month .. current, inclusive): 9,600,000 - 7*180,000 = 8,340,000.
	require.Equal(t, "8340000.00", *a.BookValue)

	// Impair to recoverable 7,000,000 (below current book value).
	_, err = h.svc.RecordImpairment(ctx, assetID, "7000000.00", "uji penurunan pra-tutup", h.actorID)
	require.NoError(t, err)

	// Recompute (still no close; watermark stays nil). Impairment must persist.
	_, err = h.svc.ComputePeriod(ctx, month, h.actorID)
	require.NoError(t, err)

	recomputed, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, recomputed.BookValue)
	assert.LessOrEqual(t, mustParseRat(t, *recomputed.BookValue).Cmp(mustParseRat(t, "7000000.00")), 0,
		"F2: book_value must stay at/below the recoverable after recompute (not ratcheted back to 8,340,000)")
	require.NotNil(t, recomputed.ImpairmentLoss)
	assert.Equal(t, 1, mustParseRat(t, *recomputed.ImpairmentLoss).Sign(), "impairment_loss must be retained (positive)")
	assert.Equal(t, "1340000.00", *recomputed.ImpairmentLoss, "8,340,000 - 7,000,000")

	// Idempotent on a third compute (the floor is stable).
	before := commercialEntriesOf(t, h, assetID)
	require.NotEmpty(t, before)
	_, err = h.svc.ComputePeriod(ctx, month, h.actorID)
	require.NoError(t, err)
	after := commercialEntriesOf(t, h, assetID)
	assert.Equal(t, valuesOf(before), valuesOf(after), "third compute must be idempotent")
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

// doReqRaw is doReq's sibling for binary endpoints (the journal export's
// xlsx/pdf downloads) — same stub-auth/fresh-engine wiring, but returns the
// raw status, headers, and body bytes instead of JSON-decoding the body
// (which would fail on non-JSON payloads).
func (hh *httpHarness) doReqRaw(method, path string, userID, roleID uuid.UUID) (int, http.Header, []byte) {
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
	return w.Code, w.Header().Clone(), w.Body.Bytes()
}

// doReqBody is doReq's sibling for requests that carry a JSON body (the
// impairment endpoint's POST /assets/:id/impairment) — same stub-auth/fresh-
// engine wiring, plus a Content-Type header and a non-nil request body.
func (hh *httpHarness) doReqBody(method, path string, userID, roleID uuid.UUID, body []byte) (int, map[string]any) {
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
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var respBody map[string]any
	if w.Body.Len() > 0 {
		require.NoError(hh.t, json.Unmarshal(w.Body.Bytes(), &respBody))
	}
	return w.Code, respBody
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

// ─── HTTP-wiring tests (Task 5 — impairment, PSAK 48) ──────────────────────

// TestDepreciation_HTTP_Impairment drives POST /assets/:id/impairment
// end-to-end: happy path (loss accumulates, book drops, an audit row is
// written), the recoverable-must-be-below-book-value guard, the
// no-book-value guard, the permission gate, and basic request validation.
func TestDepreciation_HTTP_Impairment(t *testing.T) {
	hh := newHTTPHarness(t)
	ctx := context.Background()

	t.Run("happy path: loss accumulates, book drops, audit row written", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00021", "Aset Impairment", hh.catID, hh.office, "9600000.00", 6)
		month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
		period1 := month1.Format("2006-01")

		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		require.NoError(t, hh.svc.ClosePeriod(ctx, month1, hh.actorID))

		a, err := hh.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		require.NotNil(t, a.BookValue)
		require.Equal(t, "8520000.00", *a.BookValue, "9,600,000 - 6*180,000")

		body := []byte(`{"recoverable_amount":"8000000.00","reason":"kerusakan berat akibat banjir"}`)
		code, respBody := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", hh.actorID, hh.roleID, body)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "8000000.00", respBody["book_value"])
		assert.Equal(t, "520000.00", respBody["impairment_loss"])
		assert.Equal(t, a.AccumulatedDepreciation, respBody["accumulated_depreciation"], "impairment must not touch accumulated_depreciation")

		updated, err := hh.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		require.NotNil(t, updated.BookValue)
		assert.Equal(t, "8000000.00", *updated.BookValue)
		require.NotNil(t, updated.ImpairmentLoss)
		assert.Equal(t, "520000.00", *updated.ImpairmentLoss)

		var changesRaw []byte
		require.NoError(t, hh.pool.QueryRow(ctx,
			`SELECT changes FROM audit.audit_logs WHERE entity_type = 'assets' AND entity_id = $1 AND action = 'update' ORDER BY created_at DESC LIMIT 1`,
			assetID).Scan(&changesRaw))
		var changes map[string]map[string]any
		require.NoError(t, json.Unmarshal(changesRaw, &changes))
		require.Contains(t, changes, "book_value")
		assert.Equal(t, "8520000.00", changes["book_value"]["before"])
		assert.Equal(t, "8000000.00", changes["book_value"]["after"])
		require.Contains(t, changes, "impairment_loss")
		assert.Equal(t, "520000.00", changes["impairment_loss"]["after"])
		require.Contains(t, changes, "reason")
		assert.Equal(t, "kerusakan berat akibat banjir", changes["reason"]["after"])
	})

	t.Run("recoverable >= book value -> 422", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00022", "Aset Impairment Guard", hh.catID, hh.office, "6000000.00", 3)
		month1 := firstOfMonthUTC(time.Now())
		period1 := month1.Format("2006-01")
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)

		a, err := hh.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		require.NotNil(t, a.BookValue)

		body := []byte(fmt.Sprintf(`{"recoverable_amount":%q,"reason":"tidak valid"}`, *a.BookValue))
		code, respBody := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", hh.actorID, hh.roleID, body)
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, respBody, "error")
	})

	t.Run("no book value (never computed) -> 422", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00023", "Aset Belum Dihitung Impairment", hh.catID, hh.office, "4000000.00", 2)
		body := []byte(`{"recoverable_amount":"1000000.00","reason":"belum dihitung"}`)
		code, respBody := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", hh.actorID, hh.roleID, body)
		assert.Equal(t, http.StatusUnprocessableEntity, code)
		assert.Contains(t, respBody, "error")
	})

	t.Run("permission gate: Staf lacks depreciation.manage -> 403", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00024", "Aset Impairment Gate", hh.catID, hh.office, "3000000.00", 2)
		month1 := firstOfMonthUTC(time.Now())
		period1 := month1.Format("2006-01")
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)

		stafRoleID := lookupRole(t, hh.pool, "Staf")
		stafUser := seedUser(t, hh.pool, stafRoleID, hh.office, "staf.impairment."+uuid.New().String()[:8]+"@test.local")

		body := []byte(`{"recoverable_amount":"1000000.00","reason":"tidak berwenang"}`)
		code, _ = hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", stafUser, stafRoleID, body)
		assert.Equal(t, http.StatusForbidden, code)
	})

	t.Run("invalid asset id -> 400", func(t *testing.T) {
		body := []byte(`{"recoverable_amount":"1000000.00","reason":"x"}`)
		code, _ := hh.doReqBody(http.MethodPost, "/api/v1/assets/not-a-uuid/impairment", hh.actorID, hh.roleID, body)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("malformed recoverable_amount -> 400", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00025", "Aset Impairment Format", hh.catID, hh.office, "2000000.00", 2)
		month1 := firstOfMonthUTC(time.Now())
		period1 := month1.Format("2006-01")
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)

		body := []byte(`{"recoverable_amount":"1e5","reason":"format salah"}`)
		code, _ = hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", hh.actorID, hh.roleID, body)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("missing reason -> 400", func(t *testing.T) {
		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00026", "Aset Impairment No Reason", hh.catID, hh.office, "2000000.00", 2)
		body := []byte(`{"recoverable_amount":"1000000.00"}`)
		code, _ := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", hh.actorID, hh.roleID, body)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("unknown asset id -> 404", func(t *testing.T) {
		body := []byte(`{"recoverable_amount":"1000000.00","reason":"x"}`)
		code, _ := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+uuid.New().String()+"/impairment", hh.actorID, hh.roleID, body)
		assert.Equal(t, http.StatusNotFound, code)
	})
}

// seedManageRole creates a fresh role granted "depreciation.manage" (the
// permission gating compute/close/impairment) and returns its id. Needed
// because the only seeded role holding depreciation.manage is Superadmin
// (migration 000023, PRD §2.1 restriction), and Superadmin's own
// field_permissions row grants book_value view — so a book_value-denied role
// that can also reach the impairment endpoint does not exist in the seed data
// and must be built ad hoc for the field-masking test below.
func seedManageRole(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	roleID := testsupport.SeedRole(t, pool, code)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, 'depreciation.manage')`,
		roleID)
	require.NoError(t, err)
	return roleID
}

// TestDepreciation_HTTP_Impairment_FieldMasking verifies POST
// /assets/:id/impairment respects the same "assets" field-permission policy
// as assetSchedule (handler.go's maskedAssetScheduleMap guard): a role denied
// view on assets.book_value must not see book_value OR
// accumulated_depreciation in the impairment response — they are masked
// together, since accumulated_depreciation is not independently exposed on
// this endpoint — while impairment_loss (no assets field policy exists for
// it) stays visible. A role with no explicit "assets" field policy at all
// (default-allow) sees every field.
func TestDepreciation_HTTP_Impairment_FieldMasking(t *testing.T) {
	hh := newHTTPHarness(t)
	ctx := context.Background()

	t.Run("masked when book_value view is denied", func(t *testing.T) {
		roleID := seedManageRole(t, hh.pool, "DprMaskDeny-"+uuid.New().String()[:8])
		testsupport.SeedFieldPermission(t, hh.pool, roleID, "assets", "book_value", false, false)
		userID := seedUser(t, hh.pool, roleID, hh.office, "mask.deny."+uuid.New().String()[:8]+"@test.local")

		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00040", "Aset Impairment Masked", hh.catID, hh.office, "9600000.00", 6)
		month1 := firstOfMonthUTC(time.Now())
		period1 := month1.Format("2006-01")
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", userID, roleID)
		require.Equal(t, http.StatusOK, code)

		a, err := hh.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		require.NotNil(t, a.BookValue)

		body := []byte(`{"recoverable_amount":"100000.00","reason":"uji masking"}`)
		code, respBody := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", userID, roleID, body)
		require.Equal(t, http.StatusOK, code)
		assert.NotContains(t, respBody, "book_value")
		assert.NotContains(t, respBody, "accumulated_depreciation")
		assert.Contains(t, respBody, "impairment_loss", "impairment_loss has no assets field policy and stays visible")
	})

	t.Run("unmasked when no field policy exists (default-allow)", func(t *testing.T) {
		roleID := seedManageRole(t, hh.pool, "DprMaskAllow-"+uuid.New().String()[:8])
		userID := seedUser(t, hh.pool, roleID, hh.office, "mask.allow."+uuid.New().String()[:8]+"@test.local")

		assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00041", "Aset Impairment Unmasked", hh.catID, hh.office, "9600000.00", 6)
		month1 := firstOfMonthUTC(time.Now())
		period1 := month1.Format("2006-01")
		code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", userID, roleID)
		require.Equal(t, http.StatusOK, code)

		a, err := hh.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		require.NotNil(t, a.BookValue)

		body := []byte(`{"recoverable_amount":"100000.00","reason":"uji default-allow"}`)
		code, respBody := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", userID, roleID, body)
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, respBody, "book_value")
		assert.Contains(t, respBody, "accumulated_depreciation")
		assert.Contains(t, respBody, "impairment_loss")
	})
}

// TestDepreciation_Impairment_ProspectiveRecompute verifies the normative
// engine-integration rule end-to-end: impair an asset after closing month1,
// then compute month2 — the next month's amount must be
// (recoverable − salvage) / remaining, prospectively picking up the impaired
// base, while month1 (closed history) stays byte-identical.
func TestDepreciation_Impairment_ProspectiveRecompute(t *testing.T) {
	hh := newHTTPHarness(t)
	ctx := context.Background()

	// cost 9,600,000; category salvage rate 10% -> salvage 960,000; life 48m.
	// Purchased 6 months ago: month1 (current-1) is the 6th generated month
	// (index 5), closing = 9,600,000 - 6*180,000 = 8,520,000.00.
	assetID := seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00027", "Aset Prospektif", hh.catID, hh.office, "9600000.00", 6)

	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	month2 := firstOfMonthUTC(time.Now())
	period1 := month1.Format("2006-01")
	period2 := month2.Format("2006-01")

	code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period1+"/compute", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, hh.svc.ClosePeriod(ctx, month1, hh.actorID))

	a, err := hh.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, a.BookValue)
	require.Equal(t, "8520000.00", *a.BookValue)

	body := []byte(`{"recoverable_amount":"8000000.00","reason":"uji penurunan nilai"}`)
	code, respBody := hh.doReqBody(http.MethodPost, "/api/v1/assets/"+assetID.String()+"/impairment", hh.actorID, hh.roleID, body)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "8000000.00", respBody["book_value"])

	code, _ = hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period2+"/compute", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	entries, err := hh.q.ListAssetEntries(ctx, assetID)
	require.NoError(t, err)

	var m1Commercial, m2Commercial sqlc.DepreciationDepreciationEntry
	for _, e := range entries {
		if e.Basis != sqlc.SharedDepreciationBasisCommercial {
			continue
		}
		if e.Period.Time.Equal(month1) {
			m1Commercial = e
		}
		if e.Period.Time.Equal(month2) {
			m2Commercial = e
		}
	}
	require.NotEmpty(t, m1Commercial.ID, "month1 entry must exist")
	assert.Equal(t, "8520000.00", m1Commercial.ClosingValue, "closed history must not be rewritten by the impairment")

	require.NotEmpty(t, m2Commercial.ID)
	assert.Equal(t, "8000000.00", m2Commercial.OpeningValue, "month2 must open from the impaired book_value, not month1's closing")
	// remaining = 48 - monthsElapsed(start, month2) = 48 - 6 = 42.
	// amount = (8,000,000 - 960,000) / 42 = 7,040,000/42 = 167,619.047619... -> 167619.05
	assert.Equal(t, "167619.05", m2Commercial.DepreciationAmount)
	assert.Equal(t, "7832380.95", m2Commercial.ClosingValue, "8,000,000.00 - 167,619.05")
}

// TestDepreciation_Impairment_RowLock_NoLostUpdate is the deterministic
// concurrent-impairment guard (the held-tx pattern from
// TestDepreciation_ClosePeriod_BlocksOnComputeLock): a foreign transaction
// row-locks the asset (standing in for an in-flight first impairment),
// launches RecordImpairment concurrently, applies the "first" impairment's
// values from the holding tx, commits, and then requires the concurrent call
// to have re-read the POST-commit book_value — i.e. it must be rejected with
// ErrInvalidRecoverable (its recoverable 8,000,000 >= the new book 7,000,000)
// and the first impairment's values must survive untouched. Without the
// FOR UPDATE read inside RecordImpairment, the concurrent call reads the
// stale pre-commit book (8,520,000) up front, blocks only at the UPDATE, and
// then clobbers the first write (book back UP to 8,000,000, loss understated
// at 520,000) — a silent lost update.
func TestDepreciation_Impairment_RowLock_NoLostUpdate(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00028", "Aset Lock Impairment", h.catID, h.office, "9600000.00", 6)
	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)

	a, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, a.BookValue)
	require.Equal(t, "8520000.00", *a.BookValue)

	// Foreign tx: row-lock the asset, as RecordImpairment's own tx would.
	lockTx, err := h.pool.Begin(ctx)
	require.NoError(t, err)
	_, err = lockTx.Exec(ctx, `SELECT 1 FROM asset.assets WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, assetID)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, errImp := h.svc.RecordImpairment(ctx, assetID, "8000000.00", "uji konkuren", h.actorID)
		done <- errImp
	}()

	select {
	case errImp := <-done:
		require.NoError(t, lockTx.Rollback(ctx))
		t.Fatalf("RecordImpairment completed (err=%v) while the asset row lock was held — it must block", errImp)
	case <-time.After(750 * time.Millisecond):
		// Still blocked — desired. (Note: even WITHOUT the FOR UPDATE read it
		// blocks here too, at its UPDATE; the discriminating assertion is the
		// post-commit outcome below, not the blocking itself.)
	}

	// "First impairment" wins the race: write exactly what RecordImpairment
	// to recoverable 7,000,000 would persist, then commit (releasing the lock).
	_, err = lockTx.Exec(ctx,
		`UPDATE asset.assets SET book_value = '7000000.00', impairment_loss = '1520000.00' WHERE id = $1`, assetID)
	require.NoError(t, err)
	require.NoError(t, lockTx.Commit(ctx))

	select {
	case errImp := <-done:
		// The concurrent call must have re-read book_value AFTER the commit:
		// its recoverable 8,000,000 >= the new book 7,000,000 → rejected.
		require.ErrorIs(t, errImp, depreciation.ErrInvalidRecoverable,
			"concurrent impairment must re-read the post-commit book value, not clobber from a stale read")
	case <-time.After(5 * time.Second):
		t.Fatal("RecordImpairment still blocked after the row lock was released")
	}

	final, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, final.BookValue)
	require.NotNil(t, final.ImpairmentLoss)
	assert.Equal(t, "7000000.00", *final.BookValue, "first impairment's book_value must survive (no lost update)")
	assert.Equal(t, "1520000.00", *final.ImpairmentLoss, "first impairment's cumulative loss must survive (not understated)")
}

// TestDepreciation_Impairment_ConcurrentInvariant races two real
// RecordImpairment calls (recoverables 8,000,000 and 7,000,000 against book
// 8,520,000) and asserts the serial-equivalence invariant that must hold
// under EITHER lock-acquisition ordering:
//   - 8M first, then 7M: both succeed (deltas 520,000 then 1,000,000).
//   - 7M first, then 8M: 7M succeeds (delta 1,520,000); 8M is rejected with
//     ErrInvalidRecoverable (8,000,000 >= new book 7,000,000).
//
// In both orderings the final state is identical — book_value 7,000,000 and
// impairment_loss 1,520,000 (== original book − final book) — and the ONLY
// permissible error is ErrInvalidRecoverable on the 8M call. A lost update
// (book 8,000,000 / loss 520,000 with no error) fails these assertions.
func TestDepreciation_Impairment_ConcurrentInvariant(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00029", "Aset Invarian Impairment", h.catID, h.office, "9600000.00", 6)
	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	var err8M, err7M error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err8M = h.svc.RecordImpairment(ctx, assetID, "8000000.00", "uji balapan 8M", h.actorID)
	}()
	go func() {
		defer wg.Done()
		_, err7M = h.svc.RecordImpairment(ctx, assetID, "7000000.00", "uji balapan 7M", h.actorID)
	}()
	wg.Wait()

	// The 7M call must always succeed (7,000,000 is below the book value it
	// observes under either ordering); the 8M call either succeeded (it ran
	// first) or was rejected because 7M had already lowered the book.
	require.NoError(t, err7M)
	if err8M != nil {
		require.ErrorIs(t, err8M, depreciation.ErrInvalidRecoverable)
	}

	final, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, final.BookValue)
	require.NotNil(t, final.ImpairmentLoss)
	assert.Equal(t, "7000000.00", *final.BookValue, "final book must equal the lowest applied recoverable")
	assert.Equal(t, "1520000.00", *final.ImpairmentLoss, "loss must equal original book (8,520,000) − final book (7,000,000) — never a silently-lost delta")
}

// TestDepreciation_Impairment_BlocksOnComputeLock verifies RecordImpairment
// serializes against ComputePeriod/ClosePeriod via the SAME depreciation
// advisory lock (channel pattern from ClosePeriod_BlocksOnComputeLock).
// Without it, an impairment can commit mid-compute and the compute's
// refreshAssetSummary then rewrites asset.book_value from the entries —
// clobbering the impaired value back up while impairment_loss keeps the
// write-down (inconsistent state; the next recompute's min() override sees
// the clobbered-higher book_value, so the impairment is effectively lost).
func TestDepreciation_Impairment_BlocksOnComputeLock(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00030", "Aset Advisory Impairment", h.catID, h.office, "9600000.00", 6)
	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)

	// Hold the depreciation advisory lock in a foreign transaction, standing
	// in for an in-flight ComputePeriod (which holds it for its whole run).
	lockTx, err := h.pool.Begin(ctx)
	require.NoError(t, err)
	_, err = lockTx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('depreciation.compute'))`)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, errImp := h.svc.RecordImpairment(ctx, assetID, "8000000.00", "uji advisory lock", h.actorID)
		done <- errImp
	}()

	select {
	case errImp := <-done:
		require.NoError(t, lockTx.Rollback(ctx))
		t.Fatalf("RecordImpairment completed (err=%v) while the depreciation advisory lock was held — it must block", errImp)
	case <-time.After(750 * time.Millisecond):
		// Still blocked — the desired behavior.
	}

	require.NoError(t, lockTx.Rollback(ctx), "release the advisory lock")

	select {
	case errImp := <-done:
		require.NoError(t, errImp, "RecordImpairment must succeed once the lock is released")
	case <-time.After(5 * time.Second):
		t.Fatal("RecordImpairment still blocked after the advisory lock was released")
	}

	final, err := h.q.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.NotNil(t, final.BookValue)
	require.NotNil(t, final.ImpairmentLoss)
	assert.Equal(t, "8000000.00", *final.BookValue)
	assert.Equal(t, "520000.00", *final.ImpairmentLoss, "8,520,000 − 8,000,000")
}

// ─── HTTP-wiring tests (Task 6 — journal export xlsx/PDF) ─────────────────

// TestDepreciation_HTTP_JournalExport drives GET /depreciation/journal/export
// end-to-end for both supported formats: the xlsx body must be a well-formed
// zip container (PK magic bytes) of non-trivial size with the documented
// content-type + Content-Disposition; the pdf body must start with the PDF
// magic bytes. An unrecognized format is rejected with 400 (same as an
// invalid period), and the view permission gate matches the plain journal
// endpoint (Staf lacks depreciation.view → 403).
func TestDepreciation_HTTP_JournalExport(t *testing.T) {
	hh := newHTTPHarness(t)
	seedAssetMonthsAgo(t, hh.pool, "DPR-2026-00031", "Aset Ekspor Jurnal", hh.catID, hh.office, "12000000.00", 2)

	target := firstOfMonthUTC(time.Now())
	period := target.Format("2006-01")

	code, _ := hh.doReq(http.MethodPost, "/api/v1/depreciation/periods/"+period+"/compute", hh.actorID, hh.roleID)
	require.Equal(t, http.StatusOK, code)

	t.Run("xlsx format", func(t *testing.T) {
		code, headers, body := hh.doReqRaw(http.MethodGet,
			"/api/v1/depreciation/journal/export?period="+period+"&basis=commercial&format=xlsx", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		require.Greater(t, len(body), 100, "xlsx body must be non-trivial")
		assert.Equal(t, []byte("PK\x03\x04"), body[:4], "xlsx is a zip container (PK magic bytes)")
		assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", headers.Get("Content-Type"))
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"), "binary downloads must set nosniff (codebase download convention)")
		wantDisposition := fmt.Sprintf(`attachment; filename="jurnal-penyusutan-%s-commercial.xlsx"`, period)
		assert.Equal(t, wantDisposition, headers.Get("Content-Disposition"))
	})

	t.Run("pdf format", func(t *testing.T) {
		code, headers, body := hh.doReqRaw(http.MethodGet,
			"/api/v1/depreciation/journal/export?period="+period+"&basis=commercial&format=pdf", hh.actorID, hh.roleID)
		require.Equal(t, http.StatusOK, code)
		require.Greater(t, len(body), 100, "pdf body must be non-trivial")
		assert.Equal(t, []byte("%PDF"), body[:4], "pdf magic bytes")
		assert.Equal(t, "application/pdf", headers.Get("Content-Type"))
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"), "binary downloads must set nosniff (codebase download convention)")
		wantDisposition := fmt.Sprintf(`attachment; filename="jurnal-penyusutan-%s-commercial.pdf"`, period)
		assert.Equal(t, wantDisposition, headers.Get("Content-Disposition"))
	})

	t.Run("unknown format -> 400", func(t *testing.T) {
		code, _, _ := hh.doReqRaw(http.MethodGet,
			"/api/v1/depreciation/journal/export?period="+period+"&basis=commercial&format=bogus", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("invalid period -> 400", func(t *testing.T) {
		code, _, _ := hh.doReqRaw(http.MethodGet,
			"/api/v1/depreciation/journal/export?period=not-a-period&basis=commercial&format=xlsx", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("invalid basis -> 400", func(t *testing.T) {
		code, _, _ := hh.doReqRaw(http.MethodGet,
			"/api/v1/depreciation/journal/export?period="+period+"&basis=bogus&format=xlsx", hh.actorID, hh.roleID)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("permission gate: Staf lacks depreciation.view -> 403", func(t *testing.T) {
		stafRoleID := lookupRole(t, hh.pool, "Staf")
		stafUser := seedUser(t, hh.pool, stafRoleID, hh.office, "staf.journalexport."+uuid.New().String()[:8]+"@test.local")
		code, _, _ := hh.doReqRaw(http.MethodGet,
			"/api/v1/depreciation/journal/export?period="+period+"&basis=commercial&format=xlsx", stafUser, stafRoleID)
		assert.Equal(t, http.StatusForbidden, code)
	})
}

// ─── Task 2 — SQL-aggregated, paginated schedule ───────────────────────────

// TestSchedulePaginationAndParity seeds three assets exercising the three row
// kinds Schedule() must union in SQL now:
//
//   - "Sched A Alpha Berjalan" — a normal, still-depreciating asset (real
//     entry this period). Cost 18,500,000.00, purchased 3 months ago; same
//     numbers as TestDepreciation_Compute_HappyPath, so the anchor values
//     (amount 346,875.00, closing 17,112,500.00, accumulated 1,387,500.00)
//     are independently known-correct.
//   - "Sched B Beta Impairment" — an impaired asset (real entry this period,
//     but closing != cost-accumulated because the write-down isn't part of
//     `accumulated`, which is a pure SUM of depreciation_amount entries).
//     Cost 9,600,000.00, purchased 6 months ago; same numbers as
//     TestDepreciation_Impairment_ProspectiveRecompute, so month2's opening
//     8,000,000.00 / amount 167,619.05 / closing 7,832,380.95 are
//     independently known-correct.
//   - "Sched C Gamma Habis" — a fully-depreciated asset with NO entry this
//     period (synthetic union row). Cost 1,000,000.00, useful_life_months=1,
//     salvage 0, purchased 5 months ago; same numbers as
//     TestDepreciation_HTTP_Schedule's "exhausted" asset.
//
// Names are chosen to sort A < B < C under the query's `ORDER BY a.name,
// a.id` so pagination order is deterministic.
func TestSchedulePaginationAndParity(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Asset A: normal, still depreciating.
	seedAssetMonthsAgo(t, h.pool, "DPR-2026-00050", "Sched A Alpha Berjalan", h.catID, h.office, "18500000.00", 3)

	// Asset B: impaired.
	assetB := seedAssetMonthsAgo(t, h.pool, "DPR-2026-00051", "Sched B Beta Impairment", h.catID, h.office, "9600000.00", 6)

	// Asset C: fully depreciated, no entry this period (union row).
	exhaustedPurchase := firstOfMonthUTC(time.Now()).AddDate(0, -5, 0)
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status,
		    purchase_date, purchase_cost, useful_life_months, salvage_value, depreciation_method)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', 'available', $5, $6, 1, '0', 'straight_line')
		 RETURNING id`,
		"DPR-2026-00052", "Sched C Gamma Habis", h.catID, h.office, exhaustedPurchase, "1000000.00").Scan(new(uuid.UUID)))

	month1 := firstOfMonthUTC(time.Now()).AddDate(0, -1, 0)
	target := firstOfMonthUTC(time.Now())

	// Compute+close month1 (asset B's book value lands at 8,520,000.00),
	// impair asset B down to 8,000,000.00, then compute the open target month
	// — this generates asset A's and B's target-period entries in one shot;
	// asset C never gets one (exhausted long before target).
	_, err := h.svc.ComputePeriod(ctx, month1, h.actorID)
	require.NoError(t, err)
	require.NoError(t, h.svc.ClosePeriod(ctx, month1, h.actorID))

	assetBAfterClose, err := h.q.GetAsset(ctx, assetB)
	require.NoError(t, err)
	require.NotNil(t, assetBAfterClose.BookValue)
	require.Equal(t, "8520000.00", *assetBAfterClose.BookValue, "sanity: matches TestDepreciation_Impairment_ProspectiveRecompute's anchor")

	_, err = h.svc.RecordImpairment(ctx, assetB, "8000000.00", "uji parity jadwal", h.actorID)
	require.NoError(t, err)

	_, err = h.svc.ComputePeriod(ctx, target, h.actorID)
	require.NoError(t, err)

	commercial := sqlc.SharedDepreciationBasisCommercial

	// Full page (limit big enough for all rows).
	full, err := h.svc.Schedule(ctx, target, commercial, true, nil, "", nil, nil, 100, 0)
	require.NoError(t, err)
	require.Len(t, full.Rows, 3, "asset A entry row + asset B entry row (impaired) + asset C union row")
	require.Equal(t, int64(len(full.Rows)), full.Total)

	// Row order is deterministic (ORDER BY a.name, a.id): A, B, C.
	require.Equal(t, "Sched A Alpha Berjalan", full.Rows[0].AssetName)
	require.Equal(t, "Sched B Beta Impairment", full.Rows[1].AssetName)
	require.Equal(t, "Sched C Gamma Habis", full.Rows[2].AssetName)

	rowA, rowB, rowC := full.Rows[0], full.Rows[1], full.Rows[2]

	// Asset A: known-correct anchor values (TestDepreciation_Compute_HappyPath).
	assert.Equal(t, "346875.00", rowA.Amount)
	assert.Equal(t, "17112500.00", rowA.Closing)
	assert.Equal(t, "1387500.00", rowA.Accumulated)
	assert.False(t, rowA.FullyDepreciated)
	assert.False(t, rowA.Impaired)

	// Asset B: known-correct anchor values (TestDepreciation_Impairment_ProspectiveRecompute).
	// accumulated is the pure SUM of depreciation_amount entries (6*180,000.00
	// through month1, plus month2's 167,619.05) — it does NOT absorb the
	// impairment write-down, so closing != cost - accumulated by exactly the
	// impairment_loss (520,000.00): 9,600,000.00 - 1,247,619.05 = 8,352,380.95,
	// but the entry's actual closing is 7,832,380.95.
	assert.Equal(t, "167619.05", rowB.Amount)
	assert.Equal(t, "7832380.95", rowB.Closing)
	assert.Equal(t, "1247619.05", rowB.Accumulated)
	assert.False(t, rowB.FullyDepreciated)
	assert.True(t, rowB.Impaired, "asset B carries a positive impairment_loss")
	costB, err := parseMoneyForTest(t, "9600000.00")
	require.NoError(t, err)
	accB, err := parseMoneyForTest(t, rowB.Accumulated)
	require.NoError(t, err)
	costMinusAccB := new(big.Rat).Sub(costB, accB)
	closingB, err := parseMoneyForTest(t, rowB.Closing)
	require.NoError(t, err)
	assert.NotEqual(t, costMinusAccB.RatString(), closingB.RatString(),
		"impaired asset: closing must diverge from cost-accumulated (the write-down isn't in `accumulated`)")

	// Asset C: known-correct anchor values (TestDepreciation_HTTP_Schedule).
	assert.Equal(t, "0.00", rowC.Amount)
	assert.Equal(t, "0.00", rowC.Opening)
	assert.Equal(t, "0.00", rowC.Closing)
	assert.True(t, rowC.FullyDepreciated)

	// KPI/Totals: SQL-aggregated sums across all three rows.
	assert.Equal(t, "29100000.00", full.KPI.TotalCost, "18,500,000 + 9,600,000 + 1,000,000")
	assert.Equal(t, "3635119.05", full.KPI.TotalAccumulated, "1,387,500.00 + 1,247,619.05 + 1,000,000.00")
	assert.Equal(t, "24944880.95", full.KPI.TotalBookValue, "17,112,500.00 + 7,832,380.95 + 0.00")
	assert.Equal(t, "514494.05", full.KPI.PeriodExpense, "346,875.00 + 167,619.05 + 0 (asset C has no entry this period)")
	assert.Equal(t, full.KPI.PeriodExpense, full.Totals.Amount, "tfoot amount must match the KPI period-expense tile when unfiltered")
	assert.Equal(t, "25459375.00", full.Totals.Opening, "17,459,375.00 + 8,000,000.00 + 0.00")
	assert.Equal(t, full.KPI.TotalAccumulated, full.Totals.Accumulated)
	assert.Equal(t, full.KPI.TotalBookValue, full.Totals.Closing)

	// Page 1 of size 2 + page 2 of size 2 == the full ordered set, no overlap.
	p1, err := h.svc.Schedule(ctx, target, commercial, true, nil, "", nil, nil, 2, 0)
	require.NoError(t, err)
	p2, err := h.svc.Schedule(ctx, target, commercial, true, nil, "", nil, nil, 2, 2)
	require.NoError(t, err)
	require.Len(t, p1.Rows, 2)
	require.Len(t, p2.Rows, 1)
	assert.Equal(t, full.Total, p1.Total, "total is unaffected by paging")
	assert.Equal(t, full.Total, p2.Total, "total is unaffected by paging")
	assert.Equal(t, full.Rows[0].AssetID, p1.Rows[0].AssetID)
	assert.Equal(t, full.Rows[1].AssetID, p1.Rows[1].AssetID)
	assert.Equal(t, full.Rows[2].AssetID, p2.Rows[0].AssetID)
	assert.NotEqual(t, p1.Rows[0].AssetID, p2.Rows[0].AssetID, "no overlap between pages")
	assert.NotEqual(t, p1.Rows[1].AssetID, p2.Rows[0].AssetID, "no overlap between pages")

	// A search filter shrinks rows/total/tfoot but NOT the kpi tiles.
	filtered, err := h.svc.Schedule(ctx, target, commercial, true, nil, full.Rows[0].AssetName, nil, nil, 100, 0)
	require.NoError(t, err)
	assert.Less(t, filtered.Total, full.Total)
	require.Len(t, filtered.Rows, 1)
	assert.Equal(t, full.Rows[0].AssetID, filtered.Rows[0].AssetID)
	assert.NotEqual(t, full.Totals.Amount, filtered.Totals.Amount, "tfoot must shrink under the search filter")
	assert.Equal(t, full.KPI.TotalCost, filtered.KPI.TotalCost, "kpi tiles must NOT shrink under the search filter")
	assert.Equal(t, full.KPI.TotalAccumulated, filtered.KPI.TotalAccumulated)
	assert.Equal(t, full.KPI.TotalBookValue, filtered.KPI.TotalBookValue)
	assert.Equal(t, full.KPI.PeriodExpense, filtered.KPI.PeriodExpense)

	// category_id/office_id filters behave the same way (row-level, not kpi).
	byCategory, err := h.svc.Schedule(ctx, target, commercial, true, nil, "", &h.catID, nil, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, full.Total, byCategory.Total, "all seeded assets share the one test category")

	otherCategory := seedCategory(t, h.pool, "OTH"+uuid.New().String()[:4], 36, "0.05", sqlc.SharedFiscalAssetGroupKelompok1)
	byOtherCategory, err := h.svc.Schedule(ctx, target, commercial, true, nil, "", &otherCategory, nil, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), byOtherCategory.Total, "no seeded asset belongs to this category")
	assert.Equal(t, full.KPI.TotalCost, byOtherCategory.KPI.TotalCost, "kpi tiles ignore the category_id filter too")
}

// parseMoneyForTest exposes engine.go's unexported parseMoney to this
// external test package via the one exported Schedule/Journal-adjacent path
// available — computed inline instead, since parseMoney is unexported.
func parseMoneyForTest(t *testing.T, s string) (*big.Rat, error) {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("invalid money string %q", s)
	}
	return r, nil
}
