package asset

import (
	"testing"
)

func TestParsePurchaseDate(t *testing.T) {
	t.Run("nil returns zero Date with no error", func(t *testing.T) {
		d, err := parsePurchaseDate(nil)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if d.Valid {
			t.Fatalf("expected Valid=false for nil input, got Valid=true")
		}
	})

	t.Run("valid RFC3339 date parses correctly", func(t *testing.T) {
		s := "2026-06-28"
		d, err := parsePurchaseDate(&s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Valid {
			t.Fatalf("expected Valid=true for %q", s)
		}
		if got := d.Time.Format("2006-01-02"); got != s {
			t.Fatalf("expected %q, got %q", s, got)
		}
	})

	t.Run("slash-separated date returns error", func(t *testing.T) {
		s := "2026/06/28"
		_, err := parsePurchaseDate(&s)
		if err == nil {
			t.Fatal("expected an error for malformed date, got nil")
		}
	})

	t.Run("empty string returns error", func(t *testing.T) {
		s := ""
		_, err := parsePurchaseDate(&s)
		if err == nil {
			t.Fatal("expected an error for empty string, got nil")
		}
	})

	t.Run("date-time string returns error", func(t *testing.T) {
		s := "2026-06-28T00:00:00Z"
		_, err := parsePurchaseDate(&s)
		if err == nil {
			t.Fatal("expected an error for datetime string, got nil")
		}
	})
}
