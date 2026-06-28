# Asset Barcode / QR + Label PDF Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give assets scannable Code128/QR codes from their `asset_tag`, a tag→asset scan-lookup endpoint, and print-ready PDF labels (roll page-per-label for the Epson C4050, or A4 grid sheet).

**Architecture:** New pure `internal/barcode` package (Code128/QR → PNG via boombuler/barcode). The existing `internal/asset` module gains a `GetByTag` query+service, a scan-lookup handler, a barcode-PNG handler, and a label-PDF builder (pure `renderLabelPDF` via go-pdf/fpdf + a scope-checked `BuildLabelPDF` service wrapper) exposed at `POST /assets/labels`. All endpoints reuse the asset module's auth + office-scope helpers.

**Tech Stack:** Go 1.25, Gin, pgx/v5, sqlc, `github.com/boombuler/barcode`, `github.com/go-pdf/fpdf`, testify + testcontainers-go.

## Global Constraints

- Go module `github.com/ragbuaj/inventra`; backend commands run from `backend/`.
- Dependencies: `github.com/boombuler/barcode` (Code128 + QR), `github.com/go-pdf/fpdf` (label PDF). Add via `go get`.
- Backend-only slice — do NOT modify `frontend/`. Frontend `label.vue` wiring is a separate item.
- On-the-fly generation; NO MinIO caching (deterministic from `asset_tag`).
- Intangible assets are NOT blocked (every asset has a tag).
- C4050 defaults: roll layout, label 60×24 mm, media width 64 mm; label centered (margin (media−label)/2). Apply when the request omits sizing.
- Two layouts via `layout` param: `roll` (page = media_w × label_h, one page per asset) and `sheet` (A4 210×297 with `columns` grid).
- Authz: all three endpoints `authMW` + `asset.view` + office scope on the target asset(s). by-tag out-of-scope/unknown → 404; barcode out-of-scope → 403; labels any-out-of-scope → 403. No new permission keys.
- Reads filter `deleted_at IS NULL`; never hand-edit `db/sqlc/`. Money/numeric → Go string (n/a here).
- File-serve responses set `X-Content-Type-Options: nosniff`; PDF uses the sanitized `contentDisposition` helper already in the asset module.
- Conventional Commits: `feat(barcode):`, `feat(asset):`, `feat(db):`, `docs(api):`. No Claude/AI co-author trailers.
- Reference spec: `docs/superpowers/specs/2026-06-28-asset-barcode-design.md`.

---

## File Structure

- `backend/go.mod`/`go.sum` — add boombuler/barcode, go-pdf/fpdf.
- `backend/internal/barcode/barcode.go` — `EncodeCode128`, `EncodeQR` (PNG). `barcode_test.go`.
- `backend/db/queries/assets.sql` (+ generated) — `GetAssetByTag`.
- `backend/internal/asset/barcode.go` — `Service.GetByTag`, `renderLabelPDF` (pure), `BuildLabelPDF` (scoped), `LabelInput`, `labelItem`, size resolution, sentinels.
- `backend/internal/asset/barcode_handler.go` — `getByTag`, `getBarcode`, `generateLabels` + `LabelRequest` DTO.
- `backend/internal/asset/routes.go` (extend) — three routes.
- `backend/internal/asset/barcode_test.go` — unit tests (size resolution, renderLabelPDF page counts, DTO validation).
- `backend/internal/asset/barcode_integration_test.go` (`//go:build integration`).
- `backend/api/openapi.yaml`; `docs/PROGRESS.md`.

No `internal/server/router.go` change needed — the asset `Handler` gains no new dependencies; routes are added inside the existing `asset.RegisterRoutes`.

---

## Task 1: `internal/barcode` package (Code128 + QR → PNG)

**Files:**
- Create: `backend/internal/barcode/barcode.go`, `backend/internal/barcode/barcode_test.go`
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Produces: `func EncodeCode128(s string) ([]byte, error)`; `func EncodeQR(s string) ([]byte, error)` (both PNG bytes).

- [ ] **Step 1: Add dependency**

Run:
```bash
cd backend
go get github.com/boombuler/barcode@latest
```
Expected: go.mod/go.sum updated.

