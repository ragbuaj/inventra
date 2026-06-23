# PRD — Inventra (Asset Management System)

| | |
|---|---|
| **Produk** | Inventra |
| **Versi dokumen** | 1.0 (draft) |
| **Tanggal** | 2026-06-23 |
| **Status** | Draft — menunggu review |
| **Pemilik** | — |
| **Jenis aplikasi** | Web app manajemen aset fisik / inventaris perusahaan |
| **Stack** | Go (Gin) + PostgreSQL + Redis + MinIO · Nuxt 4 + Nuxt UI |

---

## 1. Ringkasan (Overview)

**Inventra** adalah aplikasi web untuk mengelola **aset fisik / inventaris** sebuah organisasi: mencatat aset, melacak siapa memegang/menggunakannya, menjadwalkan perawatan, serta menghitung penyusutan nilai dan menghasilkan laporan. Aplikasi ditujukan untuk **satu organisasi** dengan beberapa peran pengguna (multi-role).

Tujuan proyek: aplikasi setingkat industri dengan **arsitektur rapi, fitur lengkap, dan kode berkualitas** — sekaligus menjadi bagian portfolio (berdampingan dengan `project1`).

### 1.1 Masalah yang dipecahkan
Organisasi sering melacak aset (laptop, kendaraan, mesin, perabot, peralatan) memakai spreadsheet yang rawan: data tersebar, tidak ada riwayat siapa memegang aset, perawatan terlewat, dan nilai aset tidak terhitung untuk keperluan keuangan/audit. Inventra menyatukan semua ini dalam satu sistem dengan kontrol akses dan jejak audit.

### 1.2 Tujuan (Goals)
- G1 — Satu sumber kebenaran untuk seluruh aset fisik organisasi.
- G2 — Riwayat lengkap peminjaman/penugasan aset (siapa, kapan, kondisi).
- G3 — Perawatan terjadwal dengan reminder agar tidak terlewat.
- G4 — Perhitungan depresiasi otomatis + laporan & dashboard untuk pengambilan keputusan.
- G5 — Kontrol akses berbasis peran (RBAC) + jejak audit setiap perubahan penting.

### 1.3 Non-Goals (di luar lingkup versi ini)
- Multi-tenant (banyak organisasi dalam satu instance) — single-org dulu.
- Integrasi akuntansi eksternal (mis. ke software akuntansi pihak ketiga).
- Modul procurement/purchasing penuh (PO, approval pembelian).
- Aplikasi mobile native (web responsif sudah cukup).
- Manajemen aset non-fisik (lisensi software, aset keuangan/investasi).

---

## 2. Pengguna & Peran (Roles)

Organisasi berstruktur **berjenjang 4 tingkat**: **Kantor Pusat → Kantor Wilayah → Kantor Cabang → Kantor Outlet** (pohon via `parent_id`, mendukung penambahan tingkat). Peran mencerminkan jenjang ini, dengan **lingkup akses (scoping) mengikuti subtree kantor**.

| Peran | Lingkup | Deskripsi & kemampuan inti |
|---|---|---|
| **Superadmin** | Global | Pengelola sistem penuh: kelola user/peran/field-permission, semua master data & konfigurasi, semua data & laporan. |
| **Kepala Kanwil** | Kantor Wilayah + seluruh Cabang & Outlet di bawahnya | Mengawasi & menyetujui pengajuan lintas-cabang dalam wilayahnya, lihat laporan tingkat wilayah, kelola data kantor di wilayahnya. |
| **Kepala Unit** | Satu kantor (Cabang/Outlet) + turunannya | Menyetujui pengajuan & melihat laporan dalam lingkup kantornya. |
| **Manager** (Asset Manager / Staf Aset) | Sesuai penempatan kantor | Operasional aset: CRUD aset, check-out/check-in, kelola maintenance, dalam lingkup kantornya. |
| **Staf** | Data miliknya | Pengguna aset: lihat aset yang dipegangnya, ajukan peminjaman/laporan kerusakan/pengecualian. |

### 2.1 Matriks Hak Akses (RBAC)

Legend: ✅ penuh · 🔵 terbatas pada lingkup kantor/wilayahnya (lihat §2.2) · ❌ tidak.

