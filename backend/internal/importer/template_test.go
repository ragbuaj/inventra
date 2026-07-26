package importer

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// exampleCols mirrors a real target: each column carries an illustrative
// Example value used to build the "Contoh Penggunaan" sheet.
var exampleCols = []ColumnSpec{
	{Name: "nama", Required: true, Kind: "text", Example: "Budi Santoso"},
	{Name: "harga", Required: true, Kind: "decimal", Example: "15000000"},
}

func TestBuildTemplateCSV(t *testing.T) {
	body, ct, ext, err := BuildTemplate("csv", testCols)
	if err != nil {
		t.Fatal(err)
	}
	if ext != "csv" || ct != "text/csv" {
		t.Fatalf("bad meta: %s %s", ct, ext)
	}
	if string(body) != "\xEF\xBB\xBFnama,harga\n" {
		t.Fatalf("bad body: %q", string(body))
	}
}

// TestBuildTemplateCSVHasBOM asserts the CSV template body is prefixed with
// the UTF-8 BOM so Excel on a Windows locale reads it as UTF-8 instead of
// mojibaking non-ASCII header/data as Windows-1252.
func TestBuildTemplateCSVHasBOM(t *testing.T) {
	body, _, _, err := BuildTemplate("csv", testCols)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Fatalf("expected UTF-8 BOM, got % x", body[:min(3, len(body))])
	}
}

// TestBuildTemplateXLSXHasNoBOM ensures the BOM fix is scoped to the CSV
// branch only; XLSX is a binary zip format and must be untouched.
func TestBuildTemplateXLSXHasNoBOM(t *testing.T) {
	body, _, _, err := BuildTemplate("xlsx", testCols)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		t.Fatalf("xlsx body should not carry a UTF-8 BOM")
	}
}

func TestBuildTemplateXLSX(t *testing.T) {
	body, _, ext, err := BuildTemplate("xlsx", testCols)
	if err != nil {
		t.Fatal(err)
	}
	if ext != "xlsx" {
		t.Fatalf("bad ext %s", ext)
	}
	// The first sheet ("Data Impor") is header-only, so parsing it yields no
	// data rows — proving the fill-in sheet is empty and comes first.
	rows, err := Parse("xlsx", body, testCols, 100)
	if err != ErrEmptyFile {
		t.Fatalf("template should have header only, got rows=%v err=%v", rows, err)
	}
}

// TestBuildTemplateXLSXTwoSheets asserts the XLSX template carries exactly the
// two named sheets in the required order: "Data Impor" first (the fill-in
// sheet the parser reads on re-upload) then "Contoh Penggunaan".
func TestBuildTemplateXLSXTwoSheets(t *testing.T) {
	body, _, _, err := BuildTemplate("xlsx", exampleCols)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) != 2 || sheets[0] != sheetData || sheets[1] != sheetExample {
		t.Fatalf("want sheets [%q %q], got %v", sheetData, sheetExample, sheets)
	}

	// "Data Impor" is header-only.
	dataRows, err := f.GetRows(sheetData)
	if err != nil {
		t.Fatal(err)
	}
	if len(dataRows) != 1 {
		t.Fatalf("Data Impor should have only a header row, got %d rows", len(dataRows))
	}
	if dataRows[0][0] != "nama" || dataRows[0][1] != "harga" {
		t.Fatalf("bad Data Impor header: %v", dataRows[0])
	}

	// "Contoh Penggunaan" has the header plus one example row.
	exRows, err := f.GetRows(sheetExample)
	if err != nil {
		t.Fatal(err)
	}
	if len(exRows) != 2 {
		t.Fatalf("Contoh Penggunaan should have header + 1 example row, got %d rows", len(exRows))
	}
	if exRows[1][0] != "Budi Santoso" || exRows[1][1] != "15000000" {
		t.Fatalf("bad example row: %v", exRows[1])
	}
}

func TestBuildTemplateBadFormat(t *testing.T) {
	if _, _, _, err := BuildTemplate("pdf", testCols); err != ErrBadFormat {
		t.Fatalf("want ErrBadFormat, got %v", err)
	}
}