- [ ] **Step 2: Write the failing test**

```go
package barcode

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestEncodeCode128_DecodablePNG(t *testing.T) {
	out, err := EncodeCode128("JKT01-ELK-2026-00001")
	if err != nil { t.Fatalf("err: %v", err) }
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil { t.Fatalf("not a decodable image: %v", err) }
	if format != "png" { t.Fatalf("want png, got %s", format) }
	if img.Bounds().Dx() < 100 || img.Bounds().Dy() < 20 {
		t.Fatalf("barcode too small: %v", img.Bounds())
	}
	_ = png.Encode // ensure png import used
}

func TestEncodeQR_DecodablePNG(t *testing.T) {
	out, err := EncodeQR("JKT01-ELK-2026-00001")
	if err != nil { t.Fatalf("err: %v", err) }
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil { t.Fatalf("not decodable: %v", err) }
	if format != "png" { t.Fatalf("want png, got %s", format) }
	if img.Bounds().Dx() < 100 || img.Bounds().Dy() < 100 {
		t.Fatalf("qr too small: %v", img.Bounds())
	}
}

func TestEncodeCode128_Empty(t *testing.T) {
	if _, err := EncodeCode128(""); err == nil {
		t.Fatal("expected error for empty input")
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/barcode/`
Expected: FAIL (`undefined: EncodeCode128`).

- [ ] **Step 4: Implement `barcode.go`**

```go
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
```

> `code128.Encode("")` returns an error (empty), satisfying TestEncodeCode128_Empty. Verify the boombuler import paths after `go get`; adjust `qr.M`/`qr.Auto` constant names if the installed version differs.

- [ ] **Step 5: Run to verify pass + build**

Run: `cd backend && go test ./internal/barcode/ -v && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/barcode/
git commit -m "feat(barcode): Code128 + QR PNG encoders"
```

## Task 2: `GetAssetByTag` query + `Service.GetByTag`

**Files:**
- Modify: `backend/db/queries/assets.sql` (+ generated `db/sqlc/`), `backend/internal/asset/barcode.go` (create)

**Interfaces:**
- Produces: sqlc `GetAssetByTag`; `func (s *Service) GetByTag(ctx context.Context, tag string) (sqlc.AssetAsset, error)`.
- Consumes: `mapDBError`, `ErrNotFound` (existing in the asset package).

- [ ] **Step 1: Add the query**

Append to `backend/db/queries/assets.sql`:
```sql
-- name: GetAssetByTag :one
SELECT * FROM asset.assets WHERE asset_tag = $1 AND deleted_at IS NULL;
```

- [ ] **Step 2: Generate + build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: `db/sqlc/assets.sql.go` defines `GetAssetByTag(ctx, assetTag string) (AssetAsset, error)`. No errors.

- [ ] **Step 3: Create `internal/asset/barcode.go` with `GetByTag`**

```go
package asset

import (
	"context"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// GetByTag fetches an asset by its unique asset_tag (for scan lookup).
func (s *Service) GetByTag(ctx context.Context, tag string) (sqlc.AssetAsset, error) {
	a, err := s.q.GetAssetByTag(ctx, tag)
	return a, mapDBError(err)
}
```

- [ ] **Step 4: Build**

Run: `cd backend && go build ./... && go test ./internal/asset/`
Expected: clean, existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/db/queries/assets.sql backend/db/sqlc/ backend/internal/asset/barcode.go
git commit -m "feat(db): GetAssetByTag query + asset GetByTag service"
```

## Task 3: Scan-lookup + barcode-PNG handlers + routes

**Files:**
- Create: `backend/internal/asset/barcode_handler.go`
- Modify: `backend/internal/asset/routes.go`

**Interfaces:**
- Consumes: `Handler` (svc/fieldSvc/scoped/aud), `resolveAssetInScope` (existing helper from attachments — loads `:id` asset + scope-checks, returns `(assetID, officeID, ok)`), `filterMap`, `h.svc.Get`, `h.svc.GetByTag`, `common.InScope/CallerOfficeScope/WriteError/ErrForbidden`, `barcode.EncodeCode128/EncodeQR`, `assetToMap`.
- Produces: handler methods `getByTag`, `getBarcode`; extended `RegisterRoutes`.

- [ ] **Step 1: Implement `barcode_handler.go`**

```go
package asset

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/barcode"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// getByTag: GET /assets/by-tag/:tag — scan lookup. Out-of-scope/unknown → 404.
func (h *Handler) getByTag(c *gin.Context) {
	tag := c.Param("tag")
	a, err := h.svc.GetByTag(c.Request.Context(), tag)
	if err != nil { h.svcError(c, err); return } // ErrNotFound → 404
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil { common.WriteError(c, err); return }
	if !common.InScope(all, ids, a.OfficeID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"}); return
	}
	masked, err := h.filterMap(c, assetToMap(a))
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"}); return }
	c.JSON(http.StatusOK, masked)
}

