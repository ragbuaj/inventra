# Rencana Implementasi: Panduan Penggunaan berbasis data dengan lampiran video dan PDF

- Tanggal: 2026-08-08
- Fase: Plan (`/plan`), agent `planner`
- Spesifikasi: `docs/superpowers/specs/2026-08-08-guide-media-cms-design.md`
- Skill: `vertical-slice`, `file-upload`
- Handoff berikutnya: `designer` (dua mockup), lalu `implementer` lewat `/build`

## Tujuan

Pengelola panduan bisa menyusun isi Panduan Penggunaan langsung dari aplikasi, lengkap
dengan video YouTube dan berkas PDF per modul, tanpa menunggu rilis kode; pembaca tanpa
sesi tetap membaca teksnya, sedangkan lampiran hanya terbuka bagi user yang sudah masuk.

## Kondisi sekarang

| Berkas | Perilaku hari ini | Relevansi |
|---|---|---|
| `frontend/app/pages/guide.vue` | 80 baris, merender `guidePage.sections` lewat `tm()`/`rt()`: ikon, judul, body, langkah bernomor | Akan diubah jadi konsumen API, struktur kartunya dipertahankan |
| `frontend/i18n/locales/{id,en}.json` | `guidePage.sections`: 9 modul, 16 langkah, 9 body, lengkap di kedua bahasa | Sumber data seed migration |
| `frontend/app/middleware/auth.global.ts` baris 7 | `/guide` termasuk `publicPaths` | Halaman tetap publik; lampiran yang dibatasi |
| `frontend/app/composables/useApiClient.ts` | Menyisipkan `Authorization` bila ada token; **401 memicu `auth.clear()` lalu redirect ke `/login`** | Halaman publik tidak boleh memanggil endpoint ber-auth saat anonim |
| `frontend/app/utils/nav.ts` | Model nav per-permission; grup `bantuan` memuat `/guide` tanpa permission | Menu pengelolaan masuk grup `administrasi` dengan permission baru |
| `frontend/app/composables/api/useCategories.ts` | Pola composable API: `useApiClient().request(...)` per resource | Pola yang diikuti `useGuide.ts` (ADR-0007) |
| Unduhan ber-auth di frontend | Sudah memakai pola blob + `URL.createObjectURL` (`useAccount.ts:154`, `pages/assets/[tag]/index.vue:456`) | PDF panduan memakai pola yang sama, bukan `<a href>` polos |
| `backend/internal/server/router.go` | Composition root tunggal; tiap modul `RegisterRoutes` di dalam grup `/api/v1` | Tempat modul `guide` disambungkan |
| `backend/internal/maintenance/{service,dto,handler,routes}.go` | Contoh kanonik pembagian empat berkas | Pola yang diikuti modul `guide` |
| `backend/internal/asset/attachment.go` baris 21 dan 60 | Allowlist MIME lalu batas ukuran divalidasi sebelum menyentuh DB/storage | Pola validasi unggahan yang diikuti |
| `backend/internal/asset/attachment_handler.go` | `http.MaxBytesReader`, `contentDisposition` RFC 6266, header `nosniff` + `Content-Security-Policy: sandbox`, stream `DataFromReader` | Pola penyajian PDF yang diikuti |
| `backend/internal/middleware/` | `auth.go` (`RequireAuth`), `permission.go`, `audience.go`, `ratelimit.go` — **tidak ada optional-auth** | Perlu tambahan `OptionalAuth` |
| `backend/internal/authz/permissions.go` | `PermissionService.Has(ctx, roleID, key)` | Dipakai handler untuk memutuskan apakah draf ikut ditampilkan |
| `backend/internal/audit/record.go` baris 18 | `Record(c, svc, action, entityType, entityID, officeID *uuid.UUID, changes)` — `officeID` pointer, boleh nil | Panduan global, `officeID` nil |
| `backend/db/migrations/` | Terakhir `000049`; pola tabel: soft delete, indeks unik parsial `WHERE deleted_at IS NULL`, trigger `shared.set_updated_at()`, enum di skema `shared` (contoh `000034_notification_module.up.sql`) | Migration baru mengikuti pola ini |
| `backend/internal/config/config.go` baris 150-181 | `ATTACHMENT_MAX_BYTES` 5 MB, `AVATAR_MAX_BYTES` 2 MB, `IMPORT_MAX_BYTES` 10 MB | Variabel baru berdiri sendiri |
| `docker-compose.prod.yml` baris 96-130 | Compose hanya meneruskan variabel yang disebut eksplisit | Variabel baru wajib didaftarkan |
| `ops/caddy/Caddyfile` baris 12-26 | Coraza WAF `Include @coraza.conf-recommended` + CRS di depan seluruh rute | Membawa plafon ukuran body bawaan; lihat risiko R1 |
| `frontend/test/nuxt/info-pages.spec.ts` | Menguji isi panduan **nyata** dari i18n | Wajib ditulis ulang jadi berbasis API tiruan |
| `frontend/test/unit/nav-model.spec.ts` | Mengunci jumlah grup, jumlah leaf bebas-permission, dan `EXPECTED_ROUTES` | Menambah satu leaf nav pasti memerahkan tes ini |
| `docs/adr/README.md` baris 35-40 | Penomoran ADR global tunggal; tertinggi 0017 (ADR mobile ada di `docs/mobile/adr/`) | ADR baru bernomor 0018 |

