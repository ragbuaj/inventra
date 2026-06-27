package asset

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// TestCreateExecOfficeMismatch verifies that createExec.Execute rejects payloads
// whose office_id does not match the office recorded on the approval request.
// The check fires before any DB interaction, so a nil *sqlc.Queries is safe.
func TestCreateExecOfficeMismatch(t *testing.T) {
	exec := createExec{s: nil} // s is not used before the office-match guard

	officeA := uuid.New()
	officeB := uuid.New()

	validPayload := func(oid uuid.UUID) []byte {
		p := AssetCreatePayload{
			Name:       "Test Asset",
			CategoryID: uuid.New().String(),
			OfficeID:   oid.String(),
			AssetClass: "tangible",
		}
		b, _ := json.Marshal(p)
		return b
	}

	t.Run("nil req.OfficeID returns ErrInvalidRef", func(t *testing.T) {
		req := sqlc.ApprovalRequest{
			OfficeID: nil,
			Payload:  validPayload(officeA),
		}
		err := exec.Execute(context.Background(), (*sqlc.Queries)(nil), req)
		if !errors.Is(err, ErrInvalidRef) {
			t.Fatalf("expected ErrInvalidRef, got %v", err)
		}
	})

	t.Run("payload office_id != req.OfficeID returns ErrInvalidRef", func(t *testing.T) {
		req := sqlc.ApprovalRequest{
			OfficeID: &officeA,
			Payload:  validPayload(officeB), // different office in payload
		}
		err := exec.Execute(context.Background(), (*sqlc.Queries)(nil), req)
		if !errors.Is(err, ErrInvalidRef) {
			t.Fatalf("expected ErrInvalidRef for office mismatch, got %v", err)
		}
	})

	t.Run("invalid payload office_id string returns ErrInvalidRef before guard", func(t *testing.T) {
		badPayload := []byte(`{"name":"x","category_id":"` + uuid.New().String() + `","office_id":"not-a-uuid","asset_class":"tangible"}`)
		req := sqlc.ApprovalRequest{
			OfficeID: &officeA,
			Payload:  badPayload,
		}
		err := exec.Execute(context.Background(), (*sqlc.Queries)(nil), req)
		if !errors.Is(err, ErrInvalidRef) {
			t.Fatalf("expected ErrInvalidRef for unparseable office_id, got %v", err)
		}
	})
}

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
