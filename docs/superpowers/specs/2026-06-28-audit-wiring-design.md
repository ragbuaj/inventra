# Wire Audit Trail screen to `/api/v1/audit` — Design

| | |
|---|---|
| **Tanggal** | 2026-06-28 |
| **Area** | Frontend (Nuxt 4) — `pages/settings/audit.vue` + `composables/api/useAudit.ts` |
| **Backend** | `GET /api/v1/audit` (live; gated `audit.view`, office-scoped) |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & lingkup

Layar **Audit Trail** (`frontend/app/pages/settings/audit.vue`) menampilkan jejak audit (`audit.audit_logs`) read-only. Saat ini mock-first dengan filter + paginasi **client-side** atas dataset penuh. Sub-proyek ini mewire **layar Audit Trail saja** ke backend nyata, pindah ke filter + paginasi **server-side**. User Management adalah sub-proyek terpisah berikutnya. Mengikuti pola wiring authz (id identity, DTO English, constants+i18n+fallback, gate penuh).

### Kendala backend (menentukan desain)
- Respons `auditToMap`: `{ id, entity_type, entity_id, action, ip, changes:{field:{before,after}}, actor:{id,name,email}, office_id, created_at }`. Envelope `{ data, total, limit, offset }`.
- Backend **tidak** mengembalikan `role` atau `summary` (ada di mockup) → kolom itu **di-drop**.
- Backend mengembalikan `office_id` (UUID), **bukan** nama kantor. Memuat daftar kantor butuh izin masterdata yang belum tentu dimiliki pemegang `audit.view` → kolom **kantor di-drop** (hindari lookup ter-gate/403).
- Filter actor backend memakai `actor_id` (UUID); memuat daftar user butuh `user.manage` → **dropdown actor di-drop**. Actor tetap **ditampilkan** (dari `actor.name` di respons), hanya tak bisa difilter.
- Param `search` mencocokkan `entity_type ILIKE` **OR** `entity_id::text ILIKE` (BUKAN nama actor) → placeholder pencarian diperjelas (cari entity/ID).
- Query params yang didukung: `search`, `entity_type`, `action`, `from` (RFC3339), `to` (RFC3339), `limit` (default 20, clamp 1–100), `offset`.

Keputusan ini **disetujui** (Q2 opsi swadaya) — penyimpangan dari mockup setara dengan deviasi Data Scope/Field Permission.

## 2. Composable `useAudit.ts` — tulis ulang

Hapus ketergantungan `~/mock/audit` + `fakeLatency`. Tipe:
```ts
type AuditAction = 'create' | 'update' | 'delete'
interface AuditChange { before?: unknown; after?: unknown }
interface AuditRow {
  id: string
  created_at: string      // raw RFC3339
  date: string            // derived display date
  time: string            // derived HH:mm
  actor_name: string
  actor_email: string
  initials: string        // from actor_name
  action: AuditAction
  entity_type: string     // raw key (label resolved in page via i18n)
  entity_id: string
  ip: string
  changes: Record<string, AuditChange>
}
interface AuditListParams {
  search?: string; entity_type?: string; action?: AuditAction
  from?: string; to?: string; limit: number; offset: number
}
```
Fungsi (via `useApiClient().request`):
- `async list(params: AuditListParams): Promise<{ rows: AuditRow[]; total: number }>` — build a query string from non-empty params; `GET /api/v1/audit?…`; map each `auditToMap` row → `AuditRow` (derive `date`/`time` from `created_at`; `initials` from `actor.name`; flatten `actor.{name,email}`; pass `changes` through). Return `{ rows, total }`.

Drop the mock `actors()` (no actor filter).

## 3. Katalog entity + i18n

- `app/constants/auditCatalog.ts`:
  ```ts
  export const AUDIT_ENTITY_TYPES = [
    'assets','users','roles','role_permissions','data_scope_policies','field_permissions',
    'offices','employees','categories','floors','rooms','requests','asset_attachments','asset_documents'
  ] as const
  ```
  (The real `entity_type` values recorded by `audit.Record(...)` across the backend.)
- i18n (`settings.audit`): `entity.<key>` labels (id/en) for the 14 entity types; reuse existing `action.*` labels. Add `loadError`, `retry`. Entity labels resolve via `te()/t()` with **fallback to the raw key**.

## 4. Page `audit.vue` — server-side

- **Gate fix**: `definePageMeta({ middleware:'can', permission: 'audit.view' })` (was `user.manage`; the backend route is gated by `audit.view`, so the screen must match — this also lets non-Superadmin audit viewers reach it).
- **Filters** (reactive state): `search` (text → entity/id), `entity_type` (dropdown from `AUDIT_ENTITY_TYPES`, "all" sentinel = no filter), `action` (create/update/delete, "all" sentinel), `from`/`to` (date inputs → RFC3339 day bounds). NO actor dropdown.
- **Server-side**: `page` (1-based) + `pageSize` (20). A `load()` calls `useAudit().list({...filters, limit:pageSize, offset:(page-1)*pageSize})`. `watch` the filters → reset `page=1` + `load()`; `watch` `page` → `load()`. Display `total` for pagination controls.
- **Columns**: waktu (`date` + `time`), actor (`actor_name` + `initials` avatar), action (badge via existing action meta), entity (`entityLabel(entity_type)` i18n+fallback), IP. **Expandable row** renders the diff from `changes` (iterate `Object.entries(changes)` → field, before, after). Drop role/summary/office columns.
- **States**: loading, `loadFailed` error + retry, empty. Export button stays a disabled/stub (no backend export endpoint).
- Entity/action label helpers via `te()/t()` fallback. `actor_name` from response (no lookup).

## 5. Pengujian (proaktif & luas)

- **Unit** (`test/unit/use-audit.spec.ts`, mock `~/composables/useApiClient`; hapus `audit-mock.spec.ts`):
  - `list` builds the query with exactly the non-empty params (entity_type/action/from/to/search/limit/offset); omits empties.
  - maps `auditToMap` → `AuditRow`: `actor_name`/`actor_email` flattened, `initials` derived, `date`/`time` derived from `created_at`, `changes` passed through, `total` returned.
- **Component** (`test/nuxt/settings-audit.spec.ts`, stub `/audit`):
  - render rows: actor name, action badge, entity i18n label, IP.
  - changing the entity filter / action filter triggers a new `GET /audit` with the right query (assert captured query); changing page sends the right `offset`; a filter change resets to page 1.
  - expand a row → renders the `changes` diff (a field's before/after).
  - load-error (500) → error block + retry; empty state when `data:[]`.
- **E2E** (`e2e/settings.spec.ts`): backend nyata + admin seeded — Audit screen lists real rows (the seeded admin's recent actions, e.g. a login or a seeded mutation); filter by action; pagination control present. Robust text/role locators.

## 6. "Selesai"

- `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau (E2E di CI).
- Hapus `mock/audit.ts` bila tak lagi diacu (cek importer; unit test pindah ke `use-audit.spec.ts`).
- Bandingkan ke `docs/design/Audit Trail.dc.html`: kolom **role/summary/kantor** + **dropdown actor** sengaja di-drop (backend tak menyediakan datanya / butuh lookup ter-gate) — penyimpangan disetujui; layout/filter-bar/diff-viewer/paginasi selebihnya cocok.
- `docs/PROGRESS.md`: Audit Trail wired ke `/api/v1/audit` (server-side filter+paginasi; gate `audit.view`); catat TODO: filter actor (butuh user-list yang dapat diakses audit-viewer) + nama kantor (butuh offices-lookup) ditunda; refresh "▶ Next session — start here" → **User Management** berikutnya.
