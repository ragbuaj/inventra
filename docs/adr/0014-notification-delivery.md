# ADR-0014 — Pengiriman notifikasi: transactional outbox + Redis Streams

- Status: Accepted
- Date: 2026-07-18
- Deciders: pemilik proyek + sesi desain modul notifikasi
- Menyempurnakan: [ADR-0010](0010-background-job-execution.md) (kontrak job latar). Men-supersede
  pernyataan PRD A1b/baris 458 sejauh menyangkut notifikasi (lihat "Konsekuensi").

## Konteks

Modul notifikasi in-app (FR-4.5 maintenance jatuh tempo, FR-6.5 approval) adalah mock terakhir di
app-shell frontend. Empat jenis notifikasi diperlukan: `approval_pending` (ke approver berhak),
`approval_decided` (ke maker), `maintenance_due` (ke pengelola maintenance dalam scope), dan
`asset_returned` (ke yang meng-check-out).

PRD menyebut dua hal yang **saling bertentangan**: baris 458 menjadikan Redis "backing store
notifikasi in-app", sementara catatan tepat di bawahnya (dan A1b) menyatakan Redis "bersifat
pelengkap, bukan sumber kebenaran; kehilangan Redis tidak menyebabkan kehilangan data". Menjadikan
Redis satu-satunya penyimpan feed yang dibaca user melanggar prinsip PRD-nya sendiri: notifikasi
punya state yang harus bertahan (`read_at`) dan dibaca lintas sesi.

Tiga tujuan yang diminta dari pipeline: fan-out keluar dari request path, durabilitas + retry, dan
konsumen ganda untuk integrasi masa depan (kanal email). Sekaligus, satu prinsip yang ditegaskan
pemilik proyek: **state permanen ada di DB, broker bukan penyimpanan** — event penting sudah terekam
permanen di audit log lebih dulu, jadi notifikasi hanyalah turunan yang boleh dibersihkan agresif.

## Keputusan

**Transactional outbox (Postgres) + Redis Streams sebagai transport.** Postgres adalah sumber
kebenaran; Redis Stream hanya mengantar antara dua tabel Postgres.

1. **Outbox se-transaksi.** Handler bisnis (`approval.Submit`/`Decide`, `assignment.Checkin`) menulis
   baris `notification.outbox` di transaksi yang **sama** dengan perubahan bisnisnya. Ini menutup
   dual-write: rollback tidak meninggalkan event yatim, commit tidak pernah kehilangan event. Enqueue
   karenanya berada di **service**, bukan handler — berbeda dari preseden `audit.Record` yang
   post-commit dan best-effort. Kalau enqueue gagal, tx bisnis rollback; itu benar, karena insert ke
   tabel lokal di tx yang sama hanya gagal saat DB mati, dan saat itu operasi bisnisnya toh gagal.
2. **Relay** mem-poll outbox (`FOR UPDATE SKIP LOCKED`, preseden `importer/worker.go`), `XADD` ke
   stream, lalu menandai `published_at` **hanya** setelah XADD sukses. Gagal publish = baris tetap
   belum tertandai, tick berikutnya mencoba lagi.
3. **Consumer group** (`XREADGROUP` + `XACK`) mem-fan-out ke baris `notification.notifications`.
   At-least-once; duplikat ditangani index partial-unique `uq_notif_dedup` + `ON CONFLICT DO NOTHING`.
   Pesan tersangkut karena consumer mati diambil `XAUTOCLAIM`.
4. **Kontrak job latar ADR-0010 dipenuhi.** Ini adalah Tahap 3 ADR-0010 untuk notifikasi (ticker
   in-process + advisory lock). Sweeper `maintenance_due` idempoten, dibungkus
   `pg_advisory_xact_lock(hashtext('notification.sweep'))` (preseden `depreciation.sql`), dan dipicu
   ticker in-process yang di-adapter dari `cmd/api/main.go` — service domain tak tahu pemicunya.
