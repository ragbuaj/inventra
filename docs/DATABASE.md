# Inventra — Desain Database

| | |
|---|---|
| **Produk** | Inventra (Asset Management System) |
| **Database** | PostgreSQL 16 |
| **Akses kode** | sqlc (type-safe) · migrasi golang-migrate |
| **Sumber kebenaran** | Dokumen ini menjabarkan [PRD.md §6](PRD.md) menjadi skema konkret |
| **Tanggal** | 2026-06-23 |

> Dokumen ini menjelaskan **seluruh database**: konvensi, tipe enum, relasi (ERD), dan
> kamus data (data dictionary) tiap tabel. Dibuat sebelum implementasi fitur agar skema,
> penamaan, dan integritas konsisten lintas modul.

---

## 1. Konvensi

| Aspek | Keputusan |
|---|---|
| **Primary key** | `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` (pgcrypto, migrasi `000001_init`) |
| **Penamaan** | tabel `snake_case` jamak; kolom `snake_case`; FK `<entitas>_id` |
| **Timestamp** | `created_at timestamptz NOT NULL DEFAULT now()`; `updated_at timestamptz NOT NULL DEFAULT now()` di-update via trigger `set_updated_at()` |
| **Uang** | `numeric(18,2)` (mata uang default IDR) |
| **Rate/persen** | `numeric(5,4)` (mis. salvage rate 0.1000 = 10%) |
| **Periode depresiasi** | `date` pada hari-1 bulan (mis. `2026-06-01`) |
| **Enum** | tipe ENUM PostgreSQL untuk himpunan tetap & kecil (didaftar di §2) |
| **Master data aktif/nonaktif** | kolom `is_active boolean NOT NULL DEFAULT true` (FR-7.5) |
| **Scoping** | tabel beroperasi-aset menyimpan `office_id` (didenormalisasi) untuk filter subtree; "kepemilikan" via `employee_id`/`created_by_id` |
| **Penghapusan** | utamakan **soft-retire** (`assets.status = retired`); hard-delete hanya lewat approval (§3.6 PRD) dan tercatat di `audit_logs` |
| **FK on delete** | master data direferensi → `RESTRICT`; anak yang dimiliki penuh (mis. `asset_attachments`) → `CASCADE` |

### 1.1 Refinement terhadap PRD §6
- `data_scope_policies.module` memakai sentinel **`'*'`** (NOT NULL, default `'*'`) untuk baris default per-role — agar `UNIQUE(role, module)` dapat ditegakkan (NULL di Postgres dianggap distinct).
- `assets.office_id` ditambahkan (diturunkan dari `room → floor → office`) untuk mempercepat filter scoping.
- Kolom jejak `created_by_id` ditambahkan pada entitas operasional untuk mendukung scope `own` & audit.

---

## 2. Tipe Enum

```sql
CREATE TYPE user_role           AS ENUM ('superadmin','kepala_kanwil','kepala_unit','manager','staf');
CREATE TYPE user_status         AS ENUM ('active','inactive','suspended');
CREATE TYPE scope_level         AS ENUM ('global','office_subtree','office','own');
CREATE TYPE asset_status        AS ENUM ('available','assigned','under_maintenance','retired','lost');
CREATE TYPE depreciation_method AS ENUM ('straight_line','declining_balance');
CREATE TYPE assignment_status   AS ENUM ('active','returned');
CREATE TYPE maintenance_type    AS ENUM ('preventive','corrective');
CREATE TYPE maintenance_status  AS ENUM ('scheduled','in_progress','completed','cancelled');
CREATE TYPE request_type        AS ENUM ('asset_create','asset_delete','assignment','maintenance','valuation_exclusion');
CREATE TYPE request_status      AS ENUM ('pending','approved','rejected','cancelled');
CREATE TYPE attachment_kind     AS ENUM ('photo','document');
CREATE TYPE import_status       AS ENUM ('pending','processing','completed','failed');
CREATE TYPE audit_action        AS ENUM ('create','update','delete');
```

> `assignment.status = active` yang melewati `due_date` dianggap **overdue** (turunan, bukan kolom).

---

## 3. ERD (Relasi)

### 3.1 Identity & Otorisasi
```mermaid
erDiagram
  OFFICES   ||--o{ USERS              : "ditempatkan"
  EMPLOYEES ||--o| USERS              : "ditautkan"
  USERS     ||--o{ AUDIT_LOGS         : "aktor"
  USER_ROLE ||..o{ FIELD_PERMISSIONS  : "per-role"
  USER_ROLE ||..o{ DATA_SCOPE_POLICIES: "per-role"
```

