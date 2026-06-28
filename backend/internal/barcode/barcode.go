// Package barcode renders Code128 barcodes and QR codes to PNG.
package barcode

import (
	"bytes"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
)

// EncodeCode128 returns a PNG of a Code128 barcode for s.
func EncodeCode128(s string) ([]byte, error) {
	bc, err := code128.Encode(s)
	if err != nil {
		return nil, err
	}
	scaled, err := barcode.Scale(bc, 600, 120)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeQR returns a PNG of a QR code for s.
func EncodeQR(s string) ([]byte, error) {
	bc, err := qr.Encode(s, qr.M, qr.Auto)
	if err != nil {
		return nil, err
	}
	scaled, err := barcode.Scale(bc, 300, 300)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
