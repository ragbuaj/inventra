# PRD — Inventra (Bank Fixed Asset Management System)

| | |
|---|---|
| **Produk** | Inventra |
| **Versi dokumen** | 1.1 (draft) |
| **Tanggal** | 2026-06-26 |
| **Status** | Draft — menunggu review |
| **Pemilik** | — |
| **Jenis aplikasi** | Web app manajemen **aset tetap (fixed asset) & inventaris** untuk bank |
| **Konteks domain** | Sistem manajemen aset tetap internal bank (referensi konteks: Bank BTN — BUMN/Tbk, fokus KPR, jaringan cabang luas) |
| **Stack** | Go (Gin) + PostgreSQL + Redis + MinIO · Nuxt 4 + Nuxt UI |

> **Catatan ruang lingkup (penting):** "Asset Management" di sini berarti **Manajemen Aset Tetap
> & Inventaris** (fixed asset / barang inventaris milik bank: gedung, kendaraan, perangkat IT, ATM,
> mebel, dll.) — **bukan** *investment/wealth asset management* (pengelolaan dana/portofolio
> nasabah). Seluruh dokumen ini mengacu pada pengertian fixed asset.

> **Dasar regulasi & standar (acuan requirement).** Sistem dirancang agar selaras dengan praktik &
> standar yang berlaku untuk aset tetap di Indonesia:
> - **Akuntansi:** **PSAK 16** (Aset Tetap), **PSAK 19** (Aset Takberwujud), **PSAK 48** (Penurunan
>   Nilai Aset). ⚠️ *Nomor paragraf PSAK spesifik masih perlu diverifikasi ke sumber primer (IAI/DSAK).*
> - **Penyusutan fiskal:** **PMK 72/2023** (ditetapkan 13 Jul 2023, berlaku 17 Jul 2023; mencabut
>   PMK 96/2009), pelaksana **PP 55/2022** Pasal 21(10) & 22(5) dari **Pasal 32C UU PPh** (jo. UU HPP);
>   metode garis lurus & saldo menurun per **UU PPh Pasal 11**. ✅ *Parameter kelompok harta & tarif
>   terverifikasi sumber primer — lihat **Lampiran A**.*
> - **Tata kelola & pengendalian internal bank (OJK):** **POJK 17 Tahun 2023** — Penerapan Tata Kelola
>   bagi Bank Umum (berlaku 14 Sep 2023; mencabut POJK 55/POJK.03/2016): Pasal 85 (sistem pengendalian
>   intern), Pasal 115 (*four-eyes principle* / pemisahan fungsi pada keputusan kredit), Pasal 116
>   (pemisahan fungsi dalam pengadaan); dan **POJK 18/POJK.03/2016** — Penerapan Manajemen Risiko bagi
>   Bank Umum (berlaku 22 Mar 2016, mencakup risiko operasional). ✅ *Terverifikasi — lihat **Lampiran A**.*

---

## 1. Ringkasan (Overview)

**Inventra** adalah aplikasi web untuk mengelola **aset tetap & inventaris** sebuah bank: mencatat
aset sejak perolehan, melacak penempatan & kustodian, memindahkan (mutasi) antar kantor,
menjadwalkan perawatan, menghitung penyusutan (basis komersial **dan** fiskal), melakukan
inventarisasi fisik (stock opname), hingga penghapusan/pelepasan — semuanya dengan kontrol akses
berlapis, persetujuan berjenjang, dan jejak audit menyeluruh. Aplikasi ditujukan untuk **satu
organisasi bank** dengan struktur kantor berjenjang dan beberapa peran pengguna (multi-role).

Tujuan proyek: aplikasi setingkat industri dengan **arsitektur rapi, fitur lengkap, dan kode
berkualitas**, yang **mengikuti standar & best practice industri perbankan** — sekaligus menjadi
bagian portfolio.

### 1.1 Masalah yang dipecahkan
Bank mengelola ribuan aset tetap yang tersebar di banyak kantor (pusat, wilayah, cabang, outlet).
Mengandalkan spreadsheet menimbulkan: data tersebar & tidak konsisten, tidak ada riwayat
kustodian/perpindahan, perawatan terlewat, nilai buku & penyusutan (komersial/fiskal) tidak
terhitung rapi, sulit melakukan inventarisasi fisik, dan lemahnya jejak audit/pengendalian internal
yang justru menjadi sorotan auditor & regulator. Inventra menyatukan seluruh siklus hidup aset
dalam satu sistem dengan **pemisahan fungsi (SoD)**, **persetujuan berjenjang**, dan **audit trail**.

### 1.2 Tujuan (Goals)
- G1 — Satu sumber kebenaran untuk seluruh aset tetap & inventaris bank.
- G2 — Riwayat lengkap penempatan, kustodian, dan **mutasi** aset (siapa, di mana, kapan, kondisi).
- G3 — Perawatan terjadwal dengan reminder agar tidak terlewat.
- G4 — Penyusutan otomatis **dua basis** (komersial/PSAK & fiskal/pajak) + laporan & dashboard untuk
  pengambilan keputusan dan keperluan akuntansi.
- G5 — **Inventarisasi fisik (stock opname)** terdistribusi per kantor dengan rekonsiliasi.
- G6 — Tata kelola aset: **pemisahan fungsi (SoD)**, **persetujuan berjenjang per nilai**, dan
  **jejak audit** setiap perubahan penting.
- G7 — Kontrol akses berbasis peran (RBAC) + lingkup data per-hierarki kantor + hak akses per-field.

### 1.3 Non-Goals (di luar lingkup versi ini)
- Multi-tenant (banyak organisasi dalam satu instance) — single-org dulu.
- **Integrasi langsung** ke core banking / software akuntansi pihak ketiga. *Namun* sistem
  menyediakan **output siap-jurnal** (ekspor per akun GL: beban penyusutan, laba/rugi pelepasan).
- Modul procurement/purchasing penuh (tender, PO, kontrak end-to-end) — cukup **referensi** vendor &
  nomor PO/kontrak pada aset.
- **Model revaluasi penuh** (PSAK 16 revaluation model + surplus revaluasi). Sistem memakai **model
  biaya (cost model)** + **penurunan nilai/impairment dasar** (PSAK 48); revaluasi penuh ditunda.
- ~~Aplikasi mobile native (web responsif sudah cukup; pemindaian barcode lewat web).~~
  **Dicabut di v1.2 (2026-07-18):** **aplikasi mobile companion** (Flutter) masuk lingkup sebagai
  pendamping lapangan — lihat bagian 3.11 dan ADR-0015/0016. Web tetap aplikasi utama administrasi.
- Manajemen aset keuangan/investasi (wealth/investment management) — di luar lingkup.

> **Aset takberwujud (intangible/software, PSAK 19):** **field-nya disiapkan** sejak awal
> (`asset_class = intangible`, amortisasi memakai engine penyusutan yang sama), namun **dikecualikan**
> dari fitur fisik (lokasi ruangan, barcode/label, stock opname). Workflow khusus intangible (mis.
> perpanjangan lisensi) dapat menyusul di fase berikutnya.

---

## 2. Pengguna & Peran (Roles)

Bank berstruktur **berjenjang 4 tingkat**: **Kantor Pusat → Kantor Wilayah → Kantor Cabang (Unit) →
Kantor Outlet** (pohon via `parent_id`, mendukung penambahan tingkat). *Jenis* kantor (`office_type`)
boleh banyak label, tetapi **kedalaman hierarki tetap 4 jenjang**. Peran mencerminkan jenjang ini,
dengan **lingkup akses (scoping) mengikuti subtree kantor**.

