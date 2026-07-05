package depreciation

import (
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// ---- test helpers ----

func date(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func ptr[T any](v T) *T { return &v }

// mustWalk runs a fresh Walk (no prior entries) from p.Start through target.
func mustWalk(t *testing.T, p Params, target time.Time) []Entry {
	t.Helper()
	entries, err := Walk(p, nil, nil, target)
	require.NoError(t, err)
	return entries
}

// mustWalkFrom resumes a Walk after lastPeriod/lastClosing through target.
func mustWalkFrom(t *testing.T, p Params, lastPeriod time.Time, lastClosing string, target time.Time) []Entry {
	t.Helper()
	entries, err := Walk(p, &lastPeriod, &lastClosing, target)
	require.NoError(t, err)
	return entries
}

func mustRat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	require.True(t, ok, "invalid decimal %q", s)
	return r
}

func sumAmounts(t *testing.T, entries []Entry) *big.Rat {
	t.Helper()
	sum := new(big.Rat)
	for _, e := range entries {
		sum.Add(sum, mustRat(t, e.Amount))
	}
	return sum
}

func baseAsset() sqlc.AssetAsset {
	cost := "10000000"
	return sqlc.AssetAsset{
		Status:       sqlc.SharedAssetStatusAvailable,
		Capitalized:  true,
		PurchaseCost: &cost,
		PurchaseDate: pgtype.Date{Time: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), Valid: true},
	}
}

func baseCategory() sqlc.MasterdataCategory {
	return sqlc.MasterdataCategory{}
}

// =====================================================================
// Walk: commercial straight line
// =====================================================================

func TestWalk_CommercialStraightLine(t *testing.T) {
	t.Run("SL_48m_no_salvage", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 48,
			Cost:       "18500000",
			Salvage:    "0",
			Start:      date(2024, 1),
		}
		entries := mustWalk(t, p, addMonths(date(2024, 1), 47))
		require.Len(t, entries, 48)
		assert.Equal(t, "385416.67", entries[0].Amount)
		assert.Equal(t, "18500000.00", entries[0].Opening)
		assert.Equal(t, "0.00", entries[47].Closing)
		assert.Equal(t, 0, sumAmounts(t, entries).Cmp(mustRat(t, "18500000")))
	})

	t.Run("SL_salvage_innova", func(t *testing.T) {
		// Cost 300jt, salvage 60jt, 96 bulan -> monthly 2,500,000.00 exactly.
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 96,
			Cost:       "300000000",
			Salvage:    "60000000",
			Start:      date(2020, 6),
		}
		entries := mustWalk(t, p, addMonths(date(2020, 6), 95))
		require.Len(t, entries, 96)
		for i, e := range entries {
			assert.Equalf(t, "2500000.00", e.Amount, "month %d", i+1)
		}
		assert.Equal(t, "60000000.00", entries[95].Closing)
	})

	t.Run("SL_rounding_absorb", func(t *testing.T) {
		// cost 1000, 3 months, salvage 0.
		// Hand-verified (NOT the brief's illustrative 333.33/333.33/333.34):
		//   m1: 1000/3           = 333.333... -> round half-up -> 333.33; closing 666.67
		//   m2: 666.67/2         = 333.335     -> round half-up -> 333.34; closing 333.33
		//   m3 (last, forced):     333.33 - 0  =             333.33; closing 0.00
		// Sum = 333.33+333.34+333.33 = 1000.00 exactly.
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 3,
			Cost:       "1000",
			Salvage:    "0",
			Start:      date(2023, 1),
		}
		entries := mustWalk(t, p, addMonths(date(2023, 1), 2))
		require.Len(t, entries, 3)
		assert.Equal(t, "333.33", entries[0].Amount)
		assert.Equal(t, "333.34", entries[1].Amount)
		assert.Equal(t, "333.33", entries[2].Amount)
		assert.Equal(t, "0.00", entries[2].Closing)
		assert.Equal(t, 0, sumAmounts(t, entries).Cmp(mustRat(t, "1000")))
	})

	t.Run("SL_memo_value_rp1", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 12,
			Cost:       "5000000",
			Salvage:    "1",
			Start:      date(2022, 1),
		}
		entries := mustWalk(t, p, addMonths(date(2022, 1), 11))
		require.Len(t, entries, 12)
		assert.Equal(t, "1.00", entries[11].Closing)
	})
}

