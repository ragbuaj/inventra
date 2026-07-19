import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/main.dart';

import '../helpers/fake_auth_controller.dart';
import '../helpers/test_app.dart';

void main() {
  // InventraApp membutuhkan ProviderScope; authController dipalsukan supaya
  // tes tidak menyentuh secure storage (platform channel).
  Widget buildApp() {
    return ProviderScope(
      overrides: [authControllerProvider.overrideWith(FakeAuthController.new)],
      child: const InventraApp(),
    );
  }

  testWidgets('merender MaterialApp dengan tema Inventra light + dark', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    final MaterialApp app = tester.widget<MaterialApp>(
      find.byType(MaterialApp),
    );

    // Token primary dari mockup: hijau light/dark.
    expect(app.theme?.colorScheme.primary, const Color(0xFF16A34A));
    expect(app.darkTheme?.colorScheme.primary, const Color(0xFF22C55E));
    expect(app.darkTheme?.colorScheme.onPrimary, const Color(0xFF052E16));

    // Font Inter di-bundle dan dipakai lewat tema.
    expect(app.theme?.textTheme.bodyMedium?.fontFamily, 'Inter');

    // Judul weight 700.
    expect(app.theme?.textTheme.titleLarge?.fontWeight, FontWeight.w700);

    // ThemeExtension warna status domain terpasang di kedua tema.
    expect(app.theme?.extension<InventraStatusColors>(), isNotNull);
    expect(app.darkTheme?.extension<InventraStatusColors>(), isNotNull);
  });

  testWidgets('locale default id saat locale perangkat tidak didukung', (
    WidgetTester tester,
  ) async {
    tester.platformDispatcher.localesTestValue = const <Locale>[Locale('fr')];
    addTearDown(tester.platformDispatcher.clearLocalesTestValue);

    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    // Belum login: mendarat di layar login berbahasa Indonesia.
    expect(find.text(l10nId.loginCardSubtitle), findsOneWidget);
  });

  testWidgets('locale en didukung sebagai fallback bahasa kedua', (
    WidgetTester tester,
  ) async {
    tester.platformDispatcher.localesTestValue = const <Locale>[Locale('en')];
    addTearDown(tester.platformDispatcher.clearLocalesTestValue);

    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    expect(find.text(l10nEn.loginCardSubtitle), findsOneWidget);
  });

  test('chip status aset memetakan keluarga semantik yang benar', () {
    const InventraStatusColors light = InventraStatusColors.light;
    expect(light.assetAvailable.dot, const Color(0xFF16A34A));
    expect(light.assetBorrowed.dot, const Color(0xFF2563EB));
    expect(light.assetMaintenance.dot, const Color(0xFFD97706));
    expect(light.assetDisposed.dot, const Color(0xFF64748B));
    expect(light.assetLost.dot, const Color(0xFFDC2626));

    const InventraStatusColors dark = InventraStatusColors.dark;
    expect(dark.assetAvailable.bg, const Color(0xFF14532D));
    expect(dark.assetLost.text, const Color(0xFFFCA5A5));
  });
}
