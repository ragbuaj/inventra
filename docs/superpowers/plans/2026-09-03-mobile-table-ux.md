# Rencana implementasi: UX tabel data di layar mobile

- **Spec:** `docs/superpowers/specs/2026-09-03-mobile-table-ux-design.md` (38 acceptance criteria)
- **Branch:** `feat/mobile-table-ux` (dari `origin/main` di `d8e5e96`)
- **Tanggal:** 2026-09-03
- **Jumlah tugas:** 12, dalam 4 fase, dengan 3 titik pemeriksaan

## Cara membaca rencana ini

Tiap tugas mandiri: bisa di-commit sendiri, gerbangnya hijau saat itu juga, dan tidak
meninggalkan repo dalam keadaan setengah jadi. Kolom "Menutup AC" merujuk nomor acceptance
criteria di spec.

Perintah verifikasi yang dipakai berulang, dijalankan dari `frontend/`:

```bash
pnpm lint && npx vue-tsc -p .nuxt/tsconfig.json --noEmit && pnpm test && pnpm build
```

Setelah menambah composable atau util baru, jalankan `npx nuxi prepare` lebih dulu supaya tipe
auto-import ikut ter-generate. Untuk e2e, backend harus hidup dengan `RATELIMIT_ENABLED=false`.

## Graf ketergantungan

```
T1 TablePagination pertama/terakhir      (mandiri penuh)
      |
T2 useIsCompact + FilterBar
      |
      +--> T3 settings/users pakai FilterBar
      |
      +--> T4 assets/index pakai FilterBar
                  |
   [ Titik pemeriksaan A: paritas desktop ]
                  |
T5 useInfiniteRows -----+
                        |
T6 InfiniteList --------+--> T7 assets/index mode infinite mobile
                        |         |
                        |    T8 useListStateCache + pemulihan gulir
                        |
                        +--> T9 ResourceTable mode infinite + settings/users
                  |
   [ Titik pemeriksaan B: perilaku mobile ]
                  |
T10 e2e viewport ponsel
T11 gerbang penuh + perbandingan berdampingan
T12 PROGRESS.md + vault
                  |
   [ Titik pemeriksaan C: siap PR ]
```

T5 dan T6 tidak saling bergantung dan boleh dikerjakan paralel. T1 tidak bergantung apa pun dan
boleh didahulukan kapan saja.

---

## Fase 1: Paginasi

Irisan terkecil yang berdiri sendiri, tanpa menyentuh apa pun yang lain.

### T1. Tombol halaman pertama dan terakhir di `TablePagination`

**Menutup AC:** 27, 28, 29, 30, 31, 32, 33

**Berkas:**
- `frontend/app/components/TablePagination.vue`
- `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`
- `frontend/test/nuxt/table-pagination.spec.ts`

**Yang dikerjakan:**
1. Tambah dua `UButton` mengapit kontrol yang ada: `i-lucide-chevrons-left` sebelum
   `pagination-prev`, `i-lucide-chevrons-right` sesudah `pagination-next`.
2. `data-testid`: `pagination-first`, `pagination-last`.
3. Render bersyarat `v-if="totalPages > MAX_PAGE_BUTTONS"`.
4. Nonaktif di batas, meniru pola `pagination-prev`/`next` yang ada.
5. Kunci i18n baru `common.firstPage`, `common.lastPage` untuk `aria-label`.

**Acceptance criteria tugas:**
- Dengan 20 halaman, `pagination-first` dan `pagination-last` terender.
- Dengan 3 halaman, keduanya tidak terender.
- Di halaman 1, `pagination-first` nonaktif; di halaman terakhir, `pagination-last` nonaktif.
- Klik `pagination-first` memancarkan `update:offset` bernilai `0`.
- Klik `pagination-last` memancarkan `update:offset` bernilai `(totalPages - 1) * limit`.
- Kedua tombol punya `aria-label` yang bukan kunci i18n mentah.
- Sebelas tes lama di `table-pagination.spec.ts` lulus tanpa diubah maknanya.

**Verifikasi:** `pnpm test -- table-pagination`, lalu rangkaian gerbang penuh.

---

## Fase 2: Filter compact

### T2. `useIsCompact()` dan komponen `FilterBar`

