# Asset Barcode / QR + Label PDF — Backend Design

Date: 2026-06-28
Status: Approved (decisions confirmed with user)

## Goal

Give every asset a scannable **Code128 barcode** and **QR code** derived from its `asset_tag`, expose a
**scan-lookup** endpoint (tag → asset detail) for quick find / stock opname, and produce **print-ready
PDF labels** sized for the user's **Epson ColorWorks C4050** roll-label printer (and an optional A4
multi-up sheet mode). PRD FR-2.12. `asset_tag` already exists and is unique per asset; this is a
backend-only build on the existing `internal/asset` module.

## Scope

**In scope:**
- `internal/barcode/` package: pure `EncodeCode128(tag) ([]byte, error)` and `EncodeQR(tag) ([]byte, error)` (PNG).
- sqlc `GetAssetByTag`; scan-lookup endpoint `GET /assets/by-tag/:tag` (scoped + field-masked).
- Barcode/QR PNG endpoint `GET /assets/:id/barcode?type=code128|qr` (scoped) — for preview/reuse.
- Print-ready label PDF endpoint `POST /assets/labels` (scoped) — `roll` (page-per-label) or `sheet` (A4 grid).
- OpenAPI sync, unit + integration tests, wiring.

**Out of scope (do NOT build here):**
- Frontend wiring of `frontend/app/pages/assets/label.vue` to these endpoints (separate frontend item).
- Direct/automated printer job submission to the C4050 (network/Epson SDK/CUPS) — the user prints the
  returned PDF via the OS/driver. A print-agent integration is a possible later item.
- MinIO caching of generated artifacts (generation is deterministic from `asset_tag` and cheap → on-the-fly).
- Stock-opname module itself (this only provides the scan-lookup endpoint it will use).

## Decisions (confirmed)

1. **Dependencies:** `github.com/boombuler/barcode` (Code128 + QR → `image.Image`) and
   `github.com/go-pdf/fpdf` (maintained gofpdf fork; precise mm layout + PNG embedding).
2. **Backend-only slice.** Frontend `label.vue` wiring is a separate item.
3. **Intangible assets are NOT blocked** — every asset has a tag, so barcode/QR/label generation works
   for any asset; the (frontend) caller decides what to print. (PRD frames barcode as primarily for
   tangible assets; we do not hard-enforce that.)
4. **On-the-fly generation**, no MinIO/cache (deterministic, cheap, never stale).
5. **C4050 roll stock:** media (paper) width **64 mm**, die-cut label **60 mm × 24 mm**. The roll PDF
   page = media width × label height (64×24 mm by default), label content centered to the label width
   (2 mm left/right margin). Defaults apply when the request omits sizing.
6. **Two layouts via a `layout` param:** `roll` (one label per page, page = media×label, for the C4050)
   and `sheet` (A4 with a `columns` grid, for laser/label-sheet printers). The legacy A4-grid mockup
   informs `sheet`; `roll` is the C4050 path.

## Architecture

### 1. `internal/barcode/` package

```go
// EncodeCode128 returns a PNG of a Code128 barcode for s (e.g. an asset_tag).
func EncodeCode128(s string) ([]byte, error)
// EncodeQR returns a PNG of a QR code for s.
func EncodeQR(s string) ([]byte, error)
```
- Code128: `barcode/code128.Encode(s)` → `barcode.Scale(bc, width, height)` → `png.Encode`. Use a
  generous pixel size (e.g. width scaled to ~600px, height ~120px) so the embedded image stays crisp
  when placed at mm size in the PDF.
- QR: `barcode/qr.Encode(s, qr.M, qr.Auto)` → `barcode.Scale(bc, 300, 300)` → `png.Encode`.
- Pure functions, no DB/HTTP — unit-testable by decoding the PNG back and checking dimensions.
- Errors: invalid/empty input → return the library error (handler maps to 422/500 as appropriate).

### 2. Scan lookup

- `db/queries/assets.sql` add: `-- name: GetAssetByTag :one  SELECT * FROM asset.assets WHERE asset_tag = $1 AND deleted_at IS NULL;`
- `Service.GetByTag(ctx, tag string) (sqlc.AssetAsset, error)` wrapping `mapDBError` (no-rows → ErrNotFound).
- Handler `getByTag`: `GET /assets/by-tag/:tag` (after authMW + `asset.view`). Fetch by tag → resolve
  caller scope (`CallerOfficeScope(c, "assets")`) → `InScope(all, ids, a.OfficeID)` else **404** (not
  403 — avoids confirming existence of out-of-scope tags to a scanner) → field-mask via the existing
  `filterMap` → return the asset map (same shape as `get`).

### 3. Barcode/QR PNG endpoint

