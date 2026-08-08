# Spesifikasi: Panduan Penggunaan berbasis data dengan lampiran video dan PDF

- Tanggal: 2026-08-08
- Fase: Define (`/spec`), agent `analyst`
- Handoff berikutnya: `/plan` (agent `planner`, skill `vertical-slice` dan `file-upload`)

## Masalah

Isi halaman Panduan Penggunaan terkunci di dalam kode. Sembilan modul panduan
ditulis sebagai kunci i18n (`guidePage.sections` di `frontend/i18n/locales/{id,en}.json`)
dan dirender statis oleh `frontend/app/pages/guide.vue`. Akibatnya dua hal: pertama,
setiap kali alur aplikasi berubah atau muncul pertanyaan berulang ke helpdesk, perbaikan
panduan harus menunggu siklus rilis dan orang yang bisa mengubahnya hanya developer.
Kedua, penjelasan hanya bisa berbentuk teks dan daftar langkah, padahal sebagian alur
(mutasi aset, stock opname, approval berjenjang) jauh lebih cepat dipahami lewat rekaman
layar atau dokumen ber-tangkapan-layar. User yang bingung hari ini tidak punya jalan lain
selain bertanya ke helpdesk.

## Pengguna dan pemicu

| Peran | Yang dilakukan | Frekuensi | Pemicu |
|---|---|---|---|
| Pengelola panduan (pemegang `guide.manage`, awalnya superadmin) | Membuat, mengubah, menerbitkan modul panduan beserta lampiran video/PDF | Rendah, hitungan kali per bulan | Alur aplikasi berubah, fitur baru rilis, pertanyaan berulang ke helpdesk |
| User terautentikasi | Membaca panduan, menonton video, membuka PDF | Sesekali, terutama user baru | Bingung memakai satu modul, onboarding pegawai baru |
| Pengunjung tanpa sesi | Membaca teks panduan | Jarang | Tidak bisa masuk, membuka tautan panduan dari luar aplikasi |

## Keadaan yang ada (hasil pemeriksaan kode)

| Temuan | Berkas | Konsekuensi untuk fitur ini |
|---|---|---|
| Panduan dirender dari `guidePage.sections`: judul, ikon, body, daftar langkah | `frontend/app/pages/guide.vue`, `frontend/i18n/locales/{id,en}.json` | Bentuk data baru harus bisa menampung struktur yang sama persis supaya tampilan tidak berubah |
| `/guide` terdaftar sebagai rute publik | `frontend/app/middleware/auth.global.ts` baris 7 | Halaman bisa dibuka tanpa sesi; lampiran tidak boleh ikut publik |
| Layout khusus halaman informasi (privacy, guide, faq) | `frontend/app/layouts/info.vue` | Halaman pengelolaan harus terpisah, di area terproteksi dengan layout `default` |
| MinIO sudah terpasang: `Put`, `Get`, `Remove` | `backend/internal/storage/minio.go` | Tidak perlu integrasi storage baru |
| Pola sajikan berkas lewat proxy API, bukan presigned URL, dengan `Content-Disposition` aman RFC 6266 | `backend/internal/asset/attachment_handler.go` baris 17 dan 193 | Penyajian PDF panduan mengikuti pola yang sama sehingga otorisasi tetap di tangan backend |
| Validasi unggahan: allowlist MIME dan batas ukuran diperiksa sebelum menyentuh DB atau storage | `backend/internal/asset/attachment.go` baris 21 dan 60 | Pola validasi diikuti, batasnya sendiri lebih besar (lihat aturan bisnis) |
| Batas unggahan yang ada: lampiran aset 5 MB, impor 10 MB | `backend/internal/config/config.go` baris 150 dan 181 | Butuh batas sendiri untuk PDF panduan, tidak menumpang `ATTACHMENT_MAX_BYTES` |
| Modul audit tersedia | `backend/internal/audit` | Perubahan konten panduan wajib tercatat |
| `mobile/lib` tidak punya layar panduan sama sekali | pencarian "panduan"/"guide" di `mobile/lib` nihil | Mobile di luar cakupan; bukan "mematikan fitur", memang belum ada |
| Migration terakhir `000049` | `backend/db/migrations/` | Migration baru mulai dari `000050` |

