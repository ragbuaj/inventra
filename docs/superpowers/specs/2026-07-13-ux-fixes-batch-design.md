# Design — UX Fixes Batch (7 perbaikan)

**Tanggal:** 2026-07-13
**Status:** Disetujui (menunggu review spec)
**Cakupan:** Satu spec berisi 7 perbaikan independen (5 frontend-only, 2 melibatkan backend). Diikuti satu
rencana implementasi; bagian yang independen dapat dikerjakan paralel.

## Ringkasan

| # | Perbaikan | Area |
|---|-----------|------|
| 1 | Komponen input angka reusable (`NumberInput`) + terapkan ke semua form | Frontend |
| 2 | Tab Profil: state edit + wiring backend + ganti email terverifikasi link | Frontend + Backend |
| 3 | Tab Keamanan: modal ganti password via link email (hapus input inline) | Frontend + Backend |
| 4 | Lupa kata sandi: samakan ukuran input & tombol + fitur resend berjenjang | Frontend |
| 5 | Global search: autofocus programatik saat modal dibuka | Frontend |
| 6 | Bug card detail kantor tertutup peta (z-index) | Frontend |
| 7 | Mojibake pada PDF/Excel/CSV (encoding) | Backend |

Prinsip lintas-perbaikan: ikuti konvensi repo (CLAUDE.md) — komponen `U*`, token tema, i18n `id`/`en`,
test proaktif (unit + runtime + e2e untuk flow), perbandingan 1:1 dengan mockup untuk layar yang tersentuh.

---

## 1. Komponen input angka reusable

### Masalah
Belum ada komponen input angka. Tujuh form memakai `UInput` mentah dengan **tiga pola** format yang
di-copy-paste: (A) `type="number"`; (B) `inputmode="numeric"` + strip `\D`; (C) `toLocaleString('id-ID')`
+ `replace(/\D/g,'')` (diimplementasi ulang ≥3×). CLAUDE.md mewajibkan input angka menolak keystroke
non-numerik (bukan sekadar validasi submit) dan mengekspos prop kontrol negatif.

### Desain
Komponen baru **`frontend/app/components/NumberInput.vue`** yang membungkus `UInput` dan dirancang dipakai
**di dalam** `UFormField` (tidak menggantinya), meneruskan slot & `data-testid`.

**Props (interface publik):**
- `modelValue: string` — **nilai mentah** (digit-string, mis. `"1000000"`, `"-6.2"`), lewat `defineModel`.
- `allowNegative?: boolean` — default `false`. Bila `false`, tanda minus ditolak.
- `thousandSeparator?: boolean` — default `false`. **Ini "format limiter"**: menampilkan pemisah ribuan
  id-ID (`1.000.000`) sementara `modelValue` tetap mentah (`1000000`).
- `decimals?: number` — default `0` (bilangan bulat). `>0` mengizinkan titik/koma desimal hingga N digit
  (dibutuhkan `offices` lat/lng). Saat `decimals>0`, `thousandSeparator` hanya memformat bagian integer.
- `money?: boolean` — default `false`. Bila `true`, render ikon **`Rp`** via slot `#leading` dan
  mengaktifkan `thousandSeparator` secara implisit (kecuali di-override).
- Diteruskan apa adanya: `min`, `max`, `placeholder`, `disabled`, `id`, `data-testid`, `ui`.

**Perilaku:**
- **Numeric-only pada keystroke**: handler `@beforeinput`/`@keydown`/`@paste` menolak karakter non-numerik;
  minus hanya diperbolehkan di posisi awal saat `allowNegative`; pemisah desimal hanya bila `decimals>0`.
- Menyimpan nilai mentah ke `modelValue`; tampilan diformat lewat computed display/parse.
- Sumber kebenaran format tunggal: pindahkan `formatThousands`/`parseThousands` dari
  `constants/categoryMeta.ts` ke **`utils/format.ts`** (auto-import), lalu `categoryMeta.ts` re-export
  untuk kompatibilitas. `NumberInput` memanggil helper ini.

**Penerapan (ganti pemakaian `UInput` angka):**

