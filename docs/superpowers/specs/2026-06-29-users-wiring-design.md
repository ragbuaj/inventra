# Wire User Management screen to `/api/v1/users` — Design

| | |
|---|---|
| **Tanggal** | 2026-06-29 |
| **Area** | Frontend (Nuxt 4) — `pages/settings/users.vue` + `composables/api/useUsers.ts` |
| **Backend** | `/api/v1/users` (CRUD, gated `user.manage`, field-permission filtered) |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & lingkup

Layar **Manajemen User** (`frontend/app/pages/settings/users.vue`) mengelola akun (`identity.users`) — list/create/update/delete + filter. Saat ini mock-first dengan DTO Indonesia & filter client-side. Sub-proyek ini mewire **layar User Management saja** ke backend nyata. Ini layar terakhir dari rangkaian wiring settings (RBAC/Data Scope/Field Permission/Audit sudah). Mengikuti pola: id identity, DTO English, eager lookups, gate penuh.

### Kendala backend (menentukan desain)
- **List** `GET /api/v1/users` hanya mendukung query `search/limit/offset` — **tak ada** filter `role_id/office_id/status` server-side. Envelope `{data,total,limit,offset}`. Respons di-**field-permission-filter** (entity `users`) → render apa adanya.
- **Create** `POST /users`: `{name(req), email(req), password?, role_id(req,uuid), office_id?(uuid), employee_id?(uuid)}`. **Update** `PUT /users/:id`: `{name(req), role_id(req), status(req: active|inactive|suspended), office_id?, employee_id?}` — **tanpa** email/password. **Delete** `DELETE /users/:id` → 204.
- **Tak ada** endpoint `setStatus`/`resetPassword`. Status diubah lewat `PUT`; reset-password **dihapus** dari UI (tak ada backend).
- Respons `userToMap`: `{id,name,email,role_id,office_id?,employee_id?,status,avatar_url?,google_linked,created_at,updated_at}` — id/FK adalah **UUID**, bukan nama.
- **Lookup terbuka**: `GET /offices` & `GET /employees` baca **untuk semua user terautentikasi** (hanya `authMW`); roles via `GET /authz/roles`. Picker form & resolusi nama di tabel memakai ketiganya. `employees` membawa `office_id` → picker pegawai difilter per kantor.

Keputusan disetujui (Q1 opsi server-side): **drop dropdown filter role/office/status** (tak didukung backend) — penyimpangan mockup setara deviasi sebelumnya.

## 2. Composable `useUsers.ts` — tulis ulang

Hapus ketergantungan `~/mock/users` + helpers. Tipe (DTO English, UUID):
```ts
type UserStatus = 'active' | 'inactive' | 'suspended'
interface UserView {
  id: string; name: string; email: string; role_id: string
  office_id: string | null; employee_id: string | null
  status: UserStatus; avatar_url: string | null; google_linked: boolean
  created_at: string | null; updated_at: string | null
}
interface CreateUserInput { name: string; email: string; password?: string; role_id: string; office_id?: string; employee_id?: string }
interface UpdateUserInput { name: string; role_id: string; status: UserStatus; office_id?: string; employee_id?: string }
interface Option { id: string; name: string }
interface EmployeeOption extends Option { office_id: string }
interface Lookups { roles: Option[]; offices: Option[]; employees: EmployeeOption[] }
```
Fungsi (via `useApiClient().request`):
- `async list({search,limit,offset}): Promise<{ rows: UserView[]; total: number }>` — `GET /users?…` (omit empty search).
- `async create(input: CreateUserInput): Promise<UserView>` — `POST /users` (only non-empty `password`/`office_id`/`employee_id` sent).
- `async update(id, input: UpdateUserInput): Promise<UserView>` — `PUT /users/:id`.
- `async remove(id): Promise<void>` — `DELETE /users/:id`.
- `async lookups(): Promise<Lookups>` — parallel: `GET /authz/roles` → roles `{id,name}`; `GET /offices` → offices `{id,name}`; `GET /employees` → employees `{id,name,office_id}`. (Each endpoint returns `{data}` lists; map to the option shape.)

Drop the mock `setStatus`/`resetPassword`.

## 3. Page `users.vue`

