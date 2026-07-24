# Penyelarasan Model Data untuk Penggantian Sistem Lama (Legacy Parity) — Design

**Tanggal:** 2026-07-23 · **Status:** Disetujui user — siap implementasi
**Konteks:** Inventra akan menggantikan sepenuhnya sistem manajemen aset lama (stack usang
Vue2 + Yii2 + PostgreSQL 12, tanpa Redis) yang saat ini piloting di beberapa kantor cabang milik
anak perusahaan sebuah bank. Sistem lama belum benar-benar dipakai (user masih opname manual), jadi
ini window termurah untuk menyamakan model data sebelum cutover.
**Referensi:** skema saat ini — `000003_identity`, `000006_masterdata`, `000007_offices_employees`,
`000008_asset`, `000015_fam_tables`, `000016_office_tier` · penomoran tag lama:
`formatAssetTag` + `asset.asset_tag_counters` (`BumpAssetTagCounter`) · konvensi DATABASE.md
(soft-delete, partial-unique, `set_updated_at`, enum di `shared`, money numeric ke Go string) ·
ADR-0008 (split modul & konvensi masterdata: generic reference engine vs sub-package ter-scope).

## Tujuan

Menutup gap antara field yang diminta user (spesifikasi target untuk ASET, PEGAWAI, USER, KANTOR
plus master pendukung) dan skema Inventra saat ini, sehingga Inventra mencapai paritas fungsional
dengan sistem lama dan bisa menggantikannya. Cakupan: tambahan kolom, master baru, tabel history,
pelonggaran constraint lokasi, penomoran kode aset baru, fitur batch registrasi, login
NIP-atau-email, dan divisi per-kantor.

## Keputusan produk (dikonfirmasi user 2026-07-23)

1. **Kapasitas = teks spesifikasi bebas** (mis. "2 PK" untuk AC), bukan jumlah. Satu row = satu
   aset fisik; 10 kursi di satu ruangan = 10 row. Konsekuensi: perlu **fitur batch registrasi**.
2. **Aset boleh berhenti di level lantai** (tanpa ruangan). Constraint wajib-ruangan dilonggarkan.
3. **Riwayat lokasi sampai level ruangan**, ditampilkan dari halaman Detail Aset.
4. **Divisi pelaksana = master** (bukan enum): engineering, security, housekeeping, parkir, operator.
5. **User login memakai NIP atau email** (dua-duanya diterima).
6. **Divisi kantor bersifat per-kantor** (bukan master global). Departemen **wajib** punya kantor.
7. **History untuk PIC dan Pemegang**: Pemegang memakai ulang modul Assignment yang sudah ada; PIC
   memakai kolom + tabel history baru.
8. **Kode aset tetap auto-generate, format tetap tapi scope sequence baru** (keputusan 2026-07-23):
   - **Tanpa tanda `-`.** Format: `{KODE_KANTOR}{KODE_KATEGORI}{TAHUN_PEMBELIAN}{NNNNN}` — mis.
     `JKT01ELK202600001` (tahun 4 digit, sequence 5 digit zero-padded).
   - **Sequence (NNNNN) per-kantor** (bukan lagi per kantor+kategori+tahun): satu deret berjalan
     office-wide lintas kategori/tahun. Kategori & tahun pembelian **tetap tampil** sebagai bagian
     deskriptif kode, tapi angka urutnya per-kantor.
   - **Tidak boleh dipakai ulang**: sequence terus maju; hanya turun bila aset **hard-delete** dan
     **tidak ada aset dengan sequence lebih tinggi** (yang teratas). Soft-delete tetap menahan nomor.
   - **Tag immutable pasca-pembuatan**: mengedit `purchase_date`/kategori **tidak** menomori ulang
     tag (identitas permanen, tercetak di label).

### Sub-keputusan (diputuskan)
- **Batch + maker-checker:** satu request `asset_create` menghasilkan **N aset** identik (tag per unit
  di-generate berurutan saat approval). Bukan N request terpisah.
- **Klasifikasi gedung:** tersaran otomatis dari jumlah lantai, **tetap bisa diubah** (bukan dikunci).
- **`ownership_status` & `office_kind`:** **enum** (bukan master). `ownership_status` provisional —
  konversi ke master adalah perubahan kecil bila kelak bank ingin mengelola sendiri.
