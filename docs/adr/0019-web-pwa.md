# ADR-0019 — Aplikasi web sebagai PWA: installable dan tahan jaringan putus (shell saja)

- Status: Accepted
- Date: 2026-08-27
- Deciders: pemilik produk + sesi spec/plan PWA (spec
  `docs/superpowers/specs/2026-08-27-web-pwa-design.md`, rencana
  `docs/superpowers/plans/2026-08-27-web-pwa.md`)
- Terkait: [ADR-0015](../mobile/adr/0015-mobile-companion-flutter.md) (aplikasi companion Flutter)
  dan [ADR-0016](../mobile/adr/0016-stock-opname-offline-sync.md) (offline sync opname).
  ADR ini **tidak** men-supersede keduanya; lihat "Hubungan dengan ADR-0015".

## Konteks

Aplikasi web Inventra hanya bisa dibuka lewat tab browser. Tiga akibatnya nyata di lapangan:
tidak ada ikon di layar utama sehingga petugas harus mengingat URL, chrome browser memakan
ruang vertikal yang sudah sempit untuk tabel aset dan formulir panjang, dan saat sinyal hilang
pengguna melihat halaman error bawaan browser alih-alih pesan aplikasi.

Aplikasi Flutter menutup sebagian kebutuhan itu, tetapi **fokus rilisnya Android**. Pengguna
iPhone, dan pengguna Android yang kebijakan perangkatnya melarang pemasangan APK di luar toko
aplikasi, tidak punya kanal ber-ikon sama sekali.

Empat temuan pemeriksaan kode membentuk keputusan ini:

1. Aplikasi adalah SPA murni (`ssr: false`), dan `nuxt build` menghasilkan `.output/public/`
   **tanpa satu pun berkas HTML** — shell dirender Nitro saat runtime. Shell karenanya tidak
   otomatis bisa di-precache.
2. Di **produksi** API se-origin dengan frontend (`https://DOMAIN/api/v1`), sementara di **dev**
   API beda origin (`localhost:8080`). Kesalahan caching respons API karenanya tidak akan
   terlihat saat dev.
3. Access token disimpan di memori Pinia, dan `localStorage` hanya menyimpan preferensi
   notifikasi serta riwayat command palette. Hari ini tidak ada data aset di penyimpanan klien.
4. `routeRules` sudah sengaja menetapkan `/_nuxt/**` immutable dan `/**` `no-cache`, lengkap
   dengan komentar tentang jebakan "shell basi menunjuk chunk 404".

## Keputusan

**Jadikan aplikasi web yang sama sebagai PWA yang installable dan andal, dengan offline
sebatas shell — tanpa satu pun respons API disimpan.** Implementasinya memakai
`@vite-pwa/nuxt` (pembungkus Workbox), dan seluruh konfigurasinya terpusat di `frontend/pwa/`
(`manifest.ts`, `workbox.ts`, `client.ts`, `head.ts`) supaya bisa dikunci tes unit.

1. **Kedalaman: installable dan andal, bukan offline-first.** Aplikasi dapat dipasang, aset
   build ter-precache, ada keadaan offline berbahasa Indonesia dan Inggris, dan ada ajakan
   perbarui saat versi baru menunggu.
2. **Nol runtime caching.** `pwa/workbox.ts` sama sekali tidak memuat `runtimeCaching`. Satu
   aturan runtime saja sudah cukup untuk mengendapkan respons API ke Cache Storage tiap
   perangkat, dan karena di produksi API se-origin, kesalahan itu tidak akan pernah terlihat
   saat dev. `/api/` dan `/health` juga masuk `navigateFallbackDenylist` supaya navigasi ke
   sana tidak dijawab shell.
3. **Shell dipaksa jadi berkas statis.** Rute `/` di-prerender lewat `nitro.prerender.routes`,
   berkas itu masuk precache, dan `navigateFallback` menunjuk ke sana. Karena `ssr: false`,
   HTML hasil prerender hanyalah kerangka kosong tanpa data pengguna, dan locale `/en/`
   diselesaikan di klien dari URL — satu shell melayani kedua locale.