## Keputusan yang sudah diambil

| Pertanyaan | Keputusan | Alasan |
|---|---|---|
| Bentuk video | Tautan YouTube, tidak mengunggah berkas video | Ditetapkan user; menghindari beban storage dan streaming |
| Bentuk dokumen | Berkas PDF diunggah ke MinIO | Ditetapkan user |
| Akses media | Teks panduan tetap publik, lampiran video dan PDF hanya untuk user yang sudah login | PDF panduan internal dapat memuat tangkapan layar data dan alur kerja nyata |
| Siapa yang mengelola | Permission baru `guide.manage`, di-seed ke superadmin, bisa diberikan ke peran lain lewat menu RBAC | Mengikuti pola `transfer.manage`, `disposal.manage`; tidak melebarkan arti permission yang sudah ada |
| Struktur modul | Seluruh panduan menjadi data di DB: judul, ikon, deskripsi, langkah, dan lampiran | Ditetapkan user; panduan bisa berubah tanpa deploy |
| Lingkup kantor | Global, satu set untuk seluruh organisasi | Panduan menjelaskan cara memakai aplikasi, bukan data operasional kantor |
| Dua bahasa | Konten Indonesia wajib, Inggris opsional dengan fallback ke Indonesia | Menjaga aturan i18n tanpa memaksa pengelola menulis dua versi |
| Konten lama | Sembilan modul yang ada dipindahkan ke DB lewat seed migration | Setelah rilis halaman terlihat sama persis, tidak ada momen halaman kosong |
| Publikasi | Ada status draf dan terbit | Halaman ini terbaca publik; panduan setengah jadi tidak boleh tayang |

## Alur utama

### Pengelola panduan

1. Pengelola membuka menu pengelolaan panduan di area terproteksi.
2. Sistem menampilkan daftar modul beserta status (draf atau terbit), urutan, jumlah
   lampiran, dan penanda modul yang belum punya terjemahan Inggris.
3. Pengelola membuat modul baru: judul Indonesia (wajib), judul Inggris (opsional),
   ikon, deskripsi, dan daftar langkah bernomor.
4. Sistem menyimpan modul berstatus draf.
5. Pengelola menambahkan lampiran ke modul itu, memilih jenis video atau dokumen.
   Untuk video ia menempelkan tautan YouTube; untuk dokumen ia mengunggah berkas PDF.
6. Sistem memvalidasi tautan atau berkas, menyimpan lampiran, dan menampilkannya di
   pratinjau modul.
7. Pengelola menerbitkan modul.
8. Sistem menandai modul terbit dan mencatat perubahan ke audit trail.

### Pembaca

1. User membuka halaman Panduan Penggunaan.
2. Sistem menampilkan modul berstatus terbit, urut sesuai urutan yang ditetapkan.
3. Untuk user yang sudah login, kartu lampiran menampilkan pemutar video tersemat dan
   tautan buka PDF.
4. Untuk pengunjung tanpa sesi, kartu lampiran menampilkan judul lampiran, jenisnya, dan
   ajakan masuk. Sistem tidak mengirimkan identitas video maupun tautan berkas.

## Acceptance criteria

### A. Konten modul

- **AC1** Given saya pemegang `guide.manage`, When saya menyimpan modul dengan judul
  Indonesia, ikon, dan urutan terisi, Then modul tersimpan dengan status draf dan muncul
  di daftar pengelolaan.
- **AC2** Given saya mengisi form modul, When judul Indonesia dikosongkan, Then sistem
  menolak dengan 422 dan pesan kesalahan menempel pada field judul Indonesia, dan tidak
  ada baris baru di DB.