- Handler `getBarcode`: `GET /assets/:id/barcode?type=code128|qr` (default `code128`) (authMW +
  `asset.view`). Resolve asset by id, scope-check (InScope else 403), then
  `barcode.EncodeCode128(a.AssetTag)` or `EncodeQR` → respond `image/png` with
  `X-Content-Type-Options: nosniff` (consistent with the attachment serve hardening). Unknown `type` → 400.

### 4. Label PDF endpoint

- Handler `generateLabels`: `POST /assets/labels` (authMW + `asset.view`). Body DTO:
```go
type LabelRequest struct {
    AssetIDs []string  `json:"asset_ids" binding:"required_without=Tags"`
    Tags     []string  `json:"tags"      binding:"required_without=AssetIDs"`
    Layout   string    `json:"layout"    binding:"omitempty,oneof=roll sheet"` // default roll
    Size     string    `json:"size"`        // preset "60x24" | "50x30" | "70x40" | "100x50"; default 60x24
    WidthMM  float64   `json:"w_mm"`        // optional explicit label width (overrides Size)
    HeightMM float64   `json:"h_mm"`        // optional explicit label height
    MediaWMM float64   `json:"media_w_mm"`  // roll media width; default 64
    Columns  int       `json:"columns"   binding:"omitempty,oneof=2 3 4"` // sheet only; default 3
    Mode     string    `json:"mode"      binding:"omitempty,oneof=barcode qr both"` // default barcode
    Fields   struct{ Name, Office bool } `json:"fields"`
}
```
- Service `BuildLabelPDF(ctx, in LabelInput) ([]byte, error)`:
  1. Resolve assets: by `AssetIDs` (GetAsset each) or by `Tags` (GetByTag each). Resolve caller scope
     once; if ANY asset is out of scope → return `ErrForbidden` (handler → 403; batch never leaks).
     Empty result → `ErrNoAssets` (422).
  2. Resolve dimensions: label `(w,h)` from `Size`/explicit (default 60×24); `mediaW` default 64.
  3. For each asset, encode the needed PNG(s) (`Mode`) once.
  4. **roll**: `fpdf` with custom page size `mediaW × h` (mm), one page per asset; draw label content in
     a `w × h` box centered horizontally (left margin `(mediaW-w)/2`). **sheet**: A4 page (210×297),
     compute a `columns`-wide grid of `w × h` cells with small gutters; flow assets across cells/pages.
  5. Per label, render: optional name (truncated to fit), the barcode and/or QR per `Mode`, and the tag
     text under/next to the code; optional office line. Keep it legible at the small size — barcode
     spans most of the label width, tag in a small monospace-ish font beneath.
- Handler streams `application/pdf` with `Content-Disposition: attachment; filename="labels.pdf"` (use
  the same sanitized `contentDisposition` helper from the attachments work) + `nosniff`.

### 5. Module layout (asset module)

- `internal/asset/barcode.go` — `GetByTag`, `BuildLabelPDF`, `LabelInput`, label sentinels
  (`ErrNoAssets`, `ErrUnsupportedBarcodeType`). Calls `internal/barcode` for image encoding.
- `internal/asset/barcode_handler.go` — `getByTag`, `getBarcode`, `generateLabels` + `LabelRequest` DTO
  + validation; reuses `resolveAssetInScope`/`filterMap`/scope helpers already in the module.
- `internal/asset/routes.go` (extend): register the three routes (see Authorization).
- `db/queries/assets.sql` (+ generated) — `GetAssetByTag`.

## Authorization

All three endpoints require `authMW` + `asset.view` and enforce office scope on the target asset(s):
- `GET /assets/by-tag/:tag` → out-of-scope/unknown → **404**.
- `GET /assets/:id/barcode` → out-of-scope → **403** (mirrors existing `/assets/:id`).
- `POST /assets/labels` → any out-of-scope asset → **403**.
No new permission keys. No field-permission concern for the PNG/PDF (only tag/name/office rendered);
the JSON scan-lookup reuses `filterMap` for the asset detail.

Routes:
```
g.GET("/by-tag/:tag", authMW, requireView, h.getByTag)
g.GET("/:id/barcode",  authMW, requireView, h.getBarcode)
g.POST("/labels",      authMW, requireView, h.generateLabels)
```
> Gin nesting note: `/assets/by-tag/:tag` and `/assets/labels` are static-prefixed siblings of
> `/assets/:id`. Gin's tree allows a static segment (`by-tag`, `labels`) alongside a param (`:id`) at
> the same level. Verify the router registers without panic (integration boots it); if Gin objects,
> nest barcode under `/assets/:id/...` and move scan-lookup to `/assets/lookup?tag=`.

## Error handling

- Unknown `type` on barcode → 400; encode failure → 500. Unknown/empty tag → 404 (by-tag). Empty label
  selection → 422 (`ErrNoAssets`). Out-of-scope → 403 (barcode/labels) / 404 (by-tag). Bad DTO
  (oneof/size parse) → 400. Reuse `mapDBError` for DB paths and the asset module's error helper.

