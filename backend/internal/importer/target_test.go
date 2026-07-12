package importer

import (
	"context"
	"testing"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// stubTarget is a minimal, fully-conforming TargetImporter used only to
// exercise the registry helpers.
type stubTarget struct{ name string }

func (s stubTarget) Target() string { return s.name }

func (s stubTarget) Columns() []ColumnSpec {
	return []ColumnSpec{{Name: "a", Required: true, Kind: "text"}}
}

func (s stubTarget) ValidateRows(ctx context.Context, rows []RawRow, scope Scope) ([]RowResult, error) {
	return nil, nil
}

func (s stubTarget) Execute(ctx context.Context, qtx *sqlc.Queries, job Job, validRows []Row) (int, error) {
	return len(validRows), nil
}

func (s stubTarget) NeedsApproval() bool { return false }

func TestRegistryGet(t *testing.T) {
	r := registry{}
	r["asset"] = stubTarget{name: "asset"}

	got, ok := r.get("asset")
	if !ok {
		t.Fatal("expected hit for known target \"asset\"")
	}
	if got.Target() != "asset" {
		t.Fatalf("expected target name %q, got %q", "asset", got.Target())
	}

	if _, ok := r.get("nope"); ok {
		t.Fatal("expected miss for unknown target \"nope\"")
	}
}

func TestRegistryTargets(t *testing.T) {
	r := registry{}
	r["office"] = stubTarget{name: "office"}
	r["asset"] = stubTarget{name: "asset"}
	r["employee"] = stubTarget{name: "employee"}

	got := r.targets()
	if len(got) != 3 {
		t.Fatalf("expected 3 targets, got %d (%v)", len(got), got)
	}

	want := []string{"asset", "employee", "office"}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("expected sorted targets %v, got %v", want, got)
		}
	}
}

func TestRegistryTargetsEmpty(t *testing.T) {
	r := registry{}
	if got := r.targets(); len(got) != 0 {
		t.Fatalf("expected 0 targets for empty registry, got %d", len(got))
	}
}
