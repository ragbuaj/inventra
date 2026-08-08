import 'dart:math' as math;

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

    // Token primary: biru korporat Bank BTN (brand-500 light, brand-400 dark).
    expect(app.theme?.colorScheme.primary, const Color(0xFF005BFD));
    expect(app.darkTheme?.colorScheme.primary, const Color(0xFF5891FF));
    expect(app.darkTheme?.colorScheme.onPrimary, const Color(0xFF02194F));

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

  // Rasio kontras WCAG 2.1. Warna brand ditetapkan sekali lalu diwarisi setiap
  // layar, jadi penurunan kontras di sini menyebar diam-diam ke seluruh
  // aplikasi — dikunci sebagai tes, bukan diserahkan ke pemeriksaan manual.
  double luminance(Color c) {
    double channel(double v) {
      return v <= 0.03928
          ? v / 12.92
          : math.pow((v + 0.055) / 1.055, 2.4).toDouble();
    }

    return 0.2126 * channel(c.r) +
        0.7152 * channel(c.g) +
        0.0722 * channel(c.b);
  }

  double contrast(Color a, Color b) {
    final double x = luminance(a);
    final double y = luminance(b);
    return (math.max(x, y) + 0.05) / (math.min(x, y) + 0.05);
  }

  group('kontras warna brand memenuhi WCAG AA', () {
    final ColorScheme light = InventraTheme.light.colorScheme;
    final ColorScheme dark = InventraTheme.dark.colorScheme;

    test('teks di atas primary terbaca di kedua tema', () {
      expect(
        contrast(light.onPrimary, light.primary),
        greaterThanOrEqualTo(4.5),
      );
      expect(contrast(dark.onPrimary, dark.primary), greaterThanOrEqualTo(4.5));
    });

    test('teks di atas primaryContainer terbaca di kedua tema', () {
      expect(
        contrast(light.onPrimaryContainer, light.primaryContainer),
        greaterThanOrEqualTo(4.5),
      );
      // Regresi yang pernah nyaris lolos: padanan step 400 hanya 4.12:1, jadi
      // mode gelap memakai step 300. Menurunkannya lagi harus menggagalkan tes.
      expect(
        contrast(dark.onPrimaryContainer, dark.primaryContainer),
        greaterThanOrEqualTo(4.5),
      );
    });

    test('primary tetap terbaca di atas latar scaffold masing-masing tema', () {
      // Primary dipakai sebagai warna teks tautan/aksi, bukan hanya latar tombol.
      expect(
        contrast(light.primary, const Color(0xFFF8FAFC)),
        greaterThanOrEqualTo(4.5),
      );
      expect(
        contrast(dark.primary, const Color(0xFF0F172A)),
        greaterThanOrEqualTo(4.5),
      );
    });

    test('aksen bingkai pemindai terbaca di atas viewfinder gelap', () {
      // Elemen grafis non-teks: ambang WCAG 3:1.
      expect(
        contrast(
          InventraScanColors.frameAccent,
          InventraScanColors.viewfinderBackground,
        ),
        greaterThanOrEqualTo(3.0),
      );
    });
  });

  test('brand primary memakai biru korporat Bank BTN apa adanya', () {
    // Diambil verbatim dari logo (frontend/public/logo-btn.png). Kalau nilai ini
    // bergeser, ia tidak lagi warna resmi BTN.
    expect(InventraTheme.light.colorScheme.primary, const Color(0xFF005BFD));
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
