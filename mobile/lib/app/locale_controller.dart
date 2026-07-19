import 'dart:async';
import 'dart:ui';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/prefs/app_preferences.dart';

/// Locale aplikasi yang dipilih pengguna (pill ID/EN layar login dan baris
/// Bahasa layar Pengaturan).
///
/// Null berarti mengikuti locale perangkat (id default, en fallback via
/// resolusi `supportedLocales`). Pilihan persist ke SharedPreferences
/// (preferensi non-sensitif) dan terbaca lagi saat cold start.
final NotifierProvider<LocaleController, Locale?> localeControllerProvider =
    NotifierProvider<LocaleController, Locale?>(LocaleController.new);

class LocaleController extends Notifier<Locale?> {
  static const Set<String> _supported = <String>{'id', 'en'};

  @override
  Locale? build() {
    final String? stored = ref
        .watch(appPreferencesProvider)
        .getString(PrefKeys.locale);
    if (stored != null && _supported.contains(stored)) {
      return Locale(stored);
    }
    return null;
  }

  void setLocale(Locale locale) {
    state = locale;
    // Fire-and-forget: cache in-memory langsung terbarui; kegagalan tulis
    // disk hanya berarti preferensi tidak terbawa ke sesi berikutnya.
    unawaited(
      ref
          .read(appPreferencesProvider)
          .setString(PrefKeys.locale, locale.languageCode),
    );
  }
}
