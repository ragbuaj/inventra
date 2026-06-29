# Wire Master Data Pegawai (Employees) screen to `/api/v1/employees` — Design

| | |
|---|---|
| **Tanggal** | 2026-06-29 |
| **Area** | Backend (`internal/masterdata/employee`: tambah `phone`) + Frontend (`pages/master/employees.vue`, `composables/api/useEmployees.ts`, tipe `Employee`) |
| **Backend** | `/api/v1/employees` CRUD (gate `masterdata.office.manage`, **data-scope `employees`**) |
| **Status** | Disetujui — siap implementasi |

> Sub-proyek **keempat** dari batch "wiring menu master" (Peta Lokasi #39, Referensi #40, Kategori #41 selesai). Tersisa: Kantor (#5).

## 1. Konteks & lingkup

Layar **Master Data Pegawai** (`frontend/app/pages/master/employees.vue`) mengelola pegawai (`masterdata.employees`) — tabel + slideover form + filter bar (4 dropdown: kantor/departemen/jabatan/status). Saat ini **mock penuh** (`useEmployees` membaca `~/mock/employees`; tipe `Employee` ber-key Indonesia). Ini sub-proyek **terkompleks** dari batch: rename Indonesia→English, perubahan struktur (department/position dari nama-string → UUID FK), penambahan field backend (`phone`), resolusi nama 3 FK, dan **data-scope `employees`** (per-row office visibility, sudah server-enforced).

### Temuan yang menentukan desain
- **Kontrak CRUD** (`/api/v1/employees`): `GET` list (`{data,total,limit,offset}`, search/limit(1–100)/offset), `GET /:id` (flat), `POST` (201 flat), `PUT /:id` (200 flat), `DELETE /:id` (204). Reads `authMW`; writes `authMW + masterdata.office.manage`.
- **Data-scope `employees` dipaksakan per-verb** (server-side): list/get di-filter `(all_scope OR office_id = ANY(office_ids))`; create/update menolak office di luar scope (403 `ErrOfficeOutOfScope`); get/delete out-of-scope → 404. Frontend **tak perlu** logika scope — render apa yang diterima; office picker hanya berisi office dalam scope (karena `/offices` juga scoped). Scope module string = `"employees"`.
- **Field DTO backend (English)**: `code`(req — NIP), `name`(req), `email?`(format email), `avatar_key?`, `department_id?`(UUID FK→departments), `position_id?`(UUID FK→positions), `office_id`(req,UUID FK→offices), `status?`(enum `active|inactive|suspended`, default active). Response juga `id, created_at, updated_at`. **List mengembalikan UUID mentah, tanpa JOIN nama.**
- **`phone` belum ada** di skema/DTO backend → **ditambahkan** (keputusan disetujui). Mockup menampilkan Email/Telepon.
- **Gap frontend**: tipe `Employee` Indonesia (`nip/nama/telepon/jabatan/departemen/office_id/status`); `jabatan`/`departemen` disimpan sebagai **nama-string** (form `value: d.name`), backend butuh **UUID**. `telepon` tak ada di backend (ditambah). `nip→code`, `nama→name`.
- **`useReference` sudah wired** (dept/position via `/departments`,`/positions`). **`useOffices` masih mock** (akan di-wire di sub-proyek Kantor) → office options/map via **inline-fetch `/offices`** (`useApiClient`) di halaman, seperti users.vue #38.

### Keputusan yang disetujui
- **Tambah `phone` ke backend** (migrasi + DTO + query + openapi + tes). Mockup Email/Telepon dipertahankan.

## 2. Backend — tambah kolom `phone`

### 2.1 Migrasi `NNNNNN_employee_phone`
```sql
-- up
ALTER TABLE masterdata.employees ADD COLUMN phone text;
-- down
ALTER TABLE masterdata.employees DROP COLUMN phone;
```
(Nomor migrasi = tertinggi+1; nullable; tanpa trigger/index baru.) `sqlc generate` setelahnya.

### 2.2 DTO + query
- `employee/dto.go`: tambah `Phone *string \`json:"phone"\`` ke `Request` + `Response`; `toInput`/`toResponse` passthrough.
- service `CreateInput`: tambah `Phone *string`; `Create`/`Update` teruskan ke params sqlc.
- `db/queries/employees.sql`: `CreateEmployee`/`UpdateEmployee` sertakan kolom `phone`; `SELECT *` otomatis mengembalikannya. **Scope predicate per-verb tak diubah.**
- `sqlc generate`.

### 2.3 openapi + tes
- `backend/api/openapi.yaml`: tambah `phone` (string, nullable) ke schema employee request/response. Spectral hijau.
- Tes Go (gaya employee test existing — sudah ada scope test): `phone` round-trip (create+update mengembalikan phone; absen→null). Tes scope existing tetap hijau. `go build/vet/test ./...` + `go test -tags=integration ./...` + Spectral.

## 3. Frontend — tipe, composable, page

### 3.1 Tipe `Employee` + `EmployeeInput`
Tulis ulang English snake_case (cek dulu pemakaian `Employee` di layar lain; bila ada konsumen non-employees, definisikan `EmployeeView` lokal alih-alih ubah `~/types`, seperti `UserView` #38):
```ts
type EmployeeStatus = 'active' | 'inactive' | 'suspended'
interface Employee {
  id: string; code: string; name: string
  email: string | null; phone: string | null
  department_id: string | null; position_id: string | null
  office_id: string; status: EmployeeStatus
  avatar_key?: string | null; created_at: string | null; updated_at: string | null
}
interface EmployeeInput {
  code: string; name: string; email?: string; phone?: string
  department_id?: string; position_id?: string; office_id: string; status: EmployeeStatus
}
```

### 3.2 `useEmployees.ts` — tulis ulang ke HTTP
Hapus impor mock. Via `useApiClient().request`:
- `list(query): Promise<Paginated<Employee>>` — `GET /employees?search&limit&offset` (omit kosong).
- `get(id)`, `create(input)`→`POST /employees`, `update(id,input)`→`PUT /employees/:id`, `remove(id)`→`DELETE`. Body English; optional kosong dihilangkan.

### 3.3 Page `employees.vue` — rewrite
- **Pemuatan**: `list({limit:100})` (atau search server-side bila perlu; saat ini filter/paginasi client-side — pertahankan agar perubahan minimal) + muat opsi/peta FK: `useReference('departments')`, `useReference('positions')` (sudah wired), dan **inline `GET /offices?limit=100`** via `useApiClient`. Bangun 3 map id→name (department/position/office) + 3 option list (`value: id`, `label: name`).
- **Kolom**: NIP(`code`)/Nama(`name`+avatar)/Departemen(`deptName(department_id)`)/Jabatan(`posName(position_id)`)/Kantor(`officeName(office_id)`)/Email+Telepon(`email`/`phone`)/Status(badge). FK kosong → em-dash.
- **Filter bar** (client-side): kantor/departemen/jabatan = **value UUID** (bandingkan `office_id`/`department_id`/`position_id`); status active/inactive. Reset.
- **Form (FormSlideover)**: `code`(NIP, req), `name`(req), email, **phone** (kini ter-wire), department/position/office = `USelect` value=UUID, status toggle. Office picker tampilkan `scopeNote` (sudah ada). Validasi: code/name/office_id wajib.
- **status**: toggle active↔inactive (sesuai mockup) + render `suspended` (badge/label) bila backend mengembalikannya. Set `suspended` via UI tak diekspos.
- **State**: tambah load-error+retry; loading/empty tetap. CRUD via composable; sukses → refresh.
- Gate tetap `masterdata.office.manage`.

### 3.4 i18n
Tambah `masterdata.employees.status.suspended` (id: "Ditangguhkan", en: "Suspended") + `loadError`/`retry` bila perlu. Semua string via `$t`.

### 3.5 Hapus mock
Hapus `mock/employees.ts` bila tak ada importer tersisa. **Cek `useGlobalSearch`** (mungkin impor `employeeStore`) — bila masih dipakai, jangan hapus / repoint (seperti `mock/users` #38). Cek barrel `mock/index.ts`.

## 4. Pengujian (proaktif & luas)

- **Backend**: `phone` round-trip (create+update→phone; absen→null); regresi scope (list/get/create/update/delete tetap menghormati `employees` scope — tes existing). Build/vet/test + integration + Spectral.
- **Unit** (`test/unit/use-employees.spec.ts`, mock `useApiClient`; hapus tes mock lama): tiap verb path/body benar (English keys, UUID dept/position/office, phone; optional kosong dihilangkan); `list` query.
- **Component** (`test/nuxt/master-employees.spec.ts` — **REWRITE**, sudah ada (mock-based; sudah di-stub useReference di #40); stub `/employees`+`/departments`+`/positions`+`/offices`): render baris dgn **nama FK ter-resolve** (bukan UUID); filter by UUID menyaring; create→`POST` body UUID benar; edit/delete; status suspended ter-render; load-error+retry. **Verifikasi `pnpm test` exit 0** (pelajaran #40 — konsumen `useEmployees`/`useOffices`-inline ter-stub).
- **E2E** (`e2e/employees.spec.ts` — cek apakah sudah ada → rewrite): backend nyata + admin (global scope) — buat pegawai memilih office/department/position via picker (trigger+`role=option`, **bukan** `selectOption`), assert baris tampil. **Pelajaran e2e**: nama **dan code (NIP)** unik per run, assert-after-search, tunggu slideover tertutup; locator `data-testid` untuk picker.

## 5. "Selesai"
- Backend: `go build/vet/test ./...` + `go test -tags=integration ./...` + Spectral hijau.
- Frontend: `pnpm lint/typecheck/test/build` hijau; **`pnpm test` exit 0**.
- Hapus `mock/employees.ts` (cek importer + barrel; bila `useGlobalSearch` masih pakai → pertahankan + catat TODO).
- Bandingkan `employees.vue` ke `docs/design/Master Data Pegawai.dc.html`: tabel/filter-bar/slideover cocok 1:1; Email/Telepon dipertahankan (phone ditambahkan). Tak ada deviasi yang diharapkan.
- `docs/PROGRESS.md`: Pegawai ✅ wired ke `/api/v1/employees` (data-scope server-enforced; FK picker office/department/position; phone ditambah). Refresh "▶ Next session" → **Kantor + Lantai + Ruangan** (sub-proyek terakhir batch master).

## 6. Risiko & catatan
- **Tipe `Employee` lintas-layar**: grep pemakaian; bila dipakai layar lain (assignment/asset) dengan key Indonesia, jangan ubah `~/types` — pakai `EmployeeView` lokal (pola #38). Plan memverifikasi.
- **`useOffices` mock**: inline-fetch `/offices` di halaman (read-only, scoped); biarkan `useOffices` untuk sub-proyek Kantor (#5) — hindari konflik.
- **#40 ECONNREFUSED**: `useEmployees` mock→HTTP membuat konsumen lain (mis. `useGlobalSearch` bila mount di test) memanggil jaringan nyata; stub di test mereka; verifikasi exit 0.
- **department/position picker kosong**: bila tak ada departments/positions, picker kosong (opsional, tak wajib). office_id wajib.
- **numeric/format**: tak relevan (tak ada field numeric di employee).

## 7. Roadmap batch (konteks)
1. Peta Lokasi — ✅ #39.
2. Referensi — ✅ #40.
3. Kategori Aset — ✅ #41.
4. **Pegawai** — sub-proyek ini.
5. Kantor + Lantai + Ruangan — tree, scope `offices`, floors/rooms inline, input lat/lng (sudah ada di DTO sejak #39); **akan men-wire `useOffices`** (yang di-inline-fetch di sini).
