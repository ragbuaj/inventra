//go:build integration

package report_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/report"
)

// ─── report-builder harness ──────────────────────────────────────────────────

// env is a minimal shared scaffold (two offices, two categories, one
// employee/user) that each report-builder test hangs its own assets and
// records off — kept separate from seedFixture so its counts stay stable.
type env struct {
	officeA, officeB   uuid.UUID
	cat1, cat2         uuid.UUID
	cat1Name, cat2Name string
	emp, user          uuid.UUID
}

func baseEnv(t *testing.T, pool *pgxpool.Pool) env {
	t.Helper()
	ctx := context.Background()
	sfx := uuid.New().String()[:8]
	var e env
	e.cat1Name, e.cat2Name = "Kat1 "+sfx, "Kat2 "+sfx

	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		"T "+sfx).Scan(&typeID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`, typeID, "A "+sfx, "OA"+sfx).Scan(&e.officeA))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`, typeID, "B "+sfx, "OB"+sfx).Scan(&e.officeB))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.categories (name, code) VALUES ($1, $2) RETURNING id`,
		e.cat1Name, "C1"+sfx).Scan(&e.cat1))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.categories (name, code) VALUES ($1, $2) RETURNING id`,
		e.cat2Name, "C2"+sfx).Scan(&e.cat2))

	roleID := lookupRole(t, pool, "Superadmin")
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"u "+sfx, "u."+sfx+"@t.local", roleID, e.officeA).Scan(&e.user))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.employees (code, name, office_id) VALUES ($1, $2, $3) RETURNING id`,
		"E"+sfx, "Emp "+sfx, e.officeA).Scan(&e.emp))
	return e
}

func insertDepr(t *testing.T, pool *pgxpool.Pool, asset uuid.UUID, basis, period, opening, amount, closing string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO depreciation.depreciation_entries
		   (asset_id, basis, period, opening_value, depreciation_amount, closing_value, method)
		 VALUES ($1, $2, $3, $4, $5, $6, 'straight_line')`,
		asset, basis, period, opening, amount, closing)
	require.NoError(t, err)
}

func insertAssignment(t *testing.T, pool *pgxpool.Pool, asset, emp, user uuid.UUID, checkout string, checkin *string, status string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO assignment.assignments
		   (asset_id, employee_id, assigned_by_id, checkout_date, checkin_date, status)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		asset, emp, user, checkout, checkin, status)
	require.NoError(t, err)
}

