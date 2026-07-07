// report.go builds the Berita Acara (stock-opname report) renderings: an xlsx
// workbook (excelize) and a PDF (fpdf), mirroring
// internal/depreciation/export.go's pattern. No Gin here (ADR-0008) — these
// are pure builders over a ReportData; the handler (added in a later task)
// calls Service.ReportData then one of these, and sets the HTTP response
// headers/body.
package stockopname

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// reportDefaultCompany is the fallback company-name header for the PDF when
// the caller doesn't override it via ReportData — a package-local copy of
// internal/asset/barcode.go's defaultCompanyName (ADR-0008 module
// boundaries: no reaching into another module's internals for one constant).
const reportDefaultCompany = "PT Bank Tabungan Negara (Persero) Tbk"

// KpiCounts mirrors sqlc.SessionKpisRow's shape for the Berita Acara's KPI
// summary block.
type KpiCounts struct {
	Total    int64
	Found    int64
	Pending  int64
	Variance int64
}

// ReportItem is one row in the Berita Acara's item table/sheet.
type ReportItem struct {
	AssetName string
	AssetTag  string
	Result    string
	Note      string
}

// ReportData is the pure input to RenderPDF/RenderXLSX: everything the
// Berita Acara needs, already resolved to display strings by the service
// layer (Service.ReportData) so this file stays DB-free.
type ReportData struct {
	SessionName  string
	OfficeName   string
	Period       string
	ClosedByName string
	Kpi          KpiCounts
	Items        []ReportItem
}

// resultLabel is the Indonesian display label for an opname item result.
func resultLabel(result string) string {
	switch result {
	case "found":
		return "Ditemukan"
	case "pending":
		return "Belum Dihitung"
	case "not_found":
		return "Tidak Ditemukan"
	case "damaged":
		return "Rusak"
	case "misplaced":
		return "Salah Tempat"
	default:
		return result
	}
}

// RenderPDF renders a ReportData as a one-page-plus A4 Berita Acara PDF:
// company-name header, title, session/office/period/closed-by lines, a KPI
// summary block, an item table (asset name, tag, result, note), and a
// signatory line.
func RenderPDF(d ReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, reportDefaultCompany, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 7, "BERITA ACARA STOCK OPNAME", "", 1, "C", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Sesi: %s", d.SessionName), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Kantor: %s", d.OfficeName), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Periode: %s", d.Period), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Ditutup oleh: %s", d.ClosedByName), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(0, 6, "Ringkasan", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Aset: %d", d.Kpi.Total), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Ditemukan: %d", d.Kpi.Found), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Belum Dihitung: %d", d.Kpi.Pending), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Selisih/Varians: %d", d.Kpi.Variance), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	widths := []float64{55, 45, 30, 50}
	headers := []string{"Nama Aset", "Kode Aset", "Hasil", "Catatan"}
	pdf.SetFont("Helvetica", "B", 10)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	for _, it := range d.Items {
		pdf.CellFormat(widths[0], 7, it.AssetName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, it.AssetTag, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, resultLabel(it.Result), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 7, it.Note, "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.Ln(12)

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Yang bertanda tangan di bawah ini menyatakan bahwa hasil stock opname di atas telah", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "diperiksa dan sesuai dengan kondisi fisik aset pada tanggal penutupan sesi.", "", 1, "L", false, 0, "")
	pdf.Ln(14)
	pdf.CellFormat(0, 6, fmt.Sprintf("( %s )", d.ClosedByName), "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderXLSX renders a ReportData as an xlsx workbook: a "Ringkasan" summary
// sheet (session/office/period/closed-by + KPI rows) and an "Item" sheet
// (Aset|Kode|Hasil|Catatan, one row per item).
func RenderXLSX(d ReportData) ([]byte, error) {
	const sheetRingkasan = "Ringkasan"
	const sheetItem = "Item"

	f := excelize.NewFile()
	defer f.Close() //nolint:errcheck

	if err := f.SetSheetName(f.GetSheetName(0), sheetRingkasan); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet(sheetItem); err != nil {
		return nil, err
	}

	ringkasanRows := [][2]string{
		{"Sesi", d.SessionName},
		{"Kantor", d.OfficeName},
		{"Periode", d.Period},
		{"Ditutup oleh", d.ClosedByName},
		{"Total Aset", fmt.Sprintf("%d", d.Kpi.Total)},
		{"Ditemukan", fmt.Sprintf("%d", d.Kpi.Found)},
		{"Belum Dihitung", fmt.Sprintf("%d", d.Kpi.Pending)},
		{"Selisih/Varians", fmt.Sprintf("%d", d.Kpi.Variance)},
	}
	for i, r := range ringkasanRows {
		row := i + 1
		if err := f.SetCellValue(sheetRingkasan, fmt.Sprintf("A%d", row), r[0]); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetRingkasan, fmt.Sprintf("B%d", row), r[1]); err != nil {
			return nil, err
		}
	}
	if err := f.SetColWidth(sheetRingkasan, "A", "A", 20); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheetRingkasan, "B", "B", 32); err != nil {
		return nil, err
	}

	headers := []string{"Aset", "Kode", "Hasil", "Catatan"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetItem, cell, h); err != nil {
			return nil, err
		}
	}
	for i, it := range d.Items {
		row := i + 2
		if err := f.SetCellValue(sheetItem, fmt.Sprintf("A%d", row), it.AssetName); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetItem, fmt.Sprintf("B%d", row), it.AssetTag); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetItem, fmt.Sprintf("C%d", row), resultLabel(it.Result)); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetItem, fmt.Sprintf("D%d", row), it.Note); err != nil {
			return nil, err
		}
	}
	widths := map[string]float64{"A": 30, "B": 24, "C": 16, "D": 32}
	for col, w := range widths {
		if err := f.SetColWidth(sheetItem, col, col, w); err != nil {
			return nil, err
		}
	}

	f.SetActiveSheet(0)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReportData assembles a ReportData for the given session: GetSession
