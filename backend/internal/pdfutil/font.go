// Package pdfutil provides a shared PDF constructor that embeds a Unicode
// (DejaVu) font, so generated PDFs render UTF-8 text (middle dots, em dashes,
// accented names, etc.) correctly instead of falling back to core-font
// cp1252 mojibake.
package pdfutil

import (
	"embed"

	"github.com/go-pdf/fpdf"
)

// FontFamily is the family name registered for the embedded DejaVu Unicode
// font. Pass it to Fpdf.SetFont after constructing a PDF via NewUTF8PDF (or
// after calling RegisterFonts directly).
const FontFamily = "dejavu"

//go:embed fonts/DejaVuSansCondensed.ttf fonts/DejaVuSansCondensed-Bold.ttf fonts/DejaVuSansCondensed-Oblique.ttf fonts/DejaVuSansCondensed-BoldOblique.ttf
var fontFS embed.FS

// RegisterFonts registers all four DejaVuSansCondensed styles (regular,
// bold, oblique, bold-oblique) on pdf under FontFamily, covering every
// SetFont style ("", "B", "I", "BI") used by the report generators.
func RegisterFonts(pdf *fpdf.Fpdf) {
	registerStyle(pdf, "", "fonts/DejaVuSansCondensed.ttf")
	registerStyle(pdf, "B", "fonts/DejaVuSansCondensed-Bold.ttf")
	registerStyle(pdf, "I", "fonts/DejaVuSansCondensed-Oblique.ttf")
	registerStyle(pdf, "BI", "fonts/DejaVuSansCondensed-BoldOblique.ttf")
}

func registerStyle(pdf *fpdf.Fpdf, style, path string) {
	data, err := fontFS.ReadFile(path)
	if err != nil {
		// The font files are embedded at build time; a read failure here
		// means the embed itself is broken, which is a programmer error,
		// not a runtime condition callers can recover from.
		panic("pdfutil: failed to read embedded font " + path + ": " + err.Error())
	}
	pdf.AddUTF8FontFromBytes(FontFamily, style, data)
}

// NewUTF8PDF constructs a new Fpdf document with the embedded DejaVu Unicode
// font already registered under FontFamily, ready for
// pdf.SetFont(FontFamily, style, size).
func NewUTF8PDF(orientation, unit, size string) *fpdf.Fpdf {
	pdf := fpdf.New(orientation, unit, size, "")
	RegisterFonts(pdf)
	return pdf
}
