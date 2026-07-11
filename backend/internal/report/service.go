package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/google/uuid"
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