| Kapabilitas | Superadmin | Kepala Kanwil | Kepala Unit | Manager | Staf |
|---|:---:|:---:|:---:|:---:|:---:|
| Kelola user, peran & field-permission | ✅ | ❌ | ❌ | ❌ | ❌ |
| Master data referensi global¹ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Master data kantor & pegawai | ✅ | 🔵 | ❌ | ❌ | ❌ |
| CRUD aset | ✅ | ❌ | ❌ | 🔵 | ❌ |
| Lihat katalog aset | ✅ | 🔵 | 🔵 | 🔵 | 🔵 (miliknya) |
| Check-out / check-in aset | ✅ | ❌ | ❌ | 🔵 | ❌ |
| Ajukan pengajuan (peminjaman/kerusakan/dll) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Approve / tolak pengajuan | ✅ | 🔵 | 🔵 | 🔵² | ❌ |
| Approve pengecualian valuasi (sensitif) | ✅ | 🔵 | ❌ | ❌ | ❌ |
| Kelola jadwal & catatan maintenance | ✅ | ❌ | ❌ | 🔵 | ❌ |
| Konfigurasi & jalankan depresiasi | ✅ | ❌ | ❌ | ❌ | ❌ |
| Lihat laporan & dashboard | ✅ | 🔵 | 🔵 | 🔵 | 🔵 (miliknya) |
| Ekspor laporan (PDF/Excel) | ✅ | 🔵 | 🔵 | 🔵 | ❌ |
| Lihat audit trail | ✅ | 🔵 (read) | 🔵 (read) | ❌ | ❌ |

¹ Provinsi, kota, jenis kantor, kategori aset, kategori perawatan, kategori masalah, satuan, brand/model.
² Manager hanya untuk pengajuan operasional ringan; penghapusan aset & hal sensitif naik ke Kepala Unit/Kanwil/Superadmin.

> **Peran dapat dikonfigurasi.** 5 peran & matriks di atas adalah **default ter-seed**. Peran disimpan di tabel `roles` dan izin per-aksi di `role_permissions`, sehingga Superadmin dapat menambah peran kustom & menyesuaikan izin. Detail skema: [DATABASE.md §4.1](DATABASE.md).

### 2.2 Lingkup Akses Data (Data Scoping / Ownership)

Akses dibatasi **per-baris data** berdasarkan **kepemilikan** dan **hierarki kantor**, dengan **lingkup yang dapat dikonfigurasi** (bukan hardcode per-role).

**Scope level (tingkat lingkup) yang tersedia:**

| Level | Arti |
|---|---|
| `global` | Seluruh data tanpa batas |
| `office_subtree` | Kantor penempatan user **+ seluruh kantor turunannya** (subtree `office_id`) |
| `office` | Hanya kantor penempatan user |
| `own` | Hanya data milik user (yang ia buat / aset yang ia pegang) |

**Dapat dikonfigurasi:** pemetaan **role → scope level** disimpan di tabel **`data_scope_policies`** (bukan hardcode) dan dapat diubah Superadmin lewat halaman pengaturan. Granularitas **dua tingkat**:
- **Default per-role** (baris `module = null`) — berlaku untuk semua modul.
- **Override per-modul** (baris `module = <nama modul>`) — menimpa default untuk modul tertentu. Contoh: role Manager `office_subtree` untuk *aset* namun `own` untuk *pengajuan*.

Scope efektif = override per-modul bila ada, jika tidak pakai default per-role.

**Default ter-seed** (mengikuti struktur 4-jenjang):

- **Superadmin** → `global`
- **Kepala Kanwil** → `office_subtree` (Kantor Wilayah + Cabang + Outlet di bawahnya)
- **Kepala Unit** → `office_subtree` (kantornya + turunannya)
- **Manager** → `office_subtree` (kantor penempatan + turunannya)
- **Staf** → `own`

Penegakan dilakukan di **service layer** (bukan hanya UI): saat query, sistem membaca scope level efektif role pemanggil lalu menerapkan filter yang sesuai (`global` = tanpa filter; `office_subtree` = `office_id IN (descendant_ids)`; `office` = `office_id = user.office_id`; `own` = `created_by/holder = user`). Pelanggaran (akses di luar lingkup via ID langsung) ditolak `403`.

> Keterkaitan: user (akun login) ditautkan ke record **pegawai** (`employee_id`) dan ditempatkan pada satu **kantor** (`office_id`). "Data miliknya" = data yang terhubung ke `employee_id`; "lingkup kantor" = subtree dari `office_id`. Scope level menentukan mana yang berlaku.

### 2.3 Hak Akses Per-Field (Field-Level Permissions)

Selain per-aksi dan per-baris, **tiap field pada entitas dapat dikonfigurasi hak aksesnya per-peran** (lihat/edit). Contoh: harga beli aset hanya boleh dilihat Superadmin & Manager, tidak oleh Staf; nilai buku hanya Superadmin.

