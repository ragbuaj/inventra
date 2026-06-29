# Wire Peta Lokasi (Office Map) screen to a real geo endpoint — Design

| | |
|---|---|
| **Tanggal** | 2026-06-29 |
| **Area** | Backend (`internal/masterdata/office`, migrasi `000018`) + Frontend (`pages/master/map.vue`, `composables/api/useOfficeMap.ts`) |
| **Backend baru** | `GET /api/v1/offices/map` (authMW + data-scope `offices`); kolom `latitude`/`longitude` di `masterdata.offices` |
| **Status** | Disetujui — siap implementasi |

> Sub-proyek **pertama** dari batch "wiring menu master" (lihat §7 roadmap). Satu-satunya layar master yang belum punya backend, sehingga sub-proyek ini menyentuh backend **dan** frontend.

## 1. Konteks & lingkup

Layar **Peta Lokasi** (`frontend/app/pages/master/map.vue`) menampilkan kantor sebagai pin di peta Leaflet + panel daftar dengan filter (search, jenis, provinsi) dan kartu detail. Saat ini **mock penuh** (`useOfficeMap` membaca `mock/officeMap.ts`). Backend `masterdata.offices` **tidak** menyimpan koordinat dan tak ada endpoint geo. Sub-proyek ini: (a) menambah kolom `latitude`/`longitude` + endpoint baca khusus peta yang me-resolve nama tipe/provinsi/kota + jumlah aset, lalu (b) mewire layar ke endpoint itu. Mengikuti pola wiring settings/users: DTO English + identity UUID, `useApiClient`, i18n 2 locale, hapus mock, tes berlapis.

### Temuan backend yang menentukan desain
- `masterdata.offices` punya `name, code, office_type_id, province_id, city_id, parent_id, address, is_active` — **tanpa** koordinat. Semua query office saat ini `SELECT *` tanpa JOIN (mengembalikan UUID FK, bukan nama).
- `masterdata.office_types` punya `name` + kolom `tier shared.approver_level` (`pusat`/`wilayah`/`office`/`office_subtree`). **Tidak ada `code`.** `tier` praktis **belum terisi** (reference UI belum meng-expose `tier`; migrasi 000016 hanya meng-update by-name dan 0 baris terdampak di DB dev). Cabang & Outlet sama-sama `office` — tier tak membedakannya.
- `masterdata.provinces`/`cities` punya `name` (cities punya `province_id`).
- `asset.assets` **ada** (`000008`) dengan `office_id NOT NULL` + index `idx_assets_office_id`. Belum ada query count-per-office; jumlah aset **nyata** (akan 0 sampai modul aset terisi).
- Read office memakai `CallerOfficeScope(c, "offices")` → param sqlc `AllScope bool` + `OfficeIds []uuid`, klausa `($1::bool OR id = ANY($2::uuid[]))`. Endpoint peta **wajib** memakai pola scope yang sama.
- Migrasi terakhir = `000017`. Konvensi: soft-delete, trigger `set_updated_at` (sudah ada di offices — perubahan additive tak perlu trigger baru), partial-unique `WHERE deleted_at IS NULL`, seed idempoten.

### Keputusan yang disetujui
- **Kategori pin = berdasarkan `tier`** (Q1 opsi 1): Pusat / Wilayah / Cabang (tier `office`). Label kartu = `office_type_name` asli. Legenda **3 kategori**. `tier` NULL → fallback kategori **Cabang** (`office`). Membuat `tier` dapat diedit ditunda ke sub-proyek Referensi (TODO).
- **Sumber koordinat = tambah lat/lng ke API office** (Q2 opsi 1): `latitude`/`longitude` masuk DTO create/update office (settable via API nyata) + Response. **Tanpa** seed data contoh di produksi. Peta menampilkan empty-state sampai ada kantor berkoordinat (demo/e2e mengisi via API office nyata).

## 2. Backend — migrasi & DTO office