## Testing

**Unit** (pure, no DB):
- `EncodeCode128`/`EncodeQR`: output decodes as a PNG with expected (non-zero, scaled) dimensions; empty
  string handled (error or valid — assert the chosen behavior).
- Size resolution: `"60x24"` → (60,24); explicit `w_mm/h_mm` overrides preset; unknown preset → error;
  defaults (roll, 60×24, media 64) when omitted.
- `BuildLabelPDF` (with a fake/stub asset list and the real encoders): returns bytes starting with
  `%PDF`; **roll** → page count == number of assets; **sheet** with `columns=3` and N assets → page
  count == ceil(N / perPage) where perPage = columns × rows-that-fit; centered-margin math
  `(mediaW-w)/2` correct.
- LabelRequest validation (layout/mode/columns oneof; required_without for ids/tags).

**Integration** (`//go:build integration`, real Postgres; via httptest router):
- Seed office + asset. `GET /assets/by-tag/:tag` → 200 with the asset; unknown tag → 404; out-of-scope
  caller → 404.
- `GET /assets/:id/barcode` → 200 `image/png` (body decodes as PNG); `?type=qr` → 200 PNG; `?type=bad`
  → 400; out-of-scope → 403.
- `POST /assets/labels` (roll, 2 assets) → 200 `application/pdf`, body starts `%PDF`; out-of-scope asset
  in the set → 403; empty selection → 422; `layout=sheet` → 200 PDF.

## Verification gates

`go build ./...` · `go vet ./...` · `go test ./...` · `go test -tags=integration ./internal/asset/`
· Spectral lint `backend/api/openapi.yaml` · update `docs/PROGRESS.md`.

## Open items (flagged, non-blocking)

- Direct C4050 job submission (no OS dialog) — deferred; would need a print agent / Epson SDK.
- Frontend `label.vue` wiring to these endpoints — separate frontend item (after ADR-0007 refactor).
- PDF label typography polish (font embedding for non-ASCII names) — `fpdf` core fonts cover Latin-1;
  if asset names need full Unicode, register a TTF later. Tags are ASCII (`A-Z0-9-`), so codes/tags are safe.

---

## REVISION (2026-06-28) — BTN label template (user-supplied layout)

The user supplied a concrete BTN asset-label layout. This supersedes the simple "barcode + tag"
label of bagian 4. Confirmed decisions: logo provided as a file (embed; QR-center overlay + header) with
graceful fallback if absent; bank name + disclaimer in `app_settings`; the label's "asset code" is the
stored `asset_tag` verbatim (tag format unchanged); **two templates** — `btn` (default) and `generic`.

### Label layout (template = `btn`, 60×24 mm landscape)
```
┌──────────┬─────────────────────────────────────────────┐
│          │ [logo] <company_name>                        │
│   QR     │ <asset_tag>                                  │
│ (+logo   │─────────────────────────────────────────────│
│  center) │ <office_code>            TP: <purchase_year> │
│          │ <category_name>                             │
│          │ <asset_name>                                │
│          │ (red, small) <disclaimer>                   │
└──────────┴─────────────────────────────────────────────┘
```
- Left: QR of `asset_tag`, BTN logo composited at center (QR error-correction **High** so the center
  may be obscured). Logo loaded from a configured path; if the file is absent, render a plain QR (no
  overlay) — must not break build/tests/CI.
- Right: small logo + `company_name` (app_settings), `asset_tag`; divider; `office_code` (bold left) +
  `TP: <year>` (right, from `purchase_date`); `category_name`; `asset_name`; red `disclaimer`
  (app_settings), wrapped.

### Data & config
- New `app_settings` rows (seed migration `000017`): `label.company_name`
  (default `PT Bank Tabungan Negara (Persero) Tbk`) and `label.disclaimer`
  (default `Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung`).
  Read via a sqlc `GetAppSetting(key)` query (returns value; missing → fall back to the default constant).
- New join query for label data per asset (by id and by tag): `asset_tag, name (asset), office_code,
  category_name, purchase_date`. `asset_tag` format is NOT changed.
- Logo: config `LABEL_LOGO_PATH` (default `assets/btn-logo.png`, relative to the backend working dir);
  loaded at request time (or cached); if the file does not exist, skip the logo (plain QR + no header
  logo). Compositing uses `disintegration/imaging` (already a dependency).

### Template selection
- `LabelRequest.template`: `"btn"` (default) | `"generic"`. `generic` keeps the original `mode`
  (barcode/qr/both) + field toggles. Both honor the `layout` (roll/sheet) + size params.
- `btn` template ignores `mode` (always QR) and renders the fixed BTN field layout.

### Revised authorization / errors
Unchanged from bagian Authorization. Missing `app_settings` value → default constant (not an error). Missing
logo file → plain QR (not an error). All else as before.
