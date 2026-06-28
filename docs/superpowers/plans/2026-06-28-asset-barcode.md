# Asset Barcode / QR + Label PDF Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give assets scannable Code128/QR codes from their `asset_tag`, a tag→asset scan-lookup endpoint, and print-ready PDF labels — a rich **BTN template** (QR + bank header/logo + asset code + office code + category + asset name + purchase year + disclaimer) plus a **generic** template, in `roll` (page-per-label, Epson C4050) or `sheet` (A4 grid) layout.

**Architecture:** New pure `internal/barcode` package (Code128/QR → PNG via boombuler/barcode). The existing `internal/asset` module gains: `GetByTag` + a label-data join query + an `app_settings` read; a barcode-PNG handler; a scan-lookup handler; and a label-PDF builder (`renderLabelPDF` supporting `btn`/`generic` templates × `roll`/`sheet` layouts via go-pdf/fpdf, with QR-center logo compositing via imaging) exposed at `POST /assets/labels`. All endpoints reuse the asset module's auth + office-scope helpers.

**Tech Stack:** Go 1.25, Gin, pgx/v5, sqlc, `github.com/boombuler/barcode`, `github.com/go-pdf/fpdf`, `github.com/disintegration/imaging` (already present), testify + testcontainers-go.

## Global Constraints

- Go module `github.com/ragbuaj/inventra`; backend commands run from `backend/`.
- Dependencies: `github.com/boombuler/barcode` (Code128 + QR), `github.com/go-pdf/fpdf` (label PDF). Add via `go get`; run `go mod tidy` so they are direct (not `// indirect`).
- Backend-only slice — do NOT modify `frontend/`. Frontend `label.vue` wiring is a separate item.
- On-the-fly generation; NO MinIO caching (deterministic from `asset_tag`).
- Intangible assets are NOT blocked (every asset has a tag). `asset_tag` format is NOT changed.
- **Two templates:** `btn` (default) — fixed layout: QR(left, logo-centered) + company_name/logo header + asset_tag + office_code + `TP:<year>` + category_name + asset_name + red disclaimer; `generic` — original `mode` (barcode/qr/both) + field toggles. Both honor `layout` (roll/sheet) + size.
- **C4050 defaults:** roll layout, label 60×24 mm, media width 64 mm; label centered (margin (media−label)/2). Apply when the request omits sizing.
- **app_settings** holds `label.company_name` (default `PT Bank Tabungan Negara (Persero) Tbk`) and `label.disclaimer` (default `Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung`). Read via `GetAppSetting`; missing value → default constant (not an error).
- **Logo:** config `LABEL_LOGO_PATH` (default `assets/btn-logo.png`); loaded if the file exists, composited to QR center (QR error-correction High) + header; if absent → plain QR, no header logo (must NOT break build/tests/CI).
- Authz: all label/scan/barcode endpoints `authMW` + `asset.view` + office scope. by-tag out-of-scope/unknown → 404; barcode out-of-scope → 403; labels any-out-of-scope → 403. No new permission keys.
- Reads filter `deleted_at IS NULL`; never hand-edit `db/sqlc/`. Migrations: soft-delete/partial-unique/`set_updated_at` conventions; `.down.sql` reverses seeds.
- File-serve responses set `X-Content-Type-Options: nosniff`; PDF uses an `attachment; filename="labels.pdf"` Content-Disposition (sanitized; the filename is a constant).
- Conventional Commits: `feat(barcode):`, `feat(asset):`, `feat(db):`, `docs(api):`. No Claude/AI co-author trailers.
- Reference spec: `docs/superpowers/specs/2026-06-28-asset-barcode-design.md` (incl. the BTN-template REVISION section).

---

## File Structure

- `backend/go.mod`/`go.sum` — add boombuler/barcode, go-pdf/fpdf (tidy → direct).
- `backend/internal/barcode/barcode.go` — `EncodeCode128`, `EncodeQR`, `EncodeQRHighEC` (+ `barcode_test.go`). [Task 1 done for the first two.]
- `backend/db/migrations/000017_label_settings.up.sql`/`.down.sql` — seed `app_settings` label rows.
- `backend/db/queries/assets.sql` (+ generated) — `GetAssetByTag`, `GetAssetLabelByID`, `GetAssetLabelByTag`; `db/queries/identity.sql` (or wherever app_settings lives) — `GetAppSetting`.
- `backend/internal/config/config.go` (+ `.env.example`) — `LabelLogoPath`.
- `backend/internal/asset/barcode.go` — `GetByTag`, label-data resolution, `labelItem`, `labelOpts`, `resolveLabelDims`, `renderLabelPDF` (pure: btn+generic × roll+sheet), `BuildLabelPDF` (scoped), settings/logo loading helpers, sentinels.
- `backend/internal/asset/barcode_handler.go` — `getByTag`, `getBarcode`, `generateLabels` + `LabelRequest` DTO.
- `backend/internal/asset/routes.go` (extend) — three routes.
- `backend/internal/asset/barcode_test.go`, `barcode_integration_test.go` (`//go:build integration`).
- `backend/api/openapi.yaml`; `docs/PROGRESS.md`.