### 2.1 Migrasi `000018_office_coordinates`
```sql
-- up
ALTER TABLE masterdata.offices
  ADD COLUMN latitude  numeric(10,7),
  ADD COLUMN longitude numeric(10,7);
-- down
ALTER TABLE masterdata.offices
  DROP COLUMN latitude,
  DROP COLUMN longitude;
```
Nullable; tanpa trigger/index baru. `numeric` → override sqlc ke Go `string` (konvensi money/numeric) bila belum global; bila override hanya per-kolom money, gunakan tipe sqlc default untuk `numeric` (`pgtype.Numeric`) dan serialisasi sebagai string di DTO — **implementasi plan menentukan**, yang penting **DTO JSON mengembalikan/menerima `number`** (lihat §2.2). Setelah migrasi: `sqlc generate`.

### 2.2 DTO office (create/update/response)
- `office/dto.go` create + update request: tambah field opsional `latitude *float64` + `longitude *float64` dengan validasi `binding:"omitempty,min=-90,max=90"` (lat) dan `min=-180,max=180` (lng). (Gunakan `*float64` agar "tidak dikirim" ≠ "0".)
- `office/dto.go` Response: tambah `latitude *float64`, `longitude *float64` (null bila kosong) ke `toResponse()`.
- `CreateOffice`/`UpdateOffice` queries (`db/queries/offices.sql`) + service: sertakan kedua kolom. Service mem-passthrough nilai (NULL bila pointer nil).
- **Scope tetap dipaksakan pada create/update** seperti sekarang (tak berubah) — hanya menambah 2 kolom.

### 2.3 Query & endpoint peta
- Query baru `ListOfficesMap` (`db/queries/offices.sql`):
  ```sql
  -- name: ListOfficesMap :many
  SELECT
    o.id, o.name, o.code, o.address, o.latitude, o.longitude,
    ot.name  AS office_type_name,
    ot.tier  AS tier,
    p.name   AS province_name,
    c.name   AS city_name,
    (SELECT count(*) FROM asset.assets a
       WHERE a.office_id = o.id AND a.deleted_at IS NULL) AS asset_count
  FROM masterdata.offices o
  LEFT JOIN masterdata.office_types ot ON ot.id = o.office_type_id AND ot.deleted_at IS NULL
  LEFT JOIN masterdata.provinces    p  ON p.id  = o.province_id    AND p.deleted_at IS NULL
  LEFT JOIN masterdata.cities       c  ON c.id  = o.city_id        AND c.deleted_at IS NULL
  WHERE o.deleted_at IS NULL
    AND o.is_active = true
    AND (sqlc.arg(all_scope)::bool OR o.id = ANY(sqlc.arg(office_ids)::uuid[]))
  ORDER BY o.name;
  ```
  (Mengembalikan **semua** kantor aktif dalam scope, termasuk yang `latitude`/`longitude` NULL — frontend yang memutuskan pin.)
- Handler baru `mapList` di `office` package: resolve scope via `CallerOfficeScope(c, "offices")`, jalankan `ListOfficesMap`, serialisasi ke response peta. Route `GET /api/v1/offices/map` di `office/routes.go`, **authMW + scope only** (tanpa `RequirePermission`, konsisten dgn `GET /offices`). Pastikan route `/offices/map` didaftarkan sebelum/aman terhadap `/offices/:id` (hindari bentrok param — daftarkan path statik `map` lebih dulu atau di grup terpisah).
- Response item (JSON, English): `{ id, name, code, office_type_name, tier, province_name, city_name, address, asset_count, latitude, longitude }`. `tier` ∈ `"pusat"|"wilayah"|"office"|null`. `latitude`/`longitude` `number|null`. `asset_count` integer. Envelope: kembalikan `{ "data": [...] }` (peta memuat seluruh set ter-scope; **tanpa** paginasi — konsisten kebutuhan peta, beda dari list paginasi).

