// Unit tests for the worker's pure helpers. The tick()/phase logic requires a
// live Postgres + Redis + storage backend and is covered by integration tests
// in a later task; here we only exercise progressKey and aggregate, which
// have no external dependencies.
package importer

import (
	"testing"

	"github.com/google/uuid"
)

func TestProgressKey(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	if progressKey(id) != "import:progress:00000000-0000-0000-0000-000000000001" {
		t.Fatalf("bad key %s", progressKey(id))
	}
}

func TestAggregate(t *testing.T) {
	results := []RowResult{
		{RowNo: 1, Valid: true},
		{RowNo: 2, Valid: false},
		{RowNo: 3, Valid: true},
		{RowNo: 4, Valid: true},
		{RowNo: 5, Valid: false},
	}
	success, failed := aggregate(results)
	if success != 3 {
		t.Fatalf("expected 3 successes, got %d", success)
	}
	if failed != 2 {
		t.Fatalf("expected 2 failures, got %d", failed)
	}
}

func TestAggregateEmpty(t *testing.T) {
	success, failed := aggregate(nil)
	if success != 0 || failed != 0 {
		t.Fatalf("expected 0/0 for empty input, got %d/%d", success, failed)
	}
}

func TestAggregateAllValid(t *testing.T) {
	results := []RowResult{{Valid: true}, {Valid: true}}
	success, failed := aggregate(results)
	if success != 2 || failed != 0 {
		t.Fatalf("expected 2/0, got %d/%d", success, failed)
	}
}

func TestAggregateAllInvalid(t *testing.T) {
	results := []RowResult{{Valid: false}, {Valid: false}, {Valid: false}}
	success, failed := aggregate(results)
	if success != 0 || failed != 3 {
		t.Fatalf("expected 0/3, got %d/%d", success, failed)
	}
}

// --- sumHarga ---

func TestSumHargaValidRows(t *testing.T) {
	rows := []Row{
		{RowNo: 1, Data: map[string]string{"harga": "1000.50"}},
		{RowNo: 2, Data: map[string]string{"harga": "2449.50"}},
	}
	got, err := sumHarga(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "3450.00" {
		t.Fatalf("expected 3450.00, got %s", got)
	}
}

func TestSumHargaSkipsMissingOrEmptyCell(t *testing.T) {
	rows := []Row{
		{RowNo: 1, Data: map[string]string{"harga": "100"}},
		{RowNo: 2, Data: map[string]string{}},            // no "harga" key at all
		{RowNo: 3, Data: map[string]string{"harga": ""}}, // present but empty
		{RowNo: 4, Data: map[string]string{"harga": "50"}},
	}
	got, err := sumHarga(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "150.00" {
		t.Fatalf("expected 150.00, got %s", got)
	}
}

func TestSumHargaEmptyRows(t *testing.T) {
	got, err := sumHarga(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.00" {
		t.Fatalf("expected 0.00, got %s", got)
	}
}

func TestSumHargaExactDecimalNoFloatDrift(t *testing.T) {
	// 0.1 + 0.2 famously drifts under float64; big.Rat must not.
	rows := []Row{
		{RowNo: 1, Data: map[string]string{"harga": "0.10"}},
		{RowNo: 2, Data: map[string]string{"harga": "0.20"}},
	}
	got, err := sumHarga(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.30" {
		t.Fatalf("expected 0.30, got %s", got)
	}
}

func TestSumHargaNonNumericHargaErrors(t *testing.T) {
	// Current contract: a present, non-empty, non-numeric "harga" cell is a
	// hard error (the caller treats this as a job-failing condition), not a
	// silently-skipped row — since ValidateRows should already have rejected
	// unparseable harga cells at validate time, this is a defense-in-depth
	// invariant check, not an expected runtime path.
	rows := []Row{
		{RowNo: 1, Data: map[string]string{"harga": "100"}},
		{RowNo: 2, Data: map[string]string{"harga": "not-a-number"}},
	}
	_, err := sumHarga(rows)
	if err == nil {
		t.Fatalf("expected error for non-numeric harga, got nil")
	}
}

// --- firstValidOffice ---

func TestFirstValidOfficeParsesFirstValidRow(t *testing.T) {
	want := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	results := []RowResult{
		{RowNo: 1, Valid: false, NormalizedRef: want.String()}, // invalid row is skipped even with a ref
		{RowNo: 2, Valid: true, NormalizedRef: want.String()},
		{RowNo: 3, Valid: true, NormalizedRef: "22222222-2222-2222-2222-222222222222"},
	}
	got := firstValidOffice(results)
	if got == nil {
		t.Fatalf("expected a non-nil office id")
	}
	if *got != want {
		t.Fatalf("expected %s, got %s", want, *got)
	}
}

func TestFirstValidOfficeNoValidRows(t *testing.T) {
	results := []RowResult{
		{RowNo: 1, Valid: false, NormalizedRef: "11111111-1111-1111-1111-111111111111"},
	}
	if got := firstValidOffice(results); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestFirstValidOfficeEmptyNormalizedRef(t *testing.T) {
	// M3 edge case: a valid row whose target carries no office ref at all
	// (NormalizedRef empty) — must resolve to nil, not uuid.Nil-as-non-nil,
	// so executePhase's "noOffice" guard can fail the job cleanly rather
	// than calling approval.Submit with an unrouteable office.
	results := []RowResult{
		{RowNo: 1, Valid: true, NormalizedRef: ""},
	}
	if got := firstValidOffice(results); got != nil {
		t.Fatalf("expected nil for empty NormalizedRef, got %v", got)
	}
}

func TestFirstValidOfficeGarbageNormalizedRef(t *testing.T) {
	// A valid row whose NormalizedRef is present but not a parseable UUID
	// (garbage data) must also resolve to nil rather than propagating a
	// parse error.
	results := []RowResult{
		{RowNo: 1, Valid: true, NormalizedRef: "not-a-uuid"},
	}
	if got := firstValidOffice(results); got != nil {
		t.Fatalf("expected nil for garbage NormalizedRef, got %v", got)
	}
}

func TestFirstValidOfficeEmptyResults(t *testing.T) {
	if got := firstValidOffice(nil); got != nil {
		t.Fatalf("expected nil for empty results, got %v", got)
	}
}
