package asset

import (
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestDocumentCreateRequest_toInput_OK(t *testing.T) {
	no := "BAST-2026-001"
	cp := "PT Vendor"
	date := "2026-06-28"
	req := uuid.New().String()
	r := DocumentCreateRequest{
		DocType:          "bast_acquisition",
		DocNo:            &no,
		DocDate:          &date,
		Counterparty:     &cp,
		RelatedRequestID: &req,
	}
	assetID, createdBy := uuid.New(), uuid.New()
	in, err := r.toInput(assetID, createdBy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.AssetID != assetID || in.CreatedBy != createdBy {
		t.Fatal("asset/createdBy not propagated")
	}
	if string(in.DocType) != "bast_acquisition" {
		t.Fatalf("doc_type = %s", in.DocType)
	}
	if !in.DocDate.Valid || in.DocDate.Time.Format("2006-01-02") != "2026-06-28" {
		t.Fatal("doc_date not parsed")
	}
	if in.RelatedRequestID == nil {
		t.Fatal("related_request_id not parsed")
	}
}

func TestDocumentCreateRequest_toInput_BadDate(t *testing.T) {
	bad := "28-06-2026"
	r := DocumentCreateRequest{DocType: "invoice", DocDate: &bad}
	if _, err := r.toInput(uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error for bad date")
	}
}

func TestDocumentCreateRequest_toInput_NilOptionals(t *testing.T) {
	r := DocumentCreateRequest{DocType: "other"}
	in, err := r.toInput(uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.DocNo != nil || in.Counterparty != nil || in.RelatedRequestID != nil || in.DocDate.Valid {
		t.Fatal("optionals should be nil/invalid")
	}
}

func TestDocumentToMap_HidesObjectKeyExposesHasFile(t *testing.T) {
	key := "assets/x/documents/y.pdf"
	no := "BAST-1"
	d := sqlc.AssetAssetDocument{
		ID:        uuid.New(),
		AssetID:   uuid.New(),
		DocType:   "bast_transfer",
		DocNo:     &no,
		ObjectKey: &key,
	}
	m := documentToMap(d)
	if _, ok := m["object_key"]; ok {
		t.Fatal("object_key must not be serialized")
	}
	if m["has_file"] != true {
		t.Fatal("has_file should be true when object_key set")
	}
	if m["doc_type"] != "bast_transfer" || m["doc_no"] != &no {
		t.Fatalf("unexpected map: %v", m)
	}

	d2 := sqlc.AssetAssetDocument{ID: uuid.New(), AssetID: uuid.New(), DocType: "other"}
	if documentToMap(d2)["has_file"] != false {
		t.Fatal("has_file should be false when object_key nil")
	}
}