- **`company_id` pegawai:** **opsional** (nullable).
- **Departemen global lama:** user menyiapkan datanya nanti; migrasi **perbaiki tabel dulu** (tambah
  `office_id`). Karena data belum siap, DB constraint NOT NULL **ditunda** ke migrasi lanjutan; sifat
  wajib ditegakkan di layer aplikasi (DTO/service) sejak awal.

## Penomoran kode aset baru — rancangan detail (keputusan 8)

Aturan "tidak dipakai ulang kecuali hard-delete yang teratas" paling bersih **diturunkan dari data**,
bukan counter tersimpan: `asset_tag_counters` (per kantor/kategori/tahun) **dihapus**, diganti kolom
numerik `asset.assets.tag_seq` per aset dan sequence dihitung saat pembuatan:

```
next_seq(office) = COALESCE(MAX(tag_seq) di semua row office itu, 0) + 1
```

- MAX dihitung atas **seluruh baris** kantor tsb **termasuk yang soft-delete** (menahan nomor),
  otomatis **tidak** termasuk yang hard-delete (baris hilang). Jadi hard-delete baris teratas
  menurunkan MAX ke satu nomor bisa dipakai lagi; hard-delete baris tengah tak mengubah MAX (masih
  ada yang di atasnya) sehingga nomornya tetap tidak dipakai ulang — persis aturan user.
- **Serialisasi** per kantor dengan `pg_advisory_xact_lock(hashtext('asset_tag:'||office_id))` di
  transaksi pembuatan (batch N aset mengambil N nomor berurutan dalam satu lock). Unique index
  `asset_tag` jadi backstop.
- `asset_tag` = `office.code || category.code || tahun_pembelian || lpad(tag_seq::text, 5, '0')`
  via `formatAssetTag(officeCode, categoryCode, purchaseYear, seq)`. Tahun = tahun `purchase_date`;
  bila `purchase_date` kosong, fallback ke tahun pembuatan aset. `category.code` wajib ada untuk
  kategori yang dipakai (validasi service). NNNNN = `tag_seq` (per-kantor), bukan bagian
  kategori/tahun — komposisi string hanya kosmetik.

## Batasan & catatan jujur

- **`departments` berpindah pola**: dari generic reference engine (datar) menjadi resource
  **ter-scope kantor** (mirip pola `office/`/`employee/` di ADR-0008), atau engine diberi filter
  `office_id`. Ini kerja arsitektur, bukan sekadar kolom.
- **`building_classifications` punya kolom numerik** (min/max lantai) sehingga **tidak muat** di
  generic reference engine (text/bool/uuid saja) — perlu sub-package kecil sendiri.
- **Re-tag aset eksisting**: migrasi penomoran akan **menomori ulang** aset yang ada ke format baru
  (data pilot minimal & belum dipakai nyata, label belum dicetak) — aman untuk konteks ini.

## 1. Backend — migrasi `000038` enum baru (`shared`)

```sql
CREATE TYPE shared.office_ownership AS ENUM ('sewa', 'milik', 'hg_pakai', 'free');
CREATE TYPE shared.office_kind      AS ENUM ('konvensional', 'syariah');
CREATE TYPE shared.location_change_source AS ENUM ('registration', 'edit', 'transfer', 'migration');
```

## 2. Backend — migrasi `000039` kolom aset + pelonggaran constraint

```sql
ALTER TABLE asset.assets
  ADD COLUMN capacity           text,
  ADD COLUMN lease_date         date,
  ADD COLUMN installation_date  date,
  ADD COLUMN warranty_start     date,
  ADD COLUMN floor_id           uuid REFERENCES masterdata.floors (id),
  ADD COLUMN pic_employee_id    uuid REFERENCES masterdata.employees (id);
CREATE INDEX idx_assets_floor_id ON asset.assets (floor_id);
CREATE INDEX idx_assets_pic      ON asset.assets (pic_employee_id);

-- Lokasi boleh berhenti di lantai: tangible wajib floor_id ATAU room_id.
ALTER TABLE asset.assets DROP CONSTRAINT chk_assets_tangible_room;
ALTER TABLE asset.assets ADD CONSTRAINT chk_assets_tangible_location
  CHECK (asset_class = 'intangible' OR floor_id IS NOT NULL OR room_id IS NOT NULL);
```