| Peran | Lingkup | Deskripsi & kemampuan inti |
|---|---|---|
| **Superadmin** | Global | Pengelola sistem penuh: kelola user/peran/field-permission, semua master data & konfigurasi, semua data & laporan. |
| **Kepala Kanwil** | Kantor Wilayah + seluruh Cabang & Outlet di bawahnya | Mengawasi & menyetujui pengajuan lintas-cabang dalam wilayahnya, lihat laporan tingkat wilayah, kelola data kantor di wilayahnya. |
| **Kepala Unit** | Satu kantor (Cabang/Outlet) + turunannya | Menyetujui pengajuan & melihat laporan dalam lingkup kantornya. |
| **Manager** (Asset Manager / Staf Aset) | Sesuai penempatan kantor | Operasional aset: CRUD aset, check-out/check-in, mutasi, kelola maintenance & stock opname, dalam lingkup kantornya. |
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
| **Ajukan mutasi aset** | ✅ | 🔵 | 🔵 | 🔵 | ❌ |
| **Kelola stock opname** | ✅ | 🔵 | 🔵 | 🔵 | ❌ |
| Ajukan pengajuan (peminjaman/kerusakan/dll) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Approve / tolak pengajuan | ✅ | 🔵 | 🔵 | 🔵² | ❌ |
| Approve pengecualian valuasi (sensitif) | ✅ | 🔵 | ❌ | ❌ | ❌ |
| **Approve penghapusan/disposal** | ✅ | 🔵³ | 🔵³ | ❌ | ❌ |
| Kelola jadwal & catatan maintenance | ✅ | ❌ | ❌ | 🔵 | ❌ |
| Konfigurasi & jalankan depresiasi (komersial + fiskal) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Lihat laporan & dashboard | ✅ | 🔵 | 🔵 | 🔵 | 🔵 (miliknya) |
| Ekspor laporan (PDF/Excel/jurnal) | ✅ | 🔵 | 🔵 | 🔵 | ❌ |
| Lihat audit trail | ✅ | 🔵 (read) | 🔵 (read) | ❌ | ❌ |

¹ Provinsi, kota, jenis kantor, kategori aset, kategori perawatan, kategori masalah, satuan, brand/model.
² Manager hanya untuk pengajuan operasional ringan; penghapusan aset & hal sensitif naik ke Kepala Unit/Kanwil/Superadmin.
³ Jenjang approver penghapusan mengikuti **limit otorisasi per nilai** (§2.4) — makin besar nilai aset, makin tinggi jenjang yang wajib menyetujui.

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

### 2.4 Pemisahan Fungsi (SoD) & Limit Otorisasi Berjenjang

Tata kelola aset bank menuntut lebih dari sekadar RBAC. Dua kontrol tambahan:

**(a) Segregation of Duties (Pemisahan Fungsi).** Peran **pengaju (maker)**, **pencatat**,
**kustodian** (pemegang aset), dan **penyetuju (approver)** atas satu transaksi **tidak boleh orang
yang sama**. Minimal yang ditegakkan sistem: pengaju **tidak boleh** menyetujui pengajuannya sendiri
(§3.6), dan approver harus berbeda identitas dari maker. Pemisahan kustodian↔pencatat didukung lewat
pemodelan `employee` (kustodian) terpisah dari `user` (aktor sistem).

**(b) Limit Otorisasi per Nilai (`approval_thresholds`).** Untuk transaksi sensitif yang nilainya
bervariasi (penghapusan, registrasi/pengadaan, mutasi antar-wilayah), **jenjang approver yang wajib
naik mengikuti nilai aset**. Disimpan di tabel **`approval_thresholds`** (`request_type`,
`amount_from`, `amount_to`, `required_level`, `step_order`, `active`) — **dapat diubah Superadmin
tanpa deploy**. Model: threshold menentukan **jenjang tertinggi** yang wajib menyetujui; rantai
persetujuan **berurutan** (maker → checker → approver berlapis).

**Default ter-seed (⚠️ placeholder — angka final mengikuti kebijakan bank):**

*Penghapusan / Disposal* (basis nilai buku):

| Nilai aset | Approver tertinggi wajib |
|---|---|
| ≤ Rp 5.000.000 | Kepala Cabang/Unit |
| > Rp 5 jt – Rp 50.000.000 | + Kepala Kanwil |
| > Rp 50.000.000 | + Kantor Pusat (Superadmin) |

*Registrasi / Pengadaan aset baru* (basis nilai perolehan):

| Nilai perolehan | Approver tertinggi wajib |
|---|---|
| ≤ Rp 10.000.000 | Kepala Cabang/Unit |
| > Rp 10 jt – Rp 100.000.000 | + Kepala Kanwil |
| > Rp 100.000.000 | + Kantor Pusat |

*Mutasi aset*: dalam subtree kantor sendiri → Kepala Unit asal; **antar-wilayah** → Kepala Kanwil
kedua sisi (atau Pusat). *Pengecualian valuasi* → Kepala Kanwil/Superadmin.

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
- **FR-2.1** CRUD aset dengan atribut: kode/tag unik, nama, **kelas aset** (`tangible`/`intangible`), kategori, lokasi, status, nomor seri, tanggal beli, harga beli, pemasok, garansi, spesifikasi (fleksibel/JSON), foto, catatan, **sumber dana**, **nomor PO/kontrak** (opsional), **nomor BAST perolehan** (§3.10).
- **FR-2.2** **Kode aset (`asset_tag`) unik** dibuat otomatis dengan format **`<kode_kantor>-<kode_kategori>-<tahun_beli>-<sequence>`**, di mana `sequence` = **5 digit** yang berjalan **per kantor & kategori** dan **direset tiap tahun**. Contoh: `JKT01-ELK-2026-00001`. Validasi unik; detail generator di [DATABASE.md §4.7](DATABASE.md).
- **FR-2.3** **Status aset**: `available`, `assigned`, `under_maintenance`, `in_transfer`, `retired`, `disposed`, `lost`. Perubahan status mengikuti aturan transisi (lihat §5).
- **FR-2.4** Master data **Kategori** dengan nilai default akuntansi/pajak (lihat FR-7.4 yang diperkaya): metode & masa manfaat penyusutan **komersial** dan **fiskal**, **akun GL**, **golongan/kelompok pajak**, **batas kapitalisasi**.
- **FR-2.5** Master data **Lokasi berjenjang**: **Kantor → Lantai → Ruangan**. Aset (tangible) menunjuk ke Ruangan (yang mewarisi Lantai & Kantor). Aset intangible tidak menunjuk lokasi fisik. Lihat daftar master data lengkap di §3.7.
- **FR-2.6** Upload **foto/lampiran** aset disimpan di **MinIO** (S3-compatible) via Storage interface.
  - **Validasi**: tipe file di-whitelist (mis. jpg/png/webp/pdf), tolak file kosong/korup, dan **batas ukuran** (maks. mis. 5 MB; tolak di bawah ambang minimal yang wajar). Ambang dikonfigurasi via env.
  - **Kompresi saat simpan**: gambar di-resize ke dimensi maks. & di-re-encode (mis. WebP/JPEG mutu ~80%) sebelum disimpan, untuk hemat storage; thumbnail dibuat untuk tampilan daftar.
