package stockopname

import "testing"

func sampleReport() ReportData {
	return ReportData{
		SessionName: "Opname Semester I 2026", OfficeName: "KC Jakarta Selatan",
		Period: "Juni 2026", ClosedByName: "Dewi Lestari",
		Kpi:   KpiCounts{Total: 4, Found: 2, Pending: 0, Variance: 2},
		Items: []ReportItem{{AssetName: "Laptop", AssetTag: "JKT01-ELK-2026-00001", Result: "found"}}}
}

func TestRenderPDFNonEmpty(t *testing.T) {
	b, err := RenderPDF(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 100 || string(b[:4]) != "%PDF" {
		t.Fatalf("not a PDF: %d bytes", len(b))
	}
}

func TestRenderXLSXNonEmpty(t *testing.T) {
	b, err := RenderXLSX(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 100 || string(b[:2]) != "PK" {
		t.Fatalf("not an xlsx zip: %d bytes", len(b))
	}
}
