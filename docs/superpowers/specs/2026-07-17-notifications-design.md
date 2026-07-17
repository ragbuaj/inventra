# Spec: Modul Notifikasi (in-app)

Tanggal: 2026-07-17
Status: draft — menunggu review
Branch: `feat/notifications`

## 1. Tujuan

Mengganti `frontend/app/mock/notifications.ts` — fixture terakhir di app-shell — dengan modul
notifikasi in-app sungguhan: pipeline event yang durabel, bel + dropdown terhubung ke data nyata,
dan halaman `/notifications` penuh.

Pengguna: setiap user terautentikasi. Notifikasi bersifat **per-user** — tidak ada permission gate
dan tidak ada modul data-scope; kepemilikan ditegakkan lewat `WHERE user_id = caller`.

Sukses = bel menampilkan notifikasi nyata dari ketiga sumber yang diminta mockup, unread-count
akurat, mark-read bertahan lintas sesi, tidak ada event hilang, dan `app/mock/` terhapus seluruhnya.

## 2. Keputusan yang sudah diambil (user, 2026-07-17)

1. **Storage: tabel Postgres**, tanpa cache Redis. Menyimpang dari PRD A1b -> butuh ADR + update
   PRD & DATABASE.md.
2. **Cakupan: 3 jenis penuh + scheduler** — fidelitas 1:1 dengan mockup.
3. **Refresh: event-driven**; SSE dicatat sebagai follow-up eksplisit.
4. **Halaman `/notifications` dibangun** — tidak ada mockup, user menyetujui saya merancangnya
   mengikuti bahasa visual App Shell.
5. **Retensi 90 hari, seragam** untuk keempat jenis, via `NOTIFICATION_RETENTION_DAYS`.
6. **Purge memakai soft delete** — mengikuti konvensi DATABASE.md.
7. **`approval_pending` basi di-auto-resolve** saat giliran lewat.
8. **Transport: transactional outbox + Redis Streams.** Tujuan yang dikejar: fan-out keluar dari
   request path, durabilitas + retry, dan konsumen ganda untuk integrasi masa depan.

### Prinsip yang mendasari (user, 2026-07-17)

**(a) Notifikasi bersifat ephemeral dan turunan — bukan sumber kebenaran untuk audit.** Yang wajib
retensi panjang adalah data aset dan audit trail-nya; notifikasi hanyalah pemberitahuan atas event
yang sudah tercatat permanen di tempat lain. Konsekuensinya: notifikasi boleh dibersihkan agresif,
dan "menjaga jejak audit" tidak pernah jadi alasan sah untuk mempertahankan baris notifikasi.

Premis ini terverifikasi di codebase, bukan sekadar teori — setiap event di balik keempat jenis
sudah terekam lebih dulu: `audit.Record` di approval submit (`approval/handler.go:126`) dan decide
(`:158`, tiap langkah rantai per FR-6.6), serta check-in (`assignment/handler.go:110`).
`maintenance_due` tidak punya baris audit dan memang tidak perlu — recordnya adalah
`maintenance_schedules.next_due_date` yang persisten.

**(b) State permanen ada di DB, broker bukan penyimpanan.** Redis Stream murni transport antara
dua tabel Postgres (`outbox` -> `notifications`). Kalau Redis hilang total, tidak ada state yang
hilang; relay mengirim ulang dari outbox.

## 3. Sumber kebenaran visual

`docs/design/App Shell.dc.html` baris 123-136 (bel + dropdown) dan 276-287 (isi `NOTIFS`).
Mockup menuntut tiga baris: approval menunggu, maintenance jatuh tempo, aset dikembalikan.
Tidak ada mockup untuk halaman penuh — lihat bagian 13 (deviasi).

## 4. Temuan yang membentuk desain (tersitasi)

1. **Sistem tidak bisa menyebutkan siapa approver yang berhak.** Kelayakan approval adalah predikat
   satu-arah `(request, caller) -> bool` (`approval/service.go:172` `eligibleToDecide`), dan `Inbox`
   (`:486`) adalah **pull model**: melist semua request pending lalu menyaringnya untuk **satu**
   caller. Tidak ada invers `(request) -> []userID`, dan tidak ada query yang mengembalikan user
   pemegang sebuah permission (`db/queries/authz.sql` hanya `GetOfficeSubtree` +
   `ListFieldPermissionsByRole`). Fan-out **harus membangun invers itu**.
