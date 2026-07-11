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
	if string(body) != "nama,harga\n" {
		t.Fatalf("bad body: %q", string(body))
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
