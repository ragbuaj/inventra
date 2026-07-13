package importer

import "testing"

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
	rows, err := Parse("xlsx", body, testCols, 100)
	if err != ErrEmptyFile {
		t.Fatalf("template should have header only, got rows=%v err=%v", rows, err)
	}
}

func TestBuildTemplateBadFormat(t *testing.T) {
	if _, _, _, err := BuildTemplate("pdf", testCols); err != ErrBadFormat {
		t.Fatalf("want ErrBadFormat, got %v", err)
	}
}
