-- =============================================================================
-- Inventra — DEMO SEED (data master + aset + user login)  •  DEV ONLY
-- =============================================================================
-- Skala "bank BTN": 1 Kantor Pusat + 6 Kantor Wilayah + kantor cabang/KCP/Kas di
-- banyak kota nyata (±42 kantor). SETIAP kantor punya ≥20 user login dan ≥1.500
-- aset tetap (±63.000 aset total) dengan penyusutan komersial terisi, plus ±1.000
-- pegawai bernama Indonesia realistis.
--
-- Jalankan SETELAH semua migrasi `up`:
--   psql "postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable" \
--        -f backend/db/seed/seed_demo.sql
--
-- Idempoten: menghapus data demo/transaksional lama lalu mengisi ulang. User
-- bootstrap (admin dgn employee_id & office_id NULL, mis. admin@inventra.local
-- dari `createadmin`) TIDAK disentuh.
--
-- Login demo: semua user password = "Inventra123!"  (domain @demo.inventra.local)
--   superadmin@demo.inventra.local     (Superadmin, di Kantor Pusat)
--   kanwil.<kode>@demo.inventra.local  (Kepala Kanwil, per Wilayah)
--   kepala.<kode>@demo.inventra.local  (Kepala Unit, per Cabang/KCP/Kas)
--   sisanya (manager & staf) beremail <nama>.<kode><n>@demo.inventra.local
-- =============================================================================

BEGIN;
SET LOCAL synchronous_commit = off;          -- percepat insert massal (dev).

CREATE EXTENSION IF NOT EXISTS pgcrypto;      -- crypt()/gen_salt('bf') → bcrypt (kompatibel Go).

-- ─────────────────────────────────────────────────────────────────────────────
-- 0) RESET — bersihkan data lama (urut anak→induk sesuai FK) agar re-runnable.
--    CATATAN (DEV): mengosongkan SELURUH data transaksional karena me-referensi
--    assets/employees/offices yang diisi ulang. User bootstrap (employee_id &
--    office_id NULL) tidak dihapus.
-- ─────────────────────────────────────────────────────────────────────────────
DELETE FROM import.import_rows;
DELETE FROM import.import_jobs;
DELETE FROM approval.request_approvals;
DELETE FROM asset.asset_attachments;
DELETE FROM asset.asset_documents;
DELETE FROM assignment.assignments;
DELETE FROM transfer.asset_transfers;
DELETE FROM disposal.disposals;
DELETE FROM maintenance.maintenance_records;
DELETE FROM maintenance.maintenance_schedules;
DELETE FROM depreciation.depreciation_entries;
DELETE FROM depreciation.depreciation_periods;
DELETE FROM stockopname.stock_opname_items;
DELETE FROM stockopname.stock_opname_sessions;
DELETE FROM approval.requests;
DELETE FROM audit.audit_logs;
DELETE FROM asset.asset_tag_counters;
DELETE FROM asset.assets;
DELETE FROM identity.users WHERE employee_id IS NOT NULL OR office_id IS NOT NULL;
DELETE FROM masterdata.employees;
DELETE FROM masterdata.rooms;
DELETE FROM masterdata.floors;
DELETE FROM masterdata.offices;
DELETE FROM masterdata.models;
DELETE FROM masterdata.brands;
DELETE FROM masterdata.categories;
DELETE FROM masterdata.units;
DELETE FROM masterdata.vendors;
DELETE FROM masterdata.positions;
DELETE FROM masterdata.departments;
DELETE FROM masterdata.office_types;
DELETE FROM masterdata.cities;
DELETE FROM masterdata.provinces;

-- ─────────────────────────────────────────────────────────────────────────────
-- 1) GEOGRAFI — provinsi & kota (dengan koordinat untuk peta kantor).
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO masterdata.provinces (name, code) VALUES
  ('DKI Jakarta','31'), ('Jawa Barat','32'), ('Banten','36'), ('Jawa Tengah','33'),
  ('DI Yogyakarta','34'), ('Jawa Timur','35'), ('Bali','51'), ('Sumatera Utara','12'),
  ('Sumatera Barat','13'), ('Sumatera Selatan','16'), ('Riau','14'), ('Kepulauan Riau','21'),
  ('Sulawesi Selatan','73'), ('Sulawesi Utara','71'), ('Kalimantan Timur','64'), ('Kalimantan Selatan','63');