**Menutup AC:** 3, 4, 5, 6, 7, 8, 9 (di tingkat komponen; pembuktian di halaman nyata di T3/T4)

**Berkas:**
- `frontend/app/composables/useMediaQuery.ts` (tambah `useIsCompact`)
- `frontend/app/components/FilterBar.vue` (baru)
- `frontend/i18n/locales/{id,en}.json`
- `frontend/app/pages/dev/components.vue` (etalase)
- `frontend/test/nuxt/filter-bar.spec.ts` (baru)

**Yang dikerjakan:**
1. `useIsCompact()` memakai `useMediaQuery('(max-width: 767.98px)', false)`. Bawaan `false`
   supaya seluruh tes yang ada tetap menganggap dirinya desktop.
2. `FilterBar.vue` sesuai kontrak props/emits/slots di spec bagian 6.1.
3. Cabang reguler: kartu sebaris, meniru persis markup filter yang ada sekarang
   (`bg-default border border-default rounded-[13px] p-[14px] shadow-sm mb-4 flex items-center
   gap-2.5 flex-wrap`).
4. Cabang compact: pencarian selebar sisa, `UButton` ikon `i-lucide-sliders-horizontal` dibungkus
   `UChip` berisi `activeCount`, lalu slot `trailing`. `USlideover side="bottom"` memuat slot
   `filters` tersusun vertikal, footer berisi Reset dan tombol tutup berlabel jumlah hasil.
5. Kunci i18n baru di bawah `common.filterBar.*`: `advanced`, `apply`, `applyWithCount`,
   `activeCount`, `title`.
6. Tutup slideover otomatis saat `isCompact` berubah menjadi `false`.
7. Tambahkan etalase di `dev/components.vue`.

**Acceptance criteria tugas:**
- Dengan `useIsCompact` di-mock `false`, isi slot `filters` ada di DOM dan `filter-bar-toggle`
  tidak ada.
- Dengan `useIsCompact` di-mock `true`, `filter-bar-toggle` ada dan isi slot `filters` tidak ada
  di DOM sebelum slideover dibuka.
- Badge menampilkan `activeCount` saat lebih dari nol dan hilang saat nol.
- Menekan `filter-bar-toggle` membuka panel dan memunculkan isi slot `filters`.
- `filter-bar-reset` memancarkan `reset`.
- Mengetik di `filter-bar-search` memancarkan `update:search` dengan nilai yang benar.
- `filter-bar-toggle` membawa `aria-expanded` yang berubah saat panel dibuka, dan `aria-label`
  yang memuat jumlah filter aktif.
- Berpindah dari compact ke reguler saat panel terbuka menutup panel.
- Footer menampilkan jumlah hasil saat prop `total` diberikan, dan label polos saat tidak.

**Verifikasi:** `npx nuxi prepare`, `pnpm test -- filter-bar`, lalu gerbang penuh.

### T3. `settings/users.vue` memakai `FilterBar`, hapus `DataToolbar`

**Menutup AC:** 1, 2, 10 (untuk Manajemen User)

**Berkas:**
- `frontend/app/pages/settings/users.vue`
- `frontend/app/components/DataToolbar.vue` (dihapus)
- `frontend/test/nuxt/` (spesifikasi halaman users bila ada; kalau belum ada, dibuat)

**Yang dikerjakan:**
1. Ganti blok "Filter bar" tulisan tangan dengan `FilterBar`, memindahkan `USelect` peran,
   `AsyncSearchPicker` kantor, dan `USelect` status ke slot `filters`.
2. Pertahankan `data-testid` apa adanya: `users-role-filter`, `users-filter-office`,
   `users-status-filter`, `users-filter-reset`.
3. Sambungkan `active-count` ke hitungan filter lanjutan yang aktif, terpisah dari `search`.
4. Hapus `DataToolbar.vue`.

**Acceptance criteria tugas:**
- Di lebar reguler, markup filter secara visual sama dengan sebelum perubahan.
- Di lebar compact, hanya pencarian dan tombol filter yang terlihat.
- `grep -rn "DataToolbar" frontend/` tidak menghasilkan apa pun.
- `grep -rn "users-role-filter\|users-filter-office\|users-status-filter\|users-filter-reset"
  frontend/e2e/` masih cocok dengan testid yang terender di lebar desktop.

