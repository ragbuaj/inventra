# Wire RBAC screen to `/authz` API — Design

| | |
|---|---|
| **Tanggal** | 2026-06-28 |
| **Area** | Frontend (Nuxt 4) — `pages/settings/rbac.vue` + `composables/api/useRbac.ts` |
| **Backend** | `/api/v1/authz/*` (sudah ada, PR #33) |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & lingkup

Layar **Peran & RBAC** (`frontend/app/pages/settings/rbac.vue`) saat ini mock-first (`useRbac` + `mock/rbac.ts`). Backend authz-admin sudah live (`/authz/catalog`, `/authz/roles`, `/authz/roles/:id/permissions`). Sub-proyek ini **mewire layar RBAC saja** ke API real. Data Scope & Field Permission adalah sub-proyek terpisah (spec sendiri). Tidak menyentuh composable lain atau regroup folder (sisa ADR-0007 ditunda); rename key Indonesia→English diterapkan **hanya** pada `useRbac` saat ditulis ulang.

Endpoint yang dipakai:
- `GET /authz/catalog` → `{ permissions:[{group, items:[{key,label}]}], scope_levels:[…], scope_modules:[…] }`.
- `GET /authz/roles` → `{ data:[Role], total }`, `Role = {id, code, name, description, is_system, created_at, updated_at}`.
- `POST /authz/roles` → body `{code, name, description?}` → 201 `Role`.
- `GET /authz/roles/:id/permissions` → `{ permissions:[key] }`.
- `PUT /authz/roles/:id/permissions` → body `{ permissions:[key] }` → `{ permissions:[…] }`.

Gerbang layar tetap `definePageMeta({ middleware:'can', permission:'user.manage' })` — **tidak diubah** (catatan: backend menggerbang `/authz/*` dengan `role.manage`; UI memakai `user.manage` yang juga dimiliki Superadmin — konsisten dengan layar settings lain. Penyelarasan gate UI→`role.manage` di luar lingkup ini).

### Pola seam yang diikuti
`frontend/app/composables/useApiClient.ts` (`request<T>(path, opts)`) — inject Bearer + `X-Request-ID`, refresh-on-401, toast error. `useAuthApi.ts` adalah contoh composable yang sudah real (mis. `GET /auth/permissions`). `useRbac` ditulis ulang mengikuti pola ini.

## 2. Composable `useRbac.ts` — tulis ulang

Hapus ketergantungan ke `mock/rbac.ts` & `fakeLatency`. Adopsi key English. Tipe baru:

```ts
interface PermissionItem { key: string; label: string }      // label = fallback dari katalog
interface ModuleView { group: string; icon: string; items: PermissionItem[] }
interface RoleView { id: string; code: string; name: string; is_system: boolean; description?: string }
interface CreateRoleInput { name: string; description?: string; copyFromId?: string }
```

Fungsi (semua via `useApiClient().request`):
- `async getCatalog(): Promise<ModuleView[]>` — `GET /authz/catalog`; map tiap `group`→`{group, icon: iconForGroup(group), items}`. `icon` & label tampilan di-resolve di layer presentasi (bagian 3); composable mengembalikan data katalog mentah + ikon grup.
- `async listRoles(): Promise<RoleView[]>` — `GET /authz/roles`, kembalikan `data`.
- `async getRolePermissions(id: string): Promise<string[]>` — `GET /authz/roles/:id/permissions` → `.permissions`.
- `async createRole(input: CreateRoleInput): Promise<RoleView>` — derive `code = slugify(input.name)` (bagian 4); `POST /authz/roles {code, name, description}`; bila `copyFromId` → `getRolePermissions(copyFromId)` lalu `updateRolePermissions(newRole.id, perms)`; kembalikan role baru.
- `async updateRolePermissions(id: string, perms: string[]): Promise<void>` — `PUT /authz/roles/:id/permissions {permissions: perms}`.

`slugify(name)` → lowercase, spasi/karakter non-alfanumerik → `_`, trim `_` beruntun (mis. "Auditor Cabang" → `auditor_cabang`). Murni, dapat diuji unit.

## 3. Label & ikon (peta presentasi frontend)

Backend katalog memberi `group` (string Indonesia: "Sistem"/"Master Data"/"Aset"/"Persetujuan"/"Cadangan") + `label` per key (Indonesia). Layar butuh bilingual + ikon.

- Konstanta `frontend/app/constants/authzCatalog.ts`:
  - `GROUP_ICON: Record<string, string>` — peta nama grup → ikon (mis. `Sistem`→`i-lucide-shield`, `Aset`→`i-lucide-box`, dst.); fallback ikon default untuk grup tak dikenal.
  - `iconForGroup(group: string): string`.
- **Label tampilan via i18n**: kunci `settings.rbac.catalog.group.<slug>` untuk nama grup dan `settings.rbac.catalog.perm.<key>` untuk label permission, di `i18n/locales/{id,en}.json`. Komponen me-resolve `te()/t()`; **bila kunci i18n tak ada, fallback ke `label` dari katalog API**. Ini membuat key baru dari backend tetap tampil (label Indonesia) tanpa crash, sambil menjaga id/en untuk key yang dikenal.

Komponen `rbac/RbacPermissionCard.vue` menerima `ModuleView` + me-resolve label/ikon lewat helper presentasi (bukan dari mock).

## 4. Buat peran ("Add Role")

Form mockup hanya punya **nama / deskripsi / copy-from** — dipertahankan (tanpa field `code`). `code` di-derive otomatis dari `name` via `slugify` di `createRole`. Konflik backend (`409`, code/name sudah dipakai) ditampilkan sebagai **error validasi inline** di modal ("Nama peran sudah dipakai"), bukan toast generik — handler menangkap status 409 dari `useApiClient`. Copy-from diimplementasi klien (create → get source perms → put). Peran baru selalu `is_system=false`.

## 5. Permissions: muat, edit, simpan

- Saat sebuah peran dipilih di `RbacRoleList`, panggil `getRolePermissions(id)` (cache per-id di state; hindari refetch saat pindah-pindah). Tampilkan **loading** saat fetch.
- Matriks toggle mengubah set lokal (dirty state seperti sekarang). "Simpan" → `updateRolePermissions(id, perms)` → toast sukses; refresh cache id itu.
- Peran `is_system`: permission **tetap dapat diedit & disimpan** (inti configurable RBAC). Hanya hapus/ubah-code yang dikunci (layar tak punya hapus peran di lingkup ini); badge/lock-note tetap.

## 6. State, i18n, error

- Tambah state **loading** (saat `getCatalog`/`listRoles`/`getRolePermissions`) dan **error** (gagal muat → tampilkan pesan + tombol retry; mutasi gagal sudah ditoast `useApiClient`). Empty (tak ada peran) sudah tertangani.
- Pisahkan konstanta UI yang sebelumnya bocor dari mock bila tersentuh (tidak relevan untuk RBAC; `SCOPE_LEVELS`/`FIELD_ROLE_KEYS` milik sub-proyek lain).
- String baru di `i18n/locales/{id,en}.json` di bawah `settings.rbac.catalog.*` + pesan error/retry/konflik.

## 7. Pengujian (proaktif & luas)

- **Unit** (`frontend/test/unit/`): ganti `rbac-mock.spec.ts` → uji `useRbac` dengan `$fetch`/`useApiClient` di-mock (`registerEndpoint` atau `vi.mock`): `getCatalog` memetakan grup+ikon; `listRoles` mengembalikan data; `getRolePermissions`; `createRole` (derive code benar; copy-from memanggil get+put; 409 melempar error yang bisa ditangani); `slugify` (spasi, karakter aksen/non-alnum, ganda). Assert payload PUT `{permissions:[…]}`.
- **Nuxt component** (`frontend/test/nuxt/settings-rbac.spec.ts`): `mountSuspended` dengan endpoint `/authz/*` di-mock; assert: loading→render daftar peran & matriks; pilih peran memuat perms; toggle+save mengirim PUT benar; add-role modal (derive code, sukses, 409→error inline); system role tampil lock tapi toggle perms tetap aktif; state error muat → retry.
- **E2E** (`frontend/e2e/settings.spec.ts`): jalankan terhadap backend nyata + admin seeded; assert `/settings/rbac` menampilkan peran nyata (mis. "Superadmin", "Manager") dan label permission, toggle+save persist (reload → tetap).
- Cakupan edge: gagal muat katalog, peran tanpa permission, key katalog tak dikenal i18n (fallback label), nama duplikat saat create.

## 8. "Selesai"

- `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau (gate CI). E2E `pnpm test:e2e` butuh stack backend + admin seeded.
- `mock/rbac.ts` dipertahankan hanya bila masih dipakai test; bila tak lagi diacu, hapus untuk menghindari sumber-kebenaran ganda (cek referensi dulu).
- Bandingkan layar terhadap `docs/design/<RBAC mockup>.dc.html` 1:1 (layout/state) setelah wiring; perbaiki deviasi.
- Update `docs/PROGRESS.md`: catat RBAC wired ke API real; refresh blok "Next session" → Data Scope berikutnya.