CREATE TEMP TABLE _geo (prov text, city text, code text, lat double precision, lng double precision) ON COMMIT DROP;
INSERT INTO _geo (prov, city, code, lat, lng) VALUES
  ('DKI Jakarta',    'Jakarta Pusat',   '3171', -6.18150, 106.82720),
  ('DKI Jakarta',    'Jakarta Selatan', '3174', -6.26150, 106.81060),
  ('DKI Jakarta',    'Jakarta Barat',   '3173', -6.16830, 106.75880),
  ('DKI Jakarta',    'Jakarta Utara',   '3172', -6.12140, 106.77410),
  ('Jawa Barat',     'Bekasi',          '3275', -6.23830, 106.97560),
  ('Jawa Barat',     'Depok',           '3276', -6.40250, 106.79420),
  ('Banten',         'Tangerang',       '3671', -6.17830, 106.63190),
  ('Jawa Barat',     'Bogor',           '3271', -6.59710, 106.80600),
  ('Jawa Barat',     'Bandung',         '3273', -6.91470, 107.60980),
  ('Jawa Barat',     'Cirebon',         '3274', -6.73200, 108.55230),
  ('Jawa Barat',     'Tasikmalaya',     '3278', -7.35060, 108.21720),
  ('Jawa Barat',     'Sukabumi',        '3272', -6.92770, 106.93000),
  ('Jawa Tengah',    'Semarang',        '3374', -6.99320, 110.42290),
  ('Jawa Tengah',    'Surakarta',       '3372', -7.57550, 110.82430),
  ('DI Yogyakarta',  'Yogyakarta',      '3471', -7.79560, 110.36950),
  ('Jawa Tengah',    'Purwokerto',      '3302', -7.42180, 109.23460),
  ('Jawa Tengah',    'Tegal',           '3376', -6.86940, 109.14020),
  ('Jawa Timur',     'Surabaya',        '3578', -7.25750, 112.75210),
  ('Jawa Timur',     'Malang',          '3573', -7.96660, 112.63260),
  ('Jawa Timur',     'Sidoarjo',        '3515', -7.44780, 112.71830),
  ('Jawa Timur',     'Jember',          '3509', -8.17270, 113.70020),
  ('Bali',           'Denpasar',        '5171', -8.67050, 115.21260),
  ('Sumatera Utara', 'Medan',           '1275',  3.59520,  98.67220),
  ('Sumatera Barat', 'Padang',          '1371', -0.94710, 100.41720),
  ('Sumatera Selatan','Palembang',      '1671', -2.97610, 104.77540),
  ('Riau',           'Pekanbaru',       '1471',  0.50710, 101.44780),
  ('Kepulauan Riau', 'Batam',           '2171',  1.04560, 104.03050),
  ('Sulawesi Selatan','Makassar',       '7371', -5.14770, 119.43270),
  ('Sulawesi Utara', 'Manado',          '7171',  1.47480, 124.84210),
  ('Kalimantan Timur','Balikpapan',     '6471', -1.23790, 116.85290),
  ('Kalimantan Selatan','Banjarmasin',  '6371', -3.31860, 114.59440);

INSERT INTO masterdata.cities (province_id, name, code)
SELECT p.id, g.city, g.code FROM _geo g JOIN masterdata.provinces p ON p.name = g.prov;

-- ─────────────────────────────────────────────────────────────────────────────
-- 2) ORGANISASI — tipe kantor, departemen, jabatan, vendor.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO masterdata.office_types (name, tier) VALUES
  ('Kantor Pusat','pusat'), ('Kantor Wilayah','wilayah'),
  ('Kantor Cabang','office'), ('Kantor Cabang Pembantu','office'), ('Kantor Kas','office');

INSERT INTO masterdata.departments (name, code) VALUES
  ('Umum & Logistik','GA'), ('Teknologi Informasi','IT'), ('Operasional','OPS'),
  ('Layanan Nasabah','CS'), ('Kredit & Pembiayaan','KRD'), ('Sumber Daya Manusia','HRD'),
  ('Keuangan & Akuntansi','FIN'), ('Manajemen Risiko','RISK');

INSERT INTO masterdata.positions (name) VALUES
  ('Kepala Kantor Wilayah'), ('Kepala Cabang'), ('Manajer Operasional'),
  ('Asset Management Officer'), ('Staf Umum & Logistik'), ('Staf Teknologi Informasi'),
  ('Teller'), ('Customer Service'), ('Analis Kredit'), ('Petugas Keamanan');

INSERT INTO masterdata.vendors (name, contact_name, phone, email, address) VALUES
  ('PT Astra Graphia Information Technology','Rina Wijaya','02150881234','sales@ag-it.co.id','Jl. Kramat Raya No.43, Jakarta Pusat'),
  ('PT Metrodata Electronics Tbk','Andi Pratama','02152896000','corporate@metrodata.co.id','APL Tower, Jl. Letjen S. Parman, Jakarta Barat'),
  ('PT Datascrip','Dewi Kusuma','02165908800','info@datascrip.co.id','Kawasan Niaga Selatan, Bandara Soekarno-Hatta'),
  ('PT Multipolar Technology Tbk','Hendra Halim','02125531000','sales@multipolar.com','Jl. Boulevard Jenderal Sudirman, Tangerang'),
  ('PT Mastersystem Infotama','Yuni Permana','02129333000','contact@mastersystem.co.id','Wisma 77, Jl. Letjen S. Parman, Jakarta Barat'),
  ('PT Berca Hardayaperkasa','Eko Nugroho','02157901234','sales@berca.co.id','Jl. Abdul Muis No.62, Jakarta Pusat'),
  ('PT Dinamika Wahana Sejahtera','Sri Lestari','0315678123','cs@dinamika-ws.co.id','Jl. Raya Darmo No.101, Surabaya'),
  ('PT Sarana Solusindo Informatika','Fajar Setiawan','0617801234','sales@saranasolusindo.id','Jl. Gatot Subroto No.28, Medan'),
  ('CV Sinar Jaya Elektronik','Ratna Sari','0224201234','sinarjaya.elk@gmail.com','Jl. Pemuda No.15, Semarang'),
  ('PT Cakra Radha Mustika','Doni Saputra','02178901234','procurement@crm.co.id','Jl. TB Simatupang No.5, Jakarta Selatan');

-- ─────────────────────────────────────────────────────────────────────────────
-- 3) REFERENSI ASET — unit, brand, model, kategori (dual-basis PSAK + PMK 72/2023).
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO masterdata.units (name, symbol) VALUES
  ('Unit','unit'), ('Buah','pcs'), ('Set','set'), ('Meter Persegi','m2'), ('Lisensi','lisensi');

INSERT INTO masterdata.brands (name) VALUES
  ('Dell'), ('HP'), ('Lenovo'), ('Cisco'), ('Diebold Nixdorf'), ('Wincor Nixdorf'),
  ('Toyota'), ('Honda'), ('Daikin'), ('Panasonic'), ('Epson'), ('APC'),
  ('Indachi'), ('Modera'), ('Brother'), ('Chairman');

