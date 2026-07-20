# Inventra Mobile — Design Brief & Prompt Kit

Prompt siap-pakai untuk men-generate desain UI **aplikasi mobile companion** Inventra di Claude
(mode artifact/design). Disusun dari [PRD mobile](PRD.md) (FR-M1 sampai FR-M6) dan design system
Inventra yang ada (primary hijau, neutral slate, dark mode, i18n id/en) — **terpisah dari brief
web** ([../DESIGN_BRIEF.md](../DESIGN_BRIEF.md)) karena bahasa visualnya beda platform: layar
sentuh 6 inci, satu tangan, dipakai sambil berdiri di gudang.

Hasil mockup disimpan di `docs/mobile/design/` (satu file `.dc.html` per layar, standalone) dan
menjadi **sumber kebenaran visual** sebelum layar Flutter dibangun (konvensi design-fidelity).

**Cara pakai:** jangan generate semua sekaligus. Tempel **Master Brief Mobile** (bagian 1) sekali
di awal percakapan sebagai konteks tetap, jalankan **Component Library Mobile** (bagian 4) lebih
dulu untuk mengunci bahasa visual, lalu generate layar satu per satu (bagian 2) memakai prompt
lengkap di bagian 5.

---

## 1. Master Brief Mobile (tempel sekali di awal percakapan Claude)

```
Kamu adalah product designer + mobile engineer. Bantu saya mendesain UI untuk
"Inventra Mobile" — aplikasi Android (Flutter) pendamping lapangan dari sistem
manajemen aset tetap bank "Inventra". Outputkan sebagai artifact (React + Tailwind)
berupa mockup high-fidelity DALAM FRAME PONSEL: lebar 390px, tinggi ~844px, sudut
membulat, status bar sederhana di atas. Bila sebuah prompt meminta beberapa state,
tampilkan beberapa frame ponsel berdampingan, masing-masing berlabel state-nya.
Saya akan meminta layar satu per satu; ingat brief ini sepanjang sesi.

## Produk & konteks pemakaian
Aplikasi INTERNAL untuk pegawai bank, dipakai di lapangan: memindai label barcode/QR
aset dengan kamera, menjalankan stock opname (inventarisasi fisik) TERMASUK OFFLINE
di gudang/basement tanpa sinyal, memutus approval saat mobile, dan menerima push
notification. Ini BUKAN aplikasi admin — layar admin (master data, laporan, RBAC)
tetap di web. Dipakai satu tangan, sering sambil berdiri; aksi utama harus besar dan
terjangkau ibu jari.

## Peran (memengaruhi apa yang tampil)
- Petugas opname / Manager aset — scan, stock opname, lihat detail aset.
- Kepala Unit / Kepala Kanwil — approval on-the-go (+ semua kemampuan di atas).
- Field sensitif (harga beli, nilai buku) DISEMBUNYIKAN untuk peran tertentu —
  tampilkan "—" sebagai gantinya (field-permission dari server).

## Design system (WAJIB — saya implementasi pakai Flutter Material 3)
- Warna: primary = green (sama dengan web Inventra), neutral = slate. Token semantik
  (primary/neutral/muted, success/warning/error) — bukan warna hardcode acak.
  Status aset: tersedia (hijau), dipinjam (biru), maintenance (amber), dilepas
  (slate), hilang (merah). Hasil opname: ditemukan (hijau), rusak (amber), salah
  lokasi (biru), tidak ditemukan (merah).
- WAJIB dukung light & dark mode — tunjukkan kedua varian bila diminta.
- Tipografi bersih sans-serif (Inter), sudut membulat (rounded-xl untuk kartu,
  rounded-full untuk chip), spasi lapang, estetika modern/profesional.
- Komponen harus terpetakan ke Material 3 / Flutter: AppBar, NavigationBar (bottom),
  FloatingActionButton, Card, ListTile, FilledButton/OutlinedButton, Chip, Badge,
  BottomSheet, SnackBar, AlertDialog, SegmentedButton, LinearProgressIndicator,
  TextField. Hindari pola eksotis di luar itu.
- Navigasi utama: BOTTOM NAVIGATION 5 slot — Beranda, Opname, [Scan], Approval,
  Notifikasi — dengan slot tengah berupa TOMBOL SCAN menonjol (FAB hijau menjorok ke
  atas, ikon barcode-scan). Ini elemen khas aplikasi; konsisten di semua layar utama.
  Layar sekunder (detail, form) memakai AppBar dengan tombol kembali, tanpa bottom nav.
- Indikator konektivitas adalah warga kelas satu: banner "Offline — scan tersimpan di
  perangkat" (amber, slim, di bawah AppBar) dan PILL STATUS SYNC pada layar opname
  (mis. "12 belum tersinkron" dengan ikon; berubah "Tersinkron" hijau saat beres).
  Offline BUKAN state error — tampilkan tenang dan informatif, bukan alarm.
- Target sentuh minimal 48dp; aksi utama layar = tombol besar full-width dekat ibu
  jari; hindari aksi penting di pojok atas.
- Bahasa UI: Bahasa Indonesia natural ("Pindai", "Setujui", "Tolak", "Belum
  tersinkron", "Sesi Opname"). Aksesibel: kontras cukup, label jelas, state fokus.

## Pola umum yang harus konsisten
- List memakai Card/ListTile dengan pull-to-refresh; sertakan empty state (ikon +
  satu kalimat + aksi) dan skeleton loading.
- Aksi destruktif/final (tolak pengajuan, logout semua) memakai dialog konfirmasi.
- Toast/SnackBar untuk konfirmasi sukses; error inline dekat sumbernya.
- Data contoh realistis berbahasa Indonesia: kantor "Cabang Jakarta Selatan", kode
  aset format JKT01-ELK-2026-00001, nama pegawai Indonesia, rupiah berformat Rp.

Konfirmasi kamu paham brief ini, lalu tunggu saya menyebut layar pertama.
```

