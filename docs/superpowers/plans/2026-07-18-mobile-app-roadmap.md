# Rencana Pengembangan Mobile App Inventra — Field Companion (Flutter)

Tanggal: 2026-07-18. Status: disetujui pemilik produk (2026-07-18); dokumen pendukung
(PRD v1.2, ADR-0015, ADR-0016, PROGRESS, vault) dilengkapi di branch `feat/mobile-docs`.

## 1. Konteks dan keputusan scope

PRD v1.1 (bagian out-of-scope, baris "Aplikasi mobile native") semula **mengecualikan** aplikasi
mobile karena web responsif dianggap cukup. Pada 2026-07-18 pemilik produk memutuskan membuka
scope ini. Tiga keputusan arah sudah diambil:

1. **Scope v1: field companion** — fokus petugas lapangan dan pejabat pemutus, bukan paritas
   penuh dengan web. Layar admin (master data, RBAC, laporan penuh) tetap di web.
2. **Teknologi: Flutter** — satu codebase Android + iOS; selaras praktik perbankan Indonesia
   (BTN Mobile, BCA, BRI memakai Flutter); ekosistem scanner kamera dan offline storage matang.
3. **Offline-first untuk stock opname** — snapshot sesi diunduh ke device, scan tercatat lokal,
   sinkron saat online kembali. Sesuai realita lapangan (gudang, basement, lokasi ATM tanpa sinyal).

Tindak lanjut dokumen (selesai 2026-07-18, branch `feat/mobile-docs`): **dokumentasi mobile
dipisah ke `docs/mobile/`** — **PRD mobile** (`docs/mobile/PRD.md`, FR-M1 sampai FR-M6),
**ADR-0015** (Flutter) + **ADR-0016** (offline sync) di `docs/mobile/adr/` (penomoran global),
**design brief + prompt kit mockup** (`docs/mobile/DESIGN_BRIEF.md`); amendemen **PRD web v1.2**
(non-goal dicabut, bagian 3.11 jadi penunjuk, tahap 11 roadmap); entri PROGRESS.md bagian Mobile;
catatan keputusan produk di vault Obsidian (`Keputusan/Produk/`).

## 2. Persona dan use case v1

| Persona | Use case mobile |
|---|---|
| Petugas opname / GA cabang | Scan QR aset dengan kamera, catat hasil opname (termasuk offline), lihat variance |
| Pejabat pemutus (checker) | Inbox pengajuan, lihat detail, approve/reject on-the-go (maker-checker + SoD tetap berlaku) |
| Semua pengguna | Login, lookup detail aset via scan/katalog, ajukan peminjaman/lapor kerusakan/registrasi aset per permission (FR-M7), notifikasi push + in-app, profil dan sesi device |

Non-scope v1 (tetap di web): CRUD master data, RBAC/data scope/field permission admin, laporan dan
dashboard penuh, penyusutan, import massal, manajemen user, **mutasi, disposal** (pengajuan kedua
modul ini tetap dibuat dari web; mobile hanya memutus approval-nya).

Diperluas 2026-07-21 (keputusan produk, PRD mobile v1.1 FR-M7): **katalog aset browse read-only,
peminjaman/check-out dari detail, lapor kerusakan/maintenance, registrasi aset (form penuh), plus
self-scoped read "Pengajuan saya" (lensa maker) dan "Aset saya" (yang dipegang pengguna) masuk
mobile** — semuanya ringan/ter-anchor ke alur scan dan memakai endpoint web yang ada. Mutasi dan
disposal tetap di web.

Diperluas juga 2026-07-21 (keputusan produk kedua, PRD mobile v1.1 FR-M6/M1.5): **profil lengkap +
ubah data diri + avatar, keamanan akun (ganti password/email berbasis link), dan lupa password di
login** masuk mobile — semua endpoint sudah ada (nol backend baru); penetapan password/email
diselesaikan di web (tanpa deep-link v1). Lihat fase M8.