- **FR-2.7** Pencarian, filter (kategori/lokasi/status/kelas aset), sort, dan pagination pada daftar aset.
- **FR-2.8** Detail aset menampilkan: info, riwayat penugasan, **riwayat mutasi**, riwayat maintenance, dan jadwal depresiasi (komersial & fiskal) — **dengan field dibatasi sesuai §2.3**.
- **FR-2.9** **Registrasi & penghapusan aset lewat pengajuan + approval** (maker-checker, berjenjang per nilai §2.4) — lihat §3.6.
- **FR-2.10** Aset dapat ditandai **dikecualikan dari penghitungan kekayaan/valuasi** lewat pengajuan + approval (§3.6); aset terkecuali tidak dihitung dalam total nilai aset di laporan/dashboard, namun tetap terdata.
- **FR-2.11** **Import massal aset** via **CSV & XLSX**: unduh template, unggah berkas, **validasi per-baris** (tipe data, referensi master data, tag unik), dan **laporan hasil** (baris sukses vs gagal + alasan). Baris valid dibuat; baris gagal dilewati & dapat diunduh untuk dikoreksi. Import mengikuti aturan otorisasi & approval yang berlaku (§3.6) sesuai konfigurasi.
- **FR-2.12** **Barcode per aset**: setiap aset **tangible** otomatis memiliki **barcode** (mis. Code128) yang di-encode dari `asset_tag`. Barcode dapat **dicetak sebagai label** (tunggal maupun massal/batch) dan **dipindai** untuk look-up / pencarian aset cepat (termasuk saat stock opname §3.9). **QR code** juga disediakan sebagai alternatif label.
- **FR-2.13** **Batas kapitalisasi**: pengadaan dengan nilai **di bawah batas kapitalisasi** (config global, override per-kategori — §3.7) ditandai **dibebankan (expense)**, bukan dikapitalisasi sebagai aset tetap & tidak disusutkan. Default placeholder ⚠️ **Rp 1.000.000** (dapat diubah).

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

### 3.5 Depresiasi, Penurunan Nilai & Pelaporan
- **FR-5.1** **Penyusutan dua basis** per aset/kategori:
  - **Komersial (PSAK 16)** — metode (`straight_line` garis lurus, `declining_balance` saldo menurun), masa manfaat (bulan), nilai residu (salvage value).
  - **Fiskal (pajak)** — metode & masa manfaat mengikuti **kelompok harta** (Kelompok 1–4 / bangunan permanen·non-permanen) per **PMK 72/2023** & UU PPh Pasal 11. Parameter (masa manfaat & tarif garis lurus/saldo menurun) **terverifikasi** — lihat **Lampiran A**. Bangunan hanya metode **garis lurus**; saldo menurun khusus aset **selain bangunan**.
  - Aset **intangible** memakai jalur **amortisasi** (engine yang sama, istilah berbeda) per PSAK 19.
- **FR-5.2** Hitung **nilai buku (book value)** aset pada periode berjalan secara otomatis (basis komersial; basis fiskal dihitung paralel untuk keperluan pajak).
- **FR-5.3** **Jadwal depresiasi** per aset (per periode: nilai awal, beban penyusutan, nilai akhir) untuk **kedua basis** — disajikan via **read model** khusus agar laporan cepat.
- **FR-5.4** **Penurunan nilai (impairment, PSAK 48)**: pencatatan **write-down satu kali** + alasan bila nilai tercatat melampaui nilai terpulihkan (mis. aset rusak/usang sebelum habis umur); nilai buku turun & jadwal penyusutan disesuaikan. (Model revaluasi penuh di luar lingkup — §1.3.)
- **FR-5.5** **Dashboard**: total aset, nilai aset (perolehan vs buku), aset per status/kategori/lokasi/kelas, aset overdue, maintenance jatuh tempo, biaya maintenance.
- **FR-5.6** **Laporan**: daftar aset + nilai buku, **laporan penyusutan per periode (komersial & fiskal)**, laporan utilisasi/penugasan, laporan biaya maintenance, **laporan mutasi**, **berita acara/hasil stock opname**, **laporan penghapusan/pelepasan (laba-rugi)**.
- **FR-5.7** **Ekspor** laporan ke **PDF** (layout cetak rapi), **Excel (.xlsx)** (data tabular), dan **output siap-jurnal** (rekap per akun GL: beban penyusutan, akumulasi, laba/rugi pelepasan) untuk diposting ke sistem akuntansi — *tanpa* integrasi langsung (§1.3).
- **FR-5.8** **Audit trail menyeluruh**: setiap operasi tulis (create/update/delete) pada **seluruh entitas/tabel** dicatat ke `audit_logs` — aktor, entitas, ID, aksi, perubahan (diff before/after), dan waktu. Dapat ditelusuri per entitas maupun per user. Diterapkan secara terpusat (mis. lewat hook/decorator di repository/service), bukan per-handler manual.

### 3.6 Pengajuan & Persetujuan (Approval / Maker-Checker)

Beberapa aksi sensitif tidak langsung dieksekusi, melainkan melalui **pengajuan → review → approve/tolak**. Satu mekanisme generik melayani beberapa jenis:

- **FR-6.1** Jenis pengajuan: **Registrasi aset baru**, **Penghapusan/disposal aset**, **Mutasi aset** (§3.8), **Peminjaman** (§3.3), **Laporan kerusakan/maintenance** (§3.4), dan **Pengecualian aset dari penghitungan kekayaan** (§3.7-valuasi).
- **FR-6.2** Pengaju (maker) membuat request berisi payload + alasan; status awal `pending`.
- **FR-6.3** Approver (checker) **approve** atau **tolak** dengan catatan. **Jenjang approver mengikuti `approval_thresholds` (§2.4)** — untuk transaksi bernilai, rantai persetujuan berlapis sesuai nilai aset & lingkup kantor. Approve (di jenjang terakhir) memicu eksekusi aksi sebenarnya (aset benar-benar dibuat/dihapus/dimutasi, atau flag pengecualian diset). Tolak menutup request tanpa efek.
- **FR-6.4** Pengaju tidak boleh menyetujui pengajuannya sendiri (**segregation of duties**, §2.4); tiap approver dalam rantai harus berbeda identitas dari maker dan dari approver sebelumnya.
- **FR-6.5** Daftar pengajuan dengan filter status & jenis; notifikasi in-app ke approver saat ada pengajuan baru/giliran, dan ke pengaju saat diputuskan.
- **FR-6.6** Setiap keputusan tercatat di audit trail (§5.8) — termasuk tiap langkah rantai persetujuan.

### 3.7 Master Data & Pengecualian Valuasi

- **FR-7.1** **Kantor (office)** dengan **hierarki 4 jenjang** (`parent_id`: **Pusat → Wilayah → Cabang/Unit → Outlet**) dan **jenis kantor** (`office_type`, boleh banyak label). Tiap kantor menunjuk **Provinsi** & **Kota**, dan punya **cost center / kode unit kerja**.
- **FR-7.2** **Lokasi fisik berjenjang** di dalam kantor: **Kantor → Lantai (floor) → Ruangan (room)**; aset tangible menunjuk ke Ruangan.
- **FR-7.3** Master data **Pegawai** (employee) sebagai custodian aset — terpisah dari **User** (akun login). Satu user dapat ditautkan ke satu pegawai (`employee_id`). Tidak semua pegawai punya akun login. Pegawai menunjuk Departemen, Jabatan, dan Kantor.
- **FR-7.4** Master data referensi:
  - **Jenis kantor** (office types)
  - **Provinsi** & **Kota** (kota menunjuk provinsi)
  - **Departemen/Divisi** dan **Jabatan**
  - **Vendor/Pemasok** (pembelian & servis)
  - **Brand & Model** (normalisasi spesifikasi aset)
  - **Kategori aset** — diperkaya dengan atribut akuntansi/pajak: `default_depreciation_method` & `default_useful_life_months` (komersial), `default_fiscal_group` & `default_fiscal_life_months` (fiskal), `default_salvage_rate`, **`gl_account_code`** (akun GL/COA), **`tax_group`** (Kelompok 1–4 / bangunan), **`capitalization_threshold`** (override batas kapitalisasi), `asset_class` default (tangible/intangible).
  - **Kategori perawatan** (maintenance category — mis. Servis Rutin, Kalibrasi, Perbaikan)
  - **Kategori masalah** (problem category — dipakai saat laporan kerusakan, mis. Hardware, Listrik, Fisik)
  - **Satuan** (unit of measure — mis. Unit, Pcs, Set)
