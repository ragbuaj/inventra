# Inventra — Design Brief & Prompt Kit

Prompt siap-pakai untuk men-generate desain UI Inventra di Claude (mode artifact/design).
Disusun dari [PRD.md](PRD.md) (peran, fitur) dan design system frontend (Nuxt UI v4, primary
hijau, neutral slate, dark mode, i18n id/en).

**Cara pakai:** jangan generate semua sekaligus. Tempel **Master Brief** (§1) sekali di awal
percakapan sebagai konteks tetap, jalankan **Component Library** (§4) lebih dulu untuk mengunci
bahasa visual, lalu generate layar satu per satu (§2) memakai **template per-screen** (§3).

---

## 1. Master Brief (tempel sekali di awal percakapan Claude)

```
Kamu adalah product designer + frontend engineer. Bantu saya mendesain UI untuk
"Inventra" — aplikasi web manajemen aset/inventaris organisasi. Outputkan sebagai
artifact (React + Tailwind) berupa mockup high-fidelity yang interaktif, dengan data
contoh yang realistis. Saya akan meminta layar/komponen satu per satu; ingat brief ini
sepanjang sesi.

## Produk
Sistem manajemen aset fisik: katalog aset, check-out/check-in, maintenance, depresiasi,
laporan, dan approval maker-checker. Organisasi berstruktur 4 jenjang kantor
(Pusat → Wilayah → Cabang → Outlet) dengan hak akses berbasis peran + lingkup kantor.
Pengguna utamanya operator internal (admin aset, kepala unit), bukan publik —
jadi ini aplikasi admin/dashboard yang padat data, bukan landing page.

## Peran (memengaruhi apa yang tampil)
- Superadmin — akses penuh: user, peran, master data, konfigurasi, semua laporan.
- Kepala Kanwil / Kepala Unit — approval + laporan dalam lingkup kantornya.
- Manager (Asset Manager) — operasional aset dalam lingkup kantornya.
- Staf — hanya aset yang dipegangnya; mengajukan peminjaman/laporan kerusakan.
Desain harus mengakomodasi: menu yang muncul/disembunyikan per peran, dan
field tertentu yang disembunyikan per peran (mis. harga beli & nilai buku hanya
untuk peran tertentu).

## Design system (WAJIB diikuti — saya implementasi pakai Nuxt UI v4 + Tailwind v4)
- Pakai pola komponen yang setara Nuxt UI: Button, Card, Table, Form, Modal/Slideover,
  Input, Select, Badge, Tabs, Breadcrumb, Dropdown, Toast, Pagination. Desain harus
  bisa dipetakan langsung ke komponen-komponen ini — hindari pola eksotis di luar itu.
- Warna: primary = green, neutral = slate. Pakai token semantik (primary/neutral/
  muted, success/warning/error), bukan warna hardcode acak.
- WAJIB dukung light & dark mode — tunjukkan kedua varian bila relevan.
- Tipografi bersih, sans-serif (Inter), sudut membulat (rounded-lg), spasi lapang,
  estetika modern/profesional tapi tidak ramai.
- Responsif, desktop-first (target utama layar lebar), tetap usable di tablet.
- Bahasa UI: Bahasa Indonesia (default). Gunakan label Indonesia yang natural
  (mis. "Tambah Aset", "Simpan", "Pengajuan", "Kelola Kantor").
- Aksesibel: kontras cukup, label form jelas, focus state terlihat.

## Pola umum yang harus konsisten di semua layar
- App shell: sidebar navigasi kiri (dapat di-collapse) + topbar (search global,
  notifikasi, switch bahasa id/en, toggle light/dark, menu profil).
- Halaman list/index: page header (judul + tombol aksi primer) → filter/search bar →
  data table (kolom relevan, status badge, aksi per-baris: lihat/edit/hapus) →
  pagination. Sertakan empty state.
- Buat/edit memakai modal atau slideover (drawer kanan), bukan halaman penuh,
  untuk entitas sederhana.
- Aksi destruktif (hapus) memakai confirm dialog.
- Tampilkan loading state (skeleton) dan error/empty state.

Konfirmasi kamu paham brief ini, lalu tunggu saya menyebut layar pertama.
```

---

## 2. Daftar layar untuk di-generate (minta satu per satu)

Checklist. Untuk tiap item, kirim template di §3.

**Auth & shell**
1. Login (email/password + tombol "Masuk dengan Google") + lupa/reset password
2. App shell + sidebar navigasi (varian per peran)
3. Dashboard (KPI: total aset, nilai perolehan vs buku, aset per status/kategori/lokasi, aset overdue, maintenance jatuh tempo, biaya maintenance)

