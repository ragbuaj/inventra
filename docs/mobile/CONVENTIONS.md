# Konvensi Kode Mobile — Inventra Field Companion

Coding style dan konvensi kerja untuk `mobile/` (Flutter/Dart). Melengkapi
[ARCHITECTURE.md](ARCHITECTURE.md) (lapisan, struktur folder, aturan ketergantungan) — dokumen ini
mengatur *bagaimana menulisnya*. Berlaku untuk semua kontribusi ke `mobile/` sejak fase M0.

## 1. Gaya bahasa Dart

- Ikuti [Effective Dart](https://dart.dev/effective-dart); format wajib `dart format` (pengaturan
  default — jangan menambah konfigurasi format kustom).
- Lint: paket **`flutter_lints`** sebagai dasar di `analysis_options.yaml`, diperketat dengan
  aturan tambahan minimum:
  - `avoid_print: true` — pakai logger `core/utils` (lihat ARCHITECTURE bagian 9).
  - `prefer_const_constructors`, `prefer_final_locals`, `unawaited_futures`,
    `always_declare_return_types`.
  - `public_member_api_docs` **tidak** diaktifkan — komentar hanya untuk hal yang tidak bisa
    diungkapkan kode (konsisten konvensi repo: komentar menjelaskan constraint, bukan mengulang
    kode).
- `flutter analyze` harus **nol** warning/error — di-gate CI, sama kedudukannya dengan
  `go vet` dan `pnpm lint`.

## 2. Penamaan

| Hal | Konvensi | Contoh |
|---|---|---|
| File | snake_case, satu deklarasi publik utama per file | `asset_detail_screen.dart` |
| Class/enum/typedef | PascalCase | `AssetDetailScreen`, `AppFailure` |
| Provider Riverpod | camelCase berakhiran `Provider` | `authControllerProvider`, `syncQueueProvider` |
| Controller | berakhiran `Controller` (`AsyncNotifier`) | `ApprovalInboxController` |
| Repository / API | berakhiran `Repository` (satu per sumber API per fitur) | `OpnameRepository` |
| DTO | berakhiran `Dto`; field **English snake_case** persis kontrak OpenAPI backend | `AssetDto.assetTag` dengan `@JsonKey(name: 'asset_tag')` |
| Screen / widget | berakhiran `Screen` untuk rute; widget lokal tanpa suffix khusus | `ScanScreen`, `SyncPill` |
| Tabel drift | jamak snake_case, mengikuti gaya penamaan DATABASE.md | `scan_queue`, `opname_items` |
| Kunci ARB (i18n) | camelCase berprefix layar/fitur | `approvalDetailApproveButton`, `opnameSyncPending` |
| Konstanta rute | path kebab/lowercase, param `:id` | `/stock-opname/:id` |

## 3. Aturan struktur (ringkas — detail di ARCHITECTURE)

- Fitur baru = folder baru di `lib/features/<fitur>/` dengan `data/` + `presentation/`; tidak ada
  impor antar-fitur; kebutuhan bersama naik ke `core/`.
- Widget tidak menyentuh `Dio`/`drift` langsung — selalu lewat controller ke repository.
- Komponen UI yang dipakai lebih dari satu fitur pindah ke `core/widgets/` (paritas dengan
  konvensi "extract reusable components" di frontend web).
- String UI selalu lewat ARB id + en — tidak ada teks hardcode (aturan yang sama dengan web).
- Warna/spacing selalu dari tema — tidak ada `Color(0xFF...)` literal di widget.

## 4. Kode generated

- `freezed`, `json_serializable`, dan `drift` memakai `build_runner`; file hasil
  (`*.freezed.dart`, `*.g.dart`, `*.drift.dart`) **di-commit** (preseden repo: `backend/db/sqlc`
  di-commit) supaya CI tidak perlu langkah codegen dan diff terlihat saat kontrak berubah.
- Jangan pernah mengedit file generated — ubah sumbernya lalu jalankan
  `dart run build_runner build --delete-conflicting-outputs`.

## 5. Error handling

- Repository melempar `AppFailure` (sealed) — tidak ada `catch (e) {}` kosong, tidak ada rethrow
  string mentah backend ke UI.
- Setiap layar menangani tiga cabang `AsyncValue` (loading/error/data) + empty state — sama
  wajibnya dengan konvensi state layar web.
- Kegagalan sync opname tidak pernah senyap (FR-M5.6): status antrean selalu terlihat via
  `SyncPill`; error tercatat ke crash reporter sebagai non-fatal.

## 6. Testing

Konvensi proyek berlaku penuh: **proaktif dan ekspansif** — happy path saja tidak cukup; edge
case, empty/error/loading, input tidak valid, dan variasi permission harus tertutup tes.

- Struktur `test/` mencerminkan `lib/` (`test/features/scan/...` untuk `lib/features/scan/...`);
  nama file `<sumber>_test.dart`.
- Unit test untuk semua logika `data/` (repository dengan Dio di-mock, sync engine dengan drift
  in-memory + clock/koneksi palsu). Assert perilaku nyata, bukan sekadar "tidak error".
- Widget test untuk tiap screen: minimal satu tes per state (loading, error, empty, data) +
  interaksi utama; teks di-assert lewat kunci i18n yang ter-resolve.
- Golden test untuk layar utama, light dan dark (paritas aturan mockup 1:1).
- `integration_test/` melawan backend `docker-compose` + seeded admin; data unik per run dan
  rate limit dimatikan (pelajaran e2e web yang sudah tercatat).
- Gate sebelum commit (dan CI): `flutter analyze`, `flutter test`, build APK debug. Integration
  test berjalan di job CI terpisah (pola job e2e web).

## 7. Git dan alur kerja

- Branch `feat/mobile-<topik>` (atau `fix/mobile-<topik>`); commit **Conventional Commits**
  ber-scope `mobile`: `feat(mobile): ...`, `fix(mobile): ...`; perbaikan keamanan
  `fix(security): ...` (aturan repo, tetap berlaku).
- Tiap fase roadmap mendapat spec + plan di `docs/superpowers/` sebelum kode; layar dibangun
  hanya setelah mockup `docs/mobile/design/` ada, ditutup perbandingan 1:1 (light + dark).
- `docs/PROGRESS.md` di-update pada commit/PR yang menyelesaikan tugas — bagian dari "done".
- Prosa (komentar, docs, commit) memakai kata biasa, bukan simbol tipografi (tanpa section-sign,
  panah, dsb.) — konvensi repo.

## 8. Keamanan

- Tidak ada secret/API key di repo maupun di kode — konfigurasi build via `--dart-define` /
  berkas lokal yang di-gitignore.
- Token hanya di memori + cookie jar terenkripsi (ARCHITECTURE bagian 6); dilarang menulis
  token/kredensial/PII ke log, crash report, atau SharedPreferences.
- Data lokal opname dihapus sesuai lifecycle (FR-M5.8); tidak menyimpan data server lain di
  device.
- Build release memakai `--obfuscate --split-debug-info` (ADR-0015).