INSERT INTO masterdata.models (brand_id, name)
SELECT b.id, v.model
FROM (VALUES
  ('Dell','Latitude 5440'), ('Dell','OptiPlex 7010'), ('Dell','PowerEdge R760'), ('Dell','Monitor P2422H'),
  ('HP','EliteBook 840 G10'), ('HP','ProDesk 400 G9'), ('HP','LaserJet Pro M404dn'),
  ('Lenovo','ThinkPad E14'), ('Lenovo','ThinkCentre M70q'),
  ('Cisco','Catalyst 2960-X'), ('Cisco','ISR 4331'),
  ('Diebold Nixdorf','CINEO C4060'), ('Diebold Nixdorf','DN Series 200 CRM'), ('Wincor Nixdorf','ProCash 280'),
  ('Toyota','Avanza 1.5 G'), ('Toyota','Kijang Innova 2.4 V'), ('Toyota','Fortuner 2.4 VRZ'),
  ('Honda','Vario 160'), ('Honda','PCX 160'),
  ('Daikin','FTKC50 Split 2PK'), ('Panasonic','CS-XN9 Split 1PK'), ('Panasonic','KX-TS820 Telepon'),
  ('Epson','L3210 EcoTank'), ('Epson','EB-X500 Proyektor'), ('APC','Smart-UPS 1500VA'),
  ('Indachi','D-8036 Kursi Kerja'), ('Modera','Workstation 120'), ('Brother','B-204 Lemari Arsip'),
  ('Chairman','SV-201 Sofa')
) AS v(brand, model)
JOIN masterdata.brands b ON b.name = v.brand;

INSERT INTO masterdata.categories
  (name, code, default_depreciation_method, default_useful_life_months, default_salvage_rate,
   asset_class, default_fiscal_group, default_fiscal_life_months, gl_account_code, capitalization_threshold)
VALUES
  ('Tanah',                          'TNH', NULL,            NULL, NULL,   'tangible',   'non_susut',          NULL, '160101', 1000000),
  ('Bangunan & Gedung',              'BGN', 'straight_line', 240,  0.0000, 'tangible',   'bangunan_permanen',  240,  '160201', 1000000),
  ('Kendaraan Roda Empat',           'KR4', 'straight_line', 96,   0.1000, 'tangible',   'kelompok_2',         96,   '160301', 1000000),
  ('Kendaraan Roda Dua',             'KR2', 'straight_line', 96,   0.1000, 'tangible',   'kelompok_2',         96,   '160302', 1000000),
  ('Perangkat Komputer',             'KOM', 'straight_line', 48,   0.1000, 'tangible',   'kelompok_1',         48,   '160401', 1000000),
  ('Peralatan Jaringan',             'NET', 'straight_line', 48,   0.1000, 'tangible',   'kelompok_1',         48,   '160402', 1000000),
  ('Mesin ATM & CRM',                'ATM', 'straight_line', 96,   0.1000, 'tangible',   'kelompok_2',         96,   '160403', 1000000),
  ('Peralatan Elektronik Kantor',    'ELK', 'straight_line', 48,   0.1000, 'tangible',   'kelompok_1',         48,   '160404', 1000000),
  ('Furnitur & Perlengkapan Kantor', 'FRN', 'straight_line', 48,   0.1000, 'tangible',   'kelompok_1',         48,   '160405', 1000000),
  ('Perangkat Lunak & Lisensi',      'SWL', 'straight_line', 48,   0.0000, 'intangible', NULL,                 NULL, '170101', 1000000);

