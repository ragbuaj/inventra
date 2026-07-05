// Package depreciation implements dual-basis asset depreciation: commercial
// (PSAK 16) and fiscal (PMK 72/2023). This file (engine.go) is the pure
// calculation core — no DB, no Gin, only deterministic math over decimal
// strings via math/big.Rat — split out per ADR-0008 so the month-walk logic
// is unit-testable in isolation from persistence/orchestration (service.go,
// a later task).
package depreciation

import (
	"fmt"
	"math/big"
	"time"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Params are fully-resolved calculation inputs for one (asset, basis) pair —
// either the commercial (PSAK 16) or fiscal (PMK 72/2023) view of the same
// asset. Money fields are plain decimal strings (no thousands separators),
// matching the Postgres numeric/string convention used across the backend.
// Cost and Salvage MUST carry at most 2 decimal places (the resolvers guarantee
// this via numeric(18,2)) — the exactness invariant SUM(amounts)+salvage == cost
// holds only for 2dp-clean inputs, so never hand-build Params with finer values.
type Params struct {
	Method     sqlc.SharedDepreciationMethod
	LifeMonths int32
	Cost       string    // decimal string
	Salvage    string    // "0" for fiscal (PMK 72/2023 recognizes no residual value)
	Start      time.Time // first day of purchase month (UTC)
	// FinalAbsorb documents that this basis is a declining-balance run whose
	// final month is expected to write off the entire remaining balance
	// ("disusutkan sekaligus" per PMK 72/2023). Walk enforces full absorption
	// at the last month of useful life for EVERY method (see Walk internals
	// below) so this flag does not gate behavior inside Walk itself; it is
	// carried on Params purely as resolver-produced documentation for
	// consumers (service.go / UI) of *why* the last entry is a full write-off.
	FinalAbsorb bool
}

// Entry is one generated month of depreciation for a single (asset, basis).
type Entry struct {
	Period  time.Time // first day of month
	Opening string
	Amount  string
	Closing string
	Method  sqlc.SharedDepreciationMethod
}

// Skip explains why an asset produces no entries for a basis. Valid reasons:
// "not_capitalized", "no_cost", "no_purchase_date", "missing_params",
// "disposed", "non_susut". (Fiscal buildings requesting declining_balance are
// NOT a skip — the method falls back to straight_line; see ResolveFiscal.)
type Skip struct{ Reason string }

// FiscalRule is one row of the verified PMK 72/2023 parameter table
// (Lampiran A.1), exported for reuse by tests and UI documentation.
//
// The declining-balance annual percentages below are, by design of the
// regulation, exactly double the straight-line percentage for the same life
// (i.e. classic double-declining-balance: 2 / usefulLifeYears). Because of
// that, Walk needs only ONE declining-balance formula — 2/LifeMonths per
// month — to correctly serve BOTH commercial (PSAK 16) and fiscal (PMK
// 72/2023) declining balance, as long as LifeMonths is resolved correctly
// per basis (asset/category life for commercial; this table for fiscal).
// StraightLinePct/DecliningPct are therefore descriptive (docs/UI), not
// consumed directly by Walk's arithmetic.
type FiscalRule struct {
	LifeMonths      int32
	StraightLinePct string // annual, e.g. "25"
	DecliningPct    string // annual, "" = declining not allowed (buildings)
}

// FiscalRules is the PMK 72/2023 Lampiran A.1 constant table. non_susut is
// intentionally absent: it is handled as an explicit Skip before any lookup.
var FiscalRules = map[sqlc.SharedFiscalAssetGroup]FiscalRule{
	sqlc.SharedFiscalAssetGroupKelompok1:           {LifeMonths: 48, StraightLinePct: "25", DecliningPct: "50"},
	sqlc.SharedFiscalAssetGroupKelompok2:           {LifeMonths: 96, StraightLinePct: "12.5", DecliningPct: "25"},
	sqlc.SharedFiscalAssetGroupKelompok3:           {LifeMonths: 192, StraightLinePct: "6.25", DecliningPct: "12.5"},
	sqlc.SharedFiscalAssetGroupKelompok4:           {LifeMonths: 240, StraightLinePct: "5", DecliningPct: "10"},
	sqlc.SharedFiscalAssetGroupBangunanPermanen:    {LifeMonths: 240, StraightLinePct: "5", DecliningPct: ""},
	sqlc.SharedFiscalAssetGroupBangunanNonPermanen: {LifeMonths: 120, StraightLinePct: "10", DecliningPct: ""},
}

// Walk generates the months AFTER lastPeriod (nil ⇒ from p.Start) through
// target inclusive. lastClosing (nil ⇒ p.Cost) is the opening balance for the
// first generated month — honored whether we resume after lastPeriod or start
// fresh from p.Start (see the firstMonth/opening block for why the impairment
// override depends on this). Returns nil (no error) when the asset is already
// fully depreciated (opening has reached salvage) or target is before the
// first month to generate.
//
// Normative algorithm (PSAK 16 §58 / PMK 72/2023 catch-up-safe form):
//   - remaining = LifeMonths − monthsElapsed(Start, period); stop (no more
//     entries) once remaining <= 0 (useful life exhausted) or opening has
//     already reached salvage (fully depreciated).
//   - straight line: amount = (opening − salvage) / remaining, RECOMPUTED
//     every month from the current (rounded) opening — this is exactly what
//     makes impairment / estimate changes prospective by construction; the
//     monthly amount is deliberately never precomputed/cached.
//   - declining balance: amount = opening × (2 / LifeMonths) — see FiscalRule
//     doc comment for why one formula serves both bases.
//   - amount is clamped to opening−salvage; the final month of useful life
//     (remaining == 1) always absorbs the entire remainder, so the closing
//     balance lands on salvage EXACTLY (commercial) / 0 EXACTLY (fiscal).
//   - all arithmetic is exact math/big.Rat; each Entry's strings are produced
//     by roundHalfUp2, and the NEXT month's opening is the ROUNDED closing
//     (not the exact one) — the ledger is self-consistent to the cent.
func Walk(p Params, lastPeriod *time.Time, lastClosing *string, target time.Time) ([]Entry, error) {
	if p.LifeMonths <= 0 {
		return nil, fmt.Errorf("depreciation: params.LifeMonths must be positive, got %d", p.LifeMonths)
	}
	cost, err := parseMoney(p.Cost)
	if err != nil {
		return nil, fmt.Errorf("depreciation: params.Cost: %w", err)
	}
	salvage, err := parseMoney(p.Salvage)
	if err != nil {
		return nil, fmt.Errorf("depreciation: params.Salvage: %w", err)
	}

	start := firstOfMonth(p.Start)
	target = firstOfMonth(target)

	var firstMonth time.Time
	if lastPeriod == nil {
		firstMonth = start
	} else {
		firstMonth = addMonths(firstOfMonth(*lastPeriod), 1)
	}
	// lastClosing (nil ⇒ p.Cost) is the opening balance for the first generated
	// month, INDEPENDENT of whether we resume after lastPeriod or start fresh
	// from p.Start. The impairment resumption override (service.go) relies on
	// this: it passes an impaired-floor lastClosing with a nil lastPeriod to
	// open the genesis walk from the floor when nothing has been closed yet.
	// `remaining` is always measured from p.Start (not lastPeriod), so a lower
	// opening never distorts the useful-life clock.
	var opening *big.Rat
	if lastClosing == nil {
		opening = cost
	} else {
		opening, err = parseMoney(*lastClosing)
		if err != nil {
			return nil, fmt.Errorf("depreciation: lastClosing: %w", err)
		}
	}

	if target.Before(firstMonth) {
		return nil, nil
	}

	// Double-declining-balance monthly factor: 2 / (LifeMonths/12) / 12,
	// simplified to the exact rational 2/LifeMonths (avoids any integer
	// truncation of LifeMonths/12 for non-standard, non-12-divisible lives).
	decliningFactor := big.NewRat(2, int64(p.LifeMonths))

	var entries []Entry
	period := firstMonth
	for !period.After(target) {
		remaining := p.LifeMonths - monthsElapsed(start, period)
		if remaining <= 0 {
			break // useful life exhausted: no more entries
		}
		if opening.Cmp(salvage) <= 0 {
			break // already fully depreciated: no more entries
		}

		maxAmount := new(big.Rat).Sub(opening, salvage)

		var amount *big.Rat
		switch p.Method {
		case sqlc.SharedDepreciationMethodStraightLine:
			amount = new(big.Rat).Quo(maxAmount, big.NewRat(int64(remaining), 1))
		case sqlc.SharedDepreciationMethodDecliningBalance:
			amount = new(big.Rat).Mul(opening, decliningFactor)
		default:
			return nil, fmt.Errorf("depreciation: unknown method %q", p.Method)
		}

		if amount.Cmp(maxAmount) > 0 {
			amount = maxAmount
		}
		if remaining == 1 {
			// Last month of useful life: absorb the entire remainder so the
			// closing balance is EXACTLY salvage (commercial) / 0 (fiscal),
			// regardless of any rounding drift accumulated along the way.
			amount = maxAmount
		}

		// Round the amount ONCE and derive closing from that rounded value
		// (not from opening − exact-amount, rounded independently). Rounding
		// amount and closing separately can disagree on an exact half-cent
		// tie (opening − amount and amount can both land on an X.XX5
		// boundary and both round "up"), which would silently break the
		// opening=amount+closing ledger identity and the SUM(amount)==cost
		// invariant across the whole schedule. Since opening is already a
		// clean 2dp value, opening − roundedAmount is exact and rounding it
		// again is a no-op — so ledger self-consistency is by construction.
		amountRounded := roundHalfUp2(amount)
		amountForLedger, err := parseMoney(amountRounded)
		if err != nil {
			return nil, fmt.Errorf("depreciation: internal rounding error: %w", err)
		}
		closingExact := new(big.Rat).Sub(opening, amountForLedger)
		entry := Entry{
			Period:  period,
			Opening: roundHalfUp2(opening),
			Amount:  amountRounded,
			Closing: roundHalfUp2(closingExact),
			Method:  p.Method,
		}
		entries = append(entries, entry)

		// Next month's opening is the ROUNDED closing, not the exact
		// fractional one — the ledger must be self-consistent to the cent.
		opening, err = parseMoney(entry.Closing)
		if err != nil {
			return nil, fmt.Errorf("depreciation: internal rounding error: %w", err)
		}
		period = addMonths(period, 1)
	}

	return entries, nil
}

// ResolveCommercial resolves commercial (PSAK 16) Params for an asset, with
// the asset's own override winning over the category default. A non-nil
// *Skip means "no entries, report reason" and Params is nil in that case.
func ResolveCommercial(a sqlc.AssetAsset, c sqlc.MasterdataCategory) (*Params, *Skip) {
	if a.Status == sqlc.SharedAssetStatusDisposed {
		return nil, &Skip{Reason: "disposed"}
	}
	if !a.Capitalized {
		return nil, &Skip{Reason: "not_capitalized"}
	}
	if a.PurchaseCost == nil || *a.PurchaseCost == "" {
		return nil, &Skip{Reason: "no_cost"}
	}
	if !a.PurchaseDate.Valid {
		return nil, &Skip{Reason: "no_purchase_date"}
	}

	method := a.DepreciationMethod
	if method == nil {
		method = c.DefaultDepreciationMethod
	}
	life := a.UsefulLifeMonths
	if life == nil {
		life = c.DefaultUsefulLifeMonths
	}
	if method == nil || life == nil {
		return nil, &Skip{Reason: "missing_params"}
	}

	salvage := "0"
	switch {
	case a.SalvageValue != nil:
		salvage = *a.SalvageValue
	case c.DefaultSalvageRate != nil:
		cost, err := parseMoney(*a.PurchaseCost)
		if err != nil {
			return nil, &Skip{Reason: "missing_params"}
		}
		rate, err := parseMoney(*c.DefaultSalvageRate)
		if err != nil {
			return nil, &Skip{Reason: "missing_params"}
		}
		salvage = roundHalfUp2(new(big.Rat).Mul(cost, rate))
	}

	return &Params{
		Method:     *method,
		LifeMonths: *life,
		Cost:       *a.PurchaseCost,
		Salvage:    salvage,
		Start:      firstOfMonth(a.PurchaseDate.Time),
	}, nil
}

// ResolveFiscal resolves fiscal (PMK 72/2023) Params for an asset. Life and
// rates come from the FiscalRules constant table — asset.FiscalLifeMonths is
// ignored, the table is normative. Fiscal recognizes no salvage/residual
// value. Method follows the asset's resolved commercial method when valid
// for the fiscal group (declining balance is not valid for buildings, which
// are always straight_line fiscally — a FALLBACK, not a skip).
func ResolveFiscal(a sqlc.AssetAsset, c sqlc.MasterdataCategory) (*Params, *Skip) {
	if a.Status == sqlc.SharedAssetStatusDisposed {
		return nil, &Skip{Reason: "disposed"}
	}
	if !a.Capitalized {
		return nil, &Skip{Reason: "not_capitalized"}
	}
	if a.PurchaseCost == nil || *a.PurchaseCost == "" {
		return nil, &Skip{Reason: "no_cost"}
	}
	if !a.PurchaseDate.Valid {
		return nil, &Skip{Reason: "no_purchase_date"}
	}

	group := a.FiscalGroup
	if group == nil {
		group = c.DefaultFiscalGroup
	}
	if group == nil {
		return nil, &Skip{Reason: "missing_params"}
	}
	if *group == sqlc.SharedFiscalAssetGroupNonSusut {
		return nil, &Skip{Reason: "non_susut"}
	}
	rule, ok := FiscalRules[*group]
	if !ok {
		return nil, &Skip{Reason: "missing_params"}
	}

	commercialMethod := a.DepreciationMethod
	if commercialMethod == nil {
		commercialMethod = c.DefaultDepreciationMethod
	}
	isBuilding := rule.DecliningPct == ""

	method := sqlc.SharedDepreciationMethodStraightLine
	finalAbsorb := false
	if commercialMethod != nil && *commercialMethod == sqlc.SharedDepreciationMethodDecliningBalance && !isBuilding {
		method = sqlc.SharedDepreciationMethodDecliningBalance
		finalAbsorb = true
	}

	return &Params{
		Method:      method,
		LifeMonths:  rule.LifeMonths,
		Cost:        *a.PurchaseCost,
		Salvage:     "0",
		Start:       firstOfMonth(a.PurchaseDate.Time),
		FinalAbsorb: finalAbsorb,
	}, nil
}

// parseMoney parses a plain decimal string (as stored for Postgres numeric
// columns) into an exact rational.
func parseMoney(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", s)
	}
	return r, nil
}

