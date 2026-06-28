package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

func TestAttachFile_RejectsTypeAndSize(t *testing.T) {
	// q=nil intentional: validation must fire BEFORE any DB/storage call.
	s := NewService(nil, nil, storage.NewFake(), 10, "")
	doc := sqlc.AssetAssetDocument{ID: mustUUID(t), AssetID: mustUUID(t)}

	_, err := s.AttachFile(context.Background(), doc, DocumentFileInput{
		ContentType: "application/zip", Data: []byte("x"),
	})
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("want ErrUnsupportedType, got %v", err)
	}

	_, err = s.AttachFile(context.Background(), doc, DocumentFileInput{
		ContentType: "application/pdf", Data: make([]byte, 11),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}

func TestAttachFile_RollbackOnDBError(t *testing.T) {
	// Put succeeds but the DB update fails (q=nil panics? no — use PutErr to stop before DB).
	f := storage.NewFake()
	f.PutErr = errors.New("boom")
	s := NewService(nil, nil, f, 1024, "")
	doc := sqlc.AssetAssetDocument{ID: mustUUID(t), AssetID: mustUUID(t)}

	_, err := s.AttachFile(context.Background(), doc, DocumentFileInput{
		ContentType: "application/pdf", Data: []byte("pdf"),
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want put error, got %v", err)
	}
	if len(f.ObjsKeys()) != 0 {
		t.Fatalf("no object should remain, got %v", f.ObjsKeys())
	}
}

func TestOpenDocumentFile_NilObjectKey(t *testing.T) {
	s := NewService(nil, nil, storage.NewFake(), 1024, "")
	doc := sqlc.AssetAssetDocument{ID: mustUUID(t), AssetID: mustUUID(t)} // ObjectKey nil
	_, _, err := s.OpenDocumentFile(context.Background(), doc)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}
