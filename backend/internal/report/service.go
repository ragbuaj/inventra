package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

const maintenanceDueWindowDays = 7 // mockup: "dalam 7 hari"

const dashboardCacheTTL = 90 * time.Second

// Service assembles the dashboard and report aggregates. Read-only.
type Service struct {
	q   *sqlc.Queries
	rdb *redis.Client
	now func() time.Time
}

func NewService(q *sqlc.Queries, rdb *redis.Client) *Service {
	return &Service{q: q, rdb: rdb, now: time.Now}
}

// pgDate wraps a midnight-normalized time.Time as a valid pgtype.Date.
func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// pctChange returns (cur-prev)/prev*100 as a display percentage, nil when
// the comparison base is zero/unparseable. Floats are fine here: this is a
// trend indicator, never an accounting figure.
func pctChange(cur, prev string) *float64 {
	c, okc := new(big.Rat).SetString(cur)
	p, okp := new(big.Rat).SetString(prev)
	if !okc || !okp || p.Sign() == 0 {
		return nil
	}
	diff := new(big.Rat).Sub(c, p)
	diff.Quo(diff, p)
	return round1(diff)
}

// ratioPct returns part/whole*100 (nil when whole is zero/unparseable) — used
// for the acquisition trend (additions vs prior base) and depreciation trend.
func ratioPct(part, whole string) *float64 {
	pt, okp := new(big.Rat).SetString(part)
	wh, okw := new(big.Rat).SetString(whole)
	if !okp || !okw || wh.Sign() == 0 {
		return nil
	}
	r := new(big.Rat).Quo(pt, wh)
	return round1(r)
}

// round1 turns a ratio into a percentage rounded to one decimal place (half
// away from zero) and returns a pointer to it.
func round1(ratio *big.Rat) *float64 {
	f, _ := ratio.Float64()
	v := f * 100
	v = float64(int(v*10+copysignHalf(v))) / 10
	return &v
}

func copysignHalf(v float64) float64 {
	if v < 0 {
		return -0.5
	}
	return 0.5
}

// combineDecimal returns a±b as a fixed 2-decimal string (matching the
// numeric(18,2) money columns). Unparseable operands fall back to "0" so the
// downstream ratioPct treats the base as zero and yields a nil trend.
func combineDecimal(a, b string, add bool) string {
	ra, oka := new(big.Rat).SetString(a)
	rb, okb := new(big.Rat).SetString(b)
	if !oka || !okb {
		return "0"
	}
	out := new(big.Rat)
	if add {
		out.Add(ra, rb)
	} else {
		out.Sub(ra, rb)
	}
	return out.FloatString(2)
}