-- ─────────────────────────────────────────────────────────────────────────────
-- 4) KANTOR — hierarki Pusat → Wilayah → Cabang → KCP/Kas (nama & kota nyata BTN).
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TEMP TABLE _office (code text, name text, type_name text, parent_code text, city text, lvl int) ON COMMIT DROP;
INSERT INTO _office (code, name, type_name, parent_code, city, lvl) VALUES
  -- Pusat
  ('PST','Kantor Pusat','Kantor Pusat', NULL,'Jakarta Pusat',0),
  -- Wilayah
  ('KW1','Kantor Wilayah I Jakarta','Kantor Wilayah','PST','Jakarta Selatan',1),
  ('KW2','Kantor Wilayah II Bandung','Kantor Wilayah','PST','Bandung',1),
  ('KW3','Kantor Wilayah III Semarang','Kantor Wilayah','PST','Semarang',1),
  ('KW4','Kantor Wilayah IV Surabaya','Kantor Wilayah','PST','Surabaya',1),
  ('KW5','Kantor Wilayah V Medan','Kantor Wilayah','PST','Medan',1),
  ('KW6','Kantor Wilayah VI Makassar','Kantor Wilayah','PST','Makassar',1),
  -- Cabang — Kanwil I (Jabodetabek)
  ('JKT01','Kantor Cabang Jakarta Harmoni','Kantor Cabang','KW1','Jakarta Pusat',2),
  ('JKT02','Kantor Cabang Jakarta Kuningan','Kantor Cabang','KW1','Jakarta Selatan',2),
  ('JKT03','Kantor Cabang Jakarta Kebon Jeruk','Kantor Cabang','KW1','Jakarta Barat',2),
  ('BKS01','Kantor Cabang Bekasi','Kantor Cabang','KW1','Bekasi',2),
  ('DPK01','Kantor Cabang Depok','Kantor Cabang','KW1','Depok',2),
  ('TGR01','Kantor Cabang Tangerang BSD','Kantor Cabang','KW1','Tangerang',2),
  ('BGR01','Kantor Cabang Bogor','Kantor Cabang','KW1','Bogor',2),
  -- Cabang — Kanwil II (Jawa Barat)
  ('BDG01','Kantor Cabang Bandung Asia Afrika','Kantor Cabang','KW2','Bandung',2),
  ('CRB01','Kantor Cabang Cirebon','Kantor Cabang','KW2','Cirebon',2),
  ('TSM01','Kantor Cabang Tasikmalaya','Kantor Cabang','KW2','Tasikmalaya',2),
  ('SKB01','Kantor Cabang Sukabumi','Kantor Cabang','KW2','Sukabumi',2),
  -- Cabang — Kanwil III (Jateng/DIY)
  ('SMG01','Kantor Cabang Semarang Pemuda','Kantor Cabang','KW3','Semarang',2),
  ('SLO01','Kantor Cabang Solo Slamet Riyadi','Kantor Cabang','KW3','Surakarta',2),
  ('YOG01','Kantor Cabang Yogyakarta Malioboro','Kantor Cabang','KW3','Yogyakarta',2),
  ('PWT01','Kantor Cabang Purwokerto','Kantor Cabang','KW3','Purwokerto',2),
  ('TGL01','Kantor Cabang Tegal','Kantor Cabang','KW3','Tegal',2),
  -- Cabang — Kanwil IV (Jatim/Bali)
  ('SBY01','Kantor Cabang Surabaya Darmo','Kantor Cabang','KW4','Surabaya',2),
  ('MLG01','Kantor Cabang Malang Kayutangan','Kantor Cabang','KW4','Malang',2),
  ('SDA01','Kantor Cabang Sidoarjo','Kantor Cabang','KW4','Sidoarjo',2),
  ('JBR01','Kantor Cabang Jember','Kantor Cabang','KW4','Jember',2),
  ('DPS01','Kantor Cabang Denpasar Renon','Kantor Cabang','KW4','Denpasar',2),
  -- Cabang — Kanwil V (Sumatera)
  ('MDN01','Kantor Cabang Medan Balai Kota','Kantor Cabang','KW5','Medan',2),
  ('MDN02','Kantor Cabang Medan Iskandar Muda','Kantor Cabang','KW5','Medan',2),
  ('PDG01','Kantor Cabang Padang','Kantor Cabang','KW5','Padang',2),
  ('PLB01','Kantor Cabang Palembang','Kantor Cabang','KW5','Palembang',2),
  ('PKU01','Kantor Cabang Pekanbaru','Kantor Cabang','KW5','Pekanbaru',2),
  ('BTM01','Kantor Cabang Batam','Kantor Cabang','KW5','Batam',2),
  -- Cabang — Kanwil VI (Timur)
  ('MKS01','Kantor Cabang Makassar Panakkukang','Kantor Cabang','KW6','Makassar',2),
  ('MND01','Kantor Cabang Manado','Kantor Cabang','KW6','Manado',2),
  ('BPP01','Kantor Cabang Balikpapan','Kantor Cabang','KW6','Balikpapan',2),
  ('BJM01','Kantor Cabang Banjarmasin','Kantor Cabang','KW6','Banjarmasin',2),
  -- KCP & Kantor Kas
  ('JKT04','Kantor Cabang Pembantu Kelapa Gading','Kantor Cabang Pembantu','JKT02','Jakarta Utara',3),
  ('SBY02','Kantor Cabang Pembantu Rungkut','Kantor Cabang Pembantu','SBY01','Surabaya',3),
  ('BDG02','Kantor Kas Dago','Kantor Kas','BDG01','Bandung',3),
  ('YOG02','Kantor Kas UGM','Kantor Kas','YOG01','Yogyakarta',3);

-- Insert bertahap per level agar parent selalu sudah ada.
INSERT INTO masterdata.offices (parent_id, office_type_id, province_id, city_id, name, code, cost_center_code, address, latitude, longitude)
SELECT (SELECT id FROM masterdata.offices WHERE code = t.parent_code),
       ot.id, c.province_id, c.id, t.name, t.code, 'CC-' || t.code,
       'Jl. Utama No.1, ' || t.city, g.lat, g.lng
FROM _office t
JOIN masterdata.office_types ot ON ot.name = t.type_name
JOIN masterdata.cities c ON c.name = t.city
JOIN _geo g ON g.city = t.city
WHERE t.lvl = 0;

INSERT INTO masterdata.offices (parent_id, office_type_id, province_id, city_id, name, code, cost_center_code, address, latitude, longitude)
SELECT (SELECT id FROM masterdata.offices WHERE code = t.parent_code),
       ot.id, c.province_id, c.id, t.name, t.code, 'CC-' || t.code,
       'Jl. Utama No.1, ' || t.city, g.lat, g.lng
FROM _office t
JOIN masterdata.office_types ot ON ot.name = t.type_name
JOIN masterdata.cities c ON c.name = t.city
JOIN _geo g ON g.city = t.city
WHERE t.lvl = 1;

INSERT INTO masterdata.offices (parent_id, office_type_id, province_id, city_id, name, code, cost_center_code, address, latitude, longitude)
SELECT (SELECT id FROM masterdata.offices WHERE code = t.parent_code),
       ot.id, c.province_id, c.id, t.name, t.code, 'CC-' || t.code,
       'Jl. Utama No.1, ' || t.city, g.lat, g.lng
FROM _office t
JOIN masterdata.office_types ot ON ot.name = t.type_name
JOIN masterdata.cities c ON c.name = t.city
JOIN _geo g ON g.city = t.city
WHERE t.lvl = 2;

INSERT INTO masterdata.offices (parent_id, office_type_id, province_id, city_id, name, code, cost_center_code, address, latitude, longitude)
SELECT (SELECT id FROM masterdata.offices WHERE code = t.parent_code),
       ot.id, c.province_id, c.id, t.name, t.code, 'CC-' || t.code,
       'Jl. Utama No.1, ' || t.city, g.lat, g.lng
FROM _office t
JOIN masterdata.office_types ot ON ot.name = t.type_name
JOIN masterdata.cities c ON c.name = t.city
JOIN _geo g ON g.city = t.city
WHERE t.lvl = 3;

-- ─────────────────────────────────────────────────────────────────────────────
-- 5) LANTAI & RUANGAN — tiap kantor 3 lantai × 5 ruangan.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO masterdata.floors (office_id, name, level)
SELECT o.id, 'Lantai ' || lv, lv
FROM masterdata.offices o CROSS JOIN generate_series(1, 3) AS lv
WHERE o.deleted_at IS NULL;