// =====================================================================
// Walk: commercial declining balance
// =====================================================================

func TestWalk_CommercialDecliningBalance(t *testing.T) {
	t.Run("DB_floor_at_salvage", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodDecliningBalance,
			LifeMonths: 48,
			Cost:       "1000000",
			Salvage:    "0",
			Start:      date(2021, 1),
		}
		entries := mustWalk(t, p, addMonths(date(2021, 1), 47))
		require.Len(t, entries, 48)

		amount1 := mustRat(t, entries[0].Amount)
		amount2 := mustRat(t, entries[1].Amount)
		assert.Equal(t, -1, amount2.Cmp(amount1), "declining: month 2 amount must be < month 1 amount")

		salvage := mustRat(t, p.Salvage)
		for i, e := range entries {
			closing := mustRat(t, e.Closing)
			assert.False(t, closing.Cmp(salvage) < 0, "closing at month %d must never be below salvage", i+1)
		}
		// Life fully absorbed by the final month.
		assert.Equal(t, "0.00", entries[47].Closing)
		assert.Equal(t, entries[47].Amount, entries[47].Opening) // full remainder written off
	})
}

// =====================================================================
// Walk: fiscal (PMK 72/2023)
// =====================================================================

func TestWalk_Fiscal(t *testing.T) {
	t.Run("FIS_SL_kelompok1_matches_commercial_SL", func(t *testing.T) {
		commercial := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 48,
			Cost:       "9600000",
			Salvage:    "0",
			Start:      date(2023, 7),
		}
		rule := FiscalRules[sqlc.SharedFiscalAssetGroupKelompok1]
		require.Equal(t, int32(48), rule.LifeMonths)
		fiscal := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: rule.LifeMonths,
			Cost:       "9600000",
			Salvage:    "0",
			Start:      date(2023, 7),
		}
		target := addMonths(date(2023, 7), 47)
		commercialEntries := mustWalk(t, commercial, target)
		fiscalEntries := mustWalk(t, fiscal, target)
		require.Equal(t, len(commercialEntries), len(fiscalEntries))
		for i := range commercialEntries {
			assert.Equal(t, commercialEntries[i].Amount, fiscalEntries[i].Amount, "month %d", i+1)
			assert.Equal(t, commercialEntries[i].Closing, fiscalEntries[i].Closing, "month %d", i+1)
		}
	})

	t.Run("FIS_DB_kelompok1_final_absorb", func(t *testing.T) {
		rule := FiscalRules[sqlc.SharedFiscalAssetGroupKelompok1]
		require.Equal(t, "50", rule.DecliningPct)
		p := Params{
			Method:      sqlc.SharedDepreciationMethodDecliningBalance,
			LifeMonths:  rule.LifeMonths,
			Cost:        "5000000",
			Salvage:     "0",
			Start:       date(2019, 4),
			FinalAbsorb: true,
		}
		entries := mustWalk(t, p, addMonths(date(2019, 4), int(rule.LifeMonths)-1))
		require.Len(t, entries, int(rule.LifeMonths))
		last := entries[len(entries)-1]
		assert.Equal(t, "0.00", last.Closing, "disusutkan sekaligus: last month zeroes out fiscal book value")
		assert.Equal(t, last.Amount, last.Opening, "last month amount must equal the whole remaining opening")
	})
}

// =====================================================================
// Walk: resumption / catch-up
// =====================================================================

