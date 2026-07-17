# Implementation Plan: Modul Notifikasi

Spec: `docs/superpowers/specs/2026-07-17-notifications-design.md`
Branch: `feat/notifications`
Tanggal: 2026-07-17

> Catatan konvensi: skill `planning-and-task-breakdown` meminta output ke `tasks/plan.md` +
> `tasks/todo.md`. Konvensi repo ini (CLAUDE.md + riwayat `docs/superpowers/plans/`) menang.

## Ringkasan

Pipeline notifikasi in-app dari nol: outbox transaksional -> Redis Stream -> consumer fan-out ->
feed per-user, dengan sweeper untuk pengingat maintenance + purge retensi, lalu mengganti mock
frontend dan menghapus `app/mock/` seluruhnya.

## Keputusan arsitektural (dari spec)

- **DB state permanen, broker transport.** Redis Stream hanya mengantar `outbox` -> `notifications`.
  Redis hilang total = tidak ada state hilang; relay kirim ulang dari outbox.
- **Outbox se-transaksi** dengan perubahan bisnis -> enqueue pindah dari handler ke **service**.
  `approval.Notifier` tidak diperlukan; import cycle lenyap.
- **Invers kelayakan menyaring lewat `eligibleToDecide` yang sudah ada** — aturan SoD tidak
  diduplikasi di SQL.
- **At-least-once** ditangani `uq_notif_dedup` + `ON CONFLICT DO NOTHING`.
- **`pg_advisory_xact_lock`**, bukan lock Redis (preseden `depreciation.sql:3-5`).
- **Teks dirender klien** dari `type` + `params`.

## Urutan dependensi

```
000034 (outbox + notifications)
   |
   +-- queries + sqlc
          |
          +-- Service + handler + routes + OpenAPI     <- Fase 1: API hidup, feed kosong
                 |
                 +-- AOF + relay + enqueue Decide + consumer(request_decided)
                 |                                      <- Fase 2: notifikasi NYATA pertama
                 +-- NotifiableApprovers -> submit/chain -> auto-resolve -> checkin
                 |                                      <- Fase 3: fan-out rumit
                 +-- sweeper (due -> outbox) + purge + wiring main.go
                 |                                      <- Fase 4
                 +-- store/bel/halaman -> bunuh mock    <- Fase 5
                        |
                        +-- e2e + ADR/docs              <- Fase 6
```

Irisan **vertikal**: Fase 2 sudah menghasilkan notifikasi nyata end-to-end lewat seluruh pipeline
(tx -> outbox -> relay -> stream -> consumer -> tabel) memakai event **termudah** (penerima konkret).
Jalur transport terbukti sebelum fan-out rumit disentuh, dan sebelum frontend ada.

---

## Fase 1: Fondasi

### Task 1: Migrasi 000034
**Deskripsi:** Skema `notification`, enum `shared.notification_type`, tabel `outbox` +
`notifications`, 4 index, trigger `set_updated_at`.
**Acceptance:**
- [ ] `up` lalu `down 1` bersih (down membuang tabel, enum, skema)
- [ ] Semua index partial `WHERE deleted_at IS NULL` per spec bagian 6
- [ ] `uq_notif_dedup` menolak duplikat `(user_id, dedup_key)` saat `dedup_key` tidak NULL
**Verify:** `migrate ... up && migrate ... down 1`; `psql \d+`
**Dependencies:** None · **Scope:** S (2 file)

### Task 2: Queries + sqlc generate
**Deskripsi:** `db/queries/notification.sql` (enqueue outbox, klaim unpublished `FOR UPDATE SKIP
LOCKED`, mark published, insert notif `ON CONFLICT DO NOTHING`, list/count per-user, mark
read/read-all, soft-delete by dedup_key, purge) + `ListUsersWithPermission` (authz.sql) +
`ListSchedulesDueBetween` (maintenance.sql — `report.sql:76` tak terpakai karena `LIMIT 3`).
**Acceptance:**
- [ ] Setiap query feed memfilter `user_id` — tidak ada yang lintas-user
- [ ] Insert notif idempoten via `ON CONFLICT DO NOTHING` pada `uq_notif_dedup`
- [ ] `sqlc generate` bersih; `db/sqlc/` tidak disentuh tangan
**Verify:** `sqlc generate && go build ./...`
**Dependencies:** 1 · **Scope:** S (4 file)