INSERT INTO masterdata.rooms (floor_id, name, code)
SELECT f.id, r.rname, 'R' || f.level || '-' || r.rno
FROM masterdata.floors f
CROSS JOIN (VALUES
  (1,'Ruang Server & Jaringan'), (2,'Ruang Operasional'), (3,'Ruang Layanan Nasabah'),
  (4,'Ruang Kerja Umum'), (5,'Gudang Aset')
) AS r(rno, rname)
WHERE f.deleted_at IS NULL;

-- ─────────────────────────────────────────────────────────────────────────────
-- 6) PEGAWAI — 24 per kantor (≥20 utk penuhi kuota user), nama Indonesia realistis.
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO masterdata.employees (code, name, email, department_id, position_id, office_id, status, phone)
SELECT
  'BTN-' || lpad(x.seq::text, 5, '0'),
  x.fname || ' ' || x.lname,
  lower(x.fname) || '.' || lower(x.lname) || x.seq || '@btn.co.id',
  x.dep_ids[(abs(hashtext(x.sd || 'dp')) % array_length(x.dep_ids, 1)) + 1],
  x.pos_ids[(abs(hashtext(x.sd || 'ps')) % array_length(x.pos_ids, 1)) + 1],
  x.office_id, 'active'::shared.user_status,
  '08' || lpad((abs(hashtext(x.sd || 'ph')) % 1000000000)::text, 10, '0')
FROM (
  SELECT
    o.id AS office_id,
    row_number() OVER (ORDER BY o.code, k) AS seq,
    o.code || '-' || k AS sd,
    (ARRAY['Budi','Siti','Agus','Dewi','Andi','Rina','Joko','Sri','Bambang','Ani',
           'Hendra','Wati','Rudi','Yuni','Eko','Nur','Fajar','Indah','Doni','Ratna',
           'Aditya','Putri','Rizky','Maya','Arif','Wahyu','Fitri','Dedi','Iwan','Novi',
           'Gunawan','Ayu','Hadi','Teguh','Kartika','Slamet','Melati','Reza','Lina','Bayu'
     ])[(abs(hashtext(o.code || '-' || k || 'fn')) % 40) + 1] AS fname,
    (ARRAY['Santoso','Wijaya','Nugroho','Pratama','Kusuma','Hidayat','Saputra','Lestari','Wibowo','Suryadi',
           'Halim','Permana','Utomo','Setiawan','Maulana','Firmansyah','Anggraini','Purnama','Ramadhan','Susanto',
           'Handoko','Prasetyo','Wardana','Simanjuntak','Siregar','Nasution','Ginting','Kurniawan','Rahayu','Hartono'
     ])[(abs(hashtext(o.code || '-' || k || 'ln')) % 30) + 1] AS lname,
    (SELECT array_agg(id ORDER BY code) FROM masterdata.departments) AS dep_ids,
    (SELECT array_agg(id ORDER BY name) FROM masterdata.positions)   AS pos_ids
  FROM masterdata.offices o
  CROSS JOIN generate_series(1, 24) AS k
  WHERE o.deleted_at IS NULL
) x;

-- ─────────────────────────────────────────────────────────────────────────────
-- 7) USER LOGIN — 20 per kantor; RBAC dari role bawaan; tertaut pegawai & kantor.
--    Password semua: "Inventra123!".
-- ─────────────────────────────────────────────────────────────────────────────
WITH pw AS (SELECT crypt('Inventra123!', gen_salt('bf')) AS hash),
er AS (
  SELECT e.id AS emp_id, e.name, e.office_id, o.code AS ocode, ot.name AS otype,
         row_number() OVER (PARTITION BY e.office_id ORDER BY e.code) AS rn
  FROM masterdata.employees e
  JOIN masterdata.offices o ON o.id = e.office_id
  JOIN masterdata.office_types ot ON ot.id = o.office_type_id
  WHERE e.deleted_at IS NULL
),
u AS (
  SELECT emp_id, name, office_id, ocode, rn,
    CASE
      WHEN otype = 'Kantor Pusat'   AND rn <= 2 THEN 'superadmin'
      WHEN otype = 'Kantor Wilayah' AND rn = 1  THEN 'kepala_kanwil'
      WHEN otype NOT IN ('Kantor Pusat','Kantor Wilayah') AND rn = 1 THEN 'kepala_unit'
      WHEN rn <= 5 THEN 'manager'
      ELSE 'staf'
    END AS role_code,
    CASE
      WHEN otype = 'Kantor Pusat'   AND rn = 1 THEN 'superadmin@demo.inventra.local'
      WHEN otype = 'Kantor Wilayah' AND rn = 1 THEN 'kanwil.' || lower(ocode) || '@demo.inventra.local'
      WHEN otype NOT IN ('Kantor Pusat','Kantor Wilayah') AND rn = 1 THEN 'kepala.' || lower(ocode) || '@demo.inventra.local'
      ELSE lower(split_part(name, ' ', 1)) || '.' || lower(split_part(name, ' ', 2)) || '.' || lower(ocode) || rn || '@demo.inventra.local'
    END AS email
  FROM er WHERE rn <= 20
)
INSERT INTO identity.users (employee_id, office_id, name, email, password_hash, role_id, status)
SELECT u.emp_id, u.office_id, u.name, u.email, pw.hash, r.id, 'active'::shared.user_status
FROM u
JOIN identity.roles r ON r.code = u.role_code AND r.deleted_at IS NULL
CROSS JOIN pw
ON CONFLICT DO NOTHING;

-- ─────────────────────────────────────────────────────────────────────────────
-- 8) ASET — 1.500 aset perlengkapan per kantor + tanah & gedung per kantor.
-- ─────────────────────────────────────────────────────────────────────────────

