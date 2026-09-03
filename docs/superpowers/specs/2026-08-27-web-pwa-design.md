# Spesifikasi: PWA untuk aplikasi web Inventra (installable + andal)

- Tanggal: 2026-08-27
- Fase: Define (`/spec`)
- Handoff berikutnya: `/plan`
- Menutup gantungan: `docs/PROGRESS.md` baris 306 ("PWA sengaja dikecualikan atas permintaan user; belum ada infrastrukturnya ... cakupannya masih perlu `/spec`")

## Masalah

Aplikasi web Inventra hanya bisa dibuka lewat tab browser. Tiga akibatnya nyata bagi
pengguna di lapangan:

1. **Tidak ada pintu masuk sepersatu-ketuk.** Petugas harus mengingat URL, membuka
   browser, lalu mengetik alamat. Tidak ada ikon aplikasi di layar utama ponsel.
2. **Chrome browser memakan layar ponsel.** Address bar dan tab bar memakan ruang
   vertikal yang sudah sempit untuk tabel aset dan formulir panjang.
3. **Jaringan putus berarti layar error mentah browser.** Kantor cabang dan gudang aset
   sering ber-sinyal buruk. Saat koneksi hilang, pengguna melihat halaman error bawaan
   browser, bukan pesan aplikasi yang menjelaskan keadaan.

Aplikasi Flutter (`mobile/`) menutup sebagian kebutuhan ini, tetapi **hanya Android** —
rilis iOS tidak ada dalam fokus (PROGRESS baris 1893: "fokus rilis tetap Android").
Pengguna iPhone, dan pengguna Android yang tidak diizinkan memasang APK di luar toko
aplikasi, tidak punya kanal ber-ikon sama sekali.

## Pengguna dan pemicu

| Peran | Yang dilakukan | Frekuensi | Pemicu |
|---|---|---|---|
| Petugas aset cabang (pengguna iPhone) | Memasang Inventra ke layar utama lewat Safari, memakainya seperti aplikasi | Sekali pasang, dipakai harian | Tidak ada aplikasi iOS; hanya punya web |
| Petugas aset cabang (Android tanpa APK) | Memasang lewat Chrome saat aplikasi Flutter tidak tersedia atau tidak diizinkan dipasang | Sekali pasang, dipakai harian | Kebijakan perangkat, atau enggan memasang APK di luar toko |
| Pengguna mana pun di sinyal buruk | Membuka aplikasi yang sudah terpasang, melihat pesan offline yang jelas, bukan error browser | Insidental | Sinyal hilang di gudang atau saat berpindah kantor |
| Pengguna setelah deploy baru | Menerima ajakan "versi baru tersedia" lalu memuat ulang | Tiap rilis | Deploy produksi |

## Keadaan yang ada (hasil pemeriksaan kode)

