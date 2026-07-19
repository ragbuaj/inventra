import 'dart:ui';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/locale_controller.dart';
import 'package:inventra_mobile/core/prefs/app_preferences.dart';

import '../helpers/fake_app_preferences.dart';

void main() {
  ProviderContainer createContainer(FakeAppPreferences prefs) {
    return ProviderContainer.test(
      overrides: [appPreferencesProvider.overrideWithValue(prefs)],
    );
  }

  test('tanpa preferensi tersimpan: null (ikuti perangkat)', () {
    final ProviderContainer container = createContainer(FakeAppPreferences());

    expect(container.read(localeControllerProvider), isNull);
  });

  test('cold start membaca pilihan tersimpan', () {
    final ProviderContainer container = createContainer(
      FakeAppPreferences(<String, String>{PrefKeys.locale: 'en'}),
    );

    expect(container.read(localeControllerProvider), const Locale('en'));
  });

  test('nilai tersimpan di luar id/en diabaikan (fallback perangkat)', () {
    final ProviderContainer container = createContainer(
      FakeAppPreferences(<String, String>{PrefKeys.locale: 'fr'}),
    );

    expect(container.read(localeControllerProvider), isNull);
  });

  test('setLocale meng-update state DAN menulis preferensi', () async {
    final FakeAppPreferences prefs = FakeAppPreferences();
    final ProviderContainer container = createContainer(prefs);

    container
        .read(localeControllerProvider.notifier)
        .setLocale(const Locale('en'));
    await container.pump();

    expect(container.read(localeControllerProvider), const Locale('en'));
    expect(prefs.setCalls, <(String, String)>[(PrefKeys.locale, 'en')]);
  });
}
