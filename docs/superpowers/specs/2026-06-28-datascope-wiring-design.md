# Wire Data Scope screen to `/authz` API — Design

| | |
|---|---|
| **Tanggal** | 2026-06-28 |
| **Area** | Frontend (Nuxt 4) — `pages/settings/data-scope.vue` + `composables/api/useDataScope.ts` |
| **Backend** | `/api/v1/authz/*` (live, PR #33); RBAC screen already wired (PR #34) |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & lingkup

Layar **Data Scope** (`frontend/app/pages/settings/data-scope.vue`) mengelola kebijakan `data_scope_policies` per peran: satu **default** (sentinel module `*`) + **override per-modul**. Saat ini mock-first (`useDataScope` + `mock/dataScope.ts`). Sub-proyek ini mewire **layar Data Scope saja** ke `/authz`. Field Permission adalah sub-proyek terpisah. Mengikuti pola wiring RBAC (PR #34): id identity, DTO English, constants+i18n+fallback, eager load, gate penuh.

Endpoint:
- `GET /authz/catalog` → `{ …, scope_levels:["global","office_subtree","office","own"], scope_modules:["*","offices","employees","assets","requests","audit"] }`.
- `GET /authz/roles` → `{ data:[Role], total }`.
- `GET /authz/roles/:id/scope` → `{ policies:[{module, scope_level}] }`.
- `PUT /authz/roles/:id/scope` → body `{ policies:[{module, scope_level}] }` (replace-set).

Gerbang layar tetap `definePageMeta({ middleware:'can', permission:'user.manage' })` (penyelarasan ke `scope.manage` di luar lingkup, konsisten dengan RBAC).

### Keputusan produk (disetujui)
- **Kolom modul = `scope_modules` backend nyata** (`offices/employees/assets/requests/audit`; `*` jadi kolom "Default"), bukan 5 modul ilustratif mockup. Akurat terhadap enforcement; otomatis ikut bila backend menambah modul. Label via i18n + fallback ke key modul.
- **Simpan hanya peran yang berubah** (lacak `dirtyIds`), bukan semua peran sekaligus.
- `ScopeRoleView.sub` = role `description` (boleh kosong).

## 2. Composable `useDataScope.ts` — tulis ulang

Hapus ketergantungan `~/mock/dataScope` + `fakeLatency`. Tipe English:

```ts
type ScopeLevel = 'global' | 'office_subtree' | 'office' | 'own'
interface ScopeModuleView { key: string }                 // label di-resolve di page via i18n
interface ScopeRoleView { id: string; code: string; name: string; sub: string; def: ScopeLevel; ov: Record<string, ScopeLevel> }
```

Fungsi (via `useApiClient().request`):
- `async getModules(): Promise<ScopeModuleView[]>` — `GET /authz/catalog` → `scope_modules.filter(m => m !== '*').map(key => ({ key }))`.
- `async listRoles(): Promise<ScopeRoleView[]>` — `GET /authz/roles` → untuk tiap peran **eager paralel** `GET /authz/roles/:id/scope`; map: `def = policies.find(p => p.module === '*')?.scope_level ?? 'own'`, `ov = Object.fromEntries(policies.filter(p => p.module !== '*').map(p => [p.module, p.scope_level]))`. `sub = role.description ?? ''`.
- `async saveRoleScope(id: string, def: ScopeLevel, ov: Record<string, ScopeLevel>): Promise<void>` — `PUT /authz/roles/:id/scope` dengan `policies = [{ module:'*', scope_level: def }, ...Object.entries(ov).map(([module, scope_level]) => ({ module, scope_level }))]`. **Selalu sertakan `*`** agar default persist (replace-set: tanpa `*`, enforcement jatuh ke `own`).

Kembalikan `{ getModules, listRoles, saveRoleScope }`.

## 3. Konstanta + i18n (lepas kebocoran mock)

- `app/constants/dataScope.ts`:
  ```ts
  export const SCOPE_LEVEL_KEYS = ['global', 'office_subtree', 'office', 'own'] as const
  export type ScopeLevel = typeof SCOPE_LEVEL_KEYS[number]
  export const SCOPE_LEVEL_TONE: Record<ScopeLevel, 'info'|'primary'|'warning'|'neutral'> = {
    global: 'primary', office_subtree: 'info', office: 'warning', own: 'neutral'
  }
  ```
  (Nilai tone disesuaikan ke padanan mockup saat implementasi via perbandingan `Data Scope.dc.html`.)
- i18n (`i18n/locales/{id,en}.json`, di bawah `settings.dataScope`):
  - `level.<key>` (global/office_subtree/office/own) — deskripsi level (id/en), dipakai legend.
  - `module.<key>` (offices/employees/assets/requests/audit) — label kolom (id/en).
  - `loadError`, `retry`.
- Page & `ScopeCell` impor `SCOPE_LEVEL_KEYS`/`SCOPE_LEVEL_TONE`/`ScopeLevel` dari `~/constants/dataScope` (bukan dari `~/mock/dataScope`). Label modul & deskripsi level di-resolve via i18n di page (modul: `te()`-fallback ke key).

## 4. Page `data-scope.vue`

- **id identity**: `findRole(id)`, `setDefault(id, level)`, `setOverride(id, mod, level)`, `clearOverride(id, mod)`; `:key="r.id"`; `{{ r.name }}` / `{{ r.sub }}`.
- **Kolom**: `modules` (dari `getModules`, modul backend nyata); header kolom modul → `moduleLabel(m.key)` (i18n + fallback ke key); kolom "Default" tetap.
- **Dirty tracking per-peran**: `const dirtyIds = ref(new Set<string>())`; tiap `setDefault/setOverride/clearOverride` menambah id ke set + `dirty=true`. `save()` → `Promise.all([...dirtyIds].map(id => saveRoleScope(id, role.def, role.ov)))`; sukses → `dirtyIds.clear()`, `dirty=false`, toast.
- **State**: tambah `loadFailed` → blok error + tombol retry (pola RBAC). Legend membaca `SCOPE_LEVEL_KEYS` + `SCOPE_LEVEL_TONE` + `t('settings.dataScope.level.<key>')`.
- `ScopeCell` tak berubah selain sumber impor `ScopeLevel`/konstanta.

## 5. Pengujian (proaktif & luas)

- **Unit** (`test/unit/use-data-scope.spec.ts`, mock `~/composables/useApiClient`; hapus `data-scope-mock.spec.ts`):
  - `getModules` membuang `*` dan memetakan sisanya.
  - `listRoles` memetakan policies→`def`/`ov`; peran tanpa policy `*` → `def='own'`; override dipetakan; `sub`=description.
  - `saveRoleScope` mengirim `policies` benar: selalu mengandung `{module:'*', scope_level:def}` + tiap override; assert body persis.
- **Component** (`test/nuxt/settings-data-scope.spec.ts`, stub `/authz`):
  - loading→render grid: kolom = modul backend (label ter-i18n, mis. "Kantor"/"Aset"), baris = peran seeded.
  - ubah default → dirty + Save aktif; Save mem-`PUT /authz/roles/:id/scope` dengan body yang mengandung `*`=level baru (assert captured body); dirty clear.
  - set override pada satu modul → body PUT mengandung override itu; clear override → body tanpa modul itu.
  - **hanya peran berubah** yang di-PUT (ubah 1 peran → tepat 1 PUT).
  - load-error (GET roles 500) → blok error + retry; retry pulih.
- **E2E** (`e2e/settings.spec.ts`): backend nyata + admin seeded — grid Data Scope tampil dengan peran bawaan; ubah satu sel scope + Save; reload → perubahan tetap. Spec lain tak disentuh.

## 6. "Selesai"

- `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau (E2E di CI).
- Hapus `mock/dataScope.ts` bila tak lagi diacu (cek importer; `ScopeCell`/page kini dari constants).
- Bandingkan 1:1 ke `docs/design/Data Scope.dc.html` (kolom modul = set backend: penyimpangan disengaja & disetujui; selebihnya layout/legend/state cocok). Perbaiki deviasi lain.
- Update `docs/PROGRESS.md`: Data Scope wired ke `/authz`; refresh "Next session" → **Field Permission** berikutnya.
