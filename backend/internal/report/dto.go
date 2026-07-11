// Package report serves the dashboard aggregates and the report builder
// (7 report types) read-only, over the OLTP tables, office-scoped.
package report

import (
	"errors"
	"time"
)

var (
	ErrInvalidPeriod       = errors.New("report: invalid period")
	ErrInvalidReportType   = errors.New("report: invalid report type")
	ErrInvalidExportFormat = errors.New("report: invalid export format")
	ErrInvalidVariant      = errors.New("report: invalid export variant")
	ErrOfficeOutOfScope    = errors.New("report: office outside caller scope")
)

// scopeModule is the data_scope_policies module (seeded in 000029).
const scopeModule = "report"

// DateRange is an inclusive [From, To] date window (midnight-normalized).
type DateRange struct{ From, To time.Time }

func (r DateRange) Days() int { return int(r.To.Sub(r.From).Hours()/24) + 1 }

// ResolvePeriod turns either a preset or a custom from/to pair into the
// current window plus the equal-length window immediately preceding it
// (used for trend comparison). Supplying both or neither is an error.
func ResolvePeriod(preset, fromStr, toStr string, now time.Time) (DateRange, DateRange, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var cur DateRange
	custom := fromStr != "" || toStr != ""
	switch {
	case preset != "" && custom:
		return DateRange{}, DateRange{}, ErrInvalidPeriod
	case custom:
		if fromStr == "" || toStr == "" {
			return DateRange{}, DateRange{}, ErrInvalidPeriod
		}
		from, err1 := time.Parse("2006-01-02", fromStr)
		to, err2 := time.Parse("2006-01-02", toStr)
		if err1 != nil || err2 != nil || from.After(to) {
			return DateRange{}, DateRange{}, ErrInvalidPeriod
		}
		cur = DateRange{From: from, To: to}
	case preset == "last30":
		cur = DateRange{From: today.AddDate(0, 0, -29), To: today}
	case preset == "this_month":
		cur = DateRange{From: time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC), To: today}
	case preset == "this_quarter":
		qm := time.Month((int(today.Month())-1)/3*3 + 1)
		cur = DateRange{From: time.Date(today.Year(), qm, 1, 0, 0, 0, 0, time.UTC), To: today}
	case preset == "ytd":
		cur = DateRange{From: time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC), To: today}
	default:
		return DateRange{}, DateRange{}, ErrInvalidPeriod
	}
	days := cur.Days()
	prevTo := cur.From.AddDate(0, 0, -1)
	prev := DateRange{From: prevTo.AddDate(0, 0, -(days - 1)), To: prevTo}
	return cur, prev, nil
}

var reportTypes = map[string]bool{
	"assets": true, "depreciation": true, "utilization": true,
	"maintenance": true, "transfers": true, "disposals": true, "opname": true,
}

// ParseReportType validates :type against the whitelist (never used raw in SQL).
func ParseReportType(raw string) (string, error) {
	if reportTypes[raw] {
		return raw, nil
	}
	return "", ErrInvalidReportType
}

func parseExportFormat(raw string) (string, error) {
	switch raw {
	case "xlsx", "pdf":
		return raw, nil
	default:
		return "", ErrInvalidExportFormat
	}
}

// ---- Dashboard response ----

type Trends struct {
	AcquisitionPct     *float64 `json:"acquisition_pct"`
	BookValuePct       *float64 `json:"book_value_pct"`
	MaintenanceCostPct *float64 `json:"maintenance_cost_pct"`
}

type DashboardKpi struct {
	TotalAssets      int64  `json:"total_assets"`
	AcquisitionValue string `json:"acquisition_value"`
	BookValue        string `json:"book_value"`
	OverdueAssets    int64  `json:"overdue_assets"`
	MaintenanceDue   int64  `json:"maintenance_due"`
	MaintenanceCost  string `json:"maintenance_cost"`
	Trends           Trends `json:"trends"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type NamedCount struct {
	Name  *string `json:"name"` // nil = "no room" bucket; frontend localizes
	Count int64   `json:"count"`
}

type MaintenanceDueItem struct {
	ID           string  `json:"id"`
	AssetName    string  `json:"asset_name"`
	AssetTag     string  `json:"asset_tag"`
	CategoryName *string `json:"category_name"`
	NextDueDate  string  `json:"next_due_date"` // YYYY-MM-DD
}

type DashboardSummary struct {
	OfficeName         *string              `json:"office_name"`
	Kpi                DashboardKpi         `json:"kpi"`
	ByStatus           []StatusCount        `json:"by_status"`
	ByCategory         []NamedCount         `json:"by_category"`
	LocationKind       string               `json:"location_kind"` // "office" | "room"
	ByLocation         []NamedCount         `json:"by_location"`
	MaintenanceDueList []MaintenanceDueItem `json:"maintenance_due_list"`
	ExcludedCount      int64                `json:"excluded_count"`
}

// ---- Report responses (generic envelope, per-type rows) ----

type ReportKpi struct {
	Key   string `json:"key"`   // stable per-type key, e.g. "total_assets"
	Value string `json:"value"` // pre-stringified (count or decimal money string)
}

type ChartBar struct {
	Label string `json:"label"`
	Value string `json:"value"` // decimal string (money) or plain number string
}

// ReportResult is the JSON body of GET /reports/:type. Rows is a slice of
// per-type row structs (each defined in service.go next to its query);
// Totals mirrors the tfoot TOTAL row keyed by column.
type ReportResult struct {
	Type      string            `json:"type"`
	Kpis      []ReportKpi       `json:"kpis"`
	Chart     []ChartBar        `json:"chart"`
	Rows      any               `json:"rows"`
	Totals    map[string]string `json:"totals"`
	RowCount  int64             `json:"row_count"`
	Truncated bool              `json:"truncated"`
}

// ---- Disposal GL recap ----

type GlRow struct {
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
}

type GlRecapResult struct {
	Rows        []GlRow `json:"rows"`
	TotalDebit  string  `json:"total_debit"`
	TotalCredit string  `json:"total_credit"`
	Balanced    bool    `json:"balanced"`
}
