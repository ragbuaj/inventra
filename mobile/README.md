# Inventra Mobile

Aplikasi mobile companion Inventra (Flutter). Dokumen acuan:

- **Cara menjalankan (lengkap): `docs/mobile/SETUP.md`** — prasyarat, codegen, backend, build APK
- Arsitektur dan struktur folder: `docs/mobile/ARCHITECTURE.md`
- Konvensi kode: `docs/mobile/CONVENTIONS.md`
- Kebutuhan produk: `docs/mobile/PRD.md`

## Prasyarat singkat

- Flutter SDK stable (Dart `^3.12.2`, lihat `environment.sdk` di `pubspec.yaml`).
- Android SDK hanya untuk `flutter run` ke device/emulator atau `flutter build apk` — `analyze`
  dan `test` tidak membutuhkannya.
- Backend Inventra berjalan (`docker compose -f docker-compose.dev.yml up -d` dari root repo).

## Perintah dasar (dari folder `mobile/`)

```
flutter pub get
dart run build_runner build --delete-conflicting-outputs   # regen freezed/*.g bila DTO berubah
flutter analyze   # wajib nol issue
flutter test      # tambahkan --exclude-tags golden untuk meniru CI
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080   # emulator Android -> backend host
```

## Base URL backend (`API_BASE_URL`)

Alamat backend dibaca dari `--dart-define=API_BASE_URL=...` (lihat
`lib/core/api/dio_provider.dart`). Default `http://localhost:8080` hanya cocok
untuk desktop/web di mesin yang sama; di emulator/perangkat fisik `localhost`
menunjuk ke perangkat itu sendiri, jadi WAJIB dioverride:

- Emulator Android: host mesin diakses lewat `http://10.0.2.2:8080`.
- Emulator iOS: `http://localhost:8080` menunjuk ke host, biasanya jalan.
- Perangkat fisik: pakai IP host di jaringan yang sama, mis.
  `http://192.168.1.10:8080` (perangkat dan host harus satu jaringan).

Aplikasi menambahkan sufiks `/api/v1` sendiri, jadi isi `API_BASE_URL` hanya
sampai skema + host + port (tanpa `/api/v1`).

Contoh perintah:

```
# Jalankan di emulator Android menunjuk backend lokal di host
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080

# Jalankan di perangkat fisik menunjuk IP host
flutter run --dart-define=API_BASE_URL=http://192.168.1.10:8080

# Build APK release menunjuk backend produksi
flutter build apk --release \
  --dart-define=API_BASE_URL=https://inventra.ragilbuaj.web.id
```

Font Inter di-bundle sebagai asset (`fonts/`, lisensi SIL OFL di `fonts/OFL.txt`).
String UI lewat ARB `lib/core/i18n/arb/` (id default + en, gen_l10n).