| Temuan | Berkas | Konsekuensi untuk fitur ini |
|---|---|---|
| Tidak ada infrastruktur PWA sama sekali: tidak ada `@vite-pwa/nuxt`, manifest, service worker | `frontend/package.json`, `frontend/nuxt.config.ts` | Semua dibangun dari nol; tidak ada yang perlu dimigrasikan |
| `frontend/public/` hanya berisi `favicon.ico` dan `logo-btn.png` | `frontend/public/` | Seluruh ikon PWA harus dibuat |
| `logo-btn.png` adalah logo korporat Bank BTN (340x204, bukan persegi) dan **hanya** dipakai di label aset yang dicetak | `frontend/public/logo-btn.png`, `app/components/asset/AssetLabel.vue` baris 41 dan 53 | Bukan identitas aplikasi. Tidak dipakai sebagai ikon PWA, lihat Keputusan nomor 12 |
| Identitas aplikasi sudah ada dan konsisten: kotak sudut-membulat `bg-primary` berisi ikon Lucide `package` putih, di samping wordmark "Inventra" | `app/components/AppSidebar.vue` baris 140, `app/layouts/auth.vue` baris 18, `app/layouts/info.vue` baris 50 | Sumber ikon PWA sudah tersedia; tidak perlu karya visual baru |
| Paket ikon `@iconify-json/lucide` sudah menjadi dependency | `frontend/package.json` | Path glyph `package` bisa diambil dari paket itu, bukan disalin tangan |
| SPA murni, `ssr: false` | `frontend/nuxt.config.ts` baris 20 | Cocok untuk service worker; tidak ada HTML per-rute yang perlu dipikirkan |
| `nuxt build` menghasilkan `.output/public/` **tanpa satu pun berkas HTML** — shell dirender Nitro saat runtime | diperiksa langsung pada `.output/` hasil build terakhir | Shell tidak otomatis ter-precache; butuh keputusan eksplisit (lihat Keputusan nomor 5) |
| `routeRules` sengaja menetapkan `/_nuxt/**` immutable dan `/**` `no-cache`, dengan komentar tentang jebakan "shell basi menunjuk chunk 404" | `frontend/nuxt.config.ts` baris 40-43 | Service worker mengambil alih tanggung jawab itu; keputusan harus sadar dan tercatat, bukan tabrakan diam-diam |
| i18n `prefix_except_default`, rute Inggris berawalan `/en/` | `frontend/nuxt.config.ts` baris 61 | `scope` dan `navigateFallback` harus melayani `/` dan `/en/*` dengan satu shell yang sama |
| Access token disimpan **di memori Pinia**, tidak persisten | `frontend/app/stores/auth.ts` baris 6 | Tidak ada token di penyimpanan yang bisa ikut ter-cache; refresh tetap lewat cookie httpOnly |
| `localStorage` hanya dipakai untuk preferensi notifikasi dan riwayat command palette | `useAccount.ts` baris 213, `useCommandPalette.ts` baris 8 | Tidak ada data aset di penyimpanan klien hari ini; aturan itu harus dipertahankan |
| Produksi: API satu origin dengan frontend (`https://DOMAIN/api/v1`) | `docker-compose.prod.yml` baris 151 | `/api/*` wajib masuk denylist service worker; di produksi ia se-origin sehingga bisa tak sengaja ter-cache |
| Dev: API beda origin (`http://localhost:8080`) | `docker-compose.dev.yml` baris 159 | Perilaku dev dan produksi berbeda; pembuktian keamanan cache harus dilakukan pada mode se-origin |
| Caddy + WAF Coraza berada di depan seluruh rute | `ops/caddy/Caddyfile` | `manifest.webmanifest` dan `sw.js` melewati WAF; perlu pembuktian tidak kena false positive |
| Shell aplikasi sudah punya drawer mobile (off-canvas di bawah `lg`) | `app/components/AppSidebar.vue` baris 107-124 | Navigasi ponsel sudah ada; PWA tidak mendesain ulang layar |
| `app.vue` sudah memakai `useHead` untuk font dan favicon | `frontend/app/app.vue` baris 2-10 | Titik pasang yang jelas untuk tag manifest, theme-color, dan ikon Apple |
| Warna merek: `--color-brand-500: #005bfd` | `frontend/app/assets/css/main.css` baris 33 | Sumber kebenaran `theme_color`; jangan mengarang ulang dari mockup |
| Aplikasi Flutter sudah punya `scan/`, `stock_opname/`, `approval/`, `notifications/` | `mobile/lib/features/` | Batas cakupan: PWA tidak menduplikasi kemampuan lapangan itu |
| Backend tidak punya push apa pun (tidak ada FCM, VAPID, Web Push) | pencarian di `backend/internal/` nihil | Web Push memang butuh pekerjaan backend baru; di luar cakupan |
| e2e Playwright berjalan terhadap `pnpm preview` (build nyata), `workers: 1` di CI | `frontend/playwright.config.ts` baris 16, 45 | Service worker aktif saat e2e; risiko mengganggu 40 spec yang sudah ada harus dibuktikan tidak terjadi |

## Batas terhadap ADR-0015 (aplikasi Flutter)

ADR-0015 menolak PWA **sebagai bentuk aplikasi pendamping lapangan** dan memilih Flutter.
Spec ini **tidak membatalkan** keputusan itu, dan tidak boleh ditafsirkan begitu:

| Kemampuan | Pemilik | Alasan |
|---|---|---|
| Scan aset via kamera, stock opname offline-first, approval di jalan, push notification | Aplikasi Flutter (ADR-0015, ADR-0016) | Tetap. Tidak dipindahkan, tidak diduplikasi |
| Aplikasi web yang sama persis, dapat dipasang, tahan jaringan putus, punya alur pembaruan | PWA (spec ini) | Menutup pengguna iOS dan pengguna tanpa APK; tidak menambah satu pun kemampuan lapangan baru |

Konsekuensinya harus ditulis sebagai ADR baru yang **menunjuk balik** ke ADR-0015 dan
menegaskan keduanya hidup berdampingan dengan pembagian di atas. Nomornya **ADR-0019**,
ditetapkan pemilik produk; 0018 tetap dicadangkan untuk skema `guide` (PROGRESS baris 28)
yang belum ditulis.

## Keputusan yang sudah diambil

