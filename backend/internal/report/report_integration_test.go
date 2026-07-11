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

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/report"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── harness ────────────────────────────────────────────────────────────────

// today is the UTC-midnight reference the whole fixture is built around; the
// service reads real time.Now() for its "today" (overdue / maintenance-due
// window), so seeding relative to the same day keeps the two aligned. cur/prev
// are passed explicitly, so they are fully deterministic.
func refToday() time.Time { return time.Now().UTC().Truncate(24 * time.Hour) }

// newSvc boots a throwaway Postgres and wires a report.Service with a nil Redis
// (cache is Task 4). It also clears the mutable schemas the report reads so a
// migration-seeded row can never leak into an aggregate.
func newSvc(t *testing.T) (*report.Service, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	_, err := pool.Exec(context.Background(),
		`TRUNCATE depreciation.depreciation_entries, assignment.assignments,
		 maintenance.maintenance_records, maintenance.maintenance_schedules,
		 asset.assets CASCADE`)
	require.NoError(t, err)
	return report.NewService(sqlc.New(pool), nil), pool
}

// pgDateStr formats a time as the YYYY-MM-DD literal used in direct inserts.
func d(t time.Time) string { return t.Format("2006-01-02") }

// fixture holds the identifiers a test needs to assert against the seeded data.
type fixture struct {
	officeA, officeB uuid.UUID
	roomA            uuid.UUID
	nameA, nameB     string
	catName          string
	maintCatName     string
	assetA1Name      string
}

