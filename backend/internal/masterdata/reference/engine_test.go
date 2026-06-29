package reference

import "testing"

func TestCoerceEnum(t *testing.T) {
	r := resource{Columns: []column{
		{Name: "tier", Type: typeEnum, EnumType: "shared.approver_level", Enum: []string{"pusat", "wilayah", "office"}},
	}}

	t.Run("valid value passes through", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"tier": "wilayah"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != "wilayah" {
			t.Fatalf("got %v, want wilayah", out[0])
		}
	})

	t.Run("invalid value errors", func(t *testing.T) {
		if _, err := coerce(r, map[string]any{"tier": "bogus"}); err == nil {
			t.Fatal("expected error for invalid enum value")
		}
	})

	t.Run("absent maps to nil", func(t *testing.T) {
		out, err := coerce(r, map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != nil {
			t.Fatalf("got %v, want nil", out[0])
		}
	})

	t.Run("empty string maps to nil", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"tier": ""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != nil {
			t.Fatalf("got %v, want nil", out[0])
		}
	})
}
