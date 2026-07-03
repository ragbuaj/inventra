# Wire Assets cluster to real backend — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire Katalog, Detail, Form (new/edit), and Label/Barcode from mock fixtures to the real `/api/v1` backend — create-via-approval, server-side list, real field-permission masking, attachments, barcode/label PDF — plus extend the backend `AssetCreatePayload` to the full form field set.

**Architecture:** One small backend change (widen `AssetCreatePayload` + executor to accept the same optional fields as `PUT /assets/:id`). Frontend replaces the Indonesian tag-keyed mock `Asset` with the English snake_case backend contract (ADR-0007), rewrites `useAssets` onto `useApiClient`, adds `useAssetRequests` (submit `asset_create`) + `useAssetAttachments` + a `requestBlob` helper for PNG/PDF, and rebinds the four screens. Approval inbox/decide UI stays mock (separate batch); Import wizard stays mock (no backend).

**Tech Stack:** Go 1.25 + Gin + sqlc (backend); Nuxt 4 SPA + Nuxt UI + @nuxtjs/i18n + Pinia + Vitest + Playwright (frontend).

**Spec:** `docs/superpowers/specs/2026-07-03-assets-wiring-design.md` (decisions: extend payload; tabs Penugasan/Maintenance/Depresiasi → empty-state; Hapus action removed; Label screen in scope).

## Global Constraints

