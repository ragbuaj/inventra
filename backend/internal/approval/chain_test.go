package approval

import (
	"testing"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestBuildChain_Ordered(t *testing.T) {
	steps := []sqlc.ApprovalApprovalThreshold{
		{StepOrder: 2, RequiredLevel: "wilayah"},
		{StepOrder: 1, RequiredLevel: "office"},
		{StepOrder: 3, RequiredLevel: "pusat"},
	}
	chain := buildChain(steps)
	if len(chain) != 3 {
		t.Fatalf("want 3, got %d", len(chain))
	}
	if chain[0].Level != "office" || chain[1].Level != "wilayah" || chain[2].Level != "pusat" {
		t.Fatalf("chain not ordered: %+v", chain)
	}
}

func TestBuildChain_Empty(t *testing.T) {
	if len(buildChain(nil)) != 0 {
		t.Fatal("empty thresholds -> empty chain")
	}
}