- **Berlaku untuk semua entitas** yang diekspos API (aset, pegawai, user, master data, pengajuan, dll) — bukan hanya entitas sensitif.
- Konfigurasi disimpan di tabel **`field_permissions`** (`entity`, `field`, `role`, `can_view`, `can_edit`) — bukan hardcode.
- Ditegakkan di lapisan serialisasi response (field yang tak boleh dilihat **dihilangkan/di-mask** dari payload) dan validasi request (field yang tak boleh diedit ditolak/diabaikan).
- **Field-registry**: tiap entitas mendaftarkan daftar field-nya (otomatis dari skema) agar konfigurasi konsisten & tidak ada field terlewat; field tanpa aturan eksplisit memakai **default per-role** yang di-seed.
- Disediakan **default sensible** saat seed; Superadmin menyesuaikan lewat halaman pengaturan. Konfigurasi ditembolok di Redis (invalidasi saat diubah).

---

## 3. Kebutuhan Fungsional (Functional Requirements)

### 3.1 Identity & Akses
- **FR-1.1** Registrasi/penambahan user oleh Superadmin (nama, email, peran, **kantor penempatan** `office_id`, pegawai tertaut, departemen).
- **FR-1.2** Login dengan email + password; sesi memakai JWT (access + refresh token). **Refresh token & denylist disimpan di Redis** untuk mendukung logout/pencabutan sesi.
- **FR-1.3** **Login dengan Google (OAuth2)**: user dapat masuk memakai akun Google. Alur authorization-code: `GET /auth/google` (redirect ke Google) → `GET /auth/google/callback` → backend menukar code, mengambil profil (email terverifikasi), lalu menerbitkan JWT yang sama seperti login biasa.
- **FR-1.4** **Akun & penautan (account linking)**: jika email Google sudah ada sebagai user lokal, akun ditautkan (bukan duplikat). User baru via Google dibuat dengan peran default **Staf**; Superadmin dapat menaikkan perannya & menetapkan kantor penempatannya kemudian. Email Google harus berstatus terverifikasi.
- **FR-1.5** Password di-hash (bcrypt/argon2), reset password via token. User yang dibuat lewat Google boleh tanpa password (login hanya via Google) sampai mereka menyetel password.
- **FR-1.6** Penegakan RBAC di setiap endpoint sesuai matriks pada §2.1.
- **FR-1.7** Profil user: ubah nama, password, lihat status penautan Google, lihat aset yang sedang dipegang.
- **FR-1.8** **Rate limiting** (via Redis): batasi percobaan login per akun/IP (anti brute-force) dan throttle endpoint sensitif; token reset password & verifikasi email memakai TTL Redis.

### 3.2 Katalog & Registrasi Aset
- **FR-2.1** CRUD aset dengan atribut: kode/tag unik, nama, kategori, lokasi, status, nomor seri, tanggal beli, harga beli, pemasok, garansi, spesifikasi (fleksibel/JSON), foto, catatan.
- **FR-2.2** **Kode aset (`asset_tag`) unik** dibuat otomatis dengan format **`<kode_kantor>-<kode_kategori>-<tahun_beli>-<sequence>`**, di mana `sequence` = **5 digit** yang berjalan **per kantor & kategori** dan **direset tiap tahun**. Contoh: `JKT01-ELK-2026-00001`. Validasi unik; detail generator di [DATABASE.md §4.7](DATABASE.md).
- **FR-2.3** **Status aset**: `available`, `assigned`, `under_maintenance`, `retired`, `lost`. Perubahan status mengikuti aturan transisi (lihat §5).
- **FR-2.4** Master data **Kategori** (mis. Elektronik, Kendaraan, Furnitur) — bisa punya nilai default depresiasi (metode, masa manfaat).
- **FR-2.5** Master data **Lokasi berjenjang**: **Kantor → Lantai → Ruangan**. Aset menunjuk ke Ruangan (yang mewarisi Lantai & Kantor). Lihat daftar master data lengkap di §3.7.
- **FR-2.6** Upload **foto/lampiran** aset disimpan di **MinIO** (S3-compatible) via Storage interface.
  - **Validasi**: tipe file di-whitelist (mis. jpg/png/webp/pdf), tolak file kosong/korup, dan **batas ukuran** (maks. mis. 5 MB; tolak di bawah ambang minimal yang wajar). Ambang dikonfigurasi via env.
  - **Kompresi saat simpan**: gambar di-resize ke dimensi maks. & di-re-encode (mis. WebP/JPEG mutu ~80%) sebelum disimpan, untuk hemat storage; thumbnail dibuat untuk tampilan daftar.