// getBarcode: GET /assets/:id/barcode?type=code128|qr — PNG. Out-of-scope → 403.
func (h *Handler) getBarcode(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	a, err := h.svc.Get(c.Request.Context(), id)
	if err != nil { h.svcError(c, err); return }
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil { common.WriteError(c, err); return }
	if !common.InScope(all, ids, a.OfficeID) { common.WriteError(c, common.ErrForbidden); return }

	typ := c.DefaultQuery("type", "code128")
	var png []byte
	switch typ {
	case "code128":
		png, err = barcode.EncodeCode128(a.AssetTag)
	case "qr":
		png, err = barcode.EncodeQR(a.AssetTag)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be code128 or qr"}); return
	}
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "barcode encode failed"}); return }
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/png", png)
}
```

> Reconcile against the asset module: confirm the error-router method name (`h.svcError` vs `svcError`) used by the existing handlers, the `scopeModule` constant (`"assets"`), `filterMap` signature `(c, map) (map, error)`, `assetToMap`, and `common.ErrForbidden`. Use whatever the module already defines (don't introduce a second variant).

- [ ] **Step 2: Extend `routes.go`**

Add to `RegisterRoutes`, inside the `g := rg.Group("/assets")` block (BEFORE the `/:id` routes is fine; gin matches static `by-tag`/`labels` before the `:id` param):
```go
	g.GET("/by-tag/:tag", authMW, requireView, h.getByTag)
	g.GET("/:id/barcode", authMW, requireView, h.getBarcode)
	// (Task 5 adds: g.POST("/labels", authMW, requireView, h.generateLabels))
```

- [ ] **Step 3: Build + vet + existing tests**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/asset/`
Expected: clean. If gin panics at route registration about `:id` vs `by-tag` wildcard conflict, that surfaces when the integration test (Task 7) boots the router — but you can also write a 3-line throwaway that calls `gin.New()` + `RegisterRoutes` and run it; if it panics, restructure per the spec's fallback (move scan-lookup to `/assets/lookup?tag=`). Report if so.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/asset/barcode_handler.go backend/internal/asset/routes.go
git commit -m "feat(asset): scan-lookup + barcode PNG endpoints"
```

## Task 4: Label PDF builder (size resolution + renderLabelPDF + BuildLabelPDF)

**Files:**
- Modify: `backend/internal/asset/barcode.go`, `backend/go.mod`/`go.sum`
- Test: `backend/internal/asset/barcode_test.go` (create)

**Interfaces:**
- Consumes: `barcode.EncodeCode128/EncodeQR`, `s.q.GetAsset`/`GetByTag`, scope helpers, `go-pdf/fpdf`.
- Produces:
  - `type labelItem struct { Tag, Name, Office string }`
  - `type labelOpts struct { Layout string; LabelW, LabelH, MediaW float64; Columns int; Mode string; ShowName, ShowOffice bool }`
  - `func resolveLabelDims(size string, wMM, hMM, mediaWMM float64) (labelW, labelH, mediaW float64, err error)`
  - `func renderLabelPDF(items []labelItem, opts labelOpts) ([]byte, error)` (PURE — no DB)
  - `type LabelInput struct { AssetIDs []uuid.UUID; Tags []string; Opts labelOpts }`
  - `func (s *Service) BuildLabelPDF(ctx, in LabelInput, all bool, officeIDs []uuid.UUID) ([]byte, error)`
  - sentinels `ErrNoAssets`, `ErrUnknownSize`.

- [ ] **Step 1: Add dependency**

Run: `cd backend && go get github.com/go-pdf/fpdf@latest`
Expected: go.mod/go.sum updated.

- [ ] **Step 2: Write the failing tests (size resolution + page counts)**

Append to `backend/internal/asset/barcode_test.go`:
```go
package asset

