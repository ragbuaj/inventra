package report

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestResolvePeriodPresets(t *testing.T) {
	now := date("2026-07-11")

	cur, prev, err := ResolvePeriod("last30", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-06-12"), cur.From)
	assert.Equal(t, date("2026-07-11"), cur.To)
	assert.Equal(t, date("2026-05-13"), prev.From) // same 30-day length, ends day before cur.From
	assert.Equal(t, date("2026-06-11"), prev.To)

	cur, _, err = ResolvePeriod("this_month", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-07-01"), cur.From)
	assert.Equal(t, date("2026-07-11"), cur.To)

	cur, _, err = ResolvePeriod("this_quarter", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-07-01"), cur.From) // Q3 starts July

	cur, _, err = ResolvePeriod("ytd", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-01-01"), cur.From)
}

func TestResolvePeriodCustom(t *testing.T) {
	now := date("2026-07-11")
	cur, prev, err := ResolvePeriod("", "2026-03-01", "2026-03-31", now)
	require.NoError(t, err)
	assert.Equal(t, date("2026-03-01"), cur.From)
	assert.Equal(t, date("2026-03-31"), cur.To)
	assert.Equal(t, date("2026-01-29"), prev.From) // 31 days ending 2026-02-28
	assert.Equal(t, date("2026-02-28"), prev.To)
}

func TestResolvePeriodErrors(t *testing.T) {
	now := date("2026-07-11")
	for _, tc := range []struct{ preset, from, to string }{
		{"bogus", "", ""},                      // unknown preset
		{"", "", ""},                           // nothing given
		{"", "2026-01-01", ""},                 // half a custom range
		{"", "2026-02-01", "2026-01-01"},       // from > to
		{"", "01-02-2026", "2026-03-01"},       // bad format
		{"last30", "2026-01-01", "2026-02-01"}, // both preset and custom
	} {
		_, _, err := ResolvePeriod(tc.preset, tc.from, tc.to, now)
		assert.ErrorIs(t, err, ErrInvalidPeriod, "preset=%q from=%q to=%q", tc.preset, tc.from, tc.to)
	}
}

func TestParseReportType(t *testing.T) {
	for _, ok := range []string{"assets", "depreciation", "utilization", "maintenance", "transfers", "disposals", "opname"} {
		got, err := ParseReportType(ok)
		require.NoError(t, err)
		assert.Equal(t, ok, got)
	}
	_, err := ParseReportType("aset; DROP TABLE")
	assert.ErrorIs(t, err, ErrInvalidReportType)
}