- **FR-7.5** Semua master data mendukung CRUD oleh peran berwenang (§2.1), pencarian, dan status aktif/nonaktif.
- **FR-7.5b** **Import massal master data** (CSV/XLSX) untuk entitas bervolume besar — terutama **Pegawai** dan **Kantor**, serta provinsi/kota — dengan template, validasi per-baris, dan laporan hasil (sama seperti FR-2.11).
- **FR-7.6** **Pengecualian valuasi**: aset yang disetujui dikecualikan (§3.6) diberi flag `excluded_from_valuation` + alasan; laporan/dashboard total kekayaan mengabaikan aset ini namun tetap menampilkannya sebagai "terkecuali".
- **FR-7.7** **Limit otorisasi (`approval_thresholds`)** & **batas kapitalisasi** dapat dikelola Superadmin sebagai konfigurasi (§2.4, FR-2.13).

### 3.8 Mutasi Aset (Transfer Antar-Kantor)

Berbeda dari *assignment* (check-out ke pegawai), **mutasi** adalah **perpindahan aset antar kantor/unit** — krusial pada bank dengan banyak cabang.

- **FR-8.1** Pengajuan **mutasi** aset dari kantor asal ke kantor tujuan (alasan, tanggal, kondisi).
- **FR-8.2** Persetujuan mengikuti `approval_thresholds` (§2.4): dalam subtree sendiri → Kepala Unit asal; **antar-wilayah** → Kepala Kanwil kedua sisi / Pusat.
- **FR-8.3** Saat mutasi disetujui & **diterima** di tujuan, `office_id` (dan ruangan) aset diperbarui; status sementara `in_transfer` selama proses, kembali `available`/`assigned` setelah diterima.
- **FR-8.4** **BAST mutasi** (§3.10) tercatat; **riwayat mutasi** per aset (asal, tujuan, tanggal, pelaku, dokumen).
- **FR-8.5** Penegakan scope: pengaju & penerima harus berada dalam lingkup kantor yang relevan.

### 3.9 Stock Opname (Inventarisasi Fisik)

- **FR-9.1** Membuat **sesi stock opname** per kantor/lingkup & periode (mis. tahunan), dengan daftar aset yang seharusnya ada (snapshot dari register).
- **FR-9.2** **Pencocokan fisik**: tiap aset ditandai hasilnya — `found` (ditemukan), `not_found` (tidak ditemukan), `damaged` (rusak), `misplaced` (salah lokasi). Pemindaian **barcode/QR** mempercepat pencocokan.
- **FR-9.3** **Selisih (variance)**: aset tercatat tapi tak ditemukan, atau aset fisik tak terdaftar, dirangkum sebagai temuan.
- **FR-9.4** **Rekonsiliasi & tindak lanjut**: temuan dapat memicu pengajuan (mis. penghapusan untuk hilang, mutasi untuk salah lokasi, maintenance untuk rusak).
- **FR-9.5** **Laporan/Berita Acara stock opname** (PDF/Excel) per sesi, dengan ringkasan & daftar selisih. Tercatat di audit trail.
- **FR-9.6** Penegakan scope: sesi & itemnya terbatas pada lingkup kantor pelaksana.

### 3.10 Dokumen & BAST (Berita Acara Serah Terima)

- **FR-10.1** Transaksi **perolehan**, **mutasi**, dan **penghapusan/disposal** memiliki **nomor BAST** dan dokumen pendukung yang dilampirkan (di MinIO).
- **FR-10.2** Entitas `asset_documents` menautkan dokumen ke aset & ke transaksi terkait (jenis, nomor, tanggal, pihak, berkas).
- **FR-10.3** Dokumen mengikuti hak akses & scope aset terkait; perubahan tercatat di audit trail.

### 3.11 Aplikasi Mobile Companion (v1.2)

Scope aplikasi **mobile companion** (Flutter, field companion) dibuka pada v1.2. **Dokumentasinya
dipisah** dari PRD web agar mudah dibaca — kebutuhan lengkap klien mobile ada di
**[`docs/mobile/PRD.md`](mobile/PRD.md)** (FR-M1 sampai FR-M6: auth & sesi, scan aset, approval
on-the-go, notifikasi push FCM, stock opname offline-first, profil & preferensi). Keputusan
arsitektur di `docs/mobile/adr/` (ADR-0015 Flutter, ADR-0016 offline sync — penomoran ADR tetap
satu urutan global); design brief + prompt kit mockup di `docs/mobile/DESIGN_BRIEF.md` (hasil di
`docs/mobile/design/`); roadmap fase M0-M6 di `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`.

Prinsip yang mengikat dari PRD web: web tetap aplikasi utama administrasi; mobile tidak membuat
pengajuan modul non-opname (hanya memutus approval-nya); dan **semua otorisasi** (permission, data
scope, field permission, SoD, `approval_thresholds`) tetap ditegakkan server.

---

## 4. User Stories (contoh utama)

- Sebagai **Superadmin**, saya menambah user baru, menetapkan peran & kantor penempatannya, agar tim mengakses sistem sesuai wewenang & lingkup.
- Sebagai **Manager**, saya mengajukan registrasi laptop baru dengan kategori, ruangan, harga beli, dan nomor BAST, agar aset tercatat resmi setelah disetujui sesuai jenjang nilainya.
- Sebagai **Manager**, saya melakukan check-out laptop ke seorang pegawai dengan tanggal jatuh tempo, agar kepemilikan terlacak.
- Sebagai **Manager**, saya mengajukan **mutasi** sebuah genset dari Cabang A ke Cabang B di wilayah lain, agar perpindahan tercatat dengan BAST dan disetujui kedua Kanwil.
- Sebagai **Manager**, saya menjalankan **stock opname** tahunan kantor saya dengan memindai barcode aset, agar selisih fisik vs catatan terdeteksi.
- Sebagai **Staf**, saya mengajukan peminjaman proyektor, agar bisa dipakai presentasi setelah disetujui.
- Sebagai **Staf**, saya melaporkan AC ruangan rusak (kategori masalah: Listrik), agar dijadwalkan perbaikan.
- Sebagai **Kepala Unit**, saya menyetujui pengajuan peminjaman di unit saya, agar aset terkendali.
- Sebagai **Kepala Kanwil**, saya menyetujui **penghapusan** aset bernilai besar sesuai limit otorisasi, agar pelepasan terkendali dan tercatat laba/ruginya.
- Sebagai **Superadmin**, saya melihat laporan penyusutan akhir tahun (komersial & fiskal) dan rekap siap-jurnal, agar nilai buku & beban penyusutan diketahui untuk keperluan keuangan dan pajak.

---

## 5. Aturan Status Aset (State Machine)

