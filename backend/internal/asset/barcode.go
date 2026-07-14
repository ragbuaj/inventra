package asset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/disintegration/imaging"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	bc "github.com/ragbuaj/inventra/internal/barcode"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/pdfutil"
)

var (
	ErrNoAssets      = errors.New("no assets selected for labels")
	ErrUnknownSize   = errors.New("unknown label size preset")
	ErrSheetOverflow = errors.New("label columns overflow A4 page width — reduce columns or label width")
)

const (
	defaultCompanyName = "PT Bank Tabungan Negara (Persero) Tbk"
	defaultDisclaimer  = "Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung"
)

var sizePresets = map[string][2]float64{
	"60x24":  {60, 24},
	"50x30":  {50, 30},
	"70x40":  {70, 40},
	"100x50": {100, 50},
}

type labelItem struct{ Tag, Name, OfficeCode, CategoryName, Year string }

type labelOpts struct {
	Template, Layout        string
	LabelW, LabelH          float64
	MediaW                  float64
	Columns                 int
	Mode                    string
	ShowName, ShowOffice    bool
	CompanyName, Disclaimer string
	LogoPNG                 []byte
}

func resolveLabelDims(size string, wMM, hMM, mediaWMM float64) (labelW, labelH, mediaW float64, err error) {
	labelW, labelH = 60, 24
	if size != "" {
		p, ok := sizePresets[size]
		if !ok {
			return 0, 0, 0, ErrUnknownSize
		}
		labelW, labelH = p[0], p[1]
	}
	if wMM > 0 {
		labelW = wMM
	}
	if hMM > 0 {
		labelH = hMM
	}
	mediaW = 64
	if mediaWMM > 0 {
		mediaW = mediaWMM
	}
	if mediaW < labelW {
		mediaW = labelW
	}
	return labelW, labelH, mediaW, nil
}