- **AC3** Given sebuah modul, When saya menambahkan nol langkah, Then modul tetap valid
  dan halaman pembaca menampilkan judul beserta deskripsi tanpa daftar bernomor (perilaku
  ini sama dengan modul Dashboard yang sekarang memang tanpa langkah).
- **AC4** Given modul dengan lima langkah, When saya mengubah urutan langkah lalu
  menyimpan, Then halaman pembaca menampilkan langkah dalam urutan baru, bernomor ulang
  dari satu.
- **AC5** Given modul yang sudah terbit, When saya menghapusnya, Then modul beserta
  lampirannya hilang dari halaman pembaca dan dari daftar pengelolaan (soft delete,
  `deleted_at` terisi), dan permintaan langsung ke endpoint berkas lampirannya menjawab
  404.
- **AC6** Given dua modul dengan urutan yang sama, When halaman pembaca dimuat, Then
  urutan tampil bersifat deterministik (urutan lalu tanggal buat), bukan acak antar-muat.

### B. Otorisasi

- **AC7** Given saya user terautentikasi tanpa `guide.manage`, When saya memanggil
  endpoint pembuatan, pengubahan, penghapusan, penerbitan modul, atau unggah lampiran,
  Then setiap permintaan dijawab 403 dan tidak ada perubahan data.
- **AC8** Given saya tidak punya sesi, When saya memanggil endpoint tulis mana pun di
  modul panduan, Then dijawab 401.
- **AC9** Given saya user terautentikasi tanpa `guide.manage`, When saya memanggil
  endpoint daftar panduan dengan parameter yang meminta modul draf, Then respons hanya
  berisi modul terbit.
- **AC10** Given permission `guide.manage` baru ditambahkan, When superadmin membuka
  menu pengaturan peran, Then permission itu tampil dan bisa diberikan atau dicabut untuk
  peran lain, dan pencabutan berlaku setelah cache permission di Redis diinvalidasi
  (tanpa restart backend).
- **AC11** Given peran saya baru saja diberi `guide.manage`, When saya membuka menu
  aplikasi, Then entri menu pengelolaan panduan muncul; bagi peran tanpa permission itu
  entri menu tidak dirender sama sekali.

### C. Lampiran video (YouTube)

- **AC12** Given saya menambahkan lampiran video, When saya menempelkan tautan berbentuk
  `https://www.youtube.com/watch?v=<id>`, `https://youtu.be/<id>`,
  `https://www.youtube.com/shorts/<id>`, atau `https://m.youtube.com/watch?v=<id>`, Then
  sistem menerima dan menyimpan identitas video hasil ekstraksi, bukan URL mentahnya.
- **AC13** Given saya menempelkan tautan YouTube yang membawa parameter tambahan
  (`&t=90`, `&list=...`, `&si=...`), When lampiran disimpan, Then hanya identitas video
  yang tersimpan dan parameter lain dibuang.
- **AC14** Given saya menempelkan tautan dengan host selain `youtube.com`, `youtu.be`,
  atau subdomainnya, When saya menyimpan, Then sistem menolak dengan 422. Kasus yang
  harus ditolak mencakup minimal: `https://vimeo.com/123`, `javascript:alert(1)`,
  `data:text/html,...`, `https://evil.example.com/youtube.com/watch?v=x`, dan
  `http://youtube.com.evil.example.com/watch?v=x`.
- **AC15** Given identitas video hasil ekstraksi mengandung karakter di luar
  `[A-Za-z0-9_-]` atau panjangnya bukan 11 karakter, When saya menyimpan, Then sistem
  menolak dengan 422.
- **AC16** Given modul dengan lampiran video dan saya sudah login, When halaman pembaca
  dimuat, Then video disematkan lewat `https://www.youtube-nocookie.com/embed/<id>`,
  iframe punya atribut `title` yang terisi judul lampiran, dan tidak ada permintaan ke
  domain YouTube sebelum user menekan putar (pemuatan lazy).
