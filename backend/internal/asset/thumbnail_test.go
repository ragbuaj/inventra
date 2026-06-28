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