`notes` (sudah ada) dipakai untuk "deskripsi". Bila `room_id` diisi, `floor_id` harus konsisten
dengan lantai ruangan itu (divalidasi di service).

## 3. Backend — migrasi `000040` penomoran kode aset baru

```sql
ALTER TABLE asset.assets ADD COLUMN tag_seq int;
CREATE INDEX idx_assets_office_tagseq ON asset.assets (office_id, tag_seq);

-- Backfill tag_seq per kantor (termasuk soft-delete agar nomor tertahan) + re-tag ke format baru.
WITH ranked AS (
  SELECT id, office_id,
         row_number() OVER (PARTITION BY office_id ORDER BY created_at, id) AS seq
  FROM asset.assets
)
UPDATE asset.assets a
SET tag_seq   = r.seq,
    asset_tag = o.code || COALESCE(c.code, '') ||
                to_char(COALESCE(a.purchase_date, a.created_at::date), 'YYYY') ||
                lpad(r.seq::text, 5, '0')
FROM ranked r
JOIN masterdata.offices o    ON o.id = r.office_id
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.id = r.id;

ALTER TABLE asset.assets ALTER COLUMN tag_seq SET NOT NULL;

DROP TABLE asset.asset_tag_counters;  -- digantikan derivasi MAX(tag_seq)+1
```

Query: hapus `BumpAssetTagCounter`; tambah `GetMaxTagSeqForOffice` (`SELECT COALESCE(MAX(tag_seq),0)
FROM asset.assets WHERE office_id = $1`) + panggilan `pg_advisory_xact_lock`. `formatAssetTag` jadi
`(officeCode, categoryCode string, purchaseYear, seq int)` →
`fmt.Sprintf("%s%s%d%05d", officeCode, categoryCode, purchaseYear, seq)`; `tag_test.go` disesuaikan.
`GetOfficeCode` tetap dipakai; kode kategori & tahun pembelian diambil dari row aset saat pembuatan.

## 4. Backend — migrasi `000041` tabel history aset

```sql
CREATE TABLE asset.asset_location_history (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id    uuid NOT NULL REFERENCES asset.assets (id) ON DELETE CASCADE,
  office_id   uuid NOT NULL REFERENCES masterdata.offices (id),
  floor_id    uuid REFERENCES masterdata.floors (id),
  room_id     uuid REFERENCES masterdata.rooms (id),
  source      shared.location_change_source NOT NULL DEFAULT 'edit',
  moved_at    timestamptz NOT NULL DEFAULT now(),
  moved_by_id uuid REFERENCES identity.users (id),
  transfer_id uuid REFERENCES transfer.asset_transfers (id),
  note        text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE INDEX idx_asset_loc_hist_asset ON asset.asset_location_history (asset_id, moved_at DESC);

CREATE TABLE asset.asset_pic_history (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id        uuid NOT NULL REFERENCES asset.assets (id) ON DELETE CASCADE,
  pic_employee_id uuid NOT NULL REFERENCES masterdata.employees (id),
  assigned_at     timestamptz NOT NULL DEFAULT now(),
  released_at     timestamptz,
  assigned_by_id  uuid REFERENCES identity.users (id),
  note            text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE UNIQUE INDEX uq_asset_pic_active ON asset.asset_pic_history (asset_id)
  WHERE released_at IS NULL AND deleted_at IS NULL;
CREATE INDEX idx_asset_pic_hist_asset ON asset.asset_pic_history (asset_id, assigned_at DESC);

-- Backfill satu baris lokasi awal per aset eksisting.
INSERT INTO asset.asset_location_history (asset_id, office_id, floor_id, room_id, source, moved_at)
SELECT a.id, a.office_id, a.floor_id, a.room_id, 'migration', a.created_at
FROM asset.assets a WHERE a.deleted_at IS NULL;
-- + trigger set_updated_at untuk kedua tabel
```

