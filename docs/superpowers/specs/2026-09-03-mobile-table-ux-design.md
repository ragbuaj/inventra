# Spec: UX tabel data di layar mobile

- **Tanggal:** 2026-09-03
- **Branch:** `feat/mobile-table-ux`
- **Status:** Draft, menunggu persetujuan
- **Rencana implementasi:** `docs/superpowers/plans/2026-09-03-mobile-table-ux.md` (dibuat setelah spec disetujui)

## 1. Tujuan

Tiga perbaikan UI/UX pada lapisan daftar data (datatable) aplikasi web:

1. **Filter compact di mobile.** Di layar sempit, baris filter tidak lagi menumpahkan seluruh
   inputnya. Yang terlihat hanya kolom pencarian dan satu tombol ikon filter; filter lanjutan
   pindah ke slideover dari bawah, dengan badge penghitung filter aktif di tombolnya.
2. **Infinite scroll di mobile.** Di layar sempit, paginasi bernomor diganti pemuatan berkelanjutan
   saat pengguna menggulir, lengkap dengan indikator memuat, penanda akhir daftar, penanganan
   galat, pemulihan posisi gulir saat kembali dari halaman detail, dan windowing DOM bersyarat.
3. **Tombol halaman pertama dan terakhir** pada komponen paginasi.

### Pengguna sasaran

