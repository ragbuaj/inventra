# ADR-0010 — Background job execution: staged adoption

- Status: Accepted
- Date: 2026-07-05
- Deciders: pemilik proyek + sesi desain modul depresiasi

## Context

Sistem mulai membutuhkan pekerjaan berkala: perhitungan depresiasi bulanan (modul pertama yang
membutuhkannya), pengingat maintenance jatuh tempo, notifikasi, refresh read-model OLAP, dan
pembuatan laporan berkala. Belum ada infrastruktur scheduler/cron apa pun di backend (satu-satunya
goroutine latar adalah HTTP server). Model deployment masa depan belum pasti (saat ini Docker
compose single-instance; bisa berkembang ke multi-replika/K8s).

Menanam logika bisnis di dalam mekanisme pemicu (fungsi cron berisi kalkulasi) membuat sistem sulit
diubah; memilih teknologi scheduler sekarang berarti komitmen prematur pada model deployment yang
belum ada.

## Decision

**Yang di-future-proof adalah kontrak domain, bukan pemicunya.** Setiap pekerjaan berkala dibangun
dengan empat sifat berikut sejak hari pertama, sementara mekanisme pemicunya diadopsi bertahap:

1. **Idempotent** — operasi domain (mis. `depreciation.Service.ComputePeriod(ctx, period)`) aman
   dipanggil berulang; hasil identik; tidak menggandakan data.
2. **Single execution antar-replika** — operasi dibungkus **Postgres advisory lock** (
   `pg_advisory_xact_lock`) sehingga dua pemanggil bersamaan tidak saling menimpa — terpasang
   sebelum multi-replika dibutuhkan.
3. **Observable** — eksekusi tercatat (audit log; tabel `job_runs` menyusul saat pemicu otomatis
   diadopsi) sehingga kegagalan tidak diam-diam.
4. **Pemicu pluggable** — HTTP endpoint (manusia via UI), binary CLI, ticker in-process, atau
   scheduler eksternal semuanya hanyalah *adapter* yang memanggil metode domain yang sama.

### Tahapan adopsi pemicu

- **Tahap 1 (sekarang, modul depresiasi):** pemicu manual — endpoint HTTP + tombol UI
  ("Hitung Periode"), dengan banner pengingat di layar saat periode berjalan belum dihitung.
  Penutupan periode **selalu manual** (keputusan akuntansi, bukan keterbatasan teknis).
- **Tahap 2 (saat otomatisasi diinginkan):** binary `cmd/jobs` (preseden `cmd/createadmin`) yang
  memanggil service yang sama, dijalankan scheduler eksternal sesuai lingkungan deploy (Task
  Scheduler / cron / K8s CronJob), plus tabel `job_runs` untuk riwayat.
- **Tahap 3 (saat jumlah job banyak / multi-replika nyata):** scheduler in-process ber-advisory-lock
  atau job queue Redis (mis. asynq). Service domain tidak berubah; hanya adapter bertambah.

## Consequences

- Modul depresiasi (dan job berikutnya) dapat dibangun tanpa menunggu keputusan infra.
- Perpindahan tahap tidak menyentuh logika domain — hanya menambah adapter pemicu.
- Alternatif yang ditolak untuk saat ini: ticker in-process (risiko duplikasi multi-replika +
  observabilitas buruk tanpa job_runs), lazy compute saat halaman dibuka (perhitungan akuntansi
  sebagai efek samping render — buruk untuk disiplin audit), pg_cron (logika di Go, ketersediaan
  extension bergantung hosting), job queue penuh (overkill untuk satu job bulanan).