- **FR-2.7** Pencarian, filter (kategori/lokasi/status), sort, dan pagination pada daftar aset.
- **FR-2.8** Detail aset menampilkan: info, riwayat penugasan, riwayat maintenance, dan jadwal depresiasi — **dengan field dibatasi sesuai §2.3**.
- **FR-2.9** **Registrasi & penghapusan aset lewat pengajuan + approval** (maker-checker) — lihat §3.6.
- **FR-2.10** Aset dapat ditandai **dikecualikan dari penghitungan kekayaan/valuasi** lewat pengajuan + approval (§3.6); aset terkecuali tidak dihitung dalam total nilai aset di laporan/dashboard, namun tetap terdata.
- **FR-2.11** **Import massal aset** via **CSV & XLSX**: unduh template, unggah berkas, **validasi per-baris** (tipe data, referensi master data, tag unik), dan **laporan hasil** (baris sukses vs gagal + alasan). Baris valid dibuat; baris gagal dilewati & dapat diunduh untuk dikoreksi. Import mengikuti aturan otorisasi & approval yang berlaku (§3.6) sesuai konfigurasi.
- **FR-2.12** **Barcode per aset**: setiap aset otomatis memiliki **barcode** (mis. Code128) yang di-encode dari `asset_tag`. Barcode dapat **dicetak sebagai label** (tunggal maupun massal/batch) dan **dipindai** untuk look-up / pencarian aset cepat. **QR code** juga disediakan sebagai alternatif label.

### 3.3 Check-out / Check-in (Assignment / Peminjaman)
- **FR-3.1** **Check-out**: tugaskan aset `available` ke seorang **pegawai** (custodian) dan/atau lokasi, dengan tanggal pinjam, jatuh tempo (opsional), dan catatan kondisi keluar. Status aset → `assigned`.
- **FR-3.2** **Check-in**: kembalikan aset, catat tanggal kembali & kondisi masuk. Status aset → `available` (atau `under_maintenance` bila perlu servis).
- **FR-3.3** **Permintaan peminjaman**: Staf mengajukan (§3.6); Manager/Kepala Unit meng-approve atau menolak. Approve memicu check-out.
- **FR-3.4** **Riwayat penugasan** per aset dan per pegawai (siapa memegang apa, dan kapan).
- **FR-3.5** Penanda **overdue** untuk aset yang lewat jatuh tempo (dipakai di dashboard).
- **FR-3.6** Satu aset hanya boleh ditugaskan ke satu pemegang aktif pada satu waktu.

### 3.4 Maintenance & Perawatan
- **FR-4.1** **Jadwal perawatan berkala** per aset (interval, mis. tiap 6 bulan) → menghitung `next_due_date`.
- **FR-4.2** **Catatan maintenance**: tipe (`preventive`/`corrective`), **kategori perawatan**, tanggal, status (`scheduled`/`in_progress`/`completed`/`cancelled`), biaya, vendor/teknisi, deskripsi.
- **FR-4.3** Memulai maintenance pada aset → status aset `under_maintenance`; menyelesaikan → kembali `available`.
- **FR-4.4** **Laporan kerusakan** oleh Staf (dengan **kategori masalah**) → masuk antrean sebagai permintaan maintenance.
- **FR-4.5** **Reminder/notifikasi** maintenance jatuh tempo (in-app; email opsional di masa depan).
- **FR-4.6** Total **biaya maintenance** terakumulasi per aset (dipakai di laporan).

### 3.5 Depresiasi & Pelaporan
- **FR-5.1** Konfigurasi depresiasi per aset/kategori: **metode** (`straight_line` garis lurus, `declining_balance` saldo menurun), masa manfaat (bulan), nilai sisa (salvage value).
- **FR-5.2** Hitung **nilai buku (book value)** aset pada periode berjalan secara otomatis.
- **FR-5.3** **Jadwal depresiasi** per aset (per periode: nilai awal, beban penyusutan, nilai akhir) — disajikan via **read model** khusus agar laporan cepat.
- **FR-5.4** **Dashboard**: total aset, nilai aset (perolehan vs buku), aset per status/kategori/lokasi, aset overdue, maintenance jatuh tempo, biaya maintenance.
- **FR-5.5** **Laporan**: daftar aset + nilai buku, laporan depresiasi per periode, laporan utilisasi/penugasan, laporan biaya maintenance.
- **FR-5.6** **Ekspor** laporan ke **PDF** (layout cetak rapi) dan **Excel (.xlsx)** (data tabular siap olah).
- **FR-5.7** **Audit trail menyeluruh**: setiap operasi tulis (create/update/delete) pada **seluruh entitas/tabel** dicatat ke `audit_logs` — aktor, entitas, ID, aksi, perubahan (diff before/after), dan waktu. Dapat ditelusuri per entitas maupun per user. Diterapkan secara terpusat (mis. lewat hook/decorator di repository/service), bukan per-handler manual.

### 3.6 Pengajuan & Persetujuan (Approval / Maker-Checker)

Beberapa aksi sensitif tidak langsung dieksekusi, melainkan melalui **pengajuan → review → approve/tolak**. Satu mekanisme generik melayani beberapa jenis:

