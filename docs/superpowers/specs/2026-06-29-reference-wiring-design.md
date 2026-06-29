# Wire Master Data Referensi screen to the generic reference engine — Design

| | |
|---|---|
| **Tanggal** | 2026-06-29 |
| **Area** | Frontend (`pages/master/reference.vue`, `composables/api/useReference.ts`, `composables/api/referenceResources.ts`) + Backend (`internal/masterdata/reference` engine: `tier` enum support) |
| **Backend** | Generic reference engine: `GET/POST/PUT/DELETE /api/v1/<path>` for 11 resources (gate `masterdata.global.manage`, global/no-scope) |
| **Status** | Disetujui — siap implementasi |

> Sub-proyek **kedua** dari batch "wiring menu master" (Peta Lokasi selesai). Roadmap berikutnya: Kategori Aset → Pegawai → Kantor.

## 1. Konteks & lingkup

Layar **Master Data Referensi** (`frontend/app/pages/master/reference.vue`) mengelola 11 tabel referensi datar lewat satu sidebar pemilih + tabel + FormModal. Saat ini **mock penuh** (`useReference` membaca `~/mock/reference`, descriptor di `composables/api/referenceResources.ts`). Backend **generic reference engine** sudah lengkap untuk ke-11 resource. Sub-proyek ini mewire frontend ke engine itu, menutup gap descriptor (FK picker, kolom hilang, `is_active`), dan—sesuai keputusan—menambah field **`tier`** ke resource `office-types` (memerlukan dukungan kolom enum di engine) sehingga peta menjadi bermakna.

### Temuan yang menentukan desain
- **Kontrak engine** (per resource, `<path>` ∈ office-types, departments, positions, units, maintenance-categories, problem-categories, brands, vendors, provinces, cities, models):
  - `GET /api/v1/<path>?search&limit&offset` → `{ data:[…], total, limit, offset }` (limit default 20, clamp 1–100). `authMW` saja.
  - `GET /api/v1/<path>/:id` → objek flat. `POST` → 201 objek flat. `PUT /:id` → objek flat. `DELETE /:id` → 204. Write di-gate `authMW + masterdata.global.manage`. **Tanpa data-scope** (global).
  - Body create/update = JSON dengan **nama kolom backend** sebagai key (English): `name`, `code`, `symbol`, `email`, `phone`, `contact_name`, `address`, `province_id`, `brand_id`, `is_active`, (+`tier` baru). UUID dikembalikan sebagai text.
  - Single-row response = objek flat **tanpa** envelope; list = ber-envelope `{data,…}`.
- **Backend SUDAH mendefinisikan** `cities.province_id` (UUID, required), `models.brand_id` (UUID, required), `vendors.contact_name` + `vendors.address` di `resources.go`. Jadi gap FK & vendors **murni di frontend descriptor** — tak ada perubahan backend untuk itu.
- **`is_active`**: backend memakai key `is_active`; frontend (type/mock/form) memakai `active`. 9 resource punya kolom ini; **provinces & cities tidak** (tabel & resource def tak punya `is_active`).
- **`tier`**: tabel `masterdata.office_types` **sudah** punya kolom `tier shared.approver_level` (migrasi 000016, nullable) — **tanpa migrasi baru**. Engine belum mendukung kolom enum (`colType` hanya text/bool/uuid). Menambah `tier` ke resource def butuh tipe kolom `typeEnum` di engine.
- **Tanpa seed** provinces/brands → picker FK `cities.province_id` & `models.brand_id` kosong sampai province/brand dibuat lewat UI (kedua resource itu datar & sudah bisa dibuat).
- Frontend memakai key **English** untuk field referensi (`name`/`code`/…) — konsisten dgn backend; bukan gap. Satu-satunya rename: `active`→`is_active`.

## 2. Backend — dukungan enum di engine + tier office-types

### 2.1 Engine (`internal/masterdata/reference/engine.go`)
Tambah tipe kolom enum (perubahan minimal, terkontrol):
- `colType`: tambah `typeEnum`.
- `column`: tambah `Enum []string` (nilai sah) + `EnumType string` (nama tipe PG untuk cast, mis. `shared.approver_level`).
- `placeholder`: ubah signature `placeholder(n int, c column) string` (2 call site di `write`). `typeUUID` → `$n::uuid`; `typeEnum` → `$n::<c.EnumType>`; selain itu `$n`.
- `selectExpr`: untuk `typeEnum`, keluarkan `c.Name + "::text AS " + c.Name` (seperti uuid) agar pgx men-scan sebagai string.
- `coerce`: case `typeEnum` — seperti text (string, **opsional/nullable**: tak-hadir/nil/`""` → `nil`), tetapi bila non-kosong **validasi** `value ∈ c.Enum`, jika tidak → `fmt.Errorf("%s must be one of …", c.Name)` (→ 400). UUID/enum cast hanya diterapkan saat nilai non-nil (placeholder cast atas `NULL` aman: `NULL::shared.approver_level`).

