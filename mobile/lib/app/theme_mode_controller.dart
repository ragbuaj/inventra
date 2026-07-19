import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/prefs/app_preferences.dart';

/// Tema pilihan pengguna (baris Tema layar Pengaturan): Terang / Gelap /
/// Ikuti Sistem. Default [ThemeMode.system]; pilihan persist ke
/// SharedPreferences (preferensi non-sensitif) dan terbaca saat cold start.
final NotifierProvider<ThemeModeController, ThemeMode>
themeModeControllerProvider = NotifierProvider<ThemeModeController, ThemeMode>(
  ThemeModeController.new,
);

class ThemeModeController extends Notifier<ThemeMode> {
  @override
  ThemeMode build() {
    final String? stored = ref
        .watch(appPreferencesProvider)
        .getString(PrefKeys.themeMode);
    return switch (stored) {
      'light' => ThemeMode.light,
      'dark' => ThemeMode.dark,
      _ => ThemeMode.system,
    };
  }

  void setMode(ThemeMode mode) {
    state = mode;
    final String stored = switch (mode) {
      ThemeMode.light => 'light',
      ThemeMode.dark => 'dark',
      ThemeMode.system => 'system',
    };
    // Fire-and-forget — lihat catatan LocaleController.setLocale.
    unawaited(
      ref.read(appPreferencesProvider).setString(PrefKeys.themeMode, stored),
    );
  }
}
