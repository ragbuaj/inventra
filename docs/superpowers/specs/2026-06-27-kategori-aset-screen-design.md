# Spec — Layar Master Data: Kategori Aset

| | |
|---|---|
| **Tanggal** | 2026-06-27 |
| **Fase** | Master data (frontend) — melengkapi satu master entity tanpa layar |
| **Mockup** | `docs/design/Kategori Aset.dc.html` (sumber kebenaran visual, dibangun 1:1) |
| **Prompt desain** | `docs/DESIGN_BRIEF.md` bagian 5.24 |
| **Status** | Disetujui — siap menulis implementation plan |

## 1. Tujuan & ruang lingkup

Membangun layar **Master Data → Kategori Aset**: mengelola golongan aset tetap beserta nilai
default akuntansi/pajaknya (klasifikasi tangible/intangible, penyusutan komersial PSAK 16, fiskal
PMK 72/2023, akun GL, batas kapitalisasi). Pengguna utama: **Superadmin**. Ini satu-satunya master
entity yang belum punya layar frontend; backend API-nya (`/api/v1/masterdata/categories`) sudah ada.

**Dalam ruang lingkup:** halaman list + filter + pagination, form tambah/edit (slideover 4 section),
konfirmasi hapus, semua state (data/empty/error/loading), i18n id+en, entri nav, unit + runtime +
**e2e** tests, paritas light & dark dengan mockup.

**Di luar ruang lingkup:** wiring ke backend asli (tetap **mock-first** di belakang interface yang
sama — swap ke `$fetch` dilakukan pada fase wiring lintas-layar tersendiri); refactor composable lama
ke English keys (ADR-0007, terpisah).

## 2. Keputusan desain (disepakati)

1. **Kontrak field = English `snake_case`** mengikuti backend persis (ADR-0007). `useCategories` lahir
   dengan konvensi baru; composable lama (Indonesian keys) **tidak** diubah di sini.
2. **Mock-first** di belakang seam `composables/api/useCategories.ts`. Karena tipe & composable sudah
   memakai kontrak backend, swap ke `$fetch` nanti tidak mengubah halaman/komponen/tipe.
3. **E2E termasuk scope** — mengikuti pola `frontend/e2e/master-offices.spec.ts` (login real ke
   backend, lalu CRUD di layar mock-backed).

## 3. Arsitektur & berkas

```
app/pages/master/categories.vue            ← halaman tipis: state + handler + komposisi U*
app/components/category/CategoryTable.vue   ← tabel + pagination + indentasi/badge (props rows; emit edit/delete)
app/components/category/CategoryFormSlideover.vue ← form 4 section + perilaku kondisional + validasi (emit save/close)
app/composables/api/useCategories.ts        ← list/get/create/update/remove (mock di belakang interface)
app/mock/categories.ts                      ← seed + createStore + peta label (golongan/metode/kelas)
app/types/index.ts                          ← + interface Category, CategoryInput
i18n/locales/id.json, en.json               ← + masterdata.categories.*, + nav.categories
app/utils/nav.ts                            ← + entri Kategori di grup Master Data
```

Filter bar dibiarkan **inline** di halaman (ringkas); hanya `CategoryTable` & `CategoryFormSlideover`
yang diekstrak menjadi komponen (mengikuti aturan "pages tipis" CLAUDE.md). Konfirmasi hapus memakai
composable `useConfirm()` yang sudah ada (seperti `offices.vue`), bukan modal manual.

Gating halaman: `definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })`
(kategori = data global; tulis butuh `masterdata.global.manage` di backend).

## 4. Kontrak data

```ts
// app/types/index.ts
export type AssetClass = 'tangible' | 'intangible'
export type DepreciationMethod = 'straight_line' | 'declining_balance'
export type FiscalGroup =
  | 'kelompok_1' | 'kelompok_2' | 'kelompok_3' | 'kelompok_4'
  | 'bangunan_permanen' | 'bangunan_non_permanen' | 'non_susut'

export interface Category {
  id: string
  name: string
  code: string | null
  parent_id: string | null
  default_depreciation_method: DepreciationMethod | null
  default_useful_life_months: number | null
  default_salvage_rate: string | null          // numeric → string (mengikuti konvensi backend money/rate)
  asset_class: AssetClass
  default_fiscal_group: FiscalGroup | null
  default_fiscal_life_months: number | null
  gl_account_code: string | null
  capitalization_threshold: string | null       // numeric → string
  is_active: boolean
  created_at: string
}

// CategoryInput = Category tanpa { id, created_at }
```

`useCategories` mengekspos `list(query): Paginated<Category>`, `get(id)`, `create(input)`,
`update(id, input)`, `remove(id)` — selaras `useEmployees` (mock store + `fakeLatency`).

Seed `mock/categories.ts` mengikuti contoh bagian 5.24 (Perangkat IT→Komputer & Laptop sebagai induk→anak,
Kendaraan, Bangunan Kantor, Mesin ATM, Mebel, Software/Lisensi takberwujud, satu entri nonaktif).

## 5. Tata letak layar (1:1 mockup)

- **Header**: judul "Kategori Aset" + subjudul + tombol **Tambah Kategori** (`UButton`, ikon plus).
- **Filter bar** (`UCard`/div): `UInput` search nama·kode; `USelect` **Kelas Aset** (Semua/Berwujud/
  Takberwujud); `USelect` **Golongan Pajak** (Semua + 6 golongan); toggle **Hanya aktif**.
