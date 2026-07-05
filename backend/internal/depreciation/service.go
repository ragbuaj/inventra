// Package depreciation — service.go orchestrates the monthly compute/close
// period lifecycle on top of the pure calculation core (engine.go). No Gin
// here (ADR-0008): plain domain types and sentinel errors only; the handler
// (a later task) translates these to HTTP status codes and enforces
// permission/scope.
package depreciation

import (
	"context"
	"errors"
	"math/big"
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
