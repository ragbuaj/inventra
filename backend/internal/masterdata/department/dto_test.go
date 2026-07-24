package department

import (
	"errors"
	"testing"
)

func TestRequestToInput(t *testing.T) {
	t.Run("trims name and code", func(t *testing.T) {
		code := "  D1  "
		in, err := Request{Name: "  Ops  ", Code: &code}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.Name != "Ops" {
			t.Fatalf("name = %q, want %q", in.Name, "Ops")
		}
		if in.Code == nil || *in.Code != "D1" {
			t.Fatalf("code = %v, want %q", in.Code, "D1")
		}
	})

	t.Run("blank/whitespace name is rejected", func(t *testing.T) {
		for _, name := range []string{"", "   ", "\t"} {
			if _, err := (Request{Name: name}).toInput(); !errors.Is(err, ErrBlankName) {
				t.Fatalf("name %q: expected ErrBlankName, got %v", name, err)
			}
		}
	})

	t.Run("blank code becomes nil", func(t *testing.T) {
		code := "   "
		in, err := Request{Name: "Ops", Code: &code}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.Code != nil {
			t.Fatalf("code = %v, want nil", in.Code)
		}
	})

	t.Run("is_active defaults to true when absent", func(t *testing.T) {
		in, err := Request{Name: "Ops"}.toInput()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !in.IsActive {
			t.Fatal("is_active should default to true")
		}
	})
}
