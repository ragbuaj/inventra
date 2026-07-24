package department

import (
	"errors"
	"testing"
)

// validOffice is a well-formed office UUID string used by the success cases —
// every department now requires an office (no global/NULL-office departments).
const validOffice = "11111111-1111-1111-1111-111111111111"

func TestRequestToInput(t *testing.T) {
	office := validOffice

	t.Run("trims name and code", func(t *testing.T) {
		code := "  D1  "
		in, err := Request{Name: "  Ops  ", Code: &code, OfficeID: &office}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.Name != "Ops" {
			t.Fatalf("name = %q, want %q", in.Name, "Ops")
		}
		if in.Code == nil || *in.Code != "D1" {
			t.Fatalf("code = %v, want %q", in.Code, "D1")
		}
		if in.OfficeID == nil || in.OfficeID.String() != validOffice {
			t.Fatalf("office = %v, want %q", in.OfficeID, validOffice)
		}
	})

	t.Run("blank/whitespace name is rejected (before the office check)", func(t *testing.T) {
		for _, name := range []string{"", "   ", "\t"} {
			if _, err := (Request{Name: name}).toInput(); !errors.Is(err, ErrBlankName) {
				t.Fatalf("name %q: expected ErrBlankName, got %v", name, err)
			}
		}
	})

	t.Run("missing office is rejected", func(t *testing.T) {
		if _, err := (Request{Name: "Ops"}).toInput(); !errors.Is(err, ErrOfficeRequired) {
			t.Fatalf("expected ErrOfficeRequired, got %v", err)
		}
		blank := ""
		if _, err := (Request{Name: "Ops", OfficeID: &blank}).toInput(); !errors.Is(err, ErrOfficeRequired) {
			t.Fatalf("blank office: expected ErrOfficeRequired, got %v", err)
		}
	})

	t.Run("blank code becomes nil", func(t *testing.T) {
		code := "   "
		in, err := Request{Name: "Ops", Code: &code, OfficeID: &office}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.Code != nil {
			t.Fatalf("code = %v, want nil", in.Code)
		}
	})

	t.Run("is_active defaults to true when absent", func(t *testing.T) {
		in, err := Request{Name: "Ops", OfficeID: &office}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !in.IsActive {
			t.Fatal("is_active should default to true")
		}
	})

	t.Run("floor is parsed when supplied", func(t *testing.T) {
		floor := "22222222-2222-2222-2222-222222222222"
		in, err := Request{Name: "Ops", OfficeID: &office, FloorID: &floor}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.FloorID == nil || in.FloorID.String() != floor {
			t.Fatalf("floor = %v, want %q", in.FloorID, floor)
		}
	})
}