```
available ──checkout──▶ assigned
assigned ──checkin──▶ available
available/assigned ──start maintenance──▶ under_maintenance
under_maintenance ──complete──▶ available
available ──start transfer──▶ in_transfer ──receive──▶ available (office berubah)
available/assigned ──dispose (approved)──▶ disposed
(any) ──mark lost──▶ lost
```

- Aset `assigned` harus di-check-in dulu sebelum bisa di-**mutasi** atau **dihapus** (kecuali `lost`).
- Aset `under_maintenance` atau `in_transfer` tidak bisa di-check-out.
- **Penghapusan/disposal** hanya lewat approval berjenjang (§2.4, §3.6); status akhir `disposed` (mencatat metode pelepasan, nilai jual, laba/rugi).
- Transisi tidak valid ditolak oleh service layer.

---

## 6. Model Data (high-level)

Entitas inti dan relasi (detail kolom final ditentukan saat migrasi DB):

**Identity & RBAC**
- **users** (id, employee_id?, office_id, name, email, password_hash?, google_id?, avatar_url?, role[superadmin/kepala_kanwil/kepala_unit/manager/staf], status, timestamps) — `password_hash` & `google_id` nullable; `office_id` = kantor penempatan (dasar scoping hierarki)
- **field_permissions** (id, entity, field, role, can_view, can_edit)
- **data_scope_policies** (id, role, module?, scope_level[global/office_subtree/office/own]) — `module` null = default per-role (semua modul); terisi = **override per-modul** (menimpa default). Unik per (role, module).
- **approval_thresholds** (id, request_type, amount_from, amount_to?, required_level[office/office_subtree/wilayah/pusat], step_order, active) — limit otorisasi berjenjang per nilai (§2.4)

**Master data — referensi & geografi**
- **provinces** (id, name, code)
- **cities** (id, province_id, name, code)
- **office_types** (id, name) — jenis kantor (boleh banyak label; tetap memetakan 4 jenjang)
- **departments** (id, name, code) · **positions** (id, name) — jabatan
- **vendors** (id, name, contact, address)
- **brands** (id, name) · **models** (id, brand_id, name)
- **categories** (id, name, code, parent_id?, asset_class[tangible/intangible], default_depreciation_method, default_useful_life_months, default_fiscal_group, default_fiscal_life_months, default_salvage_rate, gl_account_code, tax_group, capitalization_threshold?) — kategori aset diperkaya (§3.7)
- **maintenance_categories** (id, name) · **problem_categories** (id, name)
- **units** (id, name, symbol) — satuan

**Master data — struktur kantor & orang**
- **offices** (id, parent_id?, office_type_id, province_id, city_id, name, code, cost_center_code?, address) — hierarki Pusat→Wilayah→Cabang/Unit→Outlet via `parent_id`
- **floors** (id, office_id, name, level) · **rooms** (id, floor_id, name, code)
- **employees** (id, nip/code, name, email?, department_id?, position_id?, office_id, status) — custodian aset

**Aset & operasional**
- **assets** (id, asset_tag, name, asset_class[tangible/intangible], category_id, brand_id?, model_id?, room_id?, office_id, unit_id?, status, serial_number, purchase_date, purchase_cost, capitalized[bool], vendor_id?, po_number?, funding_source?, warranty_expiry, specifications JSONB, depreciation_method, useful_life_months, salvage_value, fiscal_group?, fiscal_life_months?, accumulated_depreciation, book_value, impairment_loss?, current_holder_employee_id?, excluded_from_valuation, acquisition_bast_no?, notes, timestamps) — `office_id` (diturunkan dari ruangan untuk tangible) dipakai untuk scoping
- **asset_attachments** (id, asset_id, kind[photo/document], object_key, thumbnail_key?, size, mime, created_at) — file di MinIO
- **asset_documents** (id, asset_id, doc_type[bast_acquisition/bast_transfer/bast_disposal/other], doc_no, doc_date, counterparty?, object_key?, related_request_id?, created_at) — BAST & dokumen resmi (§3.10)
- **assignments** (id, asset_id, employee_id, assigned_by_id, checkout_date, due_date, checkin_date, condition_out, condition_in, status, notes)
- **asset_transfers** (id, asset_id, from_office_id, to_office_id, requested_by_id, approved_by_id?, status[pending/approved/in_transfer/received/rejected], shipped_date?, received_date?, bast_no?, reason, notes) — mutasi antar-kantor (§3.8)
- **stock_opname_sessions** (id, office_id, period, status[open/counting/reconciling/closed], started_by_id, started_at, closed_at?) · **stock_opname_items** (id, session_id, asset_id, expected[bool], result[found/not_found/damaged/misplaced], counted_by_id?, counted_at?, note) — inventarisasi fisik (§3.9)
- **maintenance_schedules** (id, asset_id, maintenance_category_id?, interval_months, last_done_date, next_due_date)
- **maintenance_records** (id, asset_id, maintenance_category_id?, problem_category_id?, type, status, scheduled_date, completed_date, cost, vendor_id?, performed_by, description, reported_by_id?)
- **depreciation_entries** (read model) (id, asset_id, basis[commercial/fiscal], period, opening_value, depreciation_amount, closing_value) — **dua basis** (komersial/PSAK & fiskal/pajak)
- **disposals** (id, asset_id, method[sale/auction/donation/write_off], disposal_date, proceeds, book_value_at_disposal, gain_loss, bast_no?, approved_by_id, request_id?) — penghapusan/pelepasan (§3.6, status aset `disposed`)

**Approval & audit**
- **requests** (id, type[asset_create/asset_disposal/asset_transfer/assignment/maintenance/valuation_exclusion], office_id, amount?, payload JSONB, reason, status[pending/approved/rejected], current_step, requested_by_id, decided_by_id?, decision_note?, timestamps) — maker-checker generik berjenjang (§3.6); `office_id` & `amount` untuk routing approver per nilai (§2.4)
- **request_approvals** (id, request_id, step_order, approver_role/level, approver_id?, decision[pending/approved/rejected], note?, decided_at?) — jejak tiap langkah rantai persetujuan
- **audit_logs** (id, actor_id, entity_type, entity_id, action[create/update/delete], changes JSONB, created_at) — mencakup **seluruh tabel**
- **import_jobs** (id, target[asset/employee/office/…], format[csv/xlsx], filename, status[pending/processing/completed/failed], total_rows, success_rows, failed_rows, error_report_key?, created_by_id, created_at) — melacak proses import massal; berkas error tersimpan di MinIO, progres dapat ditembolok di Redis

Relasi kunci: `provinces` 1—N `cities`; `offices` self-ref `parent_id`, N—1 `office_types`/`provinces`/`cities`, 1—N `floors` 1—N `rooms`; `assets` N—1 `rooms`/`offices`/`categories`/`brands`/`models`/`vendors`/`units`; `assets` 1—N `assignments`/`asset_transfers`/`maintenance_records`/`depreciation_entries`/`asset_attachments`/`asset_documents`/`stock_opname_items`; `employees` N—1 `offices`/`departments`/`positions`, 1—N `assignments`; `users` N—1 `employees`/`offices`; `requests` 1—N `request_approvals`.

---

## 7. Arsitektur Teknis (ringkas)

**Pola: Modular Monolith + Clean Architecture** (Opsi A). Satu service Go, modul berkomunikasi via service interface + **domain event in-process** (bukan message broker).

