# Wire Field Permission screen to `/authz` API — Design

| | |
|---|---|
| **Tanggal** | 2026-06-28 |
| **Area** | Frontend (Nuxt 4) — `pages/settings/field-permission.vue` + `composables/api/useFieldPermission.ts` |
| **Backend** | `/api/v1/authz/*` (live); RBAC (#34) + Data Scope (#35) already wired |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & lingkup

Layar **Field Permission** (`frontend/app/pages/settings/field-permission.vue`) mengelola `field_permissions` per `(entity, field, role)` dengan flag `can_view`/`can_edit`. Saat ini mock-first. Sub-proyek ini mewire **layar Field Permission saja** ke `/authz`. Mengikuti pola RBAC/Data Scope: id identity, DTO English, constants+i18n+fallback, eager load, gate penuh.

Endpoint:
- `GET /authz/roles` → `{ data:[Role], total }`.
- `GET /authz/roles/:id/fields` → `{ fields:[{entity, field, can_view, can_edit}] }`.
- `PUT /authz/roles/:id/fields` → body `{ fields:[{entity, field, can_view, can_edit}] }` (replace-set **lintas semua entity** untuk peran itu).

Gerbang layar tetap `definePageMeta({ middleware:'can', permission:'user.manage' })`.

### Keputusan & batasan (disetujui)
- **Entity yang dijadikan field-maskable = `assets` + `users`** — satu-satunya entity yang backend benar-benar `FilterView`-kan saat ini (`asset` & `user` handler memanggil `ForEntity(role,"assets"|"users")`). Aturan pada entity/field yang tak cocok key serialisasi backend tak berefek, jadi katalog HARUS memakai key nyata.
- **Field = key map serialisasi backend nyata** (English), bukan kode Indonesia mock. Subset kurасi per entity (§2).
- **Kolom peran dinamis** dari `GET /authz/roles` (UUID id + name), bukan 5 key tetap mock.
- **Default-allow**: tak ada policy = `view+edit`. Hanya sel **restriksi** (`can_view=false` atau `can_edit=false`) yang disimpan; sel full-allow dihilangkan.
- **Simpan = replace-set per-peran lintas SEMUA entity** → menyimpan satu entity harus **mempertahankan** rules entity lain (termasuk entity tak-dikenal katalog).
- **Entity-agnostic**: page merender apa pun yang ada di katalog; menambah entity nanti = edit konstanta + enforcement backend, tanpa ubah struktur page.
- **TODO dicatat di PROGRESS.md**: perluas enforcement `FilterView` ke `requests` (handler approval sudah menyuntik `fieldSvc` + punya `requestToMap` — murah), `employees` (perlu wire `fieldSvc`+map), serta modul lain. Saat ini hanya assets+users yang field-masked.

### Pola seam
`useApiClient().request<T>` (Bearer + refresh-on-401 + error toast). `useRbac`/`useDataScope` (PR #34/#35) adalah preseden.

## 2. Katalog field (frontend, entity-agnostic)

`app/constants/fieldCatalog.ts`:
```ts
export interface CatalogEntity { entity: string; fields: string[] }
export const FIELD_CATALOG: CatalogEntity[] = [
  { entity: 'assets', fields: [
    'name','category_id','office_id','serial_number','purchase_date',
    'purchase_cost','book_value','accumulated_depreciation','salvage_value','impairment_loss',
    'depreciation_method','po_number','funding_source','warranty_expiry','status','notes'
  ] },
  { entity: 'users', fields: ['name','email','role_id','office_id','employee_id','status'] }
]
```
Key = key serialisasi nyata (`assetToMap`/`userToMap`). Label entity & field via i18n dengan fallback ke key (§5).

## 3. Composable `useFieldPermission.ts` — tulis ulang

Hapus ketergantungan `~/mock/fieldPermission` + `fakeLatency`. Tipe:
```ts
interface EntityView { entity: string; fields: string[] }       // dari katalog
interface RoleColumn { id: string; name: string }
interface FieldRow { entity: string; field: string; can_view: boolean; can_edit: boolean }
```
Fungsi (via `useApiClient().request`):
- `getEntities(): EntityView[]` — dari `FIELD_CATALOG`.
- `async getRoleColumns(): Promise<RoleColumn[]>` — `GET /authz/roles` → `data.map(r => ({ id:r.id, name:r.name }))`.
- `async loadFields(roleIds: string[]): Promise<Record<string, FieldRow[]>>` — eager paralel `GET /authz/roles/:id/fields` per peran → `{ roleId → fields[] }` (rules lengkap lintas entity).
- `async saveRoleFields(id: string, rows: FieldRow[]): Promise<void>` — `PUT /authz/roles/:id/fields { fields: rows }`.

## 4. Pivot & simpan lintas-entity (inti)

State page (memegang data mentah agar simpan bisa rekonstruksi):
- `roleColumns: RoleColumn[]`, `entities = getEntities()`, `roleFields: Record<roleId, FieldRow[]>` (dari `loadFields`), `selectedEntity`, `dirtyRoleIds: Set<string>`.
- **Grid** entity terpilih `E`: baris = `catalog[E].fields`; kolom = `roleColumns`; sel = cari `roleFields[roleId]` dengan `entity===E && field===f` → bila ada, `{can_view,can_edit}`; bila tidak → **Default** (`view+edit`, badge "Default").
- Toggle sel mengubah representasi grid lokal + menandai `dirtyRoleIds.add(roleId)`.
- **Save**: untuk tiap `roleId ∈ dirtyRoleIds`:
  `newRows = roleFields[roleId].filter(r => r.entity !== E)`  *(pertahankan entity lain apa adanya)*
  `+ gridCellsFor(E, roleId).filter(c => !(c.can_view && c.can_edit)).map(c => ({entity:E, field:c.field, can_view, can_edit}))`  *(hanya restriksi entity E)*
  → `saveRoleFields(roleId, newRows)`; lalu update `roleFields[roleId] = newRows`. Save paralel (`Promise.all`), lalu clear `dirtyRoleIds`.

Catatan: grid hanya menyentuh entity `E`; entity lain (termasuk yang tak ada di katalog) ikut terbawa utuh → tak ada rules hilang.

## 5. Page + komponen + konstanta/i18n

- `field-permission.vue`: selector entity (dari katalog), grid (field × kolom peran), `FieldPermToggle` per sel (view/edit), `dirtyRoleIds` + Save (PUT hanya peran berubah, paralel), state loading/error+retry, badge "Default" untuk field tanpa restriksi. id identity di seluruh binding.
- `components/fieldperm/FieldPermToggle.vue`: tak berubah selain sumber tipe `CellRule` (pindah dari mock ke constants bila perlu).
- Konstanta UI yang bocor dari mock (`FIELD_ROLE_KEYS`, `CellRule`) → pindah ke `~/constants/fieldCatalog.ts` atau hilangkan (kolom peran kini dari API).
- i18n (`settings.fieldPermission`): `entity.<key>` (assets/users), `field.<field>` (**flat** — satu peta label field; key sama lintas entity mis. `name`/`status`/`office_id` berbagi label, wajar), `loadError`, `retry`. Label entity & field di-resolve via `te()/t()` dengan **fallback ke key mentah** (sehingga field tanpa entri i18n tetap tampil).

## 6. Pengujian (proaktif & luas)

- **Unit** (`use-field-permission.spec.ts`, mock `useApiClient`; hapus `field-permission-mock.spec.ts`):
  - `getEntities` dari katalog; `getRoleColumns` map id/name; `loadFields` per-peran.
  - `saveRoleFields` body persis.
  - **Rekonstruksi simpan** (uji murni helper): entity lain dipertahankan; hanya sel restriksi entity E disimpan; sel full-allow dibuang.
- **Component** (`settings-field-permission.spec.ts`, stub `/authz`):
  - render grid: entity nyata (Aset/User via i18n), kolom = peran seeded, field rows; field tanpa restriksi → badge "Default".
  - toggle view/edit → dirty + Save aktif; Save mem-`PUT /authz/roles/:id/fields` dengan body yang **mempertahankan entity lain** + **hanya restriksi entity terpilih** (assert captured body).
  - hanya peran berubah yang di-PUT; load-error→retry.
- **E2E** (`settings.spec.ts`): backend nyata + admin seeded — grid tampil; ubah satu sel + Save → persist setelah reload (locator berbasis teks/role).

## 7. "Selesai"

- `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau (E2E di CI).
- Hapus `mock/fieldPermission.ts` bila tak lagi diacu.
- Bandingkan 1:1 ke `docs/design/Field Permission.dc.html` (entity & field = set backend nyata: penyimpangan disengaja & disetujui; selebihnya layout/grid/toggle/states cocok).
- **PROGRESS.md**: Field Permission wired (assets+users); tambah TODO eksplisit untuk memperluas enforcement `FilterView` ke `requests`/`employees`/modul lain; refresh "▶ Next session — start here" (rangkaian wiring authz selesai → lanjut backend bank-FAM, mis. asset transfer/mutasi, atau enforcement field-perm tambahan).