func insertMaint(t *testing.T, pool *pgxpool.Pool, asset uuid.UUID, mtype, status, completed, cost string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO maintenance.maintenance_records
		   (asset_id, type, status, completed_date, cost, description)
		 VALUES ($1, $2, $3, $4, $5, 'rec')`,
		asset, mtype, status, completed, cost)
	require.NoError(t, err)
}

// intangibleAsset seeds a roomless (intangible) asset so tests need not wire a
// floor/room. Cost/book are irrelevant unless the test asserts on them.
func intangibleAsset(t *testing.T, pool *pgxpool.Pool, e env, cat, office uuid.UUID, tag, name string) uuid.UUID {
	t.Helper()
	return insertAsset(t, pool, assetSeed{
		tag: tag, name: name, category: cat, office: office,
		room: nil, class: "intangible", status: "available",
		cost: "1000.00", book: "1000.00", purchaseDate: d(refToday()), excluded: false,
	})
}

func customPeriod(t *testing.T, from, to string) (report.DateRange, report.DateRange) {
	t.Helper()
	cur, prev, err := report.ResolvePeriod("", from, to, time.Now().UTC())
	require.NoError(t, err)
	return cur, prev
}

func kpiMap(r report.ReportResult) map[string]string {
	m := map[string]string{}
	for _, k := range r.Kpis {
		m[k.Key] = k.Value
	}
	return m
}

func aScope(e env) report.ReportParams {
	return report.ReportParams{All: false, OfficeIDs: []uuid.UUID{e.officeA}, RowLimit: 1000}
}

func strptr(s string) *string { return &s }

// ─── assets ───────────────────────────────────────────────────────────────────

// TestReportAssetsScopeExclusionOrdering drives the A-only scope over the shared
// seedFixture: office B is invisible, rows are ordered by tag and list every
// asset (including the excluded A3), while the money KPIs/Totals/chart drop the
// excluded asset.
func TestReportAssetsScopeExclusionOrdering(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)

	res, err := svc.Run(context.Background(), "assets", report.ReportParams{
		All: false, OfficeIDs: []uuid.UUID{f.officeA}, RowLimit: 1000,
	})
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.AssetRow)
	require.True(t, ok, "rows are []AssetRow")
	require.Len(t, rows, 3, "office B invisible; A1/A2/A3 only")

	// Ordered by asset_tag: A1, A2, A3.
	assert.Equal(t, "A1-"+f.sfx, rows[0].Tag)
	assert.Equal(t, "A2-"+f.sfx, rows[1].Tag)
	assert.Equal(t, "A3-"+f.sfx, rows[2].Tag)
	for _, r := range rows {
		assert.NotEqual(t, "B1-"+f.sfx, r.Tag, "office B asset must not appear")
	}

	// A1 content.
	assert.Equal(t, f.assetA1Name, rows[0].Name)
	assert.Equal(t, f.catName, rows[0].Category)
	assert.Equal(t, "assigned", rows[0].Status)
	assert.Equal(t, "100000000.00", rows[0].PurchaseCost)
	assert.Equal(t, "0.00", rows[0].AccumDeprec)
	assert.Equal(t, "90000000.00", rows[0].BookValue)

	// A3 is the excluded asset — present in rows (with its real money), dropped
	// from the money aggregates.
	assert.Equal(t, "under_maintenance", rows[2].Status)
	assert.Equal(t, "28000000.00", rows[2].BookValue)

	// Totals exclude A3 (100M+50M cost, 90M+45M book, 0 accum).
	assert.Equal(t, "150000000.00", res.Totals["purchase_cost"])
	assert.Equal(t, "0.00", res.Totals["accum_deprec"])
	assert.Equal(t, "135000000.00", res.Totals["book_value"])

	k := kpiMap(res)
	assert.Equal(t, "3", k["total_assets"], "count includes the excluded asset")
	assert.Equal(t, "150000000.00", k["total_acquisition"])
	assert.Equal(t, "135000000.00", k["total_book"])

	// Chart: single category, book value net of the excluded asset.
	require.Len(t, res.Chart, 1)
	assert.Equal(t, f.catName, res.Chart[0].Label)
	assert.Equal(t, "135000000.00", res.Chart[0].Value)

	assert.EqualValues(t, 3, res.RowCount)
	assert.False(t, res.Truncated)
	assert.Equal(t, "assets", res.Type)
}

// TestReportAssetsAllScope: global scope sees office B, and the money totals add
// B1 (777 cost, 500 book) on top of A's non-excluded assets.
func TestReportAssetsAllScope(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)

	res, err := svc.Run(context.Background(), "assets", report.ReportParams{All: true, RowLimit: 1000})
	require.NoError(t, err)

	rows := res.Rows.([]report.AssetRow)
	require.Len(t, rows, 4)
	assert.EqualValues(t, 4, res.RowCount)
	k := kpiMap(res)
	assert.Equal(t, "4", k["total_assets"])
	assert.Equal(t, "150000777.00", k["total_acquisition"])
	assert.Equal(t, "135000500.00", k["total_book"])
	_ = f
}

// TestReportAssetsTruncation: a limit below the total row count truncates the
// rows but RowCount still reports the true total.
func TestReportAssetsTruncation(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)

	res, err := svc.Run(context.Background(), "assets", report.ReportParams{
		All: false, OfficeIDs: []uuid.UUID{f.officeA}, RowLimit: 2,
	})
	require.NoError(t, err)

	rows := res.Rows.([]report.AssetRow)
	assert.Len(t, rows, 2, "limited to 2")
	assert.EqualValues(t, 3, res.RowCount, "true total")
	assert.True(t, res.Truncated)
}

// TestReportAssetsStatusFilter narrows to a single status.
func TestReportAssetsStatusFilter(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)

	p := report.ReportParams{
		All: false, OfficeIDs: []uuid.UUID{f.officeA}, RowLimit: 1000,
		Status: strptr("available"),
	}
	res, err := svc.Run(context.Background(), "assets", p)
	require.NoError(t, err)

	rows := res.Rows.([]report.AssetRow)
	require.Len(t, rows, 1, "only A2 is available in scope (B1 is out of scope)")
	assert.Equal(t, "A2-"+f.sfx, rows[0].Tag)
	assert.Equal(t, "available", rows[0].Status)
	assert.Equal(t, "1", kpiMap(res)["total_assets"])
	assert.Equal(t, "50000000.00", res.Totals["purchase_cost"])
}

// TestReportAssetsCategoryFilter: a two-category scope keeps only the matching
// category's assets.
func TestReportAssetsCategoryFilter(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)
	intangibleAsset(t, pool, e, e.cat1, e.officeA, "K1", "Aset Kat1")
	intangibleAsset(t, pool, e, e.cat2, e.officeA, "K2", "Aset Kat2")

	p := aScope(e)
	p.CategoryID = &e.cat1
	res, err := svc.Run(context.Background(), "assets", p)
	require.NoError(t, err)

	rows := res.Rows.([]report.AssetRow)
	require.Len(t, rows, 1)
	assert.Equal(t, "K1", rows[0].Tag)
	assert.Equal(t, e.cat1Name, rows[0].Category)

	// A non-matching category yields nothing.
	other := uuid.New()
	p2 := aScope(e)
	p2.CategoryID = &other
	res2, err := svc.Run(context.Background(), "assets", p2)
	require.NoError(t, err)
	assert.Empty(t, res2.Rows.([]report.AssetRow))
	assert.EqualValues(t, 0, res2.RowCount)
}

// ─── depreciation ─────────────────────────────────────────────────────────────

// TestReportDepreciation drives a custom period across a per-month depreciation
// series and asserts row selection, boundary inclusivity, the three KPIs, the
// period totals, basis isolation, and office scope.
func TestReportDepreciation(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)
	asset := intangibleAsset(t, pool, e, e.cat1, e.officeA, "D1", "Aset Depr")

	// Commercial monthly series (period = first of month).
	insertDepr(t, pool, asset, "commercial", "2025-02-01", "100000000.00", "1000000.00", "99000000.00") // before from
	insertDepr(t, pool, asset, "commercial", "2025-03-01", "99000000.00", "2000000.00", "97000000.00")  // == from
	insertDepr(t, pool, asset, "commercial", "2025-04-01", "97000000.00", "2000000.00", "95000000.00")  // in
	insertDepr(t, pool, asset, "commercial", "2025-05-01", "95000000.00", "2000000.00", "93000000.00")  // in / == last month ≤ to
	insertDepr(t, pool, asset, "commercial", "2025-06-01", "93000000.00", "3000000.00", "90000000.00")  // after to
	// A fiscal entry inside the window must be invisible to a commercial run.
	insertDepr(t, pool, asset, "fiscal", "2025-04-01", "50000000.00", "9999999.00", "40000000.00")
	// Office B commercial entry inside the window — out of an A-only scope.
	assetB := intangibleAsset(t, pool, e, e.cat1, e.officeB, "D1B", "Aset Depr B")
	insertDepr(t, pool, assetB, "commercial", "2025-04-01", "10000000.00", "5000000.00", "5000000.00")

	cur, prev := customPeriod(t, "2025-03-01", "2025-05-31")
	p := aScope(e)
	p.Basis = "commercial"
	p.Cur, p.Prev = cur, prev

	res, err := svc.Run(context.Background(), "depreciation", p)
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.DeprRow)
	require.True(t, ok)
	require.Len(t, rows, 3, "Feb (before) and Jun (after) excluded; fiscal + office-B excluded")
	assert.Equal(t, []string{"2025-03", "2025-04", "2025-05"},
		[]string{rows[0].Period, rows[1].Period, rows[2].Period}, "ordered ascending")
	assert.Equal(t, "2000000.00", rows[0].Amount)

	// Period totals (sums over the returned rows).
	assert.Equal(t, "291000000.00", res.Totals["opening"], "99M+97M+95M")
	assert.Equal(t, "6000000.00", res.Totals["amount"], "3×2M")
	assert.Equal(t, "285000000.00", res.Totals["closing"], "97M+95M+93M")

	k := kpiMap(res)
	assert.Equal(t, "6000000.00", k["period_expense"], "in-period expense only")
	assert.Equal(t, "7000000.00", k["accumulated"], "all periods ≤ to (incl. Feb), excl. Jun")
	assert.Equal(t, "93000000.00", k["remaining_book"], "asset's last closing ≤ to (May)")

	// Chart mirrors the monthly expense.
	require.Len(t, res.Chart, 3)
	assert.Equal(t, "2025-03", res.Chart[0].Label)
	assert.Equal(t, "2000000.00", res.Chart[0].Value)

	// The fiscal basis sees its own (single) in-window entry, not the commercial series.
	pf := aScope(e)
	pf.Basis = "fiscal"
	pf.Cur, pf.Prev = cur, prev
	resF, err := svc.Run(context.Background(), "depreciation", pf)
	require.NoError(t, err)
	fr := resF.Rows.([]report.DeprRow)
	require.Len(t, fr, 1)
	assert.Equal(t, "2025-04", fr[0].Period)
	assert.Equal(t, "9999999.00", kpiMap(resF)["period_expense"])

	// Default basis (empty) resolves to commercial.
	pd := aScope(e)
	pd.Cur, pd.Prev = cur, prev
	resD, err := svc.Run(context.Background(), "depreciation", pd)
	require.NoError(t, err)
	assert.Equal(t, "6000000.00", kpiMap(resD)["period_expense"])
}

// ─── utilization ──────────────────────────────────────────────────────────────

// TestReportUtilization drives loan-day accounting over a 31-day window: a
// full-span assignment counts the whole period, an open one clips at date_to, an
// inside one counts its inclusive span, out-of-window and unassigned assets drop
// out, multiple loans on one asset sum, active_loans counts only open loans, and
// office B stays invisible.
func TestReportUtilization(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	// 31-day window.
	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")

	// U1 — spans the whole window (checkout before, checkin after) → 31 days.
	u1 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "U1", "U1")
	ci := "2025-04-10"
	insertAssignment(t, pool, u1, e.emp, e.user, "2025-02-15", &ci, "returned")

	// U2 — open (no checkin), checked out mid-window → clips at date_to (11 days).
	u2 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "U2", "U2")
	insertAssignment(t, pool, u2, e.emp, e.user, "2025-03-21", nil, "active")

	// U3 — fully inside (Mar 5 → Mar 10) → 6 inclusive days.
	u3 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "U3", "U3")
	ci3 := "2025-03-10"
	insertAssignment(t, pool, u3, e.emp, e.user, "2025-03-05", &ci3, "returned")

	// U6 — two in-window loans → 4 + 5 = 9 days, loan_count 2.
	u6 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "U6", "U6")
	c6a, c6b := "2025-03-04", "2025-03-24"
	insertAssignment(t, pool, u6, e.emp, e.user, "2025-03-01", &c6a, "returned")
	insertAssignment(t, pool, u6, e.emp, e.user, "2025-03-20", &c6b, "returned")

	// U4 — no assignments → excluded by HAVING.
	intangibleAsset(t, pool, e, e.cat1, e.officeA, "U4", "U4")

	// U5 — assignment entirely before the window → excluded.
	u5 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "U5", "U5")
	ci5 := "2025-01-31"
	insertAssignment(t, pool, u5, e.emp, e.user, "2025-01-01", &ci5, "returned")

	// UB — office B, open in-window loan → out of an A-only scope.
	ub := intangibleAsset(t, pool, e, e.cat1, e.officeB, "UB", "UB")
	insertAssignment(t, pool, ub, e.emp, e.user, "2025-03-10", nil, "active")

	p := aScope(e)
	p.Cur, p.Prev = cur, prev
	res, err := svc.Run(context.Background(), "utilization", p)
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.UtilRow)
	require.True(t, ok)
	require.Len(t, rows, 4, "U1/U2/U6/U3 only (U4 none, U5 out-of-window, UB office B)")

	// Ordered by days_loaned DESC.
	assert.Equal(t, []string{"U1", "U2", "U6", "U3"},
		[]string{rows[0].Name, rows[1].Name, rows[2].Name, rows[3].Name})
	assert.Equal(t, []int64{31, 11, 9, 6},
		[]int64{rows[0].DaysLoaned, rows[1].DaysLoaned, rows[2].DaysLoaned, rows[3].DaysLoaned})
	assert.Equal(t, []int64{1, 1, 2, 1},
		[]int64{rows[0].LoanCount, rows[1].LoanCount, rows[2].LoanCount, rows[3].LoanCount})

	// Per-row utilization pct = days / 31 × 100 (1 decimal).
	assert.InDelta(t, 100.0, rows[0].UtilizationPct, 0.001)
	assert.InDelta(t, 35.5, rows[1].UtilizationPct, 0.001)
	assert.InDelta(t, 29.0, rows[2].UtilizationPct, 0.001)
	assert.InDelta(t, 19.4, rows[3].UtilizationPct, 0.001)

	k := kpiMap(res)
	assert.Equal(t, "46.0", k["avg_utilization"], "57 total days / (4 × 31)")
	assert.Equal(t, "57", k["total_days"])
	assert.Equal(t, "1", k["active_loans"], "only U2 is active in scope (UB is office B)")

	assert.Equal(t, "57", res.Totals["days_loaned"])
	assert.Equal(t, "5", res.Totals["loan_count"], "1+1+2+1")
	assert.EqualValues(t, 4, res.RowCount)
	assert.False(t, res.Truncated)
}

// TestReportUtilizationEmpty: no assignments anywhere → no rows, avg 0.0.
func TestReportUtilizationEmpty(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)
	intangibleAsset(t, pool, e, e.cat1, e.officeA, "UE", "UE")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev
	res, err := svc.Run(context.Background(), "utilization", p)
	require.NoError(t, err)

	assert.Empty(t, res.Rows.([]report.UtilRow))
	k := kpiMap(res)
	assert.Equal(t, "0.0", k["avg_utilization"])
	assert.Equal(t, "0", k["total_days"])
	assert.Equal(t, "0", k["active_loans"])
}

// ─── maintenance ──────────────────────────────────────────────────────────────

// TestReportMaintenance drives completed-record accounting over a custom window:
// boundary inclusivity, the completed-only filter, the preventive/corrective
// split, grouping by asset+type, the KPIs/Totals/chart, and office scope.
func TestReportMaintenance(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)
	m1 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "M1", "Aset Maint")

	insertMaint(t, pool, m1, "preventive", "completed", "2025-03-01", "1000000.00") // == from
	insertMaint(t, pool, m1, "corrective", "completed", "2025-03-31", "4000000.00") // == to
	insertMaint(t, pool, m1, "corrective", "completed", "2025-02-28", "9000000.00") // before from
	insertMaint(t, pool, m1, "corrective", "completed", "2025-04-01", "8000000.00") // after to
	insertMaint(t, pool, m1, "corrective", "scheduled", "2025-03-15", "7000000.00") // not completed
	// Office B completed record inside the window — out of an A-only scope.
	mb := intangibleAsset(t, pool, e, e.cat1, e.officeB, "MB", "Aset Maint B")
	insertMaint(t, pool, mb, "corrective", "completed", "2025-03-10", "5000000.00")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev
	res, err := svc.Run(context.Background(), "maintenance", p)
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.MaintRow)
	require.True(t, ok)
	require.Len(t, rows, 2, "one preventive + one corrective (boundary/scheduled/office-B excluded)")

	// Ordered by total cost DESC → corrective (4M) then preventive (1M).
	assert.Equal(t, "corrective", rows[0].Type)
	assert.Equal(t, "4000000.00", rows[0].TotalCost)
	assert.EqualValues(t, 1, rows[0].Actions)
	assert.Equal(t, "preventive", rows[1].Type)
	assert.Equal(t, "1000000.00", rows[1].TotalCost)
	assert.Equal(t, e.cat1Name, rows[0].Category)
	assert.Equal(t, "Aset Maint", rows[0].AssetName)

	k := kpiMap(res)
	assert.Equal(t, "5000000.00", k["total_cost"])
	assert.Equal(t, "1000000.00", k["preventive"])
	assert.Equal(t, "4000000.00", k["corrective"])

	assert.Equal(t, "2", res.Totals["actions"])
	assert.Equal(t, "5000000.00", res.Totals["total_cost"])

	require.Len(t, res.Chart, 1)
	assert.Equal(t, e.cat1Name, res.Chart[0].Label)
	assert.Equal(t, "5000000.00", res.Chart[0].Value)

	assert.EqualValues(t, 2, res.RowCount)
	assert.False(t, res.Truncated)
}

// TestReportMaintenanceCategoryFilter narrows completed records by category.
func TestReportMaintenanceCategoryFilter(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)
	m1 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "MF1", "M cat1")
	m2 := intangibleAsset(t, pool, e, e.cat2, e.officeA, "MF2", "M cat2")
	insertMaint(t, pool, m1, "corrective", "completed", "2025-03-10", "3000000.00")
	insertMaint(t, pool, m2, "corrective", "completed", "2025-03-10", "6000000.00")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev
	p.CategoryID = &e.cat2
	res, err := svc.Run(context.Background(), "maintenance", p)
	require.NoError(t, err)

	rows := res.Rows.([]report.MaintRow)
	require.Len(t, rows, 1)
	assert.Equal(t, "M cat2", rows[0].AssetName)
	assert.Equal(t, "6000000.00", kpiMap(res)["total_cost"])
}

// ─── dispatch ────────────────────────────────────────────────────────────────

// TestReportRunInvalidType: an unknown type routes to the invalid-type
// sentinel (all seven real types have builders as of Task 6).
func TestReportRunInvalidType(t *testing.T) {
	svc, _ := newSvc(t)
	for _, typ := range []string{"bogus", "", "assets;drop"} {
		_, err := svc.Run(context.Background(), typ, report.ReportParams{All: true, RowLimit: 1000})
		assert.ErrorIs(t, err, report.ErrInvalidReportType, typ)
	}
}