| Nomor | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| 1 | Seberapa dalam PWA-nya | **Installable dan andal**: dapat dipasang, aset build ter-precache, ada layar offline, ada ajakan perbarui | Ditetapkan pemilik produk |
| 2 | Kedalaman offline | **Shell saja.** Tidak ada satu pun respons API yang disimpan | Ditetapkan pemilik produk. Data aset bank tidak boleh mendarat di disk perangkat, dan offline data sudah jadi milik Flutter lewat ADR-0016 |
| 3 | Web Push | **Di luar cakupan** | Ditetapkan pemilik produk. Backend belum punya VAPID maupun FCM; itu spec dan fase tersendiri |
| 4 | Target pembuktian | **Android Chrome dan iOS Safari** | Ditetapkan pemilik produk. Desktop tetap bekerja tetapi bukan yang dibuktikan |
| 5 | Shell HTML tidak ada di keluaran build | **Paksa shell menjadi berkas statis** lewat prerender rute `/`, lalu precache berkas itu, dengan `navigateFallback` ke shell tersebut | Satu-satunya cara memenuhi keputusan 1 tanpa menyentuh `/api`. Karena `ssr: false`, HTML hasil prerender hanyalah kerangka kosong: tidak ada data pengguna di dalamnya |
| 6 | Strategi pembaruan service worker | **`registerType: 'prompt'`** (pengguna menekan "muat ulang"), bukan `autoUpdate` | `autoUpdate` bisa memuat ulang halaman saat pengguna sedang mengisi formulir aset yang panjang dan membuang isiannya |
| 7 | Hubungan dengan `routeRules` `no-cache` | Aturan header **tetap apa adanya**; service worker mengambil alih pencegahan shell basi lewat precache berversi + `cleanupOutdatedCaches` | Mengubah header berarti menyentuh jebakan yang sudah didokumentasikan di kode; lebih aman membiarkan dua lapis itu tidak bertabrakan dan membuktikannya lewat tes |
| 8 | Pustaka | `@vite-pwa/nuxt` (versi 1.1.1) dan `@vite-pwa/assets-generator` (1.0.2) | Standar de-facto untuk Vite dan Nuxt, membungkus Workbox. Sejalan dengan prinsip ADR: pakai yang matang, jangan menulis service worker tangan |
| 9 | Ikon dan splash | Dihasilkan dari satu berkas sumber persegi lewat `@vite-pwa/assets-generator`, hasilnya **di-commit** ke `public/` | Menghindari ketergantungan langkah generate saat build CI |
| 10 | Ajakan pasang | Tombol pasang muncul lewat `beforeinstallprompt` di Android; di iOS ditampilkan **petunjuk manual** ("Bagikan, lalu Tambahkan ke Layar Utama") karena Safari tidak menyediakan API itu | Tanpa ini, pengguna iOS tidak akan pernah tahu aplikasinya bisa dipasang |
| 11 | Cakupan perubahan visual | PWA **tidak mendesain ulang layar mana pun**. Yang boleh disentuh hanya: penambahan dua komponen ajakan, dan penanganan safe-area saat mode standalone | Mockup di `docs/design/` tetap mengikat; PWA bukan alasan mengubah tata letak |
| 12 | Sumber ikon | **Bukan berkas baru.** Ikon dirender dari mark aplikasi yang sudah dipakai di shell: kotak persegi sudut-membulat `#005bfd` berisi glyph Lucide `package` putih, disimpan sebagai `public/logo-source.svg` 512x512. Glyph diambil dari `@iconify-json/lucide` yang sudah jadi dependency, bukan disalin tangan | Pemilik produk belum menyiapkan logo, dan memang tidak perlu: identitas ini sudah dipakai di sidebar, halaman masuk, dan halaman info. Logo korporat BTN sengaja **tidak** dipakai sebagai ikon aplikasi karena itu merek pihak lain dan menuntut persetujuan klien |
| 13 | `display` | **`standalone`** (tanpa address bar) | Default industri untuk aplikasi terpasang. Pemilik produk tidak memiliki aturan internal yang mewajibkan domain tetap terlihat. Keputusan ini satu bidang manifest dan bisa dibalik ke `minimal-ui` kapan saja tanpa menyentuh kode lain |
| 14 | `short_name` | **"Inventra"** | Ditetapkan pemilik produk. Cukup pendek untuk label di layar utama |

## Cakupan

**Di dalam cakupan**

- Modul `@vite-pwa/nuxt` beserta konfigurasinya di `nuxt.config.ts`
- Web app manifest (nama, ikon, warna, `display: standalone`, `scope`, `start_url`)
- Berkas ikon: 64, 192, 512, maskable 512, apple-touch-icon 180
- Service worker hasil generate Workbox: precache aset build dan shell; tanpa runtime caching apa pun
- Halaman atau keadaan offline berbahasa Indonesia dan Inggris
- Komponen ajakan perbarui (`needRefresh`) dan ajakan pasang (Android otomatis, iOS manual)
- Meta khusus iOS dan penanganan `env(safe-area-inset-*)` saat berjalan standalone
- Kunci i18n `pwa.*` di `id.json` dan `en.json`
- Tes: unit manifest, runtime dua komponen, e2e pemasangan-siap dan perilaku offline
- ADR baru yang menyandingkan keputusan ini dengan ADR-0015
- Pembaruan `docs/PROGRESS.md` dan vault Obsidian

