@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/login/presentation/login_screen.dart';

import '../helpers/fake_auth_controller.dart';
import '../helpers/golden_fonts.dart';

/// Golden Login light + dark (paritas mockup 1:1). Digenerate dan diverifikasi
/// lokal (Windows): `flutter test --update-goldens --tags golden`; CI
/// melewatinya (lihat dart_test.yaml).
void main() {
  setUpAll(loadAppFonts);

  Widget buildLogin(ThemeData theme) {
    return ProviderScope(
      overrides: [authControllerProvider.overrideWith(FakeAuthController.new)],
      child: MaterialApp(
        theme: theme,
        // Golden dibandingkan terhadap mockup berbahasa Indonesia.
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const LoginScreen(),
      ),
    );
  }

  Future<void> pumpAtPhoneSize(WidgetTester tester, Widget widget) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(widget);
    await tester.pumpAndSettle();
  }

  testWidgets('login default light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildLogin(InventraTheme.light));
    await expectLater(
      find.byType(LoginScreen),
      matchesGoldenFile('login_light.png'),
    );
  });

  testWidgets('login default dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildLogin(InventraTheme.dark));
    await expectLater(
      find.byType(LoginScreen),
      matchesGoldenFile('login_dark.png'),
    );
  });
}
