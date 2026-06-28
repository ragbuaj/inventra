package asset

import (
	"testing"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestAttachmentToMap_HidesKeysExposesHasThumbnail(t *testing.T) {
	tk := "assets/x/y_thumb.jpg"
	m := attachmentToMap(sqlc.AssetAssetAttachment{
		ID: uuid.New(), AssetID: uuid.New(), Kind: sqlc.SharedAttachmentKindPhoto,
		ObjectKey: "assets/x/y.jpg", ThumbnailKey: &tk, OriginalFilename: "photo.jpg",
		SizeBytes: 123, MimeType: "image/jpeg",
	})
	if _, ok := m["object_key"]; ok {
		t.Error("object_key must not be exposed")
	}
	if _, ok := m["thumbnail_key"]; ok {
		t.Error("thumbnail_key must not be exposed")
	}
	if m["has_thumbnail"] != true {
		t.Error("has_thumbnail should be true")
	}
	if m["original_filename"] != "photo.jpg" || m["mime_type"] != "image/jpeg" {
		t.Error("metadata missing")
	}
	noThumb := attachmentToMap(sqlc.AssetAssetAttachment{ID: uuid.New(), MimeType: "application/pdf", Kind: sqlc.SharedAttachmentKindDocument})
	if noThumb["has_thumbnail"] != false {
		t.Error("has_thumbnail should be false for nil thumbnail_key")
	}
}

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