**Di luar cakupan (sengaja)**

- Web Push dan segala perubahan backend (keputusan 3)
- Cache respons API, apa pun bentuknya (keputusan 2)
- Antrean tulis offline dan Background Sync (milik ADR-0016 di Flutter)
- Badging, Share Target, File Handler, Shortcuts di manifest
- Perubahan pada aplikasi Flutter
- Perbaikan responsivitas layar yang sudah ada. Pemilik produk sudah memeriksa dan menyatakan seluruh layar web responsif di berbagai perangkat, jadi pekerjaan ini tidak menyentuhnya
- Karya visual baru. Ikon diturunkan dari mark aplikasi yang sudah ada (Keputusan nomor 12)
- Pemasangan di desktop sebagai target pembuktian (tetap bekerja, tidak diuji khusus)

## Alur utama

### 1. Pemasangan di Android Chrome

1. Pengguna membuka Inventra di Chrome Android melalui HTTPS.
2. Service worker terpasang dan aktif; manifest terbaca sah.
3. Browser memicu `beforeinstallprompt`; aplikasi menahannya dan menampilkan ajakan
   sendiri di dalam UI (bukan banner bawaan yang lewat begitu saja).
4. Pengguna menekan "Pasang", dialog sistem muncul, pengguna menyetujui.
5. Ikon Inventra muncul di layar utama. Dibuka dari ikon, aplikasi tampil standalone
   tanpa address bar, dengan warna bilah status `#005bfd`.

### 2. Pemasangan di iOS Safari

1. Pengguna membuka Inventra di Safari iOS.
2. Aplikasi mendeteksi Safari iOS yang belum standalone dan menampilkan petunjuk:
   tekan tombol Bagikan, lalu "Tambahkan ke Layar Utama".
3. Petunjuk bisa ditutup, dan penutupannya diingat sehingga tidak muncul lagi.
4. Setelah dipasang dan dibuka dari ikon, aplikasi tampil standalone, ikon apple-touch
   terpakai, dan konten tidak tertutup notch maupun home indicator.

### 3. Jaringan putus

1. Pengguna sudah pernah membuka aplikasi (service worker aktif, shell ter-precache).
2. Koneksi hilang.
3. Pengguna membuka aplikasi dari ikon: shell tetap tampil dari precache, bukan halaman
   error browser.
4. Permintaan data ke `/api/**` gagal seperti biasa dan layar menampilkan keadaan error
   aplikasi yang sudah ada. **Tidak ada data lama yang ditampilkan seolah-olah baru.**
5. Saat koneksi kembali, muat ulang biasa mengembalikan keadaan normal.

### 4. Deploy versi baru

1. Deploy produksi menghasilkan service worker baru.
2. Pengguna yang sedang membuka aplikasi menerima ajakan "Versi baru tersedia" beserta
   tombol "Muat ulang".
3. Ajakan **tidak** memuat ulang sendiri; pengguna yang memutuskan.
4. Setelah dimuat ulang, cache lama dibersihkan dan tidak ada permintaan chunk 404.

## Acceptance criteria

Manifest dan ikon

1. `GET /manifest.webmanifest` menjawab 200 dengan `Content-Type` manifest yang sah.
2. Manifest memuat `name` "Inventra - Manajemen Aset", `short_name` "Inventra",
   `start_url` `/`, `scope` `/`, `display` `standalone`, `lang` `id`.
3. `theme_color` bernilai `#005bfd`, terikat pada `--color-brand-500` dan bukan angka
   yang diketik ulang lepas dari sumbernya.
4. `background_color` ditetapkan dan sepadan dengan latar shell mode terang.
5. Manifest memuat ikon 192x192 dan 512x512 `purpose: any`, serta satu ikon 512x512
   `purpose: maskable`.
6. Setiap URL ikon di manifest benar-benar ada di `frontend/public/`, dibuktikan tes dan
   bukan diperiksa mata.
7. `<link rel="manifest">` dan `<meta name="theme-color">` hadir di HTML yang dikirim.
8. `apple-touch-icon` 180x180 terpasang dan berkasnya ada.
9. Ikon menampilkan mark aplikasi (kotak `#005bfd` berisi glyph `package` putih), bukan
   logo korporat BTN.
10. Pada varian maskable, glyph tetap utuh saat dipotong lingkaran oleh Android (area aman
    terjaga, tidak terpotong di tepi).
11. `display` bernilai `standalone`.

Service worker

