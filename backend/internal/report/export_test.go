package report

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// testMeta is a stable ExportMeta fixture for the export tests below.
var testMeta = ExportMeta{
	Title:       "Daftar Aset & Nilai Buku",
	PeriodLabel: "12 Jun 2026 - 11 Jul 2026",
	OfficeLabel: "Semua Kantor",
	PrintedBy:   "Budi Santoso",
	PrintedAt:   time.Date(2026, 7, 11, 14, 30, 0, 0, time.UTC),
}

// nilSvc is a *Service constructed with a nil *sqlc.Queries — valid for
// exercising the pure PDF-rendering logic in this file (companyName's
// s.q == nil guard falls back to exportDefaultCompany instead of touching a
// real DB connection).
func nilSvc() *Service { return NewService(nil, nil) }

// assertHeaderRow asserts the sheet's full header row equals headers — every
// column label in order, and no extra column beyond the expected set — so a
// swapped, mislabeled, or added header anywhere fails the test.
func assertHeaderRow(t *testing.T, f *excelize.File, sheet string, headers []string) {
	t.Helper()
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		require.NoError(t, err)
		v, err := f.GetCellValue(sheet, cell)
		require.NoError(t, err)
		assert.Equal(t, h, v, "header col %d", i+1)
	}
	extra, err := excelize.CoordinatesToCellName(len(headers)+1, 1)
	require.NoError(t, err)
	v, err := f.GetCellValue(sheet, extra)
	require.NoError(t, err)
	assert.Equal(t, "", v, "unexpected extra header column %d", len(headers)+1)
}

// ── assets ───────────────────────────────────────────────────────────────────

func assetsFixture() ReportResult {
	return ReportResult{
		Type: "assets",
		Rows: []AssetRow{
			{Tag: "AST-1", Name: "Laptop", Category: "Elektronik", Status: "available", PurchaseCost: "15000000.00", AccumDeprec: "5000000.00", BookValue: "10000000.00"},
			{Tag: "AST-2", Name: "Meja Kerja", Category: "Furnitur", Status: "assigned", PurchaseCost: "2000000.00", AccumDeprec: "500000.00", BookValue: "1500000.00"},
		},
		Totals: map[string]string{"purchase_cost": "17000000.00", "accum_deprec": "5500000.00", "book_value": "11500000.00"},
	}
}

func TestBuildReportXLSXAssets(t *testing.T) {
	res := assetsFixture()
	body, err := BuildReportXLSX(res, ExportMeta{Title: "Daftar Aset & Nilai Buku"})
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	sheet := f.GetSheetName(0)
	assert.Equal(t, "Laporan", sheet)

	assertHeaderRow(t, f, sheet, []string{"Kode", "Nama Aset", "Kategori", "Harga Beli", "Akum. Penyusutan", "Nilai Buku"})

	v, err := f.GetCellValue(sheet, "A2")
	require.NoError(t, err)
	assert.Equal(t, "AST-1", v)
	v, err = f.GetCellValue(sheet, "D2")
	require.NoError(t, err)
	assert.Equal(t, "15000000.00", v)

	v, err = f.GetCellValue(sheet, "A3")
	require.NoError(t, err)
	assert.Equal(t, "AST-2", v)

	// last row (4) is TOTAL, with the money columns filled from res.Totals
	v, err = f.GetCellValue(sheet, "A4")
	require.NoError(t, err)
	assert.Equal(t, "TOTAL", v)
	v, err = f.GetCellValue(sheet, "D4")
	require.NoError(t, err)
	assert.Equal(t, "17000000.00", v)
	v, err = f.GetCellValue(sheet, "F4")
	require.NoError(t, err)
	assert.Equal(t, "11500000.00", v)
}

func TestBuildReportPDFAssets(t *testing.T) {
	s := nilSvc()
	body, err := s.BuildReportPDF(context.Background(), assetsFixture(), testMeta)
	require.NoError(t, err)
	assert.True(t, len(body) > 0)
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
}