Pengguna Inventra yang membuka aplikasi web dari ponsel: petugas opname di lapangan, kepala unit
yang menyetujui pengajuan sambil bergerak, dan pegawai yang memeriksa aset pinjamannya. Aplikasi
web sudah installable sebagai PWA (PR #148), sehingga pemakaian dari ponsel bukan lagi kasus
pinggiran.

### Bukan tujuan (out of scope)

- Aplikasi Flutter di `mobile/` tidak tersentuh. Ini murni perubahan web responsif.
- Migrasi seluruh 15+ halaman daftar. Fase ini membangun komponen bersama dan membuktikannya di
  dua halaman pilot; sisanya menyusul di PR terpisah (lihat bagian 8).
- Perubahan tata letak desktop. Di lebar `md` ke atas, seluruh tampilan harus identik dengan
  sekarang, kecuali dua tombol paginasi baru.
- Perubahan kontrak API backend. Seluruh pekerjaan memakai endpoint `limit`/`offset` yang ada.

## 2. Keadaan sekarang

Diverifikasi langsung terhadap repo, bukan terhadap PROGRESS.md.

| Fakta | Bukti |
| --- | --- |
| Tidak ada filter bar bersama. Setiap halaman menulis sendiri baris filternya. | `DataToolbar.vue` ada tapi tidak dirujuk satu berkas pun di `app/`, `test/`, `e2e/`. |
| `ResourceTable.vue` dipakai 6 halaman. | `master/categories`, `master/employees`, `master/reference`, `settings/guide`, `settings/users`, `dev/components`. |
| Halaman daftar lain memakai `UTable` mentah atau daftar kartu, memasang `TablePagination` sendiri. | `assets/index`, `depreciation`, `notifications`, `reports`, `settings/audit`, `stock-opname`. |
| Dua model paginasi hidup berdampingan. | Server-side (`assets`, `audit`, `notifications`: `limit`/`offset`, `PAGE_SIZE` 10) dan in-memory (`reports`, `stock-opname`: sort lalu slice di klien). |
| Kontainer gulir bukan `window`. | `app/layouts/default.vue`: `<main class="flex-1 overflow-y-auto ...">` di dalam `div.h-screen.overflow-hidden`. |
| `@tanstack/vue-virtual@3.13.32` sudah ada sebagai dependensi transitif. | `node_modules/.pnpm/@tanstack+vue-virtual@3.13.32_...`, ditarik oleh `@nuxt/ui`. |
| `useMediaQuery` sudah ada dan bisa di-mock di tes. | `app/composables/useMediaQuery.ts`; `mockNuxtImport` dipakai di belasan berkas `test/nuxt/`. |
| Mockup di `docs/design/` seluruhnya desktop. | Tidak ada mockup web-mobile untuk filter maupun tabel. |

**Konsekuensi kontrak mockup.** Karena tidak ada mockup web-mobile, butir 1 dan 2 adalah wilayah
desain baru, bukan penyimpangan dari mockup yang ada. Aturan CLAUDE.md tetap berlaku penuh untuk
lebar `md` ke atas: tampilan desktop wajib tetap 1:1 dengan `docs/design/`. Keputusan desain mobile
di spec ini dicatat sebagai keputusan produk di vault setelah disetujui.

## 3. Keputusan yang sudah diambil

Tiga keputusan ini sudah dikonfirmasi pemilik produk pada 2026-09-03 sebelum spec ditulis.

| # | Keputusan | Alasan |
| --- | --- | --- |
| K1 | Bangun komponen bersama dulu, buktikan di 2 halaman pilot. | Pola terbukti sebelum disebar ke 15+ halaman; PR tetap bisa ditinjau manusia. |
| K2 | Virtualisasi bersyarat di atas ambang, bukan selalu. | Satu fetch hanya 10 baris. Windowing merusak Ctrl+F browser, scroll anchoring, dan pembacaan pembaca layar; biayanya baru sepadan di ratusan baris DOM. |
| K3 | Filter lanjutan tampil di slideover dari bawah, dengan badge hitungan. | Pola standar aplikasi mobile; ruang lega untuk input tanggal dan picker async bertingkat. |

## 4. Batasan

**Selalu dilakukan.**

- Bangun di atas komponen Nuxt UI (`U*`). Tidak menggulirkan sendiri tombol, input, atau overlay.
- Seluruh teks yang terlihat pengguna masuk `i18n/locales/{id,en}.json`.
- Warna lewat token semantik, bukan kelas Tailwind literal.
- Setiap komponen dan composable baru mendapat tes: unit untuk logika murni, `mountSuspended`
  untuk komponen, e2e untuk alur pengguna. Cakupan menyentuh keadaan kosong, memuat, galat,
  batas ambang, dan variasi lebar layar, bukan hanya jalur bahagia.
- Gerbang CI dijalankan dan hijau sebelum commit: `pnpm lint`, `vue-tsc`, `pnpm test`, `pnpm build`.

**Ditanyakan dulu.**

- Setiap perubahan yang terlihat di lebar `md` ke atas, di luar dua tombol paginasi baru.
- Menaikkan `PAGE_SIZE` mana pun.
- Mengubah kontrak `limit`/`offset` di `backend/api/openapi.yaml`.

**Tidak boleh.**

- Menambah pustaka komponen di luar Nuxt UI.
- Mengganti `UTable` dengan tabel gulung sendiri demi virtualisasi (lihat bagian 6.4).
- Menyentuh `mobile/` (Flutter) atau berkas backend mana pun.
- Membiarkan tampilan desktop berubah tanpa persetujuan.

## 5. Titik henti breakpoint

Satu definisi dipakai konsisten:

- **Compact (mobile)** adalah lebar viewport `< 768px`, yaitu di bawah `md` Tailwind.
- **Reguler (desktop/tablet)** adalah `>= 768px`.

Disediakan `useIsCompact()` di `app/composables/useMediaQuery.ts`, memakai
`useMediaQuery('(max-width: 767.98px)', false)`. Nilai bawaan `false` berarti lingkungan tanpa
`matchMedia` (runtime tes Nuxt) menganggap dirinya desktop, sehingga seluruh tes yang sudah ada
tidak berubah perilakunya. Tes mobile meng-override lewat `mockNuxtImport`.

## 6. Rancangan

### 6.1 `FilterBar.vue` (komponen baru)

Menggantikan baris filter yang ditulis tangan di tiap halaman. `DataToolbar.vue` yang mati dihapus
dalam PR ini.

```
Props
  search?: string            (v-model:search)
  searchPlaceholder?: string
  activeCount?: number       jumlah filter lanjutan yang aktif, dihitung halaman (default 0)
  showReset?: boolean        (default true)
  total?: number             opsional, untuk label hasil di footer slideover
  testid?: string            awalan data-testid (default 'filter-bar')

Emits
  update:search, reset

Slots
  filters   kontrol filter lanjutan
  trailing  isi rata kanan, mis. pengalih tampilan tabel/grid
```

**Reguler.** Kartu satu baris, persis seperti markup yang ada sekarang di `assets/index.vue`:
`UInput` pencarian (`flex-1 min-w-[220px]`), lalu slot `filters`, lalu tombol Reset saat
`activeCount > 0`, lalu pengisi, lalu slot `trailing`.

**Compact.** Kartu satu baris berisi: `UInput` pencarian yang mengisi lebar, satu `UButton` ikon
`i-lucide-sliders-horizontal` dibungkus `UChip` yang menampilkan `activeCount` saat lebih dari nol,
lalu slot `trailing`. Menekan tombol membuka `USlideover side="bottom"` berisi slot `filters`
tersusun vertikal selebar penuh.

**Terapkan langsung, bukan draf.** Kontrol filter tetap terikat langsung ke state halaman, jadi
mengubah filter langsung berlaku dan daftar di belakang slideover ikut berubah. Tombol footer
berlabel jumlah hasil (`Lihat 128 hasil`) hanya menutup slideover; tidak ada state draf.
Alasannya: nilai filter dimiliki halaman lewat `v-model` di dalam slot, sehingga `FilterBar` tidak
punya cara generik untuk menahan dan menerapkannya belakangan tanpa memaksa setiap halaman
menduplikasi state filternya. Terapkan-langsung juga menghilangkan kelas bug "sudah diubah tapi
lupa menekan Terapkan".

**Aksesibilitas.** Tombol filter membawa `aria-expanded` dan `aria-label` yang menyebut jumlah
filter aktif. Slideover punya judul. Fokus kembali ke tombol filter saat slideover ditutup
(ditangani `USlideover`).

**Testid.** `{testid}-search`, `{testid}-toggle`, `{testid}-panel`, `{testid}-reset`,
`{testid}-apply`.

### 6.2 `useInfiniteRows` (composable baru)

Akumulasi baris di atas fungsi ambil-per-halaman. Tidak tahu-menahu soal render.

```ts
useInfiniteRows<T>(
  fetchPage: (arg: { limit: number, offset: number }) => Promise<{ data: T[], total: number }>,
  opts?: { limit?: number }   // default 10
)
// mengembalikan
{ rows, total, loading, loadingMore, error, done, loadFirst, loadMore, retry, reset }
```

- `loading` hanya untuk pemuatan halaman pertama; `loadingMore` untuk penambahan berikutnya.
  Pemisahan ini yang membuat skeleton tidak berkedip saat menggulir.
- `done` bernilai benar saat `rows.length >= total`.
- `loadMore()` tidak melakukan apa-apa saat sedang memuat, sudah `done`, atau sedang `error`.
- Penjaga balapan lewat penghitung urutan: respons basi dibuang, mengikuti pola `seq` yang sudah
  dipakai `assets/index.vue`.
- Galat saat `loadMore` mempertahankan baris yang sudah ada dan memunculkan `retry`; galat saat
  halaman pertama memunculkan keadaan galat penuh.

### 6.3 `InfiniteList.vue` (komponen baru)

Merender daftar terakumulasi untuk jalur kartu/daftar, dengan windowing bersyarat.

```
Props
  items: unknown[]
  loadingMore, done, error: boolean
  threshold?: number          default 200
  estimateSize?: number       default 96 (px, taksiran tinggi satu item)
  scrollParent?: HTMLElement | null
Emits
  load-more, retry
Slot
  item({ item, index })
```

- Di bawah `threshold`: `v-for` biasa. Seluruh baris ada di DOM, Ctrl+F dan scroll anchoring utuh.
- Di `threshold` ke atas: `useVirtualizer` dari `@tanstack/vue-virtual`, dengan
  `getScrollElement: () => scrollParent` dan `scrollMargin` dihitung dari offset daftar di dalam
  induk gulir. Pengukuran dinamis dipakai karena tinggi kartu tidak seragam.
- Sentinel `div` di bawah daftar diamati `IntersectionObserver` dengan `root = scrollParent` dan
  `rootMargin: '400px'`, memancarkan `load-more`.
- Daerah status di bawah daftar membawa `role="status"` dan `aria-live="polite"`, menampilkan
  salah satu dari: pemintal plus teks memuat, teks akhir daftar, atau pesan galat plus tombol
  coba lagi.

### 6.4 Jalur tabel: akumulasi tanpa windowing

**Ini penyimpangan sadar dari kata "DOM virtualization" di permintaan, dan perlu diketahui
eksplisit.**

`ResourceTable` merender lewat `UTable`, yang memiliki `<tbody>`-nya sendiri. Windowing `<tr>`
menuntut penempatan absolut baris dan pengambilalihan `<tbody>`, yang berarti meninggalkan
`UTable`. Itu bertabrakan langsung dengan batasan wajib "selalu bangun di atas komponen Nuxt UI".

Karena itu, di jalur tabel:

- Baris menumpuk tanpa windowing.
- Ada batas keras `MAX_TABLE_ROWS = 300`. Setelah tercapai, pemuatan otomatis berhenti dan
  digantikan tombol eksplisit `Muat lebih banyak`, sehingga DOM tidak pernah membengkak diam-diam.
- Jalur kartu (`InfiniteList`) tetap mendapat windowing penuh sesuai K2.

Alternatifnya adalah merender `ResourceTable` sebagai daftar kartu di mobile, yang sebenarnya UX
mobile yang lebih baik dan membuat windowing seragam. Itu tidak diambil di fase ini karena
menuntut keputusan per halaman tentang bidang mana yang muncul di kartu, dan itu keputusan desain
yang butuh persetujuan tersendiri. Dicatat sebagai kandidat fase lanjutan di bagian 8.

### 6.5 Pemulihan posisi gulir

`useListStateCache(key)` (composable baru) menyimpan, per jalur rute:

```ts
{ rows, total, scrollTop, signature }
```

- `signature` adalah string dari seluruh nilai filter aktif. Saat halaman dipasang kembali dan
  `signature` cocok, baris dan `total` dipulihkan tanpa fetch, lalu `scrollTop` induk gulir
  disetel setelah `nextTick`.
- Saat `signature` tidak cocok, cache dibuang dan halaman memuat dari awal.
- Cache disimpan di Map cakupan modul, bukan `sessionStorage`. Alasannya: isinya data operasional
  bank (baris aset lengkap dengan nilai buku) dan aturan privasi repo melarang menaruhnya di
  penyimpanan yang bertahan lintas sesi. Konsekuensinya cache hilang saat muat ulang penuh, dan
  itu memang perilaku yang diinginkan.
- Cache dibersihkan saat logout.

### 6.6 `TablePagination.vue`: halaman pertama dan terakhir

- Dua `UButton` baru mengapit kontrol yang ada: `i-lucide-chevrons-left` di paling kiri dan
  `i-lucide-chevrons-right` di paling kanan.
- `data-testid`: `pagination-first`, `pagination-last`.
- Nonaktif saat sudah di halaman pertama atau terakhir, mengikuti pola `pagination-prev`/`next`.
- `aria-label` lewat kunci i18n baru `common.firstPage` dan `common.lastPage`.
- Muncul hanya saat `totalPages > MAX_PAGE_BUTTONS` (yaitu lebih dari 3 halaman). Saat seluruh
  halaman sudah terlihat sebagai tombol angka, lompat ke pertama/terakhir tidak menambah apa pun
  dan hanya meramaikan baris.

### 6.7 Halaman pilot

| Halaman | Jalur render | Yang dibuktikan |
| --- | --- | --- |
| `app/pages/assets/index.vue` (Katalog Aset) | Kartu `AssetCard` di mobile, lewat `InfiniteList` | Rangkaian penuh: `FilterBar` dengan 4 filter lanjutan termasuk `AsyncSearchPicker`, `useInfiniteRows` di atas API server-side, windowing di atas ambang, pemulihan gulir saat kembali dari detail aset. |
| `app/pages/settings/users.vue` (Manajemen User) | `ResourceTable` | Jalur tabel: `FilterBar` di dalam halaman ber-`ResourceTable`, akumulasi baris dengan batas 300 plus tombol muat lebih banyak. |

`app/pages/dev/components.vue` diperbarui sebagai etalase `FilterBar` dan `InfiniteList`.

## 7. Acceptance criteria

### Filter compact

1. Di viewport `>= 768px`, baris filter `assets/index` dan `settings/users` merender kolom
   pencarian dan seluruh filter lanjutannya sebaris, sama seperti sebelum perubahan.
2. Di viewport `< 768px`, baris filter hanya merender kolom pencarian, tombol ikon filter, dan
   isi slot `trailing`. Tidak ada kontrol filter lanjutan yang terlihat.
3. Menekan tombol filter di mobile membuka slideover dari bawah yang memuat seluruh kontrol
   filter lanjutan halaman itu.
4. Tombol filter menampilkan badge berisi jumlah filter lanjutan yang aktif, dan badge itu hilang
   saat jumlahnya nol.
5. Mengubah filter di dalam slideover langsung berlaku pada daftar tanpa menekan tombol apa pun.
6. Footer slideover menampilkan jumlah hasil saat `total` diberikan.
7. Tombol Reset di dalam slideover mengosongkan seluruh filter lanjutan dan pencarian, dan badge
   kembali kosong.
8. Tombol filter membawa `aria-expanded` yang benar dan `aria-label` yang menyebut jumlah filter
   aktif.
9. Mengubah lebar viewport dari mobile ke desktop saat slideover terbuka menutup slideover dan
   memindahkan kontrol kembali ke baris sebaris, tanpa kehilangan nilai filter.
10. `DataToolbar.vue` terhapus dan tidak ada rujukan tersisa di seluruh repo.

### Infinite scroll

11. Di viewport `< 768px`, `assets/index` dan `settings/users` tidak merender `TablePagination`.
12. Di viewport `>= 768px`, keduanya tetap merender `TablePagination` dan tidak memuat apa pun
    saat digulir.
13. Menggulir hingga sentinel masuk viewport memuat halaman berikutnya dan menambahkannya di
    bawah baris yang ada, tanpa memindahkan posisi gulir pengguna.
14. Selama pemuatan tambahan berjalan, indikator memuat tampil di bawah daftar, dan skeleton
    halaman pertama tidak muncul kembali.
15. Saat seluruh baris sudah dimuat (`rows.length >= total`), penanda akhir daftar tampil dan
    tidak ada permintaan jaringan tambahan meski sentinel terus terlihat.
16. Galat jaringan saat pemuatan tambahan mempertahankan baris yang sudah tampil dan memunculkan
    tombol coba lagi; menekannya memuat ulang halaman yang gagal saja.
17. Mengubah filter atau pencarian mengosongkan akumulasi, menggulir kembali ke atas, dan memuat
    dari offset nol.
18. Respons yang datang terlambat dari permintaan yang sudah tidak relevan tidak pernah masuk ke
    daftar.
19. Di jalur kartu, saat baris terakumulasi melewati 200, hanya sebagian item yang ada di DOM,
    dan menggulir naik-turun tetap menampilkan item yang benar di posisi yang benar.
20. Di jalur kartu, di bawah 200 baris seluruh item ada di DOM (tidak ada windowing).
21. Di jalur tabel, akumulasi berhenti otomatis di 300 baris dan digantikan tombol
    `Muat lebih banyak` yang eksplisit.
22. Daerah status daftar membawa `role="status"` dan `aria-live="polite"`.
23. Membuka detail aset dari katalog mobile lalu menekan tombol kembali memulihkan baris yang
    sudah terakumulasi dan posisi gulir sebelumnya, tanpa memuat ulang dari offset nol.
24. Pemulihan pada kriteria 23 tidak terjadi jika filter berubah di antaranya; dalam hal itu
    daftar memuat ulang dari awal.
25. Cache daftar tidak ditulis ke `sessionStorage`, `localStorage`, maupun IndexedDB.
26. Cache daftar kosong setelah logout.

### Paginasi

27. `TablePagination` merender tombol halaman pertama dan terakhir saat jumlah halaman lebih dari
    3.
28. Tombol tersebut tidak dirender saat jumlah halaman 3 atau kurang.
29. Tombol halaman pertama nonaktif di halaman 1; tombol halaman terakhir nonaktif di halaman
    terakhir.
30. Menekan tombol halaman pertama memancarkan `update:offset` bernilai 0.
31. Menekan tombol halaman terakhir memancarkan `update:offset` bernilai offset halaman terakhir.
32. Keduanya membawa `aria-label` hasil terjemahan, bukan teks keras.
33. Seluruh tes `test/nuxt/table-pagination.spec.ts` yang sudah ada tetap lulus tanpa diubah
    maknanya.

### Umum

34. Seluruh teks baru ada di `i18n/locales/id.json` dan `en.json`, dan tidak ada string keras
    yang terlihat pengguna di komponen baru.
35. `pnpm lint`, `vue-tsc --noEmit`, `pnpm test`, dan `pnpm build` hijau.
36. Suite e2e Playwright hijau, termasuk spesifikasi mobile baru yang menjalankan filter compact
    dan infinite scroll di viewport ponsel.
37. Tampilan desktop `assets/index` dan `settings/users` dibandingkan berdampingan dengan
    `docs/design/Katalog Aset.dc.html` dan `docs/design/Manajemen User.dc.html`, dan cocok
    struktur demi struktur (warna merek dikecualikan sesuai CLAUDE.md).
38. `docs/PROGRESS.md` diperbarui pada commit yang menuntaskan pekerjaan ini.

## 8. Yang tersisa setelah fase ini

Dicatat agar tidak hilang, bukan bagian dari PR ini.

1. Migrasi 13+ halaman daftar sisanya ke `FilterBar` dan mode infinite mobile: `disposals`,
   `maintenance`, `transfers`, `peminjaman`, `stock-opname`, `reports`, `notifications`,
   `depreciation`, `settings/audit`, `settings/guide`, `master/categories`, `master/employees`,
   `master/reference`, `master/offices`.
2. Keputusan desain: apakah `ResourceTable` di mobile sebaiknya merender daftar kartu, bukan
   tabel yang menggulir mendatar. Kalau ya, jalur tabel ikut mendapat windowing dan batas 300
   baris di bagian 6.4 bisa dicabut.
3. Halaman yang memaginasi in-memory (`reports`, `stock-opname`) perlu jalur tersendiri karena
   `useInfiniteRows` mengasumsikan pengambilan per halaman dari server.

## 9. Strategi pengujian

**Unit (`test/unit/`, lingkungan node).**

- `useInfiniteRows`: penambahan baris, perhitungan `done`, `loadMore` yang tidak melakukan apa-apa
  saat sedang memuat atau sudah selesai, penjaga balapan membuang respons basi, galat halaman
  pertama versus galat pemuatan tambahan, `reset` mengosongkan seluruh state.
- `useListStateCache`: simpan lalu pulihkan, `signature` tidak cocok membatalkan pemulihan,
  pembersihan saat logout.

**Runtime Nuxt (`test/nuxt/`, `mountSuspended`).**

- `FilterBar`: tata letak reguler versus compact lewat `mockNuxtImport` atas `useIsCompact`, badge
  hitungan, buka dan tutup slideover, pemancaran `reset`, atribut aksesibilitas.
- `InfiniteList`: render di bawah dan di atas ambang, pemancaran `load-more`, ketiga keadaan
  daerah status.
- `TablePagination`: tambahan kasus untuk tombol pertama dan terakhir, termasuk kemunculan
  bersyarat pada 3 halaman atau kurang.
- `ResourceTable`: mode infinite mobile menyembunyikan paginasi; batas 300 baris.

**E2E (`e2e/`, Playwright).**

- Spesifikasi baru dengan viewport ponsel, disetel per berkas lewat
  `test.use({ viewport: { width: 390, height: 844 } })`.

  **Koreksi (2026-09-04, ditemukan di gerbang ship).** Rencana awal menyatakan spec ini bisa ikut
  proyek `chromium` sehingga `playwright.config.ts` dan tahapan CI tidak perlu diubah. Itu keliru:
  fase `chromium` di CI berjalan terhadap DB yang **bersih** — hanya superadmin ter-seed, nol aset
  dan satu user. Tata letak compact justru baru ada setelah daftar melewati satu halaman, dan aset
  **tidak bisa dibuat langsung** (tidak ada `POST /assets`; setiap aset lahir lewat maker-checker),
  jadi membangun 25 aset sebagai fixture per spec tidak praktis. Spec ini karena itu pindah ke
  proyek baru `seeded-ui` yang berjalan setelah seed demo diterapkan, dengan satu langkah CI
  tambahan. Lolos lokal sebelumnya semata karena DB dev sudah ter-seed.
- Isi spesifikasi itu: buka Katalog Aset, pastikan filter lanjutan tersembunyi, buka slideover,
  terapkan filter, pastikan daftar berubah dan badge terisi.
- Gulir untuk memicu pemuatan tambahan, pastikan jumlah kartu bertambah, buka detail, tekan
  kembali, pastikan posisi gulir dan jumlah kartu pulih.
- Pastikan suite desktop yang ada tidak berubah perilakunya.

Mengikuti catatan e2e repo: data unik per jalannya tes, tegaskan setelah pencarian, dan backend
lokal dijalankan dengan `RATELIMIT_ENABLED=false`.

## 10. Risiko

| Risiko | Penanganan |
| --- | --- |
| Windowing di dalam induk gulir yang juga memuat filter bar menuntut `scrollMargin` yang benar; salah hitung membuat daftar melompat. | Tes runtime yang menegaskan item yang benar tampil setelah menggulir, plus verifikasi manual di panel browser. |
| Pemulihan gulir bertabrakan dengan `scrollBehavior` bawaan Nuxt. | Pemulihan dilakukan pada induk gulir `<main>`, bukan `window`, jadi tidak beririsan dengan pemulihan bawaan. |
| Perubahan `FilterBar` di `settings/users` menyentuh `data-testid` yang dipakai e2e yang ada. `e2e/settings.spec.ts:69` menegaskan `users-filter-reset` terlihat. | Testid dipertahankan apa adanya lewat slot. Catatan: di mobile tombol reset pindah ke dalam slideover, jadi spesifikasi desktop itu tetap lulus karena proyek `chromium` memakai `devices['Desktop Chrome']`. Seluruh e2e di-grep ulang sebelum commit sesuai pelajaran PR sebelumnya soal penukaran komponen yang memecahkan selektor. |
| Menambah `@tanstack/vue-virtual` sebagai dependensi langsung bisa bertabrakan dengan versi yang ditarik `@nuxt/ui`. | Disematkan ke versi yang sudah terpasang (3.13.32) dan diperiksa `pnpm list` tidak menghasilkan dua salinan. |