---

## 2. Daftar layar untuk di-generate (minta satu per satu)

Checklist v1 — 12 layar + 1 component library (fase M0-M6, sudah di-generate), plus
**8 layar baru + 3 edit** untuk perluasan scope 2026-07-21 (fase M7 aksi aset, fase M8
profil & keamanan — lihat PRD mobile v1.1 FR-M7/FR-M6/FR-M1.5). Simpan hasil tiap layar
sebagai `docs/mobile/design/<Nama Layar>.dc.html`.

**Fondasi (fase M0)**
0. Component Library Mobile (bagian 4 — jalankan pertama)
1. Login — bagian 5.1 (**edit**: tambah tautan "Lupa password?")
2. Beranda (Home) — bagian 5.2 (**edit**: aksi cepat ke Katalog, Aset Saya, Pengajuan Saya)

**Scan & aset (fase M1)**
3. Scan (kamera) — bagian 5.3
4. Detail Aset — bagian 5.4 (**edit**: bar aksi FR-M7 per permission)

**Approval (fase M2)**
5. Inbox Approval — bagian 5.5
6. Detail Approval — bagian 5.6

**Notifikasi (fase M3)**
7. Notifikasi — bagian 5.7

**Stock opname (fase M4-M5)**
8. Daftar Sesi Opname — bagian 5.8
9. Opname Counting (scan + offline) — bagian 5.9
10. Variance & Tindak Lanjut — bagian 5.10

**Akun (fase M0/M6)**
11. Profil & Sesi Perangkat — bagian 5.11 (**edit**: profil lengkap + ubah data diri + avatar)
12. Pengaturan — bagian 5.12

**Aksi aset (fase M7) — baru**
13. Katalog Aset — bagian 5.13
14. Peminjaman / Check-out / Check-in (bottom sheet dari Detail Aset) — bagian 5.14
15. Lapor Kerusakan (bottom sheet dari Detail Aset) — bagian 5.15
16. Form Registrasi Aset — bagian 5.16
17. Pengajuan Saya — bagian 5.17
18. Aset Saya — bagian 5.18

**Profil & keamanan (fase M8) — baru**
19. Keamanan Akun — bagian 5.19
20. Lupa Password — bagian 5.20

> **Navigasi layar baru:** bottom nav tetap **5 slot** (Beranda, Opname, [Scan], Approval,
> Notifikasi) — tidak berubah. Katalog Aset, Aset Saya, dan Pengajuan Saya adalah **destinasi
> sekunder** (AppBar + tombol kembali) yang dijangkau dari **aksi cepat/menu di Beranda**; Aset
> Saya dan Pengajuan Saya juga dapat dijangkau dari area Profil. Peminjaman/Check-out/Check-in dan
> Lapor Kerusakan dipicu dari **Detail Aset** (bottom sheet). Registrasi dari **Beranda/Katalog**
> (aset baru, bukan dari detail aset yang sudah ada). Keamanan Akun dari Profil; Lupa Password dari
> Login.

---

## 3. Template prompt per-screen (kerangka — bagian 5 sudah mengisinya)

```
Sekarang desain layar: <NAMA LAYAR>.

Tujuan layar: <1 kalimat>
Pengguna utama: <peran>
Navigasi: <bottom nav aktif di tab X / AppBar dengan tombol kembali>
Elemen yang harus ada:
- <komponen / data 1>
- <aksi-aksi utama>
States: <daftar frame yang diminta, termasuk offline bila relevan>.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

---

## 4. Component Library Mobile / Style Guide (jalankan lebih dulu)

```
Sebelum mendesain layar, buat dulu satu artifact "Inventra Mobile — Component
Library" yang menampilkan semua komponen dasar mobile dalam light & dark mode,
sesuai master brief mobile, ditata dalam grid beranotasi nama komponen:
- Bottom navigation 5-slot dengan tombol Scan tengah menonjol (state per-tab aktif).
- AppBar layar utama (judul + ikon) dan AppBar layar sekunder (kembali + judul).
- Buttons: FilledButton primary, OutlinedButton, text button, destructive (merah),
  ukuran full-width vs compact; state disabled & loading (spinner).
- TextField (normal/fokus/error/disabled) + field password + search field.
- Card aset (foto kecil, nama, kode, badge status) dan ListTile dua baris.
- Chips/Badges: status aset (tersedia/dipinjam/maintenance/dilepas/hilang), hasil
  opname (ditemukan/rusak/salah lokasi/tidak ditemukan), status pengajuan
  (menunggu/disetujui/ditolak).
- SegmentedButton 4 opsi hasil opname.
- Banner offline (amber slim) + pill status sync ("12 belum tersinkron" / warna
  status: amber saat antre, hijau "Tersinkron", merah "Gagal — coba lagi").