### Task 3: Service + handler + routes + OpenAPI
**Deskripsi:** Modul empat-file, 4 endpoint (spec bagian 9), semua `RequireAuth` tanpa
`RequirePermission`.
**Acceptance:**
- [ ] `GET /notifications` -> `{data,total,limit,offset}`, clamp 1-100, filter `read`
- [ ] `GET /notifications/unread-count` -> `{count}`
- [ ] Mark-read milik user lain -> **404, bukan 403**
- [ ] Handler test menutup isolasi antar-user secara eksplisit
**Verify:** `go test ./internal/notification/...`; Spectral
**Dependencies:** 2 · **Scope:** M (5 file)

### Checkpoint: Fondasi
- [ ] `go build/vet/test` hijau; Spectral hijau
- [ ] API bisa dipanggil manual (feed kosong — belum ada produsen, itu benar)

---

## Fase 2: Transport + notifikasi nyata pertama

### Task 4: AOF Redis
**Deskripsi:** `--appendonly yes` di `docker-compose.yml:26`, `docker-compose.dev.yml`,
`docker-compose.prod.yml`. Default `redis:7-alpine` hanya RDB; stream butuh durabilitas.
**Acceptance:**
- [ ] Ketiga compose konsisten; stack naik sehat
- [ ] `redis-cli CONFIG GET appendonly` -> `yes`
**Verify:** `docker compose -f docker-compose.dev.yml up -d && docker exec inventra-redis redis-cli CONFIG GET appendonly`
**Dependencies:** None (paralel dengan 1-3) · **Scope:** XS (3 file)

### Task 5: `relay.go`
**Deskripsi:** Klaim outbox unpublished -> `XADD` -> tandai `published_at`. Menyalin
`importer/worker.go`: `NewRelay`, `Run` (ticker `NOTIFICATION_RELAY_POLL`), **`Tick(ctx)` diekspor**
untuk tes deterministik (`worker.go:150`).
**Acceptance:**
- [ ] Baris outbox terkirim tepat sekali per `Tick` yang sukses; `published_at` terisi
- [ ] `XADD` gagal -> `published_at` tetap NULL -> tick berikutnya mencoba lagi (tidak hilang)
- [ ] `FOR UPDATE SKIP LOCKED` -> dua relay paralel tidak mengirim baris yang sama
- [ ] Stream di-trim `NOTIFICATION_STREAM_MAXLEN` (transport, bukan penyimpanan)
**Verify:** `go test -tags=integration ./internal/notification/...`
**Dependencies:** 2, 4 · **Scope:** M (3-4 file)

### Task 6: Enqueue di `approval.Service.Decide` -> `request_decided`
**Deskripsi:** Insert outbox **di dalam tx** decide (`approval/service.go`, cabang ditolak `:311`
dan disetujui final `:353`). Produsen termudah dulu: penerima = `requests.maker_id`, sudah konkret.
**Acceptance:**
- [ ] Approve & reject sama-sama meng-enqueue, `payload` membedakan hasilnya
- [ ] **Rollback tx bisnis -> tidak ada baris outbox** (dibuktikan tes)
- [ ] `approval` tidak meng-import `notification` — cek dengan
      `go list -deps ./internal/approval | grep "inventra/internal/notification"`. **Grep polos
      `notification` tidak berguna**: dia kena `go-redis/v9/maintnotifications` yang ditarik
      transitif, jadi selalu positif palsu.
**Verify:** `go test ./internal/approval/...`
**Dependencies:** 2 · **Scope:** S (2-3 file)

### Task 7: `consumer.go` + handler `request_decided` -> `approval_decided`
**Deskripsi:** `XREADGROUP` (group `notification-fanout`) -> resolve penerima -> insert notifikasi
-> `XACK`. `XAUTOCLAIM` untuk pesan tersangkut karena consumer mati.
**Acceptance:**
- [ ] **At-least-once aman**: proses pesan yang sama dua kali -> satu notifikasi (via `uq_notif_dedup`)
- [ ] Pesan gagal tidak di-ack -> tetap di PEL -> `XAUTOCLAIM` mengambil alih
- [ ] Maker menerima `approval_decided` end-to-end lewat seluruh pipeline
**Verify:** `go test -tags=integration ./internal/notification/...`
**Dependencies:** 5, 6 · **Scope:** M (3-4 file)