- **FR-6.1** Jenis pengajuan: **Registrasi aset baru**, **Penghapusan aset**, **Peminjaman** (§3.3), **Laporan kerusakan/maintenance** (§3.4), dan **Pengecualian aset dari penghitungan kekayaan** (§3.7-valuasi).
- **FR-6.2** Pengaju (maker) membuat request berisi payload + alasan; status awal `pending`.
- **FR-6.3** Approver (checker = Manager / Kepala Unit / Kepala Kanwil / Superadmin sesuai jenis & lingkup kantor, lihat §A7) **approve** atau **tolak** dengan catatan. Approve memicu eksekusi aksi sebenarnya (mis. aset benar-benar dibuat/dihapus, atau flag pengecualian diset). Tolak menutup request tanpa efek.
- **FR-6.4** Pengaju tidak boleh menyetujui pengajuannya sendiri (segregation of duty).
- **FR-6.5** Daftar pengajuan dengan filter status & jenis; notifikasi in-app ke approver saat ada pengajuan baru, dan ke pengaju saat diputuskan.
- **FR-6.6** Setiap keputusan tercatat di audit trail (§5.7).

### 3.7 Master Data & Pengecualian Valuasi

- **FR-7.1** **Kantor (office)** dengan **hierarki 4 jenjang** (`parent_id`: **Pusat → Wilayah → Cabang → Outlet**) dan **jenis kantor** (`office_type`). Tiap kantor menunjuk **Provinsi** & **Kota**.
- **FR-7.2** **Lokasi fisik berjenjang** di dalam kantor: **Kantor → Lantai (floor) → Ruangan (room)**; aset menunjuk ke Ruangan.
- **FR-7.3** Master data **Pegawai** (employee) sebagai custodian aset — terpisah dari **User** (akun login). Satu user dapat ditautkan ke satu pegawai (`employee_id`). Tidak semua pegawai punya akun login. Pegawai menunjuk Departemen, Jabatan, dan Kantor.
- **FR-7.4** Master data referensi:
  - **Jenis kantor** (office types)
  - **Provinsi** & **Kota** (kota menunjuk provinsi)
  - **Departemen/Divisi** dan **Jabatan**
  - **Vendor/Pemasok** (pembelian & servis)
  - **Brand & Model** (normalisasi spesifikasi aset)
  - **Kategori aset**
  - **Kategori perawatan** (maintenance category — mis. Servis Rutin, Kalibrasi, Perbaikan)
  - **Kategori masalah** (problem category — dipakai saat laporan kerusakan, mis. Hardware, Listrik, Fisik)
  - **Satuan** (unit of measure — mis. Unit, Pcs, Set)
- **FR-7.5** Semua master data mendukung CRUD oleh peran berwenang (§2.1), pencarian, dan status aktif/nonaktif.
- **FR-7.5b** **Import massal master data** (CSV/XLSX) untuk entitas bervolume besar — terutama **Pegawai** dan **Kantor**, serta provinsi/kota — dengan template, validasi per-baris, dan laporan hasil (sama seperti FR-2.11).
- **FR-7.6** **Pengecualian valuasi**: aset yang disetujui dikecualikan (§3.6) diberi flag `excluded_from_valuation` + alasan; laporan/dashboard total kekayaan mengabaikan aset ini namun tetap menampilkannya sebagai "terkecuali".

---

## 4. User Stories (contoh utama)

- Sebagai **Superadmin**, saya menambah user baru, menetapkan peran & kantor penempatannya, agar tim mengakses sistem sesuai wewenang & lingkup.
- Sebagai **Manager**, saya mengajukan registrasi laptop baru dengan kategori, ruangan, dan harga beli, agar aset tercatat resmi setelah disetujui.
- Sebagai **Manager**, saya melakukan check-out laptop ke seorang pegawai dengan tanggal jatuh tempo, agar kepemilikan terlacak.
- Sebagai **Staf**, saya mengajukan peminjaman proyektor, agar bisa dipakai presentasi setelah disetujui.
- Sebagai **Staf**, saya melaporkan AC ruangan rusak (kategori masalah: Listrik), agar dijadwalkan perbaikan.
- Sebagai **Kepala Unit**, saya menyetujui pengajuan peminjaman di unit saya, agar aset terkendali.
- Sebagai **Kepala Kanwil**, saya melihat laporan aset seluruh unit di wilayah saya, agar punya gambaran menyeluruh.
- Sebagai **Superadmin**, saya melihat laporan depresiasi akhir tahun, agar nilai buku aset diketahui untuk keperluan keuangan.

---

## 5. Aturan Status Aset (State Machine)

```
available ──checkout──▶ assigned
assigned ──checkin──▶ available
available/assigned ──start maintenance──▶ under_maintenance
under_maintenance ──complete──▶ available
available/assigned ──dispose──▶ retired
(any) ──mark lost──▶ lost
```