- BottomSheet (hasil scan), AlertDialog konfirmasi, SnackBar sukses & error.
- LinearProgressIndicator + ring progress (untuk progres opname), skeleton loader,
  empty state (ikon + kalimat + tombol aksi), badge angka pada ikon (notifikasi).
Ini jadi acuan untuk semua layar berikutnya.
```

---

## 5. Prompt per-screen lengkap (siap copy-paste)

Kirim satu per satu setelah Master Brief (bagian 1) dan Component Library (bagian 4).

### 5.1 Login

```
Sekarang desain layar: Login.

Tujuan layar: Pegawai masuk dengan akun Inventra yang sama dengan web.
Pengguna utama: semua peran.
Navigasi: layar penuh tanpa bottom nav.
Elemen yang harus ada:
- Logo Inventra + wordmark di sepertiga atas, tagline singkat "Pendamping lapangan
  manajemen aset" — latar gradasi hijau halus (light) / slate gelap (dark).
- Card form: input Email, input Password (toggle lihat/sembunyikan), tombol
  full-width "Masuk".
- Tautan teks "Lupa password?" di bawah tombol Masuk (rata kanan) — membuka layar
  Lupa Password (bagian 5.20).
- Pesan error inline di atas form (mis. "Email atau password salah").
- Catatan kecil "Login Google menyusul" TIDAK perlu — cukup email+password saja.
- Footer kecil: versi aplikasi + switch bahasa (id/en).
States: 3 frame — form kosong (default) dengan tautan "Lupa password?", error kredensial,
tombol Masuk loading. Tampilkan versi light dan dark.

Patuhi master brief mobile.
```

### 5.2 Beranda (Home)

```
Sekarang desain layar: Beranda.