### Checkpoint: Transport terbukti
- [ ] Notifikasi nyata pertama lahir lewat tx -> outbox -> relay -> stream -> consumer -> tabel
- [ ] `go test -tags=integration ./... -p 1` hijau
- [ ] **Review dengan user sebelum lanjut** — jalur terpanjang & terisiko sudah terbukti di sini

---

## Fase 3: Fan-out rumit

### Task 8: `NotifiableApprovers` — membangun invers
**Deskripsi:** **Tugas paling berisiko.** `ListUsersWithPermission('request.decide')` lalu saring di
Go dengan `eligibleToDecide` (`approval/service.go:172`), membangun `Caller` per kandidat lewat
`scopeSvc`.
**Acceptance:**
- [ ] Maker tidak pernah masuk hasil (SoD), approver sebelumnya tidak masuk lagi — **karena predikat
      lama dipakai apa adanya**, bukan karena SQL meniru aturannya
- [ ] Kandidat di luar scope kantor tier tersaring
- [ ] Unit test menutup ketiga penyaringan itu eksplisit
**Verify:** `go test ./internal/approval/...`
**Dependencies:** 2 · **Scope:** M (3-4 file)

### Task 9: Enqueue submit + rantai maju -> `approval_pending`
**Deskripsi:** Enqueue di tx `Submit` (`service.go:99-133`, sebelum commit `:131`) dan di cabang
rantai maju (`:342`). Consumer memakai Task 8 untuk resolve penerima.

Dua event terpisah (`request_submitted`, `chain_advanced`) dengan satu payload
`approval.RequestPendingEvent` (`request_id` + `request_type` + `step`) dan satu handler consumer:
fan-out in-app-nya identik, tapi keduanya fakta bisnis berbeda sehingga konsumen email nanti bisa
membedakannya. Payload tidak memuat penerima — sesuai bagian 5, penerima diresolve di consumer.
Consumer memanggil `approval.Service.ApproversForStep(ctx, requestID, step)` (wrapper Task 8 yang
membaca request + baris step-nya) lewat interface sempit `notification.ApproverResolver` (preseden
`importer.Submitter`).

**Event basi wajib dilewati, bukan sekadar detail:** `uq_notif_dedup` partial `WHERE deleted_at IS
NULL`, jadi baris yang sudah di-soft-delete Task 10 **tidak** menghalangi insert ulang. Tanpa
pengecekan status, event yang dikirim ulang akan menghidupkan lagi `approval_pending` untuk step
yang sudah lewat. `ApproversForStep` karena itu mengembalikan `ErrStepPassed` bila request tidak
lagi `pending` di step itu (sudah maju/ditolak/final/dibatalkan) dan consumer meng-ack tanpa menulis
apa pun.

Fan-out N baris per event **tanpa transaksi**: tiap insert idempoten sendiri lewat dedup key, jadi
kegagalan di tengah menyisakan baris yang sudah benar dan redelivery menambal sisanya. Transaksi
hanya akan me-rollback notifikasi yang sebenarnya sudah benar.
**Acceptance:**
- [ ] `dedup_key = 'request:<id>:step:<n>'` terisi
- [ ] Approver berhak menerima; maker tidak
- [ ] Rantai maju menghasilkan `approval_pending` untuk step baru
- [ ] Event basi (request sudah diputus/dibatalkan) di-ack tanpa menulis notifikasi
**Verify:** `go test -tags=integration ./internal/notification/... ./internal/approval/...`
**Dependencies:** 7, 8 · **Scope:** M (3-4 file)

### Task 10: Auto-resolve notifikasi basi
**Deskripsi:** Saat giliran step lewat — rantai maju (`:342`), ditolak (`:311`), final (`:353`),
dibatalkan (`:377`) — soft-delete semua `approval_pending` step itu via `dedup_key`.

**JEBAKAN — `SoftDeleteNotificationsByDedupPrefix` (`LIKE prefix || '%'`) berbahaya di sini.**
`dedup_key` berbentuk `request:<id>:step:<n>`, jadi prefix `request:<id>:step:1` **juga menyapu
step 10, 11, 12...** — `step:1` adalah awalan dari `step:10`. Yang benar:
- **rantai maju** (bersihkan satu step): **exact match** `dedup_key = 'request:<id>:step:<n>'`.
  Semua penerima step itu berbagi dedup_key yang sama (hanya `user_id` yang beda), jadi exact match
  sudah menyapu semuanya — prefix tidak pernah dibutuhkan. Butuh query exact baru.