| File | Field | Konfigurasi |
|------|-------|-------------|
| `components/asset/AssetForm.vue` | harga (mode new) | `money` |
| `components/category/CategoryFormSlideover.vue` | useful_life, fiscal_life (bln), salvage_rate (%) | `thousandSeparator=false`, slot `#trailing` unit; salvage `max=100` |
| `components/category/CategoryFormSlideover.vue` | capitalization_threshold | `money` |
| `components/maintenance/RecordSlideover.vue` | cost | `money` |
| `components/maintenance/ScheduleSlideover.vue` | intervalMonths | `min=1` |
| `pages/disposals.vue` | proceeds | `money` |
| `pages/depreciation.vue` | impairRecoverable | `money` |
| `pages/master/offices.vue` | latitude, longitude | `decimals=7`, `allowNegative`, `thousandSeparator=false` |

Semua pola manual (`onProceedsInput`, `onImpairRecoverInput`, `formatThousands` inline, getter/setter
`toCoord`, dll.) dihapus dan digantikan komponen.

### Testing
Unit test `NumberInput`: strip non-numerik, tolak/terima minus per `allowNegative`, format ribuan,
desimal (batas `decimals`), mode `money` (ikon Rp + nilai mentah), paste kotor, `min`/`max`. Runtime mount
test (`// @vitest-environment nuxt`) memverifikasi `v-model` mentah & tampilan terformat, plus emit.
Sesuaikan test form yang terdampak (assert testid/nilai tetap lulus).

---

## 2. Tab Profil — edit state + wiring backend + ganti email terverifikasi

### Masalah
`account.vue` tab Profil sudah punya field editable (`nama`, `telepon`) tetapi `getProfile`/`updateProfile`
di `useAccount.ts` **masih mock**. Email read-only tanpa mekanisme ganti. Backend **tidak punya** endpoint
update profil maupun ganti email; infra token/email hanya untuk reset password.

### Keputusan desain
- **Telepon disimpan di `masterdata.employees.phone`** (bukan `users`). `identity.users.employee_id`
  nullable → user tanpa employee tak bisa mengedit telepon.
- **Ganti email**: link verifikasi dikirim ke **email BARU**, dan **wajib** isi password saat ini (standar
  industri, aman dari pembajakan sesi).

### Backend

**Migrasi** (nomor sekuensial berikutnya, cek folder saat implementasi):
```sql
ALTER TABLE masterdata.employees ADD COLUMN phone text;
```
`.down.sql`: `ALTER TABLE masterdata.employees DROP COLUMN phone;`

**Token store** — file baru `internal/auth/emailchange.go` (mirror `pwreset.go`):
- Prefix Redis `auth:emailchange:`.
- `GenerateEmailChangeToken()` (reuse pola rand+SHA-256).
- `SaveEmailChange(ctx, hash, userID, newEmail, ttl)` — simpan JSON `{userID, newEmail}`.
- `ConsumeEmailChange(ctx, hash)` → `(userID, newEmail, error)` single-use (`GetDel`).

**Endpoint** (`internal/identity/{routes,handler,service,dto}.go`):

| Method | Path | Auth | Body | Perilaku |
|--------|------|------|------|----------|
| `GET` | `/api/v1/auth/profile` | protected | — | Profil lengkap: name, email, phone (join employee), role, office, employee, login_method, join_date. *(Alternatif: perkaya `/auth/me`; endpoint terpisah lebih rapi untuk data profil.)* |
| `PUT` | `/api/v1/auth/profile` | protected | `{name, phone}` | Update `users.name`; jika `employee_id` ada → update `employees.phone` (dalam satu transaksi). Abaikan `phone` bila tak ada employee. |
| `POST` | `/api/v1/auth/email/change-request` | protected | `{new_email, current_password}` | Verifikasi password (`argon2`). Validasi email (format, ≠ email sekarang, belum dipakai user lain → 409). Generate token, `SaveEmailChange`, kirim link ke **email baru**: `frontendURL + "/verify-email?token=" + raw`. 200. |
| `POST` | `/api/v1/auth/email/confirm` | **public**, rate-limited | `{token}` | `ConsumeEmailChange` → jika email baru masih belum dipakai → update `users.email`. Kirim notifikasi ke email lama. 200/400 (token invalid/expired). Publik karena user bisa klik link dari device lain. |