4. **`registerType: 'prompt'`, bukan `autoUpdate`.** Pengaktifan versi baru diserahkan ke
   pengguna lewat ajakan di UI. `skipWaiting` dan `clientsClaim` sengaja tidak dipakai.
5. **`routeRules` dibiarkan apa adanya.** Service worker mengambil alih pencegahan shell basi
   lewat precache berversi + `cleanupOutdatedCaches`; dua lapis itu tidak diubah agar tidak
   bertabrakan, dan hasilnya dibuktikan lewat tes.
6. **`display: standalone`**, `scope` dan `start_url` di akar, `theme_color` `#005bfd` diambil
   dari step 500 ramp brand di `app/assets/css/main.css`.
7. **Ikon diturunkan dari mark aplikasi yang sudah ada** — kotak persegi sudut-membulat
   `#005bfd` berisi glyph Lucide `package` putih (`public/logo-source.svg`), digenerate
   `@vite-pwa/assets-generator` dan hasilnya **di-commit** ke `public/` supaya build CI tidak
   bergantung pada langkah generate. Logo korporat BTN sengaja **tidak** dipakai sebagai ikon
   aplikasi karena itu merek pihak lain dan menuntut persetujuan klien.
8. **Ajakan pasang: Android otomatis, iOS manual.** Android memakai `beforeinstallprompt` yang
   ditahan dan ditampilkan di dalam UI; Safari iOS tidak menyediakan API itu sehingga
   ditampilkan petunjuk "Bagikan, lalu Tambahkan ke Layar Utama" yang penutupannya diingat.
   Keduanya disembunyikan di seluruh layar berlayout `auth`.
9. **PWA tidak mendesain ulang layar mana pun.** Yang disentuh hanya dua komponen ajakan baru
   dan penanganan `env(safe-area-inset-*)` saat berjalan standalone.

## Hubungan dengan ADR-0015

ADR-0015 menolak **PWA murni sebagai bentuk aplikasi pendamping lapangan** dengan tiga alasan:
push di iOS terbatas, storage lokal bisa di-evict OS, dan scan kamera tetap jalur browser.
**Ketiga alasan itu tetap berlaku dan ADR ini tidak menyentuh satu pun dari ketiganya** —
justru sebaliknya, ketiganya secara eksplisit berada di luar cakupan keputusan ini:

| Kemampuan | Pemilik | Status |
|---|---|---|
| Scan aset via kamera | Flutter (ADR-0015) | Tetap. Tidak diduplikasi PWA |
| Stock opname offline-first | Flutter (ADR-0016) | Tetap. PWA tidak menyimpan data apa pun |
| Approval di jalan + push notification | Flutter (ADR-0015) | Tetap. Web Push di luar cakupan |
| Aplikasi web yang sama, dapat dipasang, tahan jaringan putus, punya alur pembaruan | PWA (ADR ini) | Baru |

Kesimpulan yang harus terbaca: **aplikasi Flutter tidak dibatalkan, tidak dikurangi, dan tidak
digantikan.** PWA hanya membuat aplikasi web yang sudah ada bisa dipasang dan tidak runtuh saat
jaringan putus, terutama untuk pengguna iOS dan pengguna Android tanpa APK. Nol kemampuan
lapangan berpindah dari Flutter ke web.

## Alternatif yang ditolak

- **Offline penuh (cache respons API atau replika data di IndexedDB).** Ini yang paling
  menggoda dan paling berbahaya. Data aset bank akan mendarat di disk perangkat yang tidak
  dikelola bank, di browser yang bisa dipakai bersama, tanpa satu pun kontrol retensi. Ia juga
  menduplikasi ADR-0016 yang sudah memiliki jalur offline yang benar (snapshot lokal + sinkron
  batch idempoten di Flutter). Ditolak.