// seedFixture builds the deterministic scenario from the brief:
//   - offices A and B (B is outside an A-only scope);
//   - 3 assets in A: A1 (assigned, in room, overdue active assignment),
//     A2 (available, no room), A3 (excluded_from_valuation, under_maintenance);
//   - 1 asset in B (available);
//   - a maintenance schedule in A due in 3 days;
//   - a completed maintenance record in A in the current window (5,000,000) and
//     one in the previous window (2,000,000);
//   - a commercial depreciation entry in A inside the current period.
func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	today := refToday()

	sfx := uuid.New().String()[:8]
	f := fixture{
		nameA:        "Kantor A " + sfx,
		nameB:        "Kantor B " + sfx,
		catName:      "Kategori " + sfx,
		maintCatName: "Servis " + sfx,
		assetA1Name:  "Laptop Direksi " + sfx,
	}

	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		"Tipe "+sfx).Scan(&typeID))

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, f.nameA, "OA"+sfx).Scan(&f.officeA))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, f.nameB, "OB"+sfx).Scan(&f.officeB))

	var floorA uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.floors (office_id, name) VALUES ($1, $2) RETURNING id`,
		f.officeA, "Lantai 1").Scan(&floorA))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.rooms (floor_id, name) VALUES ($1, $2) RETURNING id`,
		floorA, "Ruang 101").Scan(&f.roomA))

	var catID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.categories (name, code) VALUES ($1, $2) RETURNING id`,
		f.catName, "CAT"+sfx).Scan(&catID))

	var maintCatID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.maintenance_categories (name) VALUES ($1) RETURNING id`,
		f.maintCatName).Scan(&maintCatID))

	roleID := lookupRole(t, pool, "Superadmin")
	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"actor "+sfx, "actor."+sfx+"@test.local", roleID, f.officeA).Scan(&userID))
	var empID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.employees (code, name, office_id)
		 VALUES ($1, $2, $3) RETURNING id`,
		"EMP"+sfx, "Pegawai "+sfx, f.officeA).Scan(&empID))

	inPeriod := d(today.AddDate(0, 0, -10)) // inside cur (last30)

	// A1 — tangible, in room, assigned, not excluded.
	a1 := insertAsset(t, pool, assetSeed{
		tag: "A1-" + sfx, name: f.assetA1Name, category: catID, office: f.officeA,
		room: &f.roomA, class: "tangible", status: "assigned",
		cost: "100000000.00", book: "90000000.00", purchaseDate: inPeriod, excluded: false,
	})
	// A2 — intangible (no room), available, not excluded.
	insertAsset(t, pool, assetSeed{
		tag: "A2-" + sfx, name: "Lisensi " + sfx, category: catID, office: f.officeA,
		room: nil, class: "intangible", status: "available",
		cost: "50000000.00", book: "45000000.00", purchaseDate: inPeriod, excluded: false,
	})
	// A3 — excluded_from_valuation, under_maintenance (money excluded, still counted).
	insertAsset(t, pool, assetSeed{
		tag: "A3-" + sfx, name: "Aset Dikecualikan " + sfx, category: catID, office: f.officeA,
		room: nil, class: "intangible", status: "under_maintenance",
		cost: "30000000.00", book: "28000000.00", purchaseDate: inPeriod, excluded: true,
	})
	// B1 — office B (outside an A-only scope), available.
	insertAsset(t, pool, assetSeed{
		tag: "B1-" + sfx, name: "Aset Kantor B " + sfx, category: catID, office: f.officeB,
		room: nil, class: "intangible", status: "available",
		cost: "777.00", book: "500.00", purchaseDate: inPeriod, excluded: false,
	})

	// Overdue active assignment on A1 (due 5 days ago).
	_, err := pool.Exec(ctx,
		`INSERT INTO assignment.assignments (asset_id, employee_id, assigned_by_id, due_date, status)
		 VALUES ($1, $2, $3, $4, 'active')`,
		a1, empID, userID, d(today.AddDate(0, 0, -5)))
	require.NoError(t, err)

	// Maintenance schedule on A1 due in 3 days (inside the 7-day window).
	_, err = pool.Exec(ctx,
		`INSERT INTO maintenance.maintenance_schedules
		   (asset_id, maintenance_category_id, interval_months, next_due_date, is_active)
		 VALUES ($1, $2, 6, $3, true)`,
		a1, maintCatID, d(today.AddDate(0, 0, 3)))
	require.NoError(t, err)

	// Completed maintenance records: one in cur window, one in prev window.
	_, err = pool.Exec(ctx,
		`INSERT INTO maintenance.maintenance_records
		   (asset_id, type, status, completed_date, cost, description)
		 VALUES ($1, 'corrective', 'completed', $2, '5000000.00', 'servis periode ini')`,
		a1, d(today.AddDate(0, 0, -5)))
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO maintenance.maintenance_records
		   (asset_id, type, status, completed_date, cost, description)
		 VALUES ($1, 'corrective', 'completed', $2, '2000000.00', 'servis periode lalu')`,
		a1, d(today.AddDate(0, 0, -45)))
	require.NoError(t, err)

	// Commercial depreciation entry inside the current period (5,000,000).
	_, err = pool.Exec(ctx,
		`INSERT INTO depreciation.depreciation_entries
		   (asset_id, basis, period, opening_value, depreciation_amount, closing_value, method)
		 VALUES ($1, 'commercial', $2, '95000000.00', '5000000.00', '90000000.00', 'straight_line')`,
		a1, inPeriod)
	require.NoError(t, err)

	return f
}

type assetSeed struct {
	tag, name        string
	category, office uuid.UUID
	room             *uuid.UUID
	class, status    string
	cost, book       string
	purchaseDate     string
	excluded         bool
}

func insertAsset(t *testing.T, pool *pgxpool.Pool, s assetSeed) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, room_id, asset_class, capitalized,
		    specifications, status, purchase_date, purchase_cost, book_value, excluded_from_valuation)
		 VALUES ($1, $2, $3, $4, $5, $6, true, '{}', $7, $8, $9, $10, $11)
		 RETURNING id`,
		s.tag, s.name, s.category, s.office, s.room, s.class, s.status,
		s.purchaseDate, s.cost, s.book, s.excluded).Scan(&id))
	return id
}

func lookupRole(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM identity.roles WHERE name = $1 AND deleted_at IS NULL LIMIT 1`,
		name).Scan(&id))
	return id
}