### 2.4 openapi & tes backend
- `backend/api/openapi.yaml`: tambah path `GET /offices/map` (response array item di atas) + field `latitude`/`longitude` pada schema office create/update/response. Lolos Spectral.
- Tes Go (`office` package, sesuai gaya tes existing): 
  - `ListOfficesMap` menghormati scope (global vs office_ids), me-resolve `office_type_name`/`province_name`/`city_name` (termasuk NULL FK → nama null), `asset_count` benar (0 tanpa aset; >0 dengan aset office tsb), passthrough lat/lng.
  - Validasi DTO: lat/lng di luar rentang → 400; create/update menyimpan & mengembalikan lat/lng.
  - Jalankan `go build ./...`, `go vet ./...`, `go test ./...`, dan (per gate) `go test -tags=integration ./...` setelah perubahan signature bersama.

## 3. Frontend — composable, tipe & meta

### 3.1 `useOfficeMap.ts` — tulis ulang
Hapus impor `~/mock/*`. Tipe:
```ts
type OfficeTier = 'pusat' | 'wilayah' | 'office'
interface MapOffice {
  id: string
  name: string
  code: string
  office_type_name: string | null
  tier: OfficeTier            // null backend → 'office' (fallback Cabang)
  province_name: string | null
  city_name: string | null
  address: string | null
  asset_count: number
  latitude: number | null
  longitude: number | null
}
```
Fungsi (via `useApiClient().request`):
- `async list(): Promise<MapOffice[]>` — `GET /offices/map`; map tiap item: `tier = raw.tier ?? 'office'`; sisanya passthrough. Kembalikan array.

### 3.2 Tipe `~/types` + meta kategori
- Ganti `MapOffice`/`OfficeJenis` lama (Indonesia) di `~/types` dengan bentuk English di atas + `OfficeTier`. (Cek importer lain; hanya map.vue + mock yang memakainya.)
- Buat `app/constants/officeMapMeta.ts` (pindahan dari `mock/officeMap.ts`): `tierMeta: Record<OfficeTier, { labelKey, pinVar, softBg, softText, icon }>` (3 kategori — pusat/wilayah/office; pakai ulang token & ikon yang ada: pusat=primary/landmark, wilayah=info/building-2, office=warning/building) + `TIER_ORDER: OfficeTier[] = ['pusat','wilayah','office']`. Hapus `mock/officeMap.ts`.

### 3.3 Page `map.vue`
- Rebind field: `nama→name`, `kode→code`, `prov→province_name`, `kota→city_name`, `alamat→address`, `aset→asset_count`, `jenis→tier`; `jenisMeta→tierMeta`, `JENIS_ORDER→TIER_ORDER`; impor dari `~/constants/officeMapMeta` (bukan mock).
- Filter provinsi: opsi dari `province_name` unik (skip null); search match `name`/`code`. Nilai null kota/provinsi → tampilkan em-dash.
- **State**: tambah `loadFailed` + tombol retry (saat ini hanya `loading`). `onMounted`/`reload()` membungkus `list()` dgn try/catch.
- Kantor tanpa koordinat: **tidak** dipin di Leaflet (filter `latitude!=null && longitude!=null` sebelum diteruskan ke `OfficeMap`); tetap muncul di panel daftar. Kartu detail: tombol "Buka di Google Maps" disabled/ disembunyikan bila koordinat null.
- Kartu detail "X aset terdaftar" memakai `asset_count` (boleh 0).

### 3.4 Komponen `OfficeMap` (Leaflet)
- Konsumsi `latitude`/`longitude` (bukan `lat`/`lng`) dan `tier` untuk warna pin (`tierMeta[o.tier].pinVar`). Terima hanya office berkoordinat (parent sudah memfilter), tapi tetap defensif terhadap null.

### 3.5 i18n
- `map.jenis.*` → label tier: `map.tier.pusat`="Pusat"/"Pusat", `map.tier.wilayah`="Wilayah"/"Region", `map.tier.office`="Cabang"/"Branch" (id/en). (Atau pertahankan key `map.jenis.*` bila lebih kecil diff-nya — yang penting 3 label tier.)
- Tambah `map.loadError`/`map.retry` (id/en). Semua string user-facing via `$t`.