## 3. Kesiapan backend saat ini

Sudah siap dipakai klien mobile tanpa perubahan:

- API JSON `/api/v1` dengan JWT access token + 3-layer authorization (permission, data scope,
  field permission) — semua enforcement di server, klien mobile tinggal konsumsi.
- `GET /assets/by-tag/:tag` — lookup hasil scan QR (dipakai juga oleh label barcode web).
- Modul approval (`/requests`): inbox, detail, approve/reject, SoD maker tidak sama dengan checker.
- Stock opname online: sesi, `POST /stock-opname/sessions/:id/scan` (lookup by tag, auto-add
  out-of-snapshot), variance, berita acara.
- Device sessions (2026-07-15) — sesi login per device sudah tercatat (UA/IP), tinggal tampil.
- Notifikasi in-app (2026-07-17) — list + unread count.

Kesenjangan backend yang harus dibangun (mengikuti urutan standar modul: migration, queries,
sqlc generate, handler + RegisterRoutes, authz eksplisit per endpoint, wiring NewRouter, OpenAPI):

1. **Push notification (FCM)** — modul notifikasi belum punya kanal push. Butuh: tabel
   `identity.device_tokens` (user_id, token, platform, soft delete, unik parsial per token),
   endpoint register/unregister token, dispatcher FCM dari event notifikasi yang sudah ada,
   env kredensial FCM (daftarkan di docker-compose.prod.yml — pelajaran kasus env Resend).
2. **Refresh token untuk mobile** — saat ini refresh hanya via cookie httpOnly dan respons token
   sengaja tidak menyertakan `refresh_token`. Diputuskan lewat
   [ADR-0017](../../mobile/adr/0017-mobile-client-identity.md): klien mobile mengirim penanda
   `X-Client-Type: mobile` saat login; server men-stamp klaim `aud` (`web`/`mobile`) ke access
   dan refresh token, dan untuk `aud=mobile` refresh token dikembalikan di body response
   (tanpa cookie), disimpan klien di `flutter_secure_storage` (Keystore/Keychain); endpoint
   refresh menerima token dari body. Jalur web tidak berubah. (Rencana awal "Opsi A" cookie jar
   `dio_cookie_manager` digantikan keputusan ini sebelum implementasi M1.)
3. **Sync opname offline** — endpoint batch idempoten, misal
   `POST /stock-opname/sessions/:id/scans/batch`: setiap item membawa `client_scan_id` (UUID dari
   device) untuk dedup, `scanned_at` timestamp lokal; respons melaporkan per-item sukses/konflik.
   Kebijakan konflik v1: first-write-wins per aset per sesi (scan pertama yang tiba menang; scan
   duplikat dari device lain ditandai konflik, tidak menimpa), konflik dikembalikan ke device untuk
   ditampilkan. Snapshot download memakai endpoint items yang ada; evaluasi kebutuhan pagination
   besar atau ETag bila sesi ribuan item.
4. **Rate limit + OpenAPI** untuk semua endpoint baru.

## 4. Arsitektur aplikasi Flutter

- **Lokasi repo**: folder `mobile/` di monorepo ini (konsisten dengan `backend/` dan `frontend/`).
- **Stack**: Flutter stabil terbaru (Dart 3), target Android dulu (device operasional bank umumnya
  Android); proyek disiapkan agar build iOS tinggal diaktifkan.
- **State management**: Riverpod (compile-safe, testable, standar industri saat ini).
- **Networking**: Dio + interceptor auth (attach access token, auto-refresh saat 401, logout saat
  refresh gagal); model via `freezed` + `json_serializable` (codegen, selaras DTO OpenAPI).
- **Offline storage**: `drift` (SQLite) — tabel snapshot sesi opname, antrean scan lokal, status
  sync per item. Data lain (aset, approval) cukup cache memori; tidak offline di v1.
- **Scanner**: `mobile_scanner` (kamera QR/barcode) dengan fallback input tag manual — meniru pola
  scan bar web yang sudah ada.