2. **Custodian aset bukan user login.** `assignment.employee_id` menunjuk `masterdata.employees`
   (`000011_assignment.up.sql:8`); tidak ada `GetUserByEmployeeID`. **Celah ini dihindari, bukan
   ditutup**: penerima `asset_returned` dipilih `assignments.assigned_by_id` — user yang
   meng-check-out — yang sudah berupa user id konkret (`assignment/service.go:109,114`).
3. **Tidak ada query jadwal maintenance jatuh tempo yang bisa dipakai.** Satu-satunya predikat
   due-date ada di `report.sql:76` yang **hardcoded `LIMIT 3`** dan ter-scope. Butuh query baru.
4. **Tidak ada distributed lock Redis.** Presedennya Postgres:
   `pg_advisory_xact_lock(hashtext('depreciation.compute'))` (`depreciation.sql:3-5`).
5. **Redis jalan dengan config default** (`docker-compose.yml:26`, tanpa `command` override) —
   **RDB snapshot saja, AOF mati**. Volume ter-mount jadi selamat saat restart normal, tapi crash
   bisa kehilangan satu jendela snapshot. Stream butuh `--appendonly yes`.
6. **Pola antrean Postgres sudah ada** — `importer/worker.go` + `FOR UPDATE SKIP LOCKED`
   (`importer.sql:26-38`), lengkap dengan `Tick()` yang diekspor untuk tes deterministik
   (`worker.go:150`). Relay outbox menyalinnya.

## 5. Arsitektur pipeline

```
handler -> service (tx bisnis)
              |
              +-- INSERT notification.outbox        <- se-transaksi: tidak ada dual-write
                        |
                   [relay worker]  outbox -> XADD -> Redis Stream
                        |
                   [consumer group]  XREADGROUP -> resolve penerima -> INSERT notifications -> XACK
                        |
                   GET /notifications  (feed per-user)
```

- **Outbox ditulis di dalam transaksi bisnis.** Ini mengubah desain awal: hook **pindah dari
  handler ke service**. Konsekuensinya `approval.Notifier` tidak diperlukan sama sekali dan
  kekhawatiran import cycle lenyap — yang tersisa hanya satu query di tx yang sudah ada.
- **Semantik kegagalan berubah, dan itu disengaja.** Kalau insert outbox gagal, tx bisnis rollback.
  Ini bukan pelanggaran prinsip `audit.Record` yang best-effort: insert ke tabel lokal di tx yang
  sama hanya gagal kalau DB sedang mati — kondisi di mana operasi bisnisnya toh gagal juga. Ini
  harga yang benar untuk jaminan "tidak ada event hilang".
- **Penerima diresolve di consumer, bukan saat enqueue** — menjaga tx bisnis tetap pendek.
- **At-least-once**: consumer bisa memproses ulang. Duplikat ditangani `uq_notif_dedup`
  + `ON CONFLICT DO NOTHING`.
- **Retry gratis dari Redis Streams**: pesan belum di-ack nongkrong di PEL; `XAUTOCLAIM` mengambil
  alih yang tersangkut karena consumer mati.
- **Konsumen ganda**: kanal email masa depan cukup jadi consumer group kedua di stream yang sama,
  tanpa menyentuh produsen.

## 6. Model data (migrasi `000034`)

Skema `notification`, mengikuti konvensi repo (soft-delete, partial-unique, trigger
`shared.set_updated_at`).

```sql
CREATE TYPE shared.notification_type AS ENUM (
  'approval_pending', 'approval_decided', 'maintenance_due', 'asset_returned'
);

-- Transport: event bisnis, ditulis se-transaksi dengan perubahannya.
CREATE TABLE notification.outbox (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type     text NOT NULL,   -- request_submitted | request_decided | chain_advanced | ...
  aggregate_type text NOT NULL,   -- requests | assignments | maintenance_schedules
  aggregate_id   uuid NOT NULL,
  payload        jsonb NOT NULL DEFAULT '{}',
  published_at   timestamptz,     -- diisi setelah XADD sukses
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

-- State permanen: feed per-user.
CREATE TABLE notification.notifications (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES identity.users(id),
  type        shared.notification_type NOT NULL,
  params      jsonb NOT NULL DEFAULT '{}',   -- parameter interpolasi i18n
  entity_type text,                          -- deep-link
  entity_id   uuid,
  dedup_key   text,
  read_at     timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
```

Index:
- `idx_outbox_unpublished` — partial `(created_at) WHERE published_at IS NULL AND deleted_at IS NULL`
  (query klaim relay).