DTO baru: `updateProfileRequest`, `emailChangeRequest`, `emailConfirmRequest`, response profil. Sentinel
errors + `svcError` mapping mengikuti pola modul. Semua endpoint pakai `RequirePermission`? — ini
self-service akun sendiri (bukan resource lain), jadi cukup `RequireAuth` (mengikuti pola `/auth/password`
& `/auth/me` yang tak pakai `RequirePermission`).

**Email** (`internal/email/`):
- Template baru `templates/email_change_verify.{html,txt}` (ke email baru; berisi link + nama) dan
  `templates/email_changed.{html,txt}` (notifikasi ke email lama; berisi email baru).
- `Mailer.SendEmailChangeVerify(ctx, to, name, link)` + `SendEmailChanged(ctx, to, name, newEmail)`.
- Dispatch via `AsyncMailer` (konsisten dengan reset password).

**OpenAPI** `backend/api/openapi.yaml`: tambah keempat path + skema; lolos Spectral.

### Frontend
- `useAccount.ts`: ganti mock —
  - `getProfile()` → `GET /auth/profile`.
  - `updateProfile({name, phone})` → `PUT /auth/profile`.
  - `requestEmailChange({newEmail, currentPassword})` → `POST /auth/email/change-request`.
  - `confirmEmailChange(token)` → `POST /auth/email/confirm`.
- `account.vue` tab Profil:
  - **Mode view↔edit**: default read-only; tombol **"Edit"** mengaktifkan field `nama`+`telepon`; tombol
    **"Simpan"** (loading) & **"Batal"** (revert dari snapshot). Validasi nama wajib tetap.
  - Telepon **disabled + hint** bila user tak punya employee terkait.
  - **Email**: tetap read-only di card; tambah aksi **"Ubah Email"** membuka **modal** (`FormModal`): input
    email baru (validasi format) + password saat ini → `requestEmailChange` → tampilkan state "Tautan
    verifikasi dikirim ke <email baru>. Klik untuk konfirmasi." + resend cooldown (composable #4).
  - Akun Google: email tetap terkunci (note gembok) — tak bisa ganti email.
- Halaman baru **`pages/verify-email.vue`** (layout `auth`): baca `?token=`, panggil `confirmEmailChange`,
  tampilkan sukses/gagal + tombol ke login/akun. Bila user login, refresh profil.
- i18n: semua string baru di `i18n/locales/{id,en}.json`.

### Testing
- Backend: unit test service (verifikasi password gagal → error; email bentrok → conflict; token
  consume; update phone hanya bila employee ada). Integration jika pola ada.
- Frontend: runtime test tab Profil (toggle edit, disabled telepon tanpa employee, modal ubah email),
  `verify-email.vue`; e2e alur ganti email end-to-end (request → ambil token → confirm) mengikuti pola
  e2e password-reset yang ada.

---

## 3. Tab Keamanan — modal ganti password via link email

### Masalah
Tab Keamanan menampilkan input ganti-password **inline** (old/new/confirm) yang langsung memanggil
`PUT /auth/password`. Diminta: sembunyikan input inline; tombol "Ganti Password" → modal minta password
lama sebagai verifikasi → kirim **link penggantian** via email.

### Backend
Endpoint baru (protected), reuse total infra reset password yang sudah ada:

| Method | Path | Body | Perilaku |
|--------|------|------|----------|
| `POST` | `/api/v1/auth/password/change-request` | `{current_password}` | Verifikasi password lama (argon2). Jika benar → `GenerateResetToken` + `SavePasswordReset` (TTL sama) + `SendPasswordReset` ke email user. 200 (juga 200-generic bila password salah? tidak — ini authenticated, kembalikan 401 bila password salah agar UX jelas). |

Endpoint lama `PUT /auth/password` **dibiarkan ada** (masih valid di API/tes lain), tetapi frontend tak
lagi memakainya untuk self-service. OpenAPI ditambah path baru.

