// export.go builds the two downloadable renderings of a computed journal
// (Journal in service.go): an xlsx workbook (excelize) and a PDF (fpdf,
// mirroring internal/asset/barcode.go's label-PDF pattern). No Gin here
// (ADR-0008) — these are pure builders over a JournalResult; the handler
// (handler.go) calls Journal() then one of these, and sets the HTTP
// response headers/body.
package depreciation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/pdfutil"
)

// ErrInvalidExportFormat is returned when the `format` query param on the
// journal export endpoint is neither "xlsx" nor "pdf".
var ErrInvalidExportFormat = errors.New("depreciation: invalid export format")

// journalExportDefaultCompany is the fallback company name for the PDF header
// when the `label.company_name` app setting is unset — a package-local copy
// of internal/asset/barcode.go's defaultCompanyName (ADR-0008 module
// boundaries: no reaching into another module's internals for one constant).
const journalExportDefaultCompany = "PT Bank Tabungan Negara (Persero) Tbk"

// parseExportFormat parses the `format` query param, rejecting anything other
// than the two supported renderings.
func parseExportFormat(raw string) (string, error) {
	switch raw {
	case "xlsx", "pdf":
		return raw, nil
	default:
		return "", ErrInvalidExportFormat
	}
}

// basisLabel is the Indonesian display label for a depreciation basis, used
// in the PDF subtitle.
func basisLabel(basis sqlc.SharedDepreciationBasis) string {
	if basis == sqlc.SharedDepreciationBasisFiscal {
		return "Fiskal"
	}
	return "Komersial"
}

// journalExportFilename builds the shared filename base (sans extension) for
// both export formats: "jurnal-penyusutan-<period>-<basis>".
func journalExportFilename(period time.Time, basis sqlc.SharedDepreciationBasis) string {
	return fmt.Sprintf("jurnal-penyusutan-%s-%s", period.Format(periodLayout), string(basis))
}

// BuildJournalXLSX renders a JournalResult as an xlsx workbook: one sheet
// ("Jurnal Penyusutan") with a header row (Kode Akun|Nama Akun|Debit|Kredit),
// one data row per journal row, and a trailing TOTAL row.
func BuildJournalXLSX(result JournalResult) ([]byte, error) {
	const sheet = "Jurnal Penyusutan"

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
	for _, r := range result.Rows {
		if err := f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.AccountCode); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.AccountName); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.Debit); err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Credit); err != nil {
			return nil, err
		}
		row++
	}

	if err := f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "TOTAL"); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, fmt.Sprintf("C%d", row), result.TotalDebit); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, fmt.Sprintf("D%d", row), result.TotalCredit); err != nil {
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

// BuildJournalPDF renders a JournalResult as a one-page A4 PDF: company-name
// header (from the `label.company_name` app setting, tolerant of it being
// unset), a title + basis/period subtitle, the journal table, a TOTAL row,
// and a footer note that the journal balances by construction.
func (s *Service) BuildJournalPDF(ctx context.Context, period time.Time, basis sqlc.SharedDepreciationBasis, result JournalResult) ([]byte, error) {
	company := journalExportDefaultCompany
	if v, err := s.q.GetAppSetting(ctx, "label.company_name"); err == nil {
		if v != "" {
			company = v
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	pdf := pdfutil.NewUTF8PDF("P", "mm", "A4")
	pdf.AddPage()

	pdf.SetFont(pdfutil.FontFamily, "B", 14)
	pdf.CellFormat(0, 8, company, "", 1, "C", false, 0, "")

	pdf.SetFont(pdfutil.FontFamily, "", 11)
	pdf.CellFormat(0, 6, "Jurnal Penyusutan", "", 1, "C", false, 0, "")
	subtitle := fmt.Sprintf("%s · %s", basisLabel(basis), period.Format(periodLayout))
	pdf.CellFormat(0, 6, subtitle, "", 1, "C", false, 0, "")
	pdf.Ln(4)

	widths := []float64{30, 90, 35, 35}
	headers := []string{"Kode Akun", "Nama Akun", "Debit", "Kredit"}
	pdf.SetFont(pdfutil.FontFamily, "B", 10)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont(pdfutil.FontFamily, "", 10)
	for _, r := range result.Rows {
		pdf.CellFormat(widths[0], 7, r.AccountCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, r.AccountName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, r.Debit, "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[3], 7, r.Credit, "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	pdf.SetFont(pdfutil.FontFamily, "B", 10)
	pdf.CellFormat(widths[0]+widths[1], 7, "TOTAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[2], 7, result.TotalDebit, "1", 0, "R", false, 0, "")
	pdf.CellFormat(widths[3], 7, result.TotalCredit, "1", 0, "R", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont(pdfutil.FontFamily, "I", 9)
	pdf.MultiCell(0, 5, "Jurnal seimbang — debit = kredit.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