- **AC17** Given saya menyimpan tautan YouTube yang formatnya benar tapi videonya tidak
  ada atau privat, When halaman pembaca dimuat, Then halaman tetap dirender utuh dan
  kegagalan pemutaran terbatas di dalam kartu video itu saja. Sistem tidak diwajibkan
  memverifikasi keberadaan video ke YouTube saat menyimpan.

### D. Lampiran PDF

- **AC18** Given saya mengunggah berkas ber-ekstensi `.pdf` yang isinya bukan PDF
  (misalnya HTML atau PNG yang di-rename), When unggahan diproses, Then sistem menolak
  dengan 415 berdasarkan pemeriksaan isi berkas, bukan hanya ekstensi atau header
  `Content-Type` dari klien.
- **AC19** Given berkas PDF melebihi batas ukuran yang dikonfigurasi, When saya
  mengunggah, Then sistem menolak dengan 413 dan pesan menyebutkan batas dalam MB, dan
  tidak ada objek yang tertinggal di MinIO.
- **AC20** Given unggahan PDF yang valid, When berhasil, Then baris lampiran menyimpan
  nama berkas asli untuk ditampilkan, sedangkan kunci objek di MinIO dibangkitkan sistem
  (berbasis UUID). Nama berkas dari user tidak pernah menjadi bagian dari path objek.
- **AC21** Given saya sudah login dan membuka lampiran PDF, When berkas disajikan, Then
  responsnya `application/pdf` dengan `X-Content-Type-Options: nosniff` dan
  `Content-Disposition: inline` berisi nama berkas yang sudah disanitasi, mengikuti pola
  `contentDisposition` di modul aset.
- **AC22** Given saya tidak punya sesi, When saya memanggil endpoint berkas lampiran
  secara langsung dengan identitas lampiran yang benar, Then dijawab 401 dan isi berkas
  tidak terkirim.
- **AC23** Given penyimpanan objek gagal di tengah unggahan, When permintaan selesai,
  Then tidak ada baris lampiran yang tertinggal di DB; sebaliknya jika penulisan baris
  gagal setelah objek tersimpan, objek dihapus sebagai rollback best-effort dan kegagalan
  itu tercatat di log.
- **AC24** Given saya menghapus satu lampiran PDF, When penghapusan berhasil, Then baris
  lampirannya soft-delete, endpoint berkasnya menjawab 404, dan objek MinIO-nya dihapus.

### E. Akses publik dibanding terautentikasi

- **AC25** Given saya pengunjung tanpa sesi, When saya membuka halaman panduan, Then
  seluruh modul terbit tampil lengkap dengan judul, deskripsi, dan langkah-langkahnya
  (perilaku publik yang ada sekarang tidak berubah).
- **AC26** Given modul terbit punya lampiran dan saya tanpa sesi, When halaman dimuat,
  Then kartu lampiran menampilkan judul lampiran dan jenisnya beserta ajakan masuk, dan
  respons API publik tidak memuat identitas video maupun tautan berkas.
- **AC27** Given saya baru saja masuk dari ajakan itu, When saya kembali ke halaman
  panduan, Then lampiran tampil lengkap tanpa saya perlu mencari ulang modulnya.

### F. Dua bahasa

- **AC28** Given modul yang kolom Inggrisnya kosong, When pembaca dengan locale `en`
  membuka halaman, Then konten Indonesia yang ditampilkan, dan halaman tidak menampilkan
  string kosong atau kunci mentah.
- **AC29** Given modul yang kolom Inggrisnya terisi, When pembaca berganti locale dari
  `id` ke `en` tanpa memuat ulang halaman, Then judul, deskripsi, dan langkah berganti ke
  versi Inggris.
- **AC30** Given daftar modul di halaman pengelolaan, When ada modul yang belum punya
  versi Inggris, Then modul itu diberi penanda "belum diterjemahkan" yang bisa dilihat
  tanpa membuka modulnya.