- **terminal** (ditolak/final/dibatalkan — bersihkan semua step): prefix `request:<id>:step:`
  **dengan titik dua di ujung**. Aman: tidak menyentuh `request:<id>:decided`, dan mencakup semua step.

Catatan Task 9 yang jadi tanggungan Task 10: `ApproversForStep` mengembalikan `ErrStepPassed` untuk
step yang sudah lewat, dan consumer meng-ack tanpa menulis. Itu **wajib ada**, karena
`uq_notif_dedup` partial pada `deleted_at IS NULL` — baris ter-soft-delete tidak memblokir insert
ulang, sehingga event yang dikirim ulang akan menghidupkan kembali notifikasi yang baru dibersihkan.

**Acceptance:**
- [ ] Keempat cabang membersihkan step yang lewat
- [ ] Approver yang belum sempat lihat tidak lagi melihatnya di feed
- [ ] **Step 1 dibersihkan tidak ikut menghapus step 10+** (uji eksplisit dengan rantai >= 10 step,
      atau buktikan exact-match dipakai)
- [ ] Notifikasi `approval_decided` (`request:<id>:decided`) TIDAK ikut tersapu saat terminal
- [ ] Audit log tidak tersentuh
**Verify:** `go test ./internal/approval/...`
**Dependencies:** 9 · **Scope:** S (2-3 file)

### Task 11: Enqueue check-in -> `asset_returned`
**Deskripsi:** Enqueue di tx `assignment.Service.Checkin` (`service.go:192-203`). Penerima =
`assigned_by_id` — **tidak perlu query baru** (celah `GetUserByEmployeeID` dihindari, spec bagian 4).
**Acceptance:**
- [ ] Check-in menotifikasi user yang meng-check-out
- [ ] Check-in oleh orang yang sama -> tidak menotifikasi diri sendiri
- [ ] `params` memuat `asset_tag` + nama aset untuk interpolasi i18n
**Verify:** `go test ./internal/assignment/... ./internal/notification/...`
**Dependencies:** 7 · **Scope:** S (2-3 file)

### Checkpoint: Fan-out
- [ ] `go test ./...` + `go test -tags=integration ./... -p 1` hijau
- [ ] Tiga jenis terbukti lahir end-to-end

---

## Fase 4: Sweeper

### Task 12: `sweeper.go` — `maintenance_due` + purge
**Deskripsi:** Scan `ListSchedulesDueBetween` -> **enqueue outbox** (bukan tulis langsung, supaya
jalur seragam & consumer email nanti ikut menerima). Purge soft-delete >
`NOTIFICATION_RETENTION_DAYS` (notifikasi + outbox yang sudah published). Lock
`pg_advisory_xact_lock(hashtext('notification.sweep'))`.
**Acceptance:**
- [ ] **Idempoten**: dua `Tick()` -> satu notifikasi (via `dedup_key = 'schedule:<id>:due:<date>'`)
- [ ] Purge menghilangkan baris dari feed & count
- [ ] Lock mencegah dua instance menyapu bersamaan
- [ ] Penerima = `maintenance.manage` dalam scope kantor aset (memakai ulang invers Task 8)
**Verify:** `go test -tags=integration ./internal/notification/...`
**Dependencies:** 8, 11 · **Scope:** M (4 file)

### Task 13: Wiring `main.go` + config
**Deskripsi:** `NOTIFICATION_WORKER_ENABLED` (true), `NOTIFICATION_RELAY_POLL` (2s),
`NOTIFICATION_SWEEP_POLL` (1h), `NOTIFICATION_RETENTION_DAYS` (90), `NOTIFICATION_STREAM_MAXLEN`.
Relay + consumer + sweeper dijalankan dari `cmd/api/main.go` mengikuti `importWorker`
(`main.go:95-100`), ikut `workerCtx`/`stopWorker` (`main.go:107`).
**Acceptance:**
- [ ] Graceful shutdown: ketiganya berhenti sebelum `srv.Shutdown`
- [ ] `NOTIFICATION_WORKER_ENABLED=false` -> tidak satu pun jalan
- [ ] `NewRouter` mengembalikannya (pola sama `importer.Worker`)
**Verify:** `go build ./... && go vet ./...`; jalankan stack, cek log start/stop
**Dependencies:** 12 · **Scope:** S (3 file)