// lastPeriods returns the last30 current + preceding windows for the fixture's
// reference day.
func lastPeriods(t *testing.T) (report.DateRange, report.DateRange) {
	t.Helper()
	cur, prev, err := report.ResolvePeriod("last30", "", "", time.Now().UTC())
	require.NoError(t, err)
	return cur, prev
}

// ─── tests ──────────────────────────────────────────────────────────────────

// TestDashboardSummaryScopeAndExclusion drives the A-only scope: office B is
// invisible, the excluded asset drops out of money sums but stays counted, and
// every count/trend/breakdown matches the seed.
func TestDashboardSummaryScopeAndExclusion(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)
	cur, prev := lastPeriods(t)

	got, err := svc.DashboardSummary(context.Background(), false, []uuid.UUID{f.officeA}, nil, cur, prev)
	require.NoError(t, err)

	// Scope + exclusion.
	assert.EqualValues(t, 3, got.Kpi.TotalAssets, "office B invisible")
	assert.EqualValues(t, 1, got.ExcludedCount)
	assert.Equal(t, "150000000.00", got.Kpi.AcquisitionValue, "A3 money excluded")
	assert.Equal(t, "135000000.00", got.Kpi.BookValue, "A3 money excluded")

	// Operational counts.
	assert.EqualValues(t, 1, got.Kpi.OverdueAssets)
	assert.EqualValues(t, 1, got.Kpi.MaintenanceDue)
	assert.Equal(t, "5000000.00", got.Kpi.MaintenanceCost, "current-window record only")

	// Maintenance-due list.
	require.Len(t, got.MaintenanceDueList, 1)
	item := got.MaintenanceDueList[0]
	assert.Equal(t, f.assetA1Name, item.AssetName)
	assert.NotEmpty(t, item.AssetTag)
	require.NotNil(t, item.CategoryName)
	assert.Equal(t, f.maintCatName, *item.CategoryName)
	assert.Equal(t, refToday().AddDate(0, 0, 3).Format("2006-01-02"), item.NextDueDate)

	// Trends.
	require.NotNil(t, got.Kpi.Trends.MaintenanceCostPct)
	assert.InDelta(t, 150.0, *got.Kpi.Trends.MaintenanceCostPct, 0.001, "(5,000,000-2,000,000)/2,000,000")
	require.NotNil(t, got.Kpi.Trends.BookValuePct)
	assert.InDelta(t, -3.6, *got.Kpi.Trends.BookValuePct, 0.001, "-(5,000,000/140,000,000)*100 -> -3.6")
	assert.Nil(t, got.Kpi.Trends.AcquisitionPct, "all acquisitions fell in-period -> zero base -> nil")

	// Status breakdown: exact 7-key order + seeded counts.
	require.Len(t, got.ByStatus, 7)
	assert.Equal(t,
		[]string{"available", "assigned", "under_maintenance", "in_transfer", "retired", "disposed", "lost"},
		statusOrder(got.ByStatus))
	counts := statusCounts(got.ByStatus)
	assert.EqualValues(t, 1, counts["available"])
	assert.EqualValues(t, 1, counts["assigned"])
	assert.EqualValues(t, 1, counts["under_maintenance"])
	assert.EqualValues(t, 0, counts["in_transfer"])
	assert.EqualValues(t, 0, counts["lost"])

	// Category breakdown (single seeded category, 3 assets in scope).
	require.Len(t, got.ByCategory, 1)
	require.NotNil(t, got.ByCategory[0].Name)
	assert.Equal(t, f.catName, *got.ByCategory[0].Name)
	assert.EqualValues(t, 3, got.ByCategory[0].Count)

	// Location breakdown: single-office scope -> room granularity.
	assert.Equal(t, "room", got.LocationKind)
	roomBuckets := namedTotals(got.ByLocation)
	assert.EqualValues(t, 1, roomBuckets["Ruang 101"], "A1 is in the room")
	assert.EqualValues(t, 2, roomBuckets[""], "A2 and A3 have no room (nil name bucket)")

	assert.Nil(t, got.OfficeName, "no drill-down -> no office name")
}