**Modul backend:**
```
identity     → auth (lokal + Google), user, RBAC, field_permissions, data_scope, approval_thresholds
masterdata   → kantor(hierarki)/lantai/ruangan, provinsi/kota, jenis kantor, departemen, jabatan, pegawai,
               vendor, brand/model, kategori aset (akun GL/golongan pajak/kapitalisasi), kategori perawatan, kategori masalah, satuan
asset        → katalog, tag, barcode/label (Code128 + QR), lampiran & dokumen/BAST (MinIO), status, kelas (tangible/intangible),
               valuasi/pengecualian, kapitalisasi, import massal (CSV/XLSX)
assignment   → check-out/in, riwayat
transfer     → mutasi aset antar-kantor + BAST + riwayat
stockopname  → sesi inventarisasi fisik, pencocokan (scan), rekonsiliasi, berita acara
maintenance  → jadwal, catatan, reminder
depreciation → perhitungan dua basis (komersial PSAK + fiskal pajak), amortisasi intangible, impairment, read model
disposal     → penghapusan/pelepasan (metode, laba/rugi) — via approval
approval     → mekanisme pengajuan-persetujuan generik (maker-checker) berjenjang per nilai (approval_thresholds)
reporting    → dashboard, laporan (termasuk mutasi/opname/disposal), ekspor PDF/Excel + output siap-jurnal (GL)
import       → import massal CSV/XLSX (aset & master data): template, validasi per-baris, laporan hasil
```

**Concern lintas-modul (cross-cutting):** audit logging menyeluruh, **data scoping yang dapat dikonfigurasi** (`data_scope_policies`), field-level permission (`field_permissions`), dan **persetujuan berjenjang per nilai** (`approval_thresholds`) diterapkan sebagai middleware/decorator terpusat — bukan diulang di tiap handler. Bersama-sama membentuk lapisan **otorisasi & kontrol yang dapat dikonfigurasi Superadmin** (per-aksi · per-baris/lingkup · per-field · per-nilai) plus **pemisahan fungsi (SoD)**.

**Lapisan tiap modul:** `domain.go` (entity + interface) → `service.go` (business logic) → `repository.go` (sqlc) → `handler.go` (Gin, tipis) → `routes.go` → `events.go`.

**Aturan:** modul tidak saling impor repository; interface didefinisikan di sisi consumer; wiring eksplisit di `cmd/api/main.go`.

**Stack:**

| Lapisan | Teknologi |
|---|---|
| Bahasa/Framework | Go 1.25 · Gin |
| Database | PostgreSQL 16 |
| Cache & state | **Redis 7** (caching, session/token, rate limiting, token TTL, transport notifikasi via Streams — lihat ADR-0014) |
| Query | sqlc |
| Migrasi | golang-migrate |
| Auth | JWT (access + refresh) + OAuth2 (Google login) |
| File storage | **MinIO** (S3-compatible) via Storage interface; kompresi/resize gambar saat unggah |
| Ekspor | PDF (mis. maroto/gofpdf) + Excel `.xlsx` (excelize) + rekap siap-jurnal (GL) |
| Frontend | Nuxt 4 (Vue 3 + Vite) · Nuxt UI · Pinia · VeeValidate + Zod |
| i18n | @nuxtjs/i18n (ID/EN) |
| DevOps | Docker Compose · GitHub Actions |
| Mobile (v1.2) | Flutter (Dart 3) · Riverpod · Dio · drift (SQLite offline) · mobile_scanner — companion lapangan, folder `mobile/` (ADR-0015) |

**Redis (cache & state):** dipakai untuk —
- **Caching**: master data & referensi (provinsi/kota/kategori/dll), **konfigurasi otorisasi** (`field_permissions`, `data_scope_policies`, `approval_thresholds`), **subtree kantor** (daftar `descendant_ids` per kantor — mahal dihitung), dan agregat dashboard/laporan. Cache **di-invalidasi** saat data sumber berubah.
- **Session/token**: penyimpanan **refresh token** + **denylist** access token (mendukung logout & pencabutan sesi), serta data sesi ringan.
- **Rate limiting**: batasi percobaan **login** (anti brute-force) dan throttle API per user/IP.
- **Token ber-TTL**: token **reset password**, **verifikasi email**, dan OTP (bila ada) dengan kedaluwarsa otomatis.
- **Notifikasi (transport, bukan penyimpanan)**: sejak modul notifikasi (ADR-0014), notifikasi in-app disimpan **permanen di PostgreSQL** (`notification.notifications`, feed per-user). Redis dipakai sebagai **transport** lewat **Redis Streams** dalam pola **transactional outbox** — event bisnis ditulis se-transaksi ke `notification.outbox`, relay mem-publish ke stream, consumer group mem-fan-out ke feed. Kehilangan Redis tidak menghilangkan notifikasi (relay mengirim ulang dari outbox). Ini menyelaraskan notifikasi dengan prinsip "Redis bukan sumber kebenaran" di bawah.
- **Lock**: **distributed lock** untuk operasi sensitif (mis. penjadwal reminder/sweeper notifikasi, penutupan periode depresiasi) memakai **Postgres advisory lock** (`pg_advisory_xact_lock`, lihat ADR-0010 & ADR-0014), bukan lock Redis.

> Catatan: Redis bersifat pelengkap, bukan sumber kebenaran. Kehilangan Redis tidak menyebabkan kehilangan data (PostgreSQL tetap otoritatif); sistem tetap berjalan dengan degradasi performa.

**Frontend (Nuxt):** layout `admin` (Superadmin/Kepala/Manager) & `app` (Staf), route middleware untuk RBAC + scoping, halaman: dashboard, aset (list/detail/form), penugasan, **mutasi**, **stock opname**, maintenance, pengajuan/approval, laporan, master data, user, profil.

**Mobile (Flutter, v1.2):** aplikasi companion lapangan di folder `mobile/` — konsumen `/api/v1`
yang sama (endpoint tambahan hanya push-token FCM dan batch sync opname); halaman: login, scan,
detail aset, stock opname (offline-first), approval, notifikasi, profil/sesi (bagian 3.11).

---

## 8. Kebutuhan Non-Fungsional

- **Keamanan**: hashing password, JWT, dan **otorisasi berlapis yang dapat dikonfigurasi** — RBAC per-aksi, **data scope per-baris (`data_scope_policies`)**, **field-level permission**, dan **persetujuan berjenjang per nilai (`approval_thresholds`)** + **pemisahan fungsi (SoD)** — semuanya ditegakkan di server (bukan hanya UI), plus validasi input.
- **Kepatuhan & akuntansi**: penyusutan dua basis (PSAK 16 komersial + fiskal pajak), amortisasi intangible (PSAK 19), penurunan nilai (PSAK 48), output siap-jurnal per akun GL. ⚠️ *Parameter regulasi (PSAK/PMK/POJK) diverifikasi ke sumber primer.*
- **Auditability**: seluruh operasi tulis tercatat di `audit_logs` (semua tabel) dengan diff; jejak rantai persetujuan tiap langkah.
- **Penanganan file**: validasi tipe & ukuran (min/maks), kompresi + thumbnail gambar, disimpan di MinIO; URL akses melalui presigned/proxy yang menghormati hak akses.
- **Performa**: daftar memakai pagination + index DB; laporan memakai read model; **caching Redis** untuk master data, konfigurasi otorisasi, subtree kantor, dan agregat dashboard (dengan invalidasi saat sumber berubah).
- **Ketahanan**: Redis adalah cache/state pelengkap, bukan sumber kebenaran — kegagalan Redis menurunkan performa, tidak menghilangkan data.
- **Kualitas kode**: unit test di service layer (Go), test komponen kritis (Vitest), lint pada CI.
- **i18n**: Bahasa Indonesia & Inggris.
- **Responsif**: layout berfungsi di desktop & tablet.
- **Observability dasar**: endpoint `/health`, logging terstruktur (slog), korelasi request-id FE↔BE.