func TestWalk_ResumptionAndCatchUp(t *testing.T) {
	basisParams := func() Params {
		return Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 96,
			Cost:       "9600000",
			Salvage:    "0",
			Start:      date(2021, 1),
		}
	}

	t.Run("resume_from_last", func(t *testing.T) {
		p := basisParams()
		first10 := mustWalk(t, p, addMonths(date(2021, 1), 9)) // months index 0..9
		require.Len(t, first10, 10)
		entry10 := first10[9]

		next10 := mustWalkFrom(t, p, entry10.Period, entry10.Closing, addMonths(date(2021, 1), 19))
		require.Len(t, next10, 10)

		singleWalk := mustWalk(t, p, addMonths(date(2021, 1), 19))
		require.Len(t, singleWalk, 20)

		concatenated := append(append([]Entry{}, first10...), next10...)
		require.Len(t, concatenated, 20)
		for i := range singleWalk {
			assert.Equal(t, singleWalk[i].Period, concatenated[i].Period, "period at index %d", i)
			assert.Equal(t, singleWalk[i].Opening, concatenated[i].Opening, "opening at index %d", i)
			assert.Equal(t, singleWalk[i].Amount, concatenated[i].Amount, "amount at index %d", i)
			assert.Equal(t, singleWalk[i].Closing, concatenated[i].Closing, "closing at index %d", i)
		}
	})

	t.Run("catch_up_multi_year", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 120,
			Cost:       "12000000",
			Salvage:    "0",
			Start:      date(2020, 5),
		}
		entries := mustWalk(t, p, date(2026, 7))
		// monthsElapsed(2020-05, 2026-07) = 74 (0-based) -> 75 entries.
		require.Len(t, entries, 75)
		for i, e := range entries {
			assert.Equal(t, addMonths(date(2020, 5), i), e.Period, "periods must be contiguous at index %d", i)
		}
	})

	t.Run("fully_depreciated_no_new", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 12,
			Cost:       "1200000",
			Salvage:    "0",
			Start:      date(2022, 1),
		}
		full := mustWalk(t, p, addMonths(date(2022, 1), 11))
		require.Len(t, full, 12)
		last := full[11]
		require.Equal(t, "0.00", last.Closing)

		entries, err := Walk(p, &last.Period, &last.Closing, addMonths(last.Period, 6))
		require.NoError(t, err)
		assert.Nil(t, entries)
	})

	t.Run("target_before_start", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 12,
			Cost:       "1200000",
			Salvage:    "0",
			Start:      date(2023, 1),
		}
		entries, err := Walk(p, nil, nil, date(2022, 1))
		require.NoError(t, err)
		assert.Nil(t, entries)
	})
}

// =====================================================================
// Walk: prospective changes (impairment / salvage estimate revisions)
// =====================================================================

func TestWalk_ProspectiveChanges(t *testing.T) {
	t.Run("impairment_prospective", func(t *testing.T) {
		p := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 48,
			Cost:       "4800",
			Salvage:    "0",
			Start:      date(2019, 6),
		}
		first12 := mustWalk(t, p, addMonths(date(2019, 6), 11))
		require.Len(t, first12, 12)
		last := first12[11]
		require.Equal(t, "3600.00", last.Closing)

		// Impairment write-down: caller substitutes the post-impairment book
		// value (1000) as the opening for the next month instead of 3600.
		nextMonth := mustWalkFrom(t, p, last.Period, "1000", addMonths(last.Period, 1))
		require.Len(t, nextMonth, 1)
		// remaining = 48 - 12 = 36 -> 1000/36 = 27.7777... -> 27.78
		assert.Equal(t, "27.78", nextMonth[0].Amount)

		// Continuing to the final month of useful life still lands exactly
		// on salvage (0), regardless of the mid-life impairment.
		rest := mustWalkFrom(t, p, last.Period, "1000", addMonths(last.Period, 36))
		require.Len(t, rest, 36)
		assert.Equal(t, "0.00", rest[35].Closing)
	})

	t.Run("salvage_change_prospective", func(t *testing.T) {
		p1 := Params{
			Method:     sqlc.SharedDepreciationMethodStraightLine,
			LifeMonths: 24,
			Cost:       "2400",
			Salvage:    "0",
			Start:      date(2024, 1),
		}
		firstHalf := mustWalk(t, p1, addMonths(date(2024, 1), 11))
		require.Len(t, firstHalf, 12)
		last := firstHalf[11]
		require.Equal(t, "1200.00", last.Closing)

		// Change the salvage estimate for the remaining months; past entries
		// (firstHalf) are untouched by construction (Walk never mutates
		// history — it only ever returns newly generated entries).
		p2 := p1
		p2.Salvage = "200"
		secondHalf := mustWalkFrom(t, p2, last.Period, last.Closing, addMonths(last.Period, 12))
		require.Len(t, secondHalf, 12)
		// remaining = 24 - 12 = 12 -> (1200-200)/12 = 83.3333... -> 83.33
		assert.Equal(t, "83.33", secondHalf[0].Amount)
		// Final month absorbs to the NEW salvage exactly.
		assert.Equal(t, "200.00", secondHalf[11].Closing)

		// History is provably untouched: re-derive firstHalf's first entry
		// independently and confirm it is unaffected by p2.
		assert.Equal(t, "100.00", firstHalf[0].Amount)
		assert.Equal(t, "2400.00", firstHalf[0].Opening)
	})
}