// firstOfMonth normalizes t to the first day of its month at UTC midnight.
func firstOfMonth(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// addMonths returns the first day of the month n months after t (t must
// already be a first-of-month value); Go's time.Date normalizes month
// overflow/underflow (e.g. month 13 rolls into next year's January).
func addMonths(t time.Time, n int) time.Time {
	return time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
}

// monthsElapsed returns the number of whole calendar months between two
// first-of-month dates (0 when period == start).
func monthsElapsed(start, period time.Time) int32 {
	return int32((period.Year()-start.Year())*12 + int(period.Month()-start.Month()))
}

// roundHalfUp2 renders an exact rational as a decimal string rounded to 2
// decimal places, half-up (e.g. "0.005" → "0.01"). Amounts in this package
// are always non-negative, so negative-value rounding is not implemented.
func roundHalfUp2(r *big.Rat) string {
	scaled := new(big.Rat).Mul(r, big.NewRat(100, 1))
	shifted := new(big.Rat).Add(scaled, big.NewRat(1, 2))
	// shifted is non-negative by contract; truncating division toward zero
	// equals floor() for non-negative rationals.
	q := new(big.Int).Quo(shifted.Num(), shifted.Denom())

	s := q.String()
	for len(s) < 3 {
		s = "0" + s
	}
	whole, frac := s[:len(s)-2], s[len(s)-2:]
	return whole + "." + frac
}