- **AC31** Given chrome halaman (judul menu, tombol, label form, pesan kesalahan), When
  locale diganti, Then seluruhnya tetap diterjemahkan lewat berkas i18n seperti sekarang.
  Yang pindah ke DB hanya isi panduan.

### G. Migrasi konten yang sudah ada

- **AC32** Given basis data yang sudah berjalan, When migration baru dijalankan, Then
  sembilan modul yang ada hari ini tersimpan sebagai baris berstatus terbit, lengkap
  dengan judul, ikon, deskripsi, dan seluruh langkahnya, dalam versi Indonesia dan Inggris
  yang diambil dari berkas i18n yang ada.
- **AC33** Given migration sudah dijalankan dan kunci `guidePage.sections` dihapus dari
  kode, When halaman panduan dibuka, Then tampilannya sama dengan sebelum rilis: sembilan
  kartu, urutan sama, ikon sama, jumlah langkah sama.
- **AC34** Given migration dijalankan dua kali (atau ulang setelah rollback), When selesai,
  Then tidak ada modul bawaan yang terduplikasi.
- **AC35** Given `migrate down 1`, When dijalankan, Then tabel-tabel baru terhapus bersih
  tanpa menyisakan tipe, indeks, atau trigger. Catatan risiko: karena kunci i18n dihapus
  di sisi kode, rollback database saja akan membuat halaman kosong; rollback harus
  mencakup kode. Ini wajib ditulis di catatan rilis.

### H. Draf, terbit, dan urutan

- **AC36** Given modul berstatus draf, When pengunjung atau user tanpa `guide.manage`
  membuka halaman panduan, Then modul itu tidak muncul dan tidak ada jejaknya di respons
  API.
- **AC37** Given saya pemegang `guide.manage`, When saya membuka pratinjau, Then saya
  melihat modul draf ditandai jelas sebagai draf, dalam susunan yang sama seperti yang
  akan dilihat pembaca.
- **AC38** Given modul terbit, When saya menariknya kembali ke draf, Then modul langsung
  hilang dari halaman pembaca.
- **AC39** Given saya mengubah urutan modul, When halaman pembaca dimuat ulang, Then
  urutan tampil mengikuti perubahan itu.

### I. Kondisi halaman

- **AC40** Given belum ada satu pun modul terbit, When halaman panduan dibuka, Then
  halaman menampilkan keadaan kosong yang menjelaskan panduan belum tersedia, bukan
  halaman kosong tanpa keterangan atau kerangka yang berputar selamanya.
- **AC41** Given permintaan ke API gagal, When halaman panduan dibuka, Then halaman
  menampilkan keadaan error beserta tombol coba lagi, dan mencobanya berhasil memuat
  tanpa memuat ulang halaman penuh.
- **AC42** Given data sedang dimuat, When halaman dibuka, Then ditampilkan kerangka
  (skeleton) yang mengikuti bentuk kartu modul.
- **AC43** Given modul punya lebih dari satu lampiran, When halaman dibuka, Then seluruh
  lampiran tampil dalam urutan yang ditetapkan pengelola.
- **AC44** Given halaman panduan, When dilihat dalam mode terang dan gelap serta pada
  lebar layar ponsel, Then tata letak tetap terbaca, pemutar video menjaga rasio 16:9,
  dan tidak ada isi yang meluber secara horizontal.

### J. Jejak audit dan pembatasan

- **AC45** Given aksi buat, ubah, terbitkan, atau hapus pada modul maupun lampiran, When
  aksi berhasil, Then tercatat di audit trail berisi pelaku, waktu, jenis aksi, dan
  identitas objeknya.
- **AC46** Given seseorang mencoba mengunggah berkas berkali-kali dalam waktu singkat,
  When melewati batas laju yang berlaku untuk endpoint tulis, Then permintaan dijawab 429
  seperti endpoint tulis lain di aplikasi.

## Alur alternatif dan kegagalan