---

## 9. Metrik Keberhasilan (untuk konteks portfolio)

- Semua fitur inti (§3) berjalan end-to-end dengan data nyata di DB.
- Arsitektur modular terbukti: menambah modul/fitur baru tidak mengubah modul lain.
- Cakupan test bermakna pada business logic (service layer).
- Penyusutan dua basis & laporan siap-jurnal terbukti benar pada contoh aset.
- `docker compose up` + `go run` + `npm run dev` berjalan tanpa langkah manual tersembunyi.

---

## 10. Tahapan (Roadmap ringkas)

1. **Fondasi** — PRD (dokumen ini) + scaffold proyek (kerangka penuh: server jalan, DB, **Redis**, MinIO, Nuxt init).
2. **Identity & Otorisasi** — auth lokal + Google, user, peran; lapisan otorisasi configurable (RBAC + `data_scope_policies` + `field_permissions` + `approval_thresholds`) + audit logging terpusat (cross-cutting, dibangun awal).
3. **Master data** — provinsi/kota, jenis kantor, kantor (hierarki) + lantai/ruangan, departemen, jabatan, pegawai, vendor, brand/model, **kategori aset (akun GL/golongan pajak/kapitalisasi)**, kategori perawatan, kategori masalah, satuan.
4. **Asset core** — CRUD aset (tangible + field intangible), status, kapitalisasi, lampiran & dokumen/BAST (MinIO + kompresi), import massal CSV/XLSX.
5. **Approval** — mekanisme maker-checker generik **berjenjang per nilai** (registrasi/penghapusan/mutasi, dll).
6. **Assignment** — check-out/in, request, riwayat.
7. **Mutasi & Stock Opname** — mutasi antar-kantor + BAST; sesi inventarisasi fisik + rekonsiliasi.
8. **Maintenance** — jadwal, catatan, laporan kerusakan.
9. **Depreciation & Reporting** — perhitungan **dua basis** (komersial + fiskal) + amortisasi + impairment, read model, disposal/laba-rugi, pengecualian valuasi, dashboard, ekspor PDF/Excel + **output siap-jurnal**.
10. **Polish** — i18n, otorisasi config UI (field-permission + data scope + thresholds), barcode/label cetak & scan, CI.
11. **Mobile companion (v1.2)** — aplikasi Flutter pendamping lapangan, fase M0 (fondasi) sampai M6 (rilis internal): scan aset, approval on-the-go, push FCM, stock opname offline-first. Rincian: `docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`.

Tiap tahap fitur akan punya spec + plan implementasi tersendiri.

---

## 11. Asumsi & Pertanyaan Terbuka

