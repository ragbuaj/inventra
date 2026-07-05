//go:build integration

package depreciation_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/depreciation"
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

	return &harness{pool: pool, q: q, svc: svc, office: office, catID: catID, actorID: actorID}
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
