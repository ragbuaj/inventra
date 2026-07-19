# SETUP — Menjalankan Inventra Mobile

Onboarding developer untuk aplikasi Flutter di `mobile/`. Untuk arsitektur lihat
[ARCHITECTURE.md](ARCHITECTURE.md), konvensi kode [CONVENTIONS.md](CONVENTIONS.md).

Perintah cepat ada di [`mobile/README.md`](../../mobile/README.md); dokumen ini yang lengkap.

## 1. Prasyarat

| Kebutuhan | Untuk |
|---|---|
| **Flutter SDK stable** (Dart `^3.12.2`, lihat `mobile/pubspec.yaml` `environment.sdk`) | wajib semua |
| Android SDK + platform-tools (via Android Studio atau `cmdline-tools`) | `flutter run` ke device/emulator, `flutter build apk` |
| Emulator Android atau perangkat fisik (USB debugging) | menjalankan aplikasi |
| Backend Inventra berjalan (lihat bagian 4) | aplikasi memanggil API nyata |

**`flutter analyze` dan `flutter test` TIDAK butuh Android SDK** — cukup Flutter SDK. Build APK
membutuhkan Android SDK; di proyek ini APK debug juga dibangun otomatis oleh CI job `mobile`
(`.github/workflows/ci.yml`), jadi Android SDK lokal hanya perlu bila ingin menjalankan di
device/emulator sendiri.

### Memasang Flutter (Windows, tanpa Android Studio)

```bash
git clone -b stable https://github.com/flutter/flutter.git C:\flutter
# tambahkan C:\flutter\bin ke PATH user, lalu:
flutter --version      # harus channel stable
flutter doctor         # cek toolchain; Android toolchain X wajar bila belum pasang Android SDK
```

Untuk menjalankan di perangkat, pasang Android SDK (Android Studio menyediakan installer termudah)
lalu `flutter doctor --android-licenses`.

## 2. Ambil dependensi + codegen

Dari folder `mobile/`:

```bash
flutter pub get

# Codegen (freezed + json_serializable + drift bila ada) — file *.freezed.dart / *.g.dart
# DI-COMMIT (konvensi), tapi regenerate bila mengubah DTO/model:
dart run build_runner build --delete-conflicting-outputs
```

Lokalisasi (`gen_l10n`) berjalan otomatis saat `flutter pub get`/build karena `generate: true`
di `pubspec.yaml`; sumber ARB di `lib/core/i18n/arb/` (`app_id.arb` default + `app_en.arb`).

## 3. Perintah harian

```bash
flutter analyze                 # wajib NOL issue (gate CI)
flutter test                    # unit + widget + golden
flutter test --exclude-tags golden   # seperti CI (golden platform-dependent, gate lokal)
dart format lib test            # format
flutter run                     # jalankan (butuh device/emulator)
```

**Golden test** ditandai tag `golden` dan di-*exclude* dari CI karena rendering font bergantung
platform; file golden digenerate di mesin developer. Untuk meregenerasi setelah perubahan UI yang
disengaja: `flutter test --update-goldens --tags golden`.

## 4. Menjalankan terhadap backend

Aplikasi adalah konsumen `/api/v1`, jadi backend harus hidup. Untuk dev lokal, jalankan stack dari
root repo (backend + Postgres + Redis + Mailpit):

```bash
docker compose -f docker-compose.dev.yml up -d
```

Login memakai akun seeded dev (lihat `backend/db/seed/`). Backend mendengarkan di `:8080`.

### Base URL (`API_BASE_URL`)

Alamat backend dibaca dari `--dart-define=API_BASE_URL=...` (lihat `lib/core/api/dio_provider.dart`).
Default `http://localhost:8080` hanya cocok untuk host yang sama; di emulator/perangkat `localhost`
menunjuk ke perangkat itu sendiri, jadi WAJIB dioverride. Aplikasi menambahkan sufiks `/api/v1`
sendiri — isi hanya sampai skema + host + port.

| Target | API_BASE_URL |
|---|---|
| Emulator Android | `http://10.0.2.2:8080` (host diakses via 10.0.2.2) |
| Emulator iOS | `http://localhost:8080` (localhost = host) |
| Perangkat fisik | `http://<IP-host>:8080` (satu jaringan, mis. `http://192.168.1.10:8080`) |
| Produksi | `https://inventra.ragilbuaj.web.id` |

```bash
# Emulator Android menunjuk backend lokal di host
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080

# Perangkat fisik menunjuk IP host
flutter run --dart-define=API_BASE_URL=http://192.168.1.10:8080
```

Klien mobile mengirim header `X-Client-Type: mobile` (ADR-0017): menerima refresh token di body,
disimpan di `flutter_secure_storage`. Tidak ada perubahan yang perlu di klien untuk ini.

## 5. Build APK

```bash
flutter build apk --debug     # yang dibangun CI
flutter build apk --release --dart-define=API_BASE_URL=https://inventra.ragilbuaj.web.id
```

Build release memakai `--obfuscate --split-debug-info=<dir>` per CONVENTIONS bagian 8 (menyusul
diformalkan di RELEASE.md fase M6). Perangkat memerlukan izin kamera (Scan) — sudah dideklarasikan
di `AndroidManifest.xml` (`CAMERA`) dan `ios/Runner/Info.plist` (`NSCameraUsageDescription`).

## 6. Masalah umum

- **`flutter` tidak dikenali** — `C:\flutter\bin` belum di PATH, atau shell belum di-refresh.
- **Semua panggilan API gagal (SocketException)** di emulator/perangkat — `API_BASE_URL` belum
  dioverride dari `localhost` (lihat bagian 4).
- **`flutter doctor` menandai Android toolchain X** — wajar bila belum memasang Android SDK; tidak
  memblokir `analyze`/`test`. Pasang Android SDK hanya untuk `run`/`build apk`.
- **Golden test gagal padahal UI tak berubah** — golden platform-dependent; regenerasi lokal dengan
  `flutter test --update-goldens --tags golden`, jangan commit golden dari platform berbeda.
- **`build_runner` konflik output** — jalankan dengan `--delete-conflicting-outputs`.
- **Build APK gagal `Could not close incremental caches` / `Daemon compilation failed`** — terjadi
  bila pub cache (default `C:\Users\<user>\AppData\Local\Pub\Cache`) berbeda drive dengan folder
  proyek (mis. `D:`); kompiler inkremental Kotlin gagal merelatifkan path lintas-drive di Windows.
  Sudah dimitigasi dengan `kotlin.incremental=false` di `mobile/android/gradle.properties` — jangan
  dihapus selama pub cache dan proyek beda drive.
- **Error basi `Plugin directory does not exist` padahal folder pub cache ada** — daemon Gradle lama
  menahan hasil evaluasi sebelumnya; matikan proses `java` (daemon Gradle/Kotlin) lalu build ulang.