### Checkpoint: Backend selesai
- [ ] Gate backend penuh hijau: build/vet/test + integration + Spectral
- [ ] Keempat jenis terbukti lahir

---

## Fase 5: Frontend

### Task 14: `useNotifications` nyata + store + meta
**Deskripsi:** Tulis ulang ke `useApiClient().request` (preseden `useApproval.ts:54-99`) —
**breaking change** sinkron -> async. `stores/notifications.ts` meniru `stores/inbox.ts`.
`constants/notificationMeta.ts` memetakan `type -> {icon, iconBg, iconColor, i18nKey, link}`.
**Acceptance:**
- [ ] Refresh dari choke-point `useAuthApi.ts:56` (login, OAuth, restore sesi)
- [ ] Tanpa polling; gagal fetch mempertahankan count terakhir (preseden `inbox.ts`)
- [ ] Tidak toast sendiri — `useApiClient` sudah menanganinya
**Verify:** `pnpm test`
**Dependencies:** 3 · **Scope:** M (4-5 file)

### Task 15: `NotificationBell.vue` nyata
**Deskripsi:** Pertahankan `UPopover`. Ganti `computed(() => notifs.list())` yang tidak reaktif
dengan store. Waktu relatif lewat `formatRelativeTime()` (`utils/format.ts:45-68`). Klik baris =
mark-read + navigasi. "Lihat semua" -> `/notifications`.
**Acceptance:**
- [ ] Badge = unread nyata; hilang saat 0
- [ ] State kosong/loading/error tertangani
- [ ] `mountSuspended` test menutup semua state
- [ ] Cocok mockup App Shell, terang & gelap
**Verify:** `pnpm test`; banding visual `docs/design/App Shell.dc.html`
**Dependencies:** 14 · **Scope:** M (3-4 file)

### Task 16: Halaman `/notifications` + nav + i18n
**Deskripsi:** Daftar penuh: filter, paginasi server-side, "Tandai semua dibaca", state
kosong/loading/error. Entri `appNav` (`utils/nav.ts`, **bukan** `constants/nav.ts`) tanpa
`permission` — tanpa entri breadcrumb jatuh ke "Inventra" (`AppTopbar.vue:28`). `item.*` diganti
versi berparameter; `time.*` dihapus.
**Acceptance:**
- [ ] Semua string di `i18n/locales/{id,en}.json`
- [ ] `test/unit/nav-model.spec.ts` diperbarui
- [ ] `mountSuspended` test menutup filter, paginasi, mark-all, ketiga state
**Verify:** `pnpm lint && pnpm typecheck && pnpm test && pnpm build`
**Dependencies:** 15 · **Scope:** M (5 file)

### Task 17: Bunuh `app/mock/`
**Deskripsi:** Hapus `app/mock/` (3 file) + `test/unit/{notifications-mock,mock-helpers,mock-store}.spec.ts`.
Tulis ulang `test/nuxt/AppTopbar.spec.ts` (`beforeEach:21-25`, kasus `:116-141`, `:167-179`).
**Acceptance:**
- [ ] `grep -r "~/mock" frontend/` -> nol hasil
- [ ] `pnpm test` hijau; yang hilang hanya tes mock itu sendiri
**Verify:** `pnpm test && pnpm typecheck`
**Dependencies:** 16 · **Scope:** M (7 file, mayoritas penghapusan)

### Checkpoint: Frontend selesai
- [ ] `pnpm lint/typecheck/test/build` hijau
- [ ] Banding 1:1 `docs/design/App Shell.dc.html`, terang & gelap
- [ ] Tidak ada `~/mock` tersisa

---

## Fase 6: E2E + dokumen