### 3.2 Master Data & Struktur Kantor
```mermaid
erDiagram
  PROVINCES ||--o{ CITIES   : "punya"
  OFFICE_TYPES ||--o{ OFFICES : "klasifikasi"
  PROVINCES ||--o{ OFFICES  : "lokasi"
  CITIES    ||--o{ OFFICES  : "lokasi"
  OFFICES   ||--o{ OFFICES  : "parent (Pusat→Wilayah→Cabang→Outlet)"
  OFFICES   ||--o{ FLOORS   : "punya"
  FLOORS    ||--o{ ROOMS    : "punya"
  OFFICES   ||--o{ EMPLOYEES: "menempatkan"
  DEPARTMENTS ||--o{ EMPLOYEES : "bagian"
  POSITIONS   ||--o{ EMPLOYEES : "jabatan"
  BRANDS    ||--o{ MODELS   : "punya"
```

### 3.3 Aset & Operasional
```mermaid
erDiagram
  CATEGORIES ||--o{ ASSETS : "klasifikasi"
  BRANDS     ||--o{ ASSETS : "merek"
  MODELS     ||--o{ ASSETS : "model"
  ROOMS      ||--o{ ASSETS : "lokasi fisik"
  OFFICES    ||--o{ ASSETS : "scoping"
  UNITS      ||--o{ ASSETS : "satuan"
  VENDORS    ||--o{ ASSETS : "pemasok"
  EMPLOYEES  ||--o{ ASSETS : "pemegang"
  ASSETS ||--o{ ASSET_ATTACHMENTS   : "lampiran (MinIO)"
  ASSETS ||--o{ ASSIGNMENTS         : "penugasan"
  ASSETS ||--o{ MAINTENANCE_SCHEDULES : "jadwal"
  ASSETS ||--o{ MAINTENANCE_RECORDS  : "catatan"
  ASSETS ||--o{ DEPRECIATION_ENTRIES : "depresiasi"
  EMPLOYEES ||--o{ ASSIGNMENTS       : "custodian"
  MAINTENANCE_CATEGORIES ||--o{ MAINTENANCE_RECORDS : "kategori"
  PROBLEM_CATEGORIES     ||--o{ MAINTENANCE_RECORDS : "masalah"
```

### 3.4 Approval, Audit & Import
```mermaid
erDiagram
  USERS ||--o{ REQUESTS     : "pengaju/pemutus"
  OFFICES ||--o{ REQUESTS   : "routing"
  USERS ||--o{ IMPORT_JOBS  : "pembuat"
  USERS ||--o{ AUDIT_LOGS   : "aktor"
```

---

## 4. Kamus Data (Data Dictionary)

Notasi: **PK** primary key · **FK** foreign key · `?` nullable.

### 4.1 Identity & Otorisasi

#### `users`
| Kolom | Tipe | Null | Default | Keterangan |
|---|---|---|---|---|
| id | uuid | no | gen_random_uuid() | **PK** |
| employee_id | uuid? | yes | | **FK** employees — pegawai tertaut |
| office_id | uuid? | yes | | **FK** offices — kantor penempatan (NULL = global, untuk superadmin) |
| name | text | no | | nama tampil |
| email | citext | no | | **UNIQUE** |
| password_hash | text? | yes | | NULL bila login hanya via Google |
| google_id | text? | yes | | **UNIQUE** — subject Google OAuth |
| avatar_url | text? | yes | | |
| role | user_role | no | 'staf' | peran RBAC |
| status | user_status | no | 'active' | |
| created_at / updated_at | timestamptz | no | now() | |

Index: `UNIQUE(email)`, `UNIQUE(google_id)`, `idx_users_office_id`.

#### `field_permissions` — hak akses per-field per-role (§2.3 PRD, **semua entitas**)
| Kolom | Tipe | Null | Default | Keterangan |
|---|---|---|---|---|
| id | uuid | no | gen_random_uuid() | **PK** |
| entity | text | no | | nama entitas (mis. `assets`) |
| field | text | no | | nama field |
| role | user_role | no | | |
| can_view | boolean | no | true | |
| can_edit | boolean | no | false | |

Index: `UNIQUE(entity, field, role)`. Ditembolok di Redis; invalidasi saat berubah.