// =====================================================================
// roundHalfUp2
// =====================================================================

func TestRoundHalfUp2(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0.005", "0.01"},
		{"1.004", "1.00"},
		{"10", "10.00"},
		{"0", "0.00"},
		{"33.335", "33.34"},
		// Classic float64 rounding trap (2.675 stored as 2.67499999...): with
		// exact big.Rat parsing from the decimal string, this rounds UP
		// correctly, unlike naive float64 * 100 math would.
		{"2.675", "2.68"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := roundHalfUp2(mustRat(t, tc.in))
			assert.Equal(t, tc.want, got)
		})
	}
}

// =====================================================================
// Resolvers
// =====================================================================

func TestResolveCommercial(t *testing.T) {
	t.Run("asset_override_wins", func(t *testing.T) {
		a := baseAsset()
		a.DepreciationMethod = ptr(sqlc.SharedDepreciationMethodDecliningBalance)
		a.UsefulLifeMonths = ptr(int32(60))
		a.SalvageValue = ptr("500000")
		c := baseCategory()
		c.DefaultDepreciationMethod = ptr(sqlc.SharedDepreciationMethodStraightLine)
		c.DefaultUsefulLifeMonths = ptr(int32(36))
		c.DefaultSalvageRate = ptr("0.05")

		p, skip := ResolveCommercial(a, c)
		require.Nil(t, skip)
		require.NotNil(t, p)
		assert.Equal(t, sqlc.SharedDepreciationMethodDecliningBalance, p.Method)
		assert.Equal(t, int32(60), p.LifeMonths)
		assert.Equal(t, "500000", p.Salvage)
		assert.Equal(t, "10000000", p.Cost)
		assert.Equal(t, date(2024, 3), p.Start)
	})

	t.Run("category_fallback", func(t *testing.T) {
		a := baseAsset() // no overrides
		c := baseCategory()
		c.DefaultDepreciationMethod = ptr(sqlc.SharedDepreciationMethodStraightLine)
		c.DefaultUsefulLifeMonths = ptr(int32(96))
		c.DefaultSalvageRate = ptr("0.1000")

		p, skip := ResolveCommercial(a, c)
		require.Nil(t, skip)
		require.NotNil(t, p)
		assert.Equal(t, sqlc.SharedDepreciationMethodStraightLine, p.Method)
		assert.Equal(t, int32(96), p.LifeMonths)
		// cost 10_000_000 x rate 0.1000 -> salvage 1_000_000.00
		assert.Equal(t, "1000000.00", p.Salvage)
	})

	t.Run("missing_params_no_method_no_life", func(t *testing.T) {
		a := baseAsset()
		c := baseCategory()
		_, skip := ResolveCommercial(a, c)
		require.NotNil(t, skip)
		assert.Equal(t, "missing_params", skip.Reason)
	})

	t.Run("missing_params_life_nil", func(t *testing.T) {
		a := baseAsset()
		a.DepreciationMethod = ptr(sqlc.SharedDepreciationMethodStraightLine)
		c := baseCategory() // no default life either
		_, skip := ResolveCommercial(a, c)
		require.NotNil(t, skip)
		assert.Equal(t, "missing_params", skip.Reason)
	})

	t.Run("not_capitalized", func(t *testing.T) {
		a := baseAsset()
		a.Capitalized = false
		_, skip := ResolveCommercial(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "not_capitalized", skip.Reason)
	})

	t.Run("no_cost", func(t *testing.T) {
		a := baseAsset()
		a.PurchaseCost = nil
		_, skip := ResolveCommercial(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "no_cost", skip.Reason)
	})

	t.Run("no_purchase_date", func(t *testing.T) {
		a := baseAsset()
		a.PurchaseDate = pgtype.Date{}
		_, skip := ResolveCommercial(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "no_purchase_date", skip.Reason)
	})

	t.Run("disposed", func(t *testing.T) {
		a := baseAsset()
		a.Status = sqlc.SharedAssetStatusDisposed
		_, skip := ResolveCommercial(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "disposed", skip.Reason)
	})
}