| Kondisi | Perilaku yang diharapkan |
|---|---|
| Belum ada modul terbit | Keadaan kosong yang menjelaskan panduan belum tersedia (AC40) |
| Modul terbit tanpa lampiran | Kartu modul tampil seperti sekarang, tanpa area media |
| API panduan gagal dimuat | Keadaan error dengan tombol coba lagi (AC41) |
| Video YouTube dihapus atau dijadikan privat oleh pemiliknya | Halaman tetap utuh, kegagalan terbatas pada kartu video (AC17) |
| Berkas PDF hilang dari MinIO meski barisnya ada | Endpoint berkas menjawab 404 dan mencatat log error; halaman tetap dirender |
| Dua pengelola mengubah modul yang sama bersamaan | Penyimpanan terakhir menang; tidak boleh ada baris rusak atau langkah yang tergabung sebagian |
| Unggahan diulang karena user menekan simpan dua kali | Tidak menghasilkan dua lampiran identik yang terlihat sebagai duplikat di halaman |
| Sesi kedaluwarsa saat unggahan berlangsung | Dijawab 401, form tidak kehilangan isian teks yang sudah diketik |
| Pengunjung anonim menebak identitas lampiran | 401, tanpa membocorkan apakah lampiran itu ada |
| User memakai locale `en` sedangkan terjemahan belum ada | Fallback ke konten Indonesia (AC28) |
| Pengelola menghapus modul yang sedang dibaca user lain | Pembaca yang memuat ulang tidak lagi melihatnya; tidak ada error keras di layar |

## Aturan bisnis

1. Lampiran hanya dua jenis: `video` (identitas video YouTube) dan `document` (objek PDF
   di MinIO). Tidak ada jenis lain di rilis ini.
2. Satu modul boleh punya banyak lampiran, dengan urutan yang ditetapkan pengelola.
   Batas wajar: sepuluh lampiran per modul.
3. Konten Indonesia wajib; konten Inggris opsional dan jatuh kembali ke Indonesia.
4. Panduan bersifat global. Tidak ada kolom kantor dan tidak ada penyaringan data scope.
5. Modul baru selalu lahir sebagai draf.
6. Teks modul terbit dapat dibaca tanpa sesi; lampiran hanya untuk user terautentikasi
   mana pun (tidak perlu permission khusus untuk membaca).
7. Hanya pemegang `guide.manage` yang boleh menulis. Permission ini di-seed ke superadmin
   dan dapat diberikan ke peran lain lewat menu RBAC.
8. Berkas yang diterima hanya PDF, ditentukan dari isi berkas. Batasnya dikonfigurasi
   lewat variabel lingkungan tersendiri (usulan `GUIDE_PDF_MAX_BYTES`, default 20 MB) —
   bukan menumpang `ATTACHMENT_MAX_BYTES` yang 5 MB karena panduan ber-tangkapan-layar
   umumnya lebih besar. Variabel baru wajib didaftarkan juga di compose produksi.
9. Tautan video hanya dari host YouTube resmi, disimpan sebagai identitas video, dan
   disematkan lewat `youtube-nocookie.com`.
10. Penghapusan modul dan lampiran bersifat soft delete pada baris; objek MinIO milik
    lampiran yang dihapus eksplisit ikut dihapus.
11. Setiap aksi tulis tercatat di audit trail.

## Dampak ke data dan perilaku yang sudah ada

- **Konten i18n yang ada** dipindahkan ke DB lewat seed migration lalu kunci
  `guidePage.sections` dihapus dari `id.json` dan `en.json`. Kunci lain di `guidePage`
  (`title`, `subtitle`, `intro`) tetap sebagai chrome halaman.
- **Rute publik tidak berubah.** `/guide` tetap ada di `publicPaths`; yang bertambah
  adalah pemisahan antara konten teks (publik) dan lampiran (perlu sesi).
