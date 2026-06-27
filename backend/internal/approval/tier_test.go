package approval

import (
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func tier(v sqlc.SharedApproverLevel) *sqlc.SharedApproverLevel { return &v }

func TestResolveTierOffice(t *testing.T) {
	pusat := uuid.New()
	wil := uuid.New()
	cab := uuid.New()
	out := uuid.New()
	// ancestors of `out`: out(office) -> cab(office) -> wil(wilayah) -> pusat(pusat)
	anc := []sqlc.GetOfficeAncestorsRow{
		{ID: out, ParentID: &cab, Tier: tier(sqlc.SharedApproverLevelOffice)},
		{ID: cab, ParentID: &wil, Tier: tier(sqlc.SharedApproverLevelOffice)},
		{ID: wil, ParentID: &pusat, Tier: tier(sqlc.SharedApproverLevelWilayah)},
		{ID: pusat, ParentID: nil, Tier: tier(sqlc.SharedApproverLevelPusat)},
	}
	if got, ok := resolveTierOffice(anc, out, sqlc.SharedApproverLevelOffice); !ok || got != out {
		t.Errorf("office should resolve to origin, got %v ok=%v", got, ok)
	}
	if got, ok := resolveTierOffice(anc, out, sqlc.SharedApproverLevelWilayah); !ok || got != wil {
		t.Errorf("wilayah should resolve to wil, got %v", got)
	}
	if got, ok := resolveTierOffice(anc, out, sqlc.SharedApproverLevelPusat); !ok || got != pusat {
		t.Errorf("pusat should resolve to pusat, got %v", got)
	}
	// missing tier
	anc2 := []sqlc.GetOfficeAncestorsRow{{ID: out, ParentID: nil, Tier: tier(sqlc.SharedApproverLevelOffice)}}
	if _, ok := resolveTierOffice(anc2, out, sqlc.SharedApproverLevelPusat); ok {
		t.Errorf("missing pusat tier should be unsatisfiable")
	}
}
