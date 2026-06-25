# Assets Cluster — Design Spec (Katalog · Detail · Form · Import · Label/Barcode)

**Date:** 2026-06-25
**Phase:** Frontend feature screens (mock-first)
**Mockups:** `docs/design/{Katalog Aset,Detail Aset,Form Aset,Import Aset,Label Barcode}.dc.html`
**Routes:** `/assets` (catalog), `/assets/new` + `/assets/[tag]/edit` (form), `/assets/[tag]` (detail),
`/assets/import`, `/assets/label`. Wires the disabled **Aset** nav group (Katalog / Import / Label).

Builds the Assets cluster 1:1 with the mockups, mock-first (backend asset module not built). Shared
data layer: `mock/assets.ts` (asset fixtures + status/category metadata) and `useAssets` composable
(list/get/create/update/remove). Each screen is its own page; small shared components are extracted.
Built incrementally — one commit per screen — under one branch/PR.

**Deviation (flagged):** the mockups include a floating **"Preview role"** (manager/staf) demo widget to
showcase role-based price masking. That is a design-preview control, not product UI, so it is omitted;
price columns/fields show by default (admin context). Real per-role field masking is the backend
field-permission concern (later phase).

## 1. Katalog Aset (`/assets`)
Asset list. Header (title/subtitle + Scan / Import / Add actions). Filter bar: search + Status /
Category / Office / Location selects + buy-date range + reset + **table/grid view toggle**. Bulk bar
(when rows selected): count + Print Labels + Export + clear. **Table view**: select-all + per-row
checkbox, sortable columns (tag/name/category/status/buy-date/price/book-value), category & status
badges, holder, price columns, row actions (view/edit/label/delete). **Grid view**: asset cards.
Pagination (20/page), loading skeleton, empty states (no-data / no-match), delete confirm.
- `mock/assets.ts`: `Asset` type, 26 seed assets, `ASSET_STATUS_META` (tone), category list, office/
  location option lists; `assetStore` (all/find/insert/update/remove + reset).
- `useAssets`: `list(query)` → filtered/paginated `Paginated<Asset>` … (page does client filtering like
  the other screens); `get(tag)`, `create`, `update`, `remove`.
- Components: `AssetStatusBadge` (dot + label), `AssetCard` (grid cell). Selection/sort/pagination in page.

## 2. Detail Aset (`/assets/[tag]`)
Single-asset detail: header (name + tag + status + actions), spec/identity panels, valuation,
location/holder, history/timeline, attachments, QR/barcode preview. (Spec'd in detail when built.)

## 3. Form Aset (`/assets/new`, `/assets/[tag]/edit`)
Create/edit form: sectioned fields (identity, classification, location, valuation), validation, save.

## 4. Import Aset (`/assets/import`)
CSV/XLSX import wizard: upload → column mapping → per-row validation preview → confirm. Mock parsing.

## 5. Label / Barcode (`/assets/label`)
Printable Code128 + QR labels for selected assets; single/batch; print layout preview.

## 6. i18n / Nav / Testing
New `assets.*` i18n (id/en). Wire the Aset nav group routes + extend the nav-model `BUILT_ROUTES`.
Per screen: unit (mock + composable) + `mountSuspended` page tests (render, filters, sort, selection,
view toggle, pagination, empty, delete, form validation, import steps). Gated by appropriate permission.

## 7. Verification (DoD)
`pnpm lint` · `pnpm typecheck` · `pnpm test` · `pnpm build` green; live 1:1 comparison of each screen vs
its mockup in light + dark before claiming done.
