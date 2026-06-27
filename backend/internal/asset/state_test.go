package asset

import (
	"testing"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestValidTransition(t *testing.T) {
	ok := [][2]sqlc.SharedAssetStatus{
		{"available", "assigned"}, {"assigned", "available"},
		{"available", "under_maintenance"}, {"under_maintenance", "available"},
		{"available", "lost"}, {"assigned", "lost"}, {"available", "disposed"},
		{"assigned", "disposed"}, {"under_maintenance", "disposed"},
	}
	for _, p := range ok {
		if !validTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s allowed", p[0], p[1])
		}
	}
	bad := [][2]sqlc.SharedAssetStatus{
		{"disposed", "available"}, {"available", "in_transfer"},
		{"available", "retired"}, {"lost", "available"},
	}
	for _, p := range bad {
		if validTransition(p[0], p[1]) {
			t.Errorf("expected %s->%s rejected", p[0], p[1])
		}
	}
}
