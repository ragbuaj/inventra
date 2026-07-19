@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/scan/presentation/scan_camera.dart';
import 'package:inventra_mobile/features/scan/presentation/scan_screen.dart';

import '../helpers/fake_scan_camera.dart';
import '../helpers/golden_fonts.dart';

/// Golden Scan light + dark. Memakai state kamera-tak-tersedia yang
/// deterministik (frame kamera nyata tidak bisa direproduksi di tes);
/// viewfinder tetap gelap di kedua tema sesuai mockup. Digenerate dan
/// diverifikasi lokal (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScan(ThemeData theme) {
    return ProviderScope(
      overrides: [
        scanCameraFactoryProvider.overrideWithValue(
          () => FakeScanCamera(unavailable: true),
        ),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const ScanScreen(),
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

  testWidgets('scan kamera tak tersedia light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScan(InventraTheme.light));
    await expectLater(
      find.byType(ScanScreen),
      matchesGoldenFile('scan_light.png'),
    );
  });

  testWidgets('scan kamera tak tersedia dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScan(InventraTheme.dark));
    await expectLater(
      find.byType(ScanScreen),
      matchesGoldenFile('scan_dark.png'),
    );
  });
}