**Aset**
4. Katalog aset (list + filter status/kategori/kantor + search + scan barcode)
5. Detail aset (tabs: Info, Riwayat Penugasan, Riwayat Maintenance, Jadwal Depresiasi) — dengan field yang dibatasi per peran
6. Form tambah/edit aset
7. Import massal aset (upload CSV/XLSX → preview validasi per-baris → laporan sukses/gagal)
8. Label & barcode/QR (preview + cetak tunggal/batch)

**Operasional**
9. Penugasan / check-out & check-in (form + riwayat)
10. Maintenance (jadwal, catatan, laporan kerusakan, reminder jatuh tempo)
11. Approval / maker-checker (inbox pengajuan + detail + approve/reject dengan timeline)

**Master data**
12. Halaman master data generik (template list+form untuk: jenis kantor, departemen, jabatan, satuan, vendor, brand, model, kategori, kategori perawatan, kategori masalah, provinsi, kota)
13. Kantor — tampilan pohon hierarki (Pusat→Wilayah→Cabang→Outlet) + lantai/ruangan bertingkat
14. Pegawai (list + form, scoped per kantor)

> **Layar bermenu yang belum ada mockup-nya** (prompt siap-pakai sudah disiapkan):
> - **Lokasi & Geografi** (`nav.geography`, anak Master Data) — hierarki Provinsi → Kota → **§5.21**
> - **Profil & Pengaturan Akun** (menu profil topbar: `nav.profile` + `nav.accountSettings`) → **§5.22**

**Pengaturan/Admin (Superadmin)**
15. Manajemen user (list + form: peran, kantor, pegawai tertaut)
16. Konfigurasi peran & RBAC (matriks izin per-aksi)
17. Konfigurasi data scope (peran → level: global/office_subtree/office/own, + override per-modul)
18. Konfigurasi field-permission (matriks entitas × field × peran: lihat/edit)
19. Audit trail (log aktivitas, read-only)

**Laporan**
20. Laporan (daftar aset + nilai buku, depresiasi per periode, utilisasi, biaya maintenance) + tombol ekspor PDF/Excel

---

## 3. Template prompt per-screen (isi & kirim untuk tiap layar)

```
Sekarang desain layar: <NAMA LAYAR, mis. "Katalog Aset">.

Tujuan layar: <1 kalimat>
Pengguna utama: <peran>
Elemen yang harus ada:
- <komponen / data 1>
- <komponen / data 2>
- <aksi-aksi: tombol, filter, dsb>
States: tunjukkan loading (skeleton), empty state, dan satu contoh dengan data penuh.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia (nama kantor mis. "Kantor Cabang
Jakarta Selatan", kode aset format JKT01-ELK-2026-00001, status: tersedia/dipinjam/
maintenance/dilepas/hilang). Patuhi master brief.
```

---

## 4. Component Library / Style Guide (jalankan lebih dulu, sebelum layar)

```
Sebelum mendesain layar, buat dulu satu artifact "Component Library / Style Guide"
yang menampilkan semua komponen dasar dalam light & dark mode, sesuai master brief:
buttons (primary/neutral/ghost/danger + ukuran), inputs/select/textarea (normal/
focus/error/disabled), form field dengan label+hint+error, badges status aset
(tersedia/dipinjam/maintenance/dilepas/hilang) & status pengajuan (pending/disetujui/
ditolak), cards & stat/KPI card, data table (header, baris, aksi, pagination), tabs,
breadcrumb, modal & slideover, toast/alert (success/warning/error), dropdown menu,
empty state, skeleton loader, dan tree view (hierarki kantor). Tata dalam grid
beranotasi nama komponennya. Ini jadi acuan untuk semua layar berikutnya.
```

---

## 5. Prompt per-screen lengkap (siap copy-paste)

Tiap blok di bawah sudah terisi penuh sesuai PRD. Kirim satu per satu setelah Master Brief (§1)
dan Component Library (§4). Semua mengikuti format §3.

### 5.1 Login

