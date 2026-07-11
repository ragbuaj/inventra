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