## Perlu keputusan

Tidak ada yang menghalangi. Tujuh keputusan produk sudah diambil di fase `/spec`; enam
keputusan teknis di bawah saya ambil sebagai arsitek dan terbuka untuk dikoreksi sebelum
`/build` dimulai.

## Asumsi

1. Urutan modul diatur lewat kolom angka pada form, bukan seret-lepas. Seret-lepas masuk
   daftar di luar cakupan.
2. Berkas PDF diganti dengan cara menghapus lampiran lalu mengunggah ulang. Tidak ada
   endpoint "ganti berkas di tempat".
3. Halaman pengelolaan hanya untuk klien web (`RequireAudience(web)`), sejalan dengan
   `authzadmin` dan `importer` pada ADR-0017.
4. Ikon modul dipilih dari daftar tertutup yang sama dengan sembilan ikon yang dipakai
   sekarang, ditambah beberapa ikon umum. Bukan input teks bebas.
5. Pembaca yang sudah masuk tidak butuh permission apa pun untuk melihat lampiran.
6. Tidak ada pemindaian antivirus untuk PDF di rilis ini. Yang diunggah hanya pemegang
   `guide.manage` (bukan konten dari user umum), dan pemindaian belum ada di jalur
   unggahan mana pun di repo ini. Dicatat sebagai utang yang berlaku untuk seluruh
   permukaan unggahan, bukan khusus fitur ini.

## Keputusan arsitektur

### AD1 — Konten panduan pindah dari i18n ke skema `guide` di PostgreSQL

**Konteks.** Isi panduan hari ini adalah bagian dari bundel frontend. Kebutuhannya berubah:
konten harus bisa diedit tanpa rilis, dan harus bisa membawa lampiran.

**Opsi.** (a) Tetap di i18n, hanya lampiran yang ke DB. (b) Seluruh konten ke DB. (c) CMS
pihak ketiga.

**Keputusan.** (b), sesuai keputusan produk. Skema baru `guide` dengan dua tabel.

**Konsekuensi.** Konten tidak lagi ikut alur terjemahan berbasis berkas; kualitas bahasa
Inggris menjadi tanggung jawab pengelola. Menambah satu skema dan satu modul backend.
Sulit dibalik setelah pengelola mulai mengedit — isi DB menjadi sumber kebenaran dan tidak
lagi ada di git.

### AD2 — Satu endpoint baca publik dengan `OptionalAuth`, bukan dua endpoint terpisah

**Konteks.** Teks harus terbaca tanpa sesi; lampiran tidak boleh bocor ke anonim.

**Opsi.** (a) Dua path berbeda: satu publik untuk teks, satu ber-auth untuk media, lalu
frontend menggabungkan. (b) Satu path dengan middleware `OptionalAuth` baru; serializer
menghilangkan `youtube_id` dan tautan berkas ketika pemanggil anonim.

**Keputusan.** (b). `useApiClient` sudah menyisipkan `Authorization` hanya ketika token
ada, jadi klien tidak perlu percabangan sama sekali; aturan "anonim melihat lebih sedikit"
hidup di satu tempat (serializer), bukan tersebar di dua endpoint dan satu penggabung di
frontend.

**Konsekuensi.** Satu respons publik yang isinya berbeda menurut header `Authorization`.
Ini rawan diracuni cache bersama, jadi endpoint itu **wajib** mengirim
`Cache-Control: no-store` dan `Vary: Authorization`. `OptionalAuth` wajib gagal ke arah
anonim: token tidak valid, kedaluwarsa, atau sudah dicabut diperlakukan sebagai tamu, bukan
401 — kalau tidak, halaman publik akan menendang pembaca ke `/login`.

### AD3 — Langkah panduan disimpan sebagai `jsonb`, bukan tabel anak

**Konteks.** Langkah adalah daftar kalimat pendek berurutan, dua bahasa, selalu diambil
bersama modulnya dan tidak pernah dikueri satuan.

**Opsi.** (a) Tabel `guide_steps` dengan FK dan `sort_order`. (b) Kolom `jsonb` berisi array
objek `{text_id, text_en}`.