```
Sekarang desain layar: Login.

Tujuan layar: Pengguna masuk ke sistem dengan email/password atau akun Google.
Pengguna utama: semua peran.
Elemen yang harus ada:
- Layout dua kolom di desktop: panel brand kiri (logo Inventra, tagline singkat,
  ilustrasi/gradient hijau) + form kanan; satu kolom di mobile.
- Card form login: input Email, input Password (dengan toggle lihat/sembunyikan),
  checkbox "Ingat saya", link "Lupa password?".
- Tombol primer "Masuk", pemisah "atau", tombol "Masuk dengan Google" (ikon Google).
- Pesan error inline (mis. "Email atau password salah") di atas form.
- Switch bahasa (id/en) dan toggle light/dark di pojok.
States: form kosong (default), state error kredensial, tombol "Masuk" loading.
Tampilkan versi light dan dark.

Patuhi master brief. Sertakan juga varian "Lupa password" (input email + tombol
"Kirim tautan reset") dan "Reset password" (password baru + konfirmasi).
```

### 5.2 App shell + sidebar navigasi

```
Sekarang desain layar: App Shell + Sidebar Navigasi.

Tujuan layar: Kerangka aplikasi yang membungkus semua halaman (navigasi + topbar).
Pengguna utama: semua peran (tampilkan varian Superadmin & varian Staf).
Elemen yang harus ada:
- Sidebar kiri (dapat di-collapse jadi ikon saja) dengan grup menu + ikon:
  Dashboard; Aset (Katalog, Import, Label/Barcode); Penugasan; Maintenance;
  Pengajuan/Approval (dengan badge jumlah pending); Laporan;
  Master Data (Kantor, Pegawai, Lokasi & Geografi, Referensi); 
  Pengaturan (User, Peran & RBAC, Data Scope, Field-Permission, Audit Trail).
- Topbar: tombol collapse sidebar, breadcrumb/judul halaman, search global,
  ikon notifikasi (dengan badge), switch bahasa id/en, toggle light/dark,
  menu profil (avatar, nama, peran, "Profil", "Keluar").
- Area konten utama memakai placeholder.
States: tunjukkan sidebar expanded vs collapsed; varian menu Superadmin (lengkap)
vs varian Staf (hanya Dashboard, Katalog aset miliknya, Penugasan, Pengajuan).
Tampilkan versi light dan dark.

Patuhi master brief. Tandai item menu aktif dengan aksen primary (hijau).
```

### 5.3 Dashboard

```
Sekarang desain layar: Dashboard.

Tujuan layar: Ringkasan kondisi aset & operasional dalam lingkup kantor pengguna.
Pengguna utama: Kepala Unit / Manager (tunjukkan juga betapa angka bisa berbeda
karena data scope).
Elemen yang harus ada:
- Baris KPI stat-card: Total Aset, Nilai Perolehan, Nilai Buku, Aset Overdue,
  Maintenance Jatuh Tempo, Total Biaya Maintenance (tiap card: angka besar,
  label, tren kecil/ikon).
- Chart: aset per status (donut: tersedia/dipinjam/maintenance/dilepas/hilang),
  aset per kategori (bar), aset per lokasi/kantor (bar).
- Panel "Maintenance jatuh tempo" (list ringkas) dan "Pengajuan menunggu approval"
  (list ringkas dengan tombol cepat).
- Filter periode & kantor di header.
States: loading (skeleton card + chart), dan tampilan penuh dengan data.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia (kantor "Kantor Cabang Jakarta
Selatan", nilai rupiah berformat Rp). Patuhi master brief.
```

### 5.4 Katalog Aset

```
Sekarang desain layar: Katalog Aset (list).

Tujuan layar: Menelusuri, memfilter, dan mengelola daftar aset.
Pengguna utama: Manager (lihat versi Staf yang hanya menampilkan aset miliknya).
Elemen yang harus ada:
- Page header "Katalog Aset" + tombol primer "Tambah Aset" + tombol "Import" +
  tombol "Scan Barcode".
- Filter bar: search (nama/kode aset), filter Status, Kategori, Kantor, Lokasi,
  rentang tanggal beli; tombol reset filter; toggle tampilan tabel/grid kartu.
- Data table kolom: Kode Aset (asset_tag), Nama, Kategori, Brand/Model, Status
  (badge berwarna), Kantor, Pemegang, Tanggal Beli, aksi per-baris (lihat/edit/
  hapus/cetak label). Header kolom sortable, ada checkbox seleksi massal +
  aksi massal (cetak label batch).
- Pagination + info "menampilkan 1-20 dari N".
States: loading (skeleton table), empty state ("Belum ada aset"), data penuh.
Tampilkan versi light dan dark.

Pakai data contoh realistis: kode aset format JKT01-ELK-2026-00001, status
tersedia/dipinjam/maintenance/dilepas/hilang, kategori Elektronik/Furnitur/Kendaraan.
Patuhi master brief. CATATAN: kolom Harga Beli & Nilai Buku hanya muncul untuk
peran tertentu (field-permission) — pada versi Staf, sembunyikan kolom harga.
```