- **Tabel** (`UTable`): kolom **Nama** (indentasi + ikon belok untuk anak bila `parent_id`≠null),
  **Kode** (badge mono), **Kelas Aset** (`UBadge`), **Metode Susut** (komersial; "—" bila kosong),
  **Masa (bln)** (rata-kanan, tabular-nums), **Golongan Pajak** (label; "—" bila kosong), **Akun GL**
  (mono; "—"), **Status** (`UBadge` Aktif/Nonaktif), **Aksi** (edit/hapus). Footer: info "Menampilkan
  a–b dari N" + `UPagination` (page size 7).
- **Empty state**: kartu ikon + judul "Belum ada kategori" + sub + tombol Tambah.

## 6. Form slideover (4 section)

`USlideover` kanan, lebar ~520px, header judul + sub (Tambah/Edit), footer Batal + Simpan.

1. **Umum** — Nama* , Kode* , Kategori Induk (`USelect` opsional; opsi = kategori lain, **kecuali
   dirinya sendiri & keturunannya** saat edit), Kelas Aset (segmented Berwujud/Takberwujud), toggle Aktif.
2. **Penyusutan Komersial (PSAK 16)** — Metode (`USelect` Garis Lurus/Saldo Menurun), Masa Manfaat
   (bln), Nilai Residu (%).
3. **Pajak / Fiskal (PMK 72/2023)** — Golongan/Kelompok Harta (`USelect` 6 opsi), Masa Manfaat Fiskal (bln).
4. **Akuntansi** — Akun GL (COA, mono), Batas Kapitalisasi (Rp, diformat ribuan `id-ID`).

### Perilaku kondisional (wajib, dari bagian 5.24)
- **Kelas = Takberwujud** → judul section 2 jadi **"Amortisasi"** + ref **PSAK 19** (bukan
  "Penyusutan"/PSAK 16); opsi golongan **Bangunan** disembunyikan dari select fiskal.
- **Golongan = Bangunan (permanen/non-permanen)** → Metode dipaksa **Garis Lurus** + field **disabled**
  + nota "Aset bangunan wajib memakai Garis Lurus".

### Validasi
- **Nama** & **Kode** wajib → error inline "Wajib diisi" + border merah; Simpan diblok bila invalid.
- Field numerik (masa, residu, kapitalisasi) menerima angka; kapitalisasi diformat ribuan saat tampil,
  disimpan sebagai string numeric pada kontrak.

## 7. i18n

Semua string di `i18n/locales/{id,en}.json` di bawah `masterdata.categories.*` (mengikuti penamaan key
pada mockup: pageTitle, pageSub, kolom, label form, section, hint, pesan konfirmasi/empty) + `nav.categories`.
EN memakai padanan natural (IAS 16/38 untuk PSAK 16/19, "Tax Group", dst — sesuai blok `en` di mockup).
Tidak ada string UI yang di-hardcode.

## 8. Nav

`app/utils/nav.ts` grup Master Data: tambah `{ labelKey: 'nav.categories', to: '/master/categories' }`
**setelah Pegawai** (urutan final: Kantor → Pegawai → **Kategori** → Peta Lokasi → Referensi).

## 9. Testing (proaktif & luas)

**Unit** (`test/unit/categories-mock.spec.ts`, node env):
- store CRUD; filter search (nama·kode), filter kelas, filter golongan, toggle active-only; pagination
  (page size 7, batas a–b–N); helper format ribuan.
- pembentukan opsi induk yang mengecualikan diri sendiri + keturunan saat edit.

**Composable** (`test/unit/useCategories.spec.ts` atau gabung): list/get/create/update/remove, termasuk
error not-found pada update/remove id tak dikenal.

**Runtime mount** (`// @vitest-environment nuxt`):
- `CategoryTable.spec.ts` — render baris, badge kelas & status, indentasi anak, kolom "—" untuk nilai
  kosong, empty state, emit `edit`/`delete`.
- `CategoryFormSlideover.spec.ts` — render 4 section; **perilaku kondisional** (Takberwujud → label
  Amortisasi/PSAK 19 + opsi Bangunan hilang; Bangunan → metode = Garis Lurus & disabled + nota); validasi
  Nama/Kode wajib; **emit `save` dengan payload `snake_case` benar** (assert nilai field, bukan sekadar
  panjang HTML).

**i18n**: assert sejumlah key id & en ter-resolve (mis. pageTitle, secSusut vs secSusutAmort).

**E2E** (`frontend/e2e/categories.spec.ts`, pola `master-offices.spec.ts`): login real → buka
`/master/categories` → lihat kategori ter-seed → buka form Tambah → isi Nama+Kode (+kelas/golongan) →
Simpan → baris baru tampil di tabel. Tambah satu skenario: filter search mempersempit daftar.

**Verifikasi akhir**: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` hijau; lalu perbandingan
**1:1 side-by-side** layar vs mockup di light **dan** dark (layout, spacing, hierarki, setiap state &
field). Perbaiki gap sebelum klaim selesai; laporkan hasil perbandingan. (E2E dijalankan di job e2e CI
yang butuh stack up + admin ter-seed.)

## 10. Risiko & catatan

- **Paritas mockup**: indentasi induk-anak, badge, dan lebar slideover harus cocok; verifikasi visual akhir mengikat.
- **Format angka**: kapitalisasi/residu pakai locale `id-ID`; pastikan parsing balik ke string numeric
  bersih (strip pemisah ribuan) agar payload kontrak benar.
- **Page size 7** sengaja kecil (mengikuti mockup) — pagination harus benar pada >7 seed.
- Saat swap ke API asli nanti: `default_salvage_rate`/`capitalization_threshold` adalah string numeric
  (konvensi money backend) — UI mengonversi tampilan↔kontrak.