### 2.2 office-types resource (`internal/masterdata/reference/resources.go`)
Tambah kolom ke resource `office-types` (urutan: name, tier, is_active):
```go
{Path: "office-types", Table: "office_types", OrderBy: "name", Columns: []column{
  {Name: "name", Type: typeText, Required: true, Search: true},
  {Name: "tier", Type: typeEnum, EnumType: "shared.approver_level", Enum: []string{"pusat", "wilayah", "office"}},
  {Name: "is_active", Type: typeBool, Default: true},
}},
```
(Hanya 3 nilai tier yang ditawarkan; enum PG juga punya `office_subtree` tetapi tak relevan untuk tipe kantor.)

### 2.3 openapi + tes
- `backend/api/openapi.yaml`: tambah `tier` (string, nullable, enum `[pusat,wilayah,office]`) ke schema request/response office-types (atau schema referensi generik bila ada). Spectral hijau.
- Tes engine (integrasi, gaya `reference` existing bila ada; jika tidak, ikuti pola office integration test): create/update/list office-types dengan `tier` valid (round-trip), `tier` invalid → error (400), `tier` kosong/absen → NULL. `go build/vet/test ./...` + `go test -tags=integration ./...` + Spectral.

## 3. Frontend — composable, descriptor, page

### 3.1 `useReference.ts` — tulis ulang ke HTTP
Hapus impor `~/mock/*`. Via `useApiClient().request`:
- `list(key, query): Promise<Paginated<ReferenceRow>>` — `GET /<path>?search&limit&offset` (omit kosong). `<path>` = key (key descriptor == path backend).
- `create(key, input): Promise<ReferenceRow>` — `POST /<path>` (body English, termasuk `is_active`/FK/tier).
- `update(key, id, input): Promise<ReferenceRow>` — `PUT /<path>/:id`.
- `remove(key, id): Promise<void>` — `DELETE /<path>/:id`.
(Bentuk return kompatibel dgn `Paginated<ReferenceRow>` & `ReferenceRow` flat.)

### 3.2 Descriptor `referenceResources.ts`
Perkaya `ReferenceField`:
```ts
type ReferenceFieldType = 'text' | 'fk' | 'select'
interface ReferenceField {
  key: string
  labelKey: string
  type?: ReferenceFieldType         // default 'text'
  fkResource?: ReferenceKey         // type:'fk' — resource sumber opsi & resolusi nama
  options?: { value: string, labelKey: string }[]  // type:'select' — opsi statis
  required?: boolean
}
interface ReferenceDescriptor {
  key: ReferenceKey
  labelKey: string
  hasActive: boolean                // false utk provinces & cities
  fields: ReferenceField[]
}
```
Perubahan per resource:
- `office-types`: `hasActive:true`; fields `[name, tier]` di mana `tier = { key:'tier', labelKey:'…tier', type:'select', options:[{value:'pusat',labelKey:'map.tier.pusat'},{value:'wilayah',labelKey:'map.tier.wilayah'},{value:'office',labelKey:'map.tier.office'}] }` (pakai ulang label tier dari peta).
- `cities`: `hasActive:false`; fields `[{ key:'province_id', labelKey:'…province', type:'fk', fkResource:'provinces', required:true }, name, code]`.
- `models`: `hasActive:true`; fields `[{ key:'brand_id', labelKey:'…brand', type:'fk', fkResource:'brands', required:true }, name]`.
- `vendors`: tambah `contact_name` + `address` (`[name, contact_name, phone, email, address]`).
- `provinces`: `hasActive:false`.
- Sisanya (departments, positions, units, maintenance-categories, problem-categories, brands): `hasActive:true`, fields tetap (units tetap `[name, symbol]`).

### 3.3 Page `reference.vue`
- **`active`→`is_active`** di mana pun (form state, kolom tabel, slot `#is_active-cell`, `toggleActive` → `api.update(key,id,{is_active:!prev})`). Toggle & kolom is_active **hanya** dirender bila `descriptor.hasActive`.
- **FormModal** — render field per `type`:
  - `text` → `UInput` (seperti sekarang).
  - `fk` → `USelect`/`USelectMenu`, opsi dari `fkOptions(fkResource)` (lihat di bawah); value = id, label = nama. Wajib bila `required` (validasi inline; bila opsi kosong → pesan "buat <resource> dulu").
  - `select` → `USelect` dgn `options` statis (label via i18n).
  - `is_active` tetap `USwitch` terpisah, hanya bila `hasActive`.