- **A1** — Storage file memakai **MinIO** (S3-compatible) sejak awal; gambar dikompres + dibuat thumbnail saat unggah.
- **A1b** — **Redis** dipakai untuk caching, session/refresh-token + denylist, rate limiting, dan token ber-TTL. Bersifat pelengkap (bukan sumber kebenaran). **Diperbarui (ADR-0014):** notifikasi in-app **tidak** lagi disimpan di Redis — sumber kebenarannya PostgreSQL; Redis hanya **transport** (Redis Streams, pola outbox). Distributed lock memakai Postgres advisory lock, bukan Redis.
- **A2** — Notifikasi (maintenance & approval) bersifat in-app dulu; email menyusul. **Terwujud (ADR-0014):** modul notifikasi in-app dibangun dengan empat jenis (`approval_pending`, `approval_decided`, `maintenance_due`, `asset_returned`); kanal email siap ditambah sebagai consumer group kedua di stream yang sama tanpa menyentuh produsen.
- **A3** — Mata uang default IDR; format angka mengikuti lokal.
- **A4** — Periode depresiasi/amortisasi dihitung bulanan, untuk **dua basis** (komersial & fiskal).
- **A5** — Login Google memakai OAuth2 authorization-code; kredensial (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`) disimpan sebagai env var. User Google baru mendapat peran default Staf.
- **A6** — Validasi batas ukuran file (menolak kosong/terlalu kecil & melebihi maks, mis. 5 MB); ambang via env.
- **A7** — **Routing approver berjenjang per nilai** (`approval_thresholds`, §2.4): operasional ringan → Manager/Kepala Unit; penghapusan/pengadaan/mutasi sesuai band nilai → Kanwil/Pusat. Angka **placeholder**, disesuaikan kebijakan bank.
- **A8** — Role **Kepala Unit** & **Kepala Kanwil** masuk model sejak awal (memengaruhi data scoping & skema kantor), implementasi UI-nya dapat dilakukan bertahap.
- **A9 (baru)** — **Domain = fixed asset bank** (bukan investment/wealth AM). Hierarki kantor **4 jenjang** (Pusat→Wilayah→Cabang/Unit→Outlet); `office_type` boleh banyak label. ⚠️ *Penamaan jenjang persis di BTN dikonfirmasi internal.*
- **A10 (baru)** — **Aset intangible (PSAK 19)**: field disiapkan (`asset_class`), amortisasi reuse engine penyusutan, dikecualikan dari fitur fisik. Workflow khusus menyusul.
- **A11 (baru)** — **Batas kapitalisasi** = config (placeholder Rp 1 jt ⚠️), override per-kategori.
- **A12 (baru)** — **Penyusutan dua basis**: komersial (PSAK 16) + fiskal (PMK 72/2023 & UU PPh Ps. 11, kelompok harta 1–4 / bangunan permanen·non-permanen). **Parameter fiskal terverifikasi** (Lampiran A); masa manfaat tidak berubah dari PMK 96/2009.
- **A13 (baru)** — **Model biaya (cost model)** untuk aset tetap + **impairment dasar** (PSAK 48). Model revaluasi penuh di luar lingkup. ⚠️ *Konfirmasi BTN memakai cost model.*
- **A14 (baru)** — **Output siap-jurnal** (rekap per akun GL) disediakan; **integrasi langsung** ke core banking/akuntansi di luar lingkup.
- **~~Q1~~ (selesai)** — Barcode wajib untuk setiap aset tangible (Code128 dari `asset_tag`) + label cetak/scan; QR alternatif.
- **~~Q2~~ (selesai)** — Import massal: aset & master data via CSV dan XLSX (template + validasi per-baris + laporan hasil).
- **~~Q3~~ (selesai)** — Field-level permission untuk **semua entitas** (field-registry + default per-role).
- **~~Q5~~ (selesai)** — Role "Employee" diganti menjadi **"Staf"**.
- **~~Q6~~ (selesai)** — Hierarki kantor **4 jenjang**: Pusat → Wilayah → Cabang/Unit → Outlet (via `parent_id`, dapat diperdalam).
- **~~Q7~~ (selesai)** — Data scope dapat dikonfigurasi **per-role + override per-modul** (`data_scope_policies`).

---

## Lampiran A — Parameter & Rujukan Regulasi (terverifikasi sumber primer)

> Diverifikasi via riset multi-sumber dengan verifikasi adversarial (24/25 klaim terkonfirmasi
> "high confidence"). Sumber utama: JDIH Kemenkeu, DJP (pajak.go.id), OJK (ojk.go.id),
> IAI/DSAK. *Tanggal verifikasi: 2026-06-26.* Beberapa halaman `peraturan.bpk.go.id` menolak akses
> otomatis (HTTP 403); metadatanya dikonfirmasi silang lewat JDIH/DJP/OJK sehingga tidak ada fakta
> yang bergantung pada satu sumber tak terakses.

### A.1 Penyusutan fiskal — PMK 72/2023

- **Identitas**: PMK No. 72 Tahun 2023, *"Penyusutan Harta Berwujud dan/atau Amortisasi Harta Tak
  Berwujud"*. Ditetapkan **13 Juli 2023**, berlaku **17 Juli 2023**, status **Berlaku**.
- **Mencabut**: PMK-96/PMK.03/2009 (pengelompokan harta berwujud bukan bangunan), PMK-248/PMK.03/2008
  & PMK-249/PMK.03/2008 (jo. PMK-126/PMK.011/2012).
- **Dasar hukum**: pelaksana **Pasal 21 ayat (10)** & **Pasal 22 ayat (5)** PP No. 55 Tahun 2022, dari
  **Pasal 32C UU PPh** (sebagaimana diubah UU HPP). Metode penyusutan: **UU PPh Pasal 11**.

**Harta berwujud bukan bangunan** (masa manfaat tidak berubah dari aturan sebelumnya):

| Kelompok | Masa manfaat | Tarif garis lurus | Tarif saldo menurun |
|---|---|---|---|
| Kelompok 1 | 4 tahun | 25% | 50% |
| Kelompok 2 | 8 tahun | 12,5% | 25% |
| Kelompok 3 | 16 tahun | 6,25% | 12,5% |
| Kelompok 4 | 20 tahun | 5% | 10% |

**Bangunan** (hanya metode **garis lurus**; saldo menurun tidak berlaku untuk bangunan):

| Jenis | Masa manfaat | Tarif (garis lurus) |
|---|---|---|
| Permanen | 20 tahun | 5% |
| Tidak permanen | 10 tahun | 10% |

> **Opsi baru PMK 72/2023**: bangunan **permanen** boleh disusutkan sesuai **masa manfaat sebenarnya
> > 20 tahun** menurut pembukuan WP (PP 55/2022 Pasal 21 ayat (5)). Ketentuan transisi (ayat (6)) untuk
> bangunan yang dimiliki/digunakan sebelum Tahun Pajak 2022 mensyaratkan pemberitahuan ke DJP dengan
> tenggat **30 April 2024** (sudah lewat). **Metode garis lurus** berlaku untuk semua aset berwujud;
> **saldo menurun** khusus aset **selain bangunan** (UU PPh Pasal 11).

*Sumber*: [jdih.kemenkeu.go.id/dok/pmk-72-tahun-2023](https://jdih.kemenkeu.go.id/dok/pmk-72-tahun-2023) ·
[pajak.go.id/en/node/98645](https://www.pajak.go.id/en/node/98645) ·
IAI/DSAK Sosialisasi PMK 72/2023 · peraturan.bpk.go.id/Details/257823.

### A.2 Tata kelola & pengendalian internal bank — OJK

- **POJK No. 17 Tahun 2023** — *Penerapan Tata Kelola bagi Bank Umum*. Ditetapkan & berlaku
  **14 September 2023**; mencabut **POJK 55/POJK.03/2016**.
  - **Pasal 85** — bank wajib menerapkan manajemen risiko & **sistem pengendalian intern yang tepat
    dan efektif** (mencakup pemisahan fungsi, identifikasi/penilaian risiko, aktivitas pengendalian,
    sistem informasi/akuntansi, pemantauan).
  - **Pasal 115 ayat (3)** — keputusan kredit/pembiayaan wajib menerapkan **prinsip pemisahan fungsi
    (*four-eyes principle*)** antara fungsi bisnis & risiko.
  - **Pasal 116** — **pemisahan fungsi & kewenangan dalam proses pengadaan** (kait paling langsung
    untuk kontrol perolehan aset).
- **POJK No. 18/POJK.03/2016** — *Penerapan Manajemen Risiko bagi Bank Umum*. Ditetapkan
  **16 Maret 2016**, berlaku **22 Maret 2016**, status **Berlaku** (mencakup 8 jenis risiko termasuk
  **risiko operasional**). Catatan: sebagian pasal dapat terdampak instrumen lanjutan
  (mis. POJK 13/POJK.03/2021, POJK 11/POJK.03/2022, SEOJK terkait).

> *Catatan interpretatif:* POJK di atas mengatur pengendalian intern bank secara umum dan **tidak
> menyebut "aset tetap" secara eksplisit**; penerapannya ke kontrol pencatatan aset tetap (SoD,
> dual-control/maker-checker, jejak audit — §2.4) adalah inferensi wajar, bukan kutipan harfiah.
> Istilah "dual control / maker-checker" adalah terminologi industri; padanan regulasinya adalah
> *prinsip pemisahan fungsi (four-eyes)*.

### A.3 Pertanyaan terbuka (di luar lingkup verifikasi ini)

1. Perlakuan **amortisasi harta tak berwujud** (PSAK 19 vs PMK 72/2023 Pasal 22(5), opsi >20 tahun) —
   belum diverifikasi rinci di sini.
2. Apakah SEOJK pelaksana POJK 18/2016 atau instrumen lanjutan mengubah ketentuan pengendalian
   intern/operasional yang dipakai modul aset.
3. Rekonsiliasi penyusutan **fiskal (PMK 72/2023)** vs **akuntansi (PSAK 16)** — nomor paragraf PSAK
   spesifik perlu verifikasi sumber primer IAI/DSAK.
4. Penamaan jenjang kantor persis di **BTN**, **batas kapitalisasi**, dan **limit otorisasi nominal**
   — hanya dari kebijakan/dokumen internal bank.

---

## Changelog

- **v1.2 (2026-07-18)** — **Scope mobile dibuka**: non-goal v1.1 "aplikasi mobile native" dicabut;
  masuk **aplikasi mobile companion** berbasis Flutter (field companion: scan aset, approval
  on-the-go, push FCM, stock opname offline-first) — bagian 3.11 (penunjuk), tahap 11 roadmap.
  **Dokumentasi mobile dipisah** ke `docs/mobile/` (PRD mobile, ADR-0015/0016, design brief +
  prompt kit). Web tetap aplikasi utama administrasi; semua otorisasi tetap ditegakkan server.
- **v1.1 (2026-06-26)** — Reframe ke **Fixed Asset Management bank** (konteks BTN). Tambah: pemisahan
  fungsi (SoD) & limit otorisasi berjenjang per nilai (`approval_thresholds`); **mutasi aset**;
  **stock opname**; **BAST/dokumen**; **penyusutan dua basis** (komersial PSAK + fiskal pajak),
  **amortisasi intangible** (PSAK 19), **impairment** (PSAK 48); **batas kapitalisasi**; **disposal**
  dengan laba/rugi; **output siap-jurnal** (GL). Kategori diperkaya (akun GL, golongan pajak, masa
  manfaat komersial+fiskal). Item ⚠️ menandai parameter regulasi/kebijakan yang menunggu verifikasi
  sumber primer / kebijakan bank.
  - **Verifikasi regulasi (2026-06-26):** parameter penyusutan fiskal (**PMK 72/2023**, kelompok harta
    1–4 & bangunan) dan tata kelola/pengendalian bank (**POJK 17/2023**, **POJK 18/POJK.03/2016**)
    diverifikasi ke sumber primer dan dirangkum di **Lampiran A**; ⚠️ tersisa hanya pada nomor paragraf
    PSAK dan parameter internal BTN.
- **v1.0 (2026-06-23)** — Draft awal (manajemen aset fisik organisasi generik).