### 5.5 Detail Aset

```
Sekarang desain layar: Detail Aset.

Tujuan layar: Menampilkan seluruh informasi satu aset beserta riwayatnya.
Pengguna utama: Manager (tunjukkan juga versi Staf dengan field harga disembunyikan).
Elemen yang harus ada:
- Header: nama aset, kode (asset_tag) + barcode/QR kecil, status badge,
  tombol aksi (Edit, Cetak Label, Check-out/Check-in, Ajukan Maintenance,
  Ajukan Pengecualian Valuasi, Hapus).
- Panel ringkas kiri: foto aset (galeri), info utama (kategori, brand/model,
  kantor, lokasi lantai/ruangan, vendor, kondisi).
- Tabs: 
  (1) Info — semua field termasuk tanggal beli, harga beli, metode depresiasi,
      masa manfaat, nilai buku saat ini (field sensitif ditandai).
  (2) Riwayat Penugasan — tabel siapa memegang, dari–sampai, kondisi.
  (3) Riwayat Maintenance — tabel tanggal, tipe (preventive/corrective), status,
      biaya, vendor/teknisi.
  (4) Jadwal Depresiasi — tabel per periode: nilai awal, penyusutan, nilai buku.
States: loading (skeleton), data penuh. Tunjukkan tab Info pada versi Staf di mana
Harga Beli & Nilai Buku diganti tanda "—" / disembunyikan.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia, rupiah berformat Rp. Patuhi master brief.
```

### 5.6 Form Tambah/Edit Aset

```
Sekarang desain layar: Form Tambah/Edit Aset (slideover/drawer kanan).

Tujuan layar: Membuat atau menyunting data aset.
Pengguna utama: Manager.
Elemen yang harus ada:
- Slideover lebar dari kanan, judul "Tambah Aset" / "Edit Aset".
- Form berkelompok (section): 
  Identitas (Nama, Kategori [select], Brand [select], Model [select], 
    Kode aset otomatis—tampilkan preview JKT01-ELK-2026-00001 read-only & catatan
    "dibuat otomatis"), 
  Penempatan (Kantor [select], Lantai [select], Ruangan [select], Pemegang [select pegawai]),
  Pembelian (Tanggal Beli, Harga Beli [Rp], Vendor [select]),
  Depresiasi (Metode [straight_line/declining_balance], Masa Manfaat [tahun], Nilai Residu),
  Lampiran (dropzone foto & dokumen).
- Validasi inline (field wajib bertanda *, pesan error di bawah field).
- Footer sticky: tombol "Batal" + "Simpan".
- Catatan: registrasi aset baru berjalan lewat pengajuan + approval (maker-checker) —
  tampilkan info banner kecil "Aset baru akan masuk antrean persetujuan".
States: form kosong (tambah), form terisi (edit), satu field dengan error validasi.
Tampilkan versi light dan dark.

Patuhi master brief.
```

### 5.7 Import Massal Aset

```
Sekarang desain layar: Import Massal Aset.

Tujuan layar: Mengunggah CSV/XLSX, memvalidasi per-baris, lalu membuat aset yang valid.
Pengguna utama: Manager / Superadmin.
Elemen yang harus ada:
- Stepper 3 langkah: (1) Unggah, (2) Validasi/Preview, (3) Hasil.
- Langkah Unggah: tombol "Unduh Template" (CSV & XLSX), dropzone unggah berkas,
  catatan kolom yang diharapkan.
- Langkah Validasi: tabel preview baris dengan kolom status per-baris (Valid /
  Error), highlight sel bermasalah, ringkasan "X valid, Y error", filter "tampilkan
  hanya yang error". Pesan error per-baris (mis. "Kategori tidak ditemukan",
  "asset_tag duplikat", "Tanggal tidak valid").
- Langkah Hasil: ringkasan sukses vs gagal, tombol "Unduh baris gagal" untuk koreksi,
  tombol "Selesai".
States: kosong (sebelum unggah), sedang memproses (progress), hasil dengan campuran
sukses & gagal.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.8 Label & Barcode/QR

```
Sekarang desain layar: Label & Barcode/QR.

Tujuan layar: Pratinjau dan cetak label aset (barcode Code128 + QR) tunggal/batch.
Pengguna utama: Manager.
Elemen yang harus ada:
- Panel kiri: pemilihan aset (search + daftar tercentang untuk batch), pilihan
  ukuran label & jumlah per halaman, toggle tampilkan QR / barcode / keduanya,
  pilihan field yang dicetak (nama, kode, kantor).