import (
	"bytes"
	"testing"
)

func TestResolveLabelDims_Defaults(t *testing.T) {
	w, h, media, err := resolveLabelDims("", 0, 0, 0)
	if err != nil { t.Fatal(err) }
	if w != 60 || h != 24 || media != 64 { t.Fatalf("got %v %v %v", w, h, media) }
}

func TestResolveLabelDims_PresetAndExplicit(t *testing.T) {
	w, h, _, err := resolveLabelDims("50x30", 0, 0, 0)
	if err != nil || w != 50 || h != 30 { t.Fatalf("preset: %v %v %v", w, h, err) }
	w, h, _, err = resolveLabelDims("60x24", 70, 40, 0) // explicit overrides preset
	if err != nil || w != 70 || h != 40 { t.Fatalf("explicit: %v %v %v", w, h, err) }
	if _, _, _, err := resolveLabelDims("bogus", 0, 0, 0); err == nil {
		t.Fatal("unknown preset should error")
	}
}

func TestRenderLabelPDF_Roll_OnePagePerAsset(t *testing.T) {
	items := []labelItem{{Tag: "A-1"}, {Tag: "A-2"}, {Tag: "A-3"}}
	out, err := renderLabelPDF(items, labelOpts{Layout: "roll", LabelW: 60, LabelH: 24, MediaW: 64, Mode: "barcode"})
	if err != nil { t.Fatal(err) }
	if !bytes.HasPrefix(out, []byte("%PDF")) { t.Fatal("not a PDF") }
	if n := pdfPageCount(out); n != 3 { t.Fatalf("roll want 3 pages, got %d", n) }
}

func TestRenderLabelPDF_Sheet_GridPaging(t *testing.T) {
	items := make([]labelItem, 7)
	for i := range items { items[i] = labelItem{Tag: "A"} }
	out, err := renderLabelPDF(items, labelOpts{Layout: "sheet", LabelW: 60, LabelH: 24, Columns: 3, Mode: "barcode"})
	if err != nil { t.Fatal(err) }
	// A4 height 297mm, label 24mm + gutters → many rows fit; 7 labels at 3 cols = 3 rows ⇒ 1 page.
	if n := pdfPageCount(out); n < 1 { t.Fatalf("sheet want >=1 page, got %d", n) }
}

// pdfPageCount counts "/Type /Page" occurrences (not /Pages) in the PDF bytes.
func pdfPageCount(b []byte) int {
	count := 0
	needle := []byte("/Type /Page")
	for i := 0; i+len(needle) <= len(b); i++ {
		if bytes.Equal(b[i:i+len(needle)], needle) {
			// exclude "/Type /Pages"
			if i+len(needle) < len(b) && b[i+len(needle)] == 's' { continue }
			count++
		}
	}
	return count
}
```

> `pdfPageCount` is a heuristic for the test; fpdf emits one `/Type /Page` object per page. If the count is off by the catalog's `/Type /Pages`, the `'s'` guard excludes it. If fpdf's output spacing differs (e.g. `/Type/Page`), adjust the needle to match fpdf's actual output (inspect once).

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run 'ResolveLabelDims|RenderLabelPDF'`
Expected: FAIL (undefined).

- [ ] **Step 4: Implement in `barcode.go`**

