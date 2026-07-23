//go:build integration

package reference

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func getOfficeTypesResource() resource {
	for _, r := range referenceResources {
		if r.Path == "office-types" {
			return r
		}
	}
	panic("office-types resource not found")
}

func TestOfficeTypeTierRoundTrip(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	testsupport.Reset(t, pool)
	e := &engine{pool: pool}
	ctx := context.Background()
	ot := getOfficeTypesResource()

	t.Run("create + update round-trips tier", func(t *testing.T) {
		created, err := e.write(ctx, ot, nil, map[string]any{"name": "Kantor Pusat", "tier": "pusat", "is_active": true})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created["tier"] != "pusat" {
			t.Fatalf("tier after create = %v, want pusat", created["tier"])
		}
		id := mustParseID(t, created["id"])
		updated, err := e.write(ctx, ot, &id, map[string]any{"name": "Kantor Pusat", "tier": "wilayah", "is_active": true})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated["tier"] != "wilayah" {
			t.Fatalf("tier after update = %v, want wilayah", updated["tier"])
		}
	})

	t.Run("absent tier stored as null", func(t *testing.T) {
		created, err := e.write(ctx, ot, nil, map[string]any{"name": "Tanpa Tier"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created["tier"] != nil {
			t.Fatalf("tier = %v, want nil", created["tier"])
		}
	})
}

func mustParseID(t *testing.T, v any) uuid.UUID {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("id is not a string: %T", v)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}
	return id
}

func getResource(path string) resource {
	for _, r := range referenceResources {
		if r.Path == path {
			return r
		}
	}
	panic("resource not found: " + path)
}

// asInt reads an integer value from a pgx RowToMap result (int4 -> int32/int64).
func asInt(t *testing.T, v any) int64 {
	t.Helper()
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		t.Fatalf("not an int: %T (%v)", v, v)
		return 0
	}
}

func TestBuildingClassificationRoundTrip(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	testsupport.Reset(t, pool)
	e := &engine{pool: pool}
	ctx := context.Background()
	bc := getResource("building-classifications")

	t.Run("create with min+max int, read back", func(t *testing.T) {
		created, err := e.write(ctx, bc, nil, map[string]any{"name": "Gedung Rendah", "min_floors": float64(1), "max_floors": float64(4)})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if asInt(t, created["min_floors"]) != 1 || asInt(t, created["max_floors"]) != 4 {
			t.Fatalf("min/max = %v/%v, want 1/4", created["min_floors"], created["max_floors"])
		}
	})

	t.Run("max_floors nullable (25+)", func(t *testing.T) {
		created, err := e.write(ctx, bc, nil, map[string]any{"name": "Gedung Tinggi", "min_floors": float64(25)})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if asInt(t, created["min_floors"]) != 25 {
			t.Fatalf("min = %v, want 25", created["min_floors"])
		}
		if created["max_floors"] != nil {
			t.Fatalf("max_floors = %v, want nil", created["max_floors"])
		}
	})

	t.Run("check constraint rejects max < min as a client error (23514 -> ErrCheckViolation)", func(t *testing.T) {
		_, err := e.write(ctx, bc, nil, map[string]any{"name": "Gedung Salah", "min_floors": float64(5), "max_floors": float64(2)})
		if err == nil {
			t.Fatal("expected chk_bldg_floor_range violation")
		}
		// Must map to a 4xx client-error sentinel, not a raw 500 — a check
		// violation is bad input, not an internal error.
		if !errors.Is(err, common.ErrCheckViolation) {
			t.Fatalf("expected ErrCheckViolation, got %v", err)
		}
	})

	t.Run("missing required min_floors errors", func(t *testing.T) {
		if _, err := e.write(ctx, bc, nil, map[string]any{"name": "Tanpa Min"}); err == nil {
			t.Fatal("expected error for missing min_floors")
		}
	})
}