- **Keamanan klien**: token di `flutter_secure_storage`; build release dengan `--obfuscate`;
  data opname lokal dihapus setelah sesi selesai tersinkron; tidak menyimpan kredensial.
- **i18n**: id (default) + en via `intl`/ARB — paritas kunci dengan `i18n/locales/{id,en}.json` web
  untuk istilah domain (opname, mutasi, BAST, dsb.).
- **Tema**: design tokens Inventra (primary green, neutral slate, light + dark) diterjemahkan ke
  `ThemeData`; komponen mengikuti mockup mobile (lihat bagian 5).

## 5. Desain UI — mockup dulu (konvensi proyek)

Konvensi "design fidelity is mandatory" berlaku juga untuk mobile: **sebelum membangun layar,
mockup mobile harus dibuat dulu**. Prompt kit lengkapnya sudah tersedia di
`docs/mobile/DESIGN_BRIEF.md` (master brief + component library + 12 prompt per-layar, siap
di-generate di Claude design); hasilnya disimpan di `docs/mobile/design/`. Daftar layar v1:

Login (+ Lupa Password, FR-M1.5); Home (ringkasan tugas: opname aktif, approval menunggu,
notifikasi); Scan (kamera full screen + input manual); Katalog Aset (daftar + cari/filter,
read-only, FR-M7.1); Detail Aset (hormati field permission; aksi peminjaman/check-out, lapor
kerusakan, registrasi per permission, FR-M7); Form Registrasi Aset (form penuh, FR-M7.4); Pengajuan
Saya (lensa maker + batal pending, FR-M7.5); Aset Saya (menu tersendiri, FR-M7.6); Daftar Sesi
Opname; Opname Counting (scan bar, progress, daftar item, indikator offline/antrean sync); Variance;
Approval Inbox; Approval Detail (approve/reject + catatan); Notifikasi; Profil (lengkap + ubah data
diri + avatar, FR-M6.1/6.2); Keamanan Akun (ganti password/email berbasis link, FR-M6.3); Sesi
Device (FR-M6.4); Pengaturan (bahasa, tema, FR-M6.5).

## 6. Fase implementasi

Setiap fase mendapat spec + implementation plan sendiri (konvensi `docs/superpowers/`), branch
`feat/mobile-*`, dan bergerak satu irisan vertikal per commit. Estimasi dalam "sesi kerja"
(satu sesi kira-kira setara satu batch fitur seperti riwayat repo ini).

| Fase | Isi | Prasyarat | Estimasi |
|---|---|---|---|
| M0 Fondasi | Scaffold `mobile/`, tema + i18n, navigasi shell, login/refresh/logout (jalur refresh per-klien ADR-0017: `X-Client-Type: mobile`, refresh token di body), secure storage, CI job Flutter (analyze, test, build APK), amendemen PRD + ADR Flutter | Mockup Login + shell | 2-3 sesi |
| M1 Scan aset | Kamera scan, `GET /assets/by-tag/:tag`, Detail Aset read-only dengan field-permission masking | M0, mockup Scan + Detail | 1-2 sesi |
| M2 Approval | Inbox `/requests`, detail, approve/reject, guard SoD dan permission via API | M0, mockup Approval | 1-2 sesi |
| M3 Push notification | Backend FCM (tabel device_tokens, register, dispatcher) + layar Notifikasi mobile + deep-link ke approval/opname | M0; backend gap no. 1 | 2 sesi |
| M4 Opname online | Sesi opname, counting dengan scan kamera, variance — semua online via endpoint yang ada | M1 | 2 sesi |
| M5 Opname offline | Drift snapshot + antrean lokal, endpoint batch sync idempoten di backend, resolusi konflik first-write-wins, indikator status sync | M4; backend gap no. 3 | 3-4 sesi |
| M6 Rilis | Icon/splash, signing, distribusi internal (Firebase App Distribution atau APK internal; Play Store internal track menyusul), Crashlytics/Sentry, runbook rilis di vault | M1-M5 sesuai target rilis | 1-2 sesi |
| M7 Katalog & aksi aset | Katalog aset browse read-only (cari/filter, pagination server); aksi di Detail Aset per permission x status: peminjaman/check-out/check-in, lapor kerusakan/maintenance, registrasi aset (form penuh); **Pengajuan saya** (lensa maker atas `/requests?requested_by=diri` + batal `pending` sendiri) dan **Aset saya** (`/assignments/mine`, menu tersendiri) — semua via endpoint web yang ada, pengajuan lewat maker-checker/SoD server (FR-M7) | M1 (scan + Detail Aset) | 2-3 sesi |
| M8 Profil & keamanan akun | Profil lengkap (metadata + detail pegawai read-only) + ubah data diri + avatar (`/auth/profile`, `/auth/avatar`); Keamanan Akun ganti password/email berbasis link email; Lupa Password di Login (`/auth/password/forgot`); Sesi Device (list/revoke, sudah dirender v1) — semua endpoint sudah ada, nol backend baru; penetapan password/email diselesaikan di web (tanpa deep-link v1); ganti password cabut semua sesi lalu logout (FR-M1.5, FR-M6) | M0 (auth) | 1-2 sesi |