Tujuan layar: Ringkasan tugas lapangan pengguna hari ini dan pintu ke semua fitur.
Pengguna utama: Manager aset (tunjukkan juga varian Kepala Unit yang menonjolkan
approval).
Navigasi: bottom nav aktif di tab Beranda; tombol Scan tengah menonjol.
Elemen yang harus ada:
- Header sapaan: "Halo, Andi" + peran & kantor ("Asset Manager · Cabang Jakarta
  Selatan"), avatar kecil ke Profil, ikon lonceng dengan badge angka.
- Kartu "Sesi Opname Aktif": nama sesi, progress bar (mis. 128/150), pill status
  sync, tombol "Lanjutkan".
- Kartu "Approval Menunggu": angka besar + 2 baris pratinjau pengajuan teratas +
  tombol "Buka Inbox" (varian Kepala Unit: kartu ini di posisi teratas).
- Baris aksi cepat (ikon + label): Pindai Aset, Katalog Aset, Sesi Opname, Approval,
  Aset Saya, Pengajuan Saya, Notifikasi — grid ikon (mis. 2 baris) agar destinasi baru
  FR-M7 terjangkau dari Beranda. (Katalog, Aset Saya, Pengajuan Saya = layar baru.)
- Bila offline: banner amber slim di bawah AppBar "Offline — data terakhir
  ditampilkan".
States: 3 frame — data penuh (Manager), varian Kepala Unit, dan varian offline;
tambah 1 frame loading (skeleton kartu).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.3 Scan (kamera)

```
Sekarang desain layar: Scan.

Tujuan layar: Memindai label barcode/QR aset untuk membuka detailnya (atau menandai
hasil, bila dibuka dari sesi opname).
Pengguna utama: semua peran lapangan.
Navigasi: dibuka dari tombol Scan tengah bottom nav; tampil full-screen.
Elemen yang harus ada:
- Viewfinder kamera full-screen (gambarkan sebagai foto rak gudang gelap ber-blur)
  dengan bingkai target di tengah (sudut membulat, garis aksen hijau) + garis scan
  beranimasi tersirat.
- Kontrol atas: tombol tutup (X), judul "Pindai Label Aset", toggle senter.
- Bawah: tombol "Ketik kode manual" (membuka bottom sheet dengan TextField kode
  aset + tombol "Cari").
- Bottom sheet hasil sukses: foto kecil aset, nama, kode JKT01-ELK-2026-00001,
  badge status, lokasi ringkas, tombol full-width "Lihat Detail" — muncul menutupi
  sepertiga bawah, kamera tetap terlihat di belakang.
- State error: bottom sheet "Kode tidak dikenal / di luar wewenang Anda" dengan
  tombol "Pindai lagi" dan "Ketik manual".
States: 4 frame — viewfinder siap, bottom sheet hasil sukses, bottom sheet error,
bottom sheet input manual.
Tampilkan versi light dan dark (viewfinder gelap di keduanya; sheet mengikuti tema).

Patuhi master brief mobile.
```

### 5.4 Detail Aset

```
Sekarang desain layar: Detail Aset.

Tujuan layar: Menampilkan informasi satu aset hasil scan/pencarian — read-only.
Pengguna utama: Manager (tunjukkan juga varian dengan field harga disembunyikan).
Navigasi: AppBar dengan tombol kembali, tanpa bottom nav.
Elemen yang harus ada:
- Header: foto aset (carousel dots bila >1), nama besar, kode aset + ikon barcode
  kecil, badge status.
- Kartu "Penempatan": kantor, lantai/ruangan, pemegang saat ini.
- Kartu "Informasi": kategori, brand/model, kondisi, tanggal beli, vendor.
- Kartu "Nilai" (sensitif): harga beli, nilai buku — pada varian peran terbatas,
  nilainya diganti "—" dengan ikon kunci kecil dan keterangan "dibatasi".
- Bila dibuka dari sesi opname aktif: bar aksi sticky bawah "Tandai hasil:" dengan
  SegmentedButton (Ditemukan / Rusak / Salah Lokasi) — tunjukkan sebagai frame
  terpisah.
- **Bar aksi FR-M7 (per permission x STATUS aset)**: di luar sesi opname, bar sticky
  bawah menampilkan aksi yang MUNCUL SESUAI IZIN pengguna DAN status aset:
  - Aset `Tersedia`: **"Pinjam"** (Staf berizin `request.create`, membuka sheet ajukan
    peminjaman 5.14) atau **"Check-out"** (Manager berizin `assignment.manage`, sheet 5.14).
  - Aset `Dipinjam`: **"Check-in"** (Manager berizin `assignment.manage`, membuka sheet
    check-in 5.14 — catat kondisi masuk lalu aset kembali `Tersedia`).
  - Selalu (bila berizin `request.create`): **"Lapor Kerusakan"** (membuka sheet 5.15).
  - Bila pengguna tanpa izin aksi apa pun: tanpa bar aksi (murni read-only, seperti
    sebelumnya). Tanpa tombol overflow kosong — tampilkan hanya tombol yang berlaku.
States: 7 frame — detail penuh read-only, varian field dibatasi, varian dalam-sesi-opname
(bar tandai hasil), **varian aset Tersedia untuk Staf (Pinjam + Lapor Kerusakan)**,
**varian aset Tersedia untuk Manager (Check-out + Lapor Kerusakan)**, **varian aset
Dipinjam untuk Manager (Check-in + Lapor Kerusakan)**, loading (skeleton).
Tampilkan versi light dan dark.

Pakai data contoh: "Laptop Dell Latitude 5440", JKT01-ELK-2026-00001, Tersedia,
Cabang Jakarta Selatan, Lantai 2 / Ruang Operasional. Patuhi master brief mobile.
```

### 5.5 Inbox Approval

```
Sekarang desain layar: Inbox Approval.

Tujuan layar: Pejabat pemutus meninjau daftar pengajuan dalam lingkupnya.
Pengguna utama: Kepala Unit / Kepala Kanwil.
Navigasi: bottom nav aktif di tab Approval (badge angka menunggu).
Elemen yang harus ada:
- AppBar "Approval" + filter chips horizontal: Menunggu (default, dengan angka),
  Disetujui, Ditolak, Semua.
- List kartu pengajuan, tiap kartu: ikon tipe + label tipe (Registrasi Aset /
  Penghapusan / Mutasi / Peminjaman / Pengecualian Valuasi), judul ringkas
  ("Registrasi 12 Laptop Asus ExpertBook"), pengaju + kantor, nilai Rp (bila ada),
  waktu relatif ("2 jam lalu"), badge status. Pengajuan sensitif (Penghapusan,
  Pengecualian Valuasi) diberi penanda titik amber + label kecil "sensitif".
- Pull-to-refresh; pagination infinite scroll tersirat.
States: 4 frame — daftar Menunggu terisi, empty state ("Tidak ada pengajuan
menunggu" + ilustrasi ringan), loading skeleton, dan varian offline (banner amber +
data terakhir).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.6 Detail Approval

```
Sekarang desain layar: Detail Approval.

Tujuan layar: Meninjau satu pengajuan lalu menyetujui/menolak dengan catatan.
Pengguna utama: Kepala Unit / Kepala Kanwil.
Navigasi: AppBar dengan tombol kembali, tanpa bottom nav.
Elemen yang harus ada:
- Header: tipe pengajuan (badge) + judul + status; pengaju (avatar, nama, kantor)
  dan tanggal.
- Kartu ringkasan data yang diajukan: daftar field beserta nilainya; untuk perubahan
  tampilkan nilai lama dan baru berdampingan (nilai lama dicoret, nilai baru hijau).
  Untuk penghapusan: nilai buku vs nilai jual + laba/rugi berwarna.
- Kartu "Jenjang persetujuan": timeline vertikal (maker lalu checker berjenjang) dengan
  status per langkah (selesai/menunggu/berikutnya), nama & peran.
- Lampiran: baris berkas (ikon PDF/gambar + nama) — tap membuka pratinjau (cukup
  tersirat).
- Pengajuan sensitif: banner peringatan amber "Tindakan sensitif — periksa saksama".
- Bar aksi sticky bawah: TextField catatan 1 baris + dua tombol besar "Tolak"
  (outlined merah) dan "Setujui" (filled hijau). Dialog konfirmasi saat Tolak.
States: 4 frame — detail menunggu keputusan (bar aksi aktif), dialog konfirmasi
tolak, hasil setelah disetujui (bar aksi berganti status + SnackBar sukses),
varian pengajuan sensitif dengan banner.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia, rupiah Rp. Patuhi master brief mobile.
```

### 5.7 Notifikasi

```
Sekarang desain layar: Notifikasi.

Tujuan layar: Feed notifikasi in-app dengan tanda belum-dibaca dan deep-link.
Pengguna utama: semua peran.
Navigasi: bottom nav aktif di tab Notifikasi (badge angka belum dibaca).
Elemen yang harus ada:
- AppBar "Notifikasi" + aksi teks "Tandai semua dibaca".
- List dikelompokkan per hari ("Hari ini", "Kemarin", tanggal). Tiap item: ikon
  jenis (approval masuk = stempel, keputusan = centang, maintenance jatuh tempo =
  kunci pas, aset dikembalikan = kotak), judul + ringkasan 1 baris, waktu relatif,
  titik hijau untuk yang belum dibaca (latar item belum-dibaca sedikit lebih pekat).
- Contoh isi: "Pengajuan menunggu persetujuan Anda — Penghapusan 3 unit PC",
  "Pengajuan Anda disetujui — Registrasi 12 Laptop", "Maintenance jatuh tempo —
  AC Ruang Server (JKT01-ELK-2024-00031)".
- Tap item = deep-link (tersirat, tak perlu digambar).
States: 4 frame — feed terisi campuran dibaca/belum, empty state ("Belum ada
notifikasi"), loading skeleton, dan push notification OS di lockscreen (1 frame
bonus: tampilan notifikasi Android dengan ikon app hijau + judul + isi).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.8 Daftar Sesi Opname

```
Sekarang desain layar: Daftar Sesi Opname.

Tujuan layar: Melihat sesi stock opname dalam lingkup pengguna dan membuka/melanjutkannya.
Pengguna utama: Petugas opname / Manager.
Navigasi: bottom nav aktif di tab Opname.
Elemen yang harus ada:
- AppBar "Stock Opname" + filter chips: Berjalan, Selesai, Semua.
- List kartu sesi, tiap kartu: nama sesi ("Opname Tahunan 2026 — Cabang Jakarta
  Selatan"), lingkup kantor, periode, badge status (berjalan/selesai), progress bar
  + angka (128/150 tercocokkan), dan BARIS STATUS LOKAL: "Snapshot tersimpan di
  perangkat" (ikon unduh-selesai hijau) atau tombol kecil "Unduh snapshot" bila
  belum, plus pill sync ("12 belum tersinkron") bila ada antrean.
- Kartu sesi berjalan punya tombol utama "Lanjutkan Menghitung"; sesi selesai
  read-only dengan label "Berita Acara di web".
- Catatan kecil di bawah: "Sesi dibuat dan diselesaikan dari aplikasi web".
States: 4 frame — daftar terisi (satu sesi berjalan dengan antrean sync + satu
selesai), empty state ("Tidak ada sesi opname aktif"), loading skeleton, varian
offline (banner amber; sesi ber-snapshot tetap bisa dibuka, yang belum diunduh
disabled dengan keterangan).
Tampilkan versi light dan dark.

Patuhi master brief mobile.
```

### 5.9 Opname Counting (scan + offline)

```
Sekarang desain layar: Opname Counting.

Tujuan layar: Layar kerja utama petugas opname — memindai aset satu per satu dan
menandai hasilnya, bekerja penuh saat offline.
Pengguna utama: Petugas opname.
Navigasi: AppBar dengan tombol kembali + nama sesi; tanpa bottom nav (mode fokus).
Elemen yang harus ada:
- Header sticky ringkas: progress ring + angka besar "128/150", rincian kecil per
  hasil (ikon + angka: 120 ditemukan, 5 rusak, 3 salah lokasi), dan PILL STATUS
  SYNC di kanan ("12 belum tersinkron" amber / "Tersinkron" hijau).
- Bila offline: banner amber slim "Offline — scan tersimpan di perangkat".
- Tombol scan BESAR full-width dekat ibu jari: "Pindai Aset Berikutnya" (ikon
  barcode) + tombol sekunder "Ketik kode".
- Di bawahnya, list "Baru saja dipindai" (terbaru di atas): tiap baris nama + kode
  aset, waktu, badge hasil, dan ikon status sync per item (jam pasir = antre,
  centang = tersinkron, segitiga merah = konflik).
- Setelah scan sukses, bottom sheet cepat: nama aset + SegmentedButton hasil
  (Ditemukan ✓ default / Rusak / Salah Lokasi) + field catatan opsional + tombol
  "Simpan & Lanjut" — dirancang untuk ritme scan-simpan-scan yang cepat.
- Baris konflik (dari first-write-wins server): latar merah muda tipis, keterangan
  "Sudah dicatat petugas lain: Ditemukan" + tombol kecil "Lihat".
States: 5 frame — counting online normal, bottom sheet hasil scan, varian OFFLINE
(banner + pill antrean bertambah), varian ada KONFLIK di list, dan varian selesai
sinkron ("Tersinkron" hijau + semua item centang).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.10 Variance & Tindak Lanjut

```
Sekarang desain layar: Variance Opname.

Tujuan layar: Melihat selisih hasil opname sebuah sesi dan penanda tindak lanjutnya.
Pengguna utama: Petugas opname / Manager.
Navigasi: AppBar dengan tombol kembali (dari detail sesi); tab/segmen di atas:
"Item" | "Variance" (layar ini tab Variance).
Elemen yang harus ada:
- Kartu ringkasan atas: 4 angka — Tidak Ditemukan (merah), Rusak (amber), Salah
  Lokasi (biru), Di Luar Catatan (slate, aset fisik tak terdaftar di snapshot).
- List variance dikelompokkan per jenis; tiap item: nama + kode aset, lokasi
  tercatat vs ditemukan (untuk salah lokasi), catatan petugas.
- Per item, baris tindak lanjut: label status ("Belum ditindaklanjuti" / "Diajukan:
  Penghapusan") — dengan tombol kecil "Tindak lanjut" yang membuka bottom sheet
  pilihan (Ajukan Penghapusan / Ajukan Mutasi / Laporkan Kerusakan) + catatan bahwa
  pengajuan diproses lewat approval seperti biasa.
- Footer info: "Penyelesaian sesi & Berita Acara dilakukan dari aplikasi web".
States: 4 frame — variance terisi campuran, bottom sheet tindak lanjut terbuka,
empty state ("Tidak ada selisih — semua aset tercocokkan" dengan ilustrasi
positif), loading skeleton.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.11 Profil & Sesi Perangkat

```
Sekarang desain layar: Profil & Sesi Perangkat.

Tujuan layar: Melihat & menyunting identitas akun, kelola foto profil, dan kelola sesi
login perangkat. (Diperluas FR-M6.1/6.2 — sebelumnya read-only.)
Pengguna utama: semua peran.
Navigasi: dibuka dari avatar di Beranda; AppBar dengan tombol kembali. Tombol "Ubah" di
header kartu Data Diri berpindah ke mode Simpan/Batal saat menyunting.
Elemen yang harus ada:
- Header profil: **avatar besar dengan tombol kamera kecil** (unggah/ganti; long-press
  atau menu untuk Hapus foto — tombol Hapus hanya muncul bila sudah ada foto), nama,
  badge peran "Asset Manager".
- Kartu "Data Diri": field yang BOLEH diedit (mis. nama, telepon) — read-only saat
  default, jadi TextField saat mode Ubah; di dalamnya blok "Detail Pegawai" **read-only**
  (kode pegawai, status, departemen, jabatan — bersumber master data). Akun tanpa tautan
  pegawai menampilkan catatan singkat, bukan grid kosong.
- Kartu "Informasi Akun" (read-only): peran, kantor, metode login, tanggal bergabung.
- Baris tautan "Keamanan Akun" (ke bagian 5.19: ganti password/email) dan "Pengaturan".
- Seksi "Sesi Perangkat": list sesi — tiap baris ikon perangkat (ponsel/laptop),
  nama perangkat + browser/app, lokasi ± IP, waktu aktif terakhir; sesi SAAT INI
  ditandai badge hijau "Perangkat ini". Baris lain punya aksi "Cabut".
- Tombol "Keluar dari semua perangkat lain" (outlined) + tombol "Keluar" (merah,
  full-width, paling bawah). Keduanya dengan dialog konfirmasi.
States: 6 frame — profil lengkap (mode baca) + 3 sesi (1 mobile ini, 1 Chrome Windows,
1 mobile lain); **mode Ubah Data Diri (field jadi TextField + Simpan/Batal)**; **menu
foto profil (Ganti / Hapus)**; dialog konfirmasi "Keluar dari semua perangkat lain";
setelah cabut satu sesi (SnackBar sukses + list menyusut); loading skeleton.
Tampilkan versi light dan dark.

Patuhi master brief mobile.
```

### 5.12 Pengaturan

```
Sekarang desain layar: Pengaturan.

Tujuan layar: Preferensi aplikasi — bahasa, tema, dan info aplikasi.
Pengguna utama: semua peran.
Navigasi: AppBar dengan tombol kembali.
Elemen yang harus ada:
- Grup "Tampilan": pilihan Tema (radio/list: Terang / Gelap / Ikuti Sistem, dengan
  pratinjau kecil), pilihan Bahasa (Indonesia / English).
- Grup "Notifikasi": toggle "Notifikasi push" (dengan keterangan jenis yang
  dikirim: approval & maintenance), tautan "Pengaturan sistem" bila izin OS mati
  (baris peringatan amber "Izin notifikasi dimatikan di sistem").
- Grup "Penyimpanan": baris "Data opname lokal" dengan ukuran (mis. 2,4 MB) +
  keterangan "terhapus otomatis setelah sesi tersinkron" — tanpa tombol hapus
  manual.
- Grup "Tentang": versi aplikasi, tautan bantuan/runbook internal.
States: 3 frame — pengaturan default (tema ikuti sistem), varian izin notifikasi OS
dimatikan (baris peringatan), dan pemilih tema terbuka.
Tampilkan versi light dan dark.

Patuhi master brief mobile.
```

---

## Perluasan scope 2026-07-21 — prompt layar baru (fase M7 & M8)

Layar berikut menambah scope mobile v1 sesuai PRD mobile v1.1 (FR-M7 aksi aset; FR-M6/M1.5
profil & keamanan). Semua memakai endpoint backend yang sudah ada (nol backend baru).
Generate memakai Master Brief (bagian 1) + Component Library (bagian 4) yang sama.

### 5.13 Katalog Aset

```
Sekarang desain layar: Katalog Aset.

Tujuan layar: Menelusuri daftar aset dalam lingkup pengguna tanpa harus memindai —
read-only, melengkapi Scan.
Pengguna utama: semua peran lapangan (isi mengikuti data scope + field permission server).
Navigasi: destinasi sekunder (AppBar + tombol kembali), dijangkau dari aksi cepat di
Beranda. Bottom nav tetap 5 slot dan tidak berubah.
Elemen yang harus ada:
- AppBar "Katalog Aset" + search field (cari nama/kode aset).
- Baris filter chips: Kategori, Status (Tersedia/Dipinjam/Maintenance/Dilepas/Hilang),
  Kantor — membuka bottom sheet pilihan; chip aktif menampilkan nilai terpilih.
- List Card aset: foto kecil, nama, kode (JKT01-ELK-2026-00001), badge status, lokasi
  ringkas; tap membuka Detail Aset. Pull-to-refresh; infinite scroll (pagination server).
- Field sensitif tidak ditampilkan di kartu katalog.
States: 4 frame — daftar terisi, hasil pencarian dengan filter aktif (chips terisi),
empty state ("Tidak ada aset yang cocok" + ikon + tombol "Reset filter"), loading skeleton.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.14 Peminjaman / Check-out / Check-in (bottom sheet dari Detail Aset)

```
Sekarang desain layar: Peminjaman / Check-out / Check-in (bottom sheet).

Tujuan layar: Aksi penugasan aset dari Detail Aset — TIGA alur sesuai peran & status aset.
Pengguna utama: Staf (ajukan peminjaman) dan Manager (check-out & check-in langsung).
Navigasi: bottom sheet menutupi bagian bawah di atas Detail Aset (detail tetap terlihat di
belakang); handle tarik di atas, judul, tombol tutup.
Elemen yang harus ada:
- Ringkasan aset di atas sheet (foto kecil, nama, kode, badge status).
- Varian STAF — "Ajukan Peminjaman" (aset Tersedia): tanggal pinjam (default hari ini),
  jatuh tempo (opsional, date picker kalender), catatan/alasan (TextField), tombol
  full-width "Ajukan". Keterangan kecil "Menunggu persetujuan" — ini pengajuan via approval.
- Varian MANAGER — "Check-out" (aset Tersedia): pemilih pegawai/custodian (autocomplete
  dengan empty state "Tidak ada data" bila kosong), tanggal pinjam, jatuh tempo opsional,
  catatan kondisi keluar, tombol full-width "Check-out". Keterangan "Aset langsung menjadi
  Dipinjam".
- Varian MANAGER — "Check-in" (aset Dipinjam): tampilkan pemegang saat ini (nama pegawai +
  sejak tanggal), field kondisi masuk (chips: Baik / Perlu Servis), catatan opsional, tombol
  full-width "Check-in". Keterangan "Aset kembali Tersedia (atau Maintenance bila perlu
  servis)".
- Validasi inline (mis. custodian wajib untuk check-out).
States: 6 frame — sheet Ajukan Peminjaman (Staf), sukses ajukan (SnackBar "Pengajuan
peminjaman dikirim"), sheet Check-out (Manager) dengan autocomplete pegawai terbuka, sheet
Check-in (Manager) dengan kondisi masuk, sukses (SnackBar + badge aset berubah status),
varian error validasi.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.15 Lapor Kerusakan (bottom sheet dari Detail Aset)

```
Sekarang desain layar: Lapor Kerusakan (bottom sheet).

Tujuan layar: Mengajukan laporan kerusakan/maintenance untuk sebuah aset dari lapangan.
Pengguna utama: semua peran berizin mengajukan.
Navigasi: bottom sheet dari Detail Aset (bar aksi "Lapor Kerusakan").
Elemen yang harus ada:
- Ringkasan aset (foto kecil, nama, kode).
- Field deskripsi kerusakan (TextField multi-baris, wajib), tingkat/severity opsional
  (chips: Ringan/Sedang/Berat), lampiran foto opsional (tombol "Tambah foto" + thumbnail
  grid, dari kamera/galeri).
- Keterangan kecil "Diproses sebagai pengajuan maintenance lewat approval".
- Tombol full-width "Kirim Laporan".
States: 4 frame — form kosong, form terisi dengan 2 foto lampiran, error validasi
(deskripsi kosong), sukses kirim (SnackBar "Laporan kerusakan dikirim").
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.16 Form Registrasi Aset

```
Sekarang desain layar: Form Registrasi Aset.

Tujuan layar: Mengajukan pendaftaran aset baru dari lapangan — FORM PENUH bergaya
multi-langkah (stepper) agar muat di layar ponsel; hati-hati pada field finansial.
Pengguna utama: Manager / peran berizin registrasi.
Navigasi: destinasi sekunder (AppBar + kembali), dijangkau dari Katalog/Beranda atau
Detail Aset. Stepper 3 langkah dengan indikator progres di atas.
Field mengikuti payload registrasi web (asset_create) — TANPA cek ambang kapitalisasi
(web tidak punya itu; aset baru selalu dikapitalisasi oleh server).
Elemen yang harus ada:
- Langkah 1 "Identitas": kategori (autocomplete dengan empty state "Tidak ada data"),
  nama aset, kelas aset (asset_class), brand/model/unit (opsional), nomor seri (opsional).
- Langkah 2 "Penempatan & Perolehan": kantor (default kantor pengguna, dibatasi scope),
  ruangan (opsional), harga perolehan (TextField NUMERIK-ONLY, non-negatif, prefiks Rp,
  opsional), tanggal perolehan (kalender, opsional), vendor/PO/sumber dana/garansi
  (opsional), catatan (opsional).
- Langkah 3 "Tinjau & Kirim": ringkasan semua field read-only + keterangan "Diproses
  sebagai pengajuan registrasi lewat approval; nilai pengajuan = harga perolehan", tombol
  "Kirim Pengajuan".
- Navigasi antar langkah: tombol Kembali/Lanjut; validasi per langkah (kategori & nama
  wajib; harga bila diisi harus angka valid) sebelum boleh lanjut.
States: 5 frame — langkah 1, langkah 2 dengan input harga (angka), error validasi numerik
(mis. huruf ditolak pada harga), langkah 3 tinjau, sukses kirim (SnackBar + arahkan ke
Pengajuan Saya).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia, rupiah Rp. Patuhi master brief mobile.
```

### 5.17 Pengajuan Saya

```
Sekarang desain layar: Pengajuan Saya.

Tujuan layar: Lensa MAKER — semua pengajuan yang DIBUAT pengguna sendiri beserta
statusnya (beda dari Inbox Approval yang berorientasi keputusan).
Pengguna utama: semua peran (semua bisa mengajukan).
Navigasi: destinasi sekunder (AppBar + kembali), dijangkau dari Beranda/Profil.
Elemen yang harus ada:
- AppBar "Pengajuan Saya" + filter chips: Menunggu, Disetujui, Ditolak, Semua.
- List kartu pengajuan (mirip Inbox Approval tetapi TANPA tombol setujui/tolak): ikon +
  label tipe (Registrasi / Peminjaman / Laporan Kerusakan / Mutasi / Penghapusan), judul
  ringkas, nilai Rp bila ada, waktu relatif, badge status.
- Untuk pengajuan berstatus MENUNGGU: tombol kecil "Batalkan" (dengan dialog konfirmasi)
  — hanya berlaku untuk pengajuan sendiri yang masih pending.
- Tap kartu membuka detail pengajuan (read-only, timeline jenjang approval; tanpa aksi
  keputusan).
States: 5 frame — daftar campuran status, filter "Menunggu" dengan tombol Batalkan,
dialog konfirmasi Batalkan, empty state ("Belum ada pengajuan"), loading skeleton.
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.18 Aset Saya

```
Sekarang desain layar: Aset Saya.

Tujuan layar: Daftar aset yang sedang DIPEGANG/ditugaskan ke pengguna — menu tersendiri,
read-only.
Pengguna utama: semua peran (khususnya Staf pemegang aset).
Navigasi: destinasi sekunder (AppBar + kembali), menu tersendiri dijangkau dari
Beranda/Profil.
Elemen yang harus ada:
- AppBar "Aset Saya" + hitungan ("5 aset dipegang").
- List Card aset: foto kecil, nama, kode, badge status (umumnya Dipinjam), tanggal pinjam
  dan JATUH TEMPO; item yang MELEWATI jatuh tempo diberi penanda merah "Terlambat".
- Tap membuka Detail Aset. Pull-to-refresh.
States: 4 frame — daftar terisi (termasuk satu item terlambat), empty state ("Anda belum
memegang aset apa pun"), loading skeleton, varian offline (banner amber + data terakhir).
Tampilkan versi light dan dark.

Pakai data contoh realistis berbahasa Indonesia. Patuhi master brief mobile.
```

### 5.19 Keamanan Akun

```
Sekarang desain layar: Keamanan Akun.

Tujuan layar: Ganti password dan ganti email — KEDUANYA berbasis link email (mobile
memulai, penetapan/konfirmasi diselesaikan di halaman web via link).
Pengguna utama: semua peran.
Navigasi: destinasi sekunder (AppBar + kembali), dijangkau dari Profil.
Elemen yang harus ada:
- Baris "Email" saat ini (read-only) + tombol "Ganti Email".
- Baris "Password" + tombol "Ganti Password".
- Sheet "Ganti Password": HANYA field password lama (verifikasi) + tombol "Kirim Link
  Reset"; keterangan "Kami kirim link ke email Anda untuk menyetel password baru" dan
  peringatan kecil "Semua sesi akan keluar setelah password diganti".
- Sheet "Ganti Email": field email baru + tombol "Kirim Link Verifikasi"; keterangan
  "Buka link di email baru untuk mengonfirmasi".
- State konfirmasi "Cek email Anda" (ikon amplop + alamat email tertutup sebagian +
  tombol "Selesai") — muncul setelah link dikirim; TIDAK ada field set-password di mobile.
States: 5 frame — menu Keamanan Akun, sheet Ganti Password (input password lama), konfirmasi
"Link reset terkirim — cek email", sheet Ganti Email (input email baru), konfirmasi "Link
verifikasi terkirim".
Tampilkan versi light dan dark.

Patuhi master brief mobile.
```

### 5.20 Lupa Password

```
Sekarang desain layar: Lupa Password.

Tujuan layar: Memulai reset password dari layar Login (belum masuk); penetapan password
baru diselesaikan lewat link email di halaman web.
Pengguna utama: pengguna yang belum login.
Navigasi: dibuka dari tautan "Lupa password?" di Login; AppBar dengan tombol kembali.
Elemen yang harus ada:
- Judul + kalimat penjelas "Masukkan email akun Anda; kami kirim link untuk menyetel
  password baru".
- Field Email + tombol full-width "Kirim Link Reset".
- State konfirmasi ANTI-ENUMERASI: "Jika email terdaftar, kami telah mengirim link reset.
  Cek kotak masuk Anda." (pesan SAMA baik email ada maupun tidak — jangan bocorkan
  keberadaan akun), dengan tombol "Kembali ke Login".
States: 3 frame — form email, tombol loading saat mengirim, konfirmasi anti-enumerasi.
Tampilkan versi light dan dark.

Patuhi master brief mobile.
```
