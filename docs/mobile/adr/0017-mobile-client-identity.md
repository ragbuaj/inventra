# ADR-0017 — Identitas klien mobile: backend bersama, klaim audience, dan kontrak endpoint

- Status: Accepted
- Date: 2026-07-19
- Deciders: pemilik proyek + sesi audit keamanan mobile (2026-07-19)
- Terkait: [ADR-0015](0015-mobile-companion-flutter.md) — men-supersede butir "Auth"
  (cookie jar persisten) dengan jalur refresh per-klien; [ADR-0016](0016-stock-opname-offline-sync.md)
  — menambah persyaratan keamanan endpoint batch sync; ADR-0004 (rate limiting) dan ADR-0014
  (notification delivery) — diperluas untuk klien mobile.

## Konteks

ADR-0015 memutuskan mobile companion sebagai konsumen `/api/v1` yang sama dengan web. Sebelum
implementasi M1 dimulai, muncul pertanyaan apakah keamanan menuntut backend terpisah. Audit
keamanan atas keputusan arsitektur itu (2026-07-19) menyimpulkan arsitekturnya benar, tetapi
menemukan satu kerentanan struktural dan beberapa keputusan yang harus dikunci sebelum kode
mobile ditulis:

1. **Token web dan mobile identik dan saling tertukar.** JWT tidak membawa klaim `aud`, dan
   `Parse` tidak memvalidasi issuer maupun audience (`backend/internal/auth/jwt.go`). Kompromi
   satu APK di device yang di-root memberi akses ke seluruh API web (termasuk `authzadmin`,
   importer) — blast radius satu akun web penuh, bukan subset mobile.
2. **Refresh token hanya ada di cookie httpOnly** (`backend/internal/identity/cookie.go`,
   `dto.go`) — tidak ada jalur untuk klien non-browser. Rencana awal ADR-0015 (cookie jar
   persisten di Flutter) menyimpan token di penyimpanan HTTP client, bukan Keystore/Keychain,
   dan rapuh di sebagian OEM Android.
3. **Belum ada aturan** bagaimana sebuah endpoint diklasifikasikan web/mobile, dan bagaimana
   response endpoint bersama boleh berbeda antar klien.

## Keputusan

### 1. Tetap satu backend — tidak ada backend baru dan tidak ada BFF terpisah

Otorisasi di backend ini hidup di lapisan service, bukan di router: handler memanggil
`CallerOfficeScope`, service menegakkan permission, data scope, dan field permission
(`backend/internal/authz/scope.go`, pola di `backend/internal/stockopname/handler.go`). Klien
mobile yang memanggil `/api/v1` yang sama mewarisi seluruh RBAC, scope, session revocation, dan
audit trail tanpa satu baris logika otorisasi baru. Satu-satunya kerentanan struktural dari
backend bersama (token lintas-klien, butir Konteks 1) ditutup oleh keputusan 2 dan 4.

### 2. Identitas klien dibawa sebagai klaim `aud` di JWT

- Login membawa penanda klien (header `X-Client-Type` atau field `client` di body). Server
  men-stamp klaim `aud` (`"web"` / `"mobile"`) ke access dan refresh token; `Parse` memvalidasi
  issuer (`jwt.WithIssuer`) dan audience (`jwt.WithAudience`).
- Refresh token mewarisi `aud` yang sama — klien tidak bisa berganti identitas tanpa login ulang.
- Penanda login memang bisa dipalsukan, dan itu aman by design: mengaku `mobile` menghasilkan
  token dengan akses lebih sempit, bukan lebih luas. Berbohong tidak menguntungkan.
- User-Agent tidak pernah dipakai untuk keputusan otorisasi — hanya untuk display di daftar sesi.

### 3. Jalur refresh token per-klien (men-supersede butir "Auth" ADR-0015)

- **Web: tidak berubah.** Refresh token tetap cookie httpOnly, `SameSite=Lax`,
  `Path=/api/v1/auth`; body response tidak pernah menyerialisasi `refresh_token`.
- **Mobile: refresh token dikembalikan di body** response login/refresh, tanpa set cookie sama
  sekali; endpoint refresh menerima token dari body untuk `aud=mobile`. Klien menyimpannya di
  `flutter_secure_storage` (Keystore/Keychain) — `dio_cookie_manager` tidak dipakai.