### Task 18: E2E backend-nyata
**Deskripsi:** `frontend/e2e/notifications.spec.ts`: submit -> approver menerima notifikasi ->
mark-read bertahan setelah reload. Pipeline-nya **asinkron** — e2e harus menunggu notifikasi muncul
(poll UI/`expect` auto-wait), bukan mengasumsikan langsung ada.
**Acceptance:**
- [ ] Dua user (maker != approver) — pola ada di `assets.spec.ts`
- [ ] Nama unik per run; tidak meninggalkan data yang merusak run berikutnya
- [ ] Kalau menyentuh state bersama: cleanup failure-safe via `afterEach`/API
**Verify:** `pnpm test:e2e` (butuh stack + seeded admin, `RATELIMIT_ENABLED=false`)
**Dependencies:** 17 · **Scope:** S (1-2 file)

### Task 19: ADR + PRD + DATABASE.md + PROGRESS.md
**Deskripsi:** ADR (MADR) yang **men-supersede PRD A1b untuk notifikasi**: Postgres sumber
kebenaran + Redis Streams sebagai transport (bukan penyimpanan) + lock advisory Postgres. Update
PRD (A1b/A2, baris 443/458/511), DATABASE.md (skema, data dictionary, catatan purge soft-delete),
PROGRESS.md (centang + deviasi + follow-up).
**Acceptance:**
- [ ] ADR menyebut alternatif yang ditolak + alasannya (Redis-only ephemeral; Kafka/RabbitMQ vs VPS
      4GB; Streams tanpa outbox = dual-write)
- [ ] Keenam deviasi spec bagian 15 tercatat di PROGRESS.md
- [ ] Follow-up tercatat: disk, SSE, kanal email, retensi korporat
- [ ] PROGRESS.md "Next session" menunjuk langkah nyata berikutnya
**Verify:** baca ulang; tanpa simbol tipografis per konvensi
**Dependencies:** 18 · **Scope:** M (4-5 file)

### Checkpoint: Selesai
- [ ] Gate penuh: `go build/vet/test` + integration + Spectral + `pnpm lint/typecheck/test/build` + e2e
- [ ] Kesembilan kriteria sukses spec bagian 14 terpenuhi
- [ ] Review seluruh branch sebelum PR

---

## Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Invers kelayakan salah -> notifikasi bocor ke yang tak berhak | **Tinggi** | Saring lewat `eligibleToDecide` yang sudah ada, jangan tiru di SQL. Tes: maker, approver duplikat, luar-scope. Task 8 dikerjakan awal (fail fast) |
| Isolasi antar-user bocor di endpoint | **Tinggi** | `user_id = caller` di setiap verb; tes isolasi wajib Task 3; mark-read milik orang lain -> 404 |
| Event hilang antara commit dan publish | **Tinggi** | Outbox se-transaksi; `published_at` diisi hanya setelah `XADD` sukses; tes rollback -> tanpa outbox |
| Consumer at-least-once menduplikasi notifikasi | Sedang | `uq_notif_dedup` + `ON CONFLICT DO NOTHING`; tes proses-dua-kali |
| Consumer mati -> pesan tersangkut di PEL | Sedang | `XAUTOCLAIM`; tes pesan tersangkut |
| Redis crash kehilangan stream | Sedang | AOF (Task 4); outbox tetap sumber replay |
| `useNotifications` async memecah konsumen lain | Sedang | Preseden memori: rewire mock->HTTP memecah tes konsumen. Grep konsumen sebelum ubah; cek exit code suite penuh |
| E2E flaky karena pipeline asinkron | Sedang | Auto-wait `expect`, bukan asumsi instan; Task 18 |
| N resolusi scope per event lambat | Rendah | Scope ter-cache Redis; N kecil. Diukur, bukan ditebak |
| Tabel tumbuh tanpa batas | Rendah | Index partial menjaga latensi; disk ditangani job arsip (follow-up) |

## Paralelisasi

- **Bisa paralel sekarang:** Task 4 (AOF) independen dari 1-3.
- **Harus berurutan:** 1 -> 2 -> 3; 5 -> 7; 8 -> 9 -> 10.
- **Bisa paralel setelah Task 7:** Task 11 (check-in) independen dari jalur approval.
- **Bisa paralel setelah Task 3:** Task 14 (frontend store) hanya butuh kontrak API.
- **Harus terakhir:** Task 17 (hapus mock) — menyentuh tes yang dipakai task lain.

## Pertanyaan terbuka

Tidak ada. Semua keputusan tercatat di spec bagian 2.
