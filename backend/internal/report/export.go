// export.go builds the downloadable renderings (xlsx via excelize, PDF via
// fpdf) of the three report-family payloads: the generic per-type report
// (ReportResult), the dashboard (DashboardSummary), and the disposal GL recap
// (GlRecapResult). Mirrors internal/depreciation/export.go's pattern (no Gin
// here — ADR-0008; these are pure builders, the handler sets HTTP headers)
// and internal/stockopname/report.go's two-sheet xlsx pattern for the
// dashboard.
//
// columnsFor is the single DRY seam: it maps a ReportResult's concrete row
// type to a shared column definition (header, width, alignment, optional
// Totals key, and a per-row string accessor) consumed by both
// BuildReportXLSX and BuildReportPDF, so the two renderers can never drift
// out of sync on which columns a report type shows or in what order.
package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

// exportDefaultCompany is the fallback company-name header for the PDF when
// the `label.company_name` app setting is unset — a package-local copy of
// internal/asset/barcode.go's defaultCompanyName (ADR-0008 module
// boundaries: no reaching into another module's internals for one constant).
const exportDefaultCompany = "PT Bank Tabungan Negara (Persero) Tbk"

// ExportMeta carries the display metadata common to every export (xlsx sheet
// header isn't affected, but every PDF prints Title as the document title,
// "PeriodLabel · OfficeLabel" as the subtitle, and a footer crediting
// PrintedBy/PrintedAt). The handler resolves these from the request +
// caller before calling a Build* function.
type ExportMeta struct {
	Title       string
	PeriodLabel string
	OfficeLabel string
	PrintedBy   string
	PrintedAt   time.Time
}

// exportFilename builds the shared filename base (sans extension), e.g.
// "laporan-assets-2026-06-12--2026-07-11".
func exportFilename(kind string, cur DateRange) string {
	return fmt.Sprintf("laporan-%s-%s--%s", kind, cur.From.Format("2006-01-02"), cur.To.Format("2006-01-02"))
}

// companyName resolves the PDF header's company name from the
// `label.company_name` app setting, tolerating it being unset (falls back to
// exportDefaultCompany) exactly like depreciation.BuildJournalPDF.
//
// s.q == nil is tolerated the same way: NewService(nil, nil) is otherwise a
// valid *Service for unit-testing the pure column-mapping logic in this
// file, and s.q.GetAppSetting would nil-pointer-dereference on a nil
// *sqlc.Queries. This is a test-enablement guard, not a production path —
// the router always wires a real *sqlc.Queries.
func (s *Service) companyName(ctx context.Context) (string, error) {
	if s.q == nil {
		return exportDefaultCompany, nil
	}
	v, err := s.q.GetAppSetting(ctx, "label.company_name")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exportDefaultCompany, nil
		}
		return "", err
	}
	if v == "" {
		return exportDefaultCompany, nil
	}
	return v, nil
}

// ── Column mapping (DRY seam shared by xlsx + pdf) ──────────────────────────

// column is one table column shared by BuildReportXLSX and BuildReportPDF:
// Header/Width/Align describe presentation, Value renders row i, and
// TotalsKey (when non-empty) looks up the TOTAL-row figure in
// ReportResult.Totals.
type column struct {
	Header    string
	Width     float64
	Align     string // "L" | "R" | "C" (fpdf CellFormat alignment)
	TotalsKey string
	Value     func(i int) string
}

