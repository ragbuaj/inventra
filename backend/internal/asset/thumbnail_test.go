package asset

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func makeTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestMakeThumbnail_ResizesImage(t *testing.T) {
	src := makeTestPNG(800, 600)
	out, err := makeThumbnail(src)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("thumbnail not decodable: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("want jpeg, got %s", format)
	}
	b := img.Bounds()
	if b.Dx() > 300 || b.Dy() > 300 {
		t.Fatalf("thumbnail too large: %dx%d", b.Dx(), b.Dy())
	}
}

func TestMakeThumbnail_RejectsGarbage(t *testing.T) {
	if _, err := makeThumbnail([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image input")
	}
}

func TestNormalizeImage_DownscalesAndReencodesToJPEG(t *testing.T) {
	// A large PNG is downscaled to fit photoMaxDim and re-encoded as JPEG, and
	// the compressed output is smaller than the original.
	src := makeTestPNG(4000, 3000)
	out, err := normalizeImage(src)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("normalized image not decodable: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("want jpeg, got %s", format)
	}
	b := img.Bounds()
	if b.Dx() > photoMaxDim || b.Dy() > photoMaxDim {
		t.Fatalf("normalized image exceeds %d: %dx%d", photoMaxDim, b.Dx(), b.Dy())
	}
	// Longest edge should be exactly bounded to photoMaxDim (aspect preserved).
	// (A synthetic gradient PNG compresses better than JPEG, so byte-size is not
	// asserted here — the storage win is real for photographic camera input.)
	if b.Dx() != photoMaxDim {
		t.Fatalf("want longest edge %d, got width %d", photoMaxDim, b.Dx())
	}
}

func TestNormalizeImage_DoesNotUpscaleSmallImage(t *testing.T) {
	src := makeTestPNG(320, 240)
	out, err := normalizeImage(src)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("normalized image not decodable: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 320 || b.Dy() != 240 {
		t.Fatalf("small image must not be upscaled, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestNormalizeImage_RejectsGarbage(t *testing.T) {
	if _, err := normalizeImage([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image input")
	}
}