12. Service worker terdaftar di produksi dan mencapai status `activated` pada pemuatan pertama.
13. Service worker **tidak** aktif saat `pnpm dev` kecuali dinyalakan eksplisit lewat `devOptions`.
14. Precache berisi aset build (JS, CSS, font, ikon) dan shell HTML.
15. Precache **tidak** berisi satu pun entri ber-URL mengandung `/api/`.
16. Tidak ada aturan runtime caching apa pun yang terdaftar untuk `/api/**`.
17. `navigateFallbackDenylist` mengecualikan `/api/` dan `/health`.
18. Navigasi ke rute berawalan `/en/` dilayani shell yang sama dan bekerja offline.
19. `cleanupOutdatedCaches` aktif, sehingga cache build lama terhapus saat service worker baru mengambil alih.

Offline

20. Setelah satu kali kunjungan daring, memuat ulang dalam keadaan offline menampilkan
    shell aplikasi, bukan halaman error bawaan browser.
21. Dalam keadaan offline, layar yang bergantung data menampilkan keadaan error aplikasi
    yang sudah ada, bukan data basi.
22. Setelah logout lalu offline, `Cache Storage` tidak memuat satu pun entri ber-URL
    mengandung `/api/`, dibuktikan langsung lewat pembacaan `caches` di browser.

Pembaruan

23. Saat service worker baru menunggu, muncul ajakan perbarui yang bisa dilihat pengguna.
24. Ajakan itu tidak pernah memuat ulang halaman dengan sendirinya.
25. Menekan tombol muat ulang mengaktifkan service worker baru dan memuat ulang halaman.
26. Ajakan bisa ditutup, dan tidak muncul lagi di sesi yang sama setelah ditutup.

Ajakan pasang

27. Di peramban yang memicu `beforeinstallprompt`, tombol pasang muncul di dalam UI.
28. Menekan tombol pasang memanggil dialog pasang bawaan, dan tombol menghilang setelah aplikasi terpasang.
29. Di Safari iOS yang belum standalone, muncul petunjuk manual Bagikan lalu Tambahkan ke Layar Utama.
30. Petunjuk iOS **tidak** muncul saat aplikasi sudah berjalan standalone.
31. Penutupan ajakan pasang diingat lintas pemuatan halaman.
32. Ajakan pasang tidak muncul di halaman login sehingga tidak mengganggu alur masuk;
    kalau saat implementasi diputuskan sebaliknya, keputusan itu dicatat di PROGRESS.

Mode standalone

33. Berjalan standalone, konten tidak tertutup notch atau home indicator di iOS
    (`viewport-fit=cover` plus `env(safe-area-inset-*)` pada shell).
34. Berjalan standalone di ponsel, navigasi utama tetap terjangkau lewat drawer yang sudah ada.
35. Warna bilah status mengikuti tema merek dan tidak rusak di mode gelap.

i18n, gaya, dan gerbang mutu

36. Setiap string baru ada di `i18n/locales/id.json` dan `en.json` di bawah namespace `pwa`, tanpa teks yang dikeraskan di komponen.
37. Kedua komponen ajakan disusun dari komponen `U*` Nuxt UI, bukan markup tangan.
38. `pnpm lint`, `pnpm typecheck`, `pnpm test`, dan `pnpm build` hijau.
39. Seluruh suite e2e yang ada tetap hijau dengan service worker aktif; tidak ada satu spec pun yang perlu dilonggarkan agar lulus.
40. `docs/PROGRESS.md` diperbarui dan ADR-0019 ditulis serta ditautkan dua arah dengan ADR-0015.

## Tumpukan teknologi

| Bagian | Pilihan | Versi | Catatan |
|---|---|---|---|
| Modul PWA | `@vite-pwa/nuxt` | 1.1.1 | Membawa `vite-plugin-pwa` ^1.2.0 dan Workbox |
| Generator ikon | `@vite-pwa/assets-generator` | 1.0.2 | Dipakai sekali lalu hasilnya di-commit; cukup sebagai devDependency |
| Kerangka | Nuxt | 4.5.1, `ssr: false` | Sudah ada |
| Tes unit dan runtime | Vitest dan `@nuxt/test-utils` | sudah ada | |
| Tes e2e | Playwright | sudah ada | Chromium mendukung service worker dan `context.setOffline` |

**Risiko versi yang harus dibuktikan lebih dulu.** `@vite-pwa/nuxt@1.1.1` menyatakan
ketergantungan pada `@nuxt/kit` ^3.9.0 dan dikembangkan terhadap Nuxt ^3.10; dukungan
Nuxt 4 tidak dinyatakan resmi di metadata paketnya walau dipakai luas di Nuxt 4. Langkah
pertama rencana implementasi wajib berupa pembuktian pendek: pasang modul, jalankan
`pnpm build`, pastikan `sw.js` dan manifest benar-benar terbit. Kalau gagal, jalur
cadangannya memakai `vite-plugin-pwa` langsung lewat `vite.plugins` di `nuxt.config.ts`
dengan pendaftaran service worker lewat plugin Nuxt sendiri.