**Verifikasi:** gerbang penuh, plus `e2e/settings.spec.ts` hijau.

### T4. `assets/index.vue` memakai `FilterBar`

**Menutup AC:** 1, 2 (untuk Katalog Aset)

**Berkas:** `frontend/app/pages/assets/index.vue`

**Yang dikerjakan:**
1. Ganti blok "Filter bar" dengan `FilterBar`; pindahkan status, kategori,
   `AsyncSearchPicker` kantor, dan kelas ke slot `filters`.
2. Pengalih tampilan tabel/grid pindah ke slot `trailing` supaya tetap terlihat di mobile.
3. `active-count` dihitung dari empat filter lanjutan, tidak termasuk `search`.
4. Pertahankan `assets-office-filter`.

**Acceptance criteria tugas:**
- Sama seperti T3, untuk Katalog Aset.
- Pengalih tampilan tetap terlihat dan berfungsi di lebar compact.
- `e2e/assets.spec.ts` hijau tanpa perubahan.

**Verifikasi:** gerbang penuh, plus `e2e/assets.spec.ts`.

### Titik pemeriksaan A

Buka `assets/index` dan `settings/users` di panel browser pada lebar desktop, sandingkan dengan
`docs/design/Katalog Aset.dc.html` dan `docs/design/Manajemen User.dc.html`. Pastikan cocok
struktur demi struktur (warna merek dikecualikan). Laporkan hasilnya sebelum lanjut ke Fase 3.

---

## Fase 3: Infinite scroll

### T5. Composable `useInfiniteRows`

**Menutup AC:** 18 (penjaga balapan), dan menjadi mesin bagi 13 sampai 17

**Berkas:**
- `frontend/app/composables/useInfiniteRows.ts` (baru)
- `frontend/test/unit/use-infinite-rows.spec.ts` (baru)

**Yang dikerjakan:** sesuai kontrak di spec bagian 6.2.

**Acceptance criteria tugas:**
- `loadFirst()` mengisi `rows` dan `total`, dan `loading` benar hanya selama pemanggilan itu.
- `loadMore()` menambahkan di belakang, tidak menimpa, dan menyalakan `loadingMore` bukan
  `loading`.
- `done` menjadi benar saat `rows.length >= total`.
- `loadMore()` tidak memanggil `fetchPage` saat sedang memuat, saat `done`, atau saat `error`.
- Respons yang datang setelah permintaan yang lebih baru dimulai dibuang seluruhnya.
- Galat di halaman pertama menyalakan `error` dan mengosongkan `rows`.
- Galat di pemuatan tambahan menyalakan `error` tetapi mempertahankan `rows`, dan `retry()`
  mengulang offset yang gagal saja.
- `reset()` mengosongkan `rows`, `total`, `error`, dan seluruh bendera.

**Verifikasi:** `npx nuxi prepare`, `pnpm test -- use-infinite-rows`, gerbang penuh.

### T6. Komponen `InfiniteList`

**Menutup AC:** 19, 20, 22

**Berkas:**
- `frontend/package.json` (deklarasikan `@tanstack/vue-virtual` eksplisit di versi terpasang)
- `frontend/app/components/InfiniteList.vue` (baru)
- `frontend/app/pages/dev/components.vue` (etalase)
- `frontend/test/nuxt/infinite-list.spec.ts` (baru)

**Yang dikerjakan:** sesuai kontrak di spec bagian 6.3.

Catatan versi: `@tanstack/vue-virtual@3.13.32` sudah terpasang sebagai dependensi transitif dari
`@nuxt/ui`. Deklarasikan tepat pada versi itu, lalu buktikan `pnpm list @tanstack/vue-virtual`
hanya menunjukkan satu salinan.

**Acceptance criteria tugas:**
- Dengan 50 item dan `threshold` 200, seluruh 50 item ada di DOM.
- Dengan 500 item dan `threshold` 200, jumlah item di DOM jauh lebih kecil dari 500.
- Sentinel yang memasuki viewport memancarkan `load-more` tepat satu kali per perpotongan.
- `load-more` tidak dipancarkan saat `done` atau `error` benar.
- Daerah status merender pemintal saat `loadingMore`, teks akhir daftar saat `done`, dan pesan
  galat plus tombol coba lagi saat `error`.
