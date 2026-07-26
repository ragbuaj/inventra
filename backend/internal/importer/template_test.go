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

// TestBuildTemplateWith_CustomSpec exercises the full TemplateSpec surface a
// target (e.g. the asset importer) uses: custom sheet names, an example note
// (which shifts the example header to row 2), explicit multi-row examples,
// required-column markers ("*") on the headers, and per-column dropdowns backed
// by a hidden "_pilihan" options sheet.
func TestBuildTemplateWith_CustomSpec(t *testing.T) {
	spec := TemplateSpec{
		DataSheet:    "Aset",
		ExampleSheet: "Contoh Pengisian",
		ExampleNote:  "SHEET INI HANYA CONTOH - jangan diisi.",
		ExampleRows: [][]string{
			{"Budi Santoso", "15000000"},
			{"Sri Rahayu", "250000000"},
		},
		Dropdowns:    map[string][]string{"nama": {"Budi Santoso", "Sri Rahayu"}},
		MarkRequired: true,
	}

	body, ct, ext, err := BuildTemplateWith("xlsx", exampleCols, spec)
	if err != nil {
		t.Fatal(err)
	}
	if ext != "xlsx" || ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("bad meta: %s %s", ct, ext)
	}

	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// The custom fill/example sheets come first (fill sheet MUST be first so the
	// parser reads it on re-upload), then the hidden "_pilihan" options sheet.
	sheets := f.GetSheetList()
	if len(sheets) != 3 || sheets[0] != "Aset" || sheets[1] != "Contoh Pengisian" || sheets[2] != sheetOptions {
		t.Fatalf("want sheets [Aset, Contoh Pengisian, %q], got %v", sheetOptions, sheets)
	}

	// The options sheet is hidden.
	visible, err := f.GetSheetVisible(sheetOptions)
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Fatalf("%q must be hidden", sheetOptions)
	}

	// Required headers carry the "*" marker on the fill sheet.
	dataRows, err := f.GetRows("Aset")
	if err != nil {
		t.Fatal(err)
	}
	if len(dataRows) != 1 {
		t.Fatalf("fill sheet must be header-only, got %d rows", len(dataRows))
	}
	if dataRows[0][0] != "nama*" || dataRows[0][1] != "harga*" {
		t.Fatalf("required headers must be marked with '*': %v", dataRows[0])
	}

	// Example sheet: row 1 note, row 2 marked header, rows 3-4 the two examples.
	exRows, err := f.GetRows("Contoh Pengisian")
	if err != nil {
		t.Fatal(err)
	}
	if len(exRows) != 4 {
		t.Fatalf("example sheet want note + header + 2 rows = 4, got %d: %v", len(exRows), exRows)
	}
	if exRows[0][0] != spec.ExampleNote {
		t.Fatalf("example row 1 must be the note, got %q", exRows[0][0])
	}
	if exRows[1][0] != "nama*" || exRows[1][1] != "harga*" {
		t.Fatalf("example header (row 2) must be marked: %v", exRows[1])
	}
	if exRows[2][0] != "Budi Santoso" || exRows[3][0] != "Sri Rahayu" {
		t.Fatalf("example data rows wrong: %v / %v", exRows[2], exRows[3])
	}

	// The options sheet carries the dropdown values for "nama".
	optRows, err := f.GetRows(sheetOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(optRows) < 3 || optRows[0][0] != "nama" || optRows[1][0] != "Budi Santoso" || optRows[2][0] != "Sri Rahayu" {
		t.Fatalf("options sheet must hold the nama dropdown values, got %v", optRows)
	}

	// A data validation (the drop list) is attached to the fill sheet.
	dvs, err := f.GetDataValidations("Aset")
	if err != nil {
		t.Fatal(err)
	}
	if len(dvs) == 0 {
		t.Fatalf("expected a data-validation drop list on the fill sheet, got none")
	}

	// The marked, header-only fill sheet still round-trips through Parse: the
	// "*" markers are stripped by normHeader, so the columns resolve and the
	// header-only sheet yields ErrEmptyFile (no data rows).
	if _, err := Parse("xlsx", body, exampleCols, 100); err != ErrEmptyFile {
		t.Fatalf("marked header-only template should parse to ErrEmptyFile, got %v", err)
	}
}

// TestBuildTemplateWith_NoMarkerWhenNotRequested asserts MarkRequired defaults
// off: with a zero spec the headers stay bare even for required columns.
func TestBuildTemplateWith_NoMarkerWhenNotRequested(t *testing.T) {
	body, _, _, err := BuildTemplateWith("xlsx", exampleCols, TemplateSpec{})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// No dropdowns requested -> no hidden options sheet, just the two defaults.
	sheets := f.GetSheetList()
	if len(sheets) != 2 || sheets[0] != sheetData || sheets[1] != sheetExample {
		t.Fatalf("want default two sheets, got %v", sheets)
	}
	dataRows, err := f.GetRows(sheetData)
	if err != nil {
		t.Fatal(err)
	}
	if dataRows[0][0] != "nama" || dataRows[0][1] != "harga" {
		t.Fatalf("headers must stay bare without MarkRequired: %v", dataRows[0])
	}
}