- Panel kanan: pratinjau lembar label (grid) yang mencerminkan pilihan — tiap label
  menampilkan barcode Code128 dari asset_tag, QR, nama & kode aset.
- Tombol "Cetak" dan "Unduh PDF".
States: satu aset terpilih (label tunggal), banyak aset (lembar batch), empty
(belum ada aset dipilih).
Tampilkan versi light dan dark.

Pakai kode contoh JKT01-ELK-2026-00001. Patuhi master brief.
```

### 5.9 Penugasan (Check-out / Check-in)

```
Sekarang desain layar: Penugasan Aset (Check-out & Check-in).

Tujuan layar: Meminjamkan aset ke pegawai dan mengembalikannya, dengan riwayat.
Pengguna utama: Manager.
Elemen yang harus ada:
- Tabs: "Check-out", "Check-in", "Riwayat".
- Check-out: form pilih Aset (search, hanya status tersedia), pilih Pegawai
  penerima, tanggal pinjam, catatan/kondisi keluar; tombol "Check-out".
- Check-in: form pilih penugasan aktif, tanggal kembali, kondisi masuk (select),
  opsi "perlu maintenance" (jika dicentang, status aset → maintenance); tombol "Check-in".
- Riwayat: data table (Aset, Pemegang, Tanggal Pinjam, Tanggal Kembali, Status
  [active/returned], Kondisi). Filter status & search.
States: form kosong, form terisi, riwayat dengan data, empty state.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.10 Maintenance

```
Sekarang desain layar: Maintenance.

Tujuan layar: Mengelola jadwal & catatan maintenance dan laporan kerusakan.
Pengguna utama: Manager (lihat juga aksi "Laporkan Kerusakan" milik Staf).
Elemen yang harus ada:
- Tabs/segment: "Jadwal", "Catatan", "Laporan Kerusakan".
- Banner reminder "Maintenance jatuh tempo" di atas (list ringkas).
- Catatan: data table kolom Aset, Tipe (preventive/corrective), Kategori Perawatan,
  Tanggal, Status (scheduled/in_progress/completed/cancelled — badge), Biaya,
  Vendor/Teknisi; tombol "Tambah Catatan Maintenance".
- Form tambah catatan (slideover): Aset, Tipe, Kategori Perawatan, Tanggal,
  Status, Biaya [Rp], Vendor/Teknisi, Deskripsi.
- Laporan Kerusakan: form sederhana milik Staf (pilih aset miliknya, Kategori
  Masalah [select], deskripsi, foto) yang masuk antrean sebagai permintaan maintenance.
States: list dengan data, empty state, form tambah, satu reminder overdue ditonjolkan.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia, rupiah Rp. Patuhi master brief.
```

### 5.11 Approval / Maker-Checker

```
Sekarang desain layar: Pengajuan & Approval (Maker-Checker).

Tujuan layar: Meninjau dan menyetujui/menolak pengajuan dalam lingkup pengguna.
Pengguna utama: Kepala Unit / Kepala Kanwil.
Elemen yang harus ada:
- Layout inbox: panel kiri daftar pengajuan (kartu/baris dengan Tipe, Pengaju,
  tanggal, status badge), filter tab "Menunggu / Disetujui / Ditolak / Semua",
  filter Tipe.
- Tipe pengajuan: Registrasi Aset, Penghapusan Aset, Peminjaman, Maintenance,
  Pengecualian Valuasi (tandai yang sensitif).
- Panel kanan detail pengajuan terpilih: ringkasan data yang diajukan (before/after
  bila relevan), pengaju & kantor, alasan, lampiran.
- Timeline/riwayat approval (siapa, kapan, aksi, catatan).
- Footer aksi: input catatan + tombol "Setujui" (hijau) & "Tolak" (merah);
  untuk pengecualian valuasi tampilkan peringatan "tindakan sensitif".
- Badge jumlah pending di header.
States: ada pengajuan terpilih (detail penuh), tidak ada yang dipilih (placeholder),
inbox kosong (empty state).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.12 Master Data Generik (template list + form)

```
Sekarang desain layar: Master Data Referensi (template generik list + form).

Tujuan layar: Pola CRUD seragam yang dipakai ulang untuk banyak entitas referensi
sederhana (jenis kantor, departemen, jabatan, satuan, vendor, brand, model,
kategori, kategori perawatan, kategori masalah, provinsi, kota).
Pengguna utama: Superadmin.
Elemen yang harus ada:
- Sub-navigasi (tabs atau secondary sidebar) berisi daftar entitas referensi di atas.
- Konten: page header (nama entitas + tombol "Tambah"), search bar, data table
  (kolom Nama, Kode/atribut relevan, Status Aktif [toggle/badge], aksi edit/hapus),
  pagination.