**Pemegang: tanpa tabel baru** — riwayat pemegang = `assignment.assignments` (modul Assignment).
**Penulis history lokasi** disisipkan di: executor `asset_create`, update aset saat
`office_id`/`floor_id`/`room_id` berubah, dan `transfer` receive (tautkan `transfer_id`). **Penulis
history PIC**: saat `pic_employee_id` di-set/diganti — tutup baris aktif (`released_at = now()`),
buka baris baru.

## 5. Backend — migrasi `000042` master baru

```sql
-- Datar → generic reference engine:
CREATE TABLE masterdata.office_classes (        -- kelas kantor
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX uq_office_classes_name ON masterdata.office_classes (name) WHERE deleted_at IS NULL;

CREATE TABLE masterdata.executor_divisions (    -- divisi pelaksana
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX uq_executor_divisions_name ON masterdata.executor_divisions (name) WHERE deleted_at IS NULL;
INSERT INTO masterdata.executor_divisions (name) VALUES
  ('Engineering'), ('Security'), ('Housekeeping'), ('Parkir'), ('Operator');

CREATE TABLE masterdata.companies (             -- perusahaan pegawai
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz);
CREATE UNIQUE INDEX uq_companies_name ON masterdata.companies (name) WHERE deleted_at IS NULL;

-- Punya numerik → sub-package sendiri (BUKAN generic engine). max_floors NULL = "25+" (tak terbatas).
CREATE TABLE masterdata.building_classifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
  min_floors int NOT NULL, max_floors int, is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
  CONSTRAINT chk_bldg_floor_range CHECK (max_floors IS NULL OR max_floors >= min_floors));
CREATE UNIQUE INDEX uq_building_classifications_name ON masterdata.building_classifications (name) WHERE deleted_at IS NULL;
-- + trigger set_updated_at untuk semua
```

## 6. Backend — migrasi `000043` kolom kantor

```sql
ALTER TABLE masterdata.offices
  ADD COLUMN ownership_status           shared.office_ownership,
  ADD COLUMN office_class_id            uuid REFERENCES masterdata.office_classes (id),
  ADD COLUMN building_classification_id uuid REFERENCES masterdata.building_classifications (id),
  ADD COLUMN floor_count                int,
  ADD COLUMN building_area              numeric(12,2),
  ADD COLUMN office_kind                shared.office_kind NOT NULL DEFAULT 'konvensional',
  ADD COLUMN description                text,
  ADD COLUMN head_employee_id           uuid REFERENCES masterdata.employees (id),
  ADD COLUMN contact                    text;
CREATE INDEX idx_offices_class_id ON masterdata.offices (office_class_id);
CREATE INDEX idx_offices_bldg_class_id ON masterdata.offices (building_classification_id);
CREATE INDEX idx_offices_head_employee ON masterdata.offices (head_employee_id);
```

UI menyaran `building_classification_id` dari `floor_count` (baris master yang rentang min/max-nya
memuat `floor_count`), tetap bisa diubah manual.

## 7. Backend — migrasi `000044` pegawai + divisi per-kantor

```sql
ALTER TABLE masterdata.employees
  ADD COLUMN company_id           uuid REFERENCES masterdata.companies (id),          -- opsional
  ADD COLUMN executor_division_id uuid REFERENCES masterdata.executor_divisions (id);
CREATE INDEX idx_employees_company ON masterdata.employees (company_id);
CREATE INDEX idx_employees_exec_div ON masterdata.employees (executor_division_id);

-- Divisi kantor per-kantor. office_id ditambah nullable dulu (data disiapkan user kemudian);
-- NOT NULL ditegakkan di app layer sekarang, di DB via migrasi lanjutan setelah data siap.
ALTER TABLE masterdata.departments ADD COLUMN office_id uuid REFERENCES masterdata.offices (id);
CREATE INDEX idx_departments_office ON masterdata.departments (office_id);
DROP INDEX IF EXISTS masterdata.uq_departments_code;
CREATE UNIQUE INDEX uq_departments_office_code ON masterdata.departments (office_id, code)
  WHERE deleted_at IS NULL AND code IS NOT NULL;
```

Validasi service: `employees.department_id` harus milik `employees.office_id` yang sama; department
create/update wajib `office_id`.

## 8. Backend — migrasi `000045` login NIP-atau-email