- `idx_notif_user_unread` — partial `(user_id) WHERE read_at IS NULL AND deleted_at IS NULL`.
- `idx_notif_user_created` — `(user_id, created_at DESC) WHERE deleted_at IS NULL`.
- `uq_notif_dedup` — partial unique `(user_id, dedup_key) WHERE dedup_key IS NOT NULL AND
  deleted_at IS NULL`. **Ini yang membuat consumer at-least-once aman.**

**Teks tidak disimpan di server.** Baris menyimpan `type` + `params`; frontend merender lewat
`$t()`. Menyimpan kalimat Bahasa Indonesia jadi di DB akan mematikan pergantian locale.

### Retensi

Sweeper me-soft-delete notifikasi `created_at` lebih tua dari `NOTIFICATION_RETENTION_DAYS`
(default 90), seragam untuk keempat jenis — semua sudah tertaut ke record induknya lewat
`entity_type`/`entity_id`, jadi yang approval pun tidak lebih "berbukti" daripada yang informasional.
Outbox yang sudah `published_at` ikut di-purge di window yang sama.

Soft delete tidak membuat purge kosmetik: karena **semua index-nya partial**
(`WHERE deleted_at IS NULL`), baris ter-purge keluar sepenuhnya dari index, sehingga feed dan
unread-count tetap cepat berapa pun tabel tumbuh. Yang tumbuh hanya disk — ditangani job
arsip/partisi di masa depan. Follow-up tercatat.

## 7. Penerima tiap jenis

| Jenis | Penerima | Cara resolusi (di consumer) |
|---|---|---|
| `approval_pending` | Approver berhak di step berjalan | **Invers baru** (bagian 8) |
| `approval_decided` | Maker pengajuan | `requests.maker_id` — sudah konkret |
| `maintenance_due` | User dengan `maintenance.manage` dalam scope kantor aset | Invers yang sama |
| `asset_returned` | User yang meng-check-out | `assignments.assigned_by_id` — sudah konkret |

## 8. Membangun invers: siapa yang berhak?

```sql
-- name: ListUsersWithPermission :many
SELECT u.id, u.role_id, u.office_id
FROM identity.users u
JOIN identity.role_permissions rp ON rp.role_id = u.role_id
WHERE rp.permission = @permission
  AND u.status = 'active' AND u.deleted_at IS NULL;
```

Kandidat disaring **di Go** dengan predikat yang sudah ada, bukan SQL baru yang menduplikasi
aturannya: `approval.Service` mendapat `NotifiableApprovers(ctx, req, step) ([]uuid.UUID, error)`
yang membangun `Caller` per kandidat (scope via `scopeSvc`, ter-cache Redis) lalu memanggil
`eligibleToDecide` apa adanya. **Aturan SoD tidak diduplikasi** — maker dan approver sebelumnya
tersaring otomatis. `maintenance_due` memakai invers yang sama + `common.InScope`.

Fan-out-on-write: penerima di-snapshot saat consumer memproses. Perubahan peran setelahnya tidak
mengubah notifikasi lama — perilaku standar dan diterima.

## 9. Arsitektur backend

Modul `internal/notification/`: `service.go` / `dto.go` / `handler.go` / `routes.go`, plus
`relay.go`, `consumer.go`, `sweeper.go`.

### Endpoint (semua `RequireAuth`, tanpa `RequirePermission`)

| Method | Path | Keterangan |
|---|---|---|
| GET | `/notifications` | `{data,total,limit,offset}`, filter `read`, `limit` clamp 1-100 |
| GET | `/notifications/unread-count` | `{count}` — meniru `/requests/inbox/count` |
| POST | `/notifications/:id/read` | tandai satu dibaca |
| POST | `/notifications/read-all` | tandai semua dibaca |

Kepemilikan ditegakkan di **setiap** verb lewat `WHERE user_id = caller`. Mark-read atas milik user
lain -> **404, bukan 403** (tidak membocorkan keberadaan).

### Titik enqueue (di dalam tx service)

- `approval.Service.Submit` — tx `service.go:99-133`, enqueue sebelum commit `:131`.
- `approval.Service.Decide` — tiga cabang berbeda: ditolak (`:311`), rantai maju (`:342` — event
  "giliran approver berikutnya"), disetujui final (`:353`). Plus `Cancel` (`:377`).
- `assignment.Service.Checkin` — tx `service.go:192-203`.
- Sweeper `maintenance_due`: **juga lewat outbox**, bukan tulis langsung — supaya jalurnya seragam
  dan konsumen email nanti ikut menerimanya.