```go
import (
	"bytes"
	"errors"
	"fmt"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	bc "github.com/ragbuaj/inventra/internal/barcode"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

var (
	ErrNoAssets   = errors.New("no assets selected for labels")
	ErrUnknownSize = errors.New("unknown label size preset")
)

var sizePresets = map[string][2]float64{
	"60x24":  {60, 24},
	"50x30":  {50, 30},
	"70x40":  {70, 40},
	"100x50": {100, 50},
}

type labelItem struct{ Tag, Name, Office string }

type labelOpts struct {
	Layout            string
	LabelW, LabelH    float64
	MediaW            float64
	Columns           int
	Mode              string // barcode|qr|both
	ShowName, ShowOffice bool
}

func resolveLabelDims(size string, wMM, hMM, mediaWMM float64) (labelW, labelH, mediaW float64, err error) {
	labelW, labelH = 60, 24 // C4050 default
	if size != "" {
		p, ok := sizePresets[size]
		if !ok { return 0, 0, 0, ErrUnknownSize }
		labelW, labelH = p[0], p[1]
	}
	if wMM > 0 { labelW = wMM }
	if hMM > 0 { labelH = hMM }
	mediaW = 64
	if mediaWMM > 0 { mediaW = mediaWMM }
	if mediaW < labelW { mediaW = labelW } // media at least as wide as label
	return labelW, labelH, mediaW, nil
}

func renderLabelPDF(items []labelItem, opts labelOpts) ([]byte, error) {
	if len(items) == 0 { return nil, ErrNoAssets }
	var pdf *fpdf.Fpdf
	if opts.Layout == "sheet" {
		pdf = fpdf.New("P", "mm", "A4", "")
	} else { // roll
		pdf = fpdf.NewCustom(&fpdf.InitType{
			UnitStr: "mm",
			Size:    fpdf.SizeType{Wd: opts.MediaW, Ht: opts.LabelH},
		})
	}
	pdf.SetFont("Helvetica", "", 7)

	draw := func(x, y float64, it labelItem) error {
		// content box is opts.LabelW × opts.LabelH at (x,y)
		pad := 1.5
		cx := x + pad
		cy := y + pad
		innerW := opts.LabelW - 2*pad
		if opts.ShowName && it.Name != "" {
			pdf.SetXY(cx, cy)
			pdf.CellFormat(innerW, 3, it.Name, "", 0, "L", false, 0, "")
			cy += 3.2
		}
		// barcode / qr
		imgH := opts.LabelH - (cy - y) - 4 // leave room for tag text
		if imgH < 6 { imgH = 6 }
		switch opts.Mode {
		case "qr":
			if err := drawImage(pdf, bc.EncodeQR, it.Tag, cx, cy, imgH, imgH); err != nil { return err }
		case "both":
			if err := drawImage(pdf, bc.EncodeQR, it.Tag, cx, cy, imgH, imgH); err != nil { return err }
			if err := drawImage(pdf, bc.EncodeCode128, it.Tag, cx+imgH+1.5, cy, innerW-imgH-1.5, imgH); err != nil { return err }
		default: // barcode
			if err := drawImage(pdf, bc.EncodeCode128, it.Tag, cx, cy, innerW, imgH); err != nil { return err }
		}
		// tag text under the codes
		pdf.SetXY(cx, y+opts.LabelH-3.5)
		pdf.CellFormat(innerW, 3, it.Tag, "", 0, "C", false, 0, "")
		if opts.ShowOffice && it.Office != "" {
			pdf.SetXY(cx, y+opts.LabelH-7)
			pdf.CellFormat(innerW, 3, it.Office, "", 0, "C", false, 0, "")
		}
		return nil
	}

	if opts.Layout == "sheet" {
		const pageW, pageH = 210.0, 297.0
		margin, gutter := 8.0, 3.0
		cols := opts.Columns
		if cols < 2 { cols = 3 }
		cellW := opts.LabelW
		cellH := opts.LabelH
		rows := int((pageH - 2*margin + gutter) / (cellH + gutter))
		if rows < 1 { rows = 1 }
		perPage := cols * rows
		for i, it := range items {
			if i%perPage == 0 { pdf.AddPage() }
			slot := i % perPage
			r, c := slot/cols, slot%cols
			x := margin + float64(c)*(cellW+gutter)
			y := margin + float64(r)*(cellH+gutter)
			if x+cellW > pageW-margin { continue } // skip if wider than page (defensive)
			if err := draw(x, y, it); err != nil { return nil, err }
		}
	} else { // roll: one page per label
		left := (opts.MediaW - opts.LabelW) / 2
		for _, it := range items {
			pdf.AddPage()
			if err := draw(left, 0, it); err != nil { return nil, err }
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil { return nil, err }
	return buf.Bytes(), nil
}

func drawImage(pdf *fpdf.Fpdf, enc func(string) ([]byte, error), tag string, x, y, w, h float64) error {
	png, err := enc(tag)
	if err != nil { return err }
	name := fmt.Sprintf("%s-%.1f-%.1f", tag, x, y) // unique per placement
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
	pdf.ImageOptions(name, x, y, w, h, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	return nil
}

type LabelInput struct {
	AssetIDs []uuid.UUID
	Tags     []string
	Opts     labelOpts
}

func (s *Service) BuildLabelPDF(ctx context.Context, in LabelInput, all bool, officeIDs []uuid.UUID) ([]byte, error) {
	var rows []sqlc.AssetAsset
	if len(in.AssetIDs) > 0 {
		for _, id := range in.AssetIDs {
			a, err := s.q.GetAsset(ctx, id)
			if err != nil { return nil, mapDBError(err) }
			rows = append(rows, a)
		}
	} else {
		for _, tag := range in.Tags {
			a, err := s.q.GetAssetByTag(ctx, tag)
			if err != nil { return nil, mapDBError(err) }
			rows = append(rows, a)
		}
	}
	if len(rows) == 0 { return nil, ErrNoAssets }
	items := make([]labelItem, 0, len(rows))
	for _, a := range rows {
		if !common.InScope(all, officeIDs, a.OfficeID) { return nil, common.ErrForbidden }
		items = append(items, labelItem{Tag: a.AssetTag, Name: a.Name /* Office filled by handler if needed */})
	}
	return renderLabelPDF(items, in.Opts)
}
```

