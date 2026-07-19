import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/locale_controller.dart';
import 'package:inventra_mobile/app/theme_mode_controller.dart';
import 'package:inventra_mobile/core/prefs/app_preferences.dart';
import 'package:inventra_mobile/core/utils/app_info.dart';
import 'package:inventra_mobile/features/account/presentation/settings_screen.dart';

import '../../../helpers/fake_app_preferences.dart';
import '../../../helpers/test_app.dart';

void main() {
  late FakeAppPreferences prefs;

  Future<ProviderContainer> pumpSettings(
    WidgetTester tester, {
    Map<String, String>? initialPrefs,
  }) async {
    prefs = FakeAppPreferences(initialPrefs);
    final ProviderContainer container = ProviderContainer.test(
      overrides: [appPreferencesProvider.overrideWithValue(prefs)],
    );
    await tester.pumpWidget(
      buildScreenHarness(container: container, child: const SettingsScreen()),
    );
    await tester.pumpAndSettle();
    return container;
  }

  group('nilai awal', () {
    testWidgets('default: tema Ikuti Sistem, bahasa Indonesia, versi '
        'aplikasi', (WidgetTester tester) async {
      await pumpSettings(tester);

      expect(find.text(l10nId.settingsTheme), findsOneWidget);
      expect(find.text(l10nId.settingsThemeSystem), findsOneWidget);
      expect(find.text(l10nId.settingsLanguage), findsOneWidget);
      expect(find.text(l10nId.settingsLanguageIndonesian), findsOneWidget);
      expect(find.text(l10nId.settingsAppName), findsOneWidget);
      expect(
        find.text(l10nId.settingsVersion(AppInfo.version, AppInfo.buildNumber)),
        findsOneWidget,
      );
    });

    testWidgets('preferensi tersimpan terbaca saat cold start', (
      WidgetTester tester,
    ) async {
      final ProviderContainer container = await pumpSettings(
        tester,
        initialPrefs: <String, String>{
          PrefKeys.themeMode: 'dark',
          PrefKeys.locale: 'en',
        },
      );

      expect(container.read(themeModeControllerProvider), ThemeMode.dark);
      expect(container.read(localeControllerProvider), const Locale('en'));
      // Layar merender label EN + subtitle tema Gelap (en).
      expect(find.text(l10nEn.settingsTitle), findsOneWidget);
      expect(find.text(l10nEn.settingsThemeDark), findsOneWidget);
    });
  });

  group('bahasa', () {
    testWidgets('ganti ke English: langsung berefek pada UI + persist', (
      WidgetTester tester,
    ) async {
      final ProviderContainer container = await pumpSettings(tester);

      await tester.tap(find.byKey(const ValueKey<String>('settings-language')));
      await tester.pumpAndSettle();
      expect(find.text(l10nId.settingsLanguageSheetTitle), findsOneWidget);

      await tester.tap(
        find.byKey(const ValueKey<String>('settings-language-en')),
      );
      await tester.pumpAndSettle();

      expect(container.read(localeControllerProvider), const Locale('en'));
      expect(prefs.setCalls, contains((PrefKeys.locale, 'en')));
      // Teks layar langsung berbahasa Inggris (judul seksi dirender kapital).
      expect(find.text(l10nEn.settingsTitle), findsOneWidget);
      expect(
        find.text(l10nEn.settingsSectionAppearance.toUpperCase()),
        findsOneWidget,
      );
      expect(
        find.text(l10nId.settingsSectionAppearance.toUpperCase()),
        findsNothing,
      );
    });

    testWidgets('opsi aktif bertanda centang', (WidgetTester tester) async {
      await pumpSettings(tester);

      await tester.tap(find.byKey(const ValueKey<String>('settings-language')));
      await tester.pumpAndSettle();

      // Tap opsi yang sudah aktif: tetap id, tanpa perubahan.
      await tester.tap(
        find.byKey(const ValueKey<String>('settings-language-id')),
      );
      await tester.pumpAndSettle();
      expect(find.text(l10nId.settingsTitle), findsOneWidget);
    });
  });

  group('tema', () {
    testWidgets('pilih Gelap lalu Terapkan: state berubah + persist + '
        'subtitle ter-update', (WidgetTester tester) async {
      final ProviderContainer container = await pumpSettings(tester);

      await tester.tap(find.byKey(const ValueKey<String>('settings-theme')));
      await tester.pumpAndSettle();
      expect(find.text(l10nId.settingsThemeSheetTitle), findsOneWidget);

      await tester.tap(
        find.byKey(const ValueKey<String>('settings-theme-tile-dark')),
      );
      await tester.pumpAndSettle();
      await tester.tap(
        find.byKey(const ValueKey<String>('settings-theme-apply')),
      );
      await tester.pumpAndSettle();

      expect(container.read(themeModeControllerProvider), ThemeMode.dark);
      expect(prefs.setCalls, contains((PrefKeys.themeMode, 'dark')));
      expect(find.text(l10nId.settingsThemeDark), findsOneWidget);
    });

    testWidgets('menutup sheet tanpa Terapkan tidak mengubah apa pun', (
      WidgetTester tester,
    ) async {
      final ProviderContainer container = await pumpSettings(tester);

      await tester.tap(find.byKey(const ValueKey<String>('settings-theme')));
      await tester.pumpAndSettle();
      await tester.tap(
        find.byKey(const ValueKey<String>('settings-theme-tile-dark')),
      );
      await tester.pumpAndSettle();
      // Tutup lewat barrier (tanpa Terapkan).
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      expect(container.read(themeModeControllerProvider), ThemeMode.system);
      expect(prefs.setCalls, isEmpty);
      expect(find.text(l10nId.settingsThemeSystem), findsOneWidget);
    });
  });
}