No `internal/server/router.go` change for the asset Handler (gains no new deps). The asset Service gains the `appSettings` reader + logo path; reconcile via NewService if needed (see Task 5).

---

## Task 1: `internal/barcode` package (Code128 + QR → PNG)  ✅ DONE

(Already implemented: `EncodeCode128`/`EncodeQR` PNG via boombuler. Task 5 adds `EncodeQRHighEC` for the
logo-centered QR; that is folded into Task 5's work, not re-run here.)

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

- [ ] **Step 4: Build + existing tests**

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
- Consumes: `Handler` (svc/fieldSvc/scoped/aud), `filterMap`, `h.svc.Get`, `h.svc.GetByTag`, `common.InScope/CallerOfficeScope/WriteError/ErrForbidden`, `barcode.EncodeCode128/EncodeQR`, `assetToMap`, `scopeModule`, the module's error router (`h.svcError` or `svcError` — match what exists).
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

> Reconcile against the asset module: confirm `h.svcError` vs `svcError`, `scopeModule` (`"assets"`), `filterMap(c, map) (map, error)`, `assetToMap`, `common.ErrForbidden`. Use what the module already defines.

- [ ] **Step 2: Extend `routes.go`**

Add inside the `g := rg.Group("/assets")` block:
```go
	g.GET("/by-tag/:tag", authMW, requireView, h.getByTag)
	g.GET("/:id/barcode", authMW, requireView, h.getBarcode)
	// Task 6 adds: g.POST("/labels", authMW, requireView, h.generateLabels)
```

- [ ] **Step 3: Build + vet + existing tests**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/asset/`
Expected: clean. If gin panics on `:id` vs `by-tag`/`labels` wildcard conflict at route registration, that surfaces when the integration test (Task 8) boots the router; you may also do a 3-line throwaway `gin.New()` + `RegisterRoutes` check. If it panics, restructure per the spec fallback (`/assets/lookup?tag=`) and report.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/asset/barcode_handler.go backend/internal/asset/routes.go
git commit -m "feat(asset): scan-lookup + barcode PNG endpoints"
```

## Task 4: Label data — migration 000017 (settings) + app_settings & label-join queries + logo config

**Files:**
- Create: `backend/db/migrations/000017_label_settings.up.sql`/`.down.sql`
- Modify: `backend/db/queries/assets.sql` (+ generated), an app_settings query file (+ generated), `backend/internal/config/config.go`, `backend/.env.example`

**Interfaces:**
- Produces: seeded `app_settings` rows `label.company_name`, `label.disclaimer`; sqlc `GetAppSetting(key) (value, error)`, `GetAssetLabelByID(id)`, `GetAssetLabelByTag(tag)` returning `{AssetTag, Name, OfficeCode, CategoryName, PurchaseDate}`; `Config.LabelLogoPath string`.

- [ ] **Step 1: Inspect app_settings schema**

Read the migration that creates `app_settings` (search `backend/db/migrations/` for `app_settings`) — note the schema (`identity.app_settings`?) and columns (`key`, `value`, `value_type`, `description`, timestamps, deleted_at). Confirm exact table name + columns for the seed + query.

- [ ] **Step 2: Write `000017_label_settings.up.sql`**

```sql
-- Seed label boilerplate (company name + disclaimer) for asset labels.
INSERT INTO identity.app_settings (key, value, value_type, description) VALUES
  ('label.company_name', 'PT Bank Tabungan Negara (Persero) Tbk', 'string', 'Company name printed on asset labels'),
  ('label.disclaimer', 'Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung', 'string', 'Disclaimer text printed on asset labels')
ON CONFLICT (key) DO NOTHING;
```
> Adjust the schema/table name and column list to the actual `app_settings` DDL from Step 1 (e.g. if there is no `value_type`/`description`, drop them; if the unique index is partial `WHERE deleted_at IS NULL`, `ON CONFLICT` may need the constraint/columns — match the real index, or use a `WHERE NOT EXISTS` guard).

`000017_label_settings.down.sql`:
```sql
DELETE FROM identity.app_settings WHERE key IN ('label.company_name', 'label.disclaimer');
```

- [ ] **Step 3: Apply + verify migration**

Run:
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
cd backend && migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: applies clean. Verify the two rows exist (`psql ... -c "select key from identity.app_settings where key like 'label.%'"`). Then `migrate down 1` then `up` to confirm reversible. Leave DB at the latest version.

- [ ] **Step 4: Add queries**

In the app_settings query file (e.g. `db/queries/identity.sql` — put it where other `identity.app_settings` queries live; if none, add to `assets.sql` is acceptable but prefer the identity file):
```sql
-- name: GetAppSetting :one
SELECT value FROM identity.app_settings WHERE key = $1 AND deleted_at IS NULL;
```
In `db/queries/assets.sql`:
```sql
-- name: GetAssetLabelByID :one
SELECT a.asset_tag, a.name, o.code AS office_code, c.name AS category_name, a.purchase_date
FROM asset.assets a
JOIN masterdata.offices o ON o.id = a.office_id
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.id = $1 AND a.deleted_at IS NULL;

-- name: GetAssetLabelByTag :one
SELECT a.asset_tag, a.name, o.code AS office_code, c.name AS category_name, a.purchase_date
FROM asset.assets a
JOIN masterdata.offices o ON o.id = a.office_id
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.asset_tag = $1 AND a.deleted_at IS NULL;
```
> Confirm the real column names for office code (`masterdata.offices.code`) and category name (`masterdata.categories.name`) from their migrations before generating.

- [ ] **Step 5: Generate + config + build**

Run: `cd backend && sqlc generate`. Confirm `GetAppSetting`, `GetAssetLabelByID`, `GetAssetLabelByTag` exist with row types exposing `AssetTag, Name, OfficeCode, CategoryName string` and `PurchaseDate pgtype.Date` (verify nullable office/category code/name types — adjust later usage accordingly).
In `internal/config/config.go` add `LabelLogoPath string` to the struct and in `Load()`:
```go
		LabelLogoPath: getEnv("LABEL_LOGO_PATH", "assets/btn-logo.png"),
```
Add to `.env.example`: `LABEL_LOGO_PATH=assets/btn-logo.png`.
Run `go build ./...`.

- [ ] **Step 6: Commit**

```bash
git add backend/db/migrations/000017_label_settings.up.sql backend/db/migrations/000017_label_settings.down.sql backend/db/queries/ backend/db/sqlc/ backend/internal/config/config.go backend/.env.example
git commit -m "feat(db): label app_settings (000017) + label-data join + GetAppSetting + logo path config"
```

## Task 5: Label PDF builder (btn + generic templates × roll + sheet) + logo compositing

**Files:**
- Modify: `backend/internal/asset/barcode.go`, `backend/internal/barcode/barcode.go`, `backend/go.mod`/`go.sum`, `backend/internal/asset/service.go` (Service gains `logoPath` + settings access)
- Test: `backend/internal/asset/barcode_test.go` (create)

**Interfaces:**
- Consumes: `barcode.EncodeCode128/EncodeQR/EncodeQRHighEC`, `s.q.GetAssetLabelByID/ByTag`, `s.q.GetAppSetting`, scope helpers, `go-pdf/fpdf`, `disintegration/imaging`, `Config.LabelLogoPath`.
- Produces:
  - In `internal/barcode`: `func EncodeQRHighEC(s string) (image.Image, error)` (QR at error-correction High, returns the image for compositing) and keep `EncodeQR` (PNG bytes).
  - `type labelItem struct { Tag, Name, OfficeCode, CategoryName, Year string }`
  - `type labelOpts struct { Template, Layout string; LabelW, LabelH, MediaW float64; Columns int; Mode string; ShowName, ShowOffice bool; CompanyName, Disclaimer string; LogoPNG []byte }`
  - `func resolveLabelDims(size string, wMM, hMM, mediaWMM float64) (labelW, labelH, mediaW float64, err error)`
  - `func renderLabelPDF(items []labelItem, opts labelOpts) ([]byte, error)` (PURE)
  - `func composeQRWithLogo(tag string, logoPNG []byte) ([]byte, error)` (QR-High + center logo via imaging; logoPNG nil → plain QR PNG)
  - `type LabelInput struct { AssetIDs []uuid.UUID; Tags []string; Opts labelOpts }`
  - `func (s *Service) BuildLabelPDF(ctx, in LabelInput, all bool, officeIDs []uuid.UUID) ([]byte, error)`
  - `func (s *Service) labelSettings(ctx) (companyName, disclaimer string)` (reads app_settings, falls back to defaults), `func (s *Service) loadLogo() []byte` (reads `s.logoPath` if present, else nil)
  - sentinels `ErrNoAssets`, `ErrUnknownSize`.

- [ ] **Step 1: Add dependency + EncodeQRHighEC**

Run: `cd backend && go get github.com/go-pdf/fpdf@latest`.
Add to `internal/barcode/barcode.go`:
```go
import "image"
// EncodeQRHighEC returns a QR image at High error-correction (tolerates a center logo overlay).
func EncodeQRHighEC(s string) (image.Image, error) {
	bc, err := qr.Encode(s, qr.H, qr.Auto)
	if err != nil { return nil, err }
	return barcode.Scale(bc, 300, 300)
}
```

- [ ] **Step 2: Write the failing tests (size resolution + page counts + template render)**

Create `backend/internal/asset/barcode_test.go`:
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

func TestResolveLabelDims_PresetExplicitUnknown(t *testing.T) {
	w, h, _, err := resolveLabelDims("50x30", 0, 0, 0)
	if err != nil || w != 50 || h != 30 { t.Fatalf("preset: %v %v %v", w, h, err) }
	w, h, _, _ = resolveLabelDims("60x24", 70, 40, 0)
	if w != 70 || h != 40 { t.Fatalf("explicit override: %v %v", w, h) }
	if _, _, _, err := resolveLabelDims("bogus", 0, 0, 0); err == nil { t.Fatal("unknown preset should error") }
}

func itemsN(n int) []labelItem {
	it := make([]labelItem, n)
	for i := range it { it[i] = labelItem{Tag: "711PK2201600015", Name: "Monitor Samsung", OfficeCode: "711", CategoryName: "Perabot Kantor 2", Year: "2016"} }
	return it
}

func TestRenderLabelPDF_BTN_Roll_OnePagePerAsset(t *testing.T) {
	out, err := renderLabelPDF(itemsN(3), labelOpts{Template: "btn", Layout: "roll", LabelW: 60, LabelH: 24, MediaW: 64, CompanyName: "PT BTN", Disclaimer: "x"})
	if err != nil { t.Fatal(err) }
	if !bytes.HasPrefix(out, []byte("%PDF")) { t.Fatal("not a PDF") }
	if n := pdfPageCount(out); n != 3 { t.Fatalf("roll want 3 pages, got %d", n) }
}

func TestRenderLabelPDF_Generic_Sheet(t *testing.T) {
	out, err := renderLabelPDF(itemsN(7), labelOpts{Template: "generic", Layout: "sheet", LabelW: 60, LabelH: 24, Columns: 3, Mode: "barcode"})
	if err != nil { t.Fatal(err) }
	if !bytes.HasPrefix(out, []byte("%PDF")) { t.Fatal("not a PDF") }
	if n := pdfPageCount(out); n < 1 { t.Fatalf("sheet want >=1 page, got %d", n) }
}

func pdfPageCount(b []byte) int {
	count, needle := 0, []byte("/Type /Page")
	for i := 0; i+len(needle) <= len(b); i++ {
		if bytes.Equal(b[i:i+len(needle)], needle) {
			if i+len(needle) < len(b) && b[i+len(needle)] == 's' { continue } // skip /Type /Pages
			count++
		}
	}
	return count
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run 'ResolveLabelDims|RenderLabelPDF'`
Expected: FAIL (undefined).

- [ ] **Step 4: Implement in `barcode.go`**

```go
import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/disintegration/imaging"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	bc "github.com/ragbuaj/inventra/internal/barcode"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

var (
	ErrNoAssets    = errors.New("no assets selected for labels")
	ErrUnknownSize = errors.New("unknown label size preset")
)

const (
	defaultCompanyName = "PT Bank Tabungan Negara (Persero) Tbk"
	defaultDisclaimer  = "Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung"
)

var sizePresets = map[string][2]float64{ "60x24": {60, 24}, "50x30": {50, 30}, "70x40": {70, 40}, "100x50": {100, 50} }

type labelItem struct{ Tag, Name, OfficeCode, CategoryName, Year string }

type labelOpts struct {
	Template, Layout   string
	LabelW, LabelH     float64
	MediaW             float64
	Columns            int
	Mode               string
	ShowName, ShowOffice bool
	CompanyName, Disclaimer string
	LogoPNG            []byte
}

func resolveLabelDims(size string, wMM, hMM, mediaWMM float64) (labelW, labelH, mediaW float64, err error) {
	labelW, labelH = 60, 24
	if size != "" {
		p, ok := sizePresets[size]; if !ok { return 0, 0, 0, ErrUnknownSize }
		labelW, labelH = p[0], p[1]
	}
	if wMM > 0 { labelW = wMM }
	if hMM > 0 { labelH = hMM }
	mediaW = 64; if mediaWMM > 0 { mediaW = mediaWMM }
	if mediaW < labelW { mediaW = labelW }
	return labelW, labelH, mediaW, nil
}

// composeQRWithLogo returns a PNG QR of tag with logoPNG centered (~22% of size). nil logo → plain QR.
func composeQRWithLogo(tag string, logoPNG []byte) ([]byte, error) {
	qrImg, err := bc.EncodeQRHighEC(tag)
	if err != nil { return nil, err }
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
	if err := png.Encode(&buf, qrImg); err != nil { return nil, err }
	return buf.Bytes(), nil
}

func renderLabelPDF(items []labelItem, opts labelOpts) ([]byte, error) {
	if len(items) == 0 { return nil, ErrNoAssets }
	var pdf *fpdf.Fpdf
	if opts.Layout == "sheet" {
		pdf = fpdf.New("P", "mm", "A4", "")
	} else {
		pdf = fpdf.NewCustom(&fpdf.InitType{UnitStr: "mm", Size: fpdf.SizeType{Wd: opts.MediaW, Ht: opts.LabelH}})
	}
	pdf.SetFont("Helvetica", "", 6)

	drawBTN := func(x, y float64, it labelItem) error {
		pad := 1.0
		qrSide := opts.LabelH - 2*pad
		// QR (left)
		qrPNG, err := composeQRWithLogo(it.Tag, opts.LogoPNG)
		if err != nil { return err }
		name := fmt.Sprintf("qr-%s-%.1f-%.1f", it.Tag, x, y)
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))
		pdf.ImageOptions(name, x+pad, y+pad, qrSide, qrSide, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		// right column
		rx := x + pad + qrSide + 1.5
		rw := opts.LabelW - (rx - x) - pad
		ry := y + pad
		pdf.SetFont("Helvetica", "B", 5)
		pdf.SetXY(rx, ry); pdf.CellFormat(rw, 2.4, trunc(opts.CompanyName, 40), "", 0, "L", false, 0, "")
		ry += 2.6
		pdf.SetFont("Helvetica", "", 5)
		pdf.SetXY(rx, ry); pdf.CellFormat(rw, 2.4, it.Tag, "", 0, "L", false, 0, "")
		ry += 2.4
		pdf.Line(rx, ry, rx+rw, ry); ry += 0.6
		pdf.SetFont("Helvetica", "B", 6)
		pdf.SetXY(rx, ry); pdf.CellFormat(rw/2, 2.8, it.OfficeCode, "", 0, "L", false, 0, "")
		pdf.SetXY(rx+rw/2, ry); pdf.CellFormat(rw/2, 2.8, "TP: "+it.Year, "", 0, "R", false, 0, "")
		ry += 2.9
		pdf.SetFont("Helvetica", "", 5)
		pdf.SetXY(rx, ry); pdf.CellFormat(rw, 2.4, trunc(it.CategoryName, 38), "", 0, "L", false, 0, ""); ry += 2.4
		pdf.SetXY(rx, ry); pdf.CellFormat(rw, 2.4, trunc(it.Name, 38), "", 0, "L", false, 0, ""); ry += 2.6
		pdf.SetTextColor(200, 0, 0); pdf.SetFont("Helvetica", "", 3.5)
		pdf.SetXY(rx, ry); pdf.MultiCell(rw, 1.8, opts.Disclaimer, "", "L", false)
		pdf.SetTextColor(0, 0, 0)
		return nil
	}

	drawGeneric := func(x, y float64, it labelItem) error {
		pad := 1.5
		cx, cy := x+pad, y+pad
		innerW := opts.LabelW - 2*pad
		if opts.ShowName && it.Name != "" { pdf.SetXY(cx, cy); pdf.CellFormat(innerW, 3, trunc(it.Name, 40), "", 0, "L", false, 0, ""); cy += 3.2 }
		imgH := opts.LabelH - (cy - y) - 4; if imgH < 6 { imgH = 6 }
		place := func(enc func(string) ([]byte, error), ix, iw float64) error {
			img, e := enc(it.Tag); if e != nil { return e }
			n := fmt.Sprintf("g-%s-%.1f-%.1f", it.Tag, ix, cy)
			pdf.RegisterImageOptionsReader(n, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(img))
			pdf.ImageOptions(n, ix, cy, iw, imgH, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			return nil
		}
		switch opts.Mode {
		case "qr": if err := place(bc.EncodeQR, cx, imgH); err != nil { return err }
		case "both":
			if err := place(bc.EncodeQR, cx, imgH); err != nil { return err }
			if err := place(bc.EncodeCode128, cx+imgH+1.5, innerW-imgH-1.5); err != nil { return err }
		default: if err := place(bc.EncodeCode128, cx, innerW); err != nil { return err }
		}
		pdf.SetXY(cx, y+opts.LabelH-3.5); pdf.CellFormat(innerW, 3, it.Tag, "", 0, "C", false, 0, "")
		if opts.ShowOffice && it.OfficeCode != "" { pdf.SetXY(cx, y+opts.LabelH-7); pdf.CellFormat(innerW, 3, it.OfficeCode, "", 0, "C", false, 0, "") }
		return nil
	}

	draw := drawGeneric
	if opts.Template == "btn" { draw = drawBTN }

	if opts.Layout == "sheet" {
		const pageW, pageH = 210.0, 297.0
		margin, gutter := 8.0, 3.0
		cols := opts.Columns; if cols < 2 { cols = 3 }
		cellW, cellH := opts.LabelW, opts.LabelH
		rows := int((pageH - 2*margin + gutter) / (cellH + gutter)); if rows < 1 { rows = 1 }
		perPage := cols * rows
		for i, it := range items {
			if i%perPage == 0 { pdf.AddPage() }
			slot := i % perPage
			r, cc := slot/cols, slot%cols
			x := margin + float64(cc)*(cellW+gutter)
			y := margin + float64(r)*(cellH+gutter)
			if x+cellW > pageW-margin { continue }
			if err := draw(x, y, it); err != nil { return nil, err }
		}
	} else {
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

func trunc(s string, n int) string { if len(s) > n { return s[:n] }; return s }

func (s *Service) labelSettings(ctx context.Context) (string, string) {
	company := defaultCompanyName
	if v, err := s.q.GetAppSetting(ctx, "label.company_name"); err == nil && v != "" { company = v }
	disc := defaultDisclaimer
	if v, err := s.q.GetAppSetting(ctx, "label.disclaimer"); err == nil && v != "" { disc = v }
	return company, disc
}

func (s *Service) loadLogo() []byte {
	if s.logoPath == "" { return nil }
	b, err := os.ReadFile(s.logoPath)
	if err != nil { return nil }
	return b
}

type LabelInput struct {
	AssetIDs []uuid.UUID
	Tags     []string
	Opts     labelOpts
}

func (s *Service) BuildLabelPDF(ctx context.Context, in LabelInput, all bool, officeIDs []uuid.UUID) ([]byte, error) {
	// resolve rows (id or tag) — each row carries office_id for scope; refetch the asset for OfficeID.
	type row struct { it labelItem; officeID uuid.UUID }
	var rows []row
	resolve := func(a sqlc.AssetAsset, lbl labelItem) { rows = append(rows, row{it: lbl, officeID: a.OfficeID}) }
	if len(in.AssetIDs) > 0 {
		for _, id := range in.AssetIDs {
			a, err := s.q.GetAsset(ctx, id); if err != nil { return nil, mapDBError(err) }
			l, err := s.q.GetAssetLabelByID(ctx, id); if err != nil { return nil, mapDBError(err) }
			resolve(a, toLabelItem(l.AssetTag, l.Name, l.OfficeCode, l.CategoryName, l.PurchaseDate))
		}
	} else {
		for _, tag := range in.Tags {
			a, err := s.q.GetAssetByTag(ctx, tag); if err != nil { return nil, mapDBError(err) }
			l, err := s.q.GetAssetLabelByTag(ctx, tag); if err != nil { return nil, mapDBError(err) }
			resolve(a, toLabelItem(l.AssetTag, l.Name, l.OfficeCode, l.CategoryName, l.PurchaseDate))
		}
	}
	if len(rows) == 0 { return nil, ErrNoAssets }
	items := make([]labelItem, 0, len(rows))
	for _, r := range rows {
		if !common.InScope(all, officeIDs, r.officeID) { return nil, common.ErrForbidden }
		items = append(items, r.it)
	}
	return renderLabelPDF(items, in.Opts)
}
```

Add a `toLabelItem` helper that formats the year from `pgtype.Date` (`if d.Valid { fmt.Sprintf("%d", d.Time.Year()) } else { "" }`) and copies the strings — match the generated row field types (office_code / category_name may be plain `string` from the joins; verify and adjust). Also add the `logoPath string` field to the `Service` struct and to `NewService` (thread `cfg.LabelLogoPath` from the router wiring — update `internal/server/router.go`'s `asset.NewService(...)` call to pass it; update the signature).

> Reconcile: `context`/`common` imports; the generated label-row struct field names; `s.logoPath` plumbing through `NewService` (this DOES touch `service.go` + `router.go` — keep the existing params, append `logoPath string`).

- [ ] **Step 5: Run tests + build**

Run: `cd backend && go test ./internal/asset/ -run 'ResolveLabelDims|RenderLabelPDF' -v && go build ./...`
Expected: PASS. If `pdfPageCount` mismatches fpdf's output, inspect the raw bytes once and fix the needle; keep an exact page-count assertion for roll.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/barcode/ backend/internal/asset/barcode.go backend/internal/asset/barcode_test.go backend/internal/asset/service.go backend/internal/server/router.go
git commit -m "feat(asset): label PDF builder (BTN + generic templates, roll/sheet, logo compositing)"
```

## Task 6: Label endpoint (DTO with template + handler + route)

**Files:**
- Modify: `backend/internal/asset/barcode_handler.go`, `backend/internal/asset/routes.go`
- Test: `backend/internal/asset/barcode_test.go` (append DTO validation)

**Interfaces:**
- Consumes: `BuildLabelPDF`, `resolveLabelDims`, `labelOpts`, `LabelInput`, `s.labelSettings`, `s.loadLogo`, scope helpers, `contentDisposition`.
- Produces: `LabelRequest` DTO (+ `Template`) + `generateLabels` handler + `POST /assets/labels` route.

- [ ] **Step 1: Write the failing DTO test**

Append to `barcode_test.go`:
```go
func TestLabelRequest_Validate(t *testing.T) {
	if err := (LabelRequest{}).validate(); err == nil { t.Fatal("need ids or tags") }
	if err := (LabelRequest{Tags: []string{"A"}, Layout: "weird"}).validate(); err == nil { t.Fatal("bad layout") }
	if err := (LabelRequest{Tags: []string{"A"}, Template: "nope"}).validate(); err == nil { t.Fatal("bad template") }
	if err := (LabelRequest{Tags: []string{"A"}, Template: "btn", Layout: "roll"}).validate(); err != nil { t.Fatalf("valid: %v", err) }
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
	Template string   `json:"template"` // btn (default) | generic
	Layout   string   `json:"layout"`   // roll (default) | sheet
	Size     string   `json:"size"`
	WidthMM  float64  `json:"w_mm"`
	HeightMM float64  `json:"h_mm"`
	MediaWMM float64  `json:"media_w_mm"`
	Columns  int      `json:"columns"`
	Mode     string   `json:"mode"`     // generic only: barcode|qr|both
	Fields   struct{ Name, Office bool } `json:"fields"`
}

func (r LabelRequest) validate() error {
	if len(r.AssetIDs) == 0 && len(r.Tags) == 0 { return errors.New("provide asset_ids or tags") }
	switch r.Template { case "", "btn", "generic": default: return errors.New("template must be btn or generic") }
	switch r.Layout { case "", "roll", "sheet": default: return errors.New("layout must be roll or sheet") }
	switch r.Mode { case "", "barcode", "qr", "both": default: return errors.New("mode must be barcode, qr, or both") }
	return nil
}

func (h *Handler) generateLabels(c *gin.Context) {
	var req LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	if err := req.validate(); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	labelW, labelH, mediaW, err := resolveLabelDims(req.Size, req.WidthMM, req.HeightMM, req.MediaWMM)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	tmpl := req.Template; if tmpl == "" { tmpl = "btn" }
	layout := req.Layout; if layout == "" { layout = "roll" }
	mode := req.Mode; if mode == "" { mode = "barcode" }
	company, disclaimer := h.svc.labelSettings(c.Request.Context())
	in := LabelInput{Opts: labelOpts{
		Template: tmpl, Layout: layout, LabelW: labelW, LabelH: labelH, MediaW: mediaW,
		Columns: req.Columns, Mode: mode, ShowName: req.Fields.Name, ShowOffice: req.Fields.Office,
		CompanyName: company, Disclaimer: disclaimer, LogoPNG: h.svc.loadLogo(),
	}}
	for _, sID := range req.AssetIDs {
		id, perr := uuid.Parse(sID); if perr != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id: " + sID}); return }
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
	c.Header("Content-Disposition", `attachment; filename="labels.pdf"`)
	c.Data(http.StatusOK, "application/pdf", pdf)
}
```
> `middleware` import only if used; remove if not. `contentDisposition` helper exists but here the filename is a constant, so the literal header is safe.

- [ ] **Step 4: Add the route**

In `routes.go` add inside the `/assets` group: `g.POST("/labels", authMW, requireView, h.generateLabels)`.

- [ ] **Step 5: Run tests + build + vet**

Run: `cd backend && go test ./internal/asset/ && go build ./... && go vet ./...`
Expected: PASS, clean.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/asset/barcode_handler.go backend/internal/asset/routes.go backend/internal/asset/barcode_test.go
git commit -m "feat(asset): label PDF endpoint POST /assets/labels (btn/generic templates)"
```

## Task 7: OpenAPI sync

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add paths + schema**

Mirror existing asset path style (security `bearerJWT`, reuse error response components):
- `GET /assets/by-tag/{tag}` — path param `tag`; 200 → existing `Asset` schema; 401, 404.
- `GET /assets/{id}/barcode` — query `type` (enum code128/qr, default code128); 200 `image/png` (string binary); 400, 401, 403, 404.
- `POST /assets/labels` — requestBody `LabelRequest` (asset_ids[], tags[], template enum btn/generic, layout enum roll/sheet, size, w_mm, h_mm, media_w_mm, columns, mode enum barcode/qr/both, fields{name,office}); 200 `application/pdf` (string binary); 400, 401, 403, 422.

Add schema `LabelRequest`. Reuse the existing `Asset` schema for by-tag. Confirm component ref names from the existing file.

- [ ] **Step 2: Lint**

Run: `cd /d/portfolio-project/asset-management && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): OpenAPI for barcode/scan-lookup/label endpoints"
```

## Task 8: Integration tests

**Files:**
- Create: `backend/internal/asset/barcode_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, the asset Service/Handler + `RegisterRoutes` via httptest, the seeding helpers used by existing asset integration tests.

- [ ] **Step 1: Write the suite (real Postgres; httptest router with stub auth + scope)**

Study `backend/internal/asset/integration_test.go` + `attachment_integration_test.go` for the harness (NewPostgres, seed office/category/asset, stub-auth middleware, ScopedDeps, permission middleware) and mirror it. Cover with REAL assertions:
- **Scan lookup**: seed asset with known tag → `GET /assets/by-tag/<tag>` → 200, body `asset_tag` matches; unknown → 404; out-of-scope caller → 404.
- **Barcode PNG**: `GET /assets/<id>/barcode` → 200 `image/png`, body decodes as PNG; `?type=qr` → 200 PNG; `?type=bad` → 400; out-of-scope → 403.
- **Label PDF (btn)**: `POST /assets/labels` `{asset_ids:[id], template:"btn", layout:"roll"}` → 200 `application/pdf`, body starts `%PDF`. (Seed the asset's office + category so the join resolves.)
- **Label PDF (generic sheet)**: `{tags:[tag], template:"generic", layout:"sheet", mode:"both"}` → 200 PDF.
- **Scope**: an out-of-scope asset in the set → 403. Empty `{asset_ids:[],tags:[]}` → 400. Missing asset id → 404.
- (Logo absent in test env → builder uses plain QR; assert the btn PDF still renders %PDF.)

- [ ] **Step 2: Run the suite**

Run: `cd backend && go test -tags=integration ./internal/asset/ -run 'Barcode|ByTag|Label' -v`
Expected: PASS (Docker up). Confirm `go build -tags=integration ./...` and the non-tag `go test ./internal/asset/` still pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/asset/barcode_integration_test.go
git commit -m "test: integration coverage for barcode/scan-lookup/label endpoints"
```

## Task 9: PROGRESS.md + final verification gate

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

Tick **Barcode/QR** under "Backend — Feature modules" with a one-line note (Code128 + QR PNG from asset_tag; scan-lookup `/assets/by-tag/:tag`; label PDF `/assets/labels` — BTN template (QR+logo+bank header+office/category/name/TP+disclaimer, settings in app_settings, logo via LABEL_LOGO_PATH) + generic; roll page-per-label default 60×24 on 64mm media for Epson C4050, + A4 sheet; scope-gated; integration tests). Refresh the "▶ Next session — start here" block (mark barcode done; next: BAST/asset_documents, or wire frontend Asset/Label/Approval screens after ADR-0007). Do NOT tick transfer/opname/depreciation/import/frontend-wiring. Note the PR number when merged. Keep existing style.

- [ ] **Step 3: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): barcode/QR + label PDF landed"
```

---

## Self-Review notes (spec coverage)

- Encoders (§1) → Task 1 (done) + `EncodeQRHighEC` in Task 5. Scan lookup (§2) → Tasks 2+3. Barcode PNG (§3) → Task 3. BTN-template REVISION (settings/logo/join data) → Task 4 (migration 000017 + queries + config) + Task 5 (builder btn/generic + logo compositing) + Task 6 (DTO template). Generic template + roll/sheet → Task 5. Authz → Tasks 3/6 (by-tag 404, barcode 403, labels 403). Testing → Tasks 5 unit + Task 8 integration. OpenAPI → Task 7. Gates+PROGRESS → Task 9.
- Decisions honored: deps (boombuler + go-pdf/fpdf); btn default + generic option; logo file with plain-QR fallback (no build break); company_name/disclaimer in app_settings with default-constant fallback; asset_tag shown verbatim (no tag-format change); intangible not blocked; on-the-fly (no MinIO).
- Plumbing note: Task 5 adds `Service.logoPath` → `NewService` gains a `logoPath string` param → `router.go` asset wiring passes `d.Cfg.LabelLogoPath` (the only router.go change in this feature).
- Type consistency: `labelItem`(Tag,Name,OfficeCode,CategoryName,Year), `labelOpts`(Template,Layout,...,CompanyName,Disclaimer,LogoPNG), `resolveLabelDims`, `renderLabelPDF`, `composeQRWithLogo`, `BuildLabelPDF`, `LabelRequest`(+Template), `EncodeQRHighEC` defined once and referenced consistently.