// enforces scope + existence and supplies office/closed-by names + KPIs,
// then ListOpnameItemsEnriched supplies the item rows. Pure DB assembly — no
// Gin, mirroring the rest of this service.
func (s *Service) ReportData(ctx context.Context, caller approval.Caller, sessionID uuid.UUID) (ReportData, error) {
	sess, kpi, err := s.GetSession(ctx, caller, sessionID)
	if err != nil {
		return ReportData{}, err
	}

	officeName := ""
	if sess.OfficeName != nil {
		officeName = *sess.OfficeName
	}
	closedByName := ""
	if sess.ClosedByName != nil {
		closedByName = *sess.ClosedByName
	}
	sessionName := ""
	if sess.StockopnameStockOpnameSession.Name != nil {
		sessionName = *sess.StockopnameStockOpnameSession.Name
	}
	period := ""
	if sess.StockopnameStockOpnameSession.Period.Valid {
		period = sess.StockopnameStockOpnameSession.Period.Time.Format("2006-01")
	}

	rows, err := s.q.ListOpnameItemsEnriched(ctx, sqlc.ListOpnameItemsEnrichedParams{SessionID: sessionID})
	if err != nil {
		return ReportData{}, mapDBError(err)
	}

	items := make([]ReportItem, 0, len(rows))
	for _, r := range rows {
		assetName := ""
		if r.AssetName != nil {
			assetName = *r.AssetName
		}
		assetTag := ""
		if r.AssetTag != nil {
			assetTag = *r.AssetTag
		}
		note := ""
		if r.StockopnameStockOpnameItem.Note != nil {
			note = *r.StockopnameStockOpnameItem.Note
		}
		items = append(items, ReportItem{
			AssetName: assetName,
			AssetTag:  assetTag,
			Result:    string(r.StockopnameStockOpnameItem.Result),
			Note:      note,
		})
	}

	return ReportData{
		SessionName:  sessionName,
		OfficeName:   officeName,
		Period:       period,
		ClosedByName: closedByName,
		Kpi: KpiCounts{
			Total:    kpi.Total,
			Found:    kpi.Found,
			Pending:  kpi.Pending,
			Variance: kpi.Variance,
		},
		Items: items,
	}, nil
}
