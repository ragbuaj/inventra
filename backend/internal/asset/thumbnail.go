package asset

import (
	"bytes"
	"image"
	_ "image/jpeg" // register jpeg decoder
	_ "image/png"  // register png decoder

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register webp decoder
)

const thumbMaxDim = 300

// makeThumbnail decodes an image (jpeg/png/webp) and returns a JPEG thumbnail
// fitted within thumbMaxDim x thumbMaxDim. Returns an error for undecodable input.
func makeThumbnail(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	thumb := imaging.Fit(img, thumbMaxDim, thumbMaxDim, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(80)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