- Aset `assigned` harus di-check-in dulu sebelum bisa `retired` (kecuali `lost`).
- Aset `under_maintenance` tidak bisa di-check-out.
- Transisi tidak valid ditolak oleh service layer.

---

## 6. Model Data (high-level)

Entitas inti dan relasi (detail kolom final ditentukan saat migrasi DB):

**Identity & RBAC**
- **users** (id, employee_id?, office_id, name, email, password_hash?, google_id?, avatar_url?, role[superadmin/kepala_kanwil/kepala_unit/manager/staf], status, timestamps) — `password_hash` & `google_id` nullable; `office_id` = kantor penempatan (dasar scoping hierarki)
- **field_permissions** (id, entity, field, role, can_view, can_edit)
- **data_scope_policies** (id, role, module?, scope_level[global/office_subtree/office/own]) — `module` null = default per-role (semua modul); terisi = **override per-modul** (menimpa default). Unik per (role, module).

**Master data — referensi & geografi**
- **provinces** (id, name, code)
- **cities** (id, province_id, name, code)
- **office_types** (id, name) — jenis kantor (Pusat, Wilayah, Cabang, Outlet)
- **departments** (id, name, code) · **positions** (id, name) — jabatan
- **vendors** (id, name, contact, address)
- **brands** (id, name) · **models** (id, brand_id, name)
- **categories** (id, name, code, parent_id?, default_depreciation_method, default_useful_life_months, default_salvage_rate) — kategori aset
- **maintenance_categories** (id, name) · **problem_categories** (id, name)
- **units** (id, name, symbol) — satuan

**Master data — struktur kantor & orang**
- **offices** (id, parent_id?, office_type_id, province_id, city_id, name, code, address) — hierarki Pusat→Wilayah→Cabang→Outlet via `parent_id`
- **floors** (id, office_id, name, level) · **rooms** (id, floor_id, name, code)
- **employees** (id, nip/code, name, email?, department_id?, position_id?, office_id, status) — custodian aset

**Aset & operasional**
- **assets** (id, asset_tag, name, category_id, brand_id?, model_id?, room_id, office_id, unit_id?, status, serial_number, purchase_date, purchase_cost, vendor_id?, warranty_expiry, specifications JSONB, depreciation_method, useful_life_months, salvage_value, current_holder_employee_id?, excluded_from_valuation, notes, timestamps) — `office_id` (diturunkan dari ruangan) dipakai untuk scoping
- **asset_attachments** (id, asset_id, kind[photo/document], object_key, thumbnail_key?, size, mime, created_at) — file di MinIO
- **assignments** (id, asset_id, employee_id, assigned_by_id, checkout_date, due_date, checkin_date, condition_out, condition_in, status, notes)
- **maintenance_schedules** (id, asset_id, maintenance_category_id?, interval_months, last_done_date, next_due_date)
- **maintenance_records** (id, asset_id, maintenance_category_id?, problem_category_id?, type, status, scheduled_date, completed_date, cost, vendor_id?, performed_by, description, reported_by_id?)
- **depreciation_entries** (read model) (id, asset_id, period, opening_value, depreciation_amount, closing_value)

**Approval & audit**
- **requests** (id, type[asset_create/asset_delete/assignment/maintenance/valuation_exclusion], office_id, payload JSONB, reason, status[pending/approved/rejected], requested_by_id, decided_by_id?, decision_note?, timestamps) — maker-checker generik (§3.6); `office_id` untuk routing approver berjenjang
- **audit_logs** (id, actor_id, entity_type, entity_id, action[create/update/delete], changes JSONB, created_at) — mencakup **seluruh tabel**
- **import_jobs** (id, target[asset/employee/office/…], format[csv/xlsx], filename, status[pending/processing/completed/failed], total_rows, success_rows, failed_rows, error_report_key?, created_by_id, created_at) — melacak proses import massal; berkas error tersimpan di MinIO, progres dapat ditembolok di Redis

Relasi kunci: `provinces` 1—N `cities`; `offices` self-ref `parent_id` (Kanwil→Unit), N—1 `office_types`/`provinces`/`cities`, 1—N `floors` 1—N `rooms`; `assets` N—1 `rooms`/`offices`/`categories`/`brands`/`models`/`vendors`/`units`; `assets` 1—N `assignments`/`maintenance_records`/`depreciation_entries`/`asset_attachments`; `employees` N—1 `offices`/`departments`/`positions`, 1—N `assignments`; `users` N—1 `employees`/`offices`.

---

## 7. Arsitektur Teknis (ringkas)

**Pola: Modular Monolith + Clean Architecture** (Opsi A). Satu service Go, modul berkomunikasi via service interface + **domain event in-process** (bukan message broker).

