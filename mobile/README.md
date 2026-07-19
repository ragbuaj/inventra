# Inventra Mobile

Aplikasi mobile companion Inventra (Flutter). Dokumen acuan:

- Arsitektur dan struktur folder: `docs/mobile/ARCHITECTURE.md`
- Konvensi kode: `docs/mobile/CONVENTIONS.md`
- Kebutuhan produk: `docs/mobile/PRD.md`

Perintah dasar (dari folder `mobile/`):

```
flutter pub get
flutter analyze   # wajib nol issue
flutter test
flutter run
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