// TestBuildReportPDFUsesEmbeddedUnicodeFont proves the mojibake bug is fixed:
// testMeta's subtitle interpolates "PeriodLabel · OfficeLabel" (a middle dot,
// U+00B7) which core-font Helvetica (cp1252) cannot render faithfully. The
// PDF must be built with the embedded DejaVu Unicode font instead of the
// fpdf core "Helvetica" font — assert the raw PDF bytes carry no
// "/BaseFont /Helvetica" font-object reference.
func TestBuildReportPDFUsesEmbeddedUnicodeFont(t *testing.T) {
	s := nilSvc()
	body, err := s.BuildReportPDF(context.Background(), assetsFixture(), testMeta)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
	assert.False(t, bytes.Contains(body, []byte("/BaseFont /Helvetica")), "PDF still references core Helvetica font — expected embedded Unicode font")
	assert.True(t, bytes.Contains([]byte(testMeta.PeriodLabel+" · "+testMeta.OfficeLabel), []byte("·")), "sanity: subtitle fixture must contain a middle dot")
}

// ── depreciation ─────────────────────────────────────────────────────────────

func TestBuildReportXLSXDepreciation(t *testing.T) {
	res := ReportResult{
		Type: "depreciation",
		Rows: []DeprRow{
			{Period: "2026-06", Opening: "10000000.00", Amount: "500000.00", Closing: "9500000.00"},
		},
		Totals: map[string]string{"opening": "10000000.00", "amount": "500000.00", "closing": "9500000.00"},
	}
	body, err := BuildReportXLSX(res, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Periode", "Nilai Awal", "Penyusutan", "Nilai Akhir"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "2026-06", v)
	v, _ = f.GetCellValue(sheet, "C2")
	assert.Equal(t, "500000.00", v)
	v, _ = f.GetCellValue(sheet, "A3")
	assert.Equal(t, "TOTAL", v)
	v, _ = f.GetCellValue(sheet, "D3")
	assert.Equal(t, "9500000.00", v)
}

// ── utilization ──────────────────────────────────────────────────────────────

func TestBuildReportXLSXUtilization(t *testing.T) {
	res := ReportResult{
		Type: "utilization",
		Rows: []UtilRow{
			{Name: "Proyektor", Tag: "AST-9", Category: "Elektronik", DaysLoaned: 20, LoanCount: 4, UtilizationPct: 66.7},
		},
		Totals: map[string]string{"days_loaned": "20", "loan_count": "4"},
	}
	body, err := BuildReportXLSX(res, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Nama Aset", "Kategori", "Hari Dipinjam", "Jml Peminjaman", "Utilisasi"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "Proyektor", v)
	v, _ = f.GetCellValue(sheet, "C2")
	assert.Equal(t, "20", v)
	v, _ = f.GetCellValue(sheet, "E2")
	assert.Equal(t, "66.7%", v)
	v, _ = f.GetCellValue(sheet, "A3")
	assert.Equal(t, "TOTAL", v)
	v, _ = f.GetCellValue(sheet, "D3")
	assert.Equal(t, "4", v)
}

// ── maintenance ──────────────────────────────────────────────────────────────

func TestBuildReportXLSXMaintenance(t *testing.T) {
	res := ReportResult{
		Type: "maintenance",
		Rows: []MaintRow{
			{AssetName: "AC Split", Category: "Elektronik", Type: "preventive", Actions: 3, TotalCost: "900000.00"},
		},
		Totals: map[string]string{"actions": "3", "total_cost": "900000.00"},
	}
	body, err := BuildReportXLSX(res, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Aset", "Kategori", "Tipe", "Jml Tindakan", "Total Biaya"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "AC Split", v)
	v, _ = f.GetCellValue(sheet, "C2")
	assert.Equal(t, "preventive", v)
	v, _ = f.GetCellValue(sheet, "A3")
	assert.Equal(t, "TOTAL", v)
	v, _ = f.GetCellValue(sheet, "E3")
	assert.Equal(t, "900000.00", v)
}

// ── transfers (no money totals) ─────────────────────────────────────────────

func TestBuildReportXLSXTransfers(t *testing.T) {
	res := ReportResult{
		Type: "transfers",
		Rows: []TransferRow{
			{AssetName: "Laptop", AssetTag: "AST-1", FromOffice: "KC Jakarta", ToOffice: "KC Bandung", Status: "in_transit", ShippedDate: "2026-06-15", ReceivedDate: "", BastNo: ""},
		},
		Totals: map[string]string{},
	}
	body, err := BuildReportXLSX(res, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Nama Aset", "Kode", "Dari", "Ke", "Status", "Tgl Kirim", "Tgl Terima", "No. BAST"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "Laptop", v)
	v, _ = f.GetCellValue(sheet, "F2")
	assert.Equal(t, "2026-06-15", v)
	v, _ = f.GetCellValue(sheet, "G2")
	assert.Equal(t, "", v) // nullable received date renders as ""
	// no TOTAL row: transfers has an empty Totals map
	v, _ = f.GetCellValue(sheet, "A3")
	assert.Equal(t, "", v)
}

func TestBuildReportPDFTransfers(t *testing.T) {
	s := nilSvc()
	res := ReportResult{
		Type:   "transfers",
		Rows:   []TransferRow{{AssetName: "Laptop", AssetTag: "AST-1", FromOffice: "KC Jakarta", ToOffice: "KC Bandung", Status: "received"}},
		Totals: map[string]string{},
	}
	body, err := s.BuildReportPDF(context.Background(), res, testMeta)
	require.NoError(t, err)
	assert.True(t, len(body) > 0)
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
}

// ── disposals ────────────────────────────────────────────────────────────────

func TestBuildReportXLSXDisposals(t *testing.T) {
	res := ReportResult{
		Type: "disposals",
		Rows: []DisposalRow{
			{AssetName: "Printer Lama", AssetTag: "AST-5", Method: "sale", Date: "2026-06-20", BookValue: "100000.00", Proceeds: "150000.00", GainLoss: "50000.00"},
		},
		Totals: map[string]string{"book_value": "100000.00", "proceeds": "150000.00", "gain_loss": "50000.00"},
	}
	body, err := BuildReportXLSX(res, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Nama Aset", "Kode", "Metode", "Tanggal", "Nilai Buku", "Hasil Pelepasan", "Laba/Rugi"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "Printer Lama", v)
	v, _ = f.GetCellValue(sheet, "G2")
	assert.Equal(t, "50000.00", v)
	v, _ = f.GetCellValue(sheet, "A3")
	assert.Equal(t, "TOTAL", v)
	v, _ = f.GetCellValue(sheet, "F3")
	assert.Equal(t, "150000.00", v)
}

// ── opname ───────────────────────────────────────────────────────────────────

func TestBuildReportXLSXOpname(t *testing.T) {
	res := ReportResult{
		Type: "opname",
		Rows: []OpnameRow{
			{SessionID: "sess-1", Name: "Opname Juni", OfficeName: "KC Jakarta", Period: "2026-06", Status: "closed", TotalItems: 50, Variance: 2},
		},
		Totals: map[string]string{},
	}
	body, err := BuildReportXLSX(res, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Sesi", "Kantor", "Periode", "Status", "Total Item", "Varians"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "Opname Juni", v)
	v, _ = f.GetCellValue(sheet, "E2")
	assert.Equal(t, "50", v)
	v, _ = f.GetCellValue(sheet, "F2")
	assert.Equal(t, "2", v)
	// no TOTAL row: opname has an empty Totals map
	v, _ = f.GetCellValue(sheet, "A3")
	assert.Equal(t, "", v)
}

// ── unsupported row type ─────────────────────────────────────────────────────

func TestBuildReportXLSXUnsupportedRowType(t *testing.T) {
	res := ReportResult{Type: "bogus", Rows: []struct{ X string }{{X: "y"}}}
	_, err := BuildReportXLSX(res, testMeta)
	assert.Error(t, err)
}

// ── dashboard ────────────────────────────────────────────────────────────────

func dashboardFixture() DashboardSummary {
	name := "IT & Elektronik"
	loc := "KC Jakarta"
	return DashboardSummary{
		Kpi: DashboardKpi{
			TotalAssets: 120, AcquisitionValue: "500000000.00", BookValue: "350000000.00",
			OverdueAssets: 3, MaintenanceDue: 5, MaintenanceCost: "12000000.00",
		},
		ByStatus:      []StatusCount{{Status: "available", Count: 80}, {Status: "assigned", Count: 40}},
		ByCategory:    []NamedCount{{Name: &name, Count: 60}, {Name: nil, Count: 5}},
		LocationKind:  "office",
		ByLocation:    []NamedCount{{Name: &loc, Count: 90}},
		ExcludedCount: 2,
	}
}

func TestBuildDashboardXLSX(t *testing.T) {
	sum := dashboardFixture()
	body, err := BuildDashboardXLSX(sum, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	sheets := f.GetSheetList()
	require.Len(t, sheets, 2)
	assert.Equal(t, "Ringkasan", sheets[0])
	assert.Equal(t, "Rincian", sheets[1])

	v, _ := f.GetCellValue("Ringkasan", "A1")
	assert.Equal(t, "Total Aset", v)
	v, _ = f.GetCellValue("Ringkasan", "B1")
	assert.Equal(t, "120", v)
	v, _ = f.GetCellValue("Ringkasan", "A7")
	assert.Equal(t, "Aset Dikecualikan", v)
	v, _ = f.GetCellValue("Ringkasan", "B7")
	assert.Equal(t, "2", v)

	// Rincian sheet: Status block header at row 1, first status row at row 2
	v, _ = f.GetCellValue("Rincian", "A1")
	assert.Equal(t, "Status", v)
	v, _ = f.GetCellValue("Rincian", "A2")
	assert.Equal(t, "available", v)
	v, _ = f.GetCellValue("Rincian", "B2")
	assert.Equal(t, "80", v)
}

func TestBuildDashboardPDF(t *testing.T) {
	s := nilSvc()
	body, err := s.BuildDashboardPDF(context.Background(), dashboardFixture(), testMeta)
	require.NoError(t, err)
	assert.True(t, len(body) > 0)
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
}

// ── GL recap ─────────────────────────────────────────────────────────────────

func glRecapFixture() GlRecapResult {
	return GlRecapResult{
		Rows: []GlRow{
			{AccountCode: "1101", AccountName: "Kas/Bank", Debit: "150000.00", Credit: "0.00"},
			{AccountCode: "1201", AccountName: "Nilai Buku Aset Dilepas", Debit: "0.00", Credit: "100000.00"},
			{AccountCode: "8001", AccountName: "Laba Pelepasan Aset", Debit: "0.00", Credit: "50000.00"},
		},
		TotalDebit: "150000.00", TotalCredit: "150000.00", Balanced: true,
	}
}

func TestBuildGlRecapXLSX(t *testing.T) {
	r := glRecapFixture()
	body, err := BuildGlRecapXLSX(r, testMeta)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck
	sheet := f.GetSheetName(0)

	assertHeaderRow(t, f, sheet, []string{"Kode Akun", "Nama Akun", "Debit", "Kredit"})

	v, _ := f.GetCellValue(sheet, "A2")
	assert.Equal(t, "1101", v)
	v, _ = f.GetCellValue(sheet, "C2")
	assert.Equal(t, "150000.00", v)

	// row 5 is TOTAL (3 data rows + header)
	v, _ = f.GetCellValue(sheet, "A5")
	assert.Equal(t, "TOTAL", v)
	v, _ = f.GetCellValue(sheet, "C5")
	assert.Equal(t, "150000.00", v)
	v, _ = f.GetCellValue(sheet, "D5")
	assert.Equal(t, "150000.00", v)
}

func TestBuildGlRecapPDF(t *testing.T) {
	s := nilSvc()
	body, err := s.BuildGlRecapPDF(context.Background(), glRecapFixture(), testMeta)
	require.NoError(t, err)
	assert.True(t, len(body) > 0)
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
}

// ── companyName nil-guard ────────────────────────────────────────────────────

func TestCompanyNameNilQueriesFallsBackToDefault(t *testing.T) {
	s := nilSvc()
	name, err := s.companyName(context.Background())
	require.NoError(t, err)
	assert.Equal(t, exportDefaultCompany, name)
}

// ── exportFilename ───────────────────────────────────────────────────────────

func TestExportFilename(t *testing.T) {
	cur := DateRange{From: date("2026-06-12"), To: date("2026-07-11")}
	assert.Equal(t, "laporan-assets-2026-06-12--2026-07-11", exportFilename("assets", cur))
	assert.Equal(t, "laporan-dashboard-2026-06-12--2026-07-11", exportFilename("dashboard", cur))
}