**Keputusan.** (b). Penyuntingan mengganti seluruh daftar sekaligus, sehingga tabel anak
hanya menambah baris yatim, penomoran ulang, dan satu join tanpa manfaat kueri.

**Konsekuensi.** Bentuk isi `jsonb` divalidasi di Go, bukan oleh basis data. Kalau kelak
langkah perlu dikueri satuan (misalnya pencarian teks per langkah), migrasinya adalah
memecah kolom ini menjadi tabel — pekerjaan sekali jalan yang bisa ditunda sampai ada
kebutuhannya.

### AD4 — Dua bahasa sebagai kolom berpasangan pada baris yang sama

**Konteks.** Indonesia wajib, Inggris opsional dengan fallback.

**Keputusan.** `title_id`/`title_en`, `body_id`/`body_en`, dan pasangan yang sama di dalam
`jsonb` langkah. Bukan tabel terjemahan tersendiri.

**Konsekuensi.** Menambah bahasa ketiga berarti menambah kolom, bukan menambah baris. Dua
bahasa adalah keadaan yang stabil di aplikasi ini, jadi ongkos itu tidak akan ditagih.

### AD5 — Resolusi bahasa dilakukan klien, server mengirim kedua bahasa

**Konteks.** AC29 menuntut pergantian locale tanpa memuat ulang halaman.

**Keputusan.** Respons memuat `title_id` dan `title_en` sekaligus; frontend memilih beserta
fallback lewat satu helper murni.

**Konsekuensi.** Respons sedikit lebih besar (sembilan modul, bukan ribuan baris — tidak
berarti). Fallback jadi fungsi murni yang bisa diuji unit, dan respons tidak perlu
divariasikan menurut `Accept-Language`, yang berarti satu sumbu variasi cache lebih sedikit.

### AD6 — Batas PDF 10 MB sebagai bawaan, bukan 20 MB

**Konteks.** Spesifikasi mengusulkan 20 MB. Di produksi, seluruh trafik melewati Coraza WAF
yang memuat `@coraza.conf-recommended`, dan konfigurasi rekomendasi itu memasang
`SecRequestBodyLimit` bawaan sekitar 12,5 MB dengan aksi tolak. Unggahan yang ada hari ini
(lampiran 5 MB, impor 10 MB) semuanya di bawah plafon itu, yang menjelaskan kenapa belum
pernah terlihat.

**Keputusan.** `GUIDE_PDF_MAX_BYTES` bawaan 10.485.760 (10 MB), sama seperti impor yang
sudah terbukti lewat di produksi.

**Konsekuensi.** Untuk menaikkan ke 20 MB nanti, WAF harus diubah lebih dulu
(`SecRequestBodyLimit` dinaikkan untuk jalur unggahan), bukan cukup mengubah variabel
lingkungan. Langkah 15 mewajibkan pembuktian empiris batas ini di lingkungan produksi
sebelum angkanya dinaikkan.

## Perubahan kontrak

### Skema basis data — migration `000050` (DDL) dan `000051` (seed konten)

Skema baru `guide`, dua enum di `shared`, dua tabel, seluruhnya mengikuti konvensi repo
(soft delete, indeks unik parsial, trigger `shared.set_updated_at()`).

- `shared.guide_status` — `draft`, `published`
- `shared.guide_attachment_kind` — `video`, `document`

**`guide.guide_modules`** — `id`, `slug` (unik parsial; kunci stabil untuk seed idempoten
dan penautan dalam), `icon`, `sort_order`, `status`, `published_at`, `title_id` (NOT NULL),
`title_en`, `body_id`, `body_en`, `steps jsonb NOT NULL DEFAULT '[]'`, `created_by`,
`updated_by`, plus `created_at`/`updated_at`/`deleted_at`. Indeks `(status, sort_order)`
parsial `WHERE deleted_at IS NULL` untuk kueri pembaca.

**`guide.guide_attachments`** — `id`, `module_id` (FK ke modul), `kind`, `title_id` (NOT
NULL), `title_en`, `sort_order`, `youtube_id`, `object_key`, `original_filename`,
`mime_type`, `size_bytes`, `created_by`, plus tiga stempel waktu. Satu `CHECK` yang
memaksakan bentuk: `video` wajib `youtube_id` dan tidak boleh `object_key`; `document`
kebalikannya. Indeks `(module_id, sort_order)` parsial.

**Seed permission** di `000050`: `guide.manage` untuk peran `Superadmin` memakai pola
`INSERT ... SELECT ... ON CONFLICT DO NOTHING` seperti `000027`. **Tidak ada** baris
`data_scope_policies` — panduan global dan tidak melewati `CallerOfficeScope`.