- **Resolusi nama FK di tabel**: bila descriptor punya field `type:'fk'`, page memuat resource FK sekali per perubahan resource (`api.list(fkResource,{limit:100})`) → `Map<id,name>` (`fkOptions`/`fkNameMap`). Dipakai untuk **picker form** dan **sel tabel** (kolom `province_id`→nama provinsi, `brand_id`→nama brand; kosong→em-dash). office-types: sel `tier` tampil label tier i18n.
- **Kolom dinamis**: kolom tabel diturunkan dari `descriptor.fields` (FK & select tampil sebagai nama/label ter-resolve) + `is_active` (bila `hasActive`).
- State loading/empty/error per resource tetap; tambah penanganan error muat opsi FK (toast/biarkan picker kosong).

### 3.4 `~/types`
- `ReferenceRow`: `active`→`is_active?: boolean`; tambah index fleksibel untuk FK/tier (`[k:string]: unknown` sudah/atau tambah `province_id?`, `brand_id?`, `tier?` opsional). Pastikan tak memecah konsumen lain (`ReferenceRow` hanya dipakai jalur reference).

### 3.5 i18n
Tambah di `masterdata.reference.fields` (id/en): `province`, `brand`, `contact_name`, `address`, `tier`. Label opsi tier pakai ulang `map.tier.{pusat,wilayah,office}` (Pusat/Wilayah/Cabang). Semua string via `$t`.

### 3.6 Hapus mock
Hapus `frontend/app/mock/reference.ts` bila tak ada importer tersisa (cek; barrel `mock/index.ts` meng-export `reference` → sesuaikan).

## 4. Pengujian (proaktif & luas)

- **Backend** (`reference` engine, integrasi): office-types `tier` valid round-trip (create+update+list), invalid→400, kosong→NULL; pastikan resource lain tak terdampak (regresi: create vendor dgn contact_name/address, create city dgn province_id, create model dgn brand_id — sudah didukung backend).
- **Unit** (`test/unit/use-reference.spec.ts`, mock `useApiClient`; hapus tes mock lama bila ada): tiap verb membangun path `/<key>` + body benar (key `is_active`, `province_id`/`brand_id` UUID, `tier`); `list` query (search/limit/offset, omit kosong) → `{data,total}`.
- **Component** (`test/nuxt/master-reference.spec.ts`, stub engine endpoints): ganti resource via sidebar memuat data benar; **picker FK** `cities` menampilkan opsi provinsi & submit `POST /cities` `{province_id,…}`; **select** office-types `tier` submit `{tier:'pusat'}`; **resolusi nama** — sel cities tampil nama provinsi (bukan UUID); toggle `is_active` tampil utk brands tapi **tidak** utk provinces/cities; create/edit/delete/search; 11 resource dapat dipilih.
- **E2E** (`e2e/master-reference.spec.ts`): backend nyata + admin — buat sebuah **province**, lalu buka `cities`, buat city memakai **picker provinsi** (trigger+`role=option`, **bukan** `selectOption`), assert city muncul dgn nama provinsi ter-resolve. Locator robust (tanpa `isVisible()` snapshot/`.first()` div luas/`getByText` non-exact yang ambigu).

## 5. "Selesai"
- Backend: `go build/vet/test ./...` + `go test -tags=integration ./...` + Spectral hijau.
- Frontend: `pnpm lint/typecheck/test/build` hijau (E2E di CI).
- Hapus `mock/reference.ts` (cek importer + barrel).
- Bandingkan `reference.vue` ke `docs/design/Master Data Referensi.dc.html`: sidebar 11 resource + tabel + FormModal cocok; FK picker (provinsi/brand) & field tier/vendor adalah penambahan yang didukung backend (bukan deviasi desain). Penyimpangan disetujui: toggle is_active disembunyikan utk provinces/cities (tabelnya memang tak punya kolom itu).
- `docs/PROGRESS.md`: Referensi ✅ wired ke generic engine (11 resource; FK picker cities→provinsi, models→brand; vendors contact_name/address; office-types **tier**); catat bahwa peta kini bermakna setelah tier dapat di-set; refresh "▶ Next session" → **Kategori Aset**.

## 6. Risiko & catatan
- **Enum di engine**: `typeEnum` adalah perubahan terkontrol; pastikan `NULL::<enum>` aman (placeholder cast atas nilai nil) dan invalid value → 400 (bukan 500). Tes mencakup ketiga jalur.
- **Picker FK kosong tanpa seed**: cities/models tak bisa dibuat sebelum ada province/brand; UI menampilkan opsi kosong + hint. Diterima (bukan blocker).
- **`ReferenceRow` lintas-key**: pastikan rename `active`→`is_active` + field dinamis (FK/tier) tak memecah `ResourceTable`/`FormModal` generik. Cek tipe.

## 7. Roadmap batch (konteks)
1. Peta Lokasi — ✅ selesai (#39).
2. **Referensi** — sub-proyek ini.
3. Kategori Aset — DTO 9-field bank-FAM + tree `parent_id`.
4. Pegawai — scope `employees` + FK picker (office/department/position).
5. Kantor + Lantai + Ruangan — tree, scope, floors/rooms inline, input lat/lng.
