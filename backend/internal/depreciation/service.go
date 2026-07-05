// Package depreciation — service.go orchestrates the monthly compute/close
// period lifecycle on top of the pure calculation core (engine.go). No Gin
// here (ADR-0008): plain domain types and sentinel errors only; the handler
// (a later task) translates these to HTTP status codes and enforces
// permission/scope.
package depreciation

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

var (
	// ErrPeriodClosed is returned by ComputePeriod when the target period is
	// already closed (immutable), and by ClosePeriod when it is closed already.
	ErrPeriodClosed = errors.New("depreciation: period is closed")
	// ErrPeriodNotComputed is returned by ClosePeriod when the target period
	// has no row at all, or is still `open` (never computed).
	ErrPeriodNotComputed = errors.New("depreciation: period is not computed")
	// ErrPriorPeriodOpen is returned by ClosePeriod when an earlier period
	// has a row that is not yet `closed` (sequential close guard).
	ErrPriorPeriodOpen = errors.New("depreciation: an earlier period has not been closed")
	// ErrPeriodBeforeWatermark is returned by ComputePeriod when the target
	// period is at or before the closed watermark: those months are immutable
	// closed history, so a skipped month that was never computed before later
	// months closed cannot be computed retroactively (it would only produce a
	// hollow 'computed' row — its regeneration window is empty by definition).
	ErrPeriodBeforeWatermark = errors.New("depreciation: period precedes the closed watermark; entries for it live in closed history")
	// ErrNotFound is returned when a referenced asset does not exist.
	ErrNotFound = errors.New("depreciation: not found")
	// ErrNoBookValue is returned by RecordImpairment when the asset has no
	// computed book value yet (ComputePeriod has never run for it) — an
	// impairment test needs a current carrying amount to compare against.
	ErrNoBookValue = errors.New("depreciation: asset has no book value; run depreciation first")
	// ErrInvalidRecoverable is returned by RecordImpairment when the
	// recoverable amount is malformed, negative, or not strictly below the
	// asset's current book value (an impairment must reduce the carrying
	// amount — otherwise there is nothing to write down).
	ErrInvalidRecoverable = errors.New("depreciation: recoverable amount must be non-negative and less than the current book value")
)