#### `data_scope_policies` — lingkup data per-role (+ override per-modul) (§2.2 PRD)
| Kolom | Tipe | Null | Default | Keterangan |
|---|---|---|---|---|
| id | uuid | no | gen_random_uuid() | **PK** |
| role | user_role | no | | |
| module | text | no | '*' | `'*'` = default semua modul; mis. `assets`, `requests` = override |
| scope_level | scope_level | no | | global / office_subtree / office / own |

Index: `UNIQUE(role, module)`.

### 4.2 Master Data — Referensi & Geografi

#### `provinces`
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| name | text | no | |
| code | text? | yes | **UNIQUE** (kode BPS opsional) |
| created_at / updated_at | timestamptz | no | |

#### `cities`
| id | uuid | no | **PK** |
| province_id | uuid | no | **FK** provinces |
| name | text | no | |
| code | text? | yes | **UNIQUE** |
| ts | timestamptz | no | created/updated |

#### `office_types`
| id | uuid PK · name text UNIQUE (Pusat/Wilayah/Cabang/Outlet) · is_active bool · ts |

#### `departments`
| id | uuid PK · name text · code text? UNIQUE · is_active bool · ts |

#### `positions`
| id | uuid PK · name text · is_active bool · ts |

#### `vendors`
| id uuid PK · name text · contact_name text? · phone text? · email text? · address text? · is_active bool · ts |

#### `brands`
| id uuid PK · name text UNIQUE · is_active bool · ts |

#### `models`
| id uuid PK · brand_id uuid **FK** brands · name text · is_active bool · ts · **UNIQUE(brand_id, name)** |

#### `categories` — kategori aset
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| name | text | no | |
| code | text? | yes | **UNIQUE** |
| parent_id | uuid? | yes | **FK** categories (hierarki) |
| default_depreciation_method | depreciation_method? | yes | nilai default untuk aset |
| default_useful_life_months | int? | yes | |
| default_salvage_rate | numeric(5,4)? | yes | |
| is_active | boolean | no | |
| ts | timestamptz | no | |

#### `maintenance_categories`
| id uuid PK · name text UNIQUE · is_active bool · ts | (mis. Servis Rutin, Kalibrasi) |

#### `problem_categories`
| id uuid PK · name text UNIQUE · is_active bool · ts | (mis. Hardware, Listrik, Fisik) |

#### `units` — satuan
| id uuid PK · name text · symbol text? · is_active bool · ts | (mis. Unit/Pcs/Set) |

### 4.3 Master Data — Struktur Kantor & Orang

#### `offices` — hierarki Pusat → Wilayah → Cabang → Outlet
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| parent_id | uuid? | yes | **FK** offices (self) — NULL = akar (Pusat) |
| office_type_id | uuid | no | **FK** office_types |
| province_id | uuid? | yes | **FK** provinces |
| city_id | uuid? | yes | **FK** cities |
| name | text | no | |
| code | text | no | **UNIQUE** |
| address | text? | yes | |
| is_active | boolean | no | |
| ts | timestamptz | no | |

Index: `idx_offices_parent_id`, `UNIQUE(code)`. Lihat §5 untuk komputasi subtree.

#### `floors`
| id uuid PK · office_id uuid **FK** offices · name text · level int? · ts · **UNIQUE(office_id, name)** |

#### `rooms`
| id uuid PK · floor_id uuid **FK** floors · name text · code text? · ts · **UNIQUE(floor_id, name)** |

#### `employees` — custodian aset (terpisah dari `users`)
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| code | text | no | **UNIQUE** (NIP/kode pegawai) |
| name | text | no | |
| email | text? | yes | |
| department_id | uuid? | yes | **FK** departments |
| position_id | uuid? | yes | **FK** positions |
| office_id | uuid | no | **FK** offices — penempatan |
| status | user_status | no | active/inactive |
| ts | timestamptz | no | |

Index: `UNIQUE(code)`, `idx_employees_office_id`.

### 4.4 Aset & Operasional