-- 8a) Template produk (kategori, nama, brand, model, rentang harga rupiah).
CREATE TEMP TABLE _tpl (
  tid serial PRIMARY KEY, cat_code text, item_name text,
  brand_name text, model_name text, cost_min bigint, cost_max bigint
) ON COMMIT DROP;
INSERT INTO _tpl (cat_code, item_name, brand_name, model_name, cost_min, cost_max) VALUES
  ('KOM','Laptop','Dell','Latitude 5440',12000000,22000000),
  ('KOM','Laptop','HP','EliteBook 840 G10',15000000,25000000),
  ('KOM','Laptop','Lenovo','ThinkPad E14',11000000,18000000),
  ('KOM','PC Desktop','Dell','OptiPlex 7010',9000000,15000000),
  ('KOM','PC Desktop','HP','ProDesk 400 G9',8000000,14000000),
  ('KOM','PC Desktop','Lenovo','ThinkCentre M70q',8000000,13000000),
  ('KOM','Server','Dell','PowerEdge R760',80000000,200000000),
  ('KOM','Monitor','Dell','Monitor P2422H',2000000,4000000),
  ('NET','Switch','Cisco','Catalyst 2960-X',8000000,20000000),
  ('NET','Router','Cisco','ISR 4331',25000000,60000000),
  ('ATM','Mesin ATM','Diebold Nixdorf','CINEO C4060',250000000,450000000),
  ('ATM','Mesin CRM','Diebold Nixdorf','DN Series 200 CRM',300000000,500000000),
  ('ATM','Mesin ATM','Wincor Nixdorf','ProCash 280',200000000,400000000),
  ('KR4','Mobil Operasional','Toyota','Avanza 1.5 G',220000000,280000000),
  ('KR4','Mobil Operasional','Toyota','Kijang Innova 2.4 V',400000000,550000000),
  ('KR4','Mobil Dinas','Toyota','Fortuner 2.4 VRZ',550000000,750000000),
  ('KR2','Sepeda Motor','Honda','Vario 160',26000000,30000000),
  ('KR2','Sepeda Motor','Honda','PCX 160',32000000,38000000),
  ('ELK','AC Split','Daikin','FTKC50 Split 2PK',6000000,12000000),
  ('ELK','AC Split','Panasonic','CS-XN9 Split 1PK',4000000,8000000),
  ('ELK','Printer','HP','LaserJet Pro M404dn',4000000,7000000),
  ('ELK','Printer','Epson','L3210 EcoTank',2000000,4000000),
  ('ELK','Proyektor','Epson','EB-X500 Proyektor',7000000,12000000),
  ('ELK','UPS','APC','Smart-UPS 1500VA',5000000,10000000),
  ('ELK','Telepon','Panasonic','KX-TS820 Telepon',300000,600000),
  ('FRN','Kursi Kerja','Indachi','D-8036 Kursi Kerja',1000000,3000000),
  ('FRN','Meja Kerja','Modera','Workstation 120',2000000,5000000),
  ('FRN','Lemari Arsip','Brother','B-204 Lemari Arsip',2000000,4000000),
  ('FRN','Sofa Ruang Tunggu','Chairman','SV-201 Sofa',5000000,12000000),
  ('SWL','Lisensi Microsoft 365',NULL,NULL,1500000,3000000),
  ('SWL','Lisensi Antivirus Symantec',NULL,NULL,500000,1200000),
  ('SWL','Aplikasi Modul Core Banking',NULL,NULL,100000000,500000000);