// Service orchestrates period compute/close and the read-side helpers
// (Periods, BookValueAsOf) consumed by later tasks (schedule/journal,
// disposal book-value integration, the depreciation handler).
type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewService builds a Service from the shared sqlc queries + pool (the pool is
// needed directly for transaction control around ComputePeriod/ClosePeriod).
func NewService(q *sqlc.Queries, pool *pgxpool.Pool) *Service {
	return &Service{q: q, pool: pool}
}

// RunSummary is the outcome of one ComputePeriod call.
type RunSummary struct {
	AssetCount   int
	TotalAmount  string
	SkippedCount int
	Skipped      []SkippedAsset
}

// SkippedAsset explains why one asset produced no commercial entries this run.
// See Skip (engine.go) for the set of reasons.
type SkippedAsset struct {
	AssetID uuid.UUID
	Reason  string
}

// PeriodInfo is one row of Periods() — either a persisted period row or the
// synthetic "current month, never computed" row.
type PeriodInfo struct {
	Period       time.Time
	Status       string
	AssetCount   int
	TotalAmount  string
	SkippedCount int
}

// ComputePeriod is idempotent and advisory-locked: it regenerates every
// non-closed depreciation entry (period strictly after the closed watermark,
// up to and including target) for every eligible asset, across BOTH bases,
// refreshes each processed asset's commercial accumulated_depreciation/
// book_value, and upserts the period row to `computed` with a run summary.
// Safe to call repeatedly while the period is not yet closed — each call
// fully replaces the regenerated window, so re-running with unchanged inputs
// produces byte-identical entries.
//
// Disposed assets are fully skipped from regeneration: ResolveCommercial/
// ResolveFiscal both return Skip{"disposed"} for them, so DeleteEntries* still
// removes their non-closed entries but nothing is inserted back. This is
// acceptable because disposal normally happens after the periods covering an
// asset's active life have already been closed (and closed periods are never
// touched) — a disposed asset's history therefore survives only in closed
// periods. Recording a disposal mid-open-period would lose that period's
// entries for the asset on the next compute; this is a known, documented
// limitation rather than a bug (see docs/superpowers/specs/2026-07-05
// -depreciation-module-design.md).
func (s *Service) ComputePeriod(ctx context.Context, period time.Time, actor uuid.UUID) (RunSummary, error) {
	target := firstOfMonth(period)
	targetDate := pgtype.Date{Time: target, Valid: true}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RunSummary{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	if err := qtx.AdvisoryLockDepreciation(ctx); err != nil {
		return RunSummary{}, err
	}

	existing, err := qtx.GetDepreciationPeriod(ctx, targetDate)
	switch {
	case err == nil:
		if existing.Status == sqlc.SharedDepreciationPeriodStatusClosed {
			return RunSummary{}, ErrPeriodClosed
		}
	case errors.Is(err, pgx.ErrNoRows):
		// No row yet for this period — first-ever compute of it.
	default:
		return RunSummary{}, err
	}

	// watermark: the latest CLOSED period, if any. Entries at or before it are
	// immutable and are never deleted/regenerated; entries strictly after it
	// (through target) are the regeneration window.
	var watermark *pgtype.Date
	lastClosed, err := qtx.LastClosedPeriod(ctx)
	switch {
	case err == nil:
		wm := pgtype.Date{Time: lastClosed.Time, Valid: true}
		watermark = &wm
	case errors.Is(err, pgx.ErrNoRows):
		// No closed period yet — regenerate everything through target.
	default:
		return RunSummary{}, err
	}

	// A target at/before the watermark is immutable closed history: nothing in
	// its regeneration window can be (re)generated, so computing it would only
	// mint a hollow 'computed' row. (target == watermark normally hits the
	// ErrPeriodClosed guard above first; this also covers the skipped-month
	// case, where the target has no row at all.)
	if watermark != nil && !target.After(watermark.Time) {
		return RunSummary{}, ErrPeriodBeforeWatermark
	}

	if watermark != nil {
		if err := qtx.DeleteEntriesAfterWatermark(ctx, sqlc.DeleteEntriesAfterWatermarkParams{
			Watermark: *watermark, Target: targetDate,
		}); err != nil {
			return RunSummary{}, err
		}
	} else {
		if err := qtx.DeleteEntriesThrough(ctx, targetDate); err != nil {
			return RunSummary{}, err
		}
	}

	rows, err := qtx.ListAssetsForDepreciation(ctx)
	if err != nil {
		return RunSummary{}, err
	}

	summary := RunSummary{}
	targetTotal := new(big.Rat)

	for _, row := range rows {
		a := row.AssetAsset
		c := row.MasterdataCategory

		commercialParams, commercialSkip := ResolveCommercial(a, c)
		if commercialSkip != nil {
			// A commercial skip means the asset does not depreciate under
			// EITHER basis this run — the shared preconditions (capitalized,
			// purchase_cost, purchase_date, disposed) gate ResolveCommercial
			// and ResolveFiscal identically, so fiscal would skip for the
			// same reason too. Report the asset once (not once per basis).
			summary.Skipped = append(summary.Skipped, SkippedAsset{AssetID: a.ID, Reason: commercialSkip.Reason})
		} else {
			commercialEntries, err := s.regenerateBasis(ctx, qtx, a, *commercialParams, sqlc.SharedDepreciationBasisCommercial, watermark, target)
			if err != nil {
				return RunSummary{}, err
			}

			hasTargetAmount := false
			for _, e := range commercialEntries {
				if e.Period.Equal(target) {
					amt, err := parseMoney(e.Amount)
					if err != nil {
						return RunSummary{}, err
					}
					targetTotal.Add(targetTotal, amt)
					if amt.Sign() > 0 {
						hasTargetAmount = true
					}
				}
			}
			if hasTargetAmount {
				summary.AssetCount++
			}

			if fiscalParams, fiscalSkip := ResolveFiscal(a, c); fiscalSkip == nil {
				if _, err := s.regenerateBasis(ctx, qtx, a, *fiscalParams, sqlc.SharedDepreciationBasisFiscal, watermark, target); err != nil {
					return RunSummary{}, err
				}
			}
		}

		if err := s.refreshAssetSummary(ctx, qtx, a, targetDate); err != nil {
			return RunSummary{}, err
		}
	}

	summary.SkippedCount = len(summary.Skipped)
	summary.TotalAmount = roundHalfUp2(targetTotal)

	if _, err := qtx.UpsertPeriodComputed(ctx, sqlc.UpsertPeriodComputedParams{
		Period:       targetDate,
		ComputedBy:   &actor,
		AssetCount:   int32(summary.AssetCount),
		TotalAmount:  summary.TotalAmount,
		SkippedCount: int32(summary.SkippedCount),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The upsert's own status guard refused to touch a closed period
			// (defense-in-depth against a close committing between our status
			// pre-check and here). Returning the sentinel triggers the
			// deferred rollback, discarding every entry regenerated above —
			// the closed period's entries must not be replaced.
			return RunSummary{}, ErrPeriodClosed
		}
		return RunSummary{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RunSummary{}, err
	}
	return summary, nil
}

// regenerateBasis (re)generates entries for one (asset, basis) pair from the
// watermark (exclusive; nil = from the asset's own start) through target
// (inclusive) and inserts them. Returns the generated entries so the caller
// can fold target-month amounts into the run summary.
func (s *Service) regenerateBasis(ctx context.Context, qtx *sqlc.Queries, a sqlc.AssetAsset, params Params, basis sqlc.SharedDepreciationBasis, watermark *pgtype.Date, target time.Time) ([]Entry, error) {
	var lastPeriod *time.Time
	var lastClosing *string

	if watermark != nil {
		last, err := qtx.LastEntryAtOrBefore(ctx, sqlc.LastEntryAtOrBeforeParams{
			AssetID: a.ID, Basis: basis, Period: *watermark,
		})
		switch {
		case err == nil:
			lp := last.Period.Time
			lastPeriod = &lp
			closing := last.ClosingValue
			// Commercial-only impairment override: the (Task 5) impairment
			// endpoint writes asset.book_value down directly without adding a
			// depreciation entry, since impairment is a separate loss, not a
			// depreciation expense. If that has left book_value BELOW the
			// last entry's closing, depreciation must resume from the lower
			// value — otherwise the impairment would be silently undone on
			// the next compute. Fiscal never recognizes impairment.
			if basis == sqlc.SharedDepreciationBasisCommercial && a.BookValue != nil {
				bv, errBV := parseMoney(*a.BookValue)
				cv, errCV := parseMoney(closing)
				if errBV == nil && errCV == nil && bv.Cmp(cv) < 0 {
					closing = *a.BookValue
				}
			}
			lastClosing = &closing
		case errors.Is(err, pgx.ErrNoRows):
			// No entry at/before the watermark for this asset+basis (e.g. the
			// asset's own depreciation started after the watermark) — Walk
			// starts fresh from params.Start.
		default:
			return nil, err
		}
	}

	entries, err := Walk(params, lastPeriod, lastClosing, target)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if err := qtx.InsertDepreciationEntry(ctx, sqlc.InsertDepreciationEntryParams{
			AssetID:            a.ID,
			Basis:              basis,
			Period:             pgtype.Date{Time: e.Period, Valid: true},
			OpeningValue:       e.Opening,
			DepreciationAmount: e.Amount,
			ClosingValue:       e.Closing,
			Method:             e.Method,
		}); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// refreshAssetSummary recomputes one asset's commercial accumulated_depreciation
// (sum of ALL commercial amounts ever posted — impairment is never folded in
// here, since it is not a depreciation expense) and book_value (closing of the
// last commercial entry at/before target; falling back to cost minus
// impairment_loss when the asset has no commercial entries at all).
func (s *Service) refreshAssetSummary(ctx context.Context, qtx *sqlc.Queries, a sqlc.AssetAsset, targetDate pgtype.Date) error {
	accumulated, err := qtx.SumAssetAmounts(ctx, sqlc.SumAssetAmountsParams{AssetID: a.ID, Basis: sqlc.SharedDepreciationBasisCommercial})
	if err != nil {
		return err
	}

	var bookValue *string
	last, err := qtx.LastEntryAtOrBefore(ctx, sqlc.LastEntryAtOrBeforeParams{
		AssetID: a.ID, Basis: sqlc.SharedDepreciationBasisCommercial, Period: targetDate,
	})
	switch {
	case err == nil:
		bv := last.ClosingValue
		bookValue = &bv
	case errors.Is(err, pgx.ErrNoRows):
		if a.PurchaseCost != nil {
			cost, errCost := parseMoney(*a.PurchaseCost)
			if errCost == nil {
				impairment := new(big.Rat)
				if a.ImpairmentLoss != nil {
					if imp, errImp := parseMoney(*a.ImpairmentLoss); errImp == nil {
						impairment = imp
					}
				}
				bv := roundHalfUp2(new(big.Rat).Sub(cost, impairment))
				bookValue = &bv
			}
		}
	default:
		return err
	}

	return qtx.UpdateAssetDepreciationSummary(ctx, sqlc.UpdateAssetDepreciationSummaryParams{
		Accumulated: accumulated,
		BookValue:   bookValue,
		ID:          a.ID,
	})
}

// ClosePeriod finalizes a `computed` period as `closed`, making its entries
// immutable (never touched by a later ComputePeriod). Only allowed from
// `computed`; sequential — every earlier period that HAS a row must already
// be `closed` (a period that was never computed at all, i.e. has no row, does
// not block a later close).
func (s *Service) ClosePeriod(ctx context.Context, period time.Time, actor uuid.UUID) error {
	target := firstOfMonth(period)
	targetDate := pgtype.Date{Time: target, Valid: true}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	// Serialize against ComputePeriod (and other closes): closing must never
	// interleave with an in-flight recompute, or the compute's UpsertPeriod-
	// Computed could land after our commit and silently reopen the period.
	// Same transaction-scoped lock key as ComputePeriod.
	if err := qtx.AdvisoryLockDepreciation(ctx); err != nil {
		return err
	}

	row, err := qtx.GetDepreciationPeriod(ctx, targetDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPeriodNotComputed
	}
	if err != nil {
		return err
	}
	switch row.Status {
	case sqlc.SharedDepreciationPeriodStatusClosed:
		return ErrPeriodClosed
	case sqlc.SharedDepreciationPeriodStatusOpen:
		return ErrPeriodNotComputed
	}

	openEarlier, err := qtx.CountOpenEarlierPeriods(ctx, targetDate)
	if err != nil {
		return err
	}
	if openEarlier > 0 {
		return ErrPriorPeriodOpen
	}

	if _, err := qtx.SetPeriodClosed(ctx, sqlc.SetPeriodClosedParams{Period: targetDate, ClosedBy: &actor}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The guarded UPDATE (status = 'computed' only) matched nothing:
			// the period was closed out from under us (close/close TOCTOU).
			// Surface the domain sentinel, never the raw driver error.
			return ErrPeriodClosed
		}
		return err
	}
	return tx.Commit(ctx)
}

// Periods lists every persisted period (newest first) plus a synthetic
// current-calendar-month row (status "open") when no row exists for it yet —
// the frontend's "period not yet computed" reminder banner reads this.
func (s *Service) Periods(ctx context.Context) ([]PeriodInfo, error) {
	rows, err := s.q.ListDepreciationPeriods(ctx)
	if err != nil {
		return nil, err
	}

	now := firstOfMonth(time.Now())
	found := false
	infos := make([]PeriodInfo, 0, len(rows)+1)
	for _, r := range rows {
		if r.Period.Time.Equal(now) {
			found = true
		}
		infos = append(infos, PeriodInfo{
			Period:       r.Period.Time,
			Status:       string(r.Status),
			AssetCount:   int(r.AssetCount),
			TotalAmount:  r.TotalAmount,
			SkippedCount: int(r.SkippedCount),
		})
	}
	if !found {
		virtual := PeriodInfo{Period: now, Status: string(sqlc.SharedDepreciationPeriodStatusOpen)}
		infos = append([]PeriodInfo{virtual}, infos...)
	}
	return infos, nil
}

// BookValueAsOf returns the commercial book value of an asset as of a given
// month: the closing value of its last commercial entry with period <= asOf's
// month; if the asset has no commercial entries at all, falls back to its raw
// purchase_cost; if that is also absent, "0".
func (s *Service) BookValueAsOf(ctx context.Context, assetID uuid.UUID, asOf time.Time) (string, error) {
	target := firstOfMonth(asOf)
	entry, err := s.q.LastEntryAtOrBefore(ctx, sqlc.LastEntryAtOrBeforeParams{
		AssetID: assetID, Basis: sqlc.SharedDepreciationBasisCommercial, Period: pgtype.Date{Time: target, Valid: true},
	})
	if err == nil {
		return entry.ClosingValue, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	a, err := s.q.GetAsset(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if a.PurchaseCost != nil {
		return *a.PurchaseCost, nil
	}
	return "0", nil
}

// ScheduleRow is one row of Schedule(): either a real entry for the requested
// period+basis, or a synthetic "already fully depreciated, no new entry this
// period" union row (FullyDepreciated true, Amount "0.00").
type ScheduleRow struct {
	AssetID          uuid.UUID
	AssetName        string
	AssetTag         string
	CategoryName     string
	OfficeName       *string
	Method           sqlc.SharedDepreciationMethod
	LifeMonths       int32
	Opening          string
	Amount           string
	Accumulated      string
	Closing          string
	Impaired         bool
	FullyDepreciated bool
}

// ScheduleKPI summarizes Schedule() rows for the dashboard tiles.
type ScheduleKPI struct {
	TotalCost        string
	TotalAccumulated string
	TotalBookValue   string
	PeriodExpense    string
}

// ScheduleTotals summarizes Schedule() rows for the table footer.
type ScheduleTotals struct {
	Opening     string
	Amount      string
	Accumulated string
	Closing     string
}

// ScheduleResult is the outcome of Schedule().
type ScheduleResult struct {
	KPI    ScheduleKPI
	Rows   []ScheduleRow
	Totals ScheduleTotals
}

// Schedule builds the per-asset depreciation schedule for one period+basis,
// scoped to the caller's offices. Rows are the union of (a) real entries
// posted for the period and (b) capitalized, parameterized assets that have
// NO entry this period — these already reached full depreciation earlier, so
// they are rendered with amount "0.00" and opening==closing==their current
// book value, sourced from SumAmountsThroughPeriodByAsset (works uniformly
// for both row kinds, impairment aside — see regenerateBasis's doc comment).
// search matches asset name/tag case-insensitively; categoryID/officeID are
// additional (optional) exact-match filters layered on top of the caller's
// data scope, which is already enforced in SQL via allScope/officeIDs.
func (s *Service) Schedule(ctx context.Context, period time.Time, basis sqlc.SharedDepreciationBasis, allScope bool, officeIDs []uuid.UUID, search string, categoryID, officeID *uuid.UUID) (ScheduleResult, error) {
	target := firstOfMonth(period)
	targetDate := pgtype.Date{Time: target, Valid: true}

	entryRows, err := s.q.ListEntriesForPeriod(ctx, sqlc.ListEntriesForPeriodParams{
		Period: targetDate, Basis: basis, AllScope: allScope, OfficeIds: officeIDs,
	})
	if err != nil {
		return ScheduleResult{}, err
	}

	unionRows, err := s.q.ListAssetsForScheduleUnion(ctx, sqlc.ListAssetsForScheduleUnionParams{
		Period: targetDate, Basis: basis, AllScope: allScope, OfficeIds: officeIDs,
	})
	if err != nil {
		return ScheduleResult{}, err
	}

	accRows, err := s.q.SumAmountsThroughPeriodByAsset(ctx, sqlc.SumAmountsThroughPeriodByAssetParams{
		Basis: basis, Period: targetDate,
	})
	if err != nil {
		return ScheduleResult{}, err
	}
	accByAsset := make(map[uuid.UUID]string, len(accRows))
	for _, r := range accRows {
		accByAsset[r.AssetID] = r.Accumulated
	}

	search = strings.ToLower(strings.TrimSpace(search))
	matchesFilter := func(a sqlc.AssetAsset) bool {
		if categoryID != nil && a.CategoryID != *categoryID {
			return false
		}
		if officeID != nil && a.OfficeID != *officeID {
			return false
		}
		if search != "" && !strings.Contains(strings.ToLower(a.Name), search) && !strings.Contains(strings.ToLower(a.AssetTag), search) {
			return false
		}
		return true
	}

	var rows []ScheduleRow
	totalCost := new(big.Rat)
	totalAccumulated := new(big.Rat)
	totalClosing := new(big.Rat)
	totOpening := new(big.Rat)
	totAmount := new(big.Rat)

	addTotals := func(a sqlc.AssetAsset, opening, amount, accumulated, closing string) error {
		if a.PurchaseCost != nil {
			cost, err := parseMoney(*a.PurchaseCost)
			if err != nil {
				return err
			}
			totalCost.Add(totalCost, cost)
		}
		acc, err := parseMoney(accumulated)
		if err != nil {
			return err
		}
		totalAccumulated.Add(totalAccumulated, acc)
		cl, err := parseMoney(closing)
		if err != nil {
			return err
		}
		totalClosing.Add(totalClosing, cl)
		op, err := parseMoney(opening)
		if err != nil {
			return err
		}
		totOpening.Add(totOpening, op)
		amt, err := parseMoney(amount)
		if err != nil {
			return err
		}
		totAmount.Add(totAmount, amt)
		return nil
	}

	for _, er := range entryRows {
		a := er.AssetAsset
		if !matchesFilter(a) {
			continue
		}
		c := er.MasterdataCategory
		e := er.DepreciationDepreciationEntry
		method, life := resolveScheduleParams(a, c, basis, e.Method)
		accumulated := accByAsset[a.ID]
		if accumulated == "" {
			accumulated = "0.00"
		}
		if err := addTotals(a, e.OpeningValue, e.DepreciationAmount, accumulated, e.ClosingValue); err != nil {
			return ScheduleResult{}, err
		}
		rows = append(rows, ScheduleRow{
			AssetID: a.ID, AssetName: a.Name, AssetTag: a.AssetTag,
			CategoryName: c.Name, OfficeName: er.OfficeName,
			Method: method, LifeMonths: life,
			Opening: e.OpeningValue, Amount: e.DepreciationAmount,
			Accumulated: accumulated, Closing: e.ClosingValue,
			Impaired: isImpaired(a), FullyDepreciated: false,
		})
	}

	for _, ur := range unionRows {
		a := ur.AssetAsset
		c := ur.MasterdataCategory
		var params *Params
		var skip *Skip
		if basis == sqlc.SharedDepreciationBasisCommercial {
			params, skip = ResolveCommercial(a, c)
		} else {
			params, skip = ResolveFiscal(a, c)
		}
		if skip != nil || !matchesFilter(a) {
			continue
		}
		accumulated := accByAsset[a.ID]
		if accumulated == "" {
			accumulated = "0.00"
		}
		cost, err := parseMoney(*a.PurchaseCost)
		if err != nil {
			return ScheduleResult{}, err
		}
		accRat, err := parseMoney(accumulated)
		if err != nil {
			return ScheduleResult{}, err
		}
		bookValue := roundHalfUp2(new(big.Rat).Sub(cost, accRat))
		if err := addTotals(a, bookValue, "0.00", accumulated, bookValue); err != nil {
			return ScheduleResult{}, err
		}
		rows = append(rows, ScheduleRow{
			AssetID: a.ID, AssetName: a.Name, AssetTag: a.AssetTag,
			CategoryName: c.Name, OfficeName: ur.OfficeName,
			Method: params.Method, LifeMonths: params.LifeMonths,
			Opening: bookValue, Amount: "0.00",
			Accumulated: accumulated, Closing: bookValue,
			Impaired: isImpaired(a), FullyDepreciated: true,
		})
	}

	return ScheduleResult{
		KPI: ScheduleKPI{
			TotalCost:        roundHalfUp2(totalCost),
			TotalAccumulated: roundHalfUp2(totalAccumulated),
			TotalBookValue:   roundHalfUp2(totalClosing),
			PeriodExpense:    roundHalfUp2(totAmount),
		},
		Rows: rows,
		Totals: ScheduleTotals{
			Opening:     roundHalfUp2(totOpening),
			Amount:      roundHalfUp2(totAmount),
			Accumulated: roundHalfUp2(totalAccumulated),
			Closing:     roundHalfUp2(totalClosing),
		},
	}, nil
}

// resolveScheduleParams re-resolves an entry row's method/life_months for
// display via ResolveCommercial/ResolveFiscal (entries don't persist
// life_months). Falls back to the entry's own recorded method and
// life_months 0 on the rare data-drift case where the asset/category no
// longer resolves for this basis (e.g. edited after the entry was posted).
func resolveScheduleParams(a sqlc.AssetAsset, c sqlc.MasterdataCategory, basis sqlc.SharedDepreciationBasis, entryMethod sqlc.SharedDepreciationMethod) (sqlc.SharedDepreciationMethod, int32) {
	var params *Params
	var skip *Skip
	if basis == sqlc.SharedDepreciationBasisCommercial {
		params, skip = ResolveCommercial(a, c)
	} else {
		params, skip = ResolveFiscal(a, c)
	}
	if skip != nil || params == nil {
		return entryMethod, 0
	}
	return params.Method, params.LifeMonths
}

// isImpaired reports whether an asset carries a positive impairment_loss.
func isImpaired(a sqlc.AssetAsset) bool {
	if a.ImpairmentLoss == nil {
		return false
	}
	v, err := parseMoney(*a.ImpairmentLoss)
	if err != nil {
		return false
	}
	return v.Sign() > 0
}

// JournalRow is one row of Journal(): a debit line per category GL account,
// or the single closing credit line.
type JournalRow struct {
	AccountCode string
	AccountName string
	Debit       string
	Credit      string
}

// JournalResult is the outcome of Journal().
type JournalResult struct {
	Rows        []JournalRow
	TotalDebit  string
	TotalCredit string
	Balanced    bool
}

// accumulatedGLSettingKey is the app_settings key for the journal's single
// credit account (seeded by migration 000023).
const accumulatedGLSettingKey = "depreciation.accumulated_gl_account"

// Journal builds the depreciation journal entry for one period+basis, scoped
// to the caller's offices: one debit row per distinct category GL account
// ("Beban Penyusutan — {category}"; categories with no GL account code are
// folded into a single "-" / "(tanpa akun GL)" row), and one credit row for
// the configured accumulated-depreciation GL account. Always balances by
// construction (the credit is exactly the sum of the debits).
func (s *Service) Journal(ctx context.Context, period time.Time, basis sqlc.SharedDepreciationBasis, allScope bool, officeIDs []uuid.UUID) (JournalResult, error) {
	target := firstOfMonth(period)
	targetDate := pgtype.Date{Time: target, Valid: true}

	entryRows, err := s.q.ListEntriesForPeriod(ctx, sqlc.ListEntriesForPeriodParams{
		Period: targetDate, Basis: basis, AllScope: allScope, OfficeIds: officeIDs,
	})
	if err != nil {
		return JournalResult{}, err
	}

	type group struct {
		name string
		sum  *big.Rat
	}
	order := make([]string, 0)
	groups := make(map[string]*group)
	total := new(big.Rat)

	for _, er := range entryRows {
		amt, err := parseMoney(er.DepreciationDepreciationEntry.DepreciationAmount)
		if err != nil {
			return JournalResult{}, err
		}
		total.Add(total, amt)

		c := er.MasterdataCategory
		code, name := "-", "(tanpa akun GL)"
		if c.GlAccountCode != nil && *c.GlAccountCode != "" {
			code = *c.GlAccountCode
			name = fmt.Sprintf("Beban Penyusutan — %s", c.Name)
		}
		g, ok := groups[code]
		if !ok {
			g = &group{name: name, sum: new(big.Rat)}
			groups[code] = g
			order = append(order, code)
		}
		g.sum.Add(g.sum, amt)
	}

	rows := make([]JournalRow, 0, len(order)+1)
	for _, code := range order {
		g := groups[code]
		rows = append(rows, JournalRow{AccountCode: code, AccountName: g.name, Debit: roundHalfUp2(g.sum), Credit: "0.00"})
	}

	totalDebit := roundHalfUp2(total)
	creditCode := "-"
	if setting, err := s.q.GetAppSetting(ctx, accumulatedGLSettingKey); err == nil {
		if setting != "" {
			creditCode = setting
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return JournalResult{}, err
	}
	rows = append(rows, JournalRow{AccountCode: creditCode, AccountName: "Akumulasi Penyusutan", Debit: "0.00", Credit: totalDebit})

	return JournalResult{Rows: rows, TotalDebit: totalDebit, TotalCredit: totalDebit, Balanced: true}, nil
}

// AssetScheduleEntry is one row of AssetSchedule()'s entry list.
type AssetScheduleEntry struct {
	Basis   sqlc.SharedDepreciationBasis
	Period  time.Time
	Opening string
	Amount  string
	Closing string
	Method  sqlc.SharedDepreciationMethod
}

// AssetScheduleResult is the outcome of AssetSchedule().
type AssetScheduleResult struct {
	OfficeID          uuid.UUID
	ComputedBookValue string
	Entries           []AssetScheduleEntry
}

// AssetSchedule returns one asset's full depreciation history (both bases)
// plus its current computed (commercial) book value, for GET
// /assets/:id/depreciation. ErrNotFound if the asset does not exist; the
// handler enforces asset-view data scope using the returned OfficeID.
func (s *Service) AssetSchedule(ctx context.Context, assetID uuid.UUID) (AssetScheduleResult, error) {
	a, err := s.q.GetAsset(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AssetScheduleResult{}, ErrNotFound
		}
		return AssetScheduleResult{}, err
	}

	bookValue, err := s.BookValueAsOf(ctx, assetID, time.Now())
	if err != nil {
		return AssetScheduleResult{}, err
	}

	entryRows, err := s.q.ListAssetEntries(ctx, assetID)
	if err != nil {
		return AssetScheduleResult{}, err
	}
	entries := make([]AssetScheduleEntry, 0, len(entryRows))
	for _, e := range entryRows {
		entries = append(entries, AssetScheduleEntry{
			Basis: e.Basis, Period: e.Period.Time, Opening: e.OpeningValue,
			Amount: e.DepreciationAmount, Closing: e.ClosingValue, Method: e.Method,
		})
	}

	return AssetScheduleResult{OfficeID: a.OfficeID, ComputedBookValue: bookValue, Entries: entries}, nil
}

// GetAssetSummary returns the raw asset row. The impairment handler uses it
// to resolve the asset's office (data-scope check) and to snapshot the
// pre-impairment money fields for the audit diff, both BEFORE calling
// RecordImpairment (which re-reads the row itself inside its own tx — this is
// a separate, cheap read, not a substitute for that). ErrNotFound if the
// asset does not exist.
func (s *Service) GetAssetSummary(ctx context.Context, assetID uuid.UUID) (sqlc.AssetAsset, error) {
	a, err := s.q.GetAsset(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.AssetAsset{}, ErrNotFound
		}
		return sqlc.AssetAsset{}, err
	}
	return a, nil
}

// RecordImpairment writes down an asset's book value to a tested recoverable
// amount (PSAK 48 impairment test): impairment_loss accumulates the
// shortfall (book_value − recoverable) and book_value drops to recoverable.
// This does NOT post a depreciation entry — impairment is a separate loss,
// not a depreciation expense — so depreciation history is untouched by
// construction; the next ComputePeriod resumes depreciation from the lower
// book_value via regenerateBasis's commercial resumption override (it
// compares the last closed entry's closing against asset.book_value and
// takes whichever is lower). reason is caller-supplied context for the audit
// trail (there is no dedicated schema column for it) — the handler folds it
// into the audit.Diff payload, not this method (ADR-0008: no Gin/audit
// wiring in the service layer).
// reason and actor are not used inside this method (documented above / no
// actor column on asset.assets) — kept as parameters so the handler's call
// site carries the full audit context in one place, and for signature
// symmetry with a possible future audit-in-service wiring.
func (s *Service) RecordImpairment(ctx context.Context, assetID uuid.UUID, recoverable string, reason string, actor uuid.UUID) (sqlc.AssetAsset, error) {
	recoverableRat, ok := parsePlainDecimal(recoverable)
	if !ok {
		return sqlc.AssetAsset{}, ErrInvalidRecoverable
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.AssetAsset{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	// Serialize against ComputePeriod/ClosePeriod (same transaction-scoped
	// advisory lock they take, as their first statement): compute's
	// refreshAssetSummary rewrites asset.book_value from the entries, so an
	// impairment committing mid-compute would get its book_value clobbered
	// back to the pre-impairment closing while impairment_loss kept the
	// write-down — inconsistent state, and the next recompute's lower-of
	// override would then see the clobbered-higher book_value, silently
	// losing the impairment.
	if err := qtx.AdvisoryLockDepreciation(ctx); err != nil {
		return sqlc.AssetAsset{}, err
	}

	// Row-locked read (FOR UPDATE): this method is a read-modify-write over
	// book_value/impairment_loss, so a second concurrent impairment of the
	// same asset must block HERE and re-read the post-commit values once the
	// first commits — a plain read would compute its delta from a stale book
	// value and silently clobber the first write (lost update; understated
	// cumulative write-down). The advisory lock above happens to serialize
	// impairment-vs-impairment too, but the row lock is kept deliberately:
	// it is the correctness guarantee scoped to THIS row's read-modify-write
	// and must survive any future refactor that narrows or drops the
	// module-wide advisory lock (e.g. finer-grained compute locking).
	a, err := qtx.GetAssetForUpdate(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.AssetAsset{}, ErrNotFound
		}
		return sqlc.AssetAsset{}, err
	}

	if a.BookValue == nil {
		return sqlc.AssetAsset{}, ErrNoBookValue
	}
	bookValue, err := parseMoney(*a.BookValue)
	if err != nil {
		return sqlc.AssetAsset{}, err
	}

	if recoverableRat.Sign() < 0 || recoverableRat.Cmp(bookValue) >= 0 {
		return sqlc.AssetAsset{}, ErrInvalidRecoverable
	}

	loss := new(big.Rat).Sub(bookValue, recoverableRat)
	existingImpairment := new(big.Rat)
	if a.ImpairmentLoss != nil {
		if v, errImp := parseMoney(*a.ImpairmentLoss); errImp == nil {
			existingImpairment = v
		}
	}
	newImpairment := roundHalfUp2(new(big.Rat).Add(existingImpairment, loss))
	newBookValue := roundHalfUp2(recoverableRat)

	updated, err := qtx.ApplyAssetImpairment(ctx, sqlc.ApplyAssetImpairmentParams{
		ID:             assetID,
		ImpairmentLoss: &newImpairment,
		BookValue:      &newBookValue,
	})
	if err != nil {
		return sqlc.AssetAsset{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlc.AssetAsset{}, err
	}
	return updated, nil
}