> Reconcile: `context`, `common` imports; `common.InScope`/`common.ErrForbidden`; `mapDBError`. `it.Office` is left blank here (the asset row stores `office_id`, not name) — if `ShowOffice` is requested, resolving the office name needs a lookup; for this slice keep Office empty and have the handler set `ShowOffice=false` unless a cheap office-code is available on the row. (Document this limitation; office name on the label is a nice-to-have.) Verify `fpdf` symbol names (`fpdf.New`, `fpdf.NewCustom`, `fpdf.InitType`, `fpdf.SizeType`, `RegisterImageOptionsReader`, `ImageOptions`, `CellFormat`, `Output`) against the installed version.

- [ ] **Step 5: Run tests + build**

Run: `cd backend && go test ./internal/asset/ -run 'ResolveLabelDims|RenderLabelPDF' -v && go build ./...`
Expected: PASS. If `pdfPageCount` mismatches fpdf's output, inspect the raw bytes once and fix the needle. Adjust if the page-count heuristic needs tweaking, but keep the assertion meaningful (exact page count for roll).

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/asset/barcode.go backend/internal/asset/barcode_test.go
git commit -m "feat(asset): label PDF builder (roll page-per-label + A4 sheet grid)"
```

## Task 5: Label endpoint (DTO + handler + route)

**Files:**
- Modify: `backend/internal/asset/barcode_handler.go`, `backend/internal/asset/routes.go`
- Test: `backend/internal/asset/barcode_test.go` (append DTO validation test)

**Interfaces:**
- Consumes: `BuildLabelPDF`, `resolveLabelDims`, `labelOpts`, `LabelInput`, scope helpers, `contentDisposition` (existing in attachment_handler.go).
- Produces: `LabelRequest` DTO + `generateLabels` handler + `POST /assets/labels` route.

- [ ] **Step 1: Write the failing DTO test**

Append to `barcode_test.go`:
```go
func TestLabelRequest_Validate(t *testing.T) {
	r := LabelRequest{} // neither ids nor tags
	if err := r.validate(); err == nil { t.Fatal("expected error when neither ids nor tags given") }
	r = LabelRequest{Tags: []string{"A-1"}, Layout: "weird"}
	if err := r.validate(); err == nil { t.Fatal("invalid layout should error") }
	r = LabelRequest{Tags: []string{"A-1"}, Layout: "roll", Mode: "qr"}
	if err := r.validate(); err != nil { t.Fatalf("valid request errored: %v", err) }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestLabelRequest`
Expected: FAIL.

- [ ] **Step 3: Implement DTO + handler in `barcode_handler.go`**

```go
import (
	"errors"
	"github.com/ragbuaj/inventra/internal/middleware"
)

type LabelRequest struct {
	AssetIDs []string `json:"asset_ids"`
	Tags     []string `json:"tags"`
	Layout   string   `json:"layout"`
	Size     string   `json:"size"`
	WidthMM  float64  `json:"w_mm"`
	HeightMM float64  `json:"h_mm"`
	MediaWMM float64  `json:"media_w_mm"`
	Columns  int      `json:"columns"`
	Mode     string   `json:"mode"`
	Fields   struct {
		Name   bool `json:"name"`
		Office bool `json:"office"`
	} `json:"fields"`
}

func (r LabelRequest) validate() error {
	if len(r.AssetIDs) == 0 && len(r.Tags) == 0 {
		return errors.New("provide asset_ids or tags")
	}
	switch r.Layout {
	case "", "roll", "sheet":
	default:
		return errors.New("layout must be roll or sheet")
	}
	switch r.Mode {
	case "", "barcode", "qr", "both":
	default:
		return errors.New("mode must be barcode, qr, or both")
	}
	return nil
}

func (h *Handler) generateLabels(c *gin.Context) {
	var req LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	if err := req.validate(); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }

	labelW, labelH, mediaW, err := resolveLabelDims(req.Size, req.WidthMM, req.HeightMM, req.MediaWMM)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }

	layout := req.Layout
	if layout == "" { layout = "roll" }
	mode := req.Mode
	if mode == "" { mode = "barcode" }

	in := LabelInput{Opts: labelOpts{
		Layout: layout, LabelW: labelW, LabelH: labelH, MediaW: mediaW,
		Columns: req.Columns, Mode: mode, ShowName: req.Fields.Name, ShowOffice: false,
	}}
	for _, s := range req.AssetIDs {
		id, perr := uuid.Parse(s)
		if perr != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id: " + s}); return }
		in.AssetIDs = append(in.AssetIDs, id)
	}
	in.Tags = req.Tags

	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil { common.WriteError(c, err); return }

	pdf, err := h.svc.BuildLabelPDF(c.Request.Context(), in, all, ids)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAssets): c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, common.ErrForbidden): common.WriteError(c, common.ErrForbidden)
		default: h.svcError(c, err)
		}
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", contentDisposition("labels.pdf")) // reuse sanitized helper
	c.Data(http.StatusOK, "application/pdf", pdf)
}
```

> `contentDisposition` produces `inline; filename=...`; for a PDF download `attachment` is friendlier. If the existing helper hardcodes `inline`, either add an `attachment` variant or set the header directly with a sanitized constant `attachment; filename="labels.pdf"` (the filename here is a constant, so injection is not a concern). Keep `nosniff`.

- [ ] **Step 4: Add the route**

In `routes.go`, add inside the `/assets` group:
```go
	g.POST("/labels", authMW, requireView, h.generateLabels)