### Frontend
- `account.vue` tab Keamanan: **hapus** ketiga input inline + strength meter dari alur utama. Tampilkan
  card dengan penjelasan + tombol **"Ganti Password"**.
- Klik → **modal** (`FormModal`): satu input **password lama** (type password) → submit →
  `account.requestPasswordChange({currentPassword})` → `POST /auth/password/change-request`.
  - Sukses → state "Tautan penggantian password dikirim ke email Anda." + tombol **resend** (cooldown #4).
  - Password salah (401) → pesan error di modal.
- `useAccount.ts`: `changePassword` inline lama diganti `requestPasswordChange({currentPassword})`. (Fungsi
  lama boleh dihapus bila tak dipakai.)
- Card Sessions (mock) dibiarkan apa adanya (di luar cakupan).
- i18n string baru.

### Testing
Backend: unit test (password benar → token tersimpan + email terkirim; salah → 401). Frontend: runtime
test modal (buka, submit, error 401, state terkirim + resend disabled saat cooldown). E2E: perbarui
spec change-password yang ada agar mengikuti alur baru (klik tombol → modal → password lama → verifikasi
email terkirim); alur set password baru tetap lewat halaman `reset-password`.

---

## 4. Lupa kata sandi — ukuran input & resend berjenjang

### Masalah
`forgot-password.vue`: `UInput` email tak dipaksa full-width sementara tombol pakai `block` → lebar beda.
Belum ada fitur resend.

### Desain
- **Ukuran**: beri `class="w-full"` pada `UInput` email (setara `block` tombol) di dalam `max-w-sm`.
- **Resend berjenjang** — composable baru **`composables/useResendCooldown.ts`**:
  - `useResendCooldown(baseSeconds = 30)` → `{ remaining, canResend, attempts, start(), reset() }`.
  - Cooldown **eksponensial**: percobaan ke-n menunggu `base * 2^(n-1)` detik → **30s, 60s, 120s, …**.
  - Timer via `setInterval`, dibersihkan `onScopeDispose`/`onUnmounted`.
  - Reusable: dipakai juga di modal Ubah Email (#2) dan modal Ganti Password (#3).
- `forgot-password.vue`: setelah `sent=true`, tampilkan `UAlert` sukses + tombol **"Kirim ulang tautan"**
  yang `:disabled="!canResend"` dan menampilkan hitung mundur (`{{ remaining }}s`). Klik memanggil ulang
  `submit()` lalu `start()`. Hormati 429 backend (pesan rate-limited tetap ada).

### Testing
Unit test `useResendCooldown` (increment eksponensial, canResend transisi, reset). Runtime test
`forgot-password.vue` (input full-width via kelas/atribut, tombol resend muncul setelah sent, disabled saat
cooldown). Perbarui e2e password-reset bila perlu.

---

## 5. Global search — autofocus programatik

### Masalah
`CommandPalette.vue` mengandalkan atribut HTML `autofocus` (rapuh dengan Teleport/`v-if`), kadang input
tak fokus saat modal dibuka.

### Desain
- Tambah `const inputEl = ref<HTMLInputElement>()` + `ref="inputEl"` pada input.
- `watch(isOpen, (v) => { if (v) nextTick(() => inputEl.value?.focus()) })` (immediate tak perlu; panel
  di-mount saat open). Pertahankan `autofocus` sebagai fallback.

### Testing
Runtime test: buka palette → input ter-fokus (`document.activeElement` / `toBe(inputEl)`); memperluas
`CommandPalette.spec.ts` yang ada.

---

## 6. Card detail kantor tertutup peta

### Masalah
`pages/master/map.vue`: card detail (`data-testid="office-detail-card"`) **tanpa z-index**, sedangkan pane
& kontrol Leaflet ber-z-index 400–1000 → card (dan kontrol custom) bisa tertutup peta.

### Desain
Beri z-index eksplisit di atas lapisan Leaflet:
- Card detail → `z-[1000]` (atau lebih tinggi dari card kontrol lain, mis. `z-[1100]`).
- Konsistenkan: zoom controls, reset view, empty overlay → `z-[1000]` agar selalu di atas peta.
- Pastikan tetap di bawah komponen global (modal command palette `z-60` di-`Teleport` ke body, terpisah;
  tak bentrok karena beda konteks stacking). Verifikasi tak menutupi elemen lain yang seharusnya di atas.

### Testing
Runtime test `master-map.spec.ts`: card detail memiliki kelas z-index saat `selected`. Verifikasi visual
manual (card tampil penuh di atas peta) + perbandingan dengan mockup.

---

## 7. Mojibake PDF/Excel/CSV

### Masalah
Semua PDF di-generate `go-pdf/fpdf` dengan **core font Helvetica** (encoding cp1252) padahal string Go
UTF-8 → `·` (U+00B7) dirender `Â·`, `—` (U+2014) dirender `â€"`. Titik pasti string literal:
`depreciation/export.go:146,174`, `report/export.go:289,338,491,633`. Data dinamis dari DB (nama beraksen,
en/em-dash yang diketik user) juga rusak. CSV importer ditulis UTF-8 **tanpa BOM** → Excel (locale Windows)
salah baca.

### Desain

**PDF — embed font Unicode:**
- Tambahkan aset font **DejaVuSans** (`DejaVuSans.ttf`, `DejaVuSans-Bold.ttf`, `DejaVuSans-Oblique.ttf`)
  ke repo, mis. `backend/internal/report/fonts/` (atau paket `internal/pdfutil`). Lisensi DejaVu (Bitstream
  Vera / public-domain-ish) aman untuk didistribusikan — cantumkan berkas lisensi.
- Buat helper bersama (mis. `internal/pdfutil/font.go`) yang meng-`AddUTF8Font("dejavu", "", regularPath)`,
  `AddUTF8Font("dejavu", "B", boldPath)`, `AddUTF8Font("dejavu", "I", italicPath)` pada sebuah `*fpdf.Fpdf`,
  memakai `SetFontLocation` atau byte via `AddUTF8FontFromBytes` (embed dengan `//go:embed`) agar tak
  bergantung path filesystem saat deploy. **Preferensi: `//go:embed` bytes + `AddUTF8FontFromBytes`.**
- Ganti semua `pdf.SetFont("Helvetica", …)` → `pdf.SetFont("dejavu", …)` di keempat file
  (`report/export.go`, `depreciation/export.go`, `stockopname/report.go`, `asset/barcode.go`) via helper
  konstruktor PDF bersama sehingga font terpasang sekali.
- Efek: `·`, `—`, dan seluruh data dinamis non-ASCII tampil benar.

**CSV — tambah BOM UTF-8:**
- `importer/template.go` dan `importer/errreport.go`: tulis prefiks `\xEF\xBB\xBF` sebelum konten CSV
  sehingga Excel membuka sebagai UTF-8. (XLSX via excelize tidak diubah — sudah benar.)

### Testing
- Backend unit test: render PDF ke buffer, assert font terdaftar / tak panic; **decode** teks PDF atau
  minimal pastikan pipeline tak error dengan input berisi `·`, `—`, dan string beraksen. Untuk CSV, assert
  output diawali BOM `EF BB BF`.
- Verifikasi manual: generate satu laporan PDF & satu CSV, buka, konfirmasi tak ada `Â·`/`â€"`.

---

## Urutan & paralelisasi implementasi

- **Independen (frontend-only), paralel:** #1, #4, #5, #6.
- **Backend + frontend berpasangan:** #2 (profil/email), #3 (ganti password) — berbagi infra email/token,
  kerjakan berurutan atau satu track.
- **Backend-only:** #7 (PDF/CSV).

## Verifikasi akhir (gates CI)
- Backend: `go build ./...`, `go vet ./...`, `go test ./...`, Spectral lint.
- Frontend: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`; e2e untuk flow (butuh stack + admin).
- Perbandingan 1:1 dengan mockup untuk `account`, `forgot-password`, `master/map`.
- Update `docs/PROGRESS.md` saat landing.