- Form tambah/edit dalam modal: field menyesuaikan entitas (mis. Vendor punya
  kontak/telepon/email/alamat; Model punya relasi Brand; Kota punya relasi Provinsi),
  toggle "Aktif".
- Tunjukkan 2 contoh konkret: entitas sederhana (Jabatan: Nama + Aktif) dan entitas
  dengan relasi (Model: Brand + Nama + Aktif).
States: list dengan data, empty state, modal form (tambah & edit), konfirmasi hapus.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.13 Kantor — Pohon Hierarki + Lantai/Ruangan

```
Sekarang desain layar: Master Data Kantor (hierarki) + Lantai & Ruangan.

Tujuan layar: Mengelola struktur kantor berjenjang dan lokasi di dalamnya.
Pengguna utama: Superadmin / Kepala Kanwil (scoped).
Elemen yang harus ada:
- Panel kiri: tree view hierarki kantor (Pusat → Wilayah → Cabang → Outlet) dengan
  expand/collapse, ikon level berbeda, badge jumlah anak, tombol "Tambah Kantor"
  (sebagai anak node terpilih), search.
- Panel kanan: detail kantor terpilih — info (nama, kode, jenis kantor, provinsi/kota,
  alamat, status) + tombol Edit/Hapus.
- Di detail kantor, sub-bagian "Lantai & Ruangan": daftar lantai (accordion), tiap
  lantai berisi daftar ruangan; tombol "Tambah Lantai" dan "Tambah Ruangan".
- Form tambah/edit kantor (slideover): Parent (kantor induk), Jenis Kantor, Nama,
  Kode, Provinsi, Kota, Alamat, Aktif.
States: tree dengan beberapa level, node terpilih menampilkan detail, empty state
(belum ada lantai/ruangan), form slideover.
Tampilkan versi light dan dark.

Pakai data contoh: "Kantor Pusat" > "Kanwil Jakarta" > "Cabang Jakarta Selatan" >
"Outlet Blok M"; lantai "Lantai 1", ruangan "Ruang Server". Patuhi master brief.
```

### 5.14 Pegawai

```
Sekarang desain layar: Master Data Pegawai.

Tujuan layar: Mengelola data pegawai, dibatasi lingkup kantor pengguna.
Pengguna utama: Superadmin / Kepala Kanwil (scoped per kantor).
Elemen yang harus ada:
- Page header "Pegawai" + tombol "Tambah Pegawai".
- Filter bar: search nama/NIP, filter Kantor, Departemen, Jabatan, status.
- Data table: NIP, Nama, Departemen, Jabatan, Kantor, Email/Telepon, Status,
  aksi edit/hapus. Pagination.
- Form tambah/edit (slideover): NIP, Nama, Departemen [select], Jabatan [select],
  Kantor [select—dibatasi scope], Email, Telepon, Status.
States: list dengan data, empty state, form slideover, satu field error.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.15 Manajemen User

```
Sekarang desain layar: Manajemen User.

Tujuan layar: Superadmin mengelola akun login dan menautkannya ke peran/kantor/pegawai.
Pengguna utama: Superadmin.
Elemen yang harus ada:
- Page header "Pengguna" + tombol "Tambah User".
- Filter bar: search nama/email, filter Peran, Kantor, Status (active/inactive/suspended).
- Data table: Avatar+Nama, Email, Peran (badge), Kantor Penempatan, Pegawai Tertaut,
  metode login (Email / Google — ikon), Status, aksi (edit, reset password,
  nonaktifkan, hapus). Pagination.