- **No `POST /assets`, no `DELETE /assets`.** Create = `POST /requests` `{type:"asset_create", amount:"<decimal string>" (required), office_id, payload}`; asset row exists only after final approve. Remove every delete action/i18n from Katalog & Detail.
- **`PUT /assets/:id` body is exactly** (`AssetUpdateRequest`): `name`(req), `category_id`(req,uuid), `brand_id?`, `model_id?`, `room_id?`, `unit_id?`, `vendor_id?` (uuid), `serial_number?`, `po_number?`, `funding_source?`, `purchase_date?`, `warranty_expiry?` (YYYY-MM-DD), `notes?`. NEVER send `purchase_cost`/`status`/`asset_class` on update.
- **Money fields are decimal strings** (`purchase_cost`, `book_value`, `accumulated_depreciation`, `salvage_value`); they can be **absent** from responses (field-permission masking) — absent = render masked (lock/“—”), never `Rp 0`. Types must be `string` and optional.
- `AssetStatus` = `available | assigned | under_maintenance | in_transfer | retired | disposed | lost`. `asset_class` = `tangible | intangible`.
- `GET /assets` query params: `limit`(1–100, default 20), `offset`, `search`, `status`, `category_id`, `office_id`, `asset_class`; envelope `{data,total,limit,offset}`. Invalid `status`/`asset_class` → 400, so only send validated values.
- Route guards: katalog/detail/label → `asset.view`; edit → `asset.manage`; new → `request.create` (replace the wrong `masterdata.office.manage`).
- All frontend HTTP via `useApiClient().request` / new `requestBlob`. i18n id+en for every string; ESLint `commaDangle:'never'`, 1tbs; `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` green.
- **Cross-consumer protection (memory #40):** `useGlobalSearch.ts` imports `assetStore` from `~/mock/assets`; `assets/import.vue` imports `IMPORT_SAMPLE_ROWS`/`IMPORT_COLUMNS`. **Keep `frontend/app/mock/assets.ts`** but give it a local decoupled `MockAsset` type (pattern: `MockOffice`) once the global `Asset` type changes. GlobalSearch + Import specs must stay green untouched. After each frontend task run the FULL `pnpm test` and check exit code 0 (a stray real-`:8080` fetch means a consumer wasn't stubbed).
- Backend gates (from `backend/`): `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./...`, Spectral on `backend/api/openapi.yaml`.
- Component tests stub the API seam (mock `useApiClient` or the composable) — never hit `:8080` in Vitest.
- Mockups are the visual source of truth: `docs/design/Katalog Aset.dc.html`, `Detail Aset.dc.html`, `Form Aset.dc.html`, `Label Barcode.dc.html`. Only the three approved deviations apply.
- Branch `feat/assets-wiring`; Conventional Commits; no AI attribution.

## Pre-existing files map

| Path | Action |
|---|---|
| `backend/internal/asset/executor.go` | MODIFY (payload + createExec) |
| `backend/internal/asset/asset_integration_test.go` (or nearest executor test file) | EXTEND |
| `backend/api/openapi.yaml` | MODIFY (`AssetCreatePayload` schema) |
| `frontend/app/types/index.ts` | MODIFY (`Asset`, `AssetStatus`, inputs) |
| `frontend/app/constants/assetMeta.ts` | CREATE |
| `frontend/app/mock/assets.ts` | MODIFY (decoupled `MockAsset`; keep store + import fixtures) |
| `frontend/app/composables/useApiClient.ts` | MODIFY (add `requestBlob`) |
| `frontend/app/composables/api/useAssets.ts` | REWRITE |
| `frontend/app/composables/api/useAssetRequests.ts` | CREATE |
| `frontend/app/composables/api/useAssetAttachments.ts` | CREATE |
| `frontend/app/components/asset/AssetStatusBadge.vue` | MODIFY (assetMeta, 7 statuses) |
| `frontend/app/components/asset/AssetForm.vue` | REWRITE (FK pickers, request flow) |
| `frontend/app/pages/assets/{index,new,label}.vue`, `assets/[tag].vue`, `assets/[tag]/edit.vue` | REWRITE/MODIFY |
| `frontend/i18n/locales/{id,en}.json` | MODIFY (`assets.*`) |
| `frontend/test/unit/assets-mock.spec.ts` | DELETE |
| `frontend/test/nuxt/assets-{catalog,detail,form,label}.spec.ts` | REWRITE |
| `frontend/test/nuxt/assets-import.spec.ts`, GlobalSearch specs | UNTOUCHED (must stay green) |
| `frontend/e2e/assets.spec.ts` | REWRITE (real backend) |
| `docs/PROGRESS.md` | UPDATE (final task) |

---

### Task 1: Backend — widen `AssetCreatePayload` to the full form field set

**Files:**
- Modify: `backend/internal/asset/executor.go`
- Modify: `backend/api/openapi.yaml` (AssetCreatePayload schema)
- Test: the asset integration test file that covers the approval executors (find via `grep -l "asset_create" backend/internal/asset/*_test.go backend/internal/approval/*_test.go`)

**Interfaces:**
- Produces: `AssetCreatePayload` JSON accepts optional `brand_id, model_id, unit_id, vendor_id` (uuid strings), `po_number, funding_source, notes` (strings), `warranty_expiry` ("2006-01-02"). Frontend Task 5 sends exactly these keys.
- All new fields optional — existing narrow payloads keep working.

- [ ] **Step 1: Write the failing integration test** — extend the existing asset_create executor round-trip test (submit request whose payload carries all new fields → approve → assert the created asset row persists `brand_id/model_id/unit_id/vendor_id/po_number/funding_source/warranty_expiry/notes`). Follow the file's existing seed helpers (`testsupport`). Include one sad-path assert: bad `brand_id` uuid in payload → approve fails with invalid-reference.

- [ ] **Step 2: Run it, verify it fails** — from `backend/`: `go test -tags=integration ./internal/... -run <TestName>` (fails: fields not persisted).

- [ ] **Step 3: Extend the payload + executor** — in `executor.go`:

```go
type AssetCreatePayload struct {
	Name           string  `json:"name"`
	CategoryID     string  `json:"category_id"`
	OfficeID       string  `json:"office_id"`
	RoomID         *string `json:"room_id"`
	AssetClass     string  `json:"asset_class"`
	PurchaseCost   *string `json:"purchase_cost"`
	PurchaseDate   *string `json:"purchase_date"` // "2006-01-02"
	SerialNumber   *string `json:"serial_number"`
	BrandID        *string `json:"brand_id"`
	ModelID        *string `json:"model_id"`
	UnitID         *string `json:"unit_id"`
	VendorID       *string `json:"vendor_id"`
	PONumber       *string `json:"po_number"`
	FundingSource  *string `json:"funding_source"`
	WarrantyExpiry *string `json:"warranty_expiry"` // "2006-01-02"
	Notes          *string `json:"notes"`
}
```

In `createExec.Execute`, after the existing `roomID` parse, add (same pattern):

```go
	brandID, err := common.ParseUUIDPtr(p.BrandID)
	if err != nil {
		return ErrInvalidRef
	}
	modelID, err := common.ParseUUIDPtr(p.ModelID)
	if err != nil {
		return ErrInvalidRef
	}
	unitID, err := common.ParseUUIDPtr(p.UnitID)
	if err != nil {
		return ErrInvalidRef
	}
	vendorID, err := common.ParseUUIDPtr(p.VendorID)
	if err != nil {
		return ErrInvalidRef
	}
	warrantyExpiry, derr := parsePurchaseDate(p.WarrantyExpiry)
	if derr != nil {
		return fmt.Errorf("invalid warranty_expiry: %w", derr)
	}
```

and extend `CreateAssetParams` literal with `BrandID: brandID, ModelID: modelID, UnitID: unitID, VendorID: vendorID, PoNumber: p.PONumber, FundingSource: p.FundingSource, WarrantyExpiry: warrantyExpiry, Notes: p.Notes,`.

- [ ] **Step 4: Update `backend/api/openapi.yaml`** — find the `AssetCreatePayload` (or the `POST /requests` payload description for `asset_create`) schema and add the eight optional properties (uuid format for the four ids, `date` format for `warranty_expiry`).

- [ ] **Step 5: Run gates** — from `backend/`: `go build ./... ; go vet ./... ; go test ./... ; go test -tags=integration ./...` then `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` (from repo root). All green (memory: full integration gate after shared-signature changes).

- [ ] **Step 6: Commit** — `feat(assets): widen AssetCreatePayload to full create-form field set`

---

### Task 2: Frontend — English `Asset` types, `assetMeta` constants, badge + i18n statuses, mock decoupling

**Files:**
- Modify: `frontend/app/types/index.ts`
- Create: `frontend/app/constants/assetMeta.ts`
- Modify: `frontend/app/components/asset/AssetStatusBadge.vue`
- Modify: `frontend/app/mock/assets.ts` (local `MockAsset` type; keep `assetStore`, `IMPORT_SAMPLE_ROWS`, `IMPORT_COLUMNS`; stop exporting the old global-type constants)
- Modify: `frontend/i18n/locales/{id,en}.json`
- Test: `frontend/test/unit/asset-meta.spec.ts` (CREATE), `frontend/test/nuxt/asset-status-badge.spec.ts` (CREATE or extend existing badge coverage)

**Interfaces (produced — later tasks import these exact names):**

```ts
// app/types/index.ts
export type AssetStatus = 'available' | 'assigned' | 'under_maintenance'
  | 'in_transfer' | 'retired' | 'disposed' | 'lost'
export type AssetClass = 'tangible' | 'intangible'
export interface Asset {
  id: string
  asset_tag: string
  name: string
  category_id: string
  office_id: string
  brand_id?: string | null
  model_id?: string | null
  room_id?: string | null
  unit_id?: string | null
  vendor_id?: string | null
  current_holder_employee_id?: string | null
  created_by_id?: string | null
  status: AssetStatus
  asset_class: AssetClass
  serial_number?: string | null
  purchase_date?: string | null
  purchase_cost?: string | null       // absent ⇒ masked by field permission
  book_value?: string | null          // absent ⇒ masked
  accumulated_depreciation?: string | null // absent ⇒ masked
  salvage_value?: string | null
  po_number?: string | null
  funding_source?: string | null
  warranty_expiry?: string | null
  capitalized?: boolean
  depreciation_method?: string | null
  useful_life_months?: number | null
  fiscal_group?: string | null
  fiscal_life_months?: number | null
  acquisition_bast_no?: string | null
  excluded_from_valuation?: boolean
  valuation_exclusion_reason?: string | null
  notes?: string | null
  created_at?: string
  updated_at?: string
}
export interface AssetUpdateInput {
  name: string
  category_id: string
  brand_id?: string | null
  model_id?: string | null
  room_id?: string | null
  unit_id?: string | null
  vendor_id?: string | null
  serial_number?: string | null
  po_number?: string | null
  funding_source?: string | null
  purchase_date?: string | null
  warranty_expiry?: string | null
  notes?: string | null
}
export interface AssetCreateInput extends AssetUpdateInput {
  office_id: string
  asset_class: AssetClass
  purchase_cost?: string | null
}
export interface AssetAttachment {
  id: string
  asset_id: string
  kind: string
  original_filename: string
  size_bytes: number
  mime_type: string
  has_thumbnail: boolean
  created_at: string
}
```

```ts
// app/constants/assetMeta.ts
import type { AssetStatus, AssetClass } from '~/types'
export const ASSET_STATUSES: AssetStatus[] = ['available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost']
export const ASSET_CLASSES: AssetClass[] = ['tangible', 'intangible']
// statusMeta[s] = { labelKey: `assets.status.${s}`, color: <Nuxt UI badge color> }
export const statusMeta: Record<AssetStatus, { labelKey: string, color: 'success' | 'info' | 'warning' | 'error' | 'neutral' }> = {
  available: { labelKey: 'assets.status.available', color: 'success' },
  assigned: { labelKey: 'assets.status.assigned', color: 'info' },
  under_maintenance: { labelKey: 'assets.status.under_maintenance', color: 'warning' },
  in_transfer: { labelKey: 'assets.status.in_transfer', color: 'info' },
  retired: { labelKey: 'assets.status.retired', color: 'neutral' },
  disposed: { labelKey: 'assets.status.disposed', color: 'neutral' },
  lost: { labelKey: 'assets.status.lost', color: 'error' }
}
export const classMeta: Record<AssetClass, { labelKey: string }> = {
  tangible: { labelKey: 'assets.class.tangible' },
  intangible: { labelKey: 'assets.class.intangible' }
}
```

- [ ] **Step 1: Failing unit test** `test/unit/asset-meta.spec.ts` — asserts all 7 statuses present in `ASSET_STATUSES` and `statusMeta`, label keys follow `assets.status.<value>`, both classes covered.
- [ ] **Step 2: Run** `pnpm test asset-meta` → fails (module missing).
- [ ] **Step 3: Implement** types + `assetMeta.ts` exactly as above. In `mock/assets.ts`: define `export interface MockAsset { tag: string; nama: string; kategori: string; brand: string; status: string; kantor: string; lokasi: string; holder: string; tgl: string; harga: number; buku: number }`, retype `assetSeed`/`assetStore` with it, keep `IMPORT_SAMPLE_ROWS`/`IMPORT_COLUMNS`, delete the old status/category/office constant exports (their consumers are rewired in Tasks 6–9; leave any still-referenced export in place until its consumer task removes the import, so typecheck stays green **at every task boundary** — if index.vue still imports `ASSET_STATUS_KEYS` at the end of this task, keep exporting it from the mock temporarily).
- [ ] **Step 4: Rewire `AssetStatusBadge.vue`** to `statusMeta` (prop `status: AssetStatus`; `UBadge :color="statusMeta[status].color"` + `$t(labelKey)`), and update i18n: replace the 5 Indonesian-keyed `assets.status.*` entries with the 7 new keys in **both** `id.json` (`available: "Tersedia"`, `assigned: "Digunakan"`, `under_maintenance: "Maintenance"`, `in_transfer: "Dalam Mutasi"`, `retired: "Nonaktif"`, `disposed: "Dilepas"`, `lost: "Hilang"`) and `en.json` (Available / Assigned / Under Maintenance / In Transfer / Retired / Disposed / Lost). Add `assets.class.{tangible,intangible}` (id: Berwujud/Takberwujud) and `assets.masked` (id: "Tersembunyi (izin)", en: "Hidden (permission)").
- [ ] **Step 5: Badge component test** (`// @vitest-environment nuxt`, `mountSuspended`) — each of the 7 statuses renders its resolved Indonesian label; unknown-safe (no crash on missing).
- [ ] **Step 6: FULL suite** `pnpm test` → exit 0 (GlobalSearch + Import specs must still pass; temporary mock exports keep old pages compiling) and `pnpm typecheck`.
- [ ] **Step 7: Commit** — `feat(assets): english Asset contract, assetMeta constants, 7-status badge`

---

### Task 3: Frontend — `requestBlob` on `useApiClient`

**Files:**
- Modify: `frontend/app/composables/useApiClient.ts`
- Test: extend the existing `useApiClient` spec (find `test/**/use-api-client*`; CREATE `test/nuxt/use-api-client-blob.spec.ts` if none)

**Interfaces:**
- Produces: `requestBlob(path: string, opts?: Record<string, unknown>): Promise<Blob>` — same base URL, Authorization, X-Request-ID, 401→refresh-retry, 401-final→clear+login redirect, other errors→toast+rethrow, as `request`.

- [ ] **Step 1: Failing test** — stub `$fetch`; assert `requestBlob('/assets/x/barcode?type=qr')` passes `responseType: 'blob'`, carries `Authorization`, retries once after 401+successful refresh, returns the Blob.
- [ ] **Step 2: Run** → fails (`requestBlob` not exported).
- [ ] **Step 3: Implement** — extract the shared try/refresh/notify flow so `request` and `requestBlob` reuse it (DRY):

```ts
  async function requestBlob(path: string, opts: Record<string, unknown> = {}): Promise<Blob> {
    return doFetch<Blob>(path, { ...opts, responseType: 'blob' })
  }
```

where `doFetch<T>` is the existing `request` body hoisted into a helper used by both. Return `{ request, requestBlob, refreshToken }`.
- [ ] **Step 4: Run tests** — the touched spec, then FULL `pnpm test` exit 0 + `pnpm typecheck`.
- [ ] **Step 5: Commit** — `feat(frontend): requestBlob helper on useApiClient`

---

### Task 4: Frontend — rewrite `useAssets` (real `$fetch`)

**Files:**
- Rewrite: `frontend/app/composables/api/useAssets.ts`
- Test: CREATE `frontend/test/nuxt/use-assets.spec.ts`; DELETE `frontend/test/unit/assets-mock.spec.ts`

**Interfaces:**
- Produces (exact — pages consume these):

```ts
export interface AssetListQuery {
  limit?: number
  offset?: number
  search?: string
  status?: AssetStatus
  category_id?: string
  office_id?: string
  asset_class?: AssetClass
}
export function useAssets(): {
  list(query?: AssetListQuery): Promise<Paginated<Asset>>
  get(id: string): Promise<Asset>
  getByTag(tag: string): Promise<Asset>
  update(id: string, input: AssetUpdateInput): Promise<Asset>
}
```

- Consumes: `useApiClient().request`; `Paginated<T>` existing type.

- [ ] **Step 1: Failing tests** (`use-assets.spec.ts`, stub `useApiClient`) — `list` builds `/assets?limit=20&offset=0` by default and appends only provided filters (no empty params; `search` URL-encoded); `getByTag` hits `/assets/by-tag/<tag>` (tag URL-encoded); `update` PUTs exactly the `AssetUpdateInput` keys (assert body has NO `purchase_cost`/`status`); errors propagate.
- [ ] **Step 2: Run** → fail. **Step 3: Implement** following `useEmployees` verbatim style (URLSearchParams; `request<Paginated<Asset>>(...)`; `request<Asset>('/assets/' + id, { method: 'PUT', body: input })`). No create/remove exports.
- [ ] **Step 4:** Delete `test/unit/assets-mock.spec.ts`. FULL `pnpm test` exit 0 — **expected breakage check:** the four page specs still stub nothing; if any existing spec now hits `:8080`, stub `useAssets` there minimally (they are fully rewritten in Tasks 6–9, keep the patch minimal).
- [ ] **Step 5: Commit** — `feat(assets): useAssets on real /api/v1/assets`

---

### Task 5: Frontend — `useAssetRequests` + `useAssetAttachments`

**Files:**
- Create: `frontend/app/composables/api/useAssetRequests.ts`, `frontend/app/composables/api/useAssetAttachments.ts`
- Test: CREATE `frontend/test/nuxt/use-asset-requests.spec.ts`, `frontend/test/nuxt/use-asset-attachments.spec.ts`

**Interfaces:**

```ts
// useAssetRequests.ts
export interface SubmittedRequest { id: string, type: string, status: string, amount: string, office_id: string, created_at?: string }
export function useAssetRequests(): {
  submitCreate(input: AssetCreateInput): Promise<SubmittedRequest>
}
// body sent: { type: 'asset_create', amount: input.purchase_cost ?? '0',
//              office_id: input.office_id, payload: input }
```

```ts
// useAssetAttachments.ts
export function useAssetAttachments(): {
  list(assetId: string): Promise<{ data: AssetAttachment[], total: number }>
  upload(assetId: string, file: File): Promise<AssetAttachment>   // multipart field "file"
  remove(assetId: string, attachmentId: string): Promise<void>
  thumbnailBlob(assetId: string, attachmentId: string): Promise<Blob>  // GET .../thumbnail
  contentBlob(assetId: string, attachmentId: string): Promise<Blob>    // GET .../content
}
```

- [ ] **Step 1: Failing tests** — submitCreate posts `/requests` with the exact envelope above (assert `amount` is a **string**, payload passes through all `AssetCreateInput` keys); attachments: upload builds `FormData` with field `file`, remove issues DELETE, thumbnail/content call `requestBlob` with the right paths.
- [ ] **Step 2: Run** → fail. **Step 3: Implement** (thin `useApiClient` wrappers; upload via `request(..., { method: 'POST', body: formData })`).
- [ ] **Step 4:** Full `pnpm test` exit 0 + typecheck. **Step 5: Commit** — `feat(assets): asset create-request + attachments composables`

---

### Task 6: Katalog (`assets/index.vue`) — server-side list

**Files:**
- Rewrite data layer of: `frontend/app/pages/assets/index.vue`
- Modify: `frontend/i18n/locales/{id,en}.json` (remove delete keys; add error/retry + masked)
- Test: REWRITE `frontend/test/nuxt/assets-catalog.spec.ts`

**Interfaces:**
- Consumes: `useAssets().list`, `statusMeta`/`ASSET_STATUSES`/`ASSET_CLASSES`, `useCategories().tree` (or its list), `useOffices().list`, `AssetStatusBadge`.
- Mockup: `docs/design/Katalog Aset.dc.html` — layout/columns/filter bar unchanged; approved deviation: no Hapus action.

- [ ] **Step 1: Failing component test** (stubbed composables) — renders rows from a stubbed `list` response; filter selects contain the 7 statuses (resolved i18n); changing status/kategori/kantor/class or search re-calls `list` with the matching query param; pagination passes `offset`; masked money (`purchase_cost` undefined) renders the masked indicator not `Rp 0`; error from `list` → error state with retry button that re-fetches; empty result → empty state; NO delete button rendered.
- [ ] **Step 2: Run** → fail. **Step 3: Rewrite the page data layer:**
  - State: `rows: Asset[]`, `total`, `page` (limit 20, server `offset = (page-1)*20`), `loading`, `loadError`, filters (`search` debounced 300ms, `status?: AssetStatus`, `categoryId?`, `officeId?`, `assetClass?`).
  - `load()` → `list({ limit: 20, offset, search: search || undefined, status, category_id: categoryId, office_id: officeId, asset_class: assetClass })`; watch filters → reset page → load; try/catch sets `loadError` (i18n `common.loadError` pattern used by wired pages + retry button).
  - Filter options: kategori from `useCategories` (id → name), kantor from `useOffices().list({ limit: 100 })` (scoped), status/class from `assetMeta` constants.
  - Columns per mockup; kategori/kantor cells resolve id→name via lookup maps built from the option fetches; harga/buku formatted from decimal strings (`Number(v)` + existing currency formatter), masked (“—” + lock tooltip `assets.masked`) when the field is absent; holder column: show `current_holder_employee_id` resolved only if trivially available, else “—” (assignment module not built).
  - Remove: delete action, its confirm modal usage, `ASSET_*` mock imports (drop the temporary mock exports from Task 2 if now unused), client-side filter/sort helpers.
- [ ] **Step 4:** Task spec green; FULL `pnpm test` exit 0; `pnpm lint && pnpm typecheck`.
- [ ] **Step 5: Compare 1:1 vs `docs/design/Katalog Aset.dc.html`** (light+dark, all states) — fix gaps.
- [ ] **Step 6: Commit** — `feat(assets): wire Katalog Aset to GET /assets (server-side list)`

---

### Task 7: Detail (`assets/[tag].vue`)

**Files:**
- Rewrite data layer of: `frontend/app/pages/assets/[tag].vue`
- Modify: i18n (tab empty-states, gallery empty, masked)
- Test: REWRITE `frontend/test/nuxt/assets-detail.spec.ts`

**Interfaces:**
- Consumes: `useAssets().getByTag`, `useAssetAttachments()` (list + thumbnailBlob), `useCategories`, `useOffices`, `useFloors` (floors by office + rooms by floor — for room name), `useReference` (brands/models/vendors/units name maps).
- Mockup: `docs/design/Detail Aset.dc.html`. Approved deviation: tabs Penugasan/Maintenance/Depresiasi = empty-state cards.

- [ ] **Step 1: Failing component test** — stubbed `getByTag` renders name/tag/status badge and Info fields (serial, tanggal, PO, funding, warranty, notes; FK names resolved from stubbed lookups); `purchase_cost` present → formatted value, absent → lock/“—” (`assets.masked`); the three history tabs render empty-state text (i18n `assets.detail.moduleNotAvailable`), no static sample rows; gallery: stubbed attachments list with `has_thumbnail` renders `<img>` per photo from object URL, empty list → `assets.detail.noPhotos`; unknown tag (404 from getByTag) → not-found card; NO delete button.
- [ ] **Step 2: Run** → fail. **Step 3: Rewrite:**
  - Load by `route.params.tag` → `getByTag`; 404 → notFound state.
  - Lookups in parallel (`Promise.all`, each guarded): categories, offices (name maps), reference brands/models/vendors/units, and room resolution: fetch floors for `asset.office_id`, then rooms per floor, find `asset.room_id` (skip when null).
  - Gallery: `list(asset.id)` → for image-kind rows with `has_thumbnail`, `thumbnailBlob` → `URL.createObjectURL`; revoke on unmount. Empty state per mockup.
  - Tabs: keep the mockup's four tabs; Penugasan/Maintenance/Depresiasi bodies = `UCard` empty state ("Belum ada data — modul belum tersedia" / "No data yet — module not available"). Delete `depreciationSchedule`/`sampleAssignments`/`sampleMaintenance` mock imports.
  - Sensitive rows (harga/akumulasi/nilai buku): value when key present, masked row when absent — the hardcoded `sensitive: true` lock logic is replaced by `asset[key] === undefined` checks.
  - Remove delete action + its i18n.
- [ ] **Step 4:** Spec green; FULL suite exit 0; lint+typecheck. **Step 5:** 1:1 vs `Detail Aset.dc.html` (light+dark). **Step 6: Commit** — `feat(assets): wire Detail Aset (by-tag, masking, attachments gallery, tab empty-states)`

---

### Task 8: Form (`AssetForm.vue` + `new.vue` + `[tag]/edit.vue`)

**Files:**
- Rewrite: `frontend/app/components/asset/AssetForm.vue`; modify `frontend/app/pages/assets/new.vue`, `frontend/app/pages/assets/[tag]/edit.vue`
- Modify: i18n (request-submitted flow, read-only hints, tag-auto hint, deferred-lampiran hint)
- Test: REWRITE `frontend/test/nuxt/assets-form.spec.ts`

**Interfaces:**
- Consumes: `useAssetRequests().submitCreate`, `useAssets().update/getByTag`, `useAssetAttachments()` (edit mode), `useCategories`, `useOffices`, `useFloors` (office→floors→rooms cascade), `useReference` (brands, models filtered client-side by `brand_id`, units, vendors).
- Mockup: `docs/design/Form Aset.dc.html` (sections Identitas/Penempatan/Pembelian/Depresiasi/Lampiran).
- Route guards: `new.vue` → `request.create`; `[tag]/edit.vue` → `asset.manage`; katalog/detail/label → `asset.view` (set here for all five pages in one sweep).

- [ ] **Step 1: Failing component tests:**
  - **new**: fill required (name, kategori, kantor, class, harga, tanggal) → submit calls `submitCreate` with `AssetCreateInput` (assert `purchase_cost` decimal **string**, all filled optional FK ids present); success → toast pengajuan + emitted/redirect; API error → form error banner; tag field shows auto-generated hint (no client preview); lampiran dropzone disabled with the deferred hint; ruangan picker only enabled after kantor chosen; model options filtered by chosen brand.
  - **edit**: initial from a stubbed `Asset`; `purchase_cost`/`asset_class`/`status`/tag rendered read-only; submit calls `update(id, body)` with ONLY `AssetUpdateInput` keys; lampiran section renders real list + upload/delete calls.
  - required-field validation errors (existing i18n error keys).
- [ ] **Step 2: Run** → fail. **Step 3: Rewrite `AssetForm`:**
  - Props stay `{ mode: 'new' | 'edit', initial?: Asset }`.
  - Pickers (USelect/USelectMenu with `data-testid` like employees): kategori (tree from `useCategories`), kantor (`useOffices().list({limit:100})`), lantai→ruangan cascade (`useFloors`), brand/model/unit/vendor (`useReference`); delete every hardcoded KATEGORI/KANTOR/BRAND/... array and the KANTOR_CODE/KAT_CODE tag preview.
  - Depresiasi section per mockup stays read-only informational (derived from chosen category's depreciation fields when available).
  - Submit new → build `AssetCreateInput` (dates `YYYY-MM-DD`; `purchase_cost: String(harga)`), `submitCreate`, success toast `assets.form.requestSubmitted` ("Pengajuan terkirim — menunggu persetujuan"), redirect Katalog. Maker-checker banner text now reflects the real flow.
  - Submit edit → `update(initial.id, body)` (strip non-updatable keys), toast, redirect detail.
  - Lampiran: edit-mode live (list/upload/remove via composable, accept JPG/PNG/PDF); new-mode disabled dropzone + hint `assets.form.attachmentsAfterApproval`.
- [ ] **Step 4:** Update the two wrapper pages (`new.vue` permission; `edit.vue` loads via `getByTag` and passes `initial`). Set the corrected `can` permissions on all five asset pages.
- [ ] **Step 5:** Specs green; FULL suite exit 0; lint+typecheck. **Step 6:** 1:1 vs `Form Aset.dc.html`. **Step 7: Commit** — `feat(assets): wire AssetForm — create via approval request, restricted edit`

---

### Task 9: Label (`assets/label.vue`)

**Files:**
- Rewrite data layer of: `frontend/app/pages/assets/label.vue`
- Modify: i18n (remove comingSoon for print/PDF)
- Test: REWRITE `frontend/test/nuxt/assets-label.spec.ts`

**Interfaces:**
- Consumes: `useAssets().list` (search picker), `useApiClient().requestBlob` for `GET /assets/{id}/barcode?type=code128|qr` previews and `POST /assets/labels`.
- Mockup: `docs/design/Label Barcode.dc.html`.

- [ ] **Step 1: Failing component test** — picker searches via stubbed `list({search})`; selecting assets renders preview tiles whose barcode `<img>` comes from stubbed `requestBlob` object URLs; Cetak triggers `requestBlob('/assets/labels', { method: 'POST', body: … })` with `{ asset_ids, template, layout, mode, fields }` matching the UI controls and triggers a download named `labels.pdf`; selection cap 500 enforced (i18n warning).
- [ ] **Step 2: Run** → fail. **Step 3: Implement** — replace `assetStore.all()` with server search; barcode previews lazy-fetched per selected asset (cache by `id+type`, revoke object URLs on unmount); PDF download via anchor-click on object URL. Keep the mockup's layout/controls.
- [ ] **Step 4:** Spec green; FULL suite exit 0; lint+typecheck. **Step 5:** 1:1 vs `Label Barcode.dc.html`. **Step 6: Commit** — `feat(assets): wire Label & Barcode to real barcode/label endpoints`

---

### Task 10: e2e + final verification + docs

**Files:**
- Rewrite: `frontend/e2e/assets.spec.ts`
- Modify: `docs/PROGRESS.md`

**Interfaces:**
- Consumes: full stack (`docker compose -f docker-compose.dev.yml up -d` + backend/frontend on host, or compose app profile), seeded admin `admin@inventra.local/admin12345`.

- [ ] **Step 1: Rewrite `e2e/assets.spec.ts`** (real backend; memory: unique-per-run names + assert-after-search + wait-modal-closed):
  1. **API setup via `request` fixture** (login as admin → token): create prerequisite FKs with unique-per-run codes (office type→office if none usable, category) — reuse the setup-helper style of `e2e/master-offices.spec.ts`; create a **checker user**: `GET /authz/roles` → pick the seeded superadmin role id → `POST /users` (unique email, known password).
  2. **Create flow (SoD)**: as admin, `POST /requests` type `asset_create`, small `amount` (e.g. `"500000"` — stays in the lowest threshold band so ONE approval completes it), payload with unique asset name; login as checker via API → `GET /requests/inbox` → find the request → `POST /requests/:id/approve` `{decision:'approved'}`.
  3. **UI assertions**: login (UI) as admin → Katalog → search the unique name → row appears with status Tersedia → open Detail (fields render; money visible for superadmin) → Edit: change `notes`/name suffix → save → detail reflects it → Label page: pick the asset, assert the Cetak call returns a PDF (`response.headerValue('content-type')` = `application/pdf` via `page.waitForResponse` on `/assets/labels`).
  4. Negative: Katalog search for a random non-existent string → empty state.
- [ ] **Step 2: Run e2e** — stack up + `pnpm test:e2e` → green.
- [ ] **Step 3: Full gates once more** — backend: build/vet/test/integration/Spectral; frontend: lint/typecheck/test/build.
- [ ] **Step 4: Update `docs/PROGRESS.md`** — Next-session block: add wire-batch entry "Assets cluster wired (Katalog/Detail/Form/Label) + AssetCreatePayload widened" with the deliberate deferrals (Import still mock — backend absent; Approval screen still mock; delete → disposal batch; holder column pending assignment module); tick/annotate the matching *Remaining → Wire screens* bullet; point Next-session at the next real step (e.g. approval-screen wiring or stock opname backend).
- [ ] **Step 5: Commit** — `feat(assets): real-backend e2e for assets cluster + progress update`

---

## Self-review notes (done)

- Spec coverage: payload widen (T1), types/constants/mock-decouple (T2), requestBlob (T3), useAssets (T4), requests+attachments composables (T5), Katalog (T6), Detail incl. gallery+empty tabs (T7), Form incl. deferred lampiran + permissions sweep (T8), Label (T9), e2e/SoD + gates + PROGRESS (T10). Out-of-scope list respected.
- Type consistency: `AssetCreateInput`/`AssetUpdateInput`/`AssetAttachment`/`SubmittedRequest` defined once (T2/T5) and consumed by name in T6–T10; `requestBlob` signature fixed in T3, used in T5/T9.
- Known risk called out where it bites: full-suite exit-code check after every task (memory #40); threshold band in e2e amount; temporary mock exports keep intermediate task boundaries compiling.