## Perintah

```bash
pnpm install
```

```bash
pnpm dev
```

```bash
pnpm build
```

```bash
pnpm preview
```

```bash
pnpm lint
```

```bash
pnpm typecheck
```

```bash
pnpm test
```

```bash
pnpm test:e2e
```

```bash
pnpm exec playwright test e2e/pwa.spec.ts --workers=1
```

```bash
pnpm exec pwa-assets-generator --preset minimal-2023 public/logo-source.svg
```

Semua dijalankan dari `frontend/`. Service worker mati saat `pnpm dev` kecuali
`devOptions` dinyalakan, jadi pengujian nyata dilakukan lewat `pnpm preview`. Ikon
digenerate sekali lalu hasilnya di-commit.

Catatan lapangan yang berlaku dan mahal ditemukan ulang: **e2e lokal wajib `--workers=1`**
seperti CI, dan backend wajib `RATELIMIT_ENABLED=false`.

## Struktur proyek

Berkas yang ditambahkan atau disentuh:

```
frontend/
  nuxt.config.ts                        (ubah) modul + blok pwa + prerender shell
  package.json                          (ubah) dua dependency
  pwa/manifest.ts                       (baru)  objek manifest, diekspor agar bisa diuji unit
  public/
    logo-source.svg                     (baru)  512x512, mark aplikasi, sumber generate ikon
    pwa-64x64.png                       (baru)
    pwa-192x192.png                     (baru)
    pwa-512x512.png                     (baru)
    maskable-icon-512x512.png           (baru)
    apple-touch-icon-180x180.png        (baru)
  app/
    app.vue                             (ubah) tag manifest/ikon/theme-color + pasang dua komponen
    components/
      PwaUpdatePrompt.vue               (baru)
      PwaInstallPrompt.vue              (baru)
    assets/css/main.css                 (ubah) variabel safe-area untuk mode standalone
  i18n/locales/id.json                  (ubah) namespace pwa
  i18n/locales/en.json                  (ubah) namespace pwa
  test/
    unit/pwa-manifest.spec.ts           (baru)
    nuxt/pwa-update-prompt.spec.ts      (baru)
    nuxt/pwa-install-prompt.spec.ts     (baru)
  e2e/pwa.spec.ts                       (baru)
docs/
  adr/0019-web-pwa.md                   (baru)  berdampingan dengan ADR-0015, bukan menggantikannya
  adr/README.md                         (ubah)  baris indeks
  PROGRESS.md                           (ubah)
```

Manifest diletakkan di `frontend/pwa/manifest.ts`, di luar `app/`, supaya bisa diimpor
baik oleh `nuxt.config.ts` maupun oleh tes unit tanpa menyentuh auto-import.

## Gaya kode

Konfigurasi manifest terpisah dan bisa diuji:

```ts
// frontend/pwa/manifest.ts
// Satu-satunya sumber isi manifest. Diimpor nuxt.config.ts dan diuji di
// test/unit/pwa-manifest.spec.ts, sehingga daftar ikon tidak bisa menyimpang
// diam-diam dari berkas yang benar-benar ada di public/.
export const PWA_THEME_COLOR = '#005bfd' // --color-brand-500, main.css baris 33

export const pwaManifest = {
  id: '/',
  name: 'Inventra - Manajemen Aset',
  short_name: 'Inventra',
  description: 'Manajemen aset tetap dan inventaris',
  lang: 'id',
  start_url: '/',
  scope: '/',
  display: 'standalone',
  theme_color: PWA_THEME_COLOR,
  background_color: '#ffffff',
  icons: [
    { src: '/pwa-64x64.png', sizes: '64x64', type: 'image/png' },
    { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png' },
    { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png' },
    { src: '/maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' }
  ]
} as const
```

Komponen ajakan memakai `U*` dan i18n, tanpa satu pun string keras:

```vue
<!-- frontend/app/components/PwaUpdatePrompt.vue -->
<script setup lang="ts">
const { $pwa } = useNuxtApp()
const { t } = useI18n()
const dismissed = ref(false)
const show = computed(() => !!$pwa?.needRefresh && !dismissed.value)
</script>

<template>
  <UCard v-if="show" class="fixed inset-x-4 bottom-4 z-50 sm:left-auto sm:w-96">
    <p class="text-sm">{{ t('pwa.update.message') }}</p>
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton variant="ghost" color="neutral" @click="dismissed = true">
          {{ t('pwa.update.later') }}
        </UButton>
        <UButton color="primary" @click="$pwa.updateServiceWorker()">
          {{ t('pwa.update.reload') }}
        </UButton>
      </div>
    </template>
  </UCard>
</template>
```

Aturan gaya yang berlaku dan sudah ditegakkan ESLint di repo ini: tanpa koma di akhir
(`commaDangle: 'never'`), gaya kurung 1tbs.

