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

Checklist v1 — 12 layar + 1 component library. Simpan hasil tiap layar sebagai
`docs/mobile/design/<Nama Layar>.dc.html`.

**Fondasi (fase M0)**
0. Component Library Mobile (bagian 4 — jalankan pertama)
1. Login — bagian 5.1
2. Beranda (Home) — bagian 5.2

**Scan & aset (fase M1)**
3. Scan (kamera) — bagian 5.3
4. Detail Aset — bagian 5.4

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
11. Profil & Sesi Perangkat — bagian 5.11
12. Pengaturan — bagian 5.12

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
- Pesan error inline di atas form (mis. "Email atau password salah").
- Catatan kecil "Login Google menyusul" TIDAK perlu — cukup email+password saja.
- Footer kecil: versi aplikasi + switch bahasa (id/en).
States: 3 frame — form kosong (default), error kredensial, tombol Masuk loading.
Tampilkan versi light dan dark.

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
- Baris aksi cepat (ikon + label): Pindai Aset, Sesi Opname, Approval, Notifikasi.
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
States: 4 frame — detail penuh, varian field dibatasi, varian dalam-sesi-opname
(bar aksi bawah), loading (skeleton).
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
- Kartu ringkasan data yang diajukan: daftar field → nilai; untuk perubahan tampilkan
  before → after (nilai lama dicoret, nilai baru hijau). Untuk penghapusan: nilai
  buku vs nilai jual + laba/rugi berwarna.
- Kartu "Jenjang persetujuan": timeline vertikal (maker → checker berjenjang) dengan
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

Tujuan layar: Melihat identitas akun dan mengelola sesi login perangkat.
Pengguna utama: semua peran.
Navigasi: dibuka dari avatar di Beranda; AppBar dengan tombol kembali.
Elemen yang harus ada:
- Header profil: avatar besar, nama "Andi Saputra", badge peran "Asset Manager",
  email, kantor "Cabang Jakarta Selatan". (Read-only — penyuntingan profil di web.)
- Seksi "Sesi Perangkat": list sesi — tiap baris ikon perangkat (ponsel/laptop),
  nama perangkat + browser/app, lokasi ± IP, waktu aktif terakhir; sesi SAAT INI
  ditandai badge hijau "Perangkat ini". Baris lain punya aksi "Cabut".
- Tombol "Keluar dari semua perangkat lain" (outlined) + tombol "Keluar" (merah,
  full-width, paling bawah). Keduanya dengan dialog konfirmasi.
- Tautan kecil ke Pengaturan.
States: 4 frame — profil + 3 sesi (1 mobile ini, 1 Chrome Windows, 1 mobile lain),
dialog konfirmasi "Keluar dari semua perangkat lain", setelah cabut satu sesi
(SnackBar sukses + list menyusut), loading skeleton.
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
