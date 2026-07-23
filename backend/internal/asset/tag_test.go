package asset

import "testing"

func TestFormatAssetTag(t *testing.T) {
	got := formatAssetTag("JKT01", "ELK", 2026, 1)
	if got != "JKT01ELK202600001" {
		t.Fatalf("got %q", got)
	}
	if g := formatAssetTag("BDG02", "KEN", 2026, 12345); g != "BDG02KEN202612345" {
		t.Fatalf("got %q", g)
	}
}