**Modul backend:**
```
identity     → auth (lokal + Google), user, RBAC, field_permissions
masterdata   → kantor(hierarki)/lantai/ruangan, provinsi/kota, jenis kantor, departemen, jabatan, pegawai,
               vendor, brand/model, kategori aset, kategori perawatan, kategori masalah, satuan
asset        → katalog, tag, barcode/label (Code128 + QR), lampiran (MinIO), status, valuasi/pengecualian, import massal (CSV/XLSX)
assignment   → check-out/in, riwayat
maintenance  → jadwal, catatan, reminder
depreciation → perhitungan + read model
approval     → mekanisme pengajuan-persetujuan generik (maker-checker)
reporting    → dashboard, ekspor PDF/Excel
import       → import massal CSV/XLSX (aset & master data): template, validasi per-baris, laporan hasil
```

**Concern lintas-modul (cross-cutting):** audit logging menyeluruh, **data scoping yang dapat dikonfigurasi** (`data_scope_policies`), dan field-level permission (`field_permissions`) diterapkan sebagai middleware/decorator terpusat — bukan diulang di tiap handler. Ketiganya membentuk lapisan **otorisasi yang dapat dikonfigurasi Superadmin** (per-aksi · per-baris/lingkup · per-field).

**Lapisan tiap modul:** `domain.go` (entity + interface) → `service.go` (business logic) → `repository.go` (sqlc) → `handler.go` (Gin, tipis) → `routes.go` → `events.go`.

**Aturan:** modul tidak saling impor repository; interface didefinisikan di sisi consumer; wiring eksplisit di `cmd/api/main.go`.

**Stack:**

| Lapisan | Teknologi |
|---|---|
| Bahasa/Framework | Go 1.22+ · Gin |
| Database | PostgreSQL 16 |
| Cache & state | **Redis 7** (caching, session/token, rate limiting, token TTL, notifikasi) |
| Query | sqlc |
| Migrasi | golang-migrate |
| Auth | JWT (access + refresh) + OAuth2 (Google login) |
| File storage | **MinIO** (S3-compatible) via Storage interface; kompresi/resize gambar saat unggah |
| Ekspor | PDF (mis. maroto/gofpdf) + Excel `.xlsx` (excelize) |
| Frontend | Nuxt 4 (Vue 3 + Vite) · Nuxt UI · Pinia · VeeValidate + Zod |
| i18n | @nuxtjs/i18n (ID/EN) |
| DevOps | Docker Compose · GitHub Actions |

**Redis (cache & state):** dipakai untuk —
- **Caching**: master data & referensi (provinsi/kota/kategori/dll), **konfigurasi otorisasi** (`field_permissions`, `data_scope_policies`), **subtree kantor** (daftar `descendant_ids` per kantor — mahal dihitung), dan agregat dashboard/laporan. Cache **di-invalidasi** saat data sumber berubah (mis. ubah field-permission → bust cache otorisasi).
- **Session/token**: penyimpanan **refresh token** + **denylist** access token (mendukung logout & pencabutan sesi), serta data sesi ringan.
- **Rate limiting**: batasi percobaan **login** (anti brute-force) dan throttle API per user/IP.
- **Token ber-TTL**: token **reset password**, **verifikasi email**, dan OTP (bila ada) dengan kedaluwarsa otomatis.
- **Notifikasi & lock**: backing store notifikasi in-app (approval/maintenance) dan **distributed lock** untuk operasi sensitif (mis. generate `asset_tag` berurutan, penjadwal reminder) bila diperlukan.

> Catatan: Redis bersifat pelengkap, bukan sumber kebenaran. Kehilangan Redis tidak menyebabkan kehilangan data (PostgreSQL tetap otoritatif); sistem tetap berjalan dengan degradasi performa.

**Frontend (Nuxt):** layout `admin` (Superadmin/Kepala/Manager) & `app` (Staf), route middleware untuk RBAC + scoping, halaman: dashboard, aset (list/detail/form), penugasan, maintenance, pengajuan/approval, laporan, master data, user, profil.

---

## 8. Kebutuhan Non-Fungsional

- **Keamanan**: hashing password, JWT, dan **otorisasi 3-lapis yang dapat dikonfigurasi** — RBAC per-aksi, **data scope per-baris (configurable, `data_scope_policies`)**, dan **field-level permission** — semuanya ditegakkan di server (bukan hanya UI), plus validasi input.
- **Auditability**: seluruh operasi tulis tercatat di `audit_logs` (semua tabel) dengan diff.
- **Penanganan file**: validasi tipe & ukuran (min/maks), kompresi + thumbnail gambar, disimpan di MinIO; URL akses melalui presigned/proxy yang menghormati hak akses.
- **Performa**: daftar memakai pagination + index DB; laporan memakai read model; **caching Redis** untuk master data, konfigurasi otorisasi, subtree kantor, dan agregat dashboard (dengan invalidasi saat sumber berubah).
- **Ketahanan**: Redis adalah cache/state pelengkap, bukan sumber kebenaran — kegagalan Redis menurunkan performa, tidak menghilangkan data.
- **Kualitas kode**: unit test di service layer (Go), test komponen kritis (Vitest), lint pada CI.
- **i18n**: Bahasa Indonesia & Inggris.
- **Responsif**: layout berfungsi di desktop & tablet.
- **Observability dasar**: endpoint `/health`, logging terstruktur.

