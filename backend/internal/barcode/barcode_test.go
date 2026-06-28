package barcode

import (
	"bytes"
	"image"
	_ "image/png"
	"testing"
)

func TestEncodeCode128_DecodablePNG(t *testing.T) {
	out, err := EncodeCode128("JKT01-ELK-2026-00001")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("not a decodable image: %v", err)
	}
	if format != "png" {
		t.Fatalf("want png, got %s", format)
	}
	if img.Bounds().Dx() < 100 || img.Bounds().Dy() < 20 {
		t.Fatalf("barcode too small: %v", img.Bounds())
	}
}

func TestEncodeQR_DecodablePNG(t *testing.T) {
	out, err := EncodeQR("JKT01-ELK-2026-00001")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("not decodable: %v", err)
	}
	if format != "png" {
		t.Fatalf("want png, got %s", format)
	}
	if img.Bounds().Dx() < 100 || img.Bounds().Dy() < 100 {
		t.Fatalf("qr too small: %v", img.Bounds())
	}
}

func TestEncodeCode128_Empty(t *testing.T) {
	if _, err := EncodeCode128(""); err == nil {
		t.Fatal("expected error for empty input")
	}
}