- **Rollback tidak simetris.** Menurunkan migration saja akan mengosongkan halaman karena
  kunci i18n sudah tidak ada di kode. Rollback harus menurunkan kode dan database
  bersamaan. Ini risiko rilis yang harus masuk catatan rilis dan rencana `/ship`.
- **Bucket MinIO yang sama** dipakai, dengan prefix kunci objek tersendiri agar objek
  panduan bisa dibedakan dari lampiran aset saat audit storage.
- **Tidak ada baris lama** di tabel baru, sehingga tidak ada data historis yang melanggar
  asumsi fitur ini.

## Asumsi (bisa dikoreksi sebelum `/plan`)

1. Struktur tampilan modul tetap seperti sekarang: ikon, judul, deskripsi opsional, dan
   daftar langkah bernomor. Tidak ada editor rich text, tidak ada gambar sisipan di badan
   teks.
2. Ikon dipilih dari daftar ikon yang sudah tersedia di aplikasi, bukan diketik bebas,
   supaya tidak ada ikon yang gagal render.
3. Halaman pengelolaan berdiri sebagai menu tersendiri di area terproteksi (layout
   `default`), bukan penyuntingan langsung di halaman publik.
4. Halaman FAQ dan Kebijakan Privasi tetap statis di i18n. Fitur ini tidak menyentuhnya.
5. Tidak ada riwayat versi konten. Yang ada hanya jejak audit siapa mengubah kapan.
6. Pembaca tidak perlu permission apa pun untuk melihat lampiran, cukup punya sesi.

## Di luar cakupan

- Aplikasi mobile. Belum ada layar panduan di `mobile/lib`, dan membuatnya adalah
  pekerjaan tersendiri (layar baru, navigasi, pemutar video, penampil PDF di perangkat).
- Mengunggah berkas video langsung ke MinIO, transcoding, atau subtitle.
- Menjadikan halaman FAQ dan Kebijakan Privasi berbasis data.
- Riwayat versi dan pembatalan perubahan konten.
- Pencarian teks di dalam panduan, dan penautan otomatis dari layar fitur ke modul
  panduannya.
- Analitik per modul (berapa kali video ditonton, PDF diunduh).
- Panduan per kantor atau per peran.
- Alur persetujuan berjenjang untuk penerbitan konten panduan.

## Ukuran keberhasilan

1. Perubahan panduan bisa tayang tanpa rilis kode: waktu dari permintaan perubahan sampai
   tayang turun dari satu siklus rilis menjadi di bawah satu hari kerja.
2. Setelah dua bulan, minimal lima dari sembilan modul memiliki setidaknya satu lampiran
   video atau PDF.
3. Tidak ada satu pun berkas panduan yang bisa diambil tanpa sesi. Dibuktikan lewat test
   otorisasi yang gagal jika endpoint berkas dibuat publik.
4. Tampilan halaman setelah migrasi identik dengan sebelumnya, dibuktikan lewat
   perbandingan berdampingan pada rilis.

## Pertanyaan terbuka

Tidak ada yang menghalangi perencanaan. Enam asumsi di atas terbuka untuk dikoreksi, dan
tidak satu pun mengubah bentuk skema secara mendasar.

## Handoff

Lanjut ke `/plan` dengan agent `planner`. Skill yang relevan: `vertical-slice` (fitur ini
menembus migration, sqlc, handler, otorisasi, sampai UI) dan `file-upload` (unggah dan
penyajian ulang PDF). Rules yang berlaku: `00-core`, `40-security`, `30-api-contract`,
`25-postgresql`, `20-go`, `14-nuxt`, `15-ui-design`. Referensi: `definition-of-done`,
`security-checklist`, `accessibility-checklist`, `testing-patterns`.

Catatan untuk `/plan`: halaman ini belum punya mockup di `docs/design/`. Perlu diputuskan
di fase rencana apakah tampilan pembaca cukup mengikuti bentuk kartu yang sudah ada
(ditambah area media) atau perlu melalui agent `designer` lebih dulu untuk halaman
pengelolaannya.
