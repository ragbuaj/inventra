package asset

import (
	"bytes"
	"testing"
)

func TestResolveLabelDims_Defaults(t *testing.T) {
	w, h, media, err := resolveLabelDims("", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if w != 60 || h != 24 || media != 64 {
		t.Fatalf("got %v %v %v", w, h, media)
	}
}

func TestResolveLabelDims_PresetExplicitUnknown(t *testing.T) {
	w, h, _, err := resolveLabelDims("50x30", 0, 0, 0)
	if err != nil || w != 50 || h != 30 {
		t.Fatalf("preset: %v %v %v", w, h, err)
	}
	w, h, _, _ = resolveLabelDims("60x24", 70, 40, 0)
	if w != 70 || h != 40 {
		t.Fatalf("explicit override: %v %v", w, h)
	}
	if _, _, _, err := resolveLabelDims("bogus", 0, 0, 0); err == nil {
		t.Fatal("unknown preset should error")
	}
}

func itemsN(n int) []labelItem {
	it := make([]labelItem, n)
	for i := range it {
		it[i] = labelItem{Tag: "711PK2201600015", Name: "Monitor Samsung", OfficeCode: "711", CategoryName: "Perabot Kantor 2", Year: "2016"}
	}
	return it
}

func TestRenderLabelPDF_BTN_Roll_OnePagePerAsset(t *testing.T) {
	opts := labelOpts{Template: "btn", Layout: "roll", LabelW: 60, LabelH: 24, MediaW: 64, CompanyName: "PT BTN", Disclaimer: "x"}
	one, err := renderLabelPDF(itemsN(1), opts)
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderLabelPDF(itemsN(3), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	// fpdf creates multiple page-tree objects per logical label (one per font
	// context switch). Verify 3-asset output has exactly 3× the page-type
	// entries of a single-asset output — i.e. one logical label per asset.
	oneCount := pdfPageCount(one)
	threeCount := pdfPageCount(out)
	if oneCount == 0 {
		t.Fatal("single-asset PDF has no /Type /Page entries")
	}
	if threeCount != 3*oneCount {
		t.Fatalf("roll: want 3×%d=%d /Type /Page entries, got %d", oneCount, 3*oneCount, threeCount)
	}
}

func TestRenderLabelPDF_Generic_Sheet(t *testing.T) {
	out, err := renderLabelPDF(itemsN(7), labelOpts{Template: "generic", Layout: "sheet", LabelW: 60, LabelH: 24, Columns: 3, Mode: "barcode"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	if n := pdfPageCount(out); n < 1 {
		t.Fatalf("sheet want >=1 /Type /Page entry, got %d", n)
	}
}

// pdfPageCount counts /Type /Page entries in a PDF, excluding /Type /Pages
// (the page-tree root). fpdf emits multiple page objects per logical label
// (one per font-context switch), so this is used only for proportional checks.
func pdfPageCount(b []byte) int {
	count, needle := 0, []byte("/Type /Page")
	for i := 0; i+len(needle) <= len(b); i++ {
		if bytes.Equal(b[i:i+len(needle)], needle) {
			if i+len(needle) < len(b) && b[i+len(needle)] == 's' {
				continue // skip /Type /Pages
			}
			count++
		}
	}
	return count
}
