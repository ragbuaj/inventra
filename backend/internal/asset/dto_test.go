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

func TestLocationHistoryToMap(t *testing.T) {
	fid, rid, uid := uuid.New(), uuid.New(), uuid.New()
	fname, rname, uname := "Lantai 3", "Ruang IT", "Admin"
	m := locationHistoryToMap(sqlc.ListAssetLocationHistoryRow{
		ID: uuid.New(), OfficeID: uuid.New(), OfficeName: "Cabang",
		FloorID: &fid, FloorName: &fname, RoomID: &rid, RoomName: &rname,
		Source: sqlc.SharedLocationChangeSourceEdit, MovedByID: &uid, MovedByName: &uname,
	})
	if m["source"] != "edit" {
		t.Fatalf("source = %v", m["source"])
	}
	if m["office_name"] != "Cabang" {
		t.Fatalf("office_name = %v", m["office_name"])
	}
	if got, _ := m["floor_name"].(*string); got == nil || *got != "Lantai 3" {
		t.Fatalf("floor_name = %v", got)
	}
	if got, _ := m["room_name"].(*string); got == nil || *got != "Ruang IT" {
		t.Fatalf("room_name = %v", got)
	}
	// A zero (invalid) timestamptz serializes to a nil *string.
	if got, _ := m["moved_at"].(*string); got != nil {
		t.Fatalf("moved_at should be nil for zero ts, got %v", *got)
	}
}

func TestPICHistoryToMap(t *testing.T) {
	uid := uuid.New()
	uname := "Manager"
	m := picHistoryToMap(sqlc.ListAssetPICHistoryRow{
		ID: uuid.New(), PicEmployeeID: uuid.New(), PicName: "Andi", PicCode: "NIP001",
		AssignedByID: &uid, AssignedByName: &uname,
	})
	if m["pic_name"] != "Andi" || m["pic_code"] != "NIP001" {
		t.Fatalf("pic name/code = %v / %v", m["pic_name"], m["pic_code"])
	}
	if got, _ := m["assigned_by_name"].(*string); got == nil || *got != "Manager" {
		t.Fatalf("assigned_by_name = %v", got)
	}
	// Still-active PIC: released_at (zero ts) serializes to nil.
	if got, _ := m["released_at"].(*string); got != nil {
		t.Fatalf("released_at should be nil for active PIC")
	}
}
