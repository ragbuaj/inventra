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

func TestCoerceInt(t *testing.T) {
	r := resource{Columns: []column{
		{Name: "min_floors", Type: typeInt, Required: true},
		{Name: "max_floors", Type: typeInt},
	}}

	t.Run("json number (float64) coerces to int64", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"min_floors": float64(1), "max_floors": float64(25)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != int64(1) || out[1] != int64(25) {
			t.Fatalf("got %v / %v, want 1 / 25", out[0], out[1])
		}
	})

	t.Run("numeric string coerces; empty nullable maps to nil", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"min_floors": "26", "max_floors": ""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != int64(26) {
			t.Fatalf("got %v, want 26", out[0])
		}
		if out[1] != nil {
			t.Fatalf("got %v, want nil", out[1])
		}
	})

	t.Run("required absent errors", func(t *testing.T) {
		if _, err := coerce(r, map[string]any{"max_floors": float64(5)}); err == nil {
			t.Fatal("expected error for missing required min_floors")
		}
	})

	t.Run("nullable absent maps to nil", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"min_floors": float64(3)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[1] != nil {
			t.Fatalf("got %v, want nil for absent max_floors", out[1])
		}
	})

	t.Run("non-numeric string errors", func(t *testing.T) {
		if _, err := coerce(r, map[string]any{"min_floors": "abc"}); err == nil {
			t.Fatal("expected error for non-numeric string")
		}
	})
}
