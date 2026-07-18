# Dokumentasi Mobile — Inventra Field Companion

Semua dokumen aplikasi mobile companion (Flutter) dikelompokkan di folder ini, terpisah dari
dokumentasi web agar mudah dibaca. Kode aplikasinya akan berada di `mobile/` (root repo) mulai
fase M0.

## Peta dokumen

| Dokumen | Isi | Status |
|---|---|---|
| [PRD.md](PRD.md) | Kebutuhan produk mobile: goals/non-goals, persona, FR-M1 sampai FR-M6, NFR, asumsi | v1.0 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Arsitektur klien: struktur folder feature-first, Riverpod, go_router, jaringan/kontrak API, offline opname, auth, push, tema/i18n, observability, peta testing | Aktif |
| [CONVENTIONS.md](CONVENTIONS.md) | Coding style Dart, lint, penamaan, kode generated, error handling, konvensi tes, git, keamanan | Aktif |
| [DESIGN_BRIEF.md](DESIGN_BRIEF.md) | Prompt kit mockup: master brief mobile + component library + 12 prompt per-layar (untuk Claude design) | Siap dipakai |
| [adr/](adr/) | ADR mobile — 0015 (Flutter) + 0016 (offline sync); penomoran global, indeks induk di `docs/adr/README.md` | Accepted |
| `design/` | Hasil mockup per layar (`<Nama Layar>.dc.html`) — sumber kebenaran visual sebelum layar dibangun | Belum digenerate |

Rujukan lintas-dokumen: roadmap fase M0-M6 di
`docs/superpowers/plans/2026-07-18-mobile-app-roadmap.md`; domain (peran, aturan bisnis,
regulasi) tetap otoritatif di [PRD web](../PRD.md); status pengerjaan di `docs/PROGRESS.md`
bagian *Mobile companion*.

## Dokumen yang disarankan menyusul (dibuat saat fasenya tiba)

- **SETUP.md** — onboarding developer: versi Flutter SDK (di-pin), emulator/perangkat,
  menjalankan backend compose + seeded admin, perintah harian. Dibuat di **M0** bersama scaffold
  (nilainya baru nyata setelah proyek Flutter ada).
- **TESTING.md** — rincian strategi tes per lapisan bila peta di ARCHITECTURE bagian 10 mulai
  terasa kurang; sebelum itu, bagian tersebut cukup. Evaluasi di **M1**.
- **RELEASE.md** — signing, versioning (semver + build number), jalur Firebase App Distribution,
  checklist rilis + rollback. Dibuat di **M6**; runbook operasionalnya dicermin ke vault Ops.
- **Katalog error API** (level repo, bukan mobile saja) — daftar bentuk error `/api/v1` dan
  artinya per endpoint, dipakai web dan mobile agar mapping `AppFailure` tidak menebak. Paling
  pas ditulis dari `backend/api/openapi.yaml` saat fase M0-M1.