// TestDashboardSummaryAllScope drives the global scope: all 4 assets are
// visible and the location breakdown is by office.
func TestDashboardSummaryAllScope(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)
	cur, prev := lastPeriods(t)

	got, err := svc.DashboardSummary(context.Background(), true, nil, nil, cur, prev)
	require.NoError(t, err)

	assert.EqualValues(t, 4, got.Kpi.TotalAssets)
	assert.EqualValues(t, 1, got.ExcludedCount)
	assert.Equal(t, "150000777.00", got.Kpi.AcquisitionValue, "A + B, minus the excluded A3")

	assert.Equal(t, "office", got.LocationKind)
	byOffice := namedTotals(got.ByLocation)
	assert.EqualValues(t, 3, byOffice[f.nameA])
	assert.EqualValues(t, 1, byOffice[f.nameB])
	assert.Nil(t, got.OfficeName)
}

// TestDashboardSummaryOfficeFilter drives all-scope + an A drill-down: it must
// behave like the A-scoped call and resolve the office name.
func TestDashboardSummaryOfficeFilter(t *testing.T) {
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)
	cur, prev := lastPeriods(t)

	got, err := svc.DashboardSummary(context.Background(), true, nil, &f.officeA, cur, prev)
	require.NoError(t, err)

	assert.EqualValues(t, 3, got.Kpi.TotalAssets, "drill-down narrows to A")
	assert.Equal(t, "150000000.00", got.Kpi.AcquisitionValue)
	assert.Equal(t, "room", got.LocationKind)
	require.NotNil(t, got.OfficeName)
	assert.Equal(t, f.nameA, *got.OfficeName)
}

// TestDashboardSummaryEmptyDB verifies the all-zero path: no rows anywhere,
// money strings are "0", every trend is nil, and nothing panics on the
// division-by-zero bases.
func TestDashboardSummaryEmptyDB(t *testing.T) {
	svc, _ := newSvc(t) // no seed
	cur, prev := lastPeriods(t)

	got, err := svc.DashboardSummary(context.Background(), true, nil, nil, cur, prev)
	require.NoError(t, err)

	assert.EqualValues(t, 0, got.Kpi.TotalAssets)
	assert.EqualValues(t, 0, got.ExcludedCount)
	assert.Equal(t, "0", got.Kpi.AcquisitionValue)
	assert.Equal(t, "0", got.Kpi.BookValue)
	assert.Equal(t, "0", got.Kpi.MaintenanceCost)
	assert.EqualValues(t, 0, got.Kpi.OverdueAssets)
	assert.EqualValues(t, 0, got.Kpi.MaintenanceDue)

	assert.Nil(t, got.Kpi.Trends.AcquisitionPct)
	assert.Nil(t, got.Kpi.Trends.BookValuePct)
	assert.Nil(t, got.Kpi.Trends.MaintenanceCostPct)

	require.Len(t, got.ByStatus, 7)
	for _, s := range got.ByStatus {
		assert.EqualValues(t, 0, s.Count)
	}
	assert.Empty(t, got.ByCategory)
	assert.Empty(t, got.ByLocation)
	assert.Empty(t, got.MaintenanceDueList)
	assert.Equal(t, "office", got.LocationKind, "all-scope with no single office -> office granularity")
}

