import 'dart:ui';

import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Locale aplikasi yang dipilih pengguna (pill ID/EN di layar login).
///
/// Null berarti mengikuti locale perangkat (id default, en fallback via
/// resolusi `supportedLocales`). Persist ke storage lokal menyusul di Task 12
/// (layar Pengaturan) — untuk sekarang hanya state runtime.
final NotifierProvider<LocaleController, Locale?> localeControllerProvider =
    NotifierProvider<LocaleController, Locale?>(LocaleController.new);

class LocaleController extends Notifier<Locale?> {
  @override
  Locale? build() => null;

  void setLocale(Locale locale) {
    state = locale;
  }
}
