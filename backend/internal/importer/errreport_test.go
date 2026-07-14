package importer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestBuildErrorReportCSV(t *testing.T) {
	rows := []sqlc.ImportImportRow{
		{
			Data:   []byte(`{"nama":"Meja","harga":"dua juta"}`),
			Errors: []byte(`[{"column":"harga","error_key":"harga"}]`),
		},
		{
			Data:   []byte(`{"nama":"Kursi","harga":"5000"}`),
			Errors: []byte(`[]`),
		},
	}

	body, ct, ext, err := BuildErrorReport("csv", testCols, rows)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ct != "text/csv" || ext != "csv" {
		t.Fatalf("bad meta: %s %s", ct, ext)
	}
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Fatalf("expected UTF-8 BOM, got % x", body[:min(3, len(body))])
	}
	body = body[3:]

	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), string(body))
	}
	if lines[0] != "nama,harga,keterangan" {
		t.Fatalf("bad header: %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], "harga") {
		t.Fatalf("row A should end with harga error key, got %q", lines[1])
	}
	if !strings.HasSuffix(lines[2], ",") {
		t.Fatalf("row B (valid) should have an empty keterangan, got %q", lines[2])
	}
}

// TestBuildErrorReportCSVHasBOM asserts the error-report CSV body is
// prefixed with the UTF-8 BOM at byte offset 0, so Excel on a Windows
// locale reads the "keterangan" error text as UTF-8 rather than mojibaking
// it as Windows-1252.
func TestBuildErrorReportCSVHasBOM(t *testing.T) {
	rows := []sqlc.ImportImportRow{
		{Data: []byte(`{"nama":"Meja","harga":"1000"}`), Errors: []byte(`[]`)},
	}
	body, _, _, err := BuildErrorReport("csv", testCols, rows)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Fatalf("expected UTF-8 BOM, got % x", body[:min(3, len(body))])
	}
}

// TestBuildErrorReportXLSXHasNoBOM ensures the BOM fix is scoped to the CSV
// branch only; XLSX is a binary zip format and must be untouched.
func TestBuildErrorReportXLSXHasNoBOM(t *testing.T) {
	rows := []sqlc.ImportImportRow{
		{Data: []byte(`{"nama":"Meja","harga":"1000"}`), Errors: []byte(`[]`)},
	}
	body, _, _, err := BuildErrorReport("xlsx", testCols, rows)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		t.Fatalf("xlsx body should not carry a UTF-8 BOM")
	}
}

func TestBuildErrorReportBadFormat(t *testing.T) {
	if _, _, _, err := BuildErrorReport("pdf", testCols, nil); err != ErrBadFormat {
		t.Fatalf("want ErrBadFormat, got %v", err)
	}
}

func TestBuildErrorReportXLSX(t *testing.T) {
	rows := []sqlc.ImportImportRow{
		{
			Data:   []byte(`{"nama":"Meja","harga":"dua juta"}`),
			Errors: []byte(`[{"column":"harga","error_key":"harga"}]`),
		},
	}

	body, ct, ext, err := BuildErrorReport("xlsx", testCols, rows)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ext != "xlsx" {
		t.Fatalf("bad ext %s", ext)
	}
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("bad content type: %s", ct)
	}

	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("OpenReader failed: %v", err)
	}
	defer f.Close()

	sheet := f.GetSheetList()[0]
	got, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows (header + 1 data), got %d: %+v", len(got), got)
	}
	if got[0][0] != "nama" || got[0][1] != "harga" || got[0][2] != "keterangan" {
		t.Fatalf("bad header row: %+v", got[0])
	}
	if got[1][2] != "harga" {
		t.Fatalf("want keterangan cell = harga, got %q", got[1][2])
	}
}