## Strategi pengujian

Suite ini sengaja dibuat lebar, bukan hanya jalur bahagia, sesuai konvensi frontend repo.

**Unit, `test/unit/pwa-manifest.spec.ts` (lingkungan node)**

- Bidang wajib ada dan bernilai persis: `name`, `short_name`, `start_url`, `scope`, `display`, `lang`, `id`.
- `theme_color` sama dengan nilai `--color-brand-500` yang dibaca dari `app/assets/css/main.css`, sehingga rebrand tidak bisa membuat keduanya menyimpang tanpa tes merah.
- Ada ikon 192, 512, dan tepat satu maskable 512.
- Setiap `icons[].src` benar-benar ada sebagai berkas di `public/` (baca `fs`).
- Tidak ada `src` ikon yang menunjuk ke luar origin.

**Runtime Nuxt, `test/nuxt/pwa-update-prompt.spec.ts`**

- `needRefresh` bernilai salah: tidak merender apa pun.
- `needRefresh` bernilai benar: merender pesan i18n yang benar-benar teresolusi (bukan kunci mentah), dalam `id` dan `en`.
- Menekan tombol muat ulang memanggil `updateServiceWorker` tepat sekali.
- Menekan "nanti" menyembunyikan ajakan dan tidak memanggil `updateServiceWorker`.
- `$pwa` tidak tersedia sama sekali (modul gagal atau mode dev): komponen tidak melempar error.

**Runtime Nuxt, `test/nuxt/pwa-install-prompt.spec.ts`**

- Android dengan prompt tersedia: tombol pasang tampil; ditekan memanggil `showInstallPrompt` atau `install`.
- Sudah terpasang: tombol tidak tampil.
- Safari iOS belum standalone: petunjuk manual tampil, tombol pasang tidak.
- Safari iOS sudah standalone (`display-mode: standalone`): tidak ada ajakan apa pun.
- Desktop tanpa `beforeinstallprompt`: tidak ada ajakan, tidak ada error.
- Penutupan tersimpan: pemasangan ulang komponen tidak menampilkannya lagi.

**e2e, `e2e/pwa.spec.ts` (Chromium, terhadap `pnpm preview`)**

- Manifest terlayani 200, JSON-nya terurai, dan bidang wajibnya benar.
- Setiap URL ikon di manifest menjawab 200 dengan tipe gambar.
- `<link rel="manifest">` ada di dokumen.
- Service worker mencapai `activated` dalam batas waktu wajar.
- **Keamanan:** membaca `caches` lewat `page.evaluate` dan menegaskan nol entri ber-URL mengandung `/api/`, dijalankan setelah satu putaran login dan membuka layar berdata.
- Offline setelah satu kunjungan daring: navigasi tetap menampilkan shell aplikasi.
- Offline: permintaan data gagal dan layar menampilkan keadaan error aplikasi, bukan data basi.
- Rute `/en/` bekerja offline dengan shell yang sama.
- Muat ulang daring setelah offline mengembalikan keadaan normal.

**Pembuktian tes bisa merah.** Tiap acceptance criteria yang diuji harus dibuktikan bisa
gagal lewat mutasi terarah, sesuai kebiasaan repo. Contoh: hapus satu entri denylist dan
pastikan tes keamanan cache berubah merah.

**Verifikasi manual yang wajib dicatat hasilnya**

- Android Chrome nyata: pasang, buka dari ikon, periksa standalone dan warna bilah status.
- iOS Safari nyata: Tambahkan ke Layar Utama, buka dari ikon, periksa safe-area dan ikon.
- Lighthouse kategori PWA di produksi atau preview HTTPS: installable tanpa peringatan.
- Produksi di belakang Caddy: `manifest.webmanifest` dan `sw.js` tidak diblok WAF Coraza.

## Batasan

**Selalu**

- Jalankan `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`, dan seluruh e2e sebelum menyatakan selesai.
- Taruh tiap string baru di `id.json` dan `en.json`.
- Susun UI dari komponen `U*` Nuxt UI.
- Ambil warna dari token, tidak pernah dari angka yang diketik ulang di tempat kedua.
- Perbarui `docs/PROGRESS.md` dan vault Obsidian saat pekerjaan mendarat.
- Bekerja di branch `feat/web-pwa`, bukan langsung di `main`.

**Tanya dulu**

- Menambahkan runtime caching apa pun, sekecil apa pun, untuk respons API.
- Mengubah `routeRules` atau header cache yang ada.
- Menambah dependency di luar `@vite-pwa/nuxt` dan `@vite-pwa/assets-generator`.
- Menyentuh `docker-compose.prod.yml`, `ops/caddy/Caddyfile`, atau alur CD.
- Melebarkan cakupan ke Web Push, antrean tulis offline, atau Share Target.
- Mengubah tata letak layar mana pun yang punya mockup di `docs/design/`.