- Test invariant `handler_cookie_test.go` direvisi menjadi per-klien: response untuk klien web
  tetap tidak boleh mengandung `refresh_token`.
- `JWT_ACCESS_TTL` tetap 15 menit. Access token membawa `role_id`, sehingga memperpanjang TTL
  berarti menunda pencabutan hak setelah demosi role — kenyamanan mobile dibeli lewat refresh
  yang benar, bukan token berumur panjang.

### 4. Klasifikasi endpoint: tiga kategori, default shared, ditegakkan per-rute

| Kategori | Contoh | Aturan |
|---|---|---|
| Shared (default) | assets, requests, notifikasi, profil, opname online | Tanpa middleware audience — web dan mobile sama-sama boleh |
| Mobile-only | registrasi FCM device token (M3), batch sync opname (M5) | `RequireAudience("mobile")` |
| Web-only / admin berisiko | `authzadmin`, importer, ekspor laporan | Deny `aud=mobile` |

Middleware `RequireAudience` dibangun sejajar dengan `RequirePermission`
(`backend/internal/middleware/permission.go`) dan dipasang eksplisit di
`backend/internal/server/router.go`. Klaim `aud` adalah pembatas blast radius, bukan pengganti
permission — endpoint mobile-only tetap wajib punya cek permission dan scope sendiri. Daftar
deny web-only ditulis eksplisit di router dan perubahannya lewat review PR, bukan ad-hoc.

### 5. Kontrak response endpoint shared: representasi ditentukan parameter data, bukan klien

Urutan keputusan bila kebutuhan response web dan mobile berbeda:

1. **Default: satu response kanonik.** Klien mengabaikan field yang tidak dipakai. Ini menutup
   mayoritas kasus; selisih beberapa KB bukan alasan memecah kontrak.
2. **Selisih signifikan (list besar, field mahal): parameter representasi eksplisit**, misalnya
   `?view=summary`. Parameter mendeskripsikan data yang diminta, bukan siapa pemanggilnya — web
   boleh memakainya juga. Varian wajib subset field dari response penuh (additive), dan tiap
   varian terdokumentasi sebagai schema di `openapi.yaml`.
3. **Bentuk yang memang berbeda (agregat layar): endpoint baru**, boleh mobile-only, mengikuti
   urutan standar modul backend. Ini pola BFF sebagai satu handler di monolit yang sama — tetap
   memakai service dan scope yang sama.

**Larangan:** endpoint tidak boleh mengembalikan bentuk berbeda berdasarkan `aud`, User-Agent,
atau header klien. Kontrak tersembunyi tidak terdokumentasi di OpenAPI, hanya satu cabangnya
teruji e2e, dan Hyrum's Law membuat kedua bentuk jadi komitmen permanen.

**Batas otorisasi:** field permission tetap satu-satunya mekanisme menyembunyikan field.
Urutan di service selalu: resolve field permission dulu, baru proyeksikan ke view yang diminta —
`view=full` dari user dengan field permission terbatas tetap menerima response yang dipangkas.

### 6. Persyaratan keamanan yang mengikat milestone roadmap

- **M1 (auth mobile):** klaim `aud` + validasi issuer/audience; jalur refresh per-klien
  (keputusan 3); tambahkan cek `SessionAlive` di `Refresh` (`backend/internal/identity/service.go`)
  sejajar dengan `RequireAuth` — defense-in-depth terhadap inkonsistensi dua struktur Redis.
- **M3 (push FCM):** `user_id` pemilik token diambil hanya dari context
  (`middleware.CtxUserID`), tidak pernah dari body; baris token diikat ke `session_id` dan
  dihapus saat sesi dicabut; unique constraint pada token dengan pendaftaran ulang oleh user
  lain memindahkan kepemilikan; payload push tidak memuat data sensitif (klien merender teks
  dari i18n, sesuai ARCHITECTURE.md).
- **M5 (batch sync opname):** scope dicek per item via `CallerOfficeScope`, bukan per request;
  batas eksplisit jumlah item per batch; seluruh batch dalam satu transaksi; unique constraint
  idempotensi `(session_id, user_id, client_scan_id)` agar satu device tidak bisa memblokir
  scan device lain.
