package importer

import (
	"strings"
	"testing"
)

var testCols = []ColumnSpec{
	{Name: "nama", Required: true, Kind: "text"},
	{Name: "harga", Required: true, Kind: "decimal"},
}

func TestParseCSV_OK(t *testing.T) {
	csv := "nama,harga\nMeja,1000\nKursi,2000\n"
	rows, err := Parse("csv", []byte(csv), testCols, 100)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Cells["nama"] != "Meja" || rows[1].Cells["harga"] != "2000" {
		t.Fatalf("bad cell values: %+v", rows)
	}
	if rows[0].RowNo != 1 {
		t.Fatalf("want RowNo 1, got %d", rows[0].RowNo)
	}
}

func TestParseCSV_BadHeader(t *testing.T) {
	_, err := Parse("csv", []byte("wrong,cols\n1,2\n"), testCols, 100)
	if err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("want header error, got %v", err)
	}
}

func TestParseCSV_HeaderCaseInsensitiveAndReordered(t *testing.T) {
	rows, err := Parse("csv", []byte("HARGA,Nama\n1000,Meja\n"), testCols, 100)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rows[0].Cells["nama"] != "Meja" || rows[0].Cells["harga"] != "1000" {
		t.Fatalf("column mapping wrong: %+v", rows[0])
	}
}

func TestParseCSV_TooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("nama,harga\n")
	for i := 0; i < 5; i++ {
		b.WriteString("x,1\n")
	}
	_, err := Parse("csv", []byte(b.String()), testCols, 3)
	if err != ErrTooManyRows {
		t.Fatalf("want ErrTooManyRows, got %v", err)
	}
}

func TestParseCSV_Empty(t *testing.T) {
	_, err := Parse("csv", []byte("nama,harga\n"), testCols, 100)
	if err != ErrEmptyFile {
		t.Fatalf("want ErrEmptyFile, got %v", err)
	}
}

func TestParse_BadFormat(t *testing.T) {
	_, err := Parse("pdf", []byte("x"), testCols, 100)
	if err != ErrBadFormat {
		t.Fatalf("want ErrBadFormat, got %v", err)
	}
}