**Jangan pernah**

- Menyimpan respons `/api/**` ke Cache Storage atau IndexedDB.
- Menyimpan token, identitas pengguna, atau data aset di penyimpanan klien.
- Memakai `registerType: 'autoUpdate'` tanpa persetujuan pemilik produk, karena bisa membuang isian formulir yang sedang diketik.
- Melemahkan atau menghapus aturan `no-cache` pada shell tanpa ADR yang menjelaskannya.
- Menyentuh kode aplikasi Flutter di `mobile/`.
- Melonggarkan atau menonaktifkan e2e yang sudah ada supaya suite lulus.

## Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| `@vite-pwa/nuxt` belum resmi menyatakan dukungan Nuxt 4 | Modul gagal atau build tidak menghasilkan service worker | Langkah pembuktian di awal rencana; jalur cadangan `vite-plugin-pwa` langsung |
| Service worker mengganggu 40 spec e2e yang sudah ada | Suite jadi rapuh, kegagalan yang tidak berhubungan | Jalankan suite penuh berulang kali sebelum merge; tiap konteks Playwright baru bersih dari service worker |
| Shell ter-precache membuat pengguna memakai versi lama lebih lama | Pengguna melihat UI kemarin | `registerType: 'prompt'` plus `cleanupOutdatedCaches`; acceptance criteria 20 sampai 22 |
| WAF Coraza menolak `manifest.webmanifest` atau `sw.js` | PWA mati total di produksi, dan hanya ketahuan di produksi | Verifikasi manual di produksi masuk daftar wajib, bukan opsional |
| iOS Safari membuang Cache Storage setelah beberapa hari tidak dipakai | Kunjungan berikutnya mengunduh ulang | Diterima; tidak ada data penting yang hilang karena tidak ada data yang disimpan |
| Glyph pada ikon maskable terpotong saat Android memotongnya jadi lingkaran | Ikon terlihat cacat di layar utama | Acceptance criteria 10; periksa di perangkat Android nyata, bukan hanya di penampil berkas |
| Mode standalone menyembunyikan address bar | Pengguna tidak lagi melihat nama domain | Diterima sebagai default industri (Keputusan 13). Bisa dibalik ke `minimal-ui` dengan mengubah satu bidang manifest kalau kelak ada aturan internal yang menuntutnya |

## Kriteria keberhasilan

Fitur ini selesai ketika seluruh 40 acceptance criteria terpenuhi, dan secara khusus:

1. Seorang petugas ber-iPhone bisa memasang Inventra ke layar utama dan memakainya
   standalone, tanpa satu pun baris kode Flutter berubah.
2. Membuka aplikasi tanpa jaringan memberi shell aplikasi dan pesan yang bisa dibaca,
   bukan halaman error browser.
3. Pembacaan `Cache Storage` di perangkat pengguna membuktikan nol entri `/api/`, baik
   sebelum maupun sesudah logout.
4. Deploy baru sampai ke pengguna lewat ajakan yang mereka kendalikan, tanpa muat ulang
   mendadak dan tanpa error chunk 404.
5. Seluruh gerbang CI hijau: `frontend` dan `e2e`.

## Pertanyaan terbuka

Tidak ada. Lima pertanyaan yang diajukan saat spec ini pertama ditulis sudah dijawab
pemilik produk pada 2026-08-27:

| Pertanyaan | Jawaban | Masuk ke |
|---|---|---|
| Sumber ikon persegi | Belum ada logo yang disiapkan. Ikon diturunkan dari mark aplikasi yang sudah ada, logo BTN tidak dipakai | Keputusan 12, acceptance criteria 9 dan 10 |
| Nama di layar utama | "Inventra" | Keputusan 14, acceptance criteria 2 |
| Responsivitas layar yang ada | Sudah diperiksa pemilik produk, seluruh layar responsif di berbagai perangkat | Di luar cakupan; risiko terkait dihapus |
| Kebijakan `display` | Tidak ada aturan internal yang mewajibkan domain terlihat, jadi `standalone` | Keputusan 13, acceptance criteria 11 |
| Nomor ADR | ADR-0019 | Struktur proyek, acceptance criteria 40 |

## Handoff berikutnya

`/plan`. Langkah pertama rencana wajib berupa pembuktian pendek bahwa
`@vite-pwa/nuxt@1.1.1` benar-benar menghasilkan `sw.js` dan manifest pada Nuxt 4.5.1
dengan `ssr: false`, sebelum satu pun komponen UI ditulis. Kalau modulnya gagal, jalur
cadangan `vite-plugin-pwa` langsung yang dipakai, dan rencananya disesuaikan sebelum
pekerjaan UI dimulai.