- **Rate limiting:** rute terautentikasi pindah ke limiter per user+session (`PerUser`,
  memperluas ADR-0004) — limiter per-IP runtuh di belakang CGNAT operator seluler; `PerIP`
  tetap untuk rute pra-auth; endpoint batch diberi kuota berbasis jumlah item. `TRUSTED_PROXIES`
  wajib berisi CIDR Caddy di produksi.
- **Guard rails (larangan permanen):** menaikkan `JWT_ACCESS_TTL`; mengembalikan refresh token
  di body untuk klien web; melonggarkan CORS "supaya mobile jalan" (klien mobile tidak mengirim
  `Origin` — CORS tidak relevan untuknya); mengubah `SameSite` menjadi `None`; prefix URL
  terpisah (`/api/mobile/v1/...`) untuk endpoint shared.

## Alternatif yang ditolak

- **Backend/BFF terpisah untuk mobile.** Terlihat lebih aman (isolasi blast radius), tetapi
  menuntut duplikasi otorisasi di 12+ modul dengan dua cache scope Redis yang bisa drift dan
  dua jalur revocation — logout/perubahan role di satu sisi tidak menjangkau sisi lain sampai
  TTL habis. Kelas kerentanan itu tidak bisa ditutup dengan kode, hanya dengan disiplin.
  Isolasi blast radius yang sama dibeli dengan ~30 baris klaim `aud` (keputusan 2 dan 4).
- **Cookie jar persisten di Flutter (rencana awal ADR-0015).** Menyimpan refresh token di
  penyimpanan HTTP client, bukan Keystore/Keychain; rapuh di sebagian OEM Android; dan membuat
  proteksi CSRF cookie web jadi asumsi tak terlihat di klien non-browser. Fallback yang dulu
  direncanakan reaktif kini diadopsi proaktif sebelum M1 — lebih murah daripada migrasi auth
  setelah app beredar.
- **Refresh token di body untuk semua klien.** Paling cepat, tetapi menghapus proteksi httpOnly
  untuk seluruh pengguna web — satu XSS di frontend mencuri refresh token berumur panjang.
- **Memperpanjang access TTL agar mobile jarang refresh.** Role dibaca dari klaim token; TTL
  panjang berarti pegawai yang diturunkan rolenya tetap memegang hak lama selama TTL tersisa.
- **Deteksi klien via User-Agent atau prefix URL terpisah.** UA bebas dipalsukan dan tidak
  terikat token; prefix terpisah menggandakan surface yang diaudit dan mengundang drift —
  masalah backend terpisah dalam skala kecil.
- **Content negotiation berdasarkan `aud`.** Mencampur keputusan keamanan dengan keputusan
  representasi membuat keduanya tidak bisa di-reason terpisah (lihat larangan keputusan 5).

## Konsekuensi

- `backend/internal/auth/jwt.go` mendapat klaim `aud` + validasi issuer/audience;
  `backend/internal/identity` mendapat jalur login/refresh per-klien; `handler_cookie_test.go`
  direvisi menjadi invariant per-klien. Dikerjakan sekaligus sebelum M1.
- Middleware baru `RequireAudience` di `backend/internal/middleware`; router mendaftarkan deny
  `aud=mobile` untuk grup admin berisiko.
- Spec M1, M3, dan M5 di roadmap menyerap persyaratan keputusan 6 sebagai acceptance criteria —
  bukan ditemukan saat review setelahnya.
- Daftar pustaka Flutter di ADR-0015 tetap berlaku minus `dio_cookie_manager`;
  `flutter_secure_storage` menjadi penyimpanan refresh token.
- Varian `view` didokumentasikan sebagai schema terpisah di `openapi.yaml` (divalidasi Spectral
  di CI); penambahan varian selalu additive.
- Opsional (defense-in-depth, tidak memblokir milestone): `role_epoch` per user di Redis yang
  di-bump saat role/office berubah dan dicek di `RequireAuth`, menutup jendela 15 menit klaim
  role basi.
- Certificate pinning ditunda pasca-M6 (pin leaf Let's Encrypt yang dirotasi Caddy tiap 90 hari
  berisiko mematikan app); yang masuk fase rilis: HSTS di Caddy dan `usesCleartextTraffic=false`
  di manifest Android.