**Seed konten** di `000051`: sembilan modul dari `guidePage.sections` (id dan en), status
`published`, `sort_order` mengikuti urutan sekarang, `slug` diturunkan dari judul.
Idempoten lewat `ON CONFLICT (slug) DO NOTHING`. Dipisah dari DDL supaya konten bisa
di-seed ulang atau dibatalkan tanpa menyentuh struktur.

### Endpoint HTTP baru, seluruhnya di bawah `/api/v1`

| Method | Path | Auth | Catatan |
|---|---|---|---|
| GET | `/guide/modules` | `OptionalAuth` | Modul `published`, urut `sort_order`. Anonim: tiap lampiran hanya `{id, kind, title_id, title_en, sort_order, locked:true}`. Ber-sesi: ditambah `youtube_id` atau `file_url`, `locked:false`. Pemegang `guide.manage` dengan `?status=all` juga menerima draf. Header wajib: `Cache-Control: no-store`, `Vary: Authorization` |
| POST | `/guide/modules` | auth + `guide.manage` + web-only | Membuat modul, selalu `draft` |
| PATCH | `/guide/modules/:id` | idem | Ubah isi, `sort_order`, dan `status` (jalur terbit/tarik) |
| DELETE | `/guide/modules/:id` | idem | Soft delete modul beserta lampirannya |
| POST | `/guide/modules/:id/attachments/video` | idem | JSON `{url, title_id, title_en, sort_order}`; server mengekstrak `youtube_id` |
| POST | `/guide/modules/:id/attachments/document` | idem | multipart `file` + field judul |
| PATCH | `/guide/attachments/:aid` | idem | Hanya judul dan urutan; berkas tidak diganti di tempat |
| DELETE | `/guide/attachments/:aid` | idem | Soft delete baris, objek MinIO dihapus |
| GET | `/guide/attachments/:aid/content` | `RequireAuth` saja | Stream PDF. Tanpa sesi: 401 |

Bentuk daftar mengikuti kebiasaan repo: `{data, total}`. Bentuk error `{"error": "..."}`.
Kode status: 400 id tidak valid, 401 tanpa sesi, 403 tanpa permission, 404 tidak ada,
413 berkas terlalu besar, 415 tipe tidak didukung, 422 validasi (termasuk tautan YouTube
tidak sah), 429 kena batas laju.

### Dampak ke klien lama

Tidak ada. Seluruh endpoint baru; tidak ada kontrak yang berubah bentuk. Aplikasi mobile
tidak memanggil apa pun di sini. Rilis pertama juga **tidak** menghapus kunci
`guidePage.sections` dari i18n (lihat langkah 17), sehingga membatalkan rilis cukup dengan
mengembalikan kode.

## Pertimbangan keamanan

STRIDE singkat pada permukaan baru:

| Ancaman | Jalur | Penanggulangan |
|---|---|---|
| Elevation of privilege | User biasa mengubah konten panduan yang dibaca semua orang | `RequirePermission(permSvc, "guide.manage")` pada setiap rute tulis, plus `RequireAudience(web)`. Menu di frontend bukan kontrol akses |
| Information disclosure | PDF internal terambil tanpa sesi | Berkas hanya lewat `GET /guide/attachments/:aid/content` di balik `RequireAuth`; endpoint publik tidak pernah memuat `file_url` maupun `youtube_id`. Kunci objek berbasis UUID bukan kontrol akses dan tidak diperlakukan sebagai kontrol akses |
| Information disclosure lewat cache | Cache bersama menyimpan respons ber-sesi lalu menyajikannya ke anonim | `Cache-Control: no-store` + `Vary: Authorization` pada endpoint publik |
| Tampering | Berkas berbahaya diunggah dan disajikan ulang dalam origin aplikasi | Allowlist tipe dari **isi berkas** (`%PDF-`), nama objek dibuat sistem, penyajian dengan `Content-Type` dari server, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox`, dan pratinjau di dalam `<iframe sandbox>` |
| Tampering lewat header | Nama berkas menyisipkan byte header | `contentDisposition` yang sudah ada (membuang CR/LF, `mime.FormatMediaType`) |
| Injection lewat sematan | Tautan "YouTube" yang sebenarnya mengarah ke domain lain, atau skema `javascript:`/`data:` | Parser mem-parse URL, mencocokkan host secara **tepat** dengan `youtube.com`, `www.youtube.com`, `m.youtube.com`, `youtu.be`; identitas video harus `[A-Za-z0-9_-]{11}`; yang disematkan adalah URL yang **dibangun server** ke `youtube-nocookie.com/embed/<id>`, bukan URL dari input |
| Denial of service | Unggahan raksasa atau beruntun | `http.MaxBytesReader` pada `maxBytes+1`, batas laju global per IP yang sudah ada, plus plafon WAF |
| Repudiation | Konten berubah tanpa jejak | `audit.Record` pada setiap aksi tulis, `officeID` nil karena panduan global |

Catatan tambahan: `OptionalAuth` tidak boleh memakai ulang jalur 401 milik `RequireAuth`.
Token cacat harus berujung "tamu", bukan penolakan — kalau tidak, satu token kedaluwarsa
akan membuat halaman panduan publik menendang pembacanya ke halaman login.

## Langkah

Setiap langkah meninggalkan repo dalam keadaan hijau. Perintah verifikasi backend
dijalankan dari `backend/`, frontend dari `frontend/`.

### Fase 0 — desain (sebelum kode frontend)

**0. Dua mockup lewat agent `designer`** — berkas: `docs/design/Panduan Media.dc.html`,
`docs/design/Panduan Pengelolaan.dc.html`
Alasan menjalankan `designer` dan bukan langsung membangun: halaman pengelolaan memuat dua
komponen yang belum pernah ada di repo ini — penyunting daftar langkah yang bisa
ditambah/dihapus/diurutkan, dan pengelola lampiran yang menggabungkan unggah berkas dengan
tempel tautan. Keduanya bukan tabel CRUD standar, jadi tidak ada layar lama yang bisa
ditiru. Mockup pertama mencakup blok media pada halaman pembaca beserta keadaan terkunci
untuk anonim; mockup kedua mencakup daftar modul (status, penanda belum diterjemahkan,
urutan) beserta penyuntingnya.
Verifikasi: kedua berkas dibuka di browser dan disetujui pemilik produk.
Bisa di-deploy sendiri: tidak berlaku (tidak menyentuh kode).

### Fase 1 — basis data

**1. Migration DDL** — berkas: `backend/db/migrations/000050_guide_module.{up,down}.sql`
Membuat skema `guide`, dua enum di `shared`, dua tabel beserta indeks dan trigger, dan
menyemai permission `guide.manage` untuk `Superadmin`. `down` menghapus urut terbalik
termasuk enum dan baris permission.
Verifikasi: `migrate up` lalu `migrate down 1` lalu `migrate up` pada Postgres dev tanpa
error; `\d guide.guide_modules` menampilkan trigger dan indeks parsial.
Bisa di-deploy sendiri: ya (tabel kosong yang belum dibaca siapa pun).

**2. Migration seed konten** — berkas: `backend/db/migrations/000051_guide_seed.{up,down}.sql`
Menyisipkan sembilan modul dari `guidePage.sections` (id dan en), status `published`,
idempoten lewat `ON CONFLICT (slug) DO NOTHING`. `down` menghapus baris berdasarkan
daftar `slug` yang sama.
Verifikasi: jalankan `up` dua kali, `SELECT count(*) FROM guide.guide_modules` tetap 9;
`SELECT jsonb_array_length(steps) ...` menghasilkan 3,0,4,3,3,0,3,0,0 sesuai isi i18n.
Bisa di-deploy sendiri: ya.

**3. Query dan kode terbangkit** — berkas: `backend/db/queries/guide.sql`, `backend/db/sqlc/*`
Kueri: daftar modul (dengan/tanpa draf), ambil modul, buat/ubah/hapus modul, daftar dan
CRUD lampiran, ambil lampiran satuan. Lalu `sqlc generate`.
Verifikasi: `sqlc generate` bersih lalu `go build ./...`.
Bisa di-deploy sendiri: ya.

### Fase 2 — backend

**4. Service dan validator** — berkas: `backend/internal/guide/service.go`,
`backend/internal/guide/youtube.go`, `backend/internal/guide/youtube_test.go`,
`backend/internal/guide/pdf_test.go`
Logika bisnis, sentinel error, `mapDBError`, validasi bentuk `steps`, parser YouTube
(allowlist host tepat, ekstraksi identitas, pembuangan parameter), verifikasi PDF dari
magic bytes, batas ukuran, `Put`/`Remove` ke MinIO dengan rollback dua arah.
Verifikasi: `go test ./internal/guide/ -run 'TestYouTube|TestPDF'` hijau, termasuk seluruh
tautan jahat yang disebut AC14.
Bisa di-deploy sendiri: ya (belum terpasang di rute mana pun).

**5. Optional-auth middleware** — berkas: `backend/internal/middleware/optionalauth.go`,
`backend/internal/middleware/optionalauth_test.go`
Memvalidasi Bearer bila ada, memeriksa pencabutan di Redis, menetapkan `CtxUserID` dan
`CtxRoleID` bila sah; **selalu** melanjutkan rantai. Tanpa header, token cacat, token
kedaluwarsa, dan token dicabut sama-sama berujung anonim.
Verifikasi: `go test ./internal/middleware/ -run TestOptionalAuth` mencakup empat kasus itu.
Bisa di-deploy sendiri: ya.

**6. DTO, handler, dan rute** — berkas: `backend/internal/guide/{dto,handler,routes}.go`
Serializer yang menghilangkan `youtube_id` dan `file_url` untuk pemanggil anonim; handler
memanggil `permSvc.Has(..., "guide.manage")` untuk memutuskan apakah draf ikut; pemetaan
sentinel error ke status; header `Cache-Control: no-store` dan `Vary: Authorization` pada
endpoint publik; header penyajian berkas mengikuti `attachment_handler.go`; `audit.Record`
pada setiap aksi tulis.
Verifikasi: `go build ./... && go vet ./...`.
Bisa di-deploy sendiri: belum tersambung, jadi ya.

**7. Konfigurasi dan penyambungan** — berkas: `backend/internal/config/config.go`,
`backend/internal/server/router.go`, `backend/.env.example`
`GuidePDFMaxBytes` dari `GUIDE_PDF_MAX_BYTES` dengan bawaan 10.485.760; konstruksi service
dan handler lalu `guide.RegisterRoutes` di dalam grup `api`.
Verifikasi: jalankan stack dev, `curl -i localhost:8080/api/v1/guide/modules` mengembalikan
200 dengan sembilan modul tanpa header `Authorization`, dan tidak satu pun memuat
`youtube_id` atau `file_url`.
Bisa di-deploy sendiri: ya — inilah rilis backend pertama yang berguna.

**8. Kontrak OpenAPI** — berkas: `backend/api/openapi.yaml`
Tag `Guide` baru beserta sembilan operasi, skema, dan seluruh respons error.
Verifikasi: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`.
Bisa di-deploy sendiri: ya.

**9. Tes integrasi** — berkas: `backend/internal/guide/guide_integration_test.go`
Menutupi jalur otorisasi (anonim, user biasa, pemegang permission), penyembunyian draf,
penolakan unggahan tipe salah dan ukuran lebih, 401 pada berkas tanpa sesi, dan rollback
saat penyimpanan objek gagal.
Verifikasi: `go test -tags=integration ./internal/guide/` — dijalankan di CI; testcontainers
tidak jalan di Windows lokal.
Bisa di-deploy sendiri: ya.

### Fase 3 — frontend

**10. Tipe, composable, dan helper bahasa** — berkas: `frontend/app/types/*`,
`frontend/app/composables/api/useGuide.ts`, `frontend/app/utils/guideText.ts`,
`frontend/test/unit/guide-text.spec.ts`
Composable mengikuti pola ADR-0007; helper memilih bahasa dengan fallback; pengambilan PDF
memakai pola blob seperti `useAccount.ts`, bukan `<a href>`.
Verifikasi: `pnpm test -- guide-text` hijau (fallback, kedua bahasa, string kosong).
Bisa di-deploy sendiri: ya.

**11. Halaman pembaca berbasis data** — berkas: `frontend/app/pages/guide.vue`,
`frontend/app/components/guide/GuideMediaCard.vue`, `frontend/i18n/locales/{id,en}.json`
Struktur kartu dipertahankan; ditambah keadaan muat, kosong, dan error dengan tombol coba
lagi; blok media sesuai mockup langkah 0; sematan YouTube dimuat malas ke
`youtube-nocookie.com` dengan `title` terisi; PDF ditampilkan dalam `<iframe sandbox>` dari
object URL. **Saat anonim, halaman tidak boleh memanggil endpoint ber-auth apa pun** —
`useApiClient` mengubah 401 menjadi `auth.clear()` plus redirect ke `/login`, yang akan
menendang pembaca publik. Kunci i18n baru ditambahkan; `guidePage.sections` **dibiarkan**
untuk sementara.
Verifikasi: `pnpm lint`, `vue-tsc -p .nuxt/tsconfig.json --noEmit`, dan tes runtime di
langkah 14.
Bisa di-deploy sendiri: ya.

**12. Halaman pengelolaan** — berkas: `frontend/app/pages/settings/guide.vue`,
`frontend/app/components/guide/GuideModuleForm.vue`,
`frontend/app/components/guide/GuideAttachmentManager.vue`, `frontend/app/utils/nav.ts`,
`frontend/i18n/locales/{id,en}.json`
Dibangun mengikuti mockup langkah 0, seluruhnya dari komponen `U*`; leaf nav baru di grup
`administrasi` dengan `permission: 'guide.manage'`; input angka memakai komponen angka yang
sudah ada (tolak karakter non-angka, tanpa minus).
Verifikasi: `pnpm lint` dan `vue-tsc` bersih; menu tidak muncul untuk peran tanpa permission.
Bisa di-deploy sendiri: ya.

**13. Perbaiki tes nav yang pasti merah** — berkas: `frontend/test/unit/nav-model.spec.ts`,
`frontend/test/nuxt/app-sidebar.spec.ts`
Menambah satu leaf mengubah jumlah rute yang dikunci tes ini. Ini bukan efek samping tak
terduga — ini konsekuensi yang sudah diketahui dari rilis halaman info sebelumnya.
Verifikasi: `pnpm test -- nav-model app-sidebar`.
Bisa di-deploy sendiri: ya.

**14. Tulis ulang tes halaman info dan tambah tes baru** — berkas:
`frontend/test/nuxt/info-pages.spec.ts`, `frontend/test/nuxt/guide-page.spec.ts`,
`frontend/test/nuxt/guide-admin.spec.ts`
`info-pages.spec.ts` sekarang menguji isi panduan nyata dari i18n dan akan gagal begitu
halaman beralih ke API — diubah menjadi berbasis API tiruan. Tes baru menutupi keadaan
muat/kosong/error, kartu terkunci untuk anonim, sematan video, fallback bahasa, dan validasi
form pengelolaan.
Verifikasi: `pnpm test` seluruh berkas hijau, dan **periksa kode keluarnya**, bukan hanya
keluaran berkas yang diubah.
Bisa di-deploy sendiri: ya.

**15. E2E** — berkas: `frontend/e2e/guide.spec.ts`
Admin membuat modul, menambahkan video dan PDF, menerbitkan, lalu modul itu muncul di
`/guide`; konteks anonim (`clearCookies` plus bersihkan `localStorage`) melihat kartu
terkunci dan tidak melihat `youtube_id`; user tanpa permission tidak melihat menunya. Nama
dan slug modul dibuat unik per jalannya tes karena data e2e persisten.
Verifikasi: `pnpm test:e2e -- guide` dengan stack menyala dan `RATELIMIT_ENABLED=false` di
backend lokal.
Bisa di-deploy sendiri: ya.

### Fase 4 — operasional dan dokumen

**16. Daftarkan variabel lingkungan** — berkas: `docker-compose.prod.yml`,
`.env.prod.example`, `backend/.env.example`
Compose produksi hanya meneruskan variabel yang disebut eksplisit; tanpa baris ini backend
akan diam-diam memakai bawaan. Tambahkan `GUIDE_PDF_MAX_BYTES: "${GUIDE_PDF_MAX_BYTES:-10485760}"`.
Verifikasi: `docker compose -f docker-compose.prod.yml config` menampilkan variabel itu pada
service backend.
Bisa di-deploy sendiri: ya.

**17. Buktikan plafon WAF sebelum menaikkan batas** — berkas: catatan rilis, opsional
`ops/caddy/Caddyfile`
Unggah PDF berukuran tepat di bawah dan tepat di atas batas melalui domain produksi. Kalau
yang di bawah batas lolos dan yang di atas ditolak backend (bukan WAF), angka 10 MB
terkonfirmasi. Menaikkan ke 20 MB menuntut perubahan `SecRequestBodyLimit` lebih dulu.
Verifikasi: dua `curl` terhadap `https://<domain>/api/v1/guide/modules/<id>/attachments/document`
dengan kode status yang tercatat di catatan rilis.
Bisa di-deploy sendiri: tidak menyentuh kode.

**18. Dokumen** — berkas: `docs/adr/0018-guide-content-data-driven.md`, `docs/adr/README.md`,
`docs/DATABASE.md`, `docs/PROGRESS.md`
ADR bernomor **0018** (0015-0017 dipakai ADR mobile di `docs/mobile/adr/`, penomorannya
global tunggal) memuat AD1, AD2, dan AD6; tabel master di `docs/adr/README.md` ditambah;
`docs/DATABASE.md` menerima skema `guide`; `docs/PROGRESS.md` dicentang beserta nomor PR.
Verifikasi: pembacaan; tautan ADR di README menuju berkas yang ada.
Bisa di-deploy sendiri: ya.

**19. Contract — hapus kunci i18n lama** (rilis berikutnya, bukan rilis ini) — berkas:
`frontend/i18n/locales/{id,en}.json`
Hapus `guidePage.sections` setelah halaman berbasis data terbukti jalan di produksi.
Menahannya satu rilis membuat pembatalan cukup dengan mengembalikan kode.
Verifikasi: `pnpm lint`, `pnpm test`, dan `/guide` masih menampilkan sembilan modul dari API.
Bisa di-deploy sendiri: ya.

## Risiko dan mitigasi

| Risiko | Kemungkinan | Dampak | Mitigasi |
|---|---|---|---|
| R1 — WAF menolak unggahan PDF sebelum sampai ke backend (plafon body ~12,5 MB) | Sedang | Fitur tampak rusak hanya di produksi, dengan 403/413 yang membingungkan | Bawaan 10 MB (AD6); langkah 17 membuktikannya di produksi sebelum angka dinaikkan |
| R2 — Cache bersama menyajikan respons ber-sesi ke anonim | Rendah | Kebocoran identitas video dan tautan berkas | `Cache-Control: no-store` plus `Vary: Authorization`, diuji di tes integrasi |
| R3 — Cache permission di Redis basi setelah seed `guide.manage` | Tinggi | Superadmin tidak melihat menu pengelolaan sampai cache kedaluwarsa | Setelah migration, invalidasi cache permission (di dev: `redis-cli FLUSHALL`); masukkan ke prosedur deploy |
| R4 — `OptionalAuth` salah menolak dan menendang pembaca publik ke `/login` | Sedang | Halaman panduan publik rusak untuk pengunjung bertoken basi | Middleware wajib selalu `Next()`; empat kasus diuji di langkah 5 |
| R5 — Tes yang sudah ada memerah (`info-pages.spec.ts`, `nav-model.spec.ts`) | Tinggi | CI merah, dikira regresi | Sudah dijadwalkan sebagai langkah 13 dan 14, bukan kejutan |
| R6 — Objek MinIO yatim karena kegagalan sebagian | Rendah | Sampah storage yang tidak terlihat | Rollback dua arah di service; kegagalan penghapusan dicatat log dengan kunci objeknya |
| R7 — Konten panduan hanya hidup di DB produksi, tidak lagi di git | Sedang | Konten hilang kalau basis data hilang | Sudah tercakup cadangan basis data yang ada; seed `000051` mengembalikan sembilan modul dasar |
| R8 — PDF berbahaya dibagikan ke seluruh pembaca | Rendah | Distribusi malware dalam origin aplikasi | Pengunggah terbatas pemegang `guide.manage`; `nosniff`, `sandbox`, dan `<iframe sandbox>`; pemindaian antivirus dicatat sebagai utang lintas-fitur |
| R9 — Pengelola mengunggah PDF berisi data nasabah lalu menerbitkan modulnya | Rendah | Data internal terbuka ke semua user ber-sesi | Lampiran tidak pernah publik; peringatan di form unggah; jejak audit menyebut pengunggahnya |

## Cara membatalkan

Urutannya sengaja dibuat sehingga pembatalan basis data tidak pernah mendesak.

1. **Kembalikan kode** (backend dan frontend ke rilis sebelumnya). Karena langkah 19 belum
   dijalankan, `guidePage.sections` masih ada di bundel, sehingga halaman panduan langsung
   kembali ke versi statisnya. Tabel `guide.*` tetap ada tetapi tidak dibaca siapa pun —
   tidak berbahaya.
2. **Kalau memang perlu bersih**: `migrate down 2` menghapus seed konten lalu DDL. Jangan
   dijalankan sebelum langkah 1, karena kode baru tanpa tabel akan mengembalikan 500.
3. **Setelah langkah 19 dirilis**, pembatalan menjadi tidak simetris: mengembalikan kode
   saja mengosongkan halaman karena kunci i18n sudah tidak ada. Sejak titik itu, pembatalan
   berarti mengembalikan kode **ke rilis sebelum langkah 19** sekaligus memastikan tabel
   masih terisi. Ini wajib ditulis di catatan rilis langkah 19.
4. **Objek MinIO** yang sudah diunggah dibiarkan. Ia tidak bisa diakses tanpa baris
   lampirannya, dan menghapusnya membuat pembatalan tidak bisa dibalik lagi.

## Cara mengukur keberhasilan

- Log akses menunjukkan `GET /guide/attachments/:aid/content` tidak pernah menjawab 200
  tanpa sesi. Satu pun kejadian sebaliknya adalah insiden.
- Perubahan panduan tayang tanpa deploy: hitung selisih antara `updated_at` modul dan
  waktu rilis kode terakhir; keduanya tidak lagi berkorelasi.
- Setelah dua bulan, minimal lima dari sembilan modul memiliki setidaknya satu lampiran.
- Rasio 4xx pada endpoint unggahan tetap rendah setelah rilis; lonjakan 413 berarti batas
  ukurannya salah pilih (lihat R1).

## Di luar cakupan

Mengikuti spesifikasi, ditambah dua hal yang muncul saat perencanaan:

- Aplikasi mobile (belum ada layar panduan sama sekali di `mobile/lib`).
- Seret-lepas untuk mengurutkan modul dan lampiran; urutan diatur lewat kolom angka.
- Mengganti berkas PDF di tempat; caranya hapus lalu unggah ulang.
- Riwayat versi konten, pencarian dalam panduan, analitik per modul, panduan per kantor,
  dan alur persetujuan untuk penerbitan konten.
- Menjadikan halaman FAQ dan Kebijakan Privasi berbasis data.
- Pemindaian antivirus pada unggahan — utang yang berlaku untuk seluruh permukaan unggahan
  di repo ini, bukan khusus fitur ini.