---

## 9. Metrik Keberhasilan (untuk konteks portfolio)

- Semua fitur inti (§3) berjalan end-to-end dengan data nyata di DB.
- Arsitektur modular terbukti: menambah modul/fitur baru tidak mengubah modul lain.
- Cakupan test bermakna pada business logic (service layer).
- `docker compose up` + `go run` + `npm run dev` berjalan tanpa langkah manual tersembunyi.

---

## 10. Tahapan (Roadmap ringkas)

1. **Fondasi** — PRD (dokumen ini) + scaffold proyek (kerangka penuh: server jalan, DB, **Redis**, MinIO, Nuxt init).
2. **Identity & Otorisasi** — auth lokal + Google, user, peran; lapisan otorisasi configurable (RBAC + `data_scope_policies` + `field_permissions`) + audit logging terpusat (cross-cutting, dibangun awal).
3. **Master data** — provinsi/kota, jenis kantor, kantor (hierarki) + lantai/ruangan, departemen, jabatan, pegawai, vendor, brand/model, kategori aset, kategori perawatan, kategori masalah, satuan.
4. **Asset core** — CRUD aset, status, lampiran (MinIO + kompresi), import massal CSV/XLSX.
5. **Approval** — mekanisme maker-checker generik (registrasi/penghapusan aset, dll).
6. **Assignment** — check-out/in, request, riwayat.
7. **Maintenance** — jadwal, catatan, laporan kerusakan.
8. **Depreciation & Reporting** — perhitungan, read model, pengecualian valuasi, dashboard, ekspor PDF/Excel.
9. **Polish** — i18n, otorisasi config UI (field-permission semua entitas + data scope), barcode/label cetak & scan, CI.

Tiap tahap fitur akan punya spec + plan implementasi tersendiri.

---

## 11. Asumsi & Pertanyaan Terbuka

- **A1** — Storage file memakai **MinIO** (S3-compatible) sejak awal; gambar dikompres + dibuat thumbnail saat unggah.
- **A1b** — **Redis** dipakai untuk caching, session/refresh-token + denylist, rate limiting, token ber-TTL, dan backing notifikasi/lock. Bersifat pelengkap (bukan sumber kebenaran).
- **A2** — Notifikasi (maintenance & approval) bersifat in-app dulu; email menyusul.
- **A3** — Mata uang default IDR; format angka mengikuti lokal.
- **A4** — Periode depresiasi dihitung bulanan.
- **A5** — Login Google memakai OAuth2 authorization-code; kredensial (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`) disimpan sebagai env var. User Google baru mendapat peran default Staf.
- **A6** — "Min size file" diinterpretasikan sebagai **validasi batas ukuran file** (menolak file kosong/terlalu kecil dan melebihi maksimum, mis. 5 MB); ambang dikonfigurasi via env. *Koreksi bila maksud Anda berbeda.*
- **A7** — **Routing approver berjenjang**: pengajuan operasional → Manager/Kepala Unit dalam kantor yang sama; penghapusan aset & pengecualian valuasi (sensitif) → Kepala Kanwil/Superadmin. *Bisa disesuaikan.*
- **A8** — Role **Kepala Unit** & **Kepala Kanwil** masuk model sejak awal (memengaruhi data scoping & skema kantor), implementasi UI-nya dapat dilakukan bertahap.
- **~~Q1~~ (selesai)** — **Barcode wajib** untuk setiap aset (Code128 dari `asset_tag`) + label cetak/scan; QR sebagai alternatif.
- **~~Q2~~ (selesai)** — **Import massal masuk lingkup**: aset & master data via **CSV dan XLSX** (template + validasi per-baris + laporan hasil).
- **~~Q3~~ (selesai)** — Field-level permission berlaku untuk **semua entitas** (via field-registry + default per-role), bukan hanya entitas sensitif.
- **~~Q5~~ (selesai)** — Role "Employee" diganti menjadi **"Staf"**.
- **~~Q6~~ (selesai)** — Hierarki kantor **4 jenjang**: Pusat → Wilayah → Cabang → Outlet (via `parent_id`, dapat diperdalam).
- **~~Q7~~ (selesai)** — Data scope dapat dikonfigurasi **per-role + override per-modul** (`data_scope_policies`, lihat §2.2).