#### `assets`
| Kolom | Tipe | Null | Default | Keterangan |
|---|---|---|---|---|
| id | uuid | no | gen_random_uuid() | **PK** |
| asset_tag | text | no | | **UNIQUE** — sumber barcode (Code128) |
| name | text | no | | |
| category_id | uuid | no | | **FK** categories |
| brand_id | uuid? | yes | | **FK** brands |
| model_id | uuid? | yes | | **FK** models |
| room_id | uuid | no | | **FK** rooms — lokasi fisik |
| office_id | uuid | no | | **FK** offices — diturunkan dari room, untuk scoping |
| unit_id | uuid? | yes | | **FK** units |
| status | asset_status | no | 'available' | state machine PRD §5 |
| serial_number | text? | yes | | |
| purchase_date | date? | yes | | |
| purchase_cost | numeric(18,2)? | yes | | harga perolehan |
| vendor_id | uuid? | yes | | **FK** vendors |
| warranty_expiry | date? | yes | | |
| specifications | jsonb | no | '{}' | atribut fleksibel |
| depreciation_method | depreciation_method? | yes | | override default kategori |
| useful_life_months | int? | yes | | |
| salvage_value | numeric(18,2)? | yes | | nilai sisa |
| current_holder_employee_id | uuid? | yes | | **FK** employees — pemegang aktif |
| excluded_from_valuation | boolean | no | false | hasil approval (§3.6) |
| valuation_exclusion_reason | text? | yes | | |
| created_by_id | uuid? | yes | | **FK** users |
| notes | text? | yes | | |
| ts | timestamptz | no | now() | created/updated |

Index: `UNIQUE(asset_tag)`, `idx_assets_office_id`, `idx_assets_status`, `idx_assets_category_id`, `idx_assets_holder`.

#### `asset_attachments` — file di MinIO
| id uuid PK · asset_id uuid **FK** assets `ON DELETE CASCADE` · kind attachment_kind · object_key text · thumbnail_key text? · original_filename text · size_bytes bigint · mime_type text · created_by_id uuid? **FK** users · created_at timestamptz |

Index: `idx_attachments_asset_id`.

#### `assignments` — check-out / check-in
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| asset_id | uuid | no | **FK** assets |
| employee_id | uuid | no | **FK** employees (custodian) |
| assigned_by_id | uuid | no | **FK** users |
| checkout_date | timestamptz | no | |
| due_date | date? | yes | jatuh tempo |
| checkin_date | timestamptz? | yes | NULL = masih dipegang |
| condition_out | text? | yes | kondisi keluar |
| condition_in | text? | yes | kondisi masuk |
| status | assignment_status | no | active/returned |
| notes | text? | yes | |
| ts | timestamptz | no | |

Index: `idx_assignments_asset_id`, `idx_assignments_employee_id`, `idx_assignments_status`. Aturan: hanya **satu** assignment `active` per aset (partial unique index `WHERE status='active'`).

#### `maintenance_schedules`
| id uuid PK · asset_id uuid **FK** assets · maintenance_category_id uuid? **FK** · interval_months int · last_done_date date? · next_due_date date · is_active bool · ts |

Index: `idx_msched_next_due` (reminder).

#### `maintenance_records`
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| asset_id | uuid | no | **FK** assets |
| maintenance_category_id | uuid? | yes | **FK** maintenance_categories |
| problem_category_id | uuid? | yes | **FK** problem_categories (laporan kerusakan) |
| type | maintenance_type | no | preventive/corrective |
| status | maintenance_status | no | default 'scheduled' |
| scheduled_date | date? | yes | |
| completed_date | date? | yes | |
| cost | numeric(18,2)? | yes | |
| vendor_id | uuid? | yes | **FK** vendors |
| performed_by | text? | yes | teknisi |
| description | text | no | |
| reported_by_id | uuid? | yes | **FK** users (pelapor) |
| ts | timestamptz | no | |

Index: `idx_mrec_asset_status`.

#### `depreciation_entries` (read model)
| id uuid PK · asset_id uuid **FK** assets · period date · opening_value numeric(18,2) · depreciation_amount numeric(18,2) · closing_value numeric(18,2) · method depreciation_method · created_at · **UNIQUE(asset_id, period)** |

Index: `idx_depr_asset_period`.

### 4.5 Approval, Audit & Import

#### `requests` — maker-checker generik (§3.6 PRD)
| Kolom | Tipe | Null | Keterangan |
|---|---|---|---|
| id | uuid | no | **PK** |
| type | request_type | no | asset_create / asset_delete / assignment / maintenance / valuation_exclusion |
| office_id | uuid? | yes | **FK** offices — routing approver berjenjang |
| target_entity | text? | yes | entitas terkait (mis. `assets`) |
| target_id | uuid? | yes | ID objek eksisting (untuk delete/exclusion) |
| payload | jsonb | no | data usulan |
| reason | text? | yes | |
| status | request_status | no | default 'pending' |
| requested_by_id | uuid | no | **FK** users (maker) |
| decided_by_id | uuid? | yes | **FK** users (checker) |
| decision_note | text? | yes | |
| decided_at | timestamptz? | yes | |
| ts | timestamptz | no | created/updated |