- **Gate** tetap `definePageMeta({ middleware:'can', permission:'user.manage' })`.
- **Load**: on mount, fetch `lookups()` + `list()` (parallel). Hold `lookups` for name resolution + form pickers. Build id→name maps (`roleName`, `officeName`, `employeeName`).
- **Filter**: **search** (server-side, debounce optional) + paginasi (`limit/offset`); **drop** dropdown role/office/status. Reset = clear search.
- **Tabel** (ResourceTable): nama+avatar, peran (`roleName(role_id)`), kantor (`officeName(office_id)`), pegawai (`employeeName(employee_id)`), login badge (`google_linked` → Google/Email), status badge. Em-dash untuk FK kosong.
- **Form (FormSlideover)**:
  - **Create**: name, email, password (opsional), **role_id** (picker, wajib), **office_id** (picker, opsional), **employee_id** (picker, opsional). Tanpa field login/status.
  - **Edit**: name, **role_id** (picker), **status** (select active/inactive/suspended), office_id, employee_id. Email read-only; tanpa password.
  - **Picker pegawai difilter per kantor**: opsi employee = `lookups.employees.filter(e => e.office_id === form.office_id)`. Saat `office_id` berubah, bila `employee_id` terpilih tak lagi cocok kantor baru → **clear** `employee_id`. Bila belum pilih kantor → picker pegawai kosong/disabled.
  - Picker = `USelectMenu`/`USelect` dari lookups (label = name).
  - Aksi baris: **edit**, **ubah status** (via `update` dengan field baris saat ini + status baru), **hapus** (DELETE + konfirmasi). **Drop** reset-password.
- **Validasi/error**: name/email/role_id wajib; email `409` (duplikat) → error inline di form; mutasi sukses → toast + refresh list. State loading/error+retry.

## 4. Tipe & i18n

- `User` interface Indonesia di `~/types/index.ts`: cek pemakaian lain; bila hanya dipakai users, pindahkan/ganti dengan `UserView` (dari composable). Hapus konstanta mock `ROLES`/`KANTOR_OPTIONS`/`PEGAWAI_OPTIONS` dari import page.
- i18n `settings.users.*` sebagian besar sudah ada; tambah `loadError`/`retry` + pesan konflik email bila kurang; sesuaikan/hapus label filter yang di-drop. Label via i18n; tak ada string hardcoded.

## 5. Pengujian (proaktif & luas)

- **Unit** (`test/unit/use-users.spec.ts`, mock `~/composables/useApiClient`; hapus `users-mock.spec.ts`):
  - `list` membangun query (omit empty search), kembalikan `{rows,total}`.
  - `create`/`update`/`remove` mengirim path+method+body benar (English keys, UUID; password/office_id/employee_id kosong dihilangkan dari create).
  - `lookups()` memetakan ketiga sumber: roles/offices `{id,name}`, employees `{id,name,office_id}`.
- **Component** (`test/nuxt/settings-users.spec.ts`, stub `/users`+`/authz/roles`+`/offices`+`/employees`):
  - render baris dengan **nama ter-resolve** (role/office/employee dari lookups, bukan UUID).
  - create: buka form, isi, picker role/office; **picker pegawai hanya menampilkan pegawai kantor terpilih**; ubah kantor meng-clear pegawai yang tak cocok; submit → `POST /users` body benar (assert captured body); `409`→inline error.
  - edit: PUT body benar termasuk status; delete → DELETE.
  - search → query `search=`; load-error (500) → error+retry.
- **E2E** (`e2e/settings.spec.ts`): backend nyata + admin seeded — daftar user nyata (mis. admin); buka form create; search. Locator robust (USelect via trigger+option, **bukan** `selectOption`; tanpa class Tailwind / `isVisible()` snapshot).

## 6. "Selesai"

- `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau (E2E di CI).
- Hapus `mock/users.ts` bila tak lagi diacu (cek importer; konstanta ROLES/dll hilang dari page).
- Bandingkan ke `docs/design/Manajemen User.dc.html`: **drop** dropdown filter role/kantor/status + aksi reset-password + field login = penyimpangan disetujui (backend tak mendukung); selebihnya layout/tabel/form/slideover cocok.
- `docs/PROGRESS.md`: User Management wired ke `/api/v1/users` (CRUD; server-side search+paginasi; picker role/office/employee dari lookup nyata; pegawai per-kantor). TODO: filter role/office/status server-side + reset-password menunggu dukungan backend. Refresh "▶ Next session": **rangkaian wiring layar settings tuntas** → lanjut backend bank-FAM (asset transfer/mutasi dll.) atau perluasan enforcement field-permission.
