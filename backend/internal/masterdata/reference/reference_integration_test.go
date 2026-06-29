//go:build integration

package reference

import (
	"context"
	"testing"

	"github.com/google/uuid"

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