-- 8b) Rakit semua baris aset ke temp table. Pemilihan ruangan/pegawai/vendor
--     pakai array-index (O(1)) agar sanggup skala puluhan ribu baris.
CREATE TEMP TABLE _bulk ON COMMIT DROP AS
WITH
ntpl AS (SELECT count(*)::int AS c FROM _tpl),
rooms_agg AS (
  SELECT f.office_id, array_agg(r.id) AS rids
  FROM masterdata.rooms r JOIN masterdata.floors f ON f.id = r.floor_id
  WHERE r.deleted_at IS NULL GROUP BY f.office_id
),
emps_agg AS (
  SELECT office_id, array_agg(id) AS eids
  FROM masterdata.employees WHERE deleted_at IS NULL GROUP BY office_id
),
vend AS (SELECT array_agg(id) AS vids FROM masterdata.vendors WHERE deleted_at IS NULL),
-- Perlengkapan: 1.500 per kantor.
eq AS (
  SELECT
    o.id AS office_id, o.code AS office_code, o.code || '-' || n AS seed, n,
    t.cat_code, t.item_name, t.brand_name, t.model_name, t.cost_min, t.cost_max,
    ra.rids, ea.eids, vd.vids
  FROM masterdata.offices o
  CROSS JOIN generate_series(1, 1500) AS n
  CROSS JOIN ntpl
  CROSS JOIN vend vd
  JOIN _tpl t ON t.tid = (abs(hashtext(o.code || '-' || n)) % ntpl.c) + 1
  JOIN rooms_agg ra ON ra.office_id = o.id
  JOIN emps_agg ea ON ea.office_id = o.id
  WHERE o.deleted_at IS NULL
),
eq2 AS (
  SELECT
    e.office_id, e.office_code, e.seed, e.n,
    c.id AS category_id, c.code AS cat_code, c.asset_class,
    c.default_depreciation_method AS method, c.default_useful_life_months AS life,
    c.default_salvage_rate AS salvage_rate, c.default_fiscal_group AS fiscal_group,
    c.default_fiscal_life_months AS fiscal_life,
    btrim(e.item_name || coalesce(' ' || e.brand_name, '') || coalesce(' ' || e.model_name, '')) AS name,
    b.id AS brand_id, m.id AS model_id,
    CASE WHEN c.asset_class = 'intangible' THEN NULL
         ELSE e.rids[(abs(hashtext(e.seed || 'r')) % array_length(e.rids, 1)) + 1] END AS room_id,
    e.vids[(abs(hashtext(e.seed || 'v')) % array_length(e.vids, 1)) + 1] AS vendor_id,
    e.eids[(abs(hashtext(e.seed || 'e')) % array_length(e.eids, 1)) + 1] AS holder_id,
    (round((e.cost_min + (abs(hashtext(e.seed || 'c')) % (e.cost_max - e.cost_min + 1))) / 1000.0) * 1000)::numeric(18,2) AS cost,
    (current_date - ((abs(hashtext(e.seed || 'd')) % 2900) * interval '1 day'))::date AS purchase_date,
    (abs(hashtext(e.seed || 'st')) % 100) AS srand
  FROM eq e
  JOIN masterdata.categories c ON c.code = e.cat_code AND c.deleted_at IS NULL
  LEFT JOIN masterdata.brands b ON b.name = e.brand_name AND b.deleted_at IS NULL
  LEFT JOIN masterdata.models m ON m.name = e.model_name AND m.brand_id = b.id AND m.deleted_at IS NULL
),
eq_final AS (
  SELECT
    office_id, office_code, category_id, cat_code, asset_class, name, brand_id, model_id, room_id,
    CASE WHEN asset_class = 'intangible'
         THEN (SELECT id FROM masterdata.units WHERE name = 'Lisensi')
         ELSE (SELECT id FROM masterdata.units WHERE name = 'Unit') END AS unit_id,
    vendor_id,
    (CASE
       WHEN srand < 70 THEN 'available'
       WHEN srand < 86 THEN 'assigned'
       WHEN srand < 91 THEN 'under_maintenance'
       WHEN srand < 95 THEN 'in_transfer'
       WHEN srand < 97 THEN 'retired'
       WHEN srand < 99 THEN 'disposed'
       ELSE 'lost'
     END)::shared.asset_status AS status,
    CASE WHEN srand >= 70 AND srand < 86 THEN holder_id ELSE NULL END AS holder_id,
    purchase_date, extract(year FROM purchase_date)::int AS yr, cost,
    round(cost * coalesce(salvage_rate, 0), 2) AS salvage_value,
    method, life, fiscal_group, fiscal_life,
    CASE
      WHEN method IS NULL OR life IS NULL THEN 0::numeric(18,2)
      ELSE round((cost - round(cost * coalesce(salvage_rate, 0), 2)) / life
                 * least((extract(year FROM age(current_date, purchase_date)) * 12
                          + extract(month FROM age(current_date, purchase_date)))::int, life), 2)
    END AS accumulated,
    CASE WHEN asset_class = 'intangible' THEN NULL
         ELSE upper(substr(md5(seed), 1, 4) || '-' || substr(md5(seed), 5, 8)) END AS serial_number,
    'PO/' || extract(year FROM purchase_date)::int || '/' || lpad(((abs(hashtext(seed || 'po')) % 9999) + 1)::text, 4, '0') AS po_number,
    (ARRAY['RKAP','Anggaran Investasi','Dana Operasional Kantor'])[(abs(hashtext(seed || 'f')) % 3) + 1] AS funding_source,
    CASE WHEN asset_class = 'intangible' THEN NULL ELSE (purchase_date + interval '2 years')::date END AS warranty_expiry,
    'BAST/AKU/' || office_code || '/' || to_char(purchase_date, 'YYYY') || '/' || lpad(((abs(hashtext(seed || 'ba')) % 9999) + 1)::text, 4, '0') AS bast_no,
    n AS ord
  FROM eq2
),
-- Tanah & Gedung: satu per kantor.
og AS (
  SELECT o.id AS office_id, o.code AS office_code, o.name AS office_name,
         (SELECT r.id FROM masterdata.rooms r JOIN masterdata.floors f ON f.id = r.floor_id
          WHERE f.office_id = o.id AND r.deleted_at IS NULL ORDER BY r.code LIMIT 1) AS a_room,
         row_number() OVER (ORDER BY o.code) AS rn
  FROM masterdata.offices o WHERE o.deleted_at IS NULL
),
land AS (
  SELECT
    og.office_id, og.office_code, c.id AS category_id, c.code AS cat_code, c.asset_class,
    'Tanah - ' || og.office_name AS name, NULL::uuid AS brand_id, NULL::uuid AS model_id,
    og.a_room AS room_id, (SELECT id FROM masterdata.units WHERE name = 'Meter Persegi') AS unit_id,
    NULL::uuid AS vendor_id, 'available'::shared.asset_status AS status, NULL::uuid AS holder_id,
    (current_date - (((abs(hashtext(og.office_code || 'ld')) % 6000) + 4000) * interval '1 day'))::date AS purchase_date,
    NULL::int AS yr,
    ((abs(hashtext(og.office_code || 'lc')) % 18000 + 2000)::bigint * 1000000)::numeric(18,2) AS cost,
    0::numeric(18,2) AS salvage_value,
    c.default_depreciation_method AS method, c.default_useful_life_months AS life,
    c.default_fiscal_group AS fiscal_group, c.default_fiscal_life_months AS fiscal_life,
    0::numeric(18,2) AS accumulated,
    NULL::text AS serial_number, NULL::text AS po_number, 'Anggaran Investasi'::text AS funding_source,
    NULL::date AS warranty_expiry, 'BAST/AKU/' || og.office_code || '/TANAH/0001' AS bast_no,
    1000000 + og.rn AS ord
  FROM og JOIN masterdata.categories c ON c.code = 'TNH'
),
building AS (
  SELECT
    og.office_id, og.office_code, c.id AS category_id, c.code AS cat_code, c.asset_class,
    'Gedung Kantor - ' || og.office_name AS name, NULL::uuid AS brand_id, NULL::uuid AS model_id,
    og.a_room AS room_id, (SELECT id FROM masterdata.units WHERE name = 'Unit') AS unit_id,
    NULL::uuid AS vendor_id, 'available'::shared.asset_status AS status, NULL::uuid AS holder_id,
    (current_date - (((abs(hashtext(og.office_code || 'bd')) % 5000) + 3000) * interval '1 day'))::date AS purchase_date,
    NULL::int AS yr,
    ((abs(hashtext(og.office_code || 'bc')) % 25000 + 3000)::bigint * 1000000)::numeric(18,2) AS cost,
    0::numeric(18,2) AS salvage_value,
    c.default_depreciation_method AS method, c.default_useful_life_months AS life,
    c.default_fiscal_group AS fiscal_group, c.default_fiscal_life_months AS fiscal_life,
    NULL::numeric(18,2) AS accumulated,
    NULL::text AS serial_number, NULL::text AS po_number, 'Anggaran Investasi'::text AS funding_source,
    NULL::date AS warranty_expiry, 'BAST/AKU/' || og.office_code || '/GEDUNG/0001' AS bast_no,
    2000000 + og.rn AS ord
  FROM og JOIN masterdata.categories c ON c.code = 'BGN'
),
building_final AS (
  SELECT office_id, office_code, category_id, cat_code, asset_class, name, brand_id, model_id,
         room_id, unit_id, vendor_id, status, holder_id, purchase_date, yr, cost, salvage_value,
         method, life, fiscal_group, fiscal_life,
         round((cost - salvage_value) / life
               * least((extract(year FROM age(current_date, purchase_date)) * 12
                        + extract(month FROM age(current_date, purchase_date)))::int, life), 2) AS accumulated,
         serial_number, po_number, funding_source, warranty_expiry, bast_no, ord
  FROM building
)
SELECT office_id, office_code, category_id, cat_code, asset_class, name, brand_id, model_id,
       room_id, unit_id, vendor_id, status, holder_id, purchase_date, yr, cost, salvage_value,
       method, life, fiscal_group, fiscal_life, accumulated,
       serial_number, po_number, funding_source, warranty_expiry, bast_no, ord