```

- [ ] **Step 5: Run tests + build + vet**

Run: `cd backend && go test ./internal/asset/ && go build ./... && go vet ./...`
Expected: PASS, clean.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/asset/barcode_handler.go backend/internal/asset/routes.go backend/internal/asset/barcode_test.go
git commit -m "feat(asset): label PDF endpoint POST /assets/labels"
```

## Task 6: OpenAPI sync

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add paths + schema**

Mirror existing asset path style (security `bearerJWT`, reuse error response components):
- `GET /assets/by-tag/{tag}` — path param `tag`; 200 → the existing `Asset` schema; 401, 404.
- `GET /assets/{id}/barcode` — query `type` (enum code128/qr, default code128); 200 `image/png` (schema type string format binary); 400, 401, 403, 404.
- `POST /assets/labels` — requestBody `LabelRequest` (asset_ids[], tags[], layout enum roll/sheet, size, w_mm, h_mm, media_w_mm, columns, mode enum barcode/qr/both, fields{name,office}); 200 `application/pdf` (string binary); 400, 401, 403, 422.

Add schema `LabelRequest`. Reuse the existing `Asset` schema for by-tag. Confirm component ref names from the existing file.

- [ ] **Step 2: Lint**

Run: `cd /d/portfolio-project/asset-management && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): OpenAPI for barcode/scan-lookup/label endpoints"
```

