package asset

import (
	"testing"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestAssetToMap_IncludesSensitiveKeys(t *testing.T) {
	cost := "1500000.00"
	m := assetToMap(sqlc.AssetAsset{ID: uuid.New(), Name: "Laptop", AssetTag: "JKT01-ELK-2026-00001", PurchaseCost: &cost})
	if m["name"] != "Laptop" {
		t.Fatalf("name missing")
	}
	if m["asset_tag"] != "JKT01-ELK-2026-00001" {
		t.Fatalf("tag missing")
	}
	if _, ok := m["purchase_cost"]; !ok {
		t.Fatalf("purchase_cost must be present pre-mask")
	}
}