- Daerah status membawa `role="status"` dan `aria-live="polite"`.
- Tombol coba lagi memancarkan `retry`.

**Verifikasi:** `pnpm test -- infinite-list`, `pnpm list @tanstack/vue-virtual`, gerbang penuh.

### T7. Mode infinite mobile di `assets/index.vue` (jalur kartu)

**Menutup AC:** 11, 12, 13, 14, 15, 16, 17

**Berkas:** `frontend/app/pages/assets/index.vue`

**Yang dikerjakan:**
1. Di lebar compact, paksa tampilan kartu dan render lewat `InfiniteList` dengan
   `AssetCard` di slot `item`.
2. Sumber baris pindah ke `useInfiniteRows` saat compact; jalur desktop tetap memakai
   `load()` berbasis halaman yang ada.
3. Sembunyikan `TablePagination` saat compact.
4. Teruskan elemen `<main>` sebagai `scrollParent`. Ambil lewat `document.querySelector('main')`
   pada `onMounted`, bukan lewat `ref` di layout, supaya halaman tidak perlu mengubah layout.
5. Perubahan filter atau pencarian memanggil `reset()` lalu `loadFirst()` dan menggulirkan
   `scrollParent` ke atas.

**Acceptance criteria tugas:**
- Compact: `TablePagination` tidak ada di DOM; reguler: ada.
- Reguler: menggulir tidak memicu permintaan apa pun.
- Memancarkan `load-more` menambah kartu tanpa mengganti yang sudah ada.
- Skeleton halaman pertama tidak muncul lagi selama pemuatan tambahan.
- Saat `done`, sentinel yang terlihat tidak memicu permintaan lagi.
- Galat pemuatan tambahan mempertahankan kartu yang tampil dan memunculkan tombol coba lagi.
- Mengubah filter mengosongkan akumulasi dan memuat dari offset nol.

**Verifikasi:** tes runtime baru untuk halaman ini, gerbang penuh, verifikasi manual di panel
browser pada viewport 390x844.

### T8. `useListStateCache` dan pemulihan posisi gulir

**Menutup AC:** 23, 24, 25, 26

**Berkas:**
- `frontend/app/composables/useListStateCache.ts` (baru)
- `frontend/app/pages/assets/index.vue`
- `frontend/app/composables/useAuthApi.ts` (pembersihan cache di blok `finally` milik `logout()`,
  bersebelahan dengan `auth.clear()` yang sudah ada)
- `frontend/test/unit/use-list-state-cache.spec.ts` (baru)

**Yang dikerjakan:** sesuai spec bagian 6.5. Map cakupan modul, kunci jalur rute, `signature`
dari nilai filter aktif, pemulihan `scrollTop` pada `nextTick` setelah baris dipulihkan.

**Acceptance criteria tugas:**
- Simpan lalu pulihkan dengan `signature` sama mengembalikan `rows`, `total`, dan `scrollTop`.
- `signature` berbeda tidak memulihkan apa pun dan membuang entri cache.
- Tidak ada pemanggilan `sessionStorage`, `localStorage`, maupun `indexedDB` di composable ini
  (ditegaskan lewat mata-mata di tes).
- Logout mengosongkan seluruh cache.
- Kembali dari detail aset di viewport ponsel memulihkan jumlah kartu dan posisi gulir.

**Verifikasi:** `npx nuxi prepare`, `pnpm test -- use-list-state-cache`, gerbang penuh,
verifikasi manual alur kembali di panel browser.

### T9. Mode infinite di `ResourceTable` (jalur tabel) dan `settings/users.vue`

**Menutup AC:** 11, 21 (untuk Manajemen User)

**Berkas:**
- `frontend/app/components/ResourceTable.vue`
- `frontend/app/pages/settings/users.vue`
- `frontend/i18n/locales/{id,en}.json`
- `frontend/test/nuxt/ResourceTable.spec.ts`

**Yang dikerjakan:**
1. Prop baru `infinite` pada `ResourceTable`. Saat `infinite` dan compact: sembunyikan
   `TablePagination`, render sentinel plus daerah status di bawah tabel.
2. Batas keras `MAX_TABLE_ROWS = 300`. Setelah tercapai, pemuatan otomatis berhenti dan tombol
   `Muat lebih banyak` yang eksplisit muncul.