## Task 7: Integration tests

**Files:**
- Create: `backend/internal/asset/barcode_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, the asset Service/Handler + `RegisterRoutes` via httptest, the seeding helpers used by the existing asset integration tests.

- [ ] **Step 1: Write the suite (real Postgres; httptest router with stub auth + scope)**

Study `backend/internal/asset/integration_test.go` and `attachment_integration_test.go` for the exact harness (NewPostgres, seed office/category/asset, stub-auth middleware setting CtxUserID/CtxRoleID, ScopedDeps, permission middleware) and mirror it. Cover with REAL assertions:
- **Scan lookup**: seed an asset with a known tag → `GET /assets/by-tag/<tag>` → 200, body `asset_tag` matches; unknown tag → 404; out-of-scope caller → 404.
- **Barcode PNG**: `GET /assets/<id>/barcode` → 200 `image/png`, body decodes as PNG; `?type=qr` → 200 PNG; `?type=bad` → 400; out-of-scope → 403.
- **Label PDF**: `POST /assets/labels` `{asset_ids:[id], layout:"roll"}` → 200 `application/pdf`, body starts `%PDF`; `{tags:[tag], layout:"sheet"}` → 200 PDF; an out-of-scope asset in the set → 403; empty `{asset_ids:[],tags:[]}` → 400 (DTO) ; a valid request resolving to a missing asset → 404/422 per the service mapping.

- [ ] **Step 2: Run the suite**

Run: `cd backend && go test -tags=integration ./internal/asset/ -run 'Barcode|ByTag|Label' -v`
Expected: PASS (Docker up). Confirm `go build -tags=integration ./...` and the non-tag `go test ./internal/asset/` still pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/asset/barcode_integration_test.go
git commit -m "test: integration coverage for barcode/scan-lookup/label endpoints"
```

## Task 8: PROGRESS.md + final verification gate

**Files:**
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Run the full gate**

Run:
```bash
cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./internal/asset/
cd /d/portfolio-project/asset-management && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
```
Expected: all green. If anything fails, STOP and report — do not edit PROGRESS to claim success.

- [ ] **Step 2: Update `docs/PROGRESS.md`**

Tick **Barcode/QR** under "Backend — Feature modules" with a one-line note (Code128 + QR PNG from asset_tag; scan-lookup `/assets/by-tag/:tag`; label PDF `/assets/labels` — roll page-per-label default 60×24 on 64mm media for Epson C4050, + A4 sheet grid; scope-gated; integration tests). Refresh the "▶ Next session — start here" block (mark barcode done; point at the next real step — e.g. BAST/asset_documents, or wiring frontend Asset/Label/Approval screens after ADR-0007). Do NOT tick transfer/opname/depreciation/import/frontend-wiring. Note the PR number when merged. Keep existing style.

- [ ] **Step 3: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): barcode/QR + label PDF landed"
```

---

## Self-Review notes (spec coverage)

- Spec §1 barcode package → Task 1. Scan lookup (§2) → Tasks 2+3. Barcode PNG (§3) → Task 3. Label PDF (§4) → Tasks 4+5 (pure `renderLabelPDF` unit-tested for roll page-per-label + sheet grid; `BuildLabelPDF` scope-checks). Module layout (§5) → Tasks 3/4/5. Authz → Tasks 3/5 (by-tag 404, barcode 403, labels 403). Testing → Tasks 1/4/5 unit + Task 7 integration. OpenAPI → Task 6. Gates+PROGRESS → Task 8.
- Decisions: deps (boombuler + go-pdf/fpdf) Tasks 1/4; roll 64×24 defaults via `resolveLabelDims` Task 4; intangible not blocked (no class check anywhere); on-the-fly (no MinIO) — no storage calls.
- Known limitation documented: office NAME on labels deferred (row has office_id, not name) → `ShowOffice` forced false in the handler; revisit if needed.
- Type consistency: `labelOpts`, `labelItem`, `LabelInput`, `resolveLabelDims`, `renderLabelPDF`, `BuildLabelPDF`, `LabelRequest`, `EncodeCode128/EncodeQR` defined once and referenced consistently.
