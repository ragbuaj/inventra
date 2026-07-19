import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme_mode_controller.dart';
import 'package:inventra_mobile/core/prefs/app_preferences.dart';

import '../helpers/fake_app_preferences.dart';

void main() {
  ProviderContainer createContainer(FakeAppPreferences prefs) {
    return ProviderContainer.test(
      overrides: [appPreferencesProvider.overrideWithValue(prefs)],
    );
  }

  test('tanpa preferensi tersimpan: ikuti sistem', () {
    final ProviderContainer container = createContainer(FakeAppPreferences());

    expect(container.read(themeModeControllerProvider), ThemeMode.system);
  });

  test('cold start membaca pilihan tersimpan (dark dan light)', () {
    expect(
      createContainer(
        FakeAppPreferences(<String, String>{PrefKeys.themeMode: 'dark'}),
      ).read(themeModeControllerProvider),
      ThemeMode.dark,
    );
    expect(
      createContainer(
        FakeAppPreferences(<String, String>{PrefKeys.themeMode: 'light'}),
      ).read(themeModeControllerProvider),
      ThemeMode.light,
    );
  });

  test('nilai tersimpan tak dikenal jatuh ke sistem', () {
    final ProviderContainer container = createContainer(
      FakeAppPreferences(<String, String>{PrefKeys.themeMode: 'sepia'}),
    );

    expect(container.read(themeModeControllerProvider), ThemeMode.system);
  });

  test('setMode meng-update state DAN menulis preferensi', () async {
    final FakeAppPreferences prefs = FakeAppPreferences();
    final ProviderContainer container = createContainer(prefs);

    container
        .read(themeModeControllerProvider.notifier)
        .setMode(ThemeMode.dark);
    await container.pump();

    expect(container.read(themeModeControllerProvider), ThemeMode.dark);
    expect(prefs.setCalls, <(String, String)>[(PrefKeys.themeMode, 'dark')]);

    container
        .read(themeModeControllerProvider.notifier)
        .setMode(ThemeMode.system);
    await container.pump();

    expect(container.read(themeModeControllerProvider), ThemeMode.system);
    expect(prefs.setCalls.last, (PrefKeys.themeMode, 'system'));
  });
}