Index: `idx_requests_status_type`, `idx_requests_office_id`, `idx_requests_requester`. Aturan: `requested_by_id <> decided_by_id` (segregation of duty, §FR-6.4).

#### `audit_logs` — jejak seluruh tabel (§5.7 PRD)
| id uuid PK · actor_id uuid? **FK** users · entity_type text · entity_id uuid · action audit_action · changes jsonb (diff before/after) · ip text? · created_at timestamptz |

Index: `idx_audit_entity (entity_type, entity_id)`, `idx_audit_actor`, `idx_audit_created_at`. Diisi terpusat (decorator service/repository), bukan per-handler.

#### `import_jobs` — import massal CSV/XLSX (FR-2.11 / FR-7.5b)
| id uuid PK · target text (asset/employee/office/…) · format text (csv/xlsx) · filename text · object_key text? (sumber di MinIO) · status import_status · total_rows int · success_rows int · failed_rows int · error_report_key text? (laporan error di MinIO) · created_by_id uuid **FK** users · created_at · finished_at timestamptz? |

Index: `idx_import_created_by`.

---

## 5. Scoping Hierarki Kantor

Lingkup `office_subtree` membutuhkan daftar **descendant** dari `office_id` user. Pendekatan:

```sql
WITH RECURSIVE subtree AS (
  SELECT id FROM offices WHERE id = $1
  UNION ALL
  SELECT o.id FROM offices o JOIN subtree s ON o.parent_id = s.id
)
SELECT id FROM subtree;
```

- Hasil (`descendant_ids`) **ditembolok di Redis** per `office_id` (mahal dihitung); invalidasi saat hierarki kantor berubah.
- Penegakan filter di service layer sesuai `scope_level` efektif (`data_scope_policies`):
  `global` → tanpa filter · `office_subtree` → `office_id IN (descendant_ids)` · `office` → `office_id = user.office_id` · `own` → `created_by_id/holder = user`.
- Alternatif performa (opsional, bila pohon sangat besar): kolom **materialized path** atau ekstensi **`ltree`**. Default: recursive CTE + cache.

---

## 6. Pemetaan ke Migrasi & Roadmap

Tiap fase roadmap (PRD §10) menambah migrasi `golang-migrate` di `backend/db/migrations`:

| Migrasi | Fase | Objek |
|---|---|---|
| `000001_init` | 1 | extension `pgcrypto` (sudah ada) |
| `0000xx_enums` | 2 | semua tipe enum (§2) + fungsi/trigger `set_updated_at` |
| `0000xx_identity` | 2 | `users`, `field_permissions`, `data_scope_policies` |
| `0000xx_masterdata` | 3 | provinces, cities, office_types, departments, positions, vendors, brands, models, categories, maintenance_categories, problem_categories, units |
| `0000xx_offices` | 3 | offices, floors, rooms, employees |
| `0000xx_assets` | 4 | assets, asset_attachments |
| `0000xx_approval` | 5 | requests |
| `0000xx_assignment` | 6 | assignments |
| `0000xx_maintenance` | 7 | maintenance_schedules, maintenance_records |
| `0000xx_depreciation` | 8 | depreciation_entries |
| `0000xx_audit_import` | 2/4 | audit_logs (awal), import_jobs |

> `audit_logs` dibuat lebih awal (fase 2) karena bersifat cross-cutting.

---

## 7. Catatan & Keputusan Terbuka

- **DB-Q1** — `email` memakai tipe `citext` (case-insensitive). Perlu extension `citext`; alternatif: simpan lowercase + `text`. (sementara: `citext`).
- **DB-Q2** — Hard-delete aset: default **dilarang bila punya riwayat** (assignments/maintenance) → arahkan ke `retired`. Hanya aset tanpa riwayat yang boleh dihapus (via approval). *Konfirmasi bila ingin cascade penuh.*
- **DB-Q3** — Retensi `audit_logs` & `import_jobs` (volume besar): perlu kebijakan arsip/partisi? (sementara: tanpa partisi; ditinjau saat volume tumbuh).
