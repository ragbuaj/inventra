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

// photoMaxDim bounds the longest edge of a normalized full-size photo. 2048px
// keeps a field photo sharp on mobile and web while cutting a multi-megapixel
// camera capture down to a few hundred KB. It is deliberately larger than the
// mobile client's own 1600px cap, so a mobile upload is only re-encoded (EXIF
// stripped), not downscaled; larger uploads from other clients get bounded.
const photoMaxDim = 2048

// photoJPEGQuality trades a little size for near-transparent quality. 85 is the
// same setting used for avatars.
const photoJPEGQuality = 85

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

// normalizeImage decodes a photo (jpeg/png/webp), fits it within photoMaxDim on
// the longest edge (never upscaling), and re-encodes it as JPEG. Re-encoding
// discards all EXIF metadata (including GPS) the original carried and shrinks
// storage; quality is preserved via a high JPEG quality factor. Returns an
// error for undecodable input so a mislabeled file is rejected, not stored.
func normalizeImage(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	fitted := imaging.Fit(img, photoMaxDim, photoMaxDim, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, fitted, imaging.JPEG, imaging.JPEGQuality(photoJPEGQuality)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