// TestCachedDashboardSummary drives the get-or-compute cache wrapper: a first
// call populates the cache, a second identical call is served stale from the
// cache while the direct (uncached) method always reflects the latest data,
// and a differing key argument (officeFilter) bypasses the stale entry. The
// stored key also carries a TTL bounded by the 90s cache window.
func TestCachedDashboardSummary(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	_, err := pool.Exec(ctx,
		`TRUNCATE depreciation.depreciation_entries, assignment.assignments,
		 maintenance.maintenance_records, maintenance.maintenance_schedules,
		 asset.assets CASCADE`)
	require.NoError(t, err)
	rdb := testsupport.NewRedis(t)
	svc := report.NewService(sqlc.New(pool), rdb)

	sfx := uuid.New().String()[:8]
	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		"Tipe Cache "+sfx).Scan(&typeID))
	var officeA uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, "Kantor Cache "+sfx, "OC"+sfx).Scan(&officeA))
	var catID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.categories (name, code) VALUES ($1, $2) RETURNING id`,
		"Kategori Cache "+sfx, "CATC"+sfx).Scan(&catID))

	roleID := lookupRole(t, pool, "Superadmin")
	purchaseDate := d(refToday())
	cur, prev := lastPeriods(t)

	insertAsset(t, pool, assetSeed{
		tag: "C1-" + sfx, name: "Aset Cache 1 " + sfx, category: catID, office: officeA,
		room: nil, class: "intangible", status: "available",
		cost: "1000.00", book: "900.00", purchaseDate: purchaseDate, excluded: false,
	})

	// 1. First call computes fresh and populates the cache: total 1.
	got1, err := svc.CachedDashboardSummary(ctx, roleID, true, nil, nil, cur, prev)
	require.NoError(t, err)
	assert.EqualValues(t, 1, got1.Kpi.TotalAssets)

	// 2. Insert a second asset directly via SQL, bypassing the cache entirely.
	insertAsset(t, pool, assetSeed{
		tag: "C2-" + sfx, name: "Aset Cache 2 " + sfx, category: catID, office: officeA,
		room: nil, class: "intangible", status: "available",
		cost: "1000.00", book: "900.00", purchaseDate: purchaseDate, excluded: false,
	})

	// 3. Identical args -> identical key -> still-stale cached value (1).
	got2, err := svc.CachedDashboardSummary(ctx, roleID, true, nil, nil, cur, prev)
	require.NoError(t, err)
	assert.EqualValues(t, 1, got2.Kpi.TotalAssets, "served from cache, stale")

	// 4. The direct (uncached) method is always the fresh source of truth (2).
	got3, err := svc.DashboardSummary(ctx, true, nil, nil, cur, prev)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got3.Kpi.TotalAssets, "uncached reads reflect the latest data")

	// 5. A different officeFilter arg produces a different cache key -> a
	// fresh (uncached) compute, not the stale entry from step 3.
	got4, err := svc.CachedDashboardSummary(ctx, roleID, true, nil, &officeA, cur, prev)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got4.Kpi.TotalAssets, "distinct key bypasses the stale entry")

	// 6. Every stored dashboard cache key carries a TTL within (0, 90s].
	keys, err := rdb.Keys(ctx, "report:dash:*").Result()
	require.NoError(t, err)
	require.Len(t, keys, 2, "one key per distinct (roleID, all, ids, officeFilter, cur) combination")
	for _, k := range keys {
		ttl, err := rdb.TTL(ctx, k).Result()
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, 90*time.Second)
	}
}

// ─── small assertion helpers ─────────────────────────────────────────────────

func statusOrder(rows []report.StatusCount) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Status
	}
	return out
}

func statusCounts(rows []report.StatusCount) map[string]int64 {
	m := map[string]int64{}
	for _, r := range rows {
		m[r.Status] = r.Count
	}
	return m
}

// namedTotals keys a NamedCount slice by name, mapping the nil ("no room")
// bucket to the empty string.
func namedTotals(rows []report.NamedCount) map[string]int64 {
	m := map[string]int64{}
	for _, r := range rows {
		key := ""
		if r.Name != nil {
			key = *r.Name
		}
		m[key] = r.Count
	}
	return m
}