- Form tambah/edit (slideover): Nama, Email, Password (opsional—catatan "kosongkan
  jika hanya login Google"), Peran [select], Kantor Penempatan [select], Pegawai
  Tertaut [select], Status.
States: list dengan data, empty state, form slideover, dropdown aksi per-baris terbuka.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.16 Konfigurasi Peran & RBAC

```
Sekarang desain layar: Konfigurasi Peran & RBAC.

Tujuan layar: Mengelola daftar peran dan izin per-aksi tiap peran.
Pengguna utama: Superadmin.
Elemen yang harus ada:
- Panel kiri: daftar peran (Superadmin, Kepala Kanwil, Kepala Unit, Manager, Staf,
  + peran kustom), tanda "sistem" untuk peran bawaan, tombol "Tambah Peran".
- Panel kanan: untuk peran terpilih, matriks izin per-aksi dikelompokkan per modul
  (Aset, Penugasan, Maintenance, Pengajuan, Master Data, User, Laporan, Audit) —
  tiap izin berupa toggle/checkbox (mis. "aset.create", "aset.delete",
  "pengajuan.approve"). Header peran sistem read-only/terkunci.
- Tombol "Simpan Perubahan", indikasi perubahan belum tersimpan.
States: peran sistem (terkunci), peran kustom (dapat diedit), form tambah peran (modal).
Tampilkan versi light dan dark.

Patuhi master brief. Gunakan nama izin yang masuk akal & dikelompokkan rapi.
```

### 5.17 Konfigurasi Data Scope

```
Sekarang desain layar: Konfigurasi Data Scope (lingkup data per peran).

Tujuan layar: Menetapkan level lingkup data tiap peran, dengan override per-modul.
Pengguna utama: Superadmin.
Elemen yang harus ada:
- Tabel: baris = peran, kolom = Default (semua modul) + kolom per-modul (Aset,
  Pengajuan, Maintenance, Master Data, Laporan). Tiap sel = select level:
  global / office_subtree / office / own.
- Penjelasan level (legend/tooltip): global=semua data; office_subtree=kantor +
  turunannya; office=kantor sendiri; own=data miliknya.
- Konsep "default per-role" vs "override per-modul" dijelaskan visual (sel override
  ditandai berbeda dari yang mewarisi default).
- Tombol "Simpan", contoh terisi: Manager → office_subtree untuk Aset namun own
  untuk Pengajuan.
States: tabel terisi default seed, satu sel sedang diubah (dropdown terbuka),
indikasi override aktif.
Tampilkan versi light dan dark.

Patuhi master brief.
```

### 5.18 Konfigurasi Field-Permission

```
Sekarang desain layar: Konfigurasi Field-Permission (hak akses per-field).

Tujuan layar: Menetapkan field mana yang boleh dilihat/diedit tiap peran, per entitas.
Pengguna utama: Superadmin.
Elemen yang harus ada:
- Pemilih Entitas di atas (select: Aset, Pegawai, User, Pengajuan, dst).
- Matriks: baris = field entitas terpilih (mis. untuk Aset: nama, kategori,
  harga_beli, nilai_buku, vendor, ...), kolom = peran. Tiap sel punya dua kontrol
  kecil: "Lihat" dan "Edit" (toggle). 
- Tandai field tanpa aturan eksplisit memakai "default" (badge), dan tonjolkan
  contoh: harga_beli hanya Lihat untuk Superadmin & Manager; nilai_buku hanya Superadmin.
- Search field, tombol "Simpan".
States: matriks terisi, satu sel diubah, badge "default" pada beberapa field.
Tampilkan versi light dan dark.

Patuhi master brief.
```

### 5.19 Audit Trail

```
Sekarang desain layar: Audit Trail.

Tujuan layar: Menelusuri log aktivitas sistem (read-only).
Pengguna utama: Superadmin (Kepala Unit/Kanwil read dalam lingkupnya).
Elemen yang harus ada:
- Page header "Audit Trail" + tombol "Ekspor".
- Filter bar: rentang tanggal, Aktor (user), Aksi (create/update/delete), Entitas,
  search.
- Data table: Waktu, Aktor (avatar+nama+peran), Aksi (badge), Entitas, Ringkasan
  perubahan, Kantor/IP. Baris dapat di-expand menampilkan diff before/after (JSON
  ter-format / daftar field berubah).
- Pagination, info jumlah.
States: list dengan data, satu baris ter-expand menampilkan diff, empty state.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief.
```

### 5.20 Laporan

```
Sekarang desain layar: Laporan.

Tujuan layar: Menyusun dan mengekspor laporan aset, depresiasi, utilisasi, dan biaya.
Pengguna utama: Kepala Unit / Manager (scoped).
Elemen yang harus ada:
- Pemilih jenis laporan (tabs/cards): "Daftar Aset & Nilai Buku", "Depresiasi per
  Periode", "Utilisasi/Penugasan", "Biaya Maintenance".
- Filter bar: rentang periode, Kantor, Kategori, Status; tombol "Terapkan".
- Area hasil: ringkasan KPI singkat + chart relevan (mis. nilai buku per kategori)
  + data table detail dengan total di footer.
- Tombol "Ekspor PDF" dan "Ekspor Excel" di header hasil.
States: sebelum filter diterapkan (placeholder pilih kriteria), hasil dengan data,
empty (tidak ada data untuk filter).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia, rupiah berformat Rp,
kode aset JKT01-ELK-2026-00001. Patuhi master brief.
```

### 5.21 Lokasi & Geografi

```
Sekarang desain layar: Master Data — Lokasi & Geografi.

Tujuan layar: Mengelola data geografis berjenjang (Provinsi → Kota) yang menjadi
rujukan alamat kantor. Ini terpisah dari "Master Data Kantor" (yang mengelola
struktur kantor + lantai/ruangan) dan lebih kaya daripada tabel referensi datar.
Pengguna utama: Superadmin.
Elemen yang harus ada:
- Layout dua panel:
  - Panel kiri — daftar Provinsi: search, tiap baris menampilkan nama + kode provinsi
    + badge jumlah kota; baris terpilih ditandai aksen primary; tombol "Tambah Provinsi"
    di atas. (Boleh berupa list atau tree 2-level yang bisa di-expand menampilkan kotanya.)
  - Panel kanan — Kota dari provinsi terpilih: header (nama provinsi + kode + jumlah kota
    + tombol Edit/Hapus provinsi), search kota, tombol "Tambah Kota", lalu data table kota
    (kolom: Nama, Kode, Status Aktif [badge/toggle], aksi edit/hapus) + pagination bila banyak.
- Strip ringkasan kecil di atas: "X provinsi · Y kota".
- Form tambah/edit Provinsi (modal): Nama, Kode, toggle Aktif.
- Form tambah/edit Kota (modal): Provinsi induk [select—terisi dari konteks panel],
  Nama, Kode, toggle Aktif.
- Aksi hapus memakai confirm dialog (tampilkan nama yang akan dihapus); hapus provinsi
  yang masih punya kota harus memberi peringatan.
- Catatan kecil/inline: "Data ini dipakai pada alamat Kantor (Master Data Kantor)."
States: provinsi terpilih menampilkan daftar kotanya (data penuh); provinsi tanpa kota
(empty state di panel kanan); belum ada provinsi terpilih (placeholder panel kanan);
loading (skeleton); modal form tambah & edit; konfirmasi hapus.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia: "DKI Jakarta" (kode 31) → kota
"Jakarta Selatan", "Jakarta Pusat", "Jakarta Timur"; "Jawa Barat" (32) → "Bandung",
"Bekasi", "Depok"; "Banten" (36) → "Tangerang", "Serang". Patuhi master brief.
```

### 5.22 Profil & Pengaturan Akun

```
Sekarang desain layar: Profil & Pengaturan Akun.

Tujuan layar: Pengguna melihat & menyunting profil pribadinya dan mengatur akunnya
(password, preferensi tampilan, keamanan). Satu halaman dengan tabs, dicapai dari
menu profil di topbar — item "Profil" membuka tab Profil, item "Pengaturan Akun"
membuka tab Keamanan.
Pengguna utama: semua peran.
Elemen yang harus ada:
- Header profil: avatar besar (dengan tombol ganti foto), nama, peran (badge),
  email, dan kantor penempatan.
- Tabs: (1) Profil, (2) Keamanan, (3) Preferensi.
- Tab "Profil": form data diri — Avatar (upload/ganti/hapus), Nama, Email
  (read-only bila login Google, dengan catatan), Telepon; dan blok info read-only:
  Peran, Kantor Penempatan, Pegawai Tertaut, Metode Login (badge Email / Google),
  Tanggal Bergabung. Tombol "Simpan Perubahan".
- Tab "Keamanan": Ganti Password (Password Lama, Password Baru + indikator kekuatan,
  Konfirmasi Password) — bila akun login Google-only, ganti dengan info banner
  "Akun ini masuk via Google; kelola password di akun Google Anda". Sub-bagian
  "Sesi & Perangkat" opsional: daftar sesi aktif + tombol "Keluar dari semua perangkat".
- Tab "Preferensi": Bahasa (id/en), Tema (Light / Dark / Ikuti Sistem), dan toggle
  preferensi notifikasi (keputusan approval, pengingat maintenance).
- Validasi inline (field wajib bertanda *, pesan error di bawah field); toast sukses
  "Profil diperbarui" / "Password diganti".
States: tab Profil terisi; tab Keamanan dengan form ganti password DAN varian akun
Google (banner, tanpa form password); tab Preferensi; satu field dengan error
validasi (mis. "Konfirmasi password tidak cocok"); loading (skeleton).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia: nama "Andi Saputra", peran
"Asset Manager", email "andi.saputra@inventra.local", kantor "Cabang Jakarta Selatan",
metode login Email. Patuhi master brief.
```