3. Sambungkan `settings/users.vue` ke `useInfiniteRows` saat compact.
4. Kunci i18n `common.loadMore` dan `common.endOfList`.

**Acceptance criteria tugas:**
- Compact dengan `infinite`: `TablePagination` tidak ada, sentinel ada.
- Reguler: perilaku persis seperti sekarang, sentinel tidak ada.
- Pada 300 baris terakumulasi, `load-more` otomatis berhenti dan tombol muat lebih banyak muncul.
- Menekan tombol itu memuat batch berikutnya.
- Seluruh tes `ResourceTable.spec.ts` yang ada tetap lulus.

**Verifikasi:** gerbang penuh, `e2e/settings.spec.ts` hijau.

### Titik pemeriksaan B

Verifikasi manual di panel browser pada viewport 390x844 untuk kedua halaman pilot: filter
compact, pemuatan berkelanjutan, indikator, akhir daftar, galat plus coba lagi, dan alur kembali
dari detail. Laporkan hasilnya sebelum lanjut ke Fase 4.

---

## Fase 4: Verifikasi dan dokumentasi

### T10. Spesifikasi e2e viewport ponsel

**Menutup AC:** 36

**Berkas:** `frontend/e2e/mobile-table-ux.spec.ts` (baru)

**Yang dikerjakan:** `test.use({ viewport: { width: 390, height: 844 } })` di tingkat berkas.
Spec berjalan di proyek Playwright baru `seeded-ui` (setelah seed demo), bukan `chromium` — lihat
koreksi di spec bagian 9: fase `chromium` berjalan terhadap DB bersih tanpa aset, sedangkan tata
letak compact baru ada setelah daftar melewati satu halaman. Cakupan: filter lanjutan
tersembunyi, slideover membuka dan menerapkan filter, badge terisi, gulir memuat lebih banyak,
kembali dari detail memulihkan posisi.

Mengikuti aturan e2e repo: data unik per jalannya tes, tegaskan setelah pencarian, tunggu modal
tertutup, backend dengan `RATELIMIT_ENABLED=false`.

**Acceptance criteria tugas:**
- Spesifikasi baru hijau.
- Seluruh suite `chromium` yang ada hijau, tidak ada selektor yang rusak.

**Verifikasi:** `pnpm test:e2e:run --project=chromium` dengan backend hidup.

### T11. Gerbang penuh dan perbandingan berdampingan

**Menutup AC:** 35, 37

**Yang dikerjakan:**
1. `pnpm lint`, `vue-tsc`, `pnpm test`, `pnpm build` hijau.
2. Perbandingan 1:1 desktop untuk kedua halaman pilot terhadap mockup, mode terang dan gelap.
3. `grep` ulang seluruh `e2e/` untuk selektor yang mungkin tergeser, sesuai pelajaran dari PR
   penukaran komponen sebelumnya.

### T12. Dokumentasi

**Menutup AC:** 38

**Berkas:**
- `docs/PROGRESS.md`
- Vault `D:\Obsidian\inventra`: catatan sesi di `Catatan/2026-09-03-...`, dan catatan keputusan
  atomik di `Keputusan/Produk/` untuk dua keputusan yang disetujui pemilik produk (windowing
  hanya di jalur kartu; cache daftar tidak menyentuh penyimpanan yang bertahan).

**Acceptance criteria tugas:**
- `docs/PROGRESS.md` menandai pekerjaan ini selesai dan blok "Next session" menunjuk langkah
  nyata berikutnya, yaitu migrasi halaman daftar sisanya.
- Catatan vault dan catatan keputusan tertulis dan terindeks.

### Titik pemeriksaan C

Sajikan ringkasan untuk ditinjau manusia, lalu buka PR.

---

## Yang sengaja tidak dikerjakan

Diulang dari spec bagian 8 supaya tidak hilang dari pandangan saat rencana ini dieksekusi:

1. Migrasi 13+ halaman daftar sisanya ke `FilterBar` dan mode infinite mobile.
2. Keputusan desain apakah `ResourceTable` di mobile sebaiknya merender daftar kartu. Kalau ya,
   batas 300 baris di jalur tabel bisa dicabut dan windowing menjadi seragam.
3. Jalur tersendiri untuk halaman yang memaginasi in-memory (`reports`, `stock-opname`).
