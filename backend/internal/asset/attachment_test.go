package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

func TestAllowedMIMEAndKind(t *testing.T) {
	for _, m := range []string{"image/jpeg", "image/png", "image/webp", "application/pdf"} {
		if !allowedMIME(m) {
			t.Errorf("%s should be allowed", m)
		}
	}
	if allowedMIME("application/zip") {
		t.Error("zip should be rejected")
	}
	if kindFor("image/png") != sqlc.SharedAttachmentKindPhoto {
		t.Error("image -> photo")
	}
	if kindFor("application/pdf") != sqlc.SharedAttachmentKindDocument {
		t.Error("pdf -> document")
	}
	if extFor("image/jpeg") != "jpg" || extFor("application/pdf") != "pdf" {
		t.Error("ext mapping")
	}
}

func TestUploadAttachment_RejectsTypeAndSize(t *testing.T) {
	// q=nil intentional: validation must fire BEFORE any DB/storage call.
	s := NewService(nil, nil, storage.NewFake(), 10, "")
	ctx := context.Background()

	_, err := s.UploadAttachment(ctx, UploadInput{
		AssetID:     uuid.New(),
		ContentType: "application/zip",
		Data:        []byte("x"),
	})
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("want ErrUnsupportedType, got %v", err)
	}

	_, err = s.UploadAttachment(ctx, UploadInput{
		AssetID:     uuid.New(),
		ContentType: "application/pdf",
		Data:        make([]byte, 11),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}

func TestUploadAttachment_RollbackOnDBError(t *testing.T) {
	// When Put fails the error is propagated and no object should remain in storage.
	f := storage.NewFake()
	f.PutErr = errors.New("boom")
	s := NewService(nil, nil, f, 1024, "")

	_, err := s.UploadAttachment(context.Background(), UploadInput{
		AssetID:     uuid.New(),
		ContentType: "application/pdf",
		Data:        []byte("pdf"),
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want put error, got %v", err)
	}
	if len(f.ObjsKeys()) != 0 {
		t.Fatalf("no object should remain, got keys: %v", f.ObjsKeys())
	}
}