## 4. Pengujian (proaktif & luas)

- **Unit** (`test/unit/use-office-map.spec.ts`, mock `~/composables/useApiClient`): `list()` memanggil `GET /offices/map`, memetakan item (tier null→'office', passthrough nama/koordinat/asset_count), mengembalikan array. Hapus tes mock lama bila ada.
- **Component** (`test/nuxt/master-map.spec.ts`, stub `/offices/map`): render baris daftar dgn nama/kode/tier badge/kota+provinsi ter-resolve; filter jenis(tier) & provinsi menyaring daftar; pilih baris → kartu detail (alamat, kota/prov, asset_count); kantor null-koordinat tetap di daftar tapi tombol Maps tak aktif; **load-error 500 → blok error + retry**; empty state saat `data:[]`. (Leaflet via `ClientOnly`/stub `OfficeMap`.)
- **E2E** (`e2e/` — file map sendiri atau blok di settings/master spec): backend nyata + admin seeded — buat 1 office (dengan tipe/provinsi/kota + `latitude`/`longitude`) via API office nyata, buka `/master/map`, assert kantor itu muncul di panel daftar (locator teks robust; USelect via trigger+option, **bukan** `selectOption`; tanpa `isVisible()` snapshot/`.first()` div luas). Bersihkan data uji bila perlu.

## 5. "Selesai"

- Backend: `go build/vet/test ./...` + `go test -tags=integration ./...` hijau; Spectral lint hijau.
- Frontend: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau (E2E di CI).
- Hapus `mock/officeMap.ts` (importer dipindah ke `constants/officeMapMeta.ts`); cek tak ada importer tersisa.
- Bandingkan `pages/master/map.vue` ke `docs/design/Peta Lokasi.dc.html`: layout 2 kolom / panel daftar / filter / legenda / kartu detail cocok. Penyimpangan disetujui: legenda **3 kategori** (tier) alih-alih 4 (Outlet melebur ke Cabang). Selebihnya 1:1.
- `docs/PROGRESS.md`: Peta Lokasi ✅ wired ke `GET /offices/map` (lat/lng kolom + endpoint geo + scope); TODO: `tier` belum dapat diedit (menunggu office-types reference meng-expose `tier`) → kantor tier-NULL tampil sebagai Cabang; refresh "▶ Next session" → sub-proyek master berikutnya = **Referensi**.

## 6. Risiko & catatan
- **Tier kosong**: hingga `tier` dapat diisi, sebagian/seluruh kantor jatuh ke kategori Cabang. Diterima (data-faithful) + dicatat sbg TODO.
- **Peta kosong di awal**: tanpa seed produksi, peta empty sampai ada kantor berkoordinat. Empty-state mockup menanganinya; e2e mengisi via API.
- **Bentrok route** `/offices/map` vs `/offices/:id`: daftarkan path statik lebih dulu / pastikan router Gin tak menafsirkan `map` sebagai `:id`.
- **numeric↔number**: pastikan lat/lng melintas sebagai JSON `number` (bukan string) agar Leaflet & validasi frontend langsung pakai; tentukan mapping sqlc di plan.

## 7. Roadmap batch "wiring menu master" (konteks; tiap item spec→plan→PR sendiri)
1. **Peta Lokasi** — sub-proyek ini.
2. **Referensi** — 11 resource generic engine; tambah FK picker `cities`→`province_id`, `models`→`brand_id`; (opsional) expose `office_types.tier`.
3. **Kategori Aset** — DTO 9-field bank-FAM + tree `parent_id`.
4. **Pegawai** — scope `employees` + FK picker (office/department/position).
5. **Kantor + Lantai + Ruangan** — tree, scope `offices`, floors/rooms inline, input lat/lng (DTO sudah ada dari langkah 1).