// columnsFor maps a ReportResult's concrete Rows slice to its column
// definitions plus the row count. The column set and Indonesian headers
// mirror the report-table spec (assets: Kode, Nama Aset, Kategori, Harga
// Beli, Akum. Penyusutan, Nilai Buku, …); the row's Status field is
// intentionally omitted from the assets export columns to match that spec.
func columnsFor(res ReportResult) ([]column, int, error) {
	switch rows := res.Rows.(type) {
	case []AssetRow:
		return []column{
			{"Kode", 22, "L", "", func(i int) string { return rows[i].Tag }},
			{"Nama Aset", 50, "L", "", func(i int) string { return rows[i].Name }},
			{"Kategori", 30, "L", "", func(i int) string { return rows[i].Category }},
			{"Harga Beli", 30, "R", "purchase_cost", func(i int) string { return rows[i].PurchaseCost }},
			{"Akum. Penyusutan", 32, "R", "accum_deprec", func(i int) string { return rows[i].AccumDeprec }},
			{"Nilai Buku", 30, "R", "book_value", func(i int) string { return rows[i].BookValue }},
		}, len(rows), nil

	case []DeprRow:
		return []column{
			{"Periode", 30, "L", "", func(i int) string { return rows[i].Period }},
			{"Saldo Awal", 40, "R", "opening", func(i int) string { return rows[i].Opening }},
			{"Beban Penyusutan", 40, "R", "amount", func(i int) string { return rows[i].Amount }},
			{"Saldo Akhir", 40, "R", "closing", func(i int) string { return rows[i].Closing }},
		}, len(rows), nil

	case []UtilRow:
		return []column{
			{"Nama Aset", 45, "L", "", func(i int) string { return rows[i].Name }},
			{"Kode", 22, "L", "", func(i int) string { return rows[i].Tag }},
			{"Kategori", 28, "L", "", func(i int) string { return rows[i].Category }},
			{"Hari Dipinjam", 26, "R", "days_loaned", func(i int) string { return strconv.FormatInt(rows[i].DaysLoaned, 10) }},
			{"Jumlah Peminjaman", 30, "R", "loan_count", func(i int) string { return strconv.FormatInt(rows[i].LoanCount, 10) }},
			{"Utilisasi", 22, "R", "", func(i int) string { return strconv.FormatFloat(rows[i].UtilizationPct, 'f', 1, 64) + "%" }},
		}, len(rows), nil

	case []MaintRow:
		return []column{
			{"Nama Aset", 45, "L", "", func(i int) string { return rows[i].AssetName }},
			{"Kategori", 28, "L", "", func(i int) string { return rows[i].Category }},
			{"Jenis", 22, "L", "", func(i int) string { return rows[i].Type }},
			{"Jumlah Tindakan", 28, "R", "actions", func(i int) string { return strconv.FormatInt(rows[i].Actions, 10) }},
			{"Total Biaya", 32, "R", "total_cost", func(i int) string { return rows[i].TotalCost }},
		}, len(rows), nil

	case []TransferRow:
		return []column{
			{"Nama Aset", 40, "L", "", func(i int) string { return rows[i].AssetName }},
			{"Kode", 20, "L", "", func(i int) string { return rows[i].AssetTag }},
			{"Dari", 30, "L", "", func(i int) string { return rows[i].FromOffice }},
			{"Ke", 30, "L", "", func(i int) string { return rows[i].ToOffice }},
			{"Status", 22, "L", "", func(i int) string { return rows[i].Status }},
			{"Tgl Kirim", 22, "L", "", func(i int) string { return rows[i].ShippedDate }},
			{"Tgl Terima", 22, "L", "", func(i int) string { return rows[i].ReceivedDate }},
			{"No. BAST", 24, "L", "", func(i int) string { return rows[i].BastNo }},
		}, len(rows), nil

	case []DisposalRow:
		return []column{
			{"Nama Aset", 38, "L", "", func(i int) string { return rows[i].AssetName }},
			{"Kode", 20, "L", "", func(i int) string { return rows[i].AssetTag }},
			{"Metode", 24, "L", "", func(i int) string { return rows[i].Method }},
			{"Tanggal", 22, "L", "", func(i int) string { return rows[i].Date }},
			{"Nilai Buku", 26, "R", "book_value", func(i int) string { return rows[i].BookValue }},
			{"Hasil Pelepasan", 28, "R", "proceeds", func(i int) string { return rows[i].Proceeds }},
			{"Laba/Rugi", 26, "R", "gain_loss", func(i int) string { return rows[i].GainLoss }},
		}, len(rows), nil

	case []OpnameRow:
		return []column{
			{"Sesi", 34, "L", "", func(i int) string { return rows[i].Name }},
			{"Kantor", 30, "L", "", func(i int) string { return rows[i].OfficeName }},
			{"Periode", 22, "L", "", func(i int) string { return rows[i].Period }},
			{"Status", 22, "L", "", func(i int) string { return rows[i].Status }},
			{"Total Item", 22, "R", "", func(i int) string { return strconv.FormatInt(rows[i].TotalItems, 10) }},
			{"Varians", 20, "R", "", func(i int) string { return strconv.FormatInt(rows[i].Variance, 10) }},
		}, len(rows), nil

	default:
		return nil, 0, fmt.Errorf("report: unsupported row type %T for export", res.Rows)
	}
}

// ── Report xlsx/pdf ──────────────────────────────────────────────────────────

