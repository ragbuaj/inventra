# Assets cluster wiring — design spec (2026-07-03)

Wire the frontend **Assets cluster** (Katalog, Detail, Form new/edit, Label/Barcode) from mock
fixtures to the real `/api/v1` backend, plus one small backend extension. Import wizard stays
mock (backend bulk-import not built). Approval screen (`/approval`) stays mock — separate batch;
this batch only *submits* `asset_create` requests.

Decisions locked with the user (2026-07-03):
1. **Extend `AssetCreatePayload`** (backend) so the create form stays 1:1 with the mockup.
2. Detail tabs Penugasan / Maintenance / Depresiasi → **empty-state** ("data belum tersedia")
   until their modules land; no fake static data.
3. **Remove the Hapus action** from Katalog & Detail — no `DELETE /assets` exists; asset exit is
   disposal-via-approval (Disposal screen is a later batch).
4. Scope: everything backend-ready — core screens **and** Label/Barcode.

## 1. Backend change (only one)

Extend `AssetCreatePayload` (`backend/internal/asset/executor.go`) with the same optional fields
`PUT /assets/:id` accepts, so a request payload can carry the full create form:
`brand_id?, model_id?, unit_id?, vendor_id?, po_number?, funding_source?, warranty_expiry?
(YYYY-MM-DD), notes?` (uuid/string validation identical to `AssetUpdateRequest`). The executor
passes them to `CreateAsset` on approval. Update `backend/api/openapi.yaml`
(`AssetCreatePayload`/request schema) + executor unit/integration tests (payload round-trip:
submit → approve → asset row carries the new fields). Existing narrow payloads must keep working
(all new fields optional). Gate: `go build/vet/test ./...`, integration tests
(`-tags=integration`), Spectral.

## 2. Types & data seam (frontend)

- New English snake_case types in `app/types/index.ts` (replace the Indonesian mock `Asset`):
  `Asset` mirrors the backend map: `id, asset_tag, name, category_id, office_id, brand_id?,
  model_id?, room_id?, unit_id?, vendor_id?, current_holder_employee_id?, status, asset_class,
  serial_number?, purchase_date?, purchase_cost?, book_value?, accumulated_depreciation?,
  salvage_value?, po_number?, funding_source?, warranty_expiry?, capitalized?,
  depreciation_method?, useful_life_months?, fiscal_group?, fiscal_life_months?,
  acquisition_bast_no?, excluded_from_valuation?, valuation_exclusion_reason?, notes?,
  created_at?, updated_at?`. **Money fields are `string | null | undefined`** (sqlc string
  convention + field-permission masking can drop them entirely — `undefined` means *masked*,
  render a lock/“—”, never `Rp 0`).
- `AssetStatus` = 7-value backend enum `available | assigned | under_maintenance | in_transfer |
  retired | disposed | lost`.
- **`app/constants/assetMeta.ts`** (new): status → i18n key + badge tone/icon map for all 7
  statuses; asset_class meta. `AssetStatusBadge` and pages import from here, **never** from
  `~/mock/assets`.
- **Cross-consumer protection** (memory: mock→HTTP rewire breaks other consumers):
  `mock/assets.ts` is **kept** for `useGlobalSearch` with a decoupled local `MockAsset` type
  (same pattern as `MockOffice`), and keeps `IMPORT_SAMPLE_ROWS`/`IMPORT_COLUMNS` for the
  still-mock import wizard. Everything else stops importing it. GlobalSearch tests must stay
  green untouched.

## 3. Composables (all on `useApiClient().request`, pattern = `useEmployees`)

- **`useAssets`** (rewrite): `list(query)` → `GET /assets` with `limit/offset/search/status/
  category_id/office_id/asset_class` (server-side pagination, `Paginated<Asset>`);
  `getByTag(tag)` → `GET /assets/by-tag/:tag`; `get(id)` → `GET /assets/:id`;
  `update(id, AssetUpdateInput)` → `PUT /assets/:id` (only backend-allowed fields: name,
  category_id, brand_id, model_id, room_id, unit_id, vendor_id, serial_number, po_number,
  funding_source, purchase_date, warranty_expiry, notes). **No create, no remove.**
- **`useAssetRequests`** (new, thin): `submitCreate(input)` → `POST /requests`
  `{type: 'asset_create', amount: purchase_cost ?? '0', office_id, payload: {…full form…}}`;
  returns the request map (`status: 'pending'`). Nothing else (inbox/decide belongs to the
  approval-screen batch).
- **`useAssetAttachments`** (new): `list(assetId)`, `upload(assetId, File)` (multipart `file`),
  `remove(assetId, attachmentId)`, plus authenticated **blob** URLs for content/thumbnail.
- **Blob support**: `useApiClient` gains `requestBlob(path, opts)` (same auth/refresh/base
  handling, returns `Blob`) used for attachment content/thumbnails, `GET /assets/:id/barcode
  ?type=code128|qr` previews, and `POST /assets/labels` PDF download (trigger object-URL
  download `labels.pdf`). Object URLs revoked on unmount.

## 4. Screens