M1, M2, M3 saling independen setelah M0 — bisa diparalelkan atau diurutkan sesuai prioritas. M7
bergantung pada M1 (memakai layar Detail Aset); M8 hanya bergantung M0 (jalur auth/profil). Rilis
internal pertama yang bermakna: M0 + M1 + M2 (scan + approval), menyusul opname; M7 dan M8 menyusul
sesuai prioritas.

## 7. Testing dan CI

- **Unit + widget test** (`flutter test`) untuk logika (formatter, sync queue, auth interceptor)
  dan widget kunci — cakupan luas termasuk empty/error/loading, input tidak valid, variasi
  permission (konvensi proyek: proaktif dan ekspansif).
- **Integration test** (`integration_test/`) melawan backend compose + seeded admin, meniru pola
  e2e Playwright yang ada (data unik per run, rate limit off lokal).
- **Golden test** untuk layar utama light + dark.
- **CI**: job baru di `.github/workflows/ci.yml` — `flutter analyze`, `flutter test`, build APK
  debug; integration test di job terpisah yang menaikkan docker-compose backend (pola job e2e
  yang ada).

## 8. Risiko dan mitigasi

| Risiko | Mitigasi |
|---|---|
| Konflik sync opname multi-device | Idempotensi `client_scan_id`, first-write-wins, konflik dilaporkan eksplisit ke device; integration test skenario dua device |
| Penyimpanan refresh token rapuh di beberapa OEM Android | Refresh token disimpan di `flutter_secure_storage` (Keystore/Keychain) sesuai ADR-0017; access token hanya di memori |
| Stack baru (Flutter/Dart) di samping Go + Vue | Mulai dari M0 kecil; source-driven (dokumentasi resmi Flutter/Riverpod/drift); golden test menjaga regresi UI |
| Scope creep menuju paritas web | Bagian non-scope v1 eksplisit; penambahan modul mobile baru wajib keputusan produk tercatat |
| Kamera murah gagal baca label pudar | Fallback input tag manual di semua titik scan (pola yang sudah ada di web) |
| Env produksi baru (FCM) tidak terbaca | Daftarkan di docker-compose.prod.yml saat fase M3 (pelajaran kasus Resend) |

## 9. Definisi selesai per fase

Setiap fase dianggap landed bila: semua gate hijau (backend: build/vet/test/integration + Spectral;
mobile: analyze/test/build; frontend bila tersentuh: lint/typecheck/test/build), OpenAPI sinkron,
PROGRESS.md dicentang dengan nomor PR, vault Obsidian diperbarui (Status & Roadmap, Peta Modul,
catatan sesi), dan perbandingan 1:1 layar terhadap mockup mobile-nya dilaporkan.