FROM eq_final
UNION ALL
SELECT office_id, office_code, category_id, cat_code, asset_class, name, brand_id, model_id,
       room_id, unit_id, vendor_id, status, holder_id, purchase_date,
       extract(year FROM purchase_date)::int, cost, salvage_value,
       method, life, fiscal_group, fiscal_life, accumulated,
       serial_number, po_number, funding_source, warranty_expiry, bast_no, ord
FROM land
UNION ALL
SELECT office_id, office_code, category_id, cat_code, asset_class, name, brand_id, model_id,
       room_id, unit_id, vendor_id, status, holder_id, purchase_date,
       extract(year FROM purchase_date)::int, cost, salvage_value,
       method, life, fiscal_group, fiscal_life, accumulated,
       serial_number, po_number, funding_source, warranty_expiry, bast_no, ord
FROM building_final;

-- 8c) INSERT aset final + asset_tag (seq per office/category/year) + penyusutan.
INSERT INTO asset.assets
  (asset_tag, name, category_id, brand_id, model_id, room_id, office_id, unit_id, status,
   serial_number, purchase_date, purchase_cost, vendor_id, po_number, funding_source, warranty_expiry,
   asset_class, capitalized, depreciation_method, useful_life_months, salvage_value,
   fiscal_group, fiscal_life_months, accumulated_depreciation, book_value,
   acquisition_bast_no, current_holder_employee_id, created_by_id, notes)
SELECT
  b.office_code || '-' || b.cat_code || '-' || b.yr || '-' ||
    lpad(row_number() OVER (PARTITION BY b.office_id, b.category_id, b.yr ORDER BY b.ord)::text, 5, '0'),
  b.name, b.category_id, b.brand_id, b.model_id, b.room_id, b.office_id, b.unit_id, b.status,
  b.serial_number, b.purchase_date, b.cost, b.vendor_id, b.po_number, b.funding_source, b.warranty_expiry,
  b.asset_class, true, b.method, b.life, b.salvage_value,
  b.fiscal_group, b.fiscal_life, b.accumulated, (b.cost - b.accumulated),
  b.bast_no, b.holder_id,
  (SELECT id FROM identity.users WHERE email = 'superadmin@demo.inventra.local'),
  NULL
FROM _bulk b;

-- 8d) Sinkronkan asset_tag_counters agar tag berikutnya dari app tidak bentrok.
INSERT INTO asset.asset_tag_counters (office_id, category_id, year, last_seq)
SELECT office_id, category_id, yr, count(*)
FROM _bulk GROUP BY office_id, category_id, yr;

-- ─────────────────────────────────────────────────────────────────────────────
-- 9) Ringkasan.
-- ─────────────────────────────────────────────────────────────────────────────
DO $$
DECLARE n_off int; n_emp int; n_usr int; n_ast int; min_u int; min_a int;
BEGIN
  SELECT count(*) INTO n_off FROM masterdata.offices  WHERE deleted_at IS NULL;
  SELECT count(*) INTO n_emp FROM masterdata.employees WHERE deleted_at IS NULL;
  SELECT count(*) INTO n_usr FROM identity.users       WHERE email LIKE '%@demo.inventra.local';
  SELECT count(*) INTO n_ast FROM asset.assets         WHERE deleted_at IS NULL;
  SELECT min(c) INTO min_u FROM (SELECT count(*) c FROM identity.users WHERE office_id IS NOT NULL GROUP BY office_id) z;
  SELECT min(c) INTO min_a FROM (SELECT count(*) c FROM asset.assets   WHERE deleted_at IS NULL GROUP BY office_id) z;
  RAISE NOTICE 'Seed selesai — kantor=%, pegawai=%, user demo=%, aset=% (min user/kantor=%, min aset/kantor=%)',
    n_off, n_emp, n_usr, n_ast, min_u, min_a;
END $$;

COMMIT;