// formatDate renders a pgtype.Date as YYYY-MM-DD (empty string when NULL).
func formatDate(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// DashboardSummary assembles the full dashboard payload for the caller's scope
// (all / office_ids), an optional office drill-down (officeFilter, validated
// against scope by the handler), and the current + preceding trend windows.
func (s *Service) DashboardSummary(ctx context.Context, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur, prev DateRange) (DashboardSummary, error) {
	today := s.now().UTC().Truncate(24 * time.Hour)
	dueEnd := today.AddDate(0, 0, maintenanceDueWindowDays)

	k, err := s.q.DashboardAssetKpis(ctx, sqlc.DashboardAssetKpisParams{
		PeriodFrom: pgDate(cur.From), PeriodTo: pgDate(cur.To),
		AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	overdue, err := s.q.DashboardOverdueCount(ctx, sqlc.DashboardOverdueCountParams{
		Today: pgDate(today), AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	dueCount, err := s.q.DashboardMaintenanceDueCount(ctx, sqlc.DashboardMaintenanceDueCountParams{
		WindowEnd: pgDate(dueEnd), AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	dueRows, err := s.q.DashboardMaintenanceDueList(ctx, sqlc.DashboardMaintenanceDueListParams{
		WindowEnd: pgDate(dueEnd), AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	mcost, err := s.q.DashboardMaintenanceCost(ctx, sqlc.DashboardMaintenanceCostParams{
		CurFrom: pgDate(cur.From), CurTo: pgDate(cur.To),
		PrevFrom: pgDate(prev.From), PrevTo: pgDate(prev.To),
		AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	deprInPeriod, err := s.q.DashboardDepreciationInPeriod(ctx, sqlc.DashboardDepreciationInPeriodParams{
		PeriodFrom: pgDate(cur.From), PeriodTo: pgDate(cur.To),
		AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	catRows, err := s.q.DashboardAssetsByCategory(ctx, sqlc.DashboardAssetsByCategoryParams{
		AllScope: all, OfficeIds: ids, OfficeFilter: officeFilter,
	})
	if err != nil {
		return DashboardSummary{}, err
	}

	// ── Trends ──────────────────────────────────────────────────────────────
	// acquisition: additions in period vs the prior base (total − additions).
	acqBase := combineDecimal(k.AcquisitionValue, k.AcquiredInPeriod, false)
	acquisitionPct := ratioPct(k.AcquiredInPeriod, acqBase)

	// book value: depreciation booked in period vs the pre-depreciation base
	// (current book value + that depreciation), expressed as a decline.
	bvBase := combineDecimal(k.BookValue, deprInPeriod, true)
	bookValuePct := ratioPct(deprInPeriod, bvBase)
	if bookValuePct != nil {
		neg := -*bookValuePct
		bookValuePct = &neg
	}

	// maintenance cost: current window vs the preceding equal-length window.
	maintenancePct := pctChange(mcost.CurrentCost, mcost.PreviousCost)

	// ── Status breakdown (all 7 enum values, fixed order) ───────────────────
	byStatus := []StatusCount{
		{Status: "available", Count: k.StAvailable},
		{Status: "assigned", Count: k.StAssigned},
		{Status: "under_maintenance", Count: k.StUnderMaintenance},
		{Status: "in_transfer", Count: k.StInTransfer},
		{Status: "retired", Count: k.StRetired},
		{Status: "disposed", Count: k.StDisposed},
		{Status: "lost", Count: k.StLost},
	}

	// ── Category breakdown ──────────────────────────────────────────────────
	byCategory := make([]NamedCount, 0, len(catRows))
	for _, row := range catRows {
		name := row.Name
		byCategory = append(byCategory, NamedCount{Name: &name, Count: row.Cnt})
	}

	// ── Location breakdown (room granularity for a single office, else office)
	var locationKind string
	var byLocation []NamedCount
	switch {
	case officeFilter != nil:
		locationKind = "room"
		rooms, err := s.q.DashboardAssetsByRoom(ctx, *officeFilter)
		if err != nil {
			return DashboardSummary{}, err
		}
		byLocation = roomsToNamedCounts(rooms)
	case !all && len(ids) == 1:
		locationKind = "room"
		rooms, err := s.q.DashboardAssetsByRoom(ctx, ids[0])
		if err != nil {
			return DashboardSummary{}, err
		}
		byLocation = roomsToNamedCounts(rooms)
	default:
		locationKind = "office"
		offices, err := s.q.DashboardAssetsByOffice(ctx, sqlc.DashboardAssetsByOfficeParams{
			AllScope: all, OfficeIds: ids,
		})
		if err != nil {
			return DashboardSummary{}, err
		}
		byLocation = make([]NamedCount, 0, len(offices))
		for _, row := range offices {
			name := row.Name
			byLocation = append(byLocation, NamedCount{Name: &name, Count: row.Cnt})
		}
	}

	// ── Maintenance-due list ────────────────────────────────────────────────
	dueList := make([]MaintenanceDueItem, 0, len(dueRows))
	for _, row := range dueRows {
		dueList = append(dueList, MaintenanceDueItem{
			ID:           row.ID.String(),
			AssetName:    row.AssetName,
			AssetTag:     row.AssetTag,
			CategoryName: row.CategoryName,
			NextDueDate:  formatDate(row.NextDueDate),
		})
	}

	// ── Office name (drill-down only) ───────────────────────────────────────
	var officeName *string
	if officeFilter != nil {
		office, err := s.q.GetOffice(ctx, sqlc.GetOfficeParams{ID: *officeFilter, AllScope: true})
		if err != nil {
			return DashboardSummary{}, err
		}
		name := office.Name
		officeName = &name
	}

	return DashboardSummary{
		OfficeName: officeName,
		Kpi: DashboardKpi{
			TotalAssets:      k.TotalAssets,
			AcquisitionValue: k.AcquisitionValue,
			BookValue:        k.BookValue,
			OverdueAssets:    overdue,
			MaintenanceDue:   dueCount,
			MaintenanceCost:  mcost.CurrentCost,
			Trends: Trends{
				AcquisitionPct:     acquisitionPct,
				BookValuePct:       bookValuePct,
				MaintenanceCostPct: maintenancePct,
			},
		},
		ByStatus:           byStatus,
		ByCategory:         byCategory,
		LocationKind:       locationKind,
		ByLocation:         byLocation,
		MaintenanceDueList: dueList,
		ExcludedCount:      k.ExcludedCount,
	}, nil
}

// roomsToNamedCounts maps room aggregate rows (nullable name = "no room"
// bucket) to the generic NamedCount shape.
func roomsToNamedCounts(rows []sqlc.DashboardAssetsByRoomRow) []NamedCount {
	out := make([]NamedCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, NamedCount{Name: row.Name, Count: row.Cnt})
	}
	return out
}

// ── Redis cache (get-or-compute, 90s TTL) ───────────────────────────────────
//
// cacheGetJSON/cacheSetJSON mirror the package-private helpers in
// internal/authz/cache.go (kept local here rather than exported from authz,
// since they're a tiny generic pattern, not a shared authz concern).

// cacheGetJSON loads a JSON value from Redis. Returns false on miss or any
// error (callers then compute fresh — Redis is never the source of truth).
func cacheGetJSON[T any](ctx context.Context, rdb *redis.Client, key string, out *T) bool {
	b, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(b, out) == nil
}

// cacheSetJSON stores a JSON value in Redis with a TTL (best-effort).
func cacheSetJSON(ctx context.Context, rdb *redis.Client, key string, v any, ttl time.Duration) {
	if b, err := json.Marshal(v); err == nil {
		_ = rdb.Set(ctx, key, b, ttl).Err()
	}
}

// dashboardCacheKey derives a stable cache key from every argument that
// affects the dashboard result: the caller's role (permissions/fields can
// vary the shape per role), scope (all vs. a specific office_id set),
// optional office drill-down, and the current period window. prev is
// intentionally excluded — it's fully determined by cur for callers that go
// through ResolvePeriod, and including it would just fragment the cache
// without changing which result is correct.
func dashboardCacheKey(roleID uuid.UUID, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur DateRange) string {
	h := sha256.New()
	sorted := append([]uuid.UUID(nil), ids...)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	for _, id := range sorted {
		h.Write(id[:])
	}
	filter := "-"
	if officeFilter != nil {
		filter = officeFilter.String()
	}
	return fmt.Sprintf("report:dash:%s:%t:%x:%s:%s:%s",
		roleID, all, h.Sum(nil)[:8], filter,
		cur.From.Format("2006-01-02"), cur.To.Format("2006-01-02"))
}

// ── Report builder (assets / depreciation / utilization / maintenance) ───────
//
// Run assembles the GET /reports/:type payload for the four aggregate report
// types. transfers/disposals/opname land in Task 6 (the handler that dispatches
// here doesn't exist yet, so nothing can reach the default arm until then).

// jsonRowLimit caps the rows embedded in a JSON report response; exports pass a
// larger (effectively-unbounded) limit through ReportParams.RowLimit.
const jsonRowLimit = 1000

// ReportParams carries the common + per-type filters, already validated by the
// handler (scope resolved, office_filter checked against scope, dates parsed).
type ReportParams struct {
	All          bool
	OfficeIDs    []uuid.UUID
	OfficeFilter *uuid.UUID
	CategoryID   *uuid.UUID
	Status       *string // assets only, one of shared.asset_status values
	Basis        string  // depreciation only: "commercial"|"fiscal" (default commercial)
	Cur, Prev    DateRange
	RowLimit     int64 // jsonRowLimit for JSON, effectively-unbounded for export
}

// statusEnum converts the optional asset-status filter to the sqlc pointer enum.
func (p ReportParams) statusEnum() *sqlc.SharedAssetStatus {
	if p.Status == nil {
		return nil
	}
	s := sqlc.SharedAssetStatus(*p.Status)
	return &s
}

// basisEnum converts the depreciation basis to the sqlc enum, defaulting to
// commercial when unset.
func (p ReportParams) basisEnum() sqlc.SharedDepreciationBasis {
	b := p.Basis
	if b == "" {
		b = "commercial"
	}
	return sqlc.SharedDepreciationBasis(b)
}

// lim32 narrows a row limit to the int32 the sqlc LIMIT params expect. A
// non-positive or oversized value means "unbounded" (export path).
func lim32(n int64) int32 {
	if n <= 0 || n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

// addDecimal accumulates a decimal money string into acc via big.Rat (never
// float). Unparseable operands are skipped — the DB always emits parseable
// COALESCE(...)::text money, so this only guards against surprises.
func addDecimal(acc *big.Rat, v string) {
	if r, ok := new(big.Rat).SetString(v); ok {
		acc.Add(acc, r)
	}
}

// Per-type report rows. JSON tags mirror the frontend DTO field names and are
// exported so export.go (Task 7) can range over them.

type AssetRow struct {
	Tag          string `json:"asset_tag"`
	Name         string `json:"name"`
	Category     string `json:"category_name"`
	Status       string `json:"status"`
	PurchaseCost string `json:"purchase_cost"`
	AccumDeprec  string `json:"accum_deprec"`
	BookValue    string `json:"book_value"`
}

type DeprRow struct {
	Period  string `json:"period"`
	Opening string `json:"opening"`
	Amount  string `json:"amount"`
	Closing string `json:"closing"`
}

type UtilRow struct {
	Name           string  `json:"name"`
	Tag            string  `json:"asset_tag"`
	Category       string  `json:"category_name"`
	DaysLoaned     int64   `json:"days_loaned"`
	LoanCount      int64   `json:"loan_count"`
	UtilizationPct float64 `json:"utilization_pct"`
}

type MaintRow struct {
	AssetName string `json:"asset_name"`
	Category  string `json:"category_name"`
	Type      string `json:"type"`
	Actions   int64  `json:"actions"`
	TotalCost string `json:"total_cost"`
}

type TransferRow struct {
	AssetName    string `json:"asset_name"`
	AssetTag     string `json:"asset_tag"`
	FromOffice   string `json:"from_office"`
	ToOffice     string `json:"to_office"`
	Status       string `json:"status"`
	ShippedDate  string `json:"shipped_date"`  // YYYY-MM-DD, "" when NULL
	ReceivedDate string `json:"received_date"` // YYYY-MM-DD, "" when NULL
	BastNo       string `json:"bast_no"`       // "" when NULL
}

type DisposalRow struct {
	AssetName string `json:"asset_name"`
	AssetTag  string `json:"asset_tag"`
	Method    string `json:"method"`
	Date      string `json:"disposal_date"`
	BookValue string `json:"book_value"`
	Proceeds  string `json:"proceeds"`
	GainLoss  string `json:"gain_loss"`
}

type OpnameRow struct {
	SessionID  string `json:"session_id"`
	Name       string `json:"name"`
	OfficeName string `json:"office_name"`
	Period     string `json:"period"`
	Status     string `json:"status"`
	TotalItems int64  `json:"total_items"`
	Variance   int64  `json:"variance"`
}

// strOrEmpty returns the pointed-to string, or "" for a nil pointer (nullable
// text column serialized as an empty string in the report row).
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Run dispatches to the per-type builder. The four aggregate types are
// implemented here; transfers/disposals/opname arms land in Task 6.
func (s *Service) Run(ctx context.Context, typ string, p ReportParams) (ReportResult, error) {
	switch typ {
	case "assets":
		return s.runAssets(ctx, p)
	case "depreciation":
		return s.runDepreciation(ctx, p)
	case "utilization":
		return s.runUtilization(ctx, p)
	case "maintenance":
		return s.runMaintenance(ctx, p)
	case "transfers":
		return s.runTransfers(ctx, p)
	case "disposals":
		return s.runDisposals(ctx, p)
	case "opname":
		return s.runOpname(ctx, p)
	default:
		return ReportResult{}, ErrInvalidReportType
	}
}

// runAssets: every in-scope asset (rows list all, including excluded), with the
// money KPIs/Totals/chart excluding excluded_from_valuation.
func (s *Service) runAssets(ctx context.Context, p ReportParams) (ReportResult, error) {
	rows, err := s.q.ReportAssetRows(ctx, sqlc.ReportAssetRowsParams{
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter,
		CategoryID: p.CategoryID, Status: p.statusEnum(), Lim: lim32(p.RowLimit),
	})
	if err != nil {
		return ReportResult{}, err
	}
	totals, err := s.q.ReportAssetTotals(ctx, sqlc.ReportAssetTotalsParams{
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter,
		CategoryID: p.CategoryID, Status: p.statusEnum(),
	})
	if err != nil {
		return ReportResult{}, err
	}
	chart, err := s.q.ReportAssetChart(ctx, sqlc.ReportAssetChartParams{
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter,
		CategoryID: p.CategoryID, Status: p.statusEnum(),
	})
	if err != nil {
		return ReportResult{}, err
	}

	out := make([]AssetRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, AssetRow{
			Tag: r.AssetTag, Name: r.Name, Category: r.CategoryName,
			Status: string(r.Status), PurchaseCost: r.PurchaseCost,
			AccumDeprec: r.AccumDeprec, BookValue: r.BookValue,
		})
	}
	bars := make([]ChartBar, 0, len(chart))
	for _, c := range chart {
		bars = append(bars, ChartBar{Label: c.Name, Value: c.TotalBook})
	}
	return ReportResult{
		Type: "assets",
		Kpis: []ReportKpi{
			{Key: "total_assets", Value: strconv.FormatInt(totals.RowCount, 10)},
			{Key: "total_acquisition", Value: totals.TotalCost},
			{Key: "total_book", Value: totals.TotalBook},
		},
		Chart: bars,
		Rows:  out,
		Totals: map[string]string{
			"purchase_cost": totals.TotalCost,
			"accum_deprec":  totals.TotalAccum,
			"book_value":    totals.TotalBook,
		},
		RowCount:  totals.RowCount,
		Truncated: totals.RowCount > int64(len(out)),
	}, nil
}

// runDepreciation: monthly opening/expense/closing over the period, per basis.
// Totals sum the returned period rows via big.Rat (never float).
func (s *Service) runDepreciation(ctx context.Context, p ReportParams) (ReportResult, error) {
	basis := p.basisEnum()
	rows, err := s.q.ReportDepreciationRows(ctx, sqlc.ReportDepreciationRowsParams{
		Basis: basis, DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}
	kpis, err := s.q.ReportDepreciationKpis(ctx, sqlc.ReportDepreciationKpisParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To), Basis: basis,
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}
	remaining, err := s.q.ReportDepreciationRemaining(ctx, sqlc.ReportDepreciationRemainingParams{
		Basis: basis, DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}

	out := make([]DeprRow, 0, len(rows))
	openAcc, amtAcc, closeAcc := new(big.Rat), new(big.Rat), new(big.Rat)
	bars := make([]ChartBar, 0, len(rows))
	for _, r := range rows {
		out = append(out, DeprRow{Period: r.Period, Opening: r.Opening, Amount: r.Amount, Closing: r.Closing})
		addDecimal(openAcc, r.Opening)
		addDecimal(amtAcc, r.Amount)
		addDecimal(closeAcc, r.Closing)
		bars = append(bars, ChartBar{Label: r.Period, Value: r.Amount})
	}
	return ReportResult{
		Type: "depreciation",
		Kpis: []ReportKpi{
			{Key: "period_expense", Value: kpis.PeriodExpense},
			{Key: "accumulated", Value: kpis.Accumulated},
			{Key: "remaining_book", Value: remaining},
		},
		Chart: bars,
		Rows:  out,
		Totals: map[string]string{
			"opening": openAcc.FloatString(2),
			"amount":  amtAcc.FloatString(2),
			"closing": closeAcc.FloatString(2),
		},
		RowCount:  int64(len(out)),
		Truncated: false,
	}, nil
}

// runUtilization: loan-days per asset over the period (clipped to the window),
// with avg utilization = total days / (rows × period days), computed in Go.
func (s *Service) runUtilization(ctx context.Context, p ReportParams) (ReportResult, error) {
	rows, err := s.q.ReportUtilizationRows(ctx, sqlc.ReportUtilizationRowsParams{
		DateTo: pgDate(p.Cur.To), DateFrom: pgDate(p.Cur.From),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
		Lim: lim32(p.RowLimit),
	})
	if err != nil {
		return ReportResult{}, err
	}
	active, err := s.q.ReportUtilizationKpis(ctx, sqlc.ReportUtilizationKpisParams{
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}

	curDays := int64(p.Cur.Days())
	out := make([]UtilRow, 0, len(rows))
	bars := make([]ChartBar, 0, len(rows))
	var totalDays, totalLoans int64
	for _, r := range rows {
		pct := 0.0
		if curDays > 0 {
			pct = *round1(big.NewRat(r.DaysLoaned, curDays))
		}
		out = append(out, UtilRow{
			Name: r.Name, Tag: r.AssetTag, Category: r.CategoryName,
			DaysLoaned: r.DaysLoaned, LoanCount: r.LoanCount, UtilizationPct: pct,
		})
		totalDays += r.DaysLoaned
		totalLoans += r.LoanCount
		if len(bars) < 8 {
			bars = append(bars, ChartBar{Label: r.Name, Value: strconv.FormatInt(r.DaysLoaned, 10)})
		}
	}
	avg := 0.0
	if n := int64(len(out)); n > 0 && curDays > 0 {
		avg = *round1(big.NewRat(totalDays, n*curDays))
	}
	return ReportResult{
		Type: "utilization",
		Kpis: []ReportKpi{
			{Key: "avg_utilization", Value: strconv.FormatFloat(avg, 'f', 1, 64)},
			{Key: "active_loans", Value: strconv.FormatInt(active, 10)},
			{Key: "total_days", Value: strconv.FormatInt(totalDays, 10)},
		},
		Chart: bars,
		Rows:  out,
		Totals: map[string]string{
			"days_loaned": strconv.FormatInt(totalDays, 10),
			"loan_count":  strconv.FormatInt(totalLoans, 10),
		},
		RowCount:  int64(len(out)),
		Truncated: false,
	}, nil
}

// runMaintenance: completed records grouped by asset+type over the period.
// Totals sum actions and cost (big.Rat) over the returned rows.
func (s *Service) runMaintenance(ctx context.Context, p ReportParams) (ReportResult, error) {
	rows, err := s.q.ReportMaintenanceRows(ctx, sqlc.ReportMaintenanceRowsParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
		Lim: lim32(p.RowLimit),
	})
	if err != nil {
		return ReportResult{}, err
	}
	kpis, err := s.q.ReportMaintenanceKpis(ctx, sqlc.ReportMaintenanceKpisParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}
	chart, err := s.q.ReportMaintenanceChart(ctx, sqlc.ReportMaintenanceChartParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}

	out := make([]MaintRow, 0, len(rows))
	costAcc := new(big.Rat)
	var totalActions int64
	for _, r := range rows {
		out = append(out, MaintRow{
			AssetName: r.AssetName, Category: r.CategoryName,
			Type: string(r.Type), Actions: r.Actions, TotalCost: r.TotalCost,
		})
		totalActions += r.Actions
		addDecimal(costAcc, r.TotalCost)
	}
	bars := make([]ChartBar, 0, len(chart))
	for _, c := range chart {
		bars = append(bars, ChartBar{Label: c.Name, Value: c.Total})
	}
	return ReportResult{
		Type: "maintenance",
		Kpis: []ReportKpi{
			{Key: "total_cost", Value: kpis.Total},
			{Key: "preventive", Value: kpis.Preventive},
			{Key: "corrective", Value: kpis.Corrective},
		},
		Chart: bars,
		Rows:  out,
		Totals: map[string]string{
			"actions":    strconv.FormatInt(totalActions, 10),
			"total_cost": costAcc.FloatString(2),
		},
		RowCount:  int64(len(out)),
		Truncated: false,
	}, nil
}

// runTransfers: inter-office mutasi in the period, visible when either the
// source or destination office is in scope. No money tfoot — Totals is empty.
func (s *Service) runTransfers(ctx context.Context, p ReportParams) (ReportResult, error) {
	rows, err := s.q.ReportTransferRows(ctx, sqlc.ReportTransferRowsParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
		Lim: lim32(p.RowLimit),
	})
	if err != nil {
		return ReportResult{}, err
	}
	kpis, err := s.q.ReportTransferKpis(ctx, sqlc.ReportTransferKpisParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}
	chart, err := s.q.ReportTransferChart(ctx, sqlc.ReportTransferChartParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}

	out := make([]TransferRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, TransferRow{
			AssetName: r.AssetName, AssetTag: r.AssetTag,
			FromOffice: r.FromOffice, ToOffice: r.ToOffice, Status: string(r.Status),
			ShippedDate: formatDate(r.ShippedDate), ReceivedDate: formatDate(r.ReceivedDate),
			BastNo: strOrEmpty(r.BastNo),
		})
	}
	bars := make([]ChartBar, 0, len(chart))
	for _, c := range chart {
		bars = append(bars, ChartBar{Label: c.Name, Value: strconv.FormatInt(c.Cnt, 10)})
	}
	return ReportResult{
		Type: "transfers",
		Kpis: []ReportKpi{
			{Key: "total", Value: strconv.FormatInt(kpis.Total, 10)},
			{Key: "in_transit", Value: strconv.FormatInt(kpis.InTransit, 10)},
			{Key: "received", Value: strconv.FormatInt(kpis.Received, 10)},
		},
		Chart:     bars,
		Rows:      out,
		Totals:    map[string]string{},
		RowCount:  int64(len(out)),
		Truncated: false,
	}, nil
}

// runDisposals: asset disposals in the period, with the gain/loss KPIs, the
// per-method net gain/loss chart (raw enum labels), and the money tfoot summed
// over the returned rows via big.Rat.
func (s *Service) runDisposals(ctx context.Context, p ReportParams) (ReportResult, error) {
	rows, err := s.q.ReportDisposalRows(ctx, sqlc.ReportDisposalRowsParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
		Lim: lim32(p.RowLimit),
	})
	if err != nil {
		return ReportResult{}, err
	}
	kpis, err := s.q.ReportDisposalKpis(ctx, sqlc.ReportDisposalKpisParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}
	chart, err := s.q.ReportDisposalChart(ctx, sqlc.ReportDisposalChartParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return ReportResult{}, err
	}

	out := make([]DisposalRow, 0, len(rows))
	bookAcc, proceedsAcc, glAcc := new(big.Rat), new(big.Rat), new(big.Rat)
	for _, r := range rows {
		out = append(out, DisposalRow{
			AssetName: r.AssetName, AssetTag: r.AssetTag, Method: string(r.Method),
			Date: formatDate(r.DisposalDate), BookValue: r.BookValue,
			Proceeds: r.Proceeds, GainLoss: r.GainLoss,
		})
		addDecimal(bookAcc, r.BookValue)
		addDecimal(proceedsAcc, r.Proceeds)
		addDecimal(glAcc, r.GainLoss)
	}
	bars := make([]ChartBar, 0, len(chart))
	for _, c := range chart {
		bars = append(bars, ChartBar{Label: string(c.Method), Value: c.Total})
	}
	return ReportResult{
		Type: "disposals",
		Kpis: []ReportKpi{
			{Key: "total_disposals", Value: strconv.FormatInt(kpis.Total, 10)},
			{Key: "total_proceeds", Value: kpis.TotalProceeds},
			{Key: "total_gain_loss", Value: kpis.TotalGainLoss},
		},
		Chart: bars,
		Rows:  out,
		Totals: map[string]string{
			"book_value": bookAcc.FloatString(2),
			"proceeds":   proceedsAcc.FloatString(2),
			"gain_loss":  glAcc.FloatString(2),
		},
		RowCount:  int64(len(out)),
		Truncated: false,
	}, nil
}

// runOpname: closed stock-opname sessions in the period with their item counts
// and variance (not_found/damaged/misplaced). KPIs sum over the (few) returned
// sessions in Go; chart is variance per session (top 8). No money tfoot.
func (s *Service) runOpname(ctx context.Context, p ReportParams) (ReportResult, error) {
	rows, err := s.q.ReportOpnameSessions(ctx, sqlc.ReportOpnameSessionsParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter,
		Lim: lim32(p.RowLimit),
	})
	if err != nil {
		return ReportResult{}, err
	}

	out := make([]OpnameRow, 0, len(rows))
	bars := make([]ChartBar, 0, len(rows))
	var totalItems, totalVariance int64
	for _, r := range rows {
		label := r.Name
		if label == "" {
			label = formatDate(r.Period)
		}
		out = append(out, OpnameRow{
			SessionID: r.ID.String(), Name: r.Name, OfficeName: r.OfficeName,
			Period: formatDate(r.Period), Status: string(r.Status),
			TotalItems: r.TotalItems, Variance: r.Variance,
		})
		totalItems += r.TotalItems
		totalVariance += r.Variance
		if len(bars) < 8 {
			bars = append(bars, ChartBar{Label: label, Value: strconv.FormatInt(r.Variance, 10)})
		}
	}
	return ReportResult{
		Type: "opname",
		Kpis: []ReportKpi{
			{Key: "sessions", Value: strconv.FormatInt(int64(len(out)), 10)},
			{Key: "total_items", Value: strconv.FormatInt(totalItems, 10)},
			{Key: "total_variance", Value: strconv.FormatInt(totalVariance, 10)},
		},
		Chart:     bars,
		Rows:      out,
		Totals:    map[string]string{},
		RowCount:  int64(len(out)),
		Truncated: false,
	}, nil
}

// DisposalGlRecap builds the journal-ready recap for disposals in the period:
//
//	Dr Kas/Bank                  = Σ proceeds
//	Dr Rugi Pelepasan Aset       = Σ |gain_loss| where gain_loss < 0
//	Cr Nilai Buku Aset Dilepas   = Σ book_value_at_disposal
//	Cr Laba Pelepasan Aset       = Σ gain_loss where gain_loss > 0
//
// The journal balances by construction (gain_loss = proceeds − book_value).
// Account codes come from app_settings keys report.gl.{cash,loss,asset,gain}_account
// (empty string when unset — configurable mapping is a recorded follow-up).
// Rows with a zero amount are omitted; Balanced is a big.Rat comparison of totals.
func (s *Service) DisposalGlRecap(ctx context.Context, p ReportParams) (GlRecapResult, error) {
	k, err := s.q.ReportDisposalKpis(ctx, sqlc.ReportDisposalKpisParams{
		DateFrom: pgDate(p.Cur.From), DateTo: pgDate(p.Cur.To),
		AllScope: p.All, OfficeIds: p.OfficeIDs, OfficeFilter: p.OfficeFilter, CategoryID: p.CategoryID,
	})
	if err != nil {
		return GlRecapResult{}, err
	}

	cash, err := s.glAccount(ctx, "report.gl.cash_account")
	if err != nil {
		return GlRecapResult{}, err
	}
	loss, err := s.glAccount(ctx, "report.gl.loss_account")
	if err != nil {
		return GlRecapResult{}, err
	}
	assetAcc, err := s.glAccount(ctx, "report.gl.asset_account")
	if err != nil {
		return GlRecapResult{}, err
	}
	gain, err := s.glAccount(ctx, "report.gl.gain_account")
	if err != nil {
		return GlRecapResult{}, err
	}

	lines := []struct {
		code, name, amount string
		debit              bool
	}{
		{cash, "Kas/Bank", k.TotalProceeds, true},
		{loss, "Rugi Pelepasan Aset", k.TotalLoss, true},
		{assetAcc, "Nilai Buku Aset Dilepas", k.TotalBookValue, false},
		{gain, "Laba Pelepasan Aset", k.TotalGain, false},
	}
	rows := make([]GlRow, 0, len(lines))
	debit, credit := new(big.Rat), new(big.Rat)
	for _, l := range lines {
		r, ok := new(big.Rat).SetString(l.amount)
		if !ok || r.Sign() == 0 {
			continue // zero-amount rows are omitted
		}
		amt := r.FloatString(2)
		if l.debit {
			debit.Add(debit, r)
			rows = append(rows, GlRow{AccountCode: l.code, AccountName: l.name, Debit: amt, Credit: "0.00"})
		} else {
			credit.Add(credit, r)
			rows = append(rows, GlRow{AccountCode: l.code, AccountName: l.name, Debit: "0.00", Credit: amt})
		}
	}
	return GlRecapResult{
		Rows:        rows,
		TotalDebit:  debit.FloatString(2),
		TotalCredit: credit.FloatString(2),
		Balanced:    debit.Cmp(credit) == 0,
	}, nil
}

// glAccount reads a GL account-code app setting, tolerating an unset key
// (returns "" like depreciation's BuildJournalPDF does for its label setting).
func (s *Service) glAccount(ctx context.Context, key string) (string, error) {
	v, err := s.q.GetAppSetting(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

// CachedDashboardSummary is a get-or-compute wrapper around DashboardSummary:
// it serves a cached result (TTL 90s) when present, else computes fresh and
// populates the cache. With a nil Redis client (rdb == nil, e.g. in tests
// that don't need caching) it always computes fresh. DashboardSummary itself
// stays exported and always fresh — callers that need guaranteed up-to-date
// data (e.g. right after a mutation) should call it directly.
func (s *Service) CachedDashboardSummary(ctx context.Context, roleID uuid.UUID, all bool, ids []uuid.UUID, officeFilter *uuid.UUID, cur, prev DateRange) (DashboardSummary, error) {
	key := dashboardCacheKey(roleID, all, ids, officeFilter, cur)
	var cached DashboardSummary
	if s.rdb != nil && cacheGetJSON(ctx, s.rdb, key, &cached) {
		return cached, nil
	}
	out, err := s.DashboardSummary(ctx, all, ids, officeFilter, cur, prev)
	if err != nil {
		return DashboardSummary{}, err
	}
	if s.rdb != nil {
		cacheSetJSON(ctx, s.rdb, key, out, dashboardCacheTTL)
	}
	return out, nil
}
