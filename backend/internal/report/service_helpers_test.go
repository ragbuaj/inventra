package report

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func floatPtr(v float64) *float64 { return &v }

func TestPctChange(t *testing.T) {
	cases := []struct {
		name      string
		cur, prev string
		want      *float64
	}{
		{"increase", "150", "100", floatPtr(50.0)},
		{"decrease", "80", "100", floatPtr(-20.0)},
		{"zero base -> nil", "100", "0", nil},
		{"unparseable cur -> nil", "abc", "100", nil},
		{"unparseable prev -> nil", "100", "xyz", nil},
		{"half-up rounding", "1030", "1000", floatPtr(3.0)},
		{"one decimal", "5000000.00", "2000000.00", floatPtr(150.0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pctChange(c.cur, c.prev)
			assertFloatPtr(t, c.want, got)
		})
	}
}

func TestRatioPct(t *testing.T) {
	cases := []struct {
		name        string
		part, whole string
		want        *float64
	}{
		{"simple ratio", "5000000", "140000000", floatPtr(3.6)},
		{"whole zero -> nil", "10", "0", nil},
		{"unparseable whole -> nil", "10", "nope", nil},
		{"full ratio", "50", "100", floatPtr(50.0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ratioPct(c.part, c.whole)
			assertFloatPtr(t, c.want, got)
		})
	}
}

func TestCombineDecimal(t *testing.T) {
	if got := combineDecimal("150000000.00", "150000000.00", false); got != "0.00" {
		t.Fatalf("sub: want 0.00, got %q", got)
	}
	if got := combineDecimal("135000000.00", "5000000.00", true); got != "140000000.00" {
		t.Fatalf("add: want 140000000.00, got %q", got)
	}
	if got := combineDecimal("bad", "1", true); got != "0" {
		t.Fatalf("unparseable operand should fall back to 0, got %q", got)
	}
}

func TestFormatDate(t *testing.T) {
	valid := pgtype.Date{Time: time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), Valid: true}
	if got := formatDate(valid); got != "2026-07-11" {
		t.Fatalf("want 2026-07-11, got %q", got)
	}
	if got := formatDate(pgtype.Date{}); got != "" {
		t.Fatalf("NULL date should format to empty string, got %q", got)
	}
}

func assertFloatPtr(t *testing.T, want, got *float64) {
	t.Helper()
	switch {
	case want == nil && got == nil:
		return
	case want == nil || got == nil:
		t.Fatalf("want %v, got %v", derefStr(want), derefStr(got))
	case *want != *got:
		t.Fatalf("want %v, got %v", *want, *got)
	}
}

func derefStr(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
