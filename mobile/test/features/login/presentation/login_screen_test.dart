import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/features/login/presentation/login_screen.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/test_app.dart';

void main() {
  Widget buildLogin(FakeAuthController fake) {
    return buildScreenHarness(
      container: ProviderContainer.test(
        overrides: [authControllerProvider.overrideWith(() => fake)],
      ),
      child: const LoginScreen(),
    );
  }

  // Ukuran layar ponsel mockup (390x844) supaya seluruh konten - termasuk
  // footer pill bahasa - berada dalam viewport dan bisa di-tap.
  Future<void> pumpLogin(WidgetTester tester, FakeAuthController fake) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(buildLogin(fake));
    await tester.pumpAndSettle();
  }

  Future<void> submitCredentials(
    WidgetTester tester, {
    String email = 'budi.santoso@bank.co.id',
    String password = 'rahasia-123',
  }) async {
    await tester.enterText(find.byType(TextField).first, email);
    await tester.enterText(find.byType(TextField).last, password);
    // Judul card memakai teks yang sama ("Masuk") — target tombolnya.
    await tester.tap(find.byType(FilledButton));
  }

  group('state default', () {
    testWidgets('menampilkan branding, form, dan footer', (
      WidgetTester tester,
    ) async {
      await pumpLogin(tester, FakeAuthController());

      expect(find.text(l10nId.loginBrandName), findsOneWidget);
      expect(find.text(l10nId.loginBrandBadge), findsOneWidget);
      expect(find.text(l10nId.loginTagline), findsOneWidget);
      // Judul card dan tombol memakai teks yang sama ("Masuk").
      expect(find.text(l10nId.loginCardTitle), findsNWidgets(2));
      expect(find.text(l10nId.loginCardSubtitle), findsOneWidget);
      expect(find.text(l10nId.loginEmailLabel), findsOneWidget);
      expect(find.text(l10nId.loginEmailHint), findsOneWidget);
      expect(find.text(l10nId.loginPasswordLabel), findsOneWidget);
      expect(find.text(l10nId.loginPasswordHint), findsOneWidget);
      expect(find.text(l10nId.loginLanguageIndonesian), findsOneWidget);
      expect(find.text(l10nId.loginLanguageEnglish), findsOneWidget);
      expect(find.textContaining('Inventra Mobile v'), findsOneWidget);
      // Tanpa error, banner tidak dirender.
      expect(find.text(l10nId.loginErrorInvalidCredentials), findsNothing);
    });

    testWidgets('kata sandi tersembunyi dan bisa di-toggle', (
      WidgetTester tester,
    ) async {
      await pumpLogin(tester, FakeAuthController());

      TextField passwordField() =>
          tester.widget<TextField>(find.byType(TextField).last);
      expect(passwordField().obscureText, isTrue);

      await tester.tap(find.byTooltip(l10nId.loginShowPassword));
      await tester.pump();
      expect(passwordField().obscureText, isFalse);
      expect(find.byIcon(Symbols.visibility_rounded), findsOneWidget);

      await tester.tap(find.byTooltip(l10nId.loginHidePassword));
      await tester.pump();
      expect(passwordField().obscureText, isTrue);
    });
  });

  group('submit', () {
    testWidgets('meneruskan kredensial (email di-trim) ke authController', (
      WidgetTester tester,
    ) async {
      final FakeAuthController fake = FakeAuthController();
      await pumpLogin(tester, fake);

      await tester.enterText(
        find.byType(TextField).first,
        '  budi.santoso@bank.co.id ',
      );
      await tester.enterText(find.byType(TextField).last, 'rahasia-123');
      await tester.tap(find.byType(FilledButton));
      await tester.pumpAndSettle();

      expect(fake.loginCalls, hasLength(1));
      expect(fake.loginCalls.single.email, 'budi.santoso@bank.co.id');
      expect(fake.loginCalls.single.password, 'rahasia-123');
    });
  });

  group('state loading', () {
    testWidgets('input disabled dan tombol berisi spinner + Memproses', (
      WidgetTester tester,
    ) async {
      final FakeAuthController fake = FakeAuthController(holdLogin: true);
      await pumpLogin(tester, fake);

      await submitCredentials(tester);
      await tester.pump(const Duration(milliseconds: 50));

      expect(find.text(l10nId.loginSubmitLoading), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      for (final TextField field in tester.widgetList<TextField>(
        find.byType(TextField),
      )) {
        expect(field.enabled, isFalse);
      }
      final FilledButton button = tester.widget<FilledButton>(
        find.byType(FilledButton),
      );
      expect(button.onPressed, isNull);

      fake.releaseLogin();
      await tester.pumpAndSettle();
    });
  });

  group('state error', () {
    Future<void> expectFailureMessage(
      WidgetTester tester,
      AppFailure failure,
      String message,
    ) async {
      await pumpLogin(tester, FakeAuthController(failureOnLogin: failure));

      await submitCredentials(tester);
      await tester.pumpAndSettle();

      expect(find.text(message), findsOneWidget);
      expect(find.byIcon(Symbols.error_rounded), findsOneWidget);
    }

    testWidgets('401 menampilkan pesan kredensial salah', (
      WidgetTester tester,
    ) async {
      await expectFailureMessage(
        tester,
        const UnauthorizedFailure(),
        l10nId.loginErrorInvalidCredentials,
      );
    });

    testWidgets('400 validasi juga dipetakan ke kredensial salah', (
      WidgetTester tester,
    ) async {
      await expectFailureMessage(
        tester,
        const ValidationFailure('invalid body'),
        l10nId.loginErrorInvalidCredentials,
      );
    });

    testWidgets('kegagalan jaringan menampilkan pesan offline', (
      WidgetTester tester,
    ) async {
      await expectFailureMessage(
        tester,
        const NetworkFailure(),
        l10nId.loginErrorNetwork,
      );
    });

    testWidgets('429 menampilkan pesan coba lagi nanti', (
      WidgetTester tester,
    ) async {
      await expectFailureMessage(
        tester,
        const RateLimitedFailure(),
        l10nId.loginErrorRateLimited,
      );
    });

    testWidgets('kegagalan lain menampilkan pesan generik', (
      WidgetTester tester,
    ) async {
      await expectFailureMessage(
        tester,
        const ServerFailure(),
        l10nId.loginErrorGeneric,
      );
    });

    testWidgets('border kedua input berubah menjadi error', (
      WidgetTester tester,
    ) async {
      await pumpLogin(
        tester,
        FakeAuthController(failureOnLogin: const UnauthorizedFailure()),
      );
      await submitCredentials(tester);
      await tester.pumpAndSettle();

      final ColorScheme scheme = Theme.of(
        tester.element(find.byType(LoginScreen)),
      ).colorScheme;
      for (final TextField field in tester.widgetList<TextField>(
        find.byType(TextField),
      )) {
        final InputBorder? border = field.decoration?.enabledBorder;
        expect(border, isNotNull);
        expect(border!.borderSide.color, scheme.error);
      }
    });
  });

  group('switch bahasa', () {
    testWidgets('tap EN mengganti seluruh teks ke bahasa Inggris', (
      WidgetTester tester,
    ) async {
      await pumpLogin(tester, FakeAuthController());

      await tester.tap(find.text(l10nId.loginLanguageEnglish));
      await tester.pumpAndSettle();

      expect(find.text(l10nEn.loginCardSubtitle), findsOneWidget);
      expect(find.text(l10nEn.loginPasswordLabel), findsOneWidget);
      expect(find.text(l10nId.loginCardSubtitle), findsNothing);
    });
  });
}
