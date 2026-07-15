package geoip

import "testing"

func TestNew_EmptyPathReturnsNoop(t *testing.T) {
	loc := New("", nil)
	if _, ok := loc.(noopLocator); !ok {
		t.Fatalf("empty path must yield a noopLocator, got %T", loc)
	}
	city, country := loc.Lookup("8.8.8.8")
	if city != "" || country != "" {
		t.Fatalf("noop must resolve nothing, got %q/%q", city, country)
	}
	if err := loc.Close(); err != nil {
		t.Fatalf("noop Close: %v", err)
	}
}

func TestNew_BadPathFallsBackToNoop(t *testing.T) {
	loc := New("/nonexistent/does-not-exist.mmdb", nil)
	if _, ok := loc.(noopLocator); !ok {
		t.Fatalf("unreadable path must fall back to noopLocator, got %T", loc)
	}
}
