package assignment

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestMarshalBorrowPayload(t *testing.T) {
	id := uuid.New()
	due := "2026-07-15"
	notes := "presentasi"
	b, err := marshalBorrowPayload(BorrowInput{AssetID: id, DueDate: &due, Notes: &notes})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p BorrowPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.AssetID != id.String() {
		t.Errorf("asset_id = %q, want %q", p.AssetID, id.String())
	}
	if p.DueDate == nil || *p.DueDate != due {
		t.Errorf("due_date = %v, want %q", p.DueDate, due)
	}
	if p.Notes == nil || *p.Notes != notes {
		t.Errorf("notes = %v, want %q", p.Notes, notes)
	}
}

func TestMarshalBorrowPayload_NilOptionals(t *testing.T) {
	b, err := marshalBorrowPayload(BorrowInput{AssetID: uuid.New()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p BorrowPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.DueDate != nil || p.Notes != nil || p.ConditionOut != nil {
		t.Errorf("expected nil optionals, got %+v", p)
	}
}