- **Katalog (`assets/index.vue`)** — server-side list: search (debounced), filter status (7 EN
  statuses), kategori (real `useCategories` tree), kantor (real scoped `useOffices` list, capped
  option list like employees), asset_class; server pagination (`limit 20`); load-error/retry
  state (added — pages currently lack an error state). Row shows masked “—/lock” for absent
  money fields. **Hapus action removed** (and its confirm/i18n). Detail link by `asset_tag`
  (unchanged routes).
- **Detail (`assets/[tag].vue`)** — `getByTag`; Info tab remapped to real fields; FK ids resolved
  to names (category/office/room/brand/model/vendor lookups, same resolution style as offices
  detail). The mockup's **"Foto Aset" gallery** is backed by real attachments (image kinds,
  thumbnail blobs; empty-state when none). Sensitive rows show the lock **only when the field is
  absent** (real masking), value when present. Tabs Penugasan/Maintenance/Depresiasi keep their
  headers but render an empty-state card ("Belum ada data — modul belum tersedia" style, i18n) —
  static samples and their mock imports deleted. Hapus action removed.
- **Form (`AssetForm.vue`, new.vue, [tag]/edit.vue)** — real FK pickers: kategori (tree),
  kantor (scoped; drives ruangan picker via office → floors → rooms cascade), brand → model
  (filtered by brand), unit, vendor (reference engine). Client-side tag preview removed
  (tag is generated server-side on approval) — the field shows an “auto on approval” hint.
  - **mode=new**: full mockup fields; submit → `useAssetRequests.submitCreate`; success UI =
    "pengajuan terkirim, menunggu persetujuan" (toast + redirect to Katalog; the maker-checker
    banner becomes real). `purchase_cost` entered as number, sent as decimal string.
  - **mode=edit**: only backend-updatable fields editable; `purchase_cost`/`asset_class`/
    `status`/tag rendered read-only. Submit → `update(id, …)`. The mockup's **Lampiran dropzone**
    is live here (real upload/list/delete via `useAssetAttachments`, JPG/PNG/PDF per backend
    validation).
  - Lampiran in **mode=new** is necessarily deferred: the asset doesn't exist until the request
    is approved and requests carry no attachments — the dropzone renders disabled with an i18n
    hint "lampiran dapat diunggah setelah aset disetujui" (documented deviation, backend-imposed).
- **Label (`assets/label.vue`)** — picker backed by real `list({search})` (no `assetStore`);
  barcode/QR preview via authenticated blob from `GET /assets/:id/barcode`; Cetak → real
  `POST /assets/labels` (template btn|generic, layout roll|sheet, mode barcode|qr|both, fields
  name/office — matching the existing UI controls) downloading the PDF (replaces comingSoon
  toasts). Client cap ≤500 selections.
- **Permissions**: route middleware `can` — katalog/detail/label → `asset.view`; edit →
  `asset.manage`; new → `request.create`. (Replaces the wrong `masterdata.office.manage`.)
- **i18n**: id/en updates — 7 status keys, masked-field label, tab empty-states, request-submitted
  flow, label print keys; remove dead delete keys. No hardcoded UI text.

## 5. Tests

- **Unit/Nuxt**: rewrite `useAssets` spec (stub `useApiClient`; list params, getByTag, update
  body, error propagation); new specs for `useAssetRequests` (payload shape incl. amount string)
  and `useAssetAttachments`/`requestBlob`; component specs for Katalog (filters → query params,
  pagination, masked money, error/retry, empty), Detail (masked vs visible, tab empty-states,
  not-found), Form (new → request payload; edit → restricted body; picker cascades; validation),
  Label (picker, request body for PDF). Delete `assets-mock.spec.ts` + obsolete mock-based specs.
  GlobalSearch specs untouched and green. **Run the full `pnpm test` suite and check exit code**
  (memory: rewire breaks unrelated consumers via real :8080 fetches — stub every consumer).
- **e2e (real backend, seeded admin)**: rewrite `e2e/assets.spec.ts` — unique-per-run data
  (memory: unique name+code, assert-after-search): flow = seed FKs via API (category etc.),
  submit `asset_create` request as admin, create a **second user** (checker role with
  `request.decide`) via API, approve as that user (SoD: maker ≠ checker), then assert the asset
  in Katalog (search by unique name), open Detail, Edit a field, and label-print smoke (response
  is a PDF). Skip-safe patterns per existing e2e conventions.

## 6. Verification & docs

Full gates before commit: backend `go build/vet/test ./...` + `-tags=integration` (shared
signature touched: executor) + Spectral; frontend `pnpm lint`, `pnpm typecheck`, `pnpm test`,
`pnpm build`; e2e with stack up. Side-by-side 1:1 comparison of each screen vs its
`docs/design/*.dc.html` mockup (light + dark) — deviations limited to the three approved ones.
Update `docs/PROGRESS.md` (wire-batch entry + Next-session block). Branch `feat/assets-wiring`,
Conventional Commits (`feat(assets): …`, backend payload change `feat(approval):` or
`feat(assets):` scope as fits).

## Out of scope

Approval screen wiring, Disposal/Mutasi screens, assignment/maintenance/depreciation modules,
bulk import backend, document (BAST) management UI beyond what Detail already shows, global
search real `/search`.
