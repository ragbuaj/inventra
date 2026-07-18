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

Font Inter di-bundle sebagai asset (`fonts/`, lisensi SIL OFL di `fonts/OFL.txt`).
String UI lewat ARB `lib/core/i18n/arb/` (id default + en, gen_l10n).