```sql
ALTER TABLE identity.users ADD COLUMN username text;
CREATE UNIQUE INDEX uq_users_username ON identity.users (username)
  WHERE deleted_at IS NULL AND username IS NOT NULL;
UPDATE identity.users u SET username = e.code
FROM masterdata.employees e
WHERE u.employee_id = e.id AND u.username IS NULL AND u.deleted_at IS NULL;
```

Handler login menerima satu field identifier: cari user by `email` ATAU `username`. Jalur Google
OAuth tak terpengaruh; rate-limit & revocation tetap.

## 9. Fitur batch registrasi aset

Satu request `asset_create` dengan field `quantity` (default 1). Payload menyimpan template aset +
`quantity`. Saat approval, executor membuat **N** baris aset; tiap unit mengambil `tag_seq` berurutan
(`MAX(tag_seq)+1` di bawah advisory lock kantor). Amount approval = `purchase_cost * quantity`
(cross-check server mengikuti pola `SubmitRequest.validate()`). Frontend: field "Jumlah" + ringkasan
"akan dibuat N aset" sebelum submit.

## 10. Dampak turunan (checklist)

- **sqlc**: query baru/ubah assets (kolom + tag_seq, hapus BumpAssetTagCounter, tambah
  GetMaxTagSeqForOffice + advisory lock), offices, employees, users, 4 master, 2 history →
  `sqlc generate`. Money tetap numeric→string.
- **Reference engine** (`reference/resources.go`): tambah `office_classes`, `executor_divisions`,
  `companies`. `building_classifications` = sub-package baru. `departments` jadi resource ter-scope kantor.
- **DTO/handler**: assets (capacity, tanggal, pic, floor, quantity), offices (9 field + 2 picker
  master + head employee), employees (company, executor division, department per-kantor), users (username).
- **Frontend**: form Aset (kapasitas, tanggal instalasi/garansi-mulai/sewa, PIC, lokasi berhenti di
  lantai, jumlah/batch), form Kantor (9 field + auto-saran klasifikasi), form Pegawai (perusahaan,
  divisi pelaksana, divisi kantor tersaring kantor), login (NIP atau email); layar master baru di
  Referensi; tab "Riwayat Lokasi" + "Riwayat PIC" + "Riwayat Pemegang" di Detail Aset.
- **Field-permission catalog**: tak ada field finansial baru; `capacity`/`pic` non-sensitif (tinjau
  bila ada peran yang perlu masking).
- **OpenAPI**: sinkronkan semua path/skema yang berubah; Spectral hijau.
- **Test**: unit (formatAssetTag baru + derivasi MAX/advisory-lock, validasi department-in-office,
  penulis history, batch executor, login by-username), component (form baru, tab history), e2e
  (registrasi batch + tag berurutan, perpindahan lokasi tercatat, login NIP). Cakupan proaktif.
- **Dokumen**: update `docs/DATABASE.md` (kolom/tabel/enum baru, skema tag baru) + `docs/PROGRESS.md`
  (centang saat landing) + `docs/ERD.md` bila relevan.

## 11. Fase implementasi (satu commit per fase, gate task-13 tiap fase)

1. **Kolom aset + constraint** (`000038`–`000039`) + sqlc/DTO/form Aset dasar.
2. **Penomoran kode aset baru** (`000040`) + `formatAssetTag`/derivasi MAX + advisory lock + test.
3. **History aset** (`000041`) + penulis history (create/edit/transfer/PIC) + tab Detail Aset.
4. **Master baru** (`000042`) + reference engine/sub-package + layar Referensi.
5. **Kolom kantor** (`000043`) + form Kantor + auto-saran klasifikasi.
6. **Pegawai + divisi per-kantor** (`000044`) + form Pegawai + validasi department-in-office.
7. **Login NIP** (`000045`) + handler auth + UI login.
8. **Batch registrasi** — executor + form jumlah.

## 12. Item terbuka

1. **DB NOT NULL `departments.office_id`**: migrasi lanjutan menyusul setelah user menyiapkan data
   departemen per-kantor.
2. **Kode kategori wajib**: format tag butuh `category.code` non-null untuk kategori yang dipakai
   (validasi registrasi). Tahun = 4 digit dari `purchase_date` (fallback tahun pembuatan).