- **Web Push sekarang.** Backend sama sekali belum punya VAPID maupun FCM, jadi ini pekerjaan
  backend baru, bukan pekerjaan frontend. Push di iOS pun baru bekerja setelah aplikasi
  dipasang ke layar utama. Ditunda sebagai spec tersendiri, bukan diselundupkan ke sini.
- **`display: minimal-ui` alih-alih `standalone`.** `minimal-ui` menahan sebagian kontrol
  browser sehingga domain tetap terlihat — argumen yang masuk akal untuk aplikasi perbankan.
  Ditolak karena pemilik produk menyatakan tidak ada aturan internal yang mewajibkan domain
  terlihat, dan `standalone` adalah default industri untuk aplikasi terpasang. Keputusan ini
  satu bidang manifest dan bisa dibalik kapan saja tanpa menyentuh kode lain.
- **`registerType: 'autoUpdate'`.** Lebih sedikit kode dan pengguna selalu di versi terbaru,
  tetapi ia bisa memuat ulang halaman saat pengguna sedang mengisi formulir aset yang panjang
  dan membuang isiannya. Ditolak.
- **Menulis service worker tangan.** Kontrol penuh, tetapi service worker adalah kode yang
  salahnya mahal dan sulit terlihat. Prinsip ADR proyek ini adalah memilih yang matang;
  Workbox lewat `@vite-pwa/nuxt` adalah standar de-facto untuk Vite dan Nuxt.
- **Mengubah `routeRules` supaya shell bisa di-cache HTTP.** Menyentuh jebakan "shell basi
  menunjuk chunk 404" yang sudah didokumentasikan di kode. Ditolak; precache berversi milik
  service worker menyelesaikan masalah yang sama tanpa menyentuh header.

## Konsekuensi

**Positif**

- Pengguna iOS dan pengguna Android tanpa APK akhirnya punya kanal ber-ikon.
- Jaringan putus memberi shell aplikasi beserta keadaan error aplikasi, bukan halaman error
  browser.
- Aset build ter-precache, sehingga buka berikutnya lebih cepat.
- Rilis baru mengumumkan dirinya lewat ajakan perbarui, bukan lewat pengguna yang kebetulan
  menekan muat ulang.

**Negatif dan yang harus dijaga**

- **Nol runtime caching adalah invarian, bukan preferensi.** Menambahkan satu aturan
  `runtimeCaching` untuk `/api/` akan mengendapkan respons API ke Cache Storage di produksi dan
  itu tidak terlihat saat dev. Invarian ini dikunci `test/unit/pwa-workbox.spec.ts` dan
  dibuktikan e2e `frontend/e2e/pwa.spec.ts` yang membaca `caches` lewat `page.evaluate` dan
  menuntut nol entri ber-URL `/api/`, sebelum dan sesudah logout.
- **Service worker kini aktif di seluruh e2e** karena Playwright berjalan terhadap
  `pnpm preview` (build nyata). Regresi suite penuh menjadi bagian tetap dari pekerjaan yang
  menyentuh konfigurasi PWA.
- **Rute `/` sekarang di-prerender.** Apa pun yang kelak membuat shell memuat data pengguna
  akan membocorkannya ke HTML statis. `ssr: false` menjaga ini hari ini, tetapi ia asumsi yang
  harus diperiksa ulang bila mode render berubah.
- **`manifest.webmanifest` dan `sw.js` melewati Caddy + WAF Coraza** di produksi. False
  positive OWASP CRS hanya ketahuan setelah deploy, sehingga verifikasi produksi wajib.
- **Dua kanal untuk dipelihara.** Pengguna kini bisa memakai Inventra lewat aplikasi Flutter
  atau PWA. Pembagian tanggung jawab di tabel "Hubungan dengan ADR-0015" adalah yang mencegah
  keduanya saling menduplikasi.