// composeQRWithLogo returns a PNG QR of tag with logoPNG centered (~22% of size). nil logo → plain QR.
func composeQRWithLogo(tag string, logoPNG []byte) ([]byte, error) {
	qrImg, err := bc.EncodeQRHighEC(tag)
	if err != nil {
		return nil, err
	}
	if len(logoPNG) > 0 {
		logo, derr := png.Decode(bytes.NewReader(logoPNG))
		if derr == nil {
			b := qrImg.Bounds()
			side := b.Dx() * 22 / 100
			logoR := imaging.Resize(logo, side, side, imaging.Lanczos)
			canvas := imaging.Clone(qrImg)
			off := image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2)
			qrImg = imaging.Overlay(canvas, logoR, off, 1.0)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, qrImg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sheetFits reports whether cols labels of width labelW mm fit across A4 (210 mm)
// with the layout's fixed margins (8 mm each side) and gutters (3 mm between cols).
func sheetFits(cols int, labelW float64) bool {
	const pageW, margin, gutter = 210.0, 8.0, 3.0
	return float64(cols)*labelW+float64(cols-1)*gutter+2*margin <= pageW
}

func renderLabelPDF(items []labelItem, opts labelOpts) ([]byte, error) {
	if len(items) == 0 {
		return nil, ErrNoAssets
	}
	var pdf *fpdf.Fpdf
	if opts.Layout == "sheet" {
		pdf = pdfutil.NewUTF8PDF("P", "mm", "A4")
	} else {
		pdf = fpdf.NewCustom(&fpdf.InitType{UnitStr: "mm", Size: fpdf.SizeType{Wd: opts.MediaW, Ht: opts.LabelH}})
		pdfutil.RegisterFonts(pdf)
	}
	pdf.SetFont(pdfutil.FontFamily, "", 6)

	drawBTN := func(x, y float64, it labelItem) error {
		pad := 1.0
		qrSide := opts.LabelH - 2*pad
		// QR (left)
		qrPNG, err := composeQRWithLogo(it.Tag, opts.LogoPNG)
		if err != nil {
			return err
		}
		name := fmt.Sprintf("qr-%s-%.1f-%.1f", it.Tag, x, y)
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))
		pdf.ImageOptions(name, x+pad, y+pad, qrSide, qrSide, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		// right column
		rx := x + pad + qrSide + 1.5
		rw := opts.LabelW - (rx - x) - pad
		ry := y + pad
		pdf.SetFont(pdfutil.FontFamily, "B", 5)
		pdf.SetXY(rx, ry)
		pdf.CellFormat(rw, 2.4, trunc(opts.CompanyName, 40), "", 0, "L", false, 0, "")
		ry += 2.6
		pdf.SetFont(pdfutil.FontFamily, "", 5)
		pdf.SetXY(rx, ry)
		pdf.CellFormat(rw, 2.4, it.Tag, "", 0, "L", false, 0, "")
		ry += 2.4
		pdf.Line(rx, ry, rx+rw, ry)
		ry += 0.6
		pdf.SetFont(pdfutil.FontFamily, "B", 6)
		pdf.SetXY(rx, ry)
		pdf.CellFormat(rw/2, 2.8, it.OfficeCode, "", 0, "L", false, 0, "")
		pdf.SetXY(rx+rw/2, ry)
		pdf.CellFormat(rw/2, 2.8, "TP: "+it.Year, "", 0, "R", false, 0, "")
		ry += 2.9
		pdf.SetFont(pdfutil.FontFamily, "", 5)
		pdf.SetXY(rx, ry)
		pdf.CellFormat(rw, 2.4, trunc(it.CategoryName, 38), "", 0, "L", false, 0, "")
		ry += 2.4
		pdf.SetXY(rx, ry)
		pdf.CellFormat(rw, 2.4, trunc(it.Name, 38), "", 0, "L", false, 0, "")
		ry += 2.6
		pdf.SetTextColor(200, 0, 0)
		pdf.SetFont(pdfutil.FontFamily, "", 3.5)
		pdf.SetXY(rx, ry)
		pdf.MultiCell(rw, 1.8, opts.Disclaimer, "", "L", false)
		pdf.SetTextColor(0, 0, 0)
		return nil
	}

	drawGeneric := func(x, y float64, it labelItem) error {
		pad := 1.5
		cx, cy := x+pad, y+pad
		innerW := opts.LabelW - 2*pad
		if opts.ShowName && it.Name != "" {
			pdf.SetXY(cx, cy)
			pdf.CellFormat(innerW, 3, trunc(it.Name, 40), "", 0, "L", false, 0, "")
			cy += 3.2
		}
		imgH := opts.LabelH - (cy - y) - 4
		if imgH < 6 {
			imgH = 6
		}
		place := func(enc func(string) ([]byte, error), ix, iw float64) error {
			img, e := enc(it.Tag)
			if e != nil {
				return e
			}
			n := fmt.Sprintf("g-%s-%.1f-%.1f", it.Tag, ix, cy)
			pdf.RegisterImageOptionsReader(n, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(img))
			pdf.ImageOptions(n, ix, cy, iw, imgH, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			return nil
		}
		switch opts.Mode {
		case "qr":
			if err := place(bc.EncodeQR, cx, imgH); err != nil {
				return err
			}
		case "both":
			if err := place(bc.EncodeQR, cx, imgH); err != nil {
				return err
			}
			if err := place(bc.EncodeCode128, cx+imgH+1.5, innerW-imgH-1.5); err != nil {
				return err
			}
		default:
			if err := place(bc.EncodeCode128, cx, innerW); err != nil {
				return err
			}
		}
		pdf.SetXY(cx, y+opts.LabelH-3.5)
		pdf.CellFormat(innerW, 3, it.Tag, "", 0, "C", false, 0, "")
		if opts.ShowOffice && it.OfficeCode != "" {
			pdf.SetXY(cx, y+opts.LabelH-7)
			pdf.CellFormat(innerW, 3, it.OfficeCode, "", 0, "C", false, 0, "")
		}
		return nil
	}

	draw := drawGeneric
	if opts.Template == "btn" {
		draw = drawBTN
	}

	if opts.Layout == "sheet" {
		const pageW, pageH = 210.0, 297.0
		margin, gutter := 8.0, 3.0
		cols := opts.Columns
		if cols < 1 {
			cols = 3
		}
		if !sheetFits(cols, opts.LabelW) {
			return nil, ErrSheetOverflow
		}
		cellW, cellH := opts.LabelW, opts.LabelH
		rows := int((pageH - 2*margin + gutter) / (cellH + gutter))
		if rows < 1 {
			rows = 1
		}
		perPage := cols * rows
		for i, it := range items {
			if i%perPage == 0 {
				pdf.AddPage()
			}
			slot := i % perPage
			r, cc := slot/cols, slot%cols
			x := margin + float64(cc)*(cellW+gutter)
			y := margin + float64(r)*(cellH+gutter)
			if err := draw(x, y, it); err != nil {
				return nil, err
			}
		}
	} else {
		left := (opts.MediaW - opts.LabelW) / 2
		for _, it := range items {
			pdf.AddPage()
			if err := draw(left, 0, it); err != nil {
				return nil, err
			}
		}
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func toLabelItem(tag, name, officeCode, categoryName string, d pgtype.Date) labelItem {
	year := ""
	if d.Valid {
		year = fmt.Sprintf("%d", d.Time.Year())
	}
	return labelItem{Tag: tag, Name: name, OfficeCode: officeCode, CategoryName: categoryName, Year: year}
}

func (s *Service) labelSettings(ctx context.Context) (string, string) {
	company := defaultCompanyName
	if v, err := s.q.GetAppSetting(ctx, "label.company_name"); err == nil && v != "" {
		company = v
	}
	disc := defaultDisclaimer
	if v, err := s.q.GetAppSetting(ctx, "label.disclaimer"); err == nil && v != "" {
		disc = v
	}
	return company, disc
}

func (s *Service) loadLogo() []byte {
	if s.logoPath == "" {
		return nil
	}
	b, err := os.ReadFile(s.logoPath)
	if err != nil {
		return nil
	}
	return b
}

// LabelInput carries the asset selection + rendering options for BuildLabelPDF.
type LabelInput struct {
	AssetIDs []uuid.UUID
	Tags     []string
	Opts     labelOpts
}

// BuildLabelPDF resolves assets (by ID or tag), enforces scope, and renders the label PDF.
func (s *Service) BuildLabelPDF(ctx context.Context, in LabelInput, all bool, officeIDs []uuid.UUID) ([]byte, error) {
	type row struct {
		it       labelItem
		officeID uuid.UUID
	}
	var rows []row

	resolve := func(a sqlc.AssetAsset, lbl labelItem) {
		rows = append(rows, row{it: lbl, officeID: a.OfficeID})
	}

	if len(in.AssetIDs) > 0 {
		for _, id := range in.AssetIDs {
			a, err := s.q.GetAsset(ctx, id)
			if err != nil {
				return nil, mapDBError(err)
			}
			l, err := s.q.GetAssetLabelByID(ctx, id)
			if err != nil {
				return nil, mapDBError(err)
			}
			resolve(a, toLabelItem(l.AssetTag, l.Name, l.OfficeCode, l.CategoryName, l.PurchaseDate))
		}
	} else {
		for _, tag := range in.Tags {
			a, err := s.q.GetAssetByTag(ctx, tag)
			if err != nil {
				return nil, mapDBError(err)
			}
			l, err := s.q.GetAssetLabelByTag(ctx, tag)
			if err != nil {
				return nil, mapDBError(err)
			}
			resolve(a, toLabelItem(l.AssetTag, l.Name, l.OfficeCode, l.CategoryName, l.PurchaseDate))
		}
	}

	if len(rows) == 0 {
		return nil, ErrNoAssets
	}

	items := make([]labelItem, 0, len(rows))
	for _, r := range rows {
		if !common.InScope(all, officeIDs, r.officeID) {
			return nil, common.ErrForbidden
		}
		items = append(items, r.it)
	}

	// Populate settings + logo into opts
	in.Opts.CompanyName, in.Opts.Disclaimer = s.labelSettings(ctx)
	in.Opts.LogoPNG = s.loadLogo()

	return renderLabelPDF(items, in.Opts)
}

// GetByTag fetches an asset by its unique asset_tag (for scan lookup).
func (s *Service) GetByTag(ctx context.Context, tag string) (sqlc.AssetAsset, error) {
	a, err := s.q.GetAssetByTag(ctx, tag)
	return a, mapDBError(err)
}
