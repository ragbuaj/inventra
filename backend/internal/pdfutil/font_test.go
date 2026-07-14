package pdfutil

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewUTF8PDF_RendersUnicodeWithoutCoreFontFallback(t *testing.T) {
	pdf := NewUTF8PDF("P", "mm", "A4")
	pdf.AddPage()

	pdf.SetFont(FontFamily, "B", 12)
	pdf.CellFormat(0, 10, "Lokasi · Gedung — José Peña", "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("pdf.Output returned error: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected non-empty PDF output")
	}

	if pdf.Err() {
		t.Fatalf("fpdf internal error: %s", pdf.Error())
	}

	if strings.Contains(buf.String(), "/BaseFont /Helvetica") {
		t.Fatal("expected embedded DejaVu font, but output references core Helvetica BaseFont")
	}

	t.Run("bold-italic style registered", func(t *testing.T) {
		pdf2 := NewUTF8PDF("P", "mm", "A4")
		pdf2.AddPage()

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SetFont(FontFamily, \"BI\", 10) panicked: %v", r)
			}
		}()

		pdf2.SetFont(FontFamily, "BI", 10)
		pdf2.CellFormat(0, 10, "Bold Italic — test", "", 1, "L", false, 0, "")

		var buf2 bytes.Buffer
		if err := pdf2.Output(&buf2); err != nil {
			t.Fatalf("pdf.Output returned error: %v", err)
		}
		if buf2.Len() == 0 {
			t.Fatal("expected non-empty PDF output for bold-italic style")
		}
	})
}
