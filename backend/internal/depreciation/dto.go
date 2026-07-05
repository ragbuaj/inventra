// dto.go holds request parsing and response serialization for the
// depreciation HTTP layer. No business logic here (ADR-0008) — that lives in
// service.go; this file only shapes what crosses the wire.
package depreciation

import (
	"fmt"

	"github.com/gin-gonic/gin"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// periodLayout is the wire format for a period ("YYYY-MM") across every
// depreciation endpoint (path param, query param, and response field).
const periodLayout = "2006-01"

// parseBasis parses the `basis` query param, defaulting to "commercial" when
// absent (empty string) and rejecting anything other than the two known
// bases.
func parseBasis(raw string) (sqlc.SharedDepreciationBasis, error) {
	switch raw {
	case "":
		return sqlc.SharedDepreciationBasisCommercial, nil
	case string(sqlc.SharedDepreciationBasisCommercial):
		return sqlc.SharedDepreciationBasisCommercial, nil
	case string(sqlc.SharedDepreciationBasisFiscal):
		return sqlc.SharedDepreciationBasisFiscal, nil
	default:
		return "", fmt.Errorf("invalid basis %q", raw)
	}
}

// periodInfoToMap serializes one Periods() row.
func periodInfoToMap(p PeriodInfo) gin.H {
	return gin.H{
		"period":        p.Period.Format(periodLayout),
		"status":        p.Status,
		"asset_count":   p.AssetCount,
		"total_amount":  p.TotalAmount,
		"skipped_count": p.SkippedCount,
	}
}

// scheduleToMap serializes a ScheduleResult.
func scheduleToMap(r ScheduleResult) gin.H {
	rows := make([]gin.H, 0, len(r.Rows))
	for _, row := range r.Rows {
		rows = append(rows, gin.H{
			"asset_id":          row.AssetID,
			"asset_name":        row.AssetName,
			"asset_tag":         row.AssetTag,
			"category_name":     row.CategoryName,
			"office_name":       row.OfficeName,
			"method":            string(row.Method),
			"life_months":       row.LifeMonths,
			"opening":           row.Opening,
			"amount":            row.Amount,
			"accumulated":       row.Accumulated,
			"closing":           row.Closing,
			"impaired":          row.Impaired,
			"fully_depreciated": row.FullyDepreciated,
		})
	}
	return gin.H{
		"kpi": gin.H{
			"total_cost":        r.KPI.TotalCost,
			"total_accumulated": r.KPI.TotalAccumulated,
			"total_book_value":  r.KPI.TotalBookValue,
			"period_expense":    r.KPI.PeriodExpense,
		},
		"rows": rows,
		"totals": gin.H{
			"opening":     r.Totals.Opening,
			"amount":      r.Totals.Amount,
			"accumulated": r.Totals.Accumulated,
			"closing":     r.Totals.Closing,
		},
	}
}

// journalToMap serializes a JournalResult.
func journalToMap(r JournalResult) gin.H {
	rows := make([]gin.H, 0, len(r.Rows))
	for _, row := range r.Rows {
		rows = append(rows, gin.H{
			"account_code": row.AccountCode,
			"account_name": row.AccountName,
			"debit":        row.Debit,
			"credit":       row.Credit,
		})
	}
	return gin.H{
		"rows":         rows,
		"total_debit":  r.TotalDebit,
		"total_credit": r.TotalCredit,
		"balanced":     r.Balanced,
	}
}

// assetScheduleToMap serializes an AssetScheduleResult (unmasked path).
func assetScheduleToMap(r AssetScheduleResult) gin.H {
	entries := make([]gin.H, 0, len(r.Entries))
	for _, e := range r.Entries {
		entries = append(entries, gin.H{
			"basis":   string(e.Basis),
			"period":  e.Period.Format(periodLayout),
			"opening": e.Opening,
			"amount":  e.Amount,
			"closing": e.Closing,
			"method":  string(e.Method),
		})
	}
	return gin.H{
		"masked":              false,
		"computed_book_value": r.ComputedBookValue,
		"entries":             entries,
	}
}

// maskedAssetScheduleMap is the response when the caller's role is denied
// view on the "assets" entity's book_value field.
func maskedAssetScheduleMap() gin.H {
	return gin.H{
		"masked":              true,
		"computed_book_value": nil,
		"entries":             []gin.H{},
	}
}