func TestResolveFiscal(t *testing.T) {
	t.Run("non_susut", func(t *testing.T) {
		a := baseAsset()
		a.FiscalGroup = ptr(sqlc.SharedFiscalAssetGroupNonSusut)
		_, skip := ResolveFiscal(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "non_susut", skip.Reason)
	})

	t.Run("no_group", func(t *testing.T) {
		a := baseAsset() // no FiscalGroup override
		c := baseCategory()
		_, skip := ResolveFiscal(a, c) // no DefaultFiscalGroup either
		require.NotNil(t, skip)
		assert.Equal(t, "missing_params", skip.Reason)
	})

	t.Run("building_falls_back_to_SL", func(t *testing.T) {
		a := baseAsset()
		a.FiscalGroup = ptr(sqlc.SharedFiscalAssetGroupBangunanPermanen)
		a.DepreciationMethod = ptr(sqlc.SharedDepreciationMethodDecliningBalance)

		p, skip := ResolveFiscal(a, baseCategory())
		require.Nil(t, skip)
		require.NotNil(t, p)
		assert.Equal(t, sqlc.SharedDepreciationMethodStraightLine, p.Method, "buildings are always straight-line fiscally (fallback, not skip)")
		assert.Equal(t, int32(240), p.LifeMonths)
		assert.False(t, p.FinalAbsorb)
	})

	t.Run("salvage_always_zero", func(t *testing.T) {
		a := baseAsset()
		a.FiscalGroup = ptr(sqlc.SharedFiscalAssetGroupKelompok3)
		p, skip := ResolveFiscal(a, baseCategory())
		require.Nil(t, skip)
		require.NotNil(t, p)
		assert.Equal(t, "0", p.Salvage)
	})

	t.Run("life_from_table_not_asset", func(t *testing.T) {
		a := baseAsset()
		a.FiscalGroup = ptr(sqlc.SharedFiscalAssetGroupKelompok2)
		a.FiscalLifeMonths = ptr(int32(999)) // must be ignored in favor of the table
		p, skip := ResolveFiscal(a, baseCategory())
		require.Nil(t, skip)
		require.NotNil(t, p)
		assert.Equal(t, int32(96), p.LifeMonths)
	})

	t.Run("method_follows_commercial_when_valid", func(t *testing.T) {
		a := baseAsset()
		a.FiscalGroup = ptr(sqlc.SharedFiscalAssetGroupKelompok2)
		a.DepreciationMethod = ptr(sqlc.SharedDepreciationMethodDecliningBalance)
		p, skip := ResolveFiscal(a, baseCategory())
		require.Nil(t, skip)
		require.NotNil(t, p)
		assert.Equal(t, sqlc.SharedDepreciationMethodDecliningBalance, p.Method)
		assert.True(t, p.FinalAbsorb)
	})

	t.Run("shared_guards_not_capitalized_no_cost_no_date_disposed", func(t *testing.T) {
		a := baseAsset()
		a.Capitalized = false
		_, skip := ResolveFiscal(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "not_capitalized", skip.Reason)

		a = baseAsset()
		a.PurchaseCost = nil
		_, skip = ResolveFiscal(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "no_cost", skip.Reason)

		a = baseAsset()
		a.PurchaseDate = pgtype.Date{}
		_, skip = ResolveFiscal(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "no_purchase_date", skip.Reason)

		a = baseAsset()
		a.Status = sqlc.SharedAssetStatusDisposed
		_, skip = ResolveFiscal(a, baseCategory())
		require.NotNil(t, skip)
		assert.Equal(t, "disposed", skip.Reason)
	})
}