5. **Fan-out lewat invers kelayakan yang sudah ada.** Menentukan "siapa approver yang berhak" memakai
   `ListUsersWithPermission` lalu menyaring tiap kandidat lewat `eligibleToDecide` yang sudah ada —
   aturan SoD/scope **tidak diduplikasi** di SQL, supaya "siapa yang dinotifikasi" tidak bisa
   melenceng dari "siapa yang boleh memutuskan".
6. **Teks dirender klien.** Baris menyimpan `type` + `params` (jsonb), tidak pernah kalimat jadi —
   i18n wajib, dan menyimpan kalimat Indonesia akan mematikan pergantian locale.
7. **AOF Redis dinyalakan** (`--appendonly yes --appendfsync everysec`). Default `redis:7-alpine`
   hanya RDB; stream sebagai transport butuh durabilitas. Outbox tetap sumber replay kalau stream
   hilang, jadi jendela AOF ~1 detik bukan lubang data.

## Alternatif yang ditolak

- **Redis-only sebagai penyimpan feed (PRD baris 458 apa adanya).** Ephemeral — notifikasi hilang saat
  Redis evict/restart tanpa persistence, dan bertentangan dengan pernyataan PRD sendiri bahwa Redis
  bukan sumber kebenaran. `read_at` butuh durabilitas.
- **Redis Streams tanpa outbox.** Publish ke Redis setelah commit DB adalah dual-write: commit sukses,
  publish gagal, notifikasi hilang diam-diam. Tepat lubang yang tujuan "durabilitas" ingin ditutup.
- **Kafka.** JVM + KRaft di VPS produksi 4GB yang sudah menjalankan Postgres/Redis/MinIO/backend/
  frontend/Caddy/monitoring. Skalanya tidak cocok dengan beban (FAM internal, bukan event streaming
  volume tinggi).
- **RabbitMQ.** Komponen stateful baru dengan operasionalnya sendiri (compose, Ansible, exporter/alert,
  backup) di VPS yang sama, dan tetap butuh outbox untuk menghindari dual-write.
- **Lock Redis (PRD baris 458).** Preseden repo adalah `pg_advisory_xact_lock` (ADR-0010,
  `depreciation.sql`) — tx-scoped, otomatis lepas saat commit/rollback, tanpa komponen baru.
- **Enqueue best-effort di handler (preseden `audit.Record`).** Tidak bisa menjamin "tidak ada event
  hilang" — publish post-commit bisa gagal setelah commit sukses. Outbox transaksional adalah harga
  yang benar untuk jaminan itu.

## Konsekuensi

- **Men-supersede PRD A1b/baris 458 untuk notifikasi:** Redis turun dari "backing store notifikasi"
  jadi **transport** saja. Ini menyelaraskan PRD dengan pernyataannya sendiri (Redis bukan sumber
  kebenaran). PRD dan DATABASE.md diperbarui.
- Retensi: notifikasi ephemeral, di-soft-delete > `NOTIFICATION_RETENTION_DAYS` (default 90). Karena
  semua index partial pada `deleted_at IS NULL`, baris ter-purge keluar dari index — feed dan
  unread-count tetap cepat berapa pun tabel tumbuh. Pertumbuhan disk ditangani job arsip/partisi
  (follow-up), sejalan dengan perlakuan audit trail.
- **Jalur migrasi ke broker sungguhan tetap terbuka:** outbox adalah jalurnya — worker relay tinggal
  diganti mendorong ke broker eksternal kalau konsumen lintas-sistem benar-benar muncul.
- **Konsumen ganda gratis:** kanal email masa depan cukup jadi consumer group kedua di stream yang
  sama, tanpa menyentuh produsen.
- Kanal notifikasi baru = satu handler di dispatch map consumer (keyed `event_type`) + satu titik
  enqueue di service. Frontend refresh event-driven (choke-point fetchMe), tanpa polling; SSE dicatat
  sebagai follow-up (perlu verifikasi buffering di balik Caddy/Coraza WAF).