### Auto-resolve notifikasi basi

Begitu giliran step lewat (rantai maju, ditolak, disetujui final, dibatalkan), seluruh
`approval_pending` untuk step itu **di-soft-delete** — bukan sekadar ditandai dibaca, karena
notifikasi tak tertindaklanjuti tidak layak tampil. Disasar lewat `dedup_key =
'request:<id>:step:<n>'`. Alasannya murni UX; tidak ada bukti yang hilang (audit log mencatat tiap
langkah, FR-6.6).

### Komponen latar

Ketiganya menyalin pola `importer/worker.go` (`NewX`, `Recover`, `Run` dengan ticker, **`Tick(ctx)`
diekspor** untuk tes deterministik — `worker.go:150`) dan dijalankan dari `cmd/api/main.go` seperti
`importWorker` (`main.go:95-100`), ikut `workerCtx`/`stopWorker` (`main.go:107`).

- **`relay.go`** — klaim outbox `WHERE published_at IS NULL ... FOR UPDATE SKIP LOCKED`, `XADD`,
  tandai `published_at`.
- **`consumer.go`** — `XREADGROUP` (group `notification-fanout`) -> resolve penerima -> insert
  notifikasi -> `XACK`; `XAUTOCLAIM` untuk pesan tersangkut.
- **`sweeper.go`** — scan `ListSchedulesDueBetween` -> enqueue outbox; purge retensi. Lock
  `pg_advisory_xact_lock(hashtext('notification.sweep'))` (preseden `depreciation.sql:3-5`, bukan
  lock Redis).

### Infra

`--appendonly yes` pada Redis di `docker-compose.yml`, `docker-compose.dev.yml`,
`docker-compose.prod.yml` — default `redis:7-alpine` hanya RDB, dan stream butuh durabilitas.

### Config

`NOTIFICATION_WORKER_ENABLED` (true), `NOTIFICATION_RELAY_POLL` (2s, meniru importer),
`NOTIFICATION_SWEEP_POLL` (1h), `NOTIFICATION_RETENTION_DAYS` (90), `NOTIFICATION_STREAM_MAXLEN`
(trim stream — transport, bukan penyimpanan).

## 10. Frontend

- **`NotificationBell.vue`** (bel ada di sini, bukan `AppTopbar.vue` — `:75` hanya memasangnya).
  Struktur `UPopover` dipertahankan.
- **`useNotifications.ts` ditulis ulang** ke `useApiClient().request` (preseden `useApproval.ts`).
  **Breaking change**: sinkron -> async; `computed(() => notifs.list())` (`NotificationBell.vue:7-8`)
  tidak reaktif dan tidak akan selamat — diganti Pinia store.
- **`stores/notifications.ts`** meniru `stores/inbox.ts`: `{items, unreadCount}`, `refresh()`, tanpa
  polling. Refresh dari choke-point `useAuthApi.ts:56` + setelah mark-read.
- **`constants/notificationMeta.ts`**: peta `type -> {icon, iconBg, iconColor, i18nKey, link}` —
  kelas Tailwind pindah dari data (`mock/notifications.ts:15-21`) ke katalog (pola `approvalMeta`).
- **Waktu relatif**: `formatRelativeTime()` (`utils/format.ts:45-68`) menggantikan subtree
  `notifications.time.*`.
- **i18n**: `title/markRead/viewAll/empty` dipertahankan; `item.*` diganti versi berparameter
  (`"Maintenance {asset} jatuh tempo {when}"`); `time.*` dihapus.
- **Klik baris** = mark-read + navigasi ke entitas. **Link "Lihat semua"** (`:84-89`, kini hanya
  menutup popover) -> `/notifications`.
- **Halaman `/notifications`**: daftar penuh, filter, paginasi server-side, state
  kosong/loading/error, "Tandai semua dibaca".
- **Nav & breadcrumb**: `appNav` ada di `utils/nav.ts` (**bukan** `constants/nav.ts`); tanpa entri,
  breadcrumb jatuh ke "Inventra" (`AppTopbar.vue:28`). Entri tanpa `permission`.

## 11. Membunuh `app/mock/`

Setelah wiring, **seluruh `frontend/app/mock/` jadi kode mati** — `useNotifications.ts` satu-satunya
konsumen produksi; `createStore`/`fakeLatency`/`filterBy`/`paginate`/`generateId` tidak punya
pemanggil produksi, hanya dihidupkan tesnya sendiri (sirkular).