// BuildReportXLSX renders a ReportResult as a single-sheet xlsx workbook:
// header row (columnsFor(res)), one data row per res.Rows entry, and a
// trailing TOTAL row populated from res.Totals for the money columns.
func BuildReportXLSX(res ReportResult, meta ExportMeta) ([]byte, error) {
	cols, n, err := columnsFor(res)
	if err != nil {
		return nil, err
	}

	const sheet = "Laporan"
	f := excelize.NewFile()
	defer f.Close() //nolint:errcheck

	if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
		return nil, err
	}

	for i, c := range cols {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, c.Header); err != nil {
			return nil, err
		}
	}

	for r := 0; r < n; r++ {
		for i, c := range cols {
			cell, err := excelize.CoordinatesToCellName(i+1, r+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, c.Value(r)); err != nil {
				return nil, err
			}
		}
	}

	totalRow := n + 2
	labelCell, err := excelize.CoordinatesToCellName(1, totalRow)
	if err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, labelCell, "TOTAL"); err != nil {
		return nil, err
	}
	for i, c := range cols {
		if c.TotalsKey == "" {
			continue
		}
		v, ok := res.Totals[c.TotalsKey]
		if !ok {
			continue
		}
		cell, err := excelize.CoordinatesToCellName(i+1, totalRow)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return nil, err
		}
	}

	for i, c := range cols {
		colName, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetColWidth(sheet, colName, colName, c.Width); err != nil {
			return nil, err
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildReportPDF renders a ReportResult as a landscape A4 PDF (landscape
// rather than depreciation's portrait: report tables run up to 8 columns,
// e.g. transfers, and need the extra width): company header, title +
// "PeriodLabel · OfficeLabel" subtitle, a bordered table (columnsFor(res)),
// a TOTAL row when res.Totals is non-empty, and an italic footer crediting
// who printed it and when.
func (s *Service) BuildReportPDF(ctx context.Context, res ReportResult, meta ExportMeta) ([]byte, error) {
	cols, n, err := columnsFor(res)
	if err != nil {
		return nil, err
	}
	company, err := s.companyName(ctx)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, company, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 6, meta.Title, "", 1, "C", false, 0, "")
	subtitle := fmt.Sprintf("%s · %s", meta.PeriodLabel, meta.OfficeLabel)
	pdf.CellFormat(0, 6, subtitle, "", 1, "C", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 9)
	for _, c := range cols {
		pdf.CellFormat(c.Width, 7, c.Header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	for i := 0; i < n; i++ {
		for _, c := range cols {
			pdf.CellFormat(c.Width, 7, c.Value(i), "1", 0, c.Align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	if len(res.Totals) > 0 {
		pdf.SetFont("Helvetica", "B", 9)
		for i, c := range cols {
			if i == 0 {
				pdf.CellFormat(c.Width, 7, "TOTAL", "1", 0, "R", false, 0, "")
				continue
			}
			v := ""
			if c.TotalsKey != "" {
				v = res.Totals[c.TotalsKey]
			}
			pdf.CellFormat(c.Width, 7, v, "1", 0, c.Align, false, 0, "")
		}
		pdf.Ln(10)
	} else {
		pdf.Ln(6)
	}

	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(0, 5, printedFooter(meta), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// printedFooter formats the italic "Dicetak oleh …" footer line shared by
// every PDF export.
func printedFooter(meta ExportMeta) string {
	return fmt.Sprintf("Dicetak oleh %s · %s", meta.PrintedBy, meta.PrintedAt.Format("2006-01-02 15:04"))
}

// ── Dashboard xlsx/pdf ───────────────────────────────────────────────────────

// dashboardKpiRows formats the 6 dashboard KPIs + excluded-asset count as
// Indonesian label/value pairs, shared by BuildDashboardXLSX (Ringkasan
// sheet) and BuildDashboardPDF (summary block).
func dashboardKpiRows(sum DashboardSummary) [][2]string {
	return [][2]string{
		{"Total Aset", strconv.FormatInt(sum.Kpi.TotalAssets, 10)},
		{"Nilai Perolehan", sum.Kpi.AcquisitionValue},
		{"Nilai Buku", sum.Kpi.BookValue},
		{"Aset Overdue", strconv.FormatInt(sum.Kpi.OverdueAssets, 10)},
		{"Maintenance Jatuh Tempo", strconv.FormatInt(sum.Kpi.MaintenanceDue, 10)},
		{"Biaya Maintenance", sum.Kpi.MaintenanceCost},
		{"Aset Dikecualikan", strconv.FormatInt(sum.ExcludedCount, 10)},
	}
}

// namedCountLabel renders a NamedCount's nullable Name for display ("-" for
// the nil/"no room" bucket).
func namedCountLabel(n NamedCount) string {
	if n.Name == nil {
		return "-"
	}
	return *n.Name
}

// locationBreakdownLabel returns the Indonesian header for the by-location
// breakdown, keyed off DashboardSummary.LocationKind.
func locationBreakdownLabel(kind string) string {
	if kind == "room" {
		return "Ruangan"
	}
	return "Kantor"
}

// BuildDashboardXLSX renders a DashboardSummary as a two-sheet xlsx
// workbook: "Ringkasan" (the 6 KPIs + excluded count as label/value pairs)
// and "Rincian" (by_status / by_category / by_location breakdowns stacked,
// each with its own Label|Jumlah header).
func BuildDashboardXLSX(sum DashboardSummary, meta ExportMeta) ([]byte, error) {
	const sheetRingkasan = "Ringkasan"
	const sheetRincian = "Rincian"

	f := excelize.NewFile()
	defer f.Close() //nolint:errcheck

	if err := f.SetSheetName(f.GetSheetName(0), sheetRingkasan); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet(sheetRincian); err != nil {
		return nil, err
	}

	for i, r := range dashboardKpiRows(sum) {
		row := i + 1
		if err := f.SetCellValue(sheetRingkasan, fmt.Sprintf("A%d", row), r[0]); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetRingkasan, fmt.Sprintf("B%d", row), r[1]); err != nil {
			return nil, err
		}
	}
	if err := f.SetColWidth(sheetRingkasan, "A", "A", 26); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheetRingkasan, "B", "B", 22); err != nil {
		return nil, err
	}

	writeBreakdown := func(row int, label string, rows [][2]string) (int, error) {
		if err := f.SetCellValue(sheetRincian, fmt.Sprintf("A%d", row), label); err != nil {
			return 0, err
		}
		if err := f.SetCellValue(sheetRincian, fmt.Sprintf("B%d", row), "Jumlah"); err != nil {
			return 0, err
		}
		row++
		for _, r := range rows {
			if err := f.SetCellValue(sheetRincian, fmt.Sprintf("A%d", row), r[0]); err != nil {
				return 0, err
			}
			if err := f.SetCellValue(sheetRincian, fmt.Sprintf("B%d", row), r[1]); err != nil {
				return 0, err
			}
			row++
		}
		return row + 1, nil // blank separator row
	}

	statusRows := make([][2]string, 0, len(sum.ByStatus))
	for _, st := range sum.ByStatus {
		statusRows = append(statusRows, [2]string{st.Status, strconv.FormatInt(st.Count, 10)})
	}
	catRows := make([][2]string, 0, len(sum.ByCategory))
	for _, c := range sum.ByCategory {
		catRows = append(catRows, [2]string{namedCountLabel(c), strconv.FormatInt(c.Count, 10)})
	}
	locRows := make([][2]string, 0, len(sum.ByLocation))
	for _, l := range sum.ByLocation {
		locRows = append(locRows, [2]string{namedCountLabel(l), strconv.FormatInt(l.Count, 10)})
	}

	row := 1
	var err error
	row, err = writeBreakdown(row, "Status", statusRows)
	if err != nil {
		return nil, err
	}
	row, err = writeBreakdown(row, "Kategori", catRows)
	if err != nil {
		return nil, err
	}
	if _, err = writeBreakdown(row, locationBreakdownLabel(sum.LocationKind), locRows); err != nil {
		return nil, err
	}

	if err := f.SetColWidth(sheetRincian, "A", "A", 26); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheetRincian, "B", "B", 14); err != nil {
		return nil, err
	}

	f.SetActiveSheet(0)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildDashboardPDF renders a DashboardSummary as a portrait A4 PDF: company
// header, title + "PeriodLabel · OfficeLabel" subtitle, a "Ringkasan" KPI
// block, three bordered Label|Jumlah breakdown tables (status/category/
// location), and the printed-by footer.
func (s *Service) BuildDashboardPDF(ctx context.Context, sum DashboardSummary, meta ExportMeta) ([]byte, error) {
	company, err := s.companyName(ctx)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, company, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 6, meta.Title, "", 1, "C", false, 0, "")
	subtitle := fmt.Sprintf("%s · %s", meta.PeriodLabel, meta.OfficeLabel)
	pdf.CellFormat(0, 6, subtitle, "", 1, "C", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(0, 6, "Ringkasan", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	for _, l := range dashboardKpiRows(sum) {
		pdf.CellFormat(0, 6, fmt.Sprintf("%s: %s", l[0], l[1]), "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	renderBreakdown := func(title string, rows [][2]string) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(0, 6, title, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(120, 6, "Label", "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, "Jumlah", "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
		pdf.SetFont("Helvetica", "", 9)
		for _, r := range rows {
			pdf.CellFormat(120, 6, r[0], "1", 0, "L", false, 0, "")
			pdf.CellFormat(40, 6, r[1], "1", 0, "R", false, 0, "")
			pdf.Ln(-1)
		}
		pdf.Ln(3)
	}

	statusRows := make([][2]string, 0, len(sum.ByStatus))
	for _, st := range sum.ByStatus {
		statusRows = append(statusRows, [2]string{st.Status, strconv.FormatInt(st.Count, 10)})
	}
	renderBreakdown("Berdasarkan Status", statusRows)

	catRows := make([][2]string, 0, len(sum.ByCategory))
	for _, c := range sum.ByCategory {
		catRows = append(catRows, [2]string{namedCountLabel(c), strconv.FormatInt(c.Count, 10)})
	}
	renderBreakdown("Berdasarkan Kategori", catRows)

	locRows := make([][2]string, 0, len(sum.ByLocation))
	for _, l := range sum.ByLocation {
		locRows = append(locRows, [2]string{namedCountLabel(l), strconv.FormatInt(l.Count, 10)})
	}
	renderBreakdown("Berdasarkan "+locationBreakdownLabel(sum.LocationKind), locRows)

	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(0, 5, printedFooter(meta), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── GL recap xlsx/pdf ────────────────────────────────────────────────────────

// BuildGlRecapXLSX renders a GlRecapResult as a single-sheet xlsx workbook:
// header row (Kode Akun|Nama Akun|Debit|Kredit), one data row per GL line,
// and a trailing TOTAL row — the same layout as
// depreciation.BuildJournalXLSX, since a GL recap is journal-shaped.
func BuildGlRecapXLSX(r GlRecapResult, meta ExportMeta) ([]byte, error) {
	const sheet = "Rekap GL"

	f := excelize.NewFile()
	defer f.Close() //nolint:errcheck

	if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
		return nil, err
	}

	headers := []string{"Kode Akun", "Nama Akun", "Debit", "Kredit"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}

	row := 2
	for _, gr := range r.Rows {
		if err := f.SetCellValue(sheet, fmt.Sprintf("A%d", row), gr.AccountCode); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("B%d", row), gr.AccountName); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("C%d", row), gr.Debit); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("D%d", row), gr.Credit); err != nil {
			return nil, err
		}
		row++
	}

	if err := f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "TOTAL"); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.TotalDebit); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.TotalCredit); err != nil {
		return nil, err
	}

	widths := map[string]float64{"A": 16, "B": 42, "C": 18, "D": 18}
	for col, w := range widths {
		if err := f.SetColWidth(sheet, col, col, w); err != nil {
			return nil, err
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildGlRecapPDF renders a GlRecapResult as a one-page A4 PDF — layout
// verbatim from depreciation.BuildJournalPDF (company header, title +
// subtitle, bordered Kode Akun|Nama Akun|Debit|Kredit table, TOTAL row) plus
// the shared printed-by footer.
func (s *Service) BuildGlRecapPDF(ctx context.Context, r GlRecapResult, meta ExportMeta) ([]byte, error) {
	company, err := s.companyName(ctx)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, company, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 6, meta.Title, "", 1, "C", false, 0, "")
	subtitle := fmt.Sprintf("%s · %s", meta.PeriodLabel, meta.OfficeLabel)
	pdf.CellFormat(0, 6, subtitle, "", 1, "C", false, 0, "")
	pdf.Ln(4)

	widths := []float64{30, 90, 35, 35}
	headers := []string{"Kode Akun", "Nama Akun", "Debit", "Kredit"}
	pdf.SetFont("Helvetica", "B", 10)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 10)
	for _, gr := range r.Rows {
		pdf.CellFormat(widths[0], 7, gr.AccountCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, gr.AccountName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, gr.Debit, "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[3], 7, gr.Credit, "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(widths[0]+widths[1], 7, "TOTAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[2], 7, r.TotalDebit, "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[3], 7, r.TotalCredit, "1", 0, "R", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont("Helvetica", "I", 9)
	pdf.MultiCell(0, 5, printedFooter(meta), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