- Hapus: `app/mock/` (3 file), `test/unit/notifications-mock.spec.ts`, `test/unit/mock-helpers.spec.ts`,
  `test/unit/mock-store.spec.ts`
- Tulis ulang: `test/nuxt/AppTopbar.spec.ts` (`beforeEach:21-25` me-reset store mock; kasus
  `:116-141`, `:167-179` menyasar mock)

## 12. Strategi tes

- **Backend unit**: resolusi penerima (maker tersaring, approver duplikat tersaring, luar-scope
  tersaring); serialisasi.
- **Backend handler**: keempat endpoint + **isolasi antar-user** (A tidak bisa baca/tandai milik B).
- **Backend integration**: relay (outbox -> stream), consumer (**at-least-once: proses dua kali ->
  satu notifikasi**), `XAUTOCLAIM` pesan tersangkut, sweeper idempoten (dua `Tick()` -> satu),
  purge, dan **rollback tx bisnis -> tidak ada outbox row**.
- **Frontend**: unit store + meta (preseden `test/nuxt/inbox-store.spec.ts:6-18`); `mountSuspended`
  untuk bel + halaman (kosong/loading/error/populated, badge 0 vs >0, mark-read, klik baris).
- **E2E**: submit -> approver menerima notifikasi -> mark-read bertahan setelah reload.
- Sesuai CLAUDE.md: proaktif dan ekspansif, bukan hanya happy path.

## 13. Batasan

- **Selalu**: `user_id = caller` di setiap verb; outbox se-transaksi dengan perubahan bisnis; i18n
  untuk semua teks; gate lengkap sebelum commit.
- **Tanya dulu**: deviasi mockup; perubahan skema di luar `000034`; dependency baru.
- **Jangan pernah**: memakai broker sebagai penyimpanan (state permanen di DB); menyimpan teks jadi
  di DB; polling di frontend; menduplikasi aturan SoD di SQL; meng-edit `db/sqlc/` dengan tangan.

## 14. Kriteria sukses

1. Bel menampilkan notifikasi nyata dari ketiga sumber mockup; badge = unread sebenarnya.
2. Mark-read bertahan setelah reload dan lintas sesi.
3. **Tidak ada event hilang**: rollback tx -> tidak ada outbox; Redis mati -> relay mengirim ulang.
4. Consumer at-least-once tidak menduplikasi notifikasi (dibuktikan tes).
5. User A tidak bisa membaca/menandai notifikasi user B (dibuktikan tes).
6. `frontend/app/mock/` terhapus; tidak ada importer `~/mock` tersisa.
7. Halaman `/notifications` cocok dengan bahasa visual App Shell, terang & gelap.
8. Gate hijau: `go build/vet/test` + integration, Spectral, `pnpm lint/typecheck/test/build`, e2e.
9. ADR + PRD + DATABASE.md + PROGRESS.md ter-update.

## 15. Deviasi yang perlu persetujuan (catat-deviasi)

1. **Halaman `/notifications` tanpa mockup** — disetujui user 2026-07-17.
2. **Subtree i18n `notifications.item.*`/`time.*` diganti** — kunci lama meng-hardcode nama
   ("Toyota Avanza", "INV-2024-0312"), mustahil dipakai data nyata.
3. **Storage Postgres, bukan Redis** — menyimpang PRD A1b. ADR + update PRD/DATABASE.md.
4. **Lock Postgres advisory, bukan Redis** — menyimpang saran PRD baris 458; mengikuti preseden
   repo (`depreciation.sql:3-5`).
5. **Redis Streams sebagai transport + AOF dinyalakan** — komponen baru di jalur runtime; PRD baris
   443 hanya menyebut Redis untuk cache/session/ratelimit/notifikasi, tanpa menyebut Streams.
6. **Enqueue di service, bukan handler** — menyimpang dari preseden `audit.Record` yang best-effort
   di handler; dituntut oleh outbox transaksional (lihat bagian 5).

## 16. Pertanyaan terbuka

Tidak ada yang memblokir. Follow-up tercatat (tidak memblokir):
1. **Pertumbuhan disk** — index partial menjaga latensi; job arsip/partisi menyusul.
2. **SSE** — refresh event-driven dulu; SSE perlu verifikasi buffering di balik Caddy + Coraza WAF.
3. **Kanal email** — consumer group kedua di stream yang sama; tidak dibangun di iterasi ini.
4. **Kebijakan retensi korporat** — 90 hari adalah default teknis; idealnya diselaraskan dengan
   records retention schedule internal bank dan divalidasi ke compliance. Karena itu dibuat config.
